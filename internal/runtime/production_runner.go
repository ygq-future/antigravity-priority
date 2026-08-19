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
	if !request.Config.Enabled && request.Trigger != TriggerManual {
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

	if len(credentials) == 0 {
		// Ensure state cache file is initialized on disk even if no credentials exist
		if store, err := state.Load(ctx, cachePath); err == nil {
			_ = store.SaveAtomic(ctx)
		}
		return nil
	}
	credentials, authMaterials, err := enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return err
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return err
	}
	store.ClearExpiredCooldowns(now)

	forceProbe := request.Trigger == TriggerManualApply || request.Trigger == TriggerProbe
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

	// Probe-only: evidence collected and cached, no plan or apply needed (REQ-04).
	if request.Trigger == TriggerProbe {
		r.snapshotRunEntry(apply.Result{}, "probe completed", RunHistoryEntry{
			Kind:    "probe",
			Trigger: string(request.Trigger),
			Message: fmt.Sprintf("probe completed: %d credentials probed", len(evidence)),
		})
		resJSON, _ := json.Marshal(apply.Result{})
		histJSON, _ := json.Marshal(r.currentRunHistory())
		store.SetRuntimeSnapshot("probe completed", resJSON, histJSON)
		_ = store.SaveAtomic(ctx)
		return nil
	}

	plan := priority.PlanFreshOnly(credentials, evidence, priorityOptions(request.Config, store, now))
	plan = withProbeFailureTemporaryDisables(plan, evidence)

	// Build dual-group snapshot (REQ-05): compute predicted plan for the alternate model group.
	primarySnapshot := apply.Snapshot(plan)
	altGroup := alternateModelGroup(request.Config.AntigravityModelGroup)
	altEvidence := buildCachedEvidenceForGroup(store, credentials, string(altGroup))
	altPlan := priority.PlanFreshOnly(credentials, altEvidence, priorityOptions(request.Config, store, now))
	altPlan = withProbeFailureTemporaryDisables(altPlan, altEvidence)
	predictedSnapshot := apply.SnapshotPredicted(altPlan)
	dualSnap := apply.NewDualGroupSnapshot(
		string(request.Config.AntigravityModelGroup), now, primarySnapshot, predictedSnapshot)
	r.setDualSnapshot(dualSnap)

	if request.Trigger == TriggerManual {
		result := apply.Result{Snapshot: primarySnapshot}
		audit := "dry-run plan generated"
		snap := primarySnapshot
		r.snapshotRunEntry(result, audit, RunHistoryEntry{
			Kind:      "dry_run",
			Trigger:   string(request.Trigger),
			Attempted: result.Attempted,
			Succeeded: result.Succeeded,
			Failed:    result.Failed,
			Skipped:   result.Skipped,
			Message:   audit,
			Snapshot:  &snap,
		})
		resJSON, _ := json.Marshal(result)
		histJSON, _ := json.Marshal(r.currentRunHistory())
		store.SetRuntimeSnapshot(audit, resJSON, histJSON)
		_ = store.SaveAtomic(ctx)
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
		Kind:      "apply",
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

func withProbeFailureTemporaryDisables(plan priority.Plan, evidence []priority.ProbeEvidence) priority.Plan {
	failures := make(map[string]priority.ProbeEvidence)
	for _, item := range evidence {
		if item.Status == priority.EvidenceStatusProbeFailed {
			failures[item.AuthIndex] = item
		}
	}
	if len(failures) == 0 {
		return plan
	}

	for index := range plan.Items {
		if _, ok := failures[plan.Items[index].Credential.AuthIndex]; !ok {
			continue
		}
		if plan.Items[index].Credential.Disabled {
			continue
		}
		plan.Items[index].Disabled = true
		plan.Items[index].Reason = "failedQuotaFetch"
	}

	changeIndex := make(map[string]int, len(plan.Changes))
	for index, change := range plan.Changes {
		changeIndex[change.Credential.AuthIndex] = index
	}

	for _, item := range plan.Items {
		if _, ok := failures[item.Credential.AuthIndex]; !ok {
			continue
		}
		if item.Credential.Disabled {
			continue
		}
		if existing, ok := changeIndex[item.Credential.AuthIndex]; ok {
			plan.Changes[existing].Disabled = true
			plan.Changes[existing].EvidenceFresh = true
			if plan.Changes[existing].Reason == "" || plan.Changes[existing].Reason == "keep current state" {
				plan.Changes[existing].Reason = "failedQuotaFetch"
			}
			continue
		}
		plan.Changes = append(plan.Changes, priority.Change{
			Credential:    item.Credential,
			Priority:      item.Priority,
			Disabled:      true,
			EvidenceFresh: true,
			Reason:        "failedQuotaFetch",
		})
	}
	return plan
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
	boostStart := 999
	normalStart := 100
	if cfg.PriorityRules.Enabled {
		if cfg.PriorityRules.BoostStartPriority > 0 {
			boostStart = cfg.PriorityRules.BoostStartPriority
		}
		if cfg.PriorityRules.NormalStartPriority > 0 {
			normalStart = cfg.PriorityRules.NormalStartPriority
		}
	}
	tolerance := 0.05
	var cooldowns map[string]time.Time
	if store != nil {
		if dyn, ok := store.GetDynamicConfig(); ok && dyn.UrgencyTolerance > 0 {
			tolerance = dyn.UrgencyTolerance
		}
		cooldowns = store.GetActiveCooldowns(now)
	}
	return priority.Options{
		Now:                 now,
		BoostStartPriority:  boostStart,
		NormalStartPriority: normalStart,
		MinChange:           cfg.MinChange,
		UrgencyTolerance:    tolerance,
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
