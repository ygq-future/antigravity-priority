package runtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strings"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

const (
	rateLimitStormWindow = 30 * time.Second
	rateLimitPendingTTL  = 24 * time.Hour

	serverErrorBaseCooldown = 30 * time.Second
	unauthorizedQuarantine  = 24 * time.Hour
	healthBackoffMax        = 30 * time.Minute
	healthBackoffJitter     = 0.2
)

type pending429 struct {
	AuthIndex  string
	ModelGroup string
	Model      string
	FirstSeen  time.Time
	UpdatedAt  time.Time
	Count      int
}

type failureInput struct {
	AuthIndex    string
	ModelGroup   string
	Model        string
	ProbeModel   string
	Kind         state.FailureKind
	StatusCode   int
	Reason       string
	Cooldown     time.Duration
	ProbeEnabled bool
}

func (r *Runtime) observe429(ctx context.Context, authIndex, modelGroup, model string, acquireRunLock bool) error {
	now := r.clock.Now().UTC()
	key := state.CooldownKey(authIndex, modelGroup)
	r.rateLimitMu.Lock()
	if _, active := r.activeCooldowns[key]; active {
		r.rateLimitMu.Unlock()
		return nil
	}
	if _, active := r.activeCooldowns[authIndex]; active {
		r.rateLimitMu.Unlock()
		return nil
	}
	if now.Before(r.stormUntil) {
		r.stormAccounts[key] = false
		r.rateLimitMu.Unlock()
		return nil
	}
	if pending, ok := r.pending429[key]; ok && pending.Count >= 1 {
		delete(r.pending429, key)
		r.rateLimitMu.Unlock()
		r.signalRateLimitWorker()
		if acquireRunLock {
			r.runMu.Lock()
		}
		err := r.triggerCooldown(ctx, pending.AuthIndex, pending.ModelGroup, firstNonEmpty(pending.Model, model))
		if acquireRunLock {
			r.runMu.Unlock()
		}
		return err
	}
	r.pending429[key] = pending429{AuthIndex: authIndex, ModelGroup: modelGroup, Model: model, FirstSeen: now, UpdatedAt: now, Count: 1}
	pendingCount := len(r.pending429)
	r.rateLimitMu.Unlock()

	active := r.activeAntigravityCredentialCount(ctx)
	if pendingCount >= 3 && active > 0 && pendingCount*2 >= active {
		r.rateLimitMu.Lock()
		if len(r.pending429) >= 3 && len(r.pending429)*2 >= active {
			r.stormAccounts = make(map[string]bool, len(r.pending429))
			for authIndex := range r.pending429 {
				r.stormAccounts[authIndex] = false
			}
			r.pending429 = make(map[string]pending429)
			r.stormUntil = now.Add(rateLimitStormWindow)
		}
		r.rateLimitMu.Unlock()
	}
	r.signalRateLimitWorker()
	return nil
}

func (r *Runtime) observeFailure(ctx context.Context, input failureInput, acquireRunLock bool) error {
	if input.Cooldown <= 0 {
		cfg, err := r.Config()
		if err != nil {
			return err
		}
		input.Cooldown = failureBaseCooldown(input.Kind, cfg)
	}
	key := state.FailureKey(input.AuthIndex, input.ModelGroup, input.Model)
	r.rateLimitMu.Lock()
	if _, active := r.activeCooldowns[key]; active {
		r.rateLimitMu.Unlock()
		return nil
	}
	if input.Kind != state.FailureUnauthorized && input.Kind != state.FailureForbidden {
		if _, active := r.activeCooldowns[input.AuthIndex]; active {
			r.rateLimitMu.Unlock()
			return nil
		}
	}
	r.rateLimitMu.Unlock()
	if acquireRunLock {
		r.runMu.Lock()
		defer r.runMu.Unlock()
	}
	return r.triggerFailure(ctx, input)
}

