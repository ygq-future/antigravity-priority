package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/state"
)

const maxRunHistory = 10

// Runtime manages plugin lifecycle, configuration, ticker worker, and single-flight execution.
type Runtime struct {
	mu                 sync.Mutex
	runMu              sync.Mutex
	tickerFactory      TickerFactory
	runner             TaskRunner
	rootCtx            context.Context
	cancel             context.CancelFunc
	cfg                config.Config
	configWarnings     []string
	hostCallbacks      host.HostCallbacks
	clock              Clock
	sleeper            Sleeper
	management         *management.Handler
	latestResult       apply.Result
	latestAudit        string
	latestDualSnapshot *apply.DualGroupSnapshot
	scheduleConfig     state.ScheduleConfig
	runHistory         []RunHistoryEntry
	lastAutoApplyAt    time.Time
	worker             *tickerWorker
	shutdown           bool
}

// New creates an initialized Runtime instance.
func New(options Options) *Runtime {
	factory := options.TickerFactory
	if factory == nil {
		factory = timeTickerFactory{}
	}
	clock := options.Clock
	if clock == nil {
		clock = realRuntimeClock{}
	}
	sleeper := options.Sleeper
	if sleeper == nil {
		sleeper = realSleeper{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	rt := &Runtime{
		tickerFactory: factory,
		rootCtx:       ctx,
		cancel:        cancel,
		cfg:           config.Default(),
		hostCallbacks: options.Host,
		clock:         clock,
		sleeper:       sleeper,
	}
	if options.Runner != nil {
		rt.runner = options.Runner
	} else {
		rt.runner = rt.runProductionTask
	}
	rt.management = management.NewHandler(managementRunner{runtime: rt})

	// Restore persisted cache, learned rates, and execution snapshot from disk on startup
	cachePath := rt.cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(context.Background(), cachePath); err == nil {
		audit, resJSON, histJSON := store.GetRuntimeSnapshot()
		if len(resJSON) > 0 {
			var res apply.Result
			if err := json.Unmarshal(resJSON, &res); err == nil {
				rt.latestResult = res
			}
		}
		if len(histJSON) > 0 {
			var hist []RunHistoryEntry
			if err := json.Unmarshal(histJSON, &hist); err == nil {
				rt.runHistory = hist
			}
		}
		if audit != "" {
			rt.latestAudit = audit
		}
		rt.scheduleConfig = store.GetScheduleConfig()
		if dynCfg, ok := store.GetDynamicConfig(); ok {
			if merged, err := dynCfg.ApplyTo(rt.cfg); err == nil {
				rt.cfg = merged
				rt.scheduleConfig = merged.Schedule
			}
		}
		// Unconditionally ensure cache file exists on disk upon initialization
		_ = store.SaveAtomic(context.Background())
	}

	return rt
}

// Handle routes CPA JSON-RPC method calls to their respective handlers.
func (r *Runtime) Handle(ctx context.Context, method string, request []byte) []byte {
	switch method {
	case "plugin.register":
		parsed, err := decodeRegisterRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Register(ctx, parsed)
		return envelopeRegister(result, err)
	case "plugin.reconfigure":
		parsed, err := decodeReconfigureRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Reconfigure(ctx, parsed)
		return envelopeRegister(result, err)
	case "plugin.shutdown":
		return envelopeStatus(r.Shutdown(ctx))
	case "management.register":
		return r.registerManagement()
	case "management.handle":
		return r.handleManagement(ctx, request)
	case "filter.response", "filter.complete", "filter.error", "filter.outbound", "filter.inbound":
		return r.handleFilterEvent(ctx, request)
	default:
		return failure(fmt.Errorf("%w: method %q", ErrInvalidRequest, method))
	}
}

// Register initializes the plugin with configuration received from CPA and starts scheduled workers.
func (r *Runtime) Register(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	cfg, warnings, err := config.LoadBytes([]byte(req.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load register config: %w", err)
	}
	r.mu.Lock()
	r.configWarnings = warnings
	r.mu.Unlock()
	cfg = r.mergePersistedDynamicConfig(cfg)
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

// Reconfigure updates runtime configuration dynamically and adjusts scheduled workers.
func (r *Runtime) Reconfigure(ctx context.Context, req ReconfigureRequest) (RegisterResult, error) {
	cfg, warnings, err := config.LoadBytes([]byte(req.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load reconfigure config: %w", err)
	}
	r.mu.Lock()
	r.configWarnings = warnings
	r.mu.Unlock()
	cfg = r.mergePersistedDynamicConfig(cfg)
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

// DryRun triggers a dry-run scheduling calculation without host write-back.
func (r *Runtime) DryRun(ctx context.Context, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	return r.run(ctx, TriggerManual, modelGroup, authIndexes)
}

// ManualApply triggers an immediate priority calculation and host write-back.
func (r *Runtime) ManualApply(ctx context.Context, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	return r.run(ctx, TriggerManualApply, modelGroup, authIndexes)
}

// Probe triggers a probe-only execution: fetches fresh quota and updates the cache without planning or applying.
func (r *Runtime) Probe(ctx context.Context, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	return r.run(ctx, TriggerProbe, modelGroup, authIndexes)
}

// ResetAllPriorities removes the priority field from all Antigravity credentials in CPA host.
func (r *Runtime) ResetAllPriorities(ctx context.Context) (map[string]any, error) {
	if !r.runMu.TryLock() {
		return nil, ErrRunInProgress
	}
	defer r.runMu.Unlock()

	if r.hostCallbacks == nil {
		return nil, errMissingHostCallbacks
	}
	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return nil, err
	}

	credentials := credentialsFromAuthFiles(files)
	resetCount := 0
	for _, cred := range credentials {
		if err := client.ResetPriority(ctx, cred.AuthIndex); err == nil {
			resetCount++
		}
	}

	summary := fmt.Sprintf("reset %d Antigravity credentials priority to default unset state", resetCount)
	res := apply.Result{
		Attempted: resetCount,
		Succeeded: resetCount,
		Snapshot: apply.PlanSnapshot{
			Items: make([]apply.SnapshotItem, 0),
		},
	}
	r.snapshotRunEntry(res, summary, RunHistoryEntry{
		Kind:      "reset",
		Trigger:   "manual",
		Attempted: resetCount,
		Succeeded: resetCount,
		Message:   summary,
	})

	return map[string]any{
		"ok":          true,
		"message":     summary,
		"reset_count": resetCount,
	}, nil
}

// AutoApply executes a background scheduled run respecting interval cooldown.
func (r *Runtime) AutoApply(ctx context.Context) error {
	return r.runAuto(ctx)
}

func (r *Runtime) run(ctx context.Context, trigger Trigger, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	if !r.runMu.TryLock() {
		return ErrRunInProgress
	}
	defer r.runMu.Unlock()

	taskCtx, cleanup, cfg, runner, err := r.taskContext(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	if !cfg.Enabled && trigger != TriggerManual {
		return errors.New("plugin is disabled")
	}

	if modelGroup != "" {
		cfg.AntigravityModelGroup = modelGroup
	}

	if err := runner(taskCtx, TaskRequest{
		Config:      cfg,
		Trigger:     trigger,
		AuthIndexes: append([]string(nil), authIndexes...),
	}); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run %s: %w", trigger, err)
	}
	return nil
}

func (r *Runtime) runAuto(ctx context.Context) error {
	if !r.runMu.TryLock() {
		return ErrRunInProgress
	}
	defer r.runMu.Unlock()

	r.mu.Lock()
	cfg := r.cfg
	sched := r.scheduleConfig
	r.mu.Unlock()

	// Guard: skip if plugin is disabled or scheduler is paused
	if !cfg.Enabled || sched.Paused {
		return nil
	}

	taskCtx, cleanup, cfg, runner, err := r.taskContext(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	now := r.clock.Now().UTC()

	if !state.IsInScheduleWindow(now, sched) {
		return nil
	}

	r.mu.Lock()
	last := r.lastAutoApplyAt
	interval := cfg.Interval
	if interval > 0 && !last.IsZero() && now.Sub(last) < interval {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	runErr := runner(taskCtx, TaskRequest{
		Config:  cfg,
		Trigger: TriggerAutoApply,
	})
	if runErr != nil {
		if !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
			r.snapshotRunEntry(apply.Result{}, runErr.Error(), RunHistoryEntry{
				Kind:    "auto_apply",
				Trigger: string(TriggerAutoApply),
				Message: "auto_apply error: " + runErr.Error(),
			})
		}
		return fmt.Errorf("run %s: %w", TriggerAutoApply, runErr)
	}

	r.mu.Lock()
	r.lastAutoApplyAt = r.clock.Now().UTC()
	r.mu.Unlock()
	return nil
}

func (r *Runtime) nextAutoApplyWait(interval time.Duration) time.Duration {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	r.mu.Lock()
	last := r.lastAutoApplyAt
	r.mu.Unlock()
	if last.IsZero() {
		return time.Second
	}
	remaining := interval - r.clock.Now().UTC().Sub(last)
	if remaining < time.Second {
		return time.Second
	}
	return remaining
}

// Config returns the current configuration snapshot.
func (r *Runtime) Config() (config.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.shutdown {
		return config.Config{}, ErrShutdown
	}
	return r.cfg, nil
}

// Status returns current status summary for UI rendering.
func (r *Runtime) Status(ctx context.Context) (management.StatusInfo, error) {
	cfg, err := r.Config()
	if err != nil {
		return management.StatusInfo{}, err
	}
	latestAudit := "runtime management API ready"
	if !cfg.Enabled {
		latestAudit = "runtime management API disabled by config"
	}
	_, audit := r.currentRunSnapshot()
	if audit != "" {
		latestAudit = audit
	}
	return management.StatusInfo{
		LatestAudit: latestAudit,
	}, nil
}

// LatestSnapshot returns the most recently generated dual-group plan snapshot.
func (r *Runtime) LatestSnapshot(ctx context.Context) (apply.DualGroupSnapshot, error) {
	if _, err := r.Config(); err != nil {
		return apply.DualGroupSnapshot{}, err
	}
	r.mu.Lock()
	snap := r.latestDualSnapshot
	r.mu.Unlock()
	if snap != nil {
		return *snap, nil
	}
	// Fallback: wrap the legacy single-group result
	result, _ := r.currentRunSnapshot()
	cfg, _ := r.Config()
	return apply.NewDualGroupSnapshot(
		string(cfg.AntigravityModelGroup),
		r.clock.Now().UTC(),
		result.Snapshot,
		apply.PlanSnapshot{Items: []apply.SnapshotItem{}, Changes: []apply.SnapshotChange{}},
	), nil
}

// Diagnostics returns a comprehensive diagnostics map.
func (r *Runtime) Diagnostics(ctx context.Context) (map[string]any, error) {
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	result, audit := r.currentRunSnapshot()
	r.mu.Lock()
	lastAuto := r.lastAutoApplyAt
	workerActive := r.worker != nil
	warnings := append([]string(nil), r.configWarnings...)
	sched := r.scheduleConfig
	r.mu.Unlock()
	nextWait := r.nextAutoApplyWait(cfg.Interval)
	nextRunAt := r.clock.Now().UTC().Add(nextWait)
	return map[string]any{
		"management_api": map[string]any{
			"status":     "ready",
			"auto_apply": cfg.AutoApply,
			"enabled":    cfg.Enabled,
		},
		"scheduler": map[string]any{
			"interval":           cfg.Interval.String(),
			"last_auto_apply_at": lastAuto,
			"next_wait":          nextWait.String(),
			"next_run_at":        nextRunAt.Format(time.RFC3339),
			"worker_active":      workerActive,
			"paused":             sched.Paused,
			"window_enabled":     sched.WindowEnabled,
			"window_start":       sched.WindowStart,
			"window_end":         sched.WindowEnd,
		},
		"config_warnings": warnings,
		"latest_audit":    audit,
		"last_result":     result,
		"run_history":     r.currentRunHistory(),
	}, nil
}

// Shutdown terminates runtime workers and marks runtime as shutdown.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return nil
	}
	r.shutdown = true
	r.cancel()
	worker := r.worker
	r.worker = nil
	r.mu.Unlock()
	return stopWorker(ctx, worker)
}

func (r *Runtime) replaceConfig(ctx context.Context, cfg config.Config) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("runtime configure context: %w", err)
	}

	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return ErrShutdown
	}
	oldCfg := r.cfg
	oldWorker := r.worker
	needRestartWorker := oldWorker == nil ||
		oldCfg.Enabled != cfg.Enabled ||
		oldCfg.AutoApply != cfg.AutoApply ||
		oldCfg.Interval != cfg.Interval
	r.cfg = cfg
	r.mu.Unlock()

	if !needRestartWorker {
		return nil
	}

	worker := r.newWorker(cfg)
	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return stopNewWorker(worker, ErrShutdown)
	}
	oldWorker = r.worker
	r.worker = worker
	r.mu.Unlock()

	if worker != nil {
		worker.start(r.rootCtx, r)
	}
	return stopWorker(ctx, oldWorker)
}

