package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/state"
)

var errMissingStateStore = errors.New("project execution result: state store is required")

// stateRepository is the single Runtime-owned authority over persisted and
// in-memory state. All domain mutations update the single *state.Store instance
// and persist only the affected split document atomically.
type stateRepository struct {
	storeMu sync.RWMutex
	runsMu  sync.Mutex
	path    string
	store   *state.Store
	loadErr error
	clock   Clock

	latestResult apply.Result
	latestAudit  string
	runHistory   []RunHistoryEntry
}

func resolveCachePath(path string) string {
	if strings.TrimSpace(path) == "" {
		return config.DefaultStateCachePath
	}
	return path
}

func newStateRepository(ctx context.Context, path string, clock Clock, historyLimit int) *stateRepository {
	if clock == nil {
		clock = realRuntimeClock{}
	}
	resolved := resolveCachePath(path)
	repo := &stateRepository{
		path:  resolved,
		clock: clock,
	}
	repo.storeMu.Lock()
	defer repo.storeMu.Unlock()
	repo.loadLocked(ctx, resolved, historyLimit)
	return repo
}

func (r *stateRepository) loadLocked(ctx context.Context, path string, historyLimit int) {
	resolved := resolveCachePath(path)
	r.path = resolved
	store, err := state.Load(ctx, resolved)
	if err != nil {
		r.store = nil
		r.loadErr = err
		return
	}
	r.store = store
	r.loadErr = nil

	for key, entry := range store.GetCooldowns() {
		if entry.FailureKind == state.FailureForbidden && entry.ProbeEnabled {
			store.DeleteCooldownExact(key)
			accountEntry := entry
			accountEntry.ModelGroup = ""
			accountEntry.Model = ""
			accountEntry.ProbeModel = ""
			accountEntry.ProbeEnabled = false
			accountEntry.NextRecoveryAt = time.Time{}
			store.SetCooldown(accountEntry)
		}
	}

	if dynCfg, ok := store.GetDynamicConfig(); ok && dynCfg.RequestHistoryLimit > 0 {
		historyLimit = dynCfg.RequestHistoryLimit
	}
	if historyLimit <= 0 {
		historyLimit = state.DefaultMaxRequestHistory
	}
	store.TrimRequestHistory(historyLimit)
	// Ensure split cache documents exist on disk upon initialization/migration.
	if err := store.SaveAtomic(ctx); err != nil {
		log.Printf("[antigravity-priority] state cache init save failed (%s): %v", resolved, err)
	}
	audit, resJSON, histJSON := store.GetRuntimeSnapshot()
	r.runsMu.Lock()
	r.latestResult = apply.Result{}
	r.latestAudit = audit
	r.runHistory = nil
	if len(resJSON) > 0 {
		var res apply.Result
		if err := json.Unmarshal(resJSON, &res); err == nil {
			r.latestResult = res
		}
	}
	if len(histJSON) > 0 {
		var hist []RunHistoryEntry
		if err := json.Unmarshal(histJSON, &hist); err == nil {
			r.runHistory = hist
		}
	}
	r.runsMu.Unlock()
}

func (r *stateRepository) EnsurePath(ctx context.Context, path string, historyLimit int) {
	if r == nil {
		return
	}
	resolved := resolveCachePath(path)
	r.storeMu.RLock()
	same := filepath.Clean(r.path) == filepath.Clean(resolved) && r.store != nil
	r.storeMu.RUnlock()
	if same {
		return
	}
	r.storeMu.Lock()
	defer r.storeMu.Unlock()
	if filepath.Clean(r.path) == filepath.Clean(resolved) && r.store != nil {
		return
	}
	r.loadLocked(ctx, resolved, historyLimit)
}

func (r *stateRepository) Path() string {
	if r == nil {
		return config.DefaultStateCachePath
	}
	r.storeMu.RLock()
	defer r.storeMu.RUnlock()
	return resolveCachePath(r.path)
}

func (r *stateRepository) Store(ctx context.Context) (*state.Store, error) {
	if r == nil {
		return nil, state.ErrCorruptCache
	}
	r.storeMu.RLock()
	store := r.store
	r.storeMu.RUnlock()
	if store != nil {
		return store, nil
	}
	r.storeMu.Lock()
	defer r.storeMu.Unlock()
	if r.store != nil {
		return r.store, nil
	}
	r.loadLocked(ctx, r.path, state.DefaultMaxRequestHistory)
	if r.store == nil {
		return nil, r.loadErr
	}
	return r.store, nil
}

func (r *stateRepository) StoreOrNil() *state.Store {
	if r == nil {
		return nil
	}
	r.storeMu.RLock()
	defer r.storeMu.RUnlock()
	return r.store
}