func (r *Runtime) triggerFailure(ctx context.Context, input failureInput) error {
	now := r.clock.Now().UTC()
	cfg, err := r.Config()
	if err != nil {
		return err
	}
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		r.recordCooldownFailure(ctx, nil, input.AuthIndex, input.Reason, failureTrigger(input.Kind), err)
		return fmt.Errorf("record health state: %w", err)
	}
	entries := store.GetCooldowns()
	key := state.FailureKey(input.AuthIndex, input.ModelGroup, input.Model)
	if existing, active := entries[key]; active && now.Before(existing.CooldownUntil) {
		return nil
	}
	if input.Kind != state.FailureUnauthorized && input.Kind != state.FailureForbidden {
		if existing, active := entries[state.CooldownKey(input.AuthIndex, "")]; active && now.Before(existing.CooldownUntil) {
			return nil
		}
	}

	credential, findErr := r.findCredential(ctx, input.AuthIndex)
	if findErr != nil {
		r.recordCooldownFailure(ctx, store, input.AuthIndex, input.Reason, failureTrigger(input.Kind), findErr)
		return findErr
	}
	if credential.Disabled {
		return nil
	}

	previousPriority := credential.Priority
	previousPriorityMissing := credential.PriorityMissing
	previousDisabled := credential.Disabled
	baselineFound := false
	for _, existing := range entries {
		if existing.AuthIndex != input.AuthIndex {
			continue
		}
		if baselineFound && existing.PreviousPriority == priority.DepletedPriority {
			continue
		}
		previousPriority = existing.PreviousPriority
		previousPriorityMissing = existing.PreviousPriorityMissing
		previousDisabled = existing.PreviousDisabled
		baselineFound = true
	}

	probeEnabled := input.ProbeEnabled
	if input.Kind == state.FailureUnauthorized || input.Kind == state.FailureForbidden {
		probeEnabled = false
	}
	entry := state.CooldownEntry{
		AuthIndex:               input.AuthIndex,
		ModelGroup:              input.ModelGroup,
		Model:                   input.Model,
		ProbeModel:              input.ProbeModel,
		FailureKind:             input.Kind,
		StatusCode:              input.StatusCode,
		TriggeredAt:             now,
		CooldownUntil:           now.Add(input.Cooldown),
		NextRecoveryAt:          now.Add(input.Cooldown),
		ProbeEnabled:            probeEnabled,
		Reason:                  input.Reason,
		PreviousPriority:        previousPriority,
		PreviousPriorityMissing: previousPriorityMissing,
		PreviousDisabled:        previousDisabled,
		AppliedDisabled:         credential.Disabled,
	}
	if !probeEnabled {
		entry.NextRecoveryAt = time.Time{}
	}
	store.SetCooldown(entry)
	r.setActiveCooldown(entry)

	if r.hostCallbacks == nil {
		err := errMissingHostCallbacks
		r.recordCooldownFailure(ctx, store, input.AuthIndex, input.Reason, failureTrigger(input.Kind), err)
		return err
	}
	client := host.NewClient(r.hostCallbacks)
	result, applyErr := apply.ExecuteRound(ctx, apply.NewHostTransition(client), apply.TransitionRound{
		Intents: []apply.TransitionIntent{apply.CooldownIntent(credential, input.Reason)},
	})
	result.Snapshot = apply.Snapshot(priority.Plan{
		DecidedAt: now,
		Items: []priority.PlanItem{{
			Credential: credential,
			Priority:   priority.DepletedPriority,
			Disabled:   false,
			Reason:     input.Reason,
		}},
	})
	if applyErr != nil {
		r.recordCooldownFailure(ctx, store, input.AuthIndex, input.Reason, failureTrigger(input.Kind), applyErr)
		return applyErr
	}
	r.patchSnapshotCooldown(input.AuthIndex, true, input.Reason, priority.DepletedPriority, credential.Disabled)
	summary := resultSummary("health cooldown", result)
	_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
		Kind:     KindCooldown,
		Trigger:  failureTrigger(input.Kind),
		Message:  summary,
		Snapshot: &result.Snapshot,
	})
	if projectErr != nil {
		return projectErr
	}
	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()
	r.signalRateLimitWorker()
	if result.Transitions.Totals.Failed > 0 || result.Transitions.Totals.Conflicts > 0 || result.Transitions.Totals.Uncertain > 0 {
		return errors.New("health cooldown host mutation failed")
	}
	return nil
}