func (r *Runtime) newWorker(cfg config.Config) *tickerWorker {
	if !cfg.Enabled || !cfg.AutoApply {
		return nil
	}
	return &tickerWorker{
		interval: cfg.Interval,
		ticker:   r.tickerFactory.NewTicker(cfg.Interval),
		done:     make(chan struct{}),
	}
}

func (r *Runtime) taskContext(ctx context.Context) (context.Context, func(), config.Config, TaskRunner, error) {
	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return nil, nil, config.Config{}, nil, ErrShutdown
	}
	rootCtx, cfg, runner := r.rootCtx, r.cfg, r.runner
	r.mu.Unlock()

	taskCtx, cancel := context.WithCancel(rootCtx)
	stop := context.AfterFunc(ctx, cancel)
	cleanup := func() {
		stop()
		cancel()
	}
	return taskCtx, cleanup, cfg, runner, nil
}

func (r *Runtime) snapshotRunEntry(result apply.Result, audit string, entry RunHistoryEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latestResult = result
	r.latestAudit = audit
	if entry.At.IsZero() {
		entry.At = r.clock.Now().UTC()
	}
	if entry.Kind == "" {
		entry.Kind = "apply"
	}
	history := make([]RunHistoryEntry, 0, maxRunHistory)
	history = append(history, entry)
	for i := 0; i < len(r.runHistory) && len(history) < maxRunHistory; i++ {
		history = append(history, r.runHistory[i])
	}
	r.runHistory = history
}

