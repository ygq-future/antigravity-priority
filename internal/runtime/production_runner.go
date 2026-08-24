package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
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

	var preview *quotaPreview
	if request.Trigger == TriggerManualApply && request.PreviewRequired {
		preview, err = r.previewForApply(request)
		if err != nil {
			return err
		}
	}

	var evidence collectedEvidence
	if preview != nil {
		evidence = collectedEvidence{
			RoundID:      preview.ID,
			ByGroup:      cloneEvidenceByGroup(preview.EvidenceByGroup),
			Observations: nil,
		}
	} else {
		forceProbe := request.Trigger == TriggerProbe || request.Trigger == TriggerAutoApply
		evidence, err = r.collectEvidenceForTrigger(ctx, collectInput{
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
	}

	if err := store.SaveAtomic(ctx); err != nil {
		return err
	}

	// Reconcile against the authoritative Host inventory after the potentially
	// slow Google requests. Credentials may have been added, removed, disabled,
	// or reprioritized while probing was in flight.
	projection, err := projectCurrentHost(ctx, client, request, evidence.ByGroup, store, now)
	if err != nil {
		return err
	}
	plan := projection.ControlPlan
	primarySnapshot := projection.ControlSnapshot
	if preview != nil && preview.HostFingerprint != hostFingerprint(primarySnapshot.Items) {
		return fmt.Errorf("quota preview %q is stale: CPA host state changed; refresh quota before applying", preview.ID)
	}

	// Probe-only: evidence collected, dual snapshot updated, no apply executed (REQ-04).
	if request.Trigger == TriggerProbe {
		previewID := evidence.RoundID
		r.setQuotaPreview(quotaPreview{
			ID:              previewID,
			ModelGroup:      request.Config.AntigravityModelGroup,
			AuthScope:       authScopeKey(request.AuthIndexes),
			HostFingerprint: hostFingerprint(primarySnapshot.Items),
			EvidenceByGroup: evidence.ByGroup,
		})
		projection.Snapshot.PreviewID = previewID
		r.setDualSnapshot(projection.Snapshot)
		result := apply.Result{Snapshot: primarySnapshot}
		audit := fmt.Sprintf("probe completed: %d probe observations", evidence.Probed)
		snap := primarySnapshot
		_, projectErr := r.projectRun(ctx, store, result, audit, RunHistoryEntry{
			Kind:         KindProbe,
			Trigger:      string(request.Trigger),
			ProbeRoundID: evidence.RoundID,
			Attempted:    evidence.Probed,
			Succeeded:    len(evidence.ByGroup[request.Config.AntigravityModelGroup].Eligible),
			Message:      audit,
			Snapshot:     &snap,
		})
		return projectErr
	}

	if len(plan.Changes) == 0 {
		projection.Snapshot.PreviewID = ""
		r.clearQuotaPreview()
		r.setDualSnapshot(projection.Snapshot)
		result := apply.Result{Snapshot: primarySnapshot}
		summary := fmt.Sprintf("all %d credentials in sync, no changes required", len(primarySnapshot.Items))
		if request.Trigger != TriggerAutoApply {
			_, projectErr := r.projectSnapshot(ctx, store, result, summary)
			return projectErr
		}
		summary = fmt.Sprintf("auto schedule probed=%d; %s", evidence.Probed, summary)
		_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
			Kind:         KindAutoApply,
			Trigger:      string(request.Trigger),
			ProbeRoundID: evidence.RoundID,
			Message:      summary,
			Snapshot:     &primarySnapshot,
		})
		return projectErr
	}

	transition := apply.NewHostTransition(client)
	result, err := apply.ExecutePlan(ctx, transition, plan, true)
	if err != nil {
		return err
	}
	postApplyProjection, err := projectCurrentHost(ctx, client, request, evidence.ByGroup, store, now)
	if err != nil {
		return fmt.Errorf("reconcile Host after apply: %w", err)
	}
	postApplyProjection.Snapshot.PreviewID = ""
	r.clearQuotaPreview()
	r.setDualSnapshot(postApplyProjection.Snapshot)

	summary := resultSummary("apply", result)
	kind := KindApply
	probeRoundID := ""
	if request.Trigger == TriggerAutoApply {
		kind = KindAutoApply
		probeRoundID = evidence.RoundID
		summary = fmt.Sprintf("auto schedule probed=%d; %s", evidence.Probed, summary)
	}

	_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
		Kind:         kind,
		Trigger:      string(request.Trigger),
		ProbeRoundID: probeRoundID,
		Message:      summary,
		Snapshot:     &primarySnapshot,
	})
	return projectErr
}