func (r *Runtime) observeUsageSuccess(ctx context.Context, authIndex, modelGroup, model string, detail usageDetail) error {
	now := r.clock.Now().UTC()
	cfg, cfgErr := r.Config()
	if cfgErr == nil {
		cachePath := cfg.StateCachePath
		if strings.TrimSpace(cachePath) == "" {
			cachePath = config.DefaultStateCachePath
		}
		if store, loadErr := state.Load(ctx, cachePath); loadErr == nil {
			weightedTokens := state.CalculateWeightedTurnTokens(
				detail.InputTokens,
				detail.OutputTokens,
				detail.ReasoningTokens,
				detail.CacheReadTokens,
				detail.TotalTokens,
			)
			if weightedTokens > 0 {
				store.RecordUsageConsumption(authIndex, modelGroup, weightedTokens, now)
				_ = store.SaveAtomic(ctx)
			}
		}
	}

	key := state.CooldownKey(authIndex, modelGroup)
	r.rateLimitMu.Lock()
	delete(r.pending429, key)
	delete(r.pending429, authIndex)
	if !r.stormUntil.IsZero() && now.Before(r.stormUntil) {
		if _, participant := r.stormAccounts[key]; participant {
			r.stormAccounts[key] = true
		}
		if _, participant := r.stormAccounts[authIndex]; participant {
			r.stormAccounts[authIndex] = true
		}
		recovered := 0
		for _, ok := range r.stormAccounts {
			if ok {
				recovered++
			}
		}
		if len(r.stormAccounts) > 0 && recovered*2 >= len(r.stormAccounts) {
			r.stormUntil = time.Time{}
			r.stormAccounts = make(map[string]bool)
		}
	}
	r.rateLimitMu.Unlock()
	r.signalRateLimitWorker()

	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()

	if cfgErr != nil {
		c, cErr := r.Config()
		if cErr != nil {
			return cErr
		}
		cfg = c
	}
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return err
	}

	entries := store.GetCooldowns()
	cleared := make([]state.CooldownEntry, 0, 3)
	seen := make(map[string]struct{}, 3)
	for _, candidate := range []string{
		state.FailureKey(authIndex, modelGroup, model),
		state.CooldownKey(authIndex, modelGroup),
		state.CooldownKey(authIndex, ""),
	} {
		if candidate == "" {
			continue
		}
		if _, duplicate := seen[candidate]; duplicate {
			continue
		}
		seen[candidate] = struct{}{}
		entry, active := entries[candidate]
		if !active || !now.Before(entry.CooldownUntil) {
			continue
		}
		store.DeleteCooldownExact(candidate)
		r.clearActiveCooldownExact(candidate)
		cleared = append(cleared, entry)
	}
	if len(cleared) == 0 {
		return nil
	}
	if hasActiveFailureForAuth(store, authIndex, now) {
		if err := store.SaveAtomic(ctx); err != nil {
			return err
		}
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		return nil
	}
	r.runMu.Lock()
	err = r.applyCooldownRecovery(ctx, store, cleared[0], "recovered by successful request", "success_recovery")
	r.runMu.Unlock()
	return err
}