func (r *stateRepository) ScheduleConfig() state.ScheduleConfig {
	store := r.StoreOrNil()
	if store == nil {
		return state.ScheduleConfig{}
	}
	return store.GetScheduleConfig()
}

func (r *stateRepository) SetScheduleConfig(ctx context.Context, sched state.ScheduleConfig) error {
	store, err := r.Store(ctx)
	if err != nil {
		return err
	}
	store.SetScheduleConfig(sched)
	return store.SaveRuntimeAtomic(ctx)
}

func (r *stateRepository) DynamicConfig(base config.Config) config.DynamicConfig {
	store := r.StoreOrNil()
	if store != nil {
		sched := store.GetScheduleConfig()
		if dyn, ok := store.GetDynamicConfig(); ok {
			dyn.Schedule = sched
			return dyn
		}
		dyn := base.Dynamic()
		dyn.Schedule = sched
		return dyn
	}
	return base.Dynamic()
}

func (r *stateRepository) SetDynamicConfig(ctx context.Context, dyn config.DynamicConfig) error {
	store, err := r.Store(ctx)
	if err != nil {
		return err
	}
	store.SetDynamicConfig(dyn)
	store.SetScheduleConfig(dyn.Schedule)
	return store.SaveRuntimeAtomic(ctx)
}

func (r *stateRepository) MergeDynamicConfig(base config.Config) config.Config {
	store := r.StoreOrNil()
	if store == nil {
		return base
	}
	sched := store.GetScheduleConfig()
	dynCfg, ok := store.GetDynamicConfig()
	if !ok {
		if sched != (config.ScheduleConfig{}) {
			base.Schedule = sched
		}
		return base
	}
	merged, err := dynCfg.ApplyTo(base)
	if err != nil {
		return base
	}
	if sched != (config.ScheduleConfig{}) {
		merged.Schedule = sched
	}
	return merged
}

func (r *stateRepository) CustomTiers(base config.Config) map[string]int {
	if store := r.StoreOrNil(); store != nil {
		if tiers := store.GetCustomTiers(); len(tiers) > 0 {
			return tiers
		}
	}
	if len(base.CustomTiers) > 0 {
		cloned := make(map[string]int, len(base.CustomTiers))
		maps.Copy(cloned, base.CustomTiers)
		return cloned
	}
	return map[string]int{}
}

func (r *stateRepository) SetCustomTier(ctx context.Context, authIndex string, tier int) error {
	store, err := r.Store(ctx)
	if err != nil {
		return err
	}
	store.SetCustomTier(authIndex, tier)
	return store.SaveRuntimeAtomic(ctx)
}

func (r *stateRepository) SetWarmupCredentialState(
	ctx context.Context,
	base config.Config,
	authIndex string,
	group config.AntigravityModelGroup,
	enabled bool,
) (config.WarmupConfig, error) {
	store, err := r.Store(ctx)
	if err != nil {
		return config.WarmupConfig{}, err
	}
	dyn := r.DynamicConfig(base)
	dyn.Warmup.Normalize()
	globalEnabled := warmupEnabledForGroup(dyn.Warmup, group)
	dyn.Warmup.ClearWarmupOverride(authIndex)
	if enabled != globalEnabled {
		dyn.Warmup.SetWarmupOverride(authIndex, enabled)
	}
	if err := dyn.Validate(); err != nil {
		return config.WarmupConfig{}, err
	}
	store.SetDynamicConfig(dyn)
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
		return config.WarmupConfig{}, err
	}
	if enabled && store.ClearWarmupCompleted(authIndex) > 0 {
		if err := store.SaveStateAtomic(ctx); err != nil {
			return config.WarmupConfig{}, err
		}
	}
	return dyn.Warmup.Clone(), nil
}

func (r *stateRepository) LastAutoApplyAt() time.Time {
	store := r.StoreOrNil()
	if store == nil {
		return time.Time{}
	}
	return store.GetLastAutoApplyAt()
}

func (r *stateRepository) RecordAutoApplyAt(ctx context.Context, at time.Time) error {
	store, err := r.Store(ctx)
	if err != nil {
		return err
	}
	store.SetLastAutoApplyAt(at)
	return store.SaveRuntimeAtomic(ctx)
}

func (r *stateRepository) SetWarmupLane(ctx context.Context, lane state.WarmupLane) error {
	store, err := r.Store(ctx)
	if err != nil {
		return err
	}
	store.SetWarmupLane(lane)
	return store.SaveRuntimeAtomic(ctx)
}

func (r *stateRepository) Cooldowns() map[string]state.CooldownEntry {
	store := r.StoreOrNil()
	if store == nil {
		return nil
	}
	return store.GetCooldowns()
}

func (r *stateRepository) RequestHistory() []state.RequestHistoryEntry {
	store := r.StoreOrNil()
	if store == nil {
		return nil
	}
	return store.GetRequestHistory()
}