func (r *Runtime) setDualSnapshot(snap apply.DualGroupSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latestDualSnapshot = &snap
}

// GetScheduleConfig returns the current dynamic schedule configuration.
func (r *Runtime) GetScheduleConfig(ctx context.Context) (config.ScheduleConfig, error) {
	if _, err := r.Config(); err != nil {
		return config.ScheduleConfig{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scheduleConfig, nil
}

// SetScheduleConfig updates the dynamic schedule configuration and persists it to the state store.
func (r *Runtime) SetScheduleConfig(ctx context.Context, cfg config.ScheduleConfig) error {
	if err := config.ValidateScheduleWindow(cfg.WindowStart, cfg.WindowEnd); err != nil {
		return err
	}
	runtimeCfg, err := r.Config()
	if err != nil {
		return err
	}
	cachePath := runtimeCfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return fmt.Errorf("load state for schedule config: %w", err)
	}
	store.SetScheduleConfig(cfg)
	if err := store.SaveAtomic(ctx); err != nil {
		return fmt.Errorf("persist schedule config: %w", err)
	}
	r.mu.Lock()
	r.scheduleConfig = cfg
	r.cfg.Schedule = cfg
	r.mu.Unlock()
	return nil
}

// GetDynamicConfig returns the active dynamic configuration.
func (r *Runtime) GetDynamicConfig(ctx context.Context) (config.DynamicConfig, error) {
	r.mu.Lock()
	cfg := r.cfg
	sched := r.scheduleConfig
	r.mu.Unlock()

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err == nil {
		if dyn, ok := store.GetDynamicConfig(); ok {
			return dyn, nil
		}
	}

	dyn := cfg.Dynamic()
	dyn.Schedule = sched
	return dyn, nil
}

// SetDynamicConfig validates, persists, and hot-applies new dynamic configuration without restarting.
func (r *Runtime) SetDynamicConfig(ctx context.Context, dyn config.DynamicConfig) error {
	r.mu.Lock()
	baseCfg := r.cfg
	cachePath := r.cfg.StateCachePath
	r.mu.Unlock()

	newCfg, err := dyn.ApplyTo(baseCfg)
	if err != nil {
		return err
	}

	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return fmt.Errorf("load state for save: %w", err)
	}
	store.SetDynamicConfig(dyn)
	store.SetScheduleConfig(dyn.Schedule)
	if err := store.SaveAtomic(ctx); err != nil {
		return fmt.Errorf("save dynamic config: %w", err)
	}

	r.mu.Lock()
	r.scheduleConfig = dyn.Schedule
	r.mu.Unlock()

	return r.replaceConfig(ctx, newCfg)
}