func (r *Runtime) activeAntigravityCredentialCount(ctx context.Context) int {
	if r.hostCallbacks == nil {
		return 0
	}
	files, err := host.NewClient(r.hostCallbacks).ListAuthFiles(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, credential := range credentialsFromAuthFiles(files) {
		if credential.Provider == core.ProviderAntigravity && !credential.Disabled {
			count++
		}
	}
	return count
}

func (r *Runtime) signalRateLimitWorker() {
	select {
	case r.rateLimitWake <- struct{}{}:
	default:
	}
}

func (r *Runtime) runRateLimitWorker() {
	defer close(r.rateLimitDone)
	for {
		next := r.nextRateLimitDeadline()
		var timer *time.Timer
		var timerC <-chan time.Time
		if !next.IsZero() {
			delay := max(next.Sub(r.clock.Now().UTC()), 0)
			timer = time.NewTimer(delay)
			timerC = timer.C
		}
		select {
		case <-r.rootCtx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-r.rateLimitWake:
			if timer != nil {
				timer.Stop()
			}
			r.processRateLimitDeadlines(r.rootCtx)
		case <-timerC:
			r.processRateLimitDeadlines(r.rootCtx)
		}
	}
}

func (r *Runtime) nextRateLimitDeadline() time.Time {
	r.rateLimitMu.Lock()
	defer r.rateLimitMu.Unlock()
	var next time.Time
	if !r.stormUntil.IsZero() {
		next = earlierTime(next, r.stormUntil)
	}
	for _, entry := range r.activeCooldowns {
		deadline := entry.CooldownUntil
		if !entry.NextRecoveryAt.IsZero() && entry.NextRecoveryAt.After(deadline) {
			deadline = entry.NextRecoveryAt
		}
		next = earlierTime(next, deadline)
	}
	return next
}

func earlierTime(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

func (r *Runtime) processRateLimitDeadlines(ctx context.Context) {
	now := r.clock.Now().UTC()
	r.rateLimitMu.Lock()
	if !r.stormUntil.IsZero() && !now.Before(r.stormUntil) {
		r.stormUntil = time.Time{}
		r.stormAccounts = make(map[string]bool)
	}
	for key, pending := range r.pending429 {
		if now.Sub(pending.UpdatedAt) > rateLimitPendingTTL {
			delete(r.pending429, key)
		}
	}
	dueCooldowns := make([]state.CooldownEntry, 0)
	for _, entry := range r.activeCooldowns {
		if !now.Before(entry.CooldownUntil) && (entry.NextRecoveryAt.IsZero() || !now.Before(entry.NextRecoveryAt)) {
			dueCooldowns = append(dueCooldowns, entry)
		}
	}
	r.rateLimitMu.Unlock()
	if len(dueCooldowns) == 0 {
		return
	}
	cfg, err := r.Config()
	if err != nil {
		r.deferCooldownRecovery(dueCooldowns, now)
		return
	}
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		r.deferCooldownRecovery(dueCooldowns, now)
		return
	}
	persisted := store.GetCooldowns()
	for _, scheduled := range dueCooldowns {
		key := state.FailureKey(scheduled.AuthIndex, scheduled.ModelGroup, scheduled.Model)
		entry, active := persisted[key]
		if !active {
			r.clearActiveCooldownExact(key)
			continue
		}
		if !entry.ProbeEnabled {
			r.runMu.Lock()
			restoreErr := r.restoreCooldown(ctx, store, entry, "health quarantine expired", "health_expiry")
			r.runMu.Unlock()
			if restoreErr != nil {
				r.rescheduleAfterRestoreFailure(ctx, store, entry, now)
			}
			continue
		}

		status, probeErr := r.probeHealth(ctx, entry, cfg)
		r.runMu.Lock()
		latestStore, loadErr := state.Load(ctx, cachePath)
		if loadErr != nil {
			r.runMu.Unlock()
			r.rescheduleAfterRestoreFailure(ctx, store, entry, now)
			continue
		}
		latest, active := latestStore.GetCooldowns()[key]
		if !active {
			r.clearActiveCooldownExact(key)
			r.runMu.Unlock()
			continue
		}
		if probeErr == nil && status == http.StatusOK {
			restoreErr := r.restoreCooldown(ctx, latestStore, latest, "health probe succeeded", failureTrigger(latest.FailureKind))
			r.runMu.Unlock()
			if restoreErr != nil {
				r.rescheduleAfterRestoreFailure(ctx, latestStore, latest, now)
			}
			continue
		}
		r.rescheduleFailure(ctx, latestStore, latest, status, probeErr, cfg, now)
		r.runMu.Unlock()
	}
}

func (r *Runtime) deferCooldownRecovery(entries []state.CooldownEntry, now time.Time) {
	for _, entry := range entries {
		entry.NextRecoveryAt = now.Add(30 * time.Second)
		entry.CooldownUntil = entry.NextRecoveryAt
		r.setActiveCooldown(entry)
	}
}

func (r *Runtime) rescheduleAfterRestoreFailure(ctx context.Context, store *state.Store, entry state.CooldownEntry, now time.Time) {
	entry.NextRecoveryAt = now.Add(30 * time.Second)
	entry.CooldownUntil = entry.NextRecoveryAt
	store.SetCooldown(entry)
	r.setActiveCooldown(entry)
	_ = store.SaveAtomic(ctx)
	r.signalRateLimitWorker()
}

func (r *Runtime) rescheduleFailure(ctx context.Context, store *state.Store, entry state.CooldownEntry, status int, probeErr error, cfg config.Config, now time.Time) {
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		r.escalateAuthProblem(ctx, store, entry, status, now)
		return
	}
	switch {
	case status == http.StatusTooManyRequests:
		entry.FailureKind = state.FailureRateLimit
		entry.Reason = priority.Reason429Cooldown
	case status >= 500 && status <= 599:
		entry.FailureKind = state.FailureServerError
		entry.Reason = fmt.Sprintf("%d server error cooldown", status)
	case status == 0:
		entry.FailureKind = state.FailureServerError
		entry.Reason = "health probe transport error"
		if probeErr != nil {
			entry.Reason += ": " + host.RedactBytes([]byte(probeErr.Error()))
		}
	default:
		entry.FailureKind = state.FailureServerError
		entry.Reason = fmt.Sprintf("health probe HTTP %d", status)
	}
	entry.StatusCode = status
	entry.BackoffStep++
	entry.CooldownUntil = now.Add(backoffDuration(failureBaseCooldown(entry.FailureKind, cfg), entry.BackoffStep))
	entry.NextRecoveryAt = entry.CooldownUntil
	entry.ProbeEnabled = true
	store.SetCooldown(entry)
	r.setActiveCooldown(entry)
	_ = store.SaveAtomic(ctx)
	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()
	r.signalRateLimitWorker()
}

