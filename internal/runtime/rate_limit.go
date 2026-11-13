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
	cooldowns := r.state.Cooldowns()
	if _, active := cooldowns[key]; active {
		return nil
	}
	if _, active := cooldowns[authIndex]; active {
		return nil
	}
	r.rateLimitMu.Lock()
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
	cooldowns := r.state.Cooldowns()
	if _, active := cooldowns[key]; active {
		return nil
	}
	if input.Kind != state.FailureUnauthorized && input.Kind != state.FailureForbidden {
		if _, active := cooldowns[input.AuthIndex]; active {
			return nil
		}
	}
	if acquireRunLock {
		r.runMu.Lock()
		defer r.runMu.Unlock()
	}
	return r.triggerFailure(ctx, input)
}

func (r *Runtime) triggerFailure(ctx context.Context, input failureInput) error {
	now := r.clock.Now().UTC()
	store, err := r.state.Store(ctx)
	if err != nil {
		r.recordCooldownFailure(ctx, false, input.AuthIndex, input.Reason, failureTrigger(input.Kind), err)
		return fmt.Errorf("record health state: %w", err)
	}
	key := state.FailureKey(input.AuthIndex, input.ModelGroup, input.Model)
	if existing, active := store.GetCooldown(key); active && now.Before(existing.CooldownUntil) {
		return nil
	}
	if input.Kind != state.FailureUnauthorized && input.Kind != state.FailureForbidden {
		if existing, active := store.GetCooldown(state.CooldownKey(input.AuthIndex, "")); active && now.Before(existing.CooldownUntil) {
			return nil
		}
	}

	credential, allCreds, found := r.findCredentialSummary(ctx, input.AuthIndex)
	if found && credential.Disabled {
		return nil
	}
	if !found {
		credential = core.Credential{
			AuthIndex: input.AuthIndex,
			Name:      input.AuthIndex,
			Provider:  core.ProviderAntigravity,
			Status:    core.CredentialStatusActive,
		}
	}

	probeEnabled := input.ProbeEnabled
	if input.Kind == state.FailureUnauthorized || input.Kind == state.FailureForbidden {
		probeEnabled = false
	}
	entry := state.CooldownEntry{
		AuthIndex:      input.AuthIndex,
		ModelGroup:     input.ModelGroup,
		Model:          input.Model,
		ProbeModel:     input.ProbeModel,
		FailureKind:    input.Kind,
		StatusCode:     input.StatusCode,
		TriggeredAt:    now,
		CooldownUntil:  now.Add(input.Cooldown),
		NextRecoveryAt: now.Add(input.Cooldown),
		ProbeEnabled:   probeEnabled,
		Reason:         input.Reason,
	}
	if !probeEnabled {
		entry.NextRecoveryAt = time.Time{}
	}
	store.SetCooldown(entry)
	if err := store.SaveStateAtomic(ctx); err != nil {
		r.recordCooldownFailure(ctx, true, input.AuthIndex, input.Reason, failureTrigger(input.Kind), err)
		return fmt.Errorf("persist health cooldown: %w", err)
	}

	var candidatesByPool map[string]PrecomputedCandidate
	if len(allCreds) > 0 {
		candidatesByPool, _, _ = r.recomputeAndCaptureRouting(now, input.AuthIndex, allCreds)
	} else {
		candidatesByPool, _, _ = r.recomputeAndCaptureRouting(now, input.AuthIndex)
	}
	r.syncSnapshotCooldown(input.AuthIndex, candidatesByPool)

	snapshot := apply.Snapshot(priority.Plan{
		DecidedAt: now,
		Items: []priority.PlanItem{{
			Credential:    credential,
			Priority:      priority.DepletedPriority,
			Disabled:      false,
			CooldownUntil: &entry.CooldownUntil,
			Reason:        input.Reason,
		}},
	})
	summary := healthIsolationSummary("health cooldown", input.AuthIndex, input.ModelGroup, input.Reason)
	result := apply.Result{
		Snapshot:  snapshot,
		Record:    apply.RecordResult{Status: apply.RecordNotAttempted},
		Attempted: 1,
		Succeeded: 1,
	}
	_, projectErr := r.state.ProjectRun(ctx, result, summary, withTransitionStats(result, RunHistoryEntry{
		Kind:     KindCooldown,
		Trigger:  failureTrigger(input.Kind),
		Message:  summary,
		Snapshot: &result.Snapshot,
	}))
	if projectErr != nil {
		return projectErr
	}
	r.signalRateLimitWorker()
	return nil
}