func (r *Runtime) mergePersistedDynamicConfig(baseCfg config.Config) config.Config {
	cachePath := baseCfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		return baseCfg
	}
	dynCfg, ok := store.GetDynamicConfig()
	if !ok {
		return baseCfg
	}
	merged, err := dynCfg.ApplyTo(baseCfg)
	if err != nil {
		return baseCfg
	}
	return merged
}

func (r *Runtime) currentRunSnapshot() (apply.Result, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latestResult, r.latestAudit
}

func (r *Runtime) currentRunHistory() []RunHistoryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]RunHistoryEntry, len(r.runHistory))
	copy(out, r.runHistory)
	return out
}

func registrationResult() RegisterResult {
	return RegisterResult{
		SchemaVersion: 1,
		Metadata:      buildMetadata(),
		Capabilities: map[string]bool{
			"management_api":  true,
			"management":      true,
			"filter.response": true,
			"filter.complete": true,
			"filter.error":    true,
		},
	}
}

func (r *Runtime) handleFilterEvent(ctx context.Context, raw []byte) []byte {
	var payload struct {
		AuthIndex  string `json:"auth_index"`
		AuthName   string `json:"auth_name"`
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
		ModelGroup string `json:"model_group"`
	}
	_ = json.Unmarshal(raw, &payload)
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthName)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}

	is429 := payload.StatusCode == 429 || strings.Contains(strings.ToLower(payload.Error), "429") ||
		strings.Contains(strings.ToUpper(payload.Error), "RESOURCE_EXHAUSTED") ||
		strings.Contains(strings.ToUpper(payload.Error), "RATE_LIMIT")

	if is429 {
		r.triggerCooldown(ctx, authIndex, payload.ModelGroup, "429 rate limit detected")
	}

	return mustMarshal(Envelope{OK: true})
}