func (r *Runtime) escalateAuthProblem(ctx context.Context, store *state.Store, entry state.CooldownEntry, status int, now time.Time) {
	key := state.FailureKey(entry.AuthIndex, entry.ModelGroup, entry.Model)
	store.DeleteCooldownExact(key)
	r.clearActiveCooldownExact(key)

	accountKey := state.CooldownKey(entry.AuthIndex, "")
	if existing, active := store.GetCooldowns()[accountKey]; active && now.Before(existing.CooldownUntil) {
		_ = store.SaveAtomic(ctx)
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		return
	}
	kind := state.FailureUnauthorized
	reason := priority.ReasonUnauthorizedCooldown
	if status == http.StatusForbidden {
		kind = state.FailureForbidden
		reason = priority.ReasonForbiddenCooldown
	}
	accountEntry := state.CooldownEntry{
		AuthIndex:               entry.AuthIndex,
		FailureKind:             kind,
		StatusCode:              status,
		TriggeredAt:             now,
		CooldownUntil:           now.Add(unauthorizedQuarantine),
		Reason:                  reason,
		PreviousPriority:        entry.PreviousPriority,
		PreviousPriorityMissing: entry.PreviousPriorityMissing,
		PreviousDisabled:        entry.PreviousDisabled,
		AppliedDisabled:         entry.AppliedDisabled,
		ProbeEnabled:            false,
	}
	store.SetCooldown(accountEntry)
	r.setActiveCooldown(accountEntry)
	_ = store.SaveAtomic(ctx)
	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()
	r.signalRateLimitWorker()
}