func (r *Runtime) previewForApply(request TaskRequest) (*quotaPreview, error) {
	previewID := strings.TrimSpace(request.PreviewID)
	if previewID == "" {
		return nil, errors.New("no pending quota preview; refresh quota before applying")
	}
	preview := r.currentQuotaPreview()
	if preview == nil || preview.ID != previewID {
		return nil, fmt.Errorf("quota preview %q is unavailable; refresh quota before applying", previewID)
	}
	if preview.ModelGroup != request.Config.AntigravityModelGroup {
		return nil, fmt.Errorf("quota preview %q belongs to another control model group; refresh quota before applying", previewID)
	}
	if preview.AuthScope != authScopeKey(request.AuthIndexes) {
		return nil, fmt.Errorf("quota preview %q belongs to another credential scope; refresh quota before applying", previewID)
	}
	if _, ok := preview.EvidenceByGroup[request.Config.AntigravityModelGroup]; !ok {
		return nil, fmt.Errorf("quota preview %q has no evidence for the control model group; refresh quota before applying", previewID)
	}
	return preview, nil
}

func cloneEvidenceByGroup(source map[config.AntigravityModelGroup]evidence.Result) map[config.AntigravityModelGroup]evidence.Result {
	cloned := make(map[config.AntigravityModelGroup]evidence.Result, len(source))
	for group, result := range source {
		cloned[group] = cloneEvidence(result)
	}
	return cloned
}

func authScopeKey(authIndexes []string) string {
	if len(authIndexes) == 0 {
		return "*"
	}
	unique := make(map[string]struct{}, len(authIndexes))
	for _, authIndex := range authIndexes {
		if trimmed := strings.TrimSpace(authIndex); trimmed != "" {
			unique[trimmed] = struct{}{}
		}
	}
	values := make([]string, 0, len(unique))
	for authIndex := range unique {
		values = append(values, authIndex)
	}
	sort.Strings(values)
	return strings.Join(values, ",")
}

func hostFingerprint(items []apply.SnapshotItem) string {
	type hostState struct {
		AuthIndex       string
		Priority        int
		PriorityMissing bool
		Disabled        bool
	}
	states := make([]hostState, 0, len(items))
	for _, item := range items {
		states = append(states, hostState{
			AuthIndex:       item.Identity.AuthIndex,
			Priority:        item.Current.Priority,
			PriorityMissing: item.Current.PriorityMissing,
			Disabled:        item.Current.Disabled,
		})
	}
	sort.Slice(states, func(left, right int) bool {
		return states[left].AuthIndex < states[right].AuthIndex
	})
	hash := sha256.New()
	for _, state := range states {
		_, _ = fmt.Fprintf(hash, "%s\x00%d\x00%t\x00%t\n", state.AuthIndex, state.Priority, state.PriorityMissing, state.Disabled)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func projectCurrentHost(
	ctx context.Context,
	client *host.Client,
	request TaskRequest,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
	store *state.Store,
	now time.Time,
) (DualModelGroupProjection, error) {
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return DualModelGroupProjection{}, err
	}
	credentials := credentialsFromAuthFiles(files)
	credentials = filterCredentialsByAuthIndex(credentials, request.AuthIndexes)
	credentials, _, err = enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return DualModelGroupProjection{}, err
	}
	return ProjectDualModelGroups(ProjectionInput{
		ControlModelGroup: request.Config.AntigravityModelGroup,
		Credentials:       credentials,
		EvidenceByGroup:   evidenceByGroup,
		PlanningOptions:   priorityOptions(request.Config, store, now),
		ProjectionTime:    now,
	})
}

func (r *Runtime) collectEvidenceForTrigger(ctx context.Context, input collectInput, trigger Trigger) (collectedEvidence, error) {
	if trigger != TriggerAutoApply {
		return collectFreshEvidence(ctx, input)
	}
	var collected collectedEvidence
	for attempt := 1; attempt <= autoQuotaProbeAttempts; attempt++ {
		current, err := collectFreshEvidence(ctx, input)
		if err != nil {
			return collectedEvidence{}, err
		}
		collected = current
		if !hasProbeFailure(current, input.modelGroup) || attempt == autoQuotaProbeAttempts {
			return collected, nil
		}
		input.forceProbe = true
		if err := r.sleeper.Sleep(ctx, autoQuotaProbeDelay); err != nil {
			return collectedEvidence{}, err
		}
	}
	return collected, nil
}

func hasProbeFailure(collected collectedEvidence, modelGroup config.AntigravityModelGroup) bool {
	for _, observation := range collected.Observations {
		if observation.ModelGroup == modelGroup && (observation.Kind == evidence.ObservationFailed || observation.Kind == evidence.ObservationInvalid) {
			return true
		}
	}
	return false
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
		IgnoreDisabledHost:  cfg.IgnoreDisabledHost,
		CooldownAuthIndexes: cooldowns,
	}
}

func buildProjectionEvidence(store *state.Store, credentials []core.Credential) map[config.AntigravityModelGroup]evidence.Result {
	historical := historicalObservations(store, credentials)
	result := make(map[config.AntigravityModelGroup]evidence.Result, 2)
	for _, group := range []config.AntigravityModelGroup{
		config.AntigravityModelGroupGemini,
		config.AntigravityModelGroupClaudeGPT,
	} {
		classified := evidence.Classify(evidence.Input{
			Round:       evidence.Round{ID: "historical", ModelGroup: group},
			Credentials: credentials,
			Historical:  historical,
		})
		result[group] = classified
	}
	return result
}