func (r *Runtime) observeUsageSuccess(ctx context.Context, authIndex, modelGroup, model string, detail usageDetail) error {
	now := r.clock.Now().UTC()
	store, storeErr := r.state.Store(ctx)
	stateDirty := false
	if storeErr == nil {
		weightedTokens := state.CalculateWeightedTurnTokens(
			detail.InputTokens,
			detail.OutputTokens,
			detail.ReasoningTokens,
			detail.CacheReadTokens,
			detail.TotalTokens,
		)
		if weightedTokens > 0 {
			store.RecordUsageConsumption(authIndex, modelGroup, weightedTokens, now)
			stateDirty = true
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

	if storeErr != nil {
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		return storeErr
	}

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
		entry, active := store.GetCooldown(candidate)
		if !active || !now.Before(entry.CooldownUntil) {
			continue
		}
		store.DeleteCooldownExact(candidate)
		cleared = append(cleared, entry)
		stateDirty = true
	}
	if len(cleared) == 0 {
		if stateDirty {
			_ = store.SaveStateAtomic(ctx)
		}
		r.routingMu.Lock()
		r.recomputeRoutingTablesLocked(now)
		r.routingMu.Unlock()
		r.signalRateLimitWorker()
		return nil
	}
	r.runMu.Lock()
	err := r.applyCooldownRecovery(ctx, store, cleared[0], "recovered by successful request", "success_recovery")
	r.runMu.Unlock()
	r.signalRateLimitWorker()
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
	cooldowns := r.state.Cooldowns()
	r.rateLimitMu.Lock()
	defer r.rateLimitMu.Unlock()
	var next time.Time
	if !r.stormUntil.IsZero() {
		next = earlierTime(next, r.stormUntil)
	}
	for _, entry := range cooldowns {
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
	cooldowns := r.state.Cooldowns()
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
	r.rateLimitMu.Unlock()

	dueCooldowns := make([]state.CooldownEntry, 0)
	for _, entry := range cooldowns {
		if !now.Before(entry.CooldownUntil) && (entry.NextRecoveryAt.IsZero() || !now.Before(entry.NextRecoveryAt)) {
			dueCooldowns = append(dueCooldowns, entry)
		}
	}
	if len(dueCooldowns) == 0 {
		return
	}
	cfg, err := r.Config()
	if err != nil {
		r.deferCooldownRecovery(dueCooldowns, now)
		return
	}
	store, err := r.state.Store(ctx)
	if err != nil {
		r.deferCooldownRecovery(dueCooldowns, now)
		return
	}
	for _, scheduled := range dueCooldowns {
		key := state.FailureKey(scheduled.AuthIndex, scheduled.ModelGroup, scheduled.Model)
		entry, active := store.GetCooldown(key)
		if !active {
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
		latest, active := store.GetCooldown(key)
		if !active {
			r.runMu.Unlock()
			continue
		}
		if probeErr == nil && status == http.StatusOK {
			restoreErr := r.restoreCooldown(ctx, store, latest, "health probe succeeded", failureTrigger(latest.FailureKind))
			r.runMu.Unlock()
			if restoreErr != nil {
				r.rescheduleAfterRestoreFailure(ctx, store, latest, now)
			}
			continue
		}
		r.rescheduleFailure(ctx, store, latest, status, probeErr, cfg, now)
		r.runMu.Unlock()
	}
}

func (r *Runtime) deferCooldownRecovery(entries []state.CooldownEntry, now time.Time) {
	store := r.state.StoreOrNil()
	if store == nil {
		return
	}
	for _, entry := range entries {
		entry.NextRecoveryAt = now.Add(30 * time.Second)
		entry.CooldownUntil = entry.NextRecoveryAt
		store.SetCooldown(entry)
	}
}

func (r *Runtime) rescheduleAfterRestoreFailure(ctx context.Context, store *state.Store, entry state.CooldownEntry, now time.Time) {
	entry.NextRecoveryAt = now.Add(30 * time.Second)
	entry.CooldownUntil = entry.NextRecoveryAt
	store.SetCooldown(entry)
	_ = store.SaveStateAtomic(ctx)
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
	_ = store.SaveStateAtomic(ctx)
	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()
	r.signalRateLimitWorker()
}

func (r *Runtime) escalateAuthProblem(ctx context.Context, store *state.Store, entry state.CooldownEntry, status int, now time.Time) {
	key := state.FailureKey(entry.AuthIndex, entry.ModelGroup, entry.Model)
	store.DeleteCooldownExact(key)
	accountKey := state.CooldownKey(entry.AuthIndex, "")
	if existing, active := store.GetCooldown(accountKey); active && now.Before(existing.CooldownUntil) {
		_ = store.SaveStateAtomic(ctx)
		candidatesByPool, _, _ := r.recomputeAndCaptureRouting(now, entry.AuthIndex)
		r.syncSnapshotCooldown(entry.AuthIndex, candidatesByPool)
		return
	}
	kind := state.FailureUnauthorized
	reason := priority.ReasonUnauthorizedCooldown
	if status == http.StatusForbidden {
		kind = state.FailureForbidden
		reason = priority.ReasonForbiddenCooldown
	}
	accountEntry := state.CooldownEntry{
		AuthIndex:     entry.AuthIndex,
		FailureKind:   kind,
		StatusCode:    status,
		TriggeredAt:   now,
		CooldownUntil: now.Add(unauthorizedQuarantine),
		Reason:        reason,
		ProbeEnabled:  false,
	}
	store.SetCooldown(accountEntry)
	_ = store.SaveStateAtomic(ctx)
	candidatesByPool, _, _ := r.recomputeAndCaptureRouting(now, entry.AuthIndex)
	r.syncSnapshotCooldown(entry.AuthIndex, candidatesByPool)
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
		// Health probes only verify that the credential serves traffic, so they keep
		// the short prompt instead of paying for a full warm-up completion.
		GenerateNaturalWarmupPrompt(),
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
	store.DeleteCooldownExact(key)
	return r.applyCooldownRecovery(ctx, store, entry, cause, trigger)
}

func (r *Runtime) applyCooldownRecovery(ctx context.Context, store *state.Store, entry state.CooldownEntry, cause, trigger string) error {
	now := r.clock.Now().UTC()
	if err := store.SaveStateAtomic(ctx); err != nil {
		r.recordCooldownFailure(ctx, true, entry.AuthIndex, cause, trigger, err)
		return fmt.Errorf("persist cooldown recovery: %w", err)
	}
	candidatesByPool, credential, found := r.recomputeAndCaptureRouting(now, entry.AuthIndex)
	r.syncSnapshotCooldown(entry.AuthIndex, candidatesByPool)
	if !found {
		credential = core.Credential{
			AuthIndex: entry.AuthIndex,
			Name:      entry.AuthIndex,
			Provider:  core.ProviderAntigravity,
			Status:    core.CredentialStatusActive,
		}
	}
	recoveredPriority := coldStartRoutingTier
	if poolKey := strings.TrimSpace(entry.ModelGroup); poolKey != "" {
		if cand, ok := candidatesByPool[poolKey]; ok {
			recoveredPriority = cand.Priority
		}
	} else if cand, ok := candidatesByPool[string(config.AntigravityModelGroupGemini)]; ok {
		recoveredPriority = cand.Priority
	}
	snapshot := apply.Snapshot(priority.Plan{
		DecidedAt: now,
		Items: []priority.PlanItem{{
			Credential: credential,
			Priority:   recoveredPriority,
			Disabled:   credential.Disabled,
			Reason:     cause,
		}},
	})
	summary := healthIsolationSummary("cooldown recovery", entry.AuthIndex, entry.ModelGroup, cause)
	result := apply.Result{
		Snapshot:  snapshot,
		Record:    apply.RecordResult{Status: apply.RecordNotAttempted},
		Attempted: 1,
		Succeeded: 1,
	}
	_, err := r.state.ProjectRun(ctx, result, summary, withTransitionStats(result, RunHistoryEntry{
		Kind:     KindCooldownRecovery,
		Trigger:  trigger,
		Message:  summary,
		Snapshot: &result.Snapshot,
	}))
	return err
}

func (r *Runtime) recomputeAndCaptureRouting(now time.Time, authIndex string, existingCreds ...[]core.Credential) (map[string]PrecomputedCandidate, core.Credential, bool) {
	r.routingMu.Lock()
	defer r.routingMu.Unlock()
	var credentials []core.Credential
	if len(existingCreds) > 0 {
		credentials = existingCreds[0]
	} else {
		credentials = r.currentCredentialsLocked()
	}
	r.recomputeRoutingTablesLocked(now, credentials)
	captured := make(map[string]PrecomputedCandidate, len(r.plansByPool))
	for pool, table := range r.plansByPool {
		if cand, ok := table.ByAuthIndex[authIndex]; ok {
			captured[string(pool)] = cand
		}
	}
	for _, cred := range credentials {
		if cred.AuthIndex == authIndex || cred.Name == authIndex || cred.Email == authIndex {
			return captured, cred, true
		}
	}
	return captured, core.Credential{}, false
}

func healthIsolationSummary(prefix, authIndex, modelGroup, reason string) string {
	scope := strings.TrimSpace(modelGroup)
	if scope == "" {
		scope = "account"
	}
	return fmt.Sprintf("%s auth=%s pool=%s reason=%s", prefix, redactRuntimeIdentifier(authIndex), scope, reason)
}

func (r *Runtime) findCredentialSummary(ctx context.Context, authIndex string) (core.Credential, []core.Credential, bool) {
	if r.hostCallbacks == nil {
		return core.Credential{}, nil, false
	}
	files, err := host.NewClient(r.hostCallbacks).ListAuthFiles(ctx)
	if err != nil {
		return core.Credential{}, nil, false
	}
	credentials := credentialsFromAuthFiles(files)
	for _, candidate := range credentials {
		if candidate.AuthIndex == authIndex || candidate.Name == authIndex || candidate.Email == authIndex {
			return candidate, credentials, true
		}
	}
	return core.Credential{}, credentials, false
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

func (r *Runtime) rateLimitDiagnostics(emailsByAuthIndex map[string]string) (pending []map[string]any, storm map[string]any) {
	r.rateLimitMu.Lock()
	defer r.rateLimitMu.Unlock()
	for _, item := range r.pending429 {
		entry := map[string]any{
			"auth_index":    redactRuntimeIdentifier(item.AuthIndex),
			"model_group":   item.ModelGroup,
			"model":         item.Model,
			"first_seen_at": item.FirstSeen,
			"updated_at":    item.UpdatedAt,
			"count":         item.Count,
		}
		if email := emailsByAuthIndex[item.AuthIndex]; email != "" {
			entry["email"] = email
		}
		pending = append(pending, entry)
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