func (r *Runtime) probeHealth(ctx context.Context, entry state.CooldownEntry, cfg config.Config) (int, error) {
	if r.hostCallbacks == nil {
		return 0, errMissingHostCallbacks
	}
	credential, err := r.findCredential(ctx, entry.AuthIndex)
	if err != nil {
		return 0, err
	}
	model := strings.TrimSpace(entry.ProbeModel)
	if model == "" {
		model = strings.TrimSpace(entry.Model)
	}
	if model == "" {
		model = defaultProbeModel(cfg, entry.ModelGroup)
	}
	cfg.Warmup.Normalize()
	client := host.NewClient(r.hostCallbacks)
	result, probeErr := executeAntigravityWarmup(
		ctx,
		client,
		entry.AuthIndex,
		accessTokenFromJSON(credential.RawJSON),
		projectIDFromJSON(credential.RawJSON),
		model,
		RenderWarmupPrompt(cfg.Warmup, ""),
	)
	status := result.StatusCode
	now := r.clock.Now().UTC()
	errorMessage := ""
	if probeErr != nil {
		errorMessage = probeErr.Error()
	} else if status != http.StatusOK {
		errorMessage = fmt.Sprintf("health probe HTTP %d", status)
	}
	email := redactRuntimeIdentifier(credential.Email)
	if email == "" {
		email = redactRuntimeIdentifier(entry.AuthIndex)
	}
	r.appendRequestHistory(state.RequestHistoryEntry{
		ID:           fmt.Sprintf("health-%s-%d", entry.AuthIndex, now.UnixNano()),
		At:           now,
		Model:        model,
		ModelPool:    entry.ModelGroup,
		AuthIndex:    entry.AuthIndex,
		Email:        email,
		Failed:       status != http.StatusOK || probeErr != nil,
		StatusCode:   status,
		ErrorMessage: host.RedactBytes([]byte(errorMessage)),
	})
	log.Printf("[antigravity-priority] health probe: auth=%s pool=%s model=%s kind=%s status=%d err=%v",
		redactRuntimeIdentifier(entry.AuthIndex), entry.ModelGroup, model, entry.FailureKind, status, probeErr)
	return status, probeErr
}

func defaultProbeModel(cfg config.Config, modelGroup string) string {
	cfg.Warmup.Normalize()
	switch config.AntigravityModelGroup(strings.ToLower(strings.TrimSpace(modelGroup))) {
	case config.AntigravityModelGroupClaudeGPT:
		return cfg.Warmup.ClaudeModel
	case config.AntigravityModelGroupArgon:
		return "gemini-4-argon-high"
	default:
		return cfg.Warmup.GeminiModel
	}
}

func failureBaseCooldown(kind state.FailureKind, cfg config.Config) time.Duration {
	switch kind {
	case state.FailureRateLimit:
		minutes := cfg.RateLimitCooldownMinutes
		if minutes <= 0 {
			minutes = config.DefaultRateLimitCooldownMinutes
		}
		return time.Duration(minutes) * time.Minute
	default:
		return serverErrorBaseCooldown
	}
}

func backoffDuration(base time.Duration, step int) time.Duration {
	if base <= 0 {
		base = serverErrorBaseCooldown
	}
	delay := base
	for i := 0; i < step && delay < healthBackoffMax; i++ {
		delay *= 2
	}
	if delay > healthBackoffMax {
		delay = healthBackoffMax
	}
	jitter := 1 + (rand.Float64()*2-1)*healthBackoffJitter
	return time.Duration(float64(delay) * jitter)
}

func failureTrigger(kind state.FailureKind) string {
	switch kind {
	case state.FailureRateLimit:
		return "rate_limit_429"
	case state.FailureServerError:
		return "server_error_5xx"
	case state.FailureForbidden:
		return "forbidden_403"
	case state.FailureUnauthorized:
		return "unauthorized_401"
	default:
		return "health_cooldown"
	}
}

func (r *Runtime) restoreCooldown(ctx context.Context, store *state.Store, entry state.CooldownEntry, cause, trigger string) error {
	key := state.FailureKey(entry.AuthIndex, entry.ModelGroup, entry.Model)
	credential, err := r.findCredential(ctx, entry.AuthIndex)
	if err != nil {
		return err
	}
	now := r.clock.Now().UTC()
	manualChange := credential.Priority != priority.DepletedPriority || credential.Disabled != entry.AppliedDisabled
	store.DeleteCooldownExact(key)
	r.clearActiveCooldownExact(key)
	if manualChange {
		_ = store.SaveAtomic(ctx)
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		return nil
	}
	if hasActiveFailureForAuth(store, entry.AuthIndex, now) {
		_ = store.SaveAtomic(ctx)
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		return nil
	}
	return r.applyCooldownRecovery(ctx, store, entry, cause, trigger)
}

