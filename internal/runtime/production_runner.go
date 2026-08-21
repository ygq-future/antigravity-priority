package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

var errMissingHostCallbacks = errors.New("runtime: host callbacks are required")

const (
	autoQuotaProbeAttempts = 3
	autoQuotaProbeDelay    = 5 * time.Second
	defaultProbeCacheTTL   = 15 * time.Minute
)

func (r *Runtime) runProductionTask(ctx context.Context, request TaskRequest) error {
	if !request.Config.Enabled && request.Trigger != TriggerManualApply {
		return nil
	}
	if request.Trigger == TriggerAutoApply && !request.Config.AutoApply {
		return nil
	}
	if r.hostCallbacks == nil {
		return errMissingHostCallbacks
	}
	now := r.clock.Now().UTC()
	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return err
	}

	credentials := credentialsFromAuthFiles(files)
	credentials = filterCredentialsByAuthIndex(credentials, request.AuthIndexes)
	cachePath := request.Config.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}

	credentials, authMaterials, err := enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return err
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return err
	}

	forceProbe := request.Trigger == TriggerManualApply || request.Trigger == TriggerProbe || request.Trigger == TriggerAutoApply
	evidence, err := r.collectEvidenceForTrigger(ctx, collectInput{
		client:         client,
		store:          store,
		credentials:    credentials,
		authMaterials:  authMaterials,
		now:            now,
		cacheTTL:       defaultProbeCacheTTL,
		forceProbe:     forceProbe,
		maxConcurrency: request.Config.MaxConcurrency,
		modelGroup:     request.Config.AntigravityModelGroup,
		sampleCapacity: request.Config.QuotaSampleCapacity,
	}, request.Trigger)
	if err != nil {
		return err
	}

	if err := store.SaveAtomic(ctx); err != nil {
		return err
	}

	// Reconcile against the authoritative Host inventory after the potentially
	// slow Google requests. Credentials may have been added, removed, disabled,
	// or reprioritized while probing was in flight.
	files, err = client.ListAuthFiles(ctx)
	if err != nil {
		return err
	}
	credentials = credentialsFromAuthFiles(files)
	credentials = filterCredentialsByAuthIndex(credentials, request.AuthIndexes)
	credentials, _, err = enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return err
	}
	evidence = currentRoundEvidence(store, credentials, request.Config.AntigravityModelGroup, now)

	plan := priority.PlanFreshOnly(credentials, evidence, priorityOptions(request.Config, store, now))

	// Build dual-group snapshot (REQ-05): compute predicted plan for the alternate model group.
	primarySnapshot := apply.Snapshot(plan)
	altGroup := alternateModelGroup(request.Config.AntigravityModelGroup)
	altEvidence := currentRoundEvidence(store, credentials, altGroup, now)
	altPlan := priority.PlanFreshOnly(credentials, altEvidence, priorityOptions(request.Config, store, now))
	predictedSnapshot := apply.SnapshotPredicted(altPlan)
	dualSnap := apply.NewDualGroupSnapshot(
		string(request.Config.AntigravityModelGroup), now, primarySnapshot, predictedSnapshot)
	r.setDualSnapshot(dualSnap)

	// Probe-only: evidence collected, dual snapshot updated, no apply executed (REQ-04).
	if request.Trigger == TriggerProbe {
		result := apply.Result{Snapshot: primarySnapshot}
		audit := fmt.Sprintf("probe completed: %d credentials probed", len(evidence))
		snap := primarySnapshot
		r.snapshotRunEntry(result, audit, RunHistoryEntry{
			Kind:      KindProbe,
			Trigger:   string(request.Trigger),
			Attempted: len(evidence),
			Succeeded: len(evidence),
			Message:   audit,
			Snapshot:  &snap,
		})
		resJSON, _ := json.Marshal(result)
		histJSON, _ := json.Marshal(r.currentRunHistory())
		store.SetRuntimeSnapshot(audit, resJSON, histJSON)
		_ = store.SaveAtomic(ctx)
		return nil
	}

	if len(plan.Changes) == 0 {
		result := apply.Result{Snapshot: primarySnapshot}
		summary := fmt.Sprintf("all %d credentials in sync, no changes required", len(primarySnapshot.Items))
		r.mu.Lock()
		r.latestResult = result
		r.latestAudit = summary
		r.mu.Unlock()
		resJSON, _ := json.Marshal(result)
		histJSON, _ := json.Marshal(r.currentRunHistory())
		store.SetRuntimeSnapshot(summary, resJSON, histJSON)
		if err := store.SaveAtomic(ctx); err != nil {
			return err
		}
		return nil
	}

	result, err := apply.Apply(ctx, apply.Request{
		Host:              client,
		Auditor:           r,
		Plan:              plan,
		ReportSkippedPlan: true,
	})
	if err != nil {
		return err
	}

	summary := fmt.Sprintf("apply credentials=%d succeeded=%d failed=%d skipped=%d",
		result.Attempted+result.Skipped, result.Succeeded, result.Failed, result.Skipped)

	r.snapshotRunEntry(result, summary, RunHistoryEntry{
		Kind:      KindApply,
		Trigger:   string(request.Trigger),
		Attempted: result.Attempted,
		Succeeded: result.Succeeded,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
		Message:   summary,
		Snapshot:  &primarySnapshot,
	})

	resJSON, _ := json.Marshal(result)
	histJSON, _ := json.Marshal(r.currentRunHistory())
	store.SetRuntimeSnapshot(summary, resJSON, histJSON)
	if err := store.SaveAtomic(ctx); err != nil {
		return err
	}
	return nil
}