func (r *stateRepository) AppendRequestHistory(ctx context.Context, entry state.RequestHistoryEntry, limit int) {
	store := r.StoreOrNil()
	if store == nil {
		return
	}
	store.AppendRequestHistory(entry, limit)
	_ = store.SaveRequestsAtomic(ctx)
}

func (r *stateRepository) TrimRequestHistory(ctx context.Context, limit int) {
	store := r.StoreOrNil()
	if store == nil {
		return
	}
	if store.TrimRequestHistory(limit) {
		_ = store.SaveRequestsAtomic(ctx)
	}
}

func (r *stateRepository) ClearRequestHistory(ctx context.Context) {
	store := r.StoreOrNil()
	if store == nil {
		return
	}
	store.SetRequestHistory(nil)
	_ = store.SaveRequestsAtomic(ctx)
}

func (r *stateRepository) RunSnapshot() (apply.Result, string) {
	if r == nil {
		return apply.Result{}, ""
	}
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	return r.latestResult, r.latestAudit
}

func (r *stateRepository) RunHistory() []RunHistoryEntry {
	if r == nil {
		return nil
	}
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	return slices.Clone(r.runHistory)
}

func (r *stateRepository) LatestAutoApplyEntry() *RunHistoryEntry {
	if r == nil {
		return nil
	}
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	for _, entry := range r.runHistory {
		if entry.Kind == KindAutoApply {
			copied := entry
			return &copied
		}
	}
	return nil
}

func (r *stateRepository) SnapshotRunEntry(result apply.Result, audit string, entry RunHistoryEntry) {
	if r == nil {
		return
	}
	r.runsMu.Lock()
	defer r.runsMu.Unlock()
	r.snapshotRunEntryLocked(result, audit, entry)
}

func (r *stateRepository) snapshotRunEntryLocked(result apply.Result, audit string, entry RunHistoryEntry) {
	r.latestResult = result
	r.latestAudit = audit
	if entry.At.IsZero() {
		if r.clock != nil {
			entry.At = r.clock.Now().UTC()
		} else {
			entry.At = time.Now().UTC()
		}
	}
	history := make([]RunHistoryEntry, 0, maxRunHistory)
	history = append(history, entry)
	for i := 0; i < len(r.runHistory) && len(history) < maxRunHistory; i++ {
		history = append(history, r.runHistory[i])
	}
	r.runHistory = history
}

func (r *stateRepository) replaceTopRunEntryLocked(result apply.Result, entry RunHistoryEntry) {
	r.latestResult = result
	if len(r.runHistory) > 0 {
		if entry.At.IsZero() {
			entry.At = r.runHistory[0].At
		}
		r.runHistory[0] = entry
	}
}

func (r *stateRepository) ProjectRun(ctx context.Context, result apply.Result, audit string, entry RunHistoryEntry) (apply.Result, error) {
	if r == nil {
		result.Record = apply.RecordResult{Status: apply.RecordFailed, Error: "state store is required"}
		return result, errMissingStateStore
	}
	r.runsMu.Lock()
	defer r.runsMu.Unlock()

	store := r.StoreOrNil()
	if store == nil {
		result.Record = apply.RecordResult{Status: apply.RecordFailed, Error: "state store is required"}
		r.snapshotRunEntryLocked(result, audit, entry)
		return result, errMissingStateStore
	}
	result.Record = apply.RecordResult{Status: apply.RecordPersisted}
	entry.Record = result.Record
	r.snapshotRunEntryLocked(result, audit, entry)

	resultJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		result.Record = apply.RecordResult{Status: apply.RecordFailed, Error: host.RedactBytes([]byte(marshalErr.Error()))}
		entry.Record = result.Record
		r.replaceTopRunEntryLocked(result, entry)
		return result, fmt.Errorf("encode execution result: %w", marshalErr)
	}
	historyJSON, marshalErr := json.Marshal(r.runHistory)
	if marshalErr != nil {
		result.Record = apply.RecordResult{Status: apply.RecordFailed, Error: host.RedactBytes([]byte(marshalErr.Error()))}
		entry.Record = result.Record
		r.replaceTopRunEntryLocked(result, entry)
		return result, fmt.Errorf("encode execution history: %w", marshalErr)
	}
	store.SetRuntimeSnapshot(audit, resultJSON, historyJSON)
	if err := store.SaveRunsAtomic(ctx); err != nil {
		result.Record = apply.RecordResult{Status: apply.RecordFailed, Error: host.RedactBytes([]byte(err.Error()))}
		entry.Record = result.Record
		r.replaceTopRunEntryLocked(result, entry)
		return result, fmt.Errorf("persist execution result: %w", err)
	}
	return result, nil
}