func (r *Runtime) applyCooldownRecovery(ctx context.Context, store *state.Store, entry state.CooldownEntry, cause, trigger string) error {
	credential, err := r.findCredential(ctx, entry.AuthIndex)
	if err != nil {
		return err
	}
	if credential.Priority != priority.DepletedPriority || credential.Disabled != entry.AppliedDisabled {
		_ = store.SaveAtomic(ctx)
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(r.clock.Now().UTC())
		r.routingMu.Unlock()
		return nil
	}
	client := host.NewClient(r.hostCallbacks)
	intent := apply.CooldownRecoveryIntent(credential, entry.PreviousPriority, entry.PreviousPriorityMissing, entry.PreviousDisabled, cause)
	result, applyErr := apply.ExecuteRound(ctx, apply.NewHostTransition(client), apply.TransitionRound{Intents: []apply.TransitionIntent{intent}})
	if applyErr != nil {
		return applyErr
	}
	reason := entry.Reason
	if strings.TrimSpace(reason) == "" {
		reason = priority.Reason429Cooldown
	}
	r.patchSnapshotCooldown(entry.AuthIndex, false, "cooldown recovered", entry.PreviousPriority, entry.PreviousDisabled)
	result.Snapshot = apply.Snapshot(priority.Plan{DecidedAt: r.clock.Now().UTC(), Items: []priority.PlanItem{{Credential: credential, Priority: entry.PreviousPriority, Disabled: entry.PreviousDisabled, Reason: reason}}})
	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(r.clock.Now().UTC())
	r.routingMu.Unlock()
	summary := resultSummary("cooldown recovery", result)
	_, err = r.projectRun(ctx, store, result, summary, RunHistoryEntry{Kind: KindCooldownRecovery, Trigger: trigger, Message: summary, Snapshot: &result.Snapshot})
	return err
}

func hasActiveFailureForAuth(store *state.Store, authIndex string, now time.Time) bool {
	for _, entry := range store.GetCooldowns() {
		if entry.AuthIndex == authIndex && now.Before(entry.CooldownUntil) {
			return true
		}
	}
	return false
}

func (r *Runtime) clearActiveCooldownExact(key string) {
	r.rateLimitMu.Lock()
	delete(r.activeCooldowns, key)
	r.rateLimitMu.Unlock()
}

func (r *Runtime) setActiveCooldown(entry state.CooldownEntry) {
	r.rateLimitMu.Lock()
	key := state.FailureKey(entry.AuthIndex, entry.ModelGroup, entry.Model)
	r.activeCooldowns[key] = entry
	r.rateLimitMu.Unlock()
}

func (r *Runtime) findCredential(ctx context.Context, authIndex string) (core.Credential, error) {
	if r.hostCallbacks == nil {
		return core.Credential{}, errMissingHostCallbacks
	}
	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return core.Credential{}, err
	}
	for _, candidate := range credentialsFromAuthFiles(files) {
		if candidate.AuthIndex != authIndex && candidate.Name != authIndex && candidate.Email != authIndex {
			continue
		}
		enriched, _, enrichErr := enrichCredentialsFromAuthDocuments(ctx, client, []core.Credential{candidate})
		if enrichErr != nil {
			return core.Credential{}, enrichErr
		}
		if len(enriched) == 1 {
			return enriched[0], nil
		}
		return candidate, nil
	}
	return core.Credential{}, errors.New("cooldown credential not found in Host inventory")
}

func (r *Runtime) rateLimitDiagnostics() (pending []map[string]any, storm map[string]any) {
	r.rateLimitMu.Lock()
	defer r.rateLimitMu.Unlock()
	for _, item := range r.pending429 {
		pending = append(pending, map[string]any{
			"auth_index":    redactRuntimeIdentifier(item.AuthIndex),
			"model_group":   item.ModelGroup,
			"model":         item.Model,
			"first_seen_at": item.FirstSeen,
			"updated_at":    item.UpdatedAt,
			"count":         item.Count,
		})
	}
	if !r.stormUntil.IsZero() {
		recovered := 0
		for _, ok := range r.stormAccounts {
			if ok {
				recovered++
			}
		}
		storm = map[string]any{"active": r.clock.Now().UTC().Before(r.stormUntil), "until": r.stormUntil, "credentials": len(r.stormAccounts), "recovered": recovered}
	}
	return pending, storm
}
