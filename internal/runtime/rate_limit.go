package runtime

import (
	"context"
	"errors"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

const (
	rateLimitObservationWindow = 10 * time.Second
	rateLimitStormWindow       = 30 * time.Second
)

type pending429 struct {
	AuthIndex  string
	ModelGroup string
	Reason     string
	FirstSeen  time.Time
	Deadline   time.Time
}

func (r *Runtime) observe429(ctx context.Context, authIndex, modelGroup, reason string, acquireRunLock bool) error {
	now := r.clock.Now().UTC()
	r.rateLimitMu.Lock()
	if now.Before(r.stormUntil) {
		r.rateLimitMu.Unlock()
		return nil
	}
	if pending, ok := r.pending429[authIndex]; ok {
		delete(r.pending429, authIndex)
		r.rateLimitMu.Unlock()
		r.signalRateLimitWorker()
		if acquireRunLock {
			r.runMu.Lock()
		}
		err := r.triggerCooldown(ctx, pending.AuthIndex, pending.ModelGroup, pending.Reason)
		if acquireRunLock {
			r.runMu.Unlock()
		}
		return err
	}
	r.pending429[authIndex] = pending429{AuthIndex: authIndex, ModelGroup: modelGroup, Reason: reason, FirstSeen: now, Deadline: now.Add(rateLimitObservationWindow)}
	pendingCount := len(r.pending429)
	r.rateLimitMu.Unlock()

	active := r.activeAntigravityCredentialCount(ctx)
	if pendingCount >= 3 && active > 0 && pendingCount*2 >= active {
		r.rateLimitMu.Lock()
		if len(r.pending429) >= 3 && len(r.pending429)*2 >= active {
			r.pending429 = make(map[string]pending429)
			r.stormUntil = now.Add(rateLimitStormWindow)
		}
		r.rateLimitMu.Unlock()
	}
	r.signalRateLimitWorker()
	return nil
}

func (r *Runtime) observeUsageSuccess(authIndex string) {
	r.rateLimitMu.Lock()
	delete(r.pending429, authIndex)
	if !r.stormUntil.IsZero() {
		r.stormUntil = time.Time{}
	}
	r.rateLimitMu.Unlock()
	r.signalRateLimitWorker()
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
			delay := next.Sub(r.clock.Now().UTC())
			if delay < 0 {
				delay = 0
			}
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
		case <-timerC:
			r.processRateLimitDeadlines(r.rootCtx)
		}
	}
}

func (r *Runtime) nextRateLimitDeadline() time.Time {
	r.rateLimitMu.Lock()
	var next time.Time
	for _, pending := range r.pending429 {
		next = earlierTime(next, pending.Deadline)
	}
	if !r.stormUntil.IsZero() {
		next = earlierTime(next, r.stormUntil)
	}
	r.rateLimitMu.Unlock()
	cfg, err := r.Config()
	if err == nil {
		store, loadErr := state.Load(context.Background(), cfg.StateCachePath)
		if loadErr == nil {
			for _, entry := range store.GetCooldowns() {
				deadline := entry.CooldownUntil
				if !entry.NextRecoveryAt.IsZero() && entry.NextRecoveryAt.After(deadline) {
					deadline = entry.NextRecoveryAt
				}
				next = earlierTime(next, deadline)
			}
		}
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
	due := make([]pending429, 0)
	if !r.stormUntil.IsZero() && !now.Before(r.stormUntil) {
		r.stormUntil = time.Time{}
	}
	if r.stormUntil.IsZero() {
		for key, pending := range r.pending429 {
			if !now.Before(pending.Deadline) {
				due = append(due, pending)
				delete(r.pending429, key)
			}
		}
	}
	r.rateLimitMu.Unlock()
	for _, pending := range due {
		r.runMu.Lock()
		_ = r.triggerCooldown(ctx, pending.AuthIndex, pending.ModelGroup, pending.Reason)
		r.runMu.Unlock()
	}
	cfg, err := r.Config()
	if err != nil {
		return
	}
	store, err := state.Load(ctx, cfg.StateCachePath)
	if err != nil {
		return
	}
	for _, entry := range store.GetCooldowns() {
		if !now.Before(entry.CooldownUntil) && (entry.NextRecoveryAt.IsZero() || !now.Before(entry.NextRecoveryAt)) {
			r.runMu.Lock()
			restoreErr := r.restoreCooldown(ctx, store, entry)
			r.runMu.Unlock()
			if restoreErr != nil {
				entry.NextRecoveryAt = now.Add(30 * time.Second)
				store.SetCooldown(entry)
				_ = store.SaveAtomic(ctx)
			}
		}
	}
}

func (r *Runtime) restoreCooldown(ctx context.Context, store *state.Store, entry state.CooldownEntry) error {
	credential, err := r.findCredential(ctx, entry.AuthIndex)
	if err != nil {
		return err
	}
	// A manual change supersedes the transaction; never overwrite it.
	if credential.Priority != priority.DepletedPriority || credential.Disabled != entry.AppliedDisabled {
		store.DeleteCooldown(entry.AuthIndex)
		return store.SaveAtomic(ctx)
	}
	client := host.NewClient(r.hostCallbacks)
	intent := apply.CooldownRecoveryIntent(credential, entry.PreviousPriority, entry.PreviousPriorityMissing, entry.PreviousDisabled, "429 cooldown expired")
	result, applyErr := apply.ExecuteRound(ctx, apply.NewHostTransition(client), apply.TransitionRound{Intents: []apply.TransitionIntent{intent}})
	if applyErr != nil {
		return applyErr
	}
	result.Snapshot = apply.Snapshot(priority.Plan{DecidedAt: r.clock.Now().UTC(), Items: []priority.PlanItem{{Credential: credential, Priority: entry.PreviousPriority, Disabled: entry.PreviousDisabled, Reason: priority.Reason429Cooldown}}})
	store.DeleteCooldown(entry.AuthIndex)
	if err := store.SaveAtomic(ctx); err != nil {
		return err
	}
	summary := resultSummary("429 cooldown recovery", result)
	_, err = r.projectRun(ctx, store, result, summary, RunHistoryEntry{Kind: KindCooldownRecovery, Trigger: "rate_limit_recovery", Message: summary, Snapshot: &result.Snapshot})
	return err
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
		if candidate.AuthIndex != authIndex && candidate.Name != authIndex {
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
		pending = append(pending, map[string]any{"auth_index": redactRuntimeIdentifier(item.AuthIndex), "first_seen_at": item.FirstSeen, "confirm_at": item.Deadline})
	}
	if !r.stormUntil.IsZero() {
		storm = map[string]any{"active": r.clock.Now().UTC().Before(r.stormUntil), "until": r.stormUntil}
	}
	return pending, storm
}
