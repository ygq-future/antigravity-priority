package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	credentials, authMaterials, err := enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return err
	}

	store, err := state.Load(ctx, config.DefaultStateCachePath)
	if err != nil {
		return err
	}

	forceProbe := request.Trigger == TriggerManual || request.Trigger == TriggerManualApply
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
	}, request.Trigger)
	if err != nil {
		return err
	}

	if err := store.SaveAtomic(ctx); err != nil {
		return err
	}

	plan := priority.PlanFreshOnly(credentials, evidence, priorityOptions(request.Config, now))
	plan = withProbeFailureTemporaryDisables(plan, evidence)

	if request.Trigger == TriggerManual {
		result := apply.Result{Snapshot: apply.Snapshot(plan)}
		r.snapshotRunEntry(result, "dry-run plan generated", RunHistoryEntry{
			Kind:      "dry_run",
			Trigger:   string(request.Trigger),
			Attempted: result.Attempted,
			Succeeded: result.Succeeded,
			Failed:    result.Failed,
			Skipped:   result.Skipped,
			Message:   "dry-run plan generated",
		})
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
	})
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
		provider := core.Provider(file.Provider)
		credType := core.CredentialType(file.Type)
		if provider == core.ProviderAntigravity || credType == core.CredentialTypeAntigravity {
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

func priorityOptions(cfg config.Config, now time.Time) priority.Options {
	return priority.Options{
		Now:                 now,
		BoostStartPriority:  cfg.PriorityRules.BoostStartPriority,
		NormalStartPriority: cfg.PriorityRules.NormalStartPriority,
		MinChange:           cfg.MinChange,
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