func (r *Runtime) triggerCooldown(ctx context.Context, authIndex, modelGroup, reason string) {
	now := r.clock.Now().UTC()
	cfg, _ := r.Config()
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return
	}

	cooldownMinutes := cfg.RateLimitCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = config.DefaultRateLimitCooldownMinutes
	}
	cooldownUntil := now.Add(time.Duration(cooldownMinutes) * time.Minute)

	store.SetCooldown(state.CooldownEntry{
		AuthIndex:     authIndex,
		ModelGroup:    modelGroup,
		TriggeredAt:   now,
		CooldownUntil: cooldownUntil,
		Reason:        reason,
	})
	_ = store.SaveAtomic(ctx)

	// Immediately demote priority to -1 on the host
	if r.hostCallbacks != nil {
		client := host.NewClient(r.hostCallbacks)
		_ = client.PatchPriority(ctx, authIndex, -1)
	}
}

func buildMetadata() Metadata {
	return Metadata{
		Name:             "Antigravity Priority",
		Version:          "1.1.0",
		Author:           "ygq-future",
		GitHubRepository: "https://github.com/ygq-future/antigravity-priority",
		Description:      "Intelligent quota pacing and adaptive burn-rate priority scheduler exclusively for Google Antigravity in CLIProxyAPI.",
	}
}

type realRuntimeClock struct{}

func (realRuntimeClock) Now() time.Time {
	return time.Now().UTC()
}

type realSleeper struct{}

func (realSleeper) Sleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
