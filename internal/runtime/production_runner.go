package runtime

import (
	"context"
	"errors"
	"fmt"
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
	if !request.Config.Enabled {
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
	authModels, _ := client.ListAuthModels(ctx)
	r.routingMu.Lock()
	r.updateModelInventoryLocked(files, authModels)
	r.routingMu.Unlock()

	allCredentials := credentialsFromAuthFiles(files)
	allCredentials, allAuthMaterials, err := enrichCredentialsFromAuthDocuments(ctx, client, allCredentials)
	if err != nil {
		return err
	}

	probeCredentials := allCredentials
	probeAuthMaterials := allAuthMaterials
	if len(request.AuthIndexes) > 0 {
		probeCredentials = filterCredentialsByAuthIndex(allCredentials, request.AuthIndexes)
		probeAuthMaterials = filterAuthMaterialsByAuthIndex(allAuthMaterials, request.AuthIndexes)
	}
	cachePath := request.Config.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return err
	}

	var evidence collectedEvidence
	forceProbe := request.Trigger == TriggerProbe || request.Trigger == TriggerAutoApply
	targetedSingleAuth := len(request.AuthIndexes) > 0
	evidence, err = r.collectEvidenceForTrigger(ctx, collectInput{
		client:             client,
		store:              store,
		credentials:        probeCredentials,
		authMaterials:      probeAuthMaterials,
		now:                now,
		cacheTTL:           defaultProbeCacheTTL,
		forceProbe:         forceProbe,
		targetedSingleAuth: targetedSingleAuth,
		maxConcurrency:     request.Config.MaxConcurrency,
		modelGroup:         request.Config.AntigravityModelGroup,
		sampleCapacity:     request.Config.QuotaSampleCapacity,
	}, request.Trigger)
	if err != nil {
		return err
	}
	r.setLastProbeOutcomes(probeOutcomes(evidence.Probes, evidence.ByGroup))

	if len(evidence.RateLimited) > 0 {
		if err := store.SaveStateAtomic(ctx); err != nil {
			return err
		}
		for authIndex, modelGroup := range evidence.RateLimited {
			if err := r.observe429(ctx, authIndex, modelGroup, "", false); err != nil {
				return err
			}
		}
		store, err = state.Load(ctx, cachePath)
		if err != nil {
			return err
		}
	} else {
		if err := store.SaveStateAtomic(ctx); err != nil {
			return err
		}
	}
	if request.Config.Warmup.GeminiEnabled || request.Config.Warmup.ClaudeEnabled {
		disabledAuth := make(map[string]struct{})
		for _, c := range allCredentials {
			if c.Disabled {
				disabledAuth[c.AuthIndex] = struct{}{}
			}
		}
		if r.triggerWarmupForEligibleCredentials(ctx, client, request.Config.Warmup, evidence.ByGroup, allAuthMaterials, disabledAuth, store, now) > 0 {
			if err := store.SaveStateAtomic(ctx); err != nil {
				return fmt.Errorf("persist warm-up state: %w", err)
			}
		}
	}

	// Reconcile against the authoritative Host inventory after the potentially
	// slow Google requests. Credentials may have been added, removed, disabled,
	// or reprioritized while probing was in flight.
	projection, err := projectCurrentHost(ctx, client, request, evidence.ByGroup, store, now)
	if err != nil {
		return err
	}
	primarySnapshot := projection.ControlSnapshot

	// Probe-only: evidence collected and dual snapshot updated; no Host write occurs.
	if request.Trigger == TriggerProbe {
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
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now, allCredentials)
		r.routingMu.Unlock()
		return projectErr
	}

	// AutoApply: background quota probe and calibration only. Never writes back priorities to host files.
	if request.Trigger == TriggerAutoApply {
		r.setDualSnapshot(projection.Snapshot)
		result := apply.Result{Snapshot: primarySnapshot}
		summary := fmt.Sprintf("auto schedule probed=%d; in-memory routing tables calibrated", evidence.Probed)
		_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
			Kind:         KindAutoApply,
			Trigger:      string(request.Trigger),
			ProbeRoundID: evidence.RoundID,
			Message:      summary,
			Snapshot:     &primarySnapshot,
		})
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now, allCredentials)
		r.routingMu.Unlock()
		return projectErr
	}

	// Unknown triggers perform no Host mutation.
	return nil
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
	effectiveEvidence := evidenceByGroup
	if len(request.AuthIndexes) > 0 {
		effectiveEvidence = projectionEffectiveEvidence(store, credentials, evidenceByGroup, request)
	}
	credentials, _, err = enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return DualModelGroupProjection{}, err
	}
	credentials = enrichCredentialsPlanTypeFromStore(store, credentials, string(request.Config.AntigravityModelGroup))
	return ProjectDualModelGroups(ProjectionInput{
		ControlModelGroup: request.Config.AntigravityModelGroup,
		Credentials:       credentials,
		EvidenceByGroup:   effectiveEvidence,
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

func filterAuthMaterialsByAuthIndex(materials map[string]authMaterial, authIndexes []string) map[string]authMaterial {
	if len(authIndexes) == 0 {
		return materials
	}
	allowed := make(map[string]struct{}, len(authIndexes))
	for _, idx := range authIndexes {
		allowed[idx] = struct{}{}
	}
	filtered := make(map[string]authMaterial, len(materials))
	for idx, mat := range materials {
		if _, ok := allowed[idx]; ok {
			filtered[idx] = mat
		}
	}
	return filtered
}

func projectionEffectiveEvidence(
	store *state.Store,
	credentials []core.Credential,
	fresh map[config.AntigravityModelGroup]evidence.Result,
	request TaskRequest,
) map[config.AntigravityModelGroup]evidence.Result {
	if len(request.AuthIndexes) == 0 || store == nil {
		return fresh
	}
	base := buildProjectionEvidence(store, credentials)
	merged := make(map[config.AntigravityModelGroup]evidence.Result, len(config.AllAntigravityModelGroups))
	for _, group := range config.AllAntigravityModelGroups {
		baseRes := base[group]
		freshRes, hasFresh := fresh[group]
		if !hasFresh {
			merged[group] = baseRes
			continue
		}
		freshEligible := make(map[string]priority.QuotaEvidence, len(freshRes.Eligible))
		for _, e := range freshRes.Eligible {
			freshEligible[e.AuthIndex] = e
		}
		freshObs := make(map[string]evidence.Observation, len(freshRes.Observations))
		for _, o := range freshRes.Observations {
			freshObs[o.AuthIndex] = o
		}

		res := evidence.Result{
			Eligible:     make([]priority.QuotaEvidence, 0, len(baseRes.Eligible)+len(freshRes.Eligible)),
			Observations: make([]evidence.Observation, 0, len(baseRes.Observations)+len(freshRes.Observations)),
		}
		res.Eligible = append(res.Eligible, freshRes.Eligible...)
		for _, e := range baseRes.Eligible {
			if _, exists := freshEligible[e.AuthIndex]; !exists {
				res.Eligible = append(res.Eligible, e)
			}
		}
		res.Observations = append(res.Observations, freshRes.Observations...)
		for _, o := range baseRes.Observations {
			if _, exists := freshObs[o.AuthIndex]; !exists {
				res.Observations = append(res.Observations, o)
			}
		}

		merged[group] = res
	}
	return merged
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
	var cooldownReasons map[string]string
	var customTiers map[string]int
	if store != nil {
		cooldowns, cooldownReasons = store.GetActiveCooldownsForPlanning(now)
		customTiers = store.GetCustomTiers()
	}
	if customTiers == nil && cfg.CustomTiers != nil {
		customTiers = cfg.CustomTiers
	}
	return priority.Options{
		Now:                  now,
		BoostStartPriority:   cfg.PriorityRules.BoostStartPriority,
		NormalStartPriority:  cfg.PriorityRules.NormalStartPriority,
		FreeStartPriority:    cfg.PriorityRules.FreeStartPriority,
		MinChange:            cfg.MinChange,
		UrgencyTolerance:     cfg.UrgencyTolerance,
		CooldownAuthIndexes:  cooldowns,
		CooldownReasonsByKey: cooldownReasons,
		CustomTiers:          customTiers,
	}
}
func enrichCredentialsPlanTypeFromStore(store *state.Store, credentials []core.Credential, preferredGroup string) []core.Credential {
	if store == nil {
		return credentials
	}
	for i, cred := range credentials {
		if cred.PlanType == "" || cred.PlanType == core.PlanTypeUnknown {
			if pt := store.GetPlanType(cred.AuthIndex, preferredGroup); pt != "" && pt != core.PlanTypeUnknown {
				credentials[i].PlanType = pt
			}
		}
	}
	return credentials
}

func buildProjectionEvidence(store *state.Store, credentials []core.Credential) map[config.AntigravityModelGroup]evidence.Result {
	historical := historicalObservations(store, credentials)
	result := make(map[config.AntigravityModelGroup]evidence.Result, len(config.AllAntigravityModelGroups))
	for _, group := range config.AllAntigravityModelGroups {
		classified := evidence.Classify(evidence.Input{
			Round:       evidence.Round{ID: "historical", ModelGroup: group},
			Credentials: credentials,
			Historical:  historical,
		})
		result[group] = classified
	}
	return result
}