func currentRoundEvidence(store *state.Store, credentials []core.Credential, group config.AntigravityModelGroup, observedAt time.Time) []priority.ProbeEvidence {
	all := store.BuildGroupEvidence(credentials, string(group))
	current := make([]priority.ProbeEvidence, 0, len(all))
	for _, item := range all {
		if item.ObservedAt.Equal(observedAt) {
			current = append(current, item)
		}
	}
	return current
}

func (r *Runtime) collectEvidenceForTrigger(ctx context.Context, input collectInput, trigger Trigger) ([]priority.ProbeEvidence, error) {
	if trigger != TriggerAutoApply {
		return collectFreshEvidence(ctx, input)
	}
	var evidence []priority.ProbeEvidence
	for attempt := 1; attempt <= autoQuotaProbeAttempts; attempt++ {
		current, err := collectFreshEvidence(ctx, input)
		if err != nil {
			return nil, err
		}
		evidence = current
		if !hasProbeFailure(current) || attempt == autoQuotaProbeAttempts {
			return evidence, nil
		}
		input.forceProbe = true
		if err := r.sleeper.Sleep(ctx, autoQuotaProbeDelay); err != nil {
			return nil, err
		}
	}
	return evidence, nil
}

func hasProbeFailure(evidence []priority.ProbeEvidence) bool {
	return slices.ContainsFunc(evidence, func(item priority.ProbeEvidence) bool {
		return item.Status == priority.EvidenceStatusProbeFailed
	})
}

func credentialsFromAuthFiles(files []host.AuthFile) []core.Credential {
	credentials := make([]core.Credential, 0, len(files))
	for _, file := range files {
		if isAntigravityAuthFile(file) {
			credentials = append(credentials, core.Credential{
				Name:            file.Name,
				AuthIndex:       file.AuthIndex,
				Provider:        core.ProviderAntigravity,
				Type:            core.CredentialTypeAntigravity,
				Status:          core.CredentialStatus(file.Status),
				Disabled:        file.Disabled,
				Unavailable:     file.Unavailable,
				Priority:        file.Priority,
				PriorityMissing: file.PriorityMissing,
				Account:         file.Account,
				Email:           file.Email,
				PlanType:        core.PlanTypeUnknown,
				RawJSON:         append([]byte(nil), file.RawJSON...),
			})
		}
	}
	return credentials
}

func isAntigravityAuthFile(file host.AuthFile) bool {
	provider := strings.ToLower(strings.TrimSpace(file.Provider))
	credType := strings.ToLower(strings.TrimSpace(file.Type))
	name := strings.ToLower(strings.TrimSpace(file.Name))
	return provider == "antigravity" || provider == "google" || provider == "gemini" || provider == "google-antigravity" ||
		credType == "antigravity" || credType == "google" || credType == "gemini" ||
		strings.Contains(name, "antigravity")
}

func filterCredentialsByAuthIndex(credentials []core.Credential, authIndexes []string) []core.Credential {
	if len(authIndexes) == 0 {
		return credentials
	}
	allowed := make(map[string]struct{}, len(authIndexes))
	for _, authIndex := range authIndexes {
		allowed[authIndex] = struct{}{}
	}
	filtered := make([]core.Credential, 0, len(credentials))
	for _, credential := range credentials {
		if _, ok := allowed[credential.AuthIndex]; ok {
			filtered = append(filtered, credential)
		}
	}
	return filtered
}

func priorityOptions(cfg config.Config, store *state.Store, now time.Time) priority.Options {
	var cooldowns map[string]time.Time
	if store != nil {
		cooldowns = store.GetActiveCooldowns(now)
	}
	return priority.Options{
		Now:                 now,
		BoostStartPriority:  cfg.PriorityRules.BoostStartPriority,
		NormalStartPriority: cfg.PriorityRules.NormalStartPriority,
		MinChange:           cfg.MinChange,
		UrgencyTolerance:    cfg.UrgencyTolerance,
		CooldownAuthIndexes: cooldowns,
	}
}

// SaveSnapshot implements apply.Auditor.
func (r *Runtime) SaveSnapshot(ctx context.Context, snapshot apply.PlanSnapshot) error {
	return ctx.Err()
}

// RecordEvent implements apply.Auditor.
func (r *Runtime) RecordEvent(ctx context.Context, event apply.AuditEvent) error {
	return ctx.Err()
}

var _ apply.Auditor = (*Runtime)(nil)
