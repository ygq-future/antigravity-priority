package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

const maxRunHistory = 10

// Runtime manages plugin lifecycle, configuration, ticker worker, and single-flight execution.
type Runtime struct {
	mu                  sync.Mutex
	configMu            sync.Mutex
	runMu               sync.Mutex
	runner              TaskRunner
	rootCtx             context.Context
	cancel              context.CancelFunc
	cfg                 config.Config
	hostCallbacks       host.HostCallbacks
	clock               Clock
	sleeper             Sleeper
	management          *management.Handler
	state               *stateRepository
	latestSnapshotSet   *apply.SnapshotSet
	stateCacheOverride  string
	latestProbeOutcomes []management.ProbeOutcome
	worker              *tickerWorker
	shutdown            bool
	rateLimiter         rateLimitTracker
	rateLimitWake       chan struct{}
	rateLimitDone       chan struct{}
	router              poolRouter
	probeMu             sync.Mutex
	probeAttempts       map[string]probeAttempt
	warmupResetWake     chan struct{}
	autoApplyWake       chan struct{}
	scheduleMu          sync.Mutex
	scheduleState       adaptiveScheduleState
}

// New creates an initialized Runtime instance.
func New(options Options) *Runtime {
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
		rootCtx:         ctx,
		cancel:          cancel,
		cfg:             config.Default(),
		hostCallbacks:   options.Host,
		clock:           clock,
		sleeper:         sleeper,
		rateLimiter:     newRateLimitTracker(),
		rateLimitWake:   make(chan struct{}, 1),
		rateLimitDone:   make(chan struct{}),
		router:          newPoolRouter(),
		probeAttempts:   make(map[string]probeAttempt),
		warmupResetWake: make(chan struct{}, 1),
		autoApplyWake:   make(chan struct{}, 1),
	}
	if strings.TrimSpace(options.StateCachePath) != "" {
		rt.cfg.StateCachePath = options.StateCachePath
		rt.stateCacheOverride = options.StateCachePath
	}
	if options.Runner != nil {
		rt.runner = options.Runner
	} else {
		rt.runner = rt.runProductionTask
	}
	rt.management = management.NewHandler(managementRunner{runtime: rt})

	// Restore persisted cache, learned rates, and execution snapshot from disk on startup.
	rt.state = newStateRepository(context.Background(), rt.cfg.StateCachePath, clock, rt.cfg.RequestHistoryLimit)
	rt.cfg = rt.state.MergeDynamicConfig(rt.cfg)
	if options.DisableBackgroundWorkers {
		close(rt.rateLimitDone)
	} else {
		go rt.runRateLimitWorker()
		go rt.runWarmupResetWatcher()
	}

	return rt
}

// Handle routes CPA JSON-RPC method calls to their respective handlers.
func (r *Runtime) Handle(ctx context.Context, method string, request []byte) []byte {
	switch method {
	case MethodPluginRegister:
		parsed, err := decodeRegisterRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Register(ctx, parsed)
		return envelopeRegister(result, err)
	case MethodPluginQuiesce:
		return envelopeStatus(r.Quiesce(ctx))
	case MethodPluginReconfigure:
		parsed, err := decodeReconfigureRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Reconfigure(ctx, parsed)
		return envelopeRegister(result, err)
	case MethodPluginShutdown:
		return envelopeStatus(r.Shutdown(ctx))
	case MethodManagementRegister:
		return r.registerManagement()
	case MethodManagementHandle:
		return r.handleManagement(ctx, request)
	case MethodUsageHandle:
		return r.handleUsageEvent(ctx, request)
	case MethodSchedulerPick:
		return r.handleSchedulerPick(ctx, request)
	case MethodFilterResponse, MethodFilterComplete, MethodFilterError, MethodFilterOutbound, MethodFilterInbound:
		return r.handleFilterEvent(ctx, request)
	default:
		return failure(fmt.Errorf("%w: method %q", ErrUnknownMethod, method))
	}
}

// Register initializes the plugin with configuration received from CPA and starts scheduled workers.
func (r *Runtime) Register(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	cfg, _, err := config.LoadBytes([]byte(req.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load register config: %w", err)
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	cfg = r.applyStateCacheOverride(cfg)
	r.state.EnsurePath(ctx, cfg.StateCachePath, cfg.RequestHistoryLimit)
	cfg = r.state.MergeDynamicConfig(cfg)
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

// Reconfigure updates runtime configuration dynamically and adjusts scheduled workers.
func (r *Runtime) Reconfigure(ctx context.Context, req ReconfigureRequest) (RegisterResult, error) {
	cfg, _, err := config.LoadBytes([]byte(req.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load reconfigure config: %w", err)
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	cfg = r.applyStateCacheOverride(cfg)
	r.state.EnsurePath(ctx, cfg.StateCachePath, cfg.RequestHistoryLimit)
	cfg = r.state.MergeDynamicConfig(cfg)
	if err := r.replaceConfig(ctx, cfg); err != nil {
		return RegisterResult{}, err
	}
	return registrationResult(), nil
}

func (r *Runtime) applyStateCacheOverride(cfg config.Config) config.Config {
	r.mu.Lock()
	override := r.stateCacheOverride
	r.mu.Unlock()
	if strings.TrimSpace(override) != "" && cfg.StateCachePath == config.DefaultStateCachePath {
		cfg.StateCachePath = override
	}
	return cfg
}

// Probe triggers a probe-only execution: fetches fresh quota and updates the cache without planning or applying.
func (r *Runtime) Probe(ctx context.Context, authIndexes []string) error {
	return r.run(ctx, TriggerProbe, authIndexes)
}

// ResetAllPriorities clears the static priority field from all Antigravity
// credentials in CPA Host. It is an explicit user maintenance action: the
// scheduler never reads Host priority, so this reset does not affect the
// in-memory routing tables except through the credential inventory refresh it
// triggers afterwards.
func (r *Runtime) ResetAllPriorities(ctx context.Context) (map[string]any, error) {
	if !r.runMu.TryLock() {
		return nil, ErrRunInProgress
	}
	defer r.runMu.Unlock()

	if r.hostCallbacks == nil {
		return nil, errMissingHostCallbacks
	}
	if _, err := r.Config(); err != nil {
		return nil, err
	}
	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return nil, err
	}

	credentials := credentialsFromAuthFiles(files)
	intents := make([]apply.TransitionIntent, 0, len(credentials))
	for _, credential := range credentials {
		intents = append(intents, apply.ResetIntent(credential, "priority reset"))
	}
	transitionResult, err := apply.NewHostTransition(client).Execute(ctx, apply.TransitionRound{Intents: intents})
	if err != nil {
		return nil, err
	}
	result := apply.ResultFromTransition(transitionResult)
	resetCount := transitionResult.Totals.Committed + transitionResult.Totals.NoChange

	// Update credentials to reflect reset state (PriorityMissing = true, Priority = 0)
	for i := range credentials {
		credentials[i].Priority = 0
		credentials[i].PriorityMissing = true
	}
	// Re-read the Host inventory after the transition so the projection uses
	// the authoritative post-reset credential set.
	files, err = client.ListAuthFiles(ctx)
	if err != nil {
		return nil, err
	}
	credentials = credentialsFromAuthFiles(files)
	for i := range credentials {
		credentials[i].Priority = 0
		credentials[i].PriorityMissing = true
	}
	credentials, _, _ = enrichCredentialsFromAuthDocuments(ctx, client, credentials)

	store, err := r.state.Store(ctx)
	if err != nil {
		return nil, err
	}
	now := r.clock.Now().UTC()
	snap, err := r.commitRoutingAndSnapshot(now, credentials, buildProjectionEvidence(store, credentials))
	if err != nil {
		return nil, err
	}
	// Reset execution details still render a single plan snapshot. Use the
	// Gemini pool as the stable legacy view; the three-pool SnapshotSet remains
	// the authoritative snapshot for every other consumer.
	geminiPool := snap.Groups[string(config.AntigravityModelGroupGemini)]
	legacySnapshot := apply.PlanSnapshot{
		DecidedAt:  snap.ObservedAt,
		TotalItems: len(geminiPool.Items),
		Items:      geminiPool.Items,
	}

	summary := resultSummary("reset", result)
	result.Snapshot = legacySnapshot
	historySnap := legacySnapshot
	_, projectErr := r.state.ProjectRun(ctx, result, summary, withTransitionStats(result, RunHistoryEntry{
		Kind:     KindReset,
		Trigger:  string(TriggerManualReset),
		Message:  summary,
		Snapshot: &historySnap,
	}))
	if projectErr != nil {
		return map[string]any{
			"ok":          false,
			"message":     summary,
			"reset_count": resetCount,
			"attempted":   result.Attempted,
			"succeeded":   result.Succeeded,
			"failed":      result.Failed,
			"conflicts":   result.Conflicts,
			"uncertain":   result.Uncertain,
		}, projectErr
	}

	return map[string]any{
		"ok":          true,
		"message":     summary,
		"reset_count": resetCount,
		"attempted":   result.Attempted,
		"succeeded":   result.Succeeded,
		"failed":      result.Failed,
		"conflicts":   result.Conflicts,
		"uncertain":   result.Uncertain,
	}, nil
}

// SyncHost re-reads credentials from CPA host, re-evaluates cached evidence, and updates the three-pool snapshot and routing tables.
func (r *Runtime) SyncHost(ctx context.Context) (apply.SnapshotSet, error) {
	if !r.runMu.TryLock() {
		return apply.SnapshotSet{}, ErrRunInProgress
	}
	defer r.runMu.Unlock()

	if _, err := r.Config(); err != nil {
		return apply.SnapshotSet{}, err
	}

	if r.hostCallbacks == nil {
		return apply.SnapshotSet{}, errMissingHostCallbacks
	}

	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return apply.SnapshotSet{}, err
	}

	credentials := credentialsFromAuthFiles(files)
	credentials, _, err = enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return apply.SnapshotSet{}, err
	}

	store, err := r.state.Store(ctx)
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	authModels, _ := client.ListAuthModels(ctx)
	now := r.clock.Now().UTC()
	snap, err := r.commitRoutingAndSnapshotWithInventory(now, files, authModels, credentials, buildProjectionEvidence(store, credentials))
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	return cloneSnapshotSet(snap), nil
}

// GetSamples returns the historical quota samples for a specific credential and model group.
func (r *Runtime) GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error) {
	if _, err := r.Config(); err != nil {
		return nil, err
	}
	store, err := r.state.Store(ctx)
	if err != nil {
		return nil, err
	}
	return store.GetSamples(authIndex, modelGroup), nil
}

// GetProbeSamples returns only samples appended by one probe round.
func (r *Runtime) GetProbeSamples(ctx context.Context, probeRoundID, modelGroup string) ([]state.ProbeSampleRecord, error) {
	if _, err := r.Config(); err != nil {
		return nil, err
	}
	store, err := r.state.Store(ctx)
	if err != nil {
		return nil, err
	}
	return store.GetSamplesByProbeRound(probeRoundID, modelGroup), nil
}

// AutoApply executes a background scheduled run respecting interval cooldown.
func (r *Runtime) AutoApply(ctx context.Context) error {
	return r.runAuto(ctx)
}

func (r *Runtime) run(ctx context.Context, trigger Trigger, authIndexes []string) error {
	return r.runTaskRequest(ctx, TaskRequest{
		Trigger:     trigger,
		AuthIndexes: authIndexes,
	})
}

// runTaskRequest executes one scheduling run under the single-flight lock.
// Callers own the trigger semantics; the runner fills in the live config.
func (r *Runtime) runTaskRequest(ctx context.Context, request TaskRequest) error {
	if !r.runMu.TryLock() {
		return ErrRunInProgress
	}
	defer r.runMu.Unlock()

	taskCtx, cleanup, cfg, runner, err := r.taskContext(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	if !cfg.Enabled {
		return errors.New("plugin is disabled")
	}

	request.Config = cfg
	request.AuthIndexes = append([]string(nil), request.AuthIndexes...)
	if err := runner(taskCtx, request); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run %s: %w", request.Trigger, err)
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
	r.mu.Unlock()
	sched := r.state.ScheduleConfig()
	// Guard: skip if plugin is disabled or scheduler is paused
	if !cfg.Enabled || sched.Paused {
		return nil
	}

	taskCtx, cleanup, cfg, runner, err := r.taskContext(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	now := r.clock.Now()
	r.mu.Lock()
	interval := r.cfg.Interval
	r.mu.Unlock()
	plan := r.evaluateAutoSchedule(now.UTC(), interval, true)
	if !plan.ShouldRunNow {
		return nil
	}

	runErr := runner(taskCtx, TaskRequest{
		Config:  cfg,
		Trigger: TriggerAutoApply,
	})
	if runErr != nil {
		if !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, context.DeadlineExceeded) {
			_ = r.state.RecordRunEvent(taskCtx, false, host.RedactBytes([]byte(runErr.Error())), RunHistoryEntry{
				Kind:    KindAutoApply,
				Trigger: string(TriggerAutoApply),
				Message: "auto_apply error: " + host.RedactBytes([]byte(runErr.Error())),
			})
		}
		return fmt.Errorf("run %s: %w", TriggerAutoApply, runErr)
	}

	completedAt := r.clock.Now().UTC()
	if err := r.persistLastAutoApplyAt(taskCtx, completedAt); err != nil {
		return err
	}
	r.resetAdaptiveScheduleState()
	r.signalAutoScheduleWorker()
	return nil
}

// persistLastAutoApplyAt stores the automatic round completion time so a
// restarted Runtime resumes the remaining interval instead of scheduling a
// fresh round from scratch.
func (r *Runtime) persistLastAutoApplyAt(ctx context.Context, at time.Time) error {
	if err := r.state.RecordAutoApplyAt(ctx, at); err != nil {
		return fmt.Errorf("persist auto apply schedule: %w", err)
	}
	return nil
}

func (r *Runtime) signalAutoScheduleWorker() {
	if r.autoApplyWake == nil {
		return
	}
	select {
	case r.autoApplyWake <- struct{}{}:
	default:
	}
}

func (r *Runtime) recordRequestActivity(now time.Time) {
	if store := r.state.StoreOrNil(); store != nil {
		store.RecordRequestActivity(now)
	}
	r.signalAutoScheduleWorker()
}

func (r *Runtime) resetAdaptiveScheduleState() {
	r.scheduleMu.Lock()
	r.scheduleState = adaptiveScheduleState{}
	r.scheduleMu.Unlock()
}

func (r *Runtime) latchRetryDeadline(nextAt time.Time) {
	r.scheduleMu.Lock()
	r.scheduleState.nextRunAt = nextAt.UTC()
	r.scheduleMu.Unlock()
}

func (r *Runtime) evaluateAutoSchedule(now time.Time, interval time.Duration, forDirectAutoCall bool) AdaptiveScheduleSnapshot {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	last := r.state.LastAutoApplyAt()
	store := r.state.StoreOrNil()
	var summary state.RequestActivitySummary
	hasQuotaEvidence := false
	if store != nil {
		burstWindow := min(5*time.Minute, max(time.Minute, interval/3))
		summary = store.SummarizeRequestActivity(now, interval, burstWindow, last)
		hasQuotaEvidence = store.HasQuotaEvidence()
	}

	r.scheduleMu.Lock()
	defer r.scheduleMu.Unlock()
	snap, nextState := evaluateAdaptivePlan(adaptiveEvalInput{
		now:               now,
		interval:          interval,
		lastAutoApplyAt:   last,
		hasQuotaEvidence:  hasQuotaEvidence,
		summary:           summary,
		latched:           r.scheduleState,
		forDirectAutoCall: forDirectAutoCall,
	})
	if !forDirectAutoCall {
		r.scheduleState = nextState
	}
	return snap
}

func (r *Runtime) evaluateWorkerStep(now time.Time, interval time.Duration) (AdaptiveScheduleSnapshot, time.Duration, bool) {
	r.mu.Lock()
	enabled := r.cfg.Enabled && r.cfg.AutoApply
	r.mu.Unlock()
	sched := r.state.ScheduleConfig()
	if !enabled || sched.Paused {
		return AdaptiveScheduleSnapshot{Mode: ScheduleModeIdle}, 0, false
	}
	plan := r.evaluateAutoSchedule(now.UTC(), interval, false)
	if plan.ShouldRunNow {
		return plan, 0, true
	}
	if plan.NextRunAt.IsZero() {
		return plan, 0, false
	}
	return plan, max(plan.NextWait, time.Second), true
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

// LatestSnapshot returns the most recently generated three-pool plan snapshot.
func (r *Runtime) LatestSnapshot(ctx context.Context) (apply.SnapshotSet, error) {
	if _, err := r.Config(); err != nil {
		return apply.SnapshotSet{}, err
	}
	r.mu.Lock()
	snap := r.latestSnapshotSet
	if snap != nil {
		cloned := cloneSnapshotSet(*snap)
		r.mu.Unlock()
		return cloned, nil
	}
	r.mu.Unlock()

	now := r.clock.Now().UTC()
	cfg := r.routingConfigSnapshot()
	models, supportedMap := r.router.CatalogSnapshot()
	out, err := EvaluateRoutingAndSnapshot(RoutingEngineInput{
		EvidenceByGroup:       map[config.AntigravityModelGroup]evidence.Result{},
		Config:                cfg,
		Now:                   now,
		Models:                models,
		SupportedModelsByAuth: supportedMap,
	})
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	r.mu.Lock()
	if r.latestSnapshotSet == nil {
		stored := cloneSnapshotSet(out.Snapshot)
		r.latestSnapshotSet = &stored
	}
	cloned := cloneSnapshotSet(*r.latestSnapshotSet)
	r.mu.Unlock()
	return cloned, nil
}

// Diagnostics returns a comprehensive diagnostics map.
func (r *Runtime) Diagnostics(ctx context.Context) (map[string]any, error) {
	cfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	result, audit := r.currentRunSnapshot()
	r.mu.Lock()
	workerActive := r.worker != nil
	r.mu.Unlock()
	lastAuto := r.state.LastAutoApplyAt()
	sched := r.state.ScheduleConfig()
	nowUTC := r.clock.Now().UTC()
	plan := r.evaluateAutoSchedule(nowUTC, cfg.Interval, false)
	nextWaitStr := ""
	nextRunAtStr := ""
	if cfg.Enabled && cfg.AutoApply && !sched.Paused && !plan.NextRunAt.IsZero() {
		nextWaitStr = plan.NextWait.String()
		nextRunAtStr = plan.NextRunAt.Format(time.RFC3339)
	}
	targetIntervalStr := ""
	if plan.TargetInterval > 0 {
		targetIntervalStr = plan.TargetInterval.String()
	}
	var lastRequestAtStr any
	if !plan.LastRequestAt.IsZero() {
		lastRequestAtStr = plan.LastRequestAt.Format(time.RFC3339)
	}

	emailsByAuthIndex := r.currentSnapshotEmails()
	activeCooldowns := make([]map[string]any, 0)
	var loadErrors []string
	cachePath := r.state.Path()
	if store := r.state.StoreOrNil(); store != nil {
		loadErrors = store.LoadErrors()
		now := nowUTC
		for _, c := range store.GetCooldowns() {
			if now.Before(c.CooldownUntil) {
				entry := map[string]any{
					"auth_index":     redactRuntimeIdentifier(c.AuthIndex),
					"model_group":    c.ModelGroup,
					"model":          c.Model,
					"failure_kind":   c.FailureKind,
					"status_code":    c.StatusCode,
					"backoff_step":   c.BackoffStep,
					"probe_enabled":  c.ProbeEnabled,
					"triggered_at":   c.TriggeredAt.Format(time.RFC3339),
					"cooldown_until": c.CooldownUntil.Format(time.RFC3339),
					"reason":         c.Reason,
				}
				if email := emailsByAuthIndex[c.AuthIndex]; email != "" {
					entry["email"] = email
				}
				activeCooldowns = append(activeCooldowns, entry)
			}
		}
	}
	pending429, rateLimitStorm := r.rateLimitDiagnostics(emailsByAuthIndex)
	requestHistory := r.currentRequestHistory()

	return map[string]any{
		"management_api": map[string]any{
			"status":     "ready",
			"auto_apply": cfg.AutoApply,
			"enabled":    cfg.Enabled,
		},
		"scheduler": map[string]any{
			"interval":             cfg.Interval.String(),
			"mode":                 string(plan.Mode),
			"target_interval":      targetIntervalStr,
			"last_auto_apply_at":   lastAuto,
			"next_wait":            nextWaitStr,
			"next_run_at":          nextRunAtStr,
			"worker_active":        workerActive,
			"paused":               sched.Paused,
			"prev_window_requests": plan.PrevWindowCount,
			"prev_window_rate_rpm": plan.PrevWindowRateRPM,
			"curr_window_requests": plan.CurrWindowCount,
			"curr_window_rate_rpm": plan.CurrWindowRateRPM,
			"requests_since_last":  plan.SinceLastCount,
			"baseline_requests":    plan.BaselineRequests,
			"activity_ratio":       plan.ActivityRatio,
			"last_request_at":      lastRequestAtStr,
		},
		"active_cooldowns":      activeCooldowns,
		"pending_429":           pending429,
		"rate_limit_storm":      rateLimitStorm,
		"latest_audit":          audit,
		"last_result":           result,
		"latest_apply":          r.latestAutoApplyEntry(),
		"run_history":           r.currentRunHistory(),
		"request_history":       requestHistory,
		"request_history_limit": cfg.RequestHistoryLimit,
		"storage":               storageDiagnostics(cachePath, loadErrors),
	}, nil
}

// storageDiagnostics reports the split cache documents backing the runtime.
// Paths stay exactly as configured so operators can correlate them with disk.
func storageDiagnostics(cachePath string, loadErrors []string) map[string]any {
	paths := state.ResolvePaths(cachePath)
	specs := []struct {
		role string
		path string
	}{
		{role: "state", path: paths.Main},
		{role: "runtime", path: paths.Runtime},
		{role: "runs", path: paths.Runs},
		{role: "requests", path: paths.Requests},
	}
	files := make([]map[string]any, 0, len(specs))
	var total int64
	for _, spec := range specs {
		entry := map[string]any{
			"role":   spec.role,
			"path":   spec.path,
			"exists": false,
		}
		info, err := os.Stat(spec.path)
		switch {
		case err == nil:
			entry["exists"] = true
			entry["size_bytes"] = info.Size()
			entry["modified_at"] = info.ModTime().UTC().Format(time.RFC3339)
			total += info.Size()
		case !errors.Is(err, os.ErrNotExist):
			entry["error"] = host.RedactBytes([]byte(err.Error()))
		}
		files = append(files, entry)
	}
	result := map[string]any{
		"files":            files,
		"total_size_bytes": total,
	}
	if len(loadErrors) > 0 {
		result["load_errors"] = append([]string(nil), loadErrors...)
	}
	return result
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
	if err := stopWorker(ctx, worker); err != nil {
		return err
	}
	if store := r.state.StoreOrNil(); store != nil {
		_ = store.SaveRuntimeAtomic(ctx)
	}
	select {
	case <-r.rateLimitDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Quiesce stops scheduled background work ahead of a hot replacement while
// leaving the runtime registered so the host can roll back by re-registering.
func (r *Runtime) Quiesce(ctx context.Context) error {
	r.mu.Lock()
	if r.shutdown {
		r.mu.Unlock()
		return nil
	}
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

	// Routing reads an immutable config snapshot, so replacing the config must
	// discard tables built from the previous values and refresh any cached snapshot.
	r.router.ResetTables()
	r.mu.Lock()
	hasSnapshot := r.latestSnapshotSet != nil
	r.mu.Unlock()
	if hasSnapshot {
		_, _ = r.commitRoutingAndSnapshot(r.clock.Now().UTC(), nil, nil)
	}
	r.trimRequestHistory(ctx)
	r.signalRateLimitWorker()
	r.signalWarmupResetWatcher()
	r.resetAdaptiveScheduleState()
	r.signalAutoScheduleWorker()

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

func (r *Runtime) commitRoutingAndSnapshot(
	now time.Time,
	credentials []core.Credential,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
) (apply.SnapshotSet, error) {
	return r.commitRoutingAndSnapshotWithInventory(now, nil, nil, credentials, evidenceByGroup)
}

func (r *Runtime) commitRoutingAndSnapshotWithInventory(
	now time.Time,
	inventoryFiles []host.AuthFile,
	inventoryModels []host.AuthModelsEntry,
	credentials []core.Credential,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
) (apply.SnapshotSet, error) {
	cfg := r.routingConfigSnapshot()
	store := r.state.StoreOrNil()
	if credentials == nil {
		credentials = r.resolveCurrentCredentials()
	}

	out, err := r.router.EvaluateAndCommit(inventoryFiles, inventoryModels, func(models []apply.ModelOption, supportedMap map[string][]string) (RoutingEngineOutput, error) {
		return EvaluateRoutingAndSnapshot(RoutingEngineInput{
			Credentials:           credentials,
			EvidenceByGroup:       evidenceByGroup,
			Store:                 store,
			Config:                cfg,
			Now:                   now,
			Models:                models,
			SupportedModelsByAuth: supportedMap,
		})
	})
	if err != nil {
		return apply.SnapshotSet{}, err
	}

	stored := cloneSnapshotSet(out.Snapshot)
	r.mu.Lock()
	r.latestSnapshotSet = &stored
	r.mu.Unlock()
	return out.Snapshot, nil
}

func (r *Runtime) syncSnapshotCooldown(authIndex string, candidatesByPool map[string]PrecomputedCandidate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latestSnapshotSet == nil {
		return
	}
	for groupKey, group := range r.latestSnapshotSet.Groups {
		cand, ok := candidatesByPool[groupKey]
		if !ok {
			continue
		}
		for i, item := range group.Items {
			if snapshotAuthIndex(item) == authIndex || item.Name == authIndex || item.Identity.Email == authIndex {
				group.Items[i].Target.Priority = cand.Priority
				group.Items[i].Target.Disabled = cand.Disabled
				group.Items[i].RoutingStatus = cand.RoutingStatus
				eligible := cand.PoolEligible
				group.Items[i].PoolEligible = &eligible
				group.Items[i].Reason = host.RedactBytes([]byte(cand.Reason))
				if !cand.CooldownUntil.IsZero() {
					until := cand.CooldownUntil
					group.Items[i].CooldownUntil = &until
				} else {
					group.Items[i].CooldownUntil = nil
				}
				group.Items[i].IsBoosted = cand.IsBoosted
			}
		}
		r.latestSnapshotSet.Groups[groupKey] = group
	}
}

func (r *Runtime) syncSnapshotWarmup(warmup config.WarmupConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latestSnapshotSet == nil {
		return
	}
	for groupKey, group := range r.latestSnapshotSet.Groups {
		pool := config.AntigravityModelGroup(groupKey)
		for i, item := range group.Items {
			authIdx := snapshotAuthIndex(item)
			_, overridden := warmup.WarmupOverride(authIdx)
			isFree := item.PlanType == string(core.PlanTypeFree)
			group.Items[i].WarmupOverridden = overridden && !isFree
			group.Items[i].WarmupActive = !isFree && warmupEffectiveEnabled(warmup, authIdx, pool)
		}
		r.latestSnapshotSet.Groups[groupKey] = group
	}
}

func (r *Runtime) currentSnapshotEmails() map[string]string {
	r.mu.Lock()
	snapshot := r.latestSnapshotSet
	r.mu.Unlock()
	if snapshot == nil {
		return nil
	}
	emails := make(map[string]string)
	for _, group := range snapshot.Groups {
		for _, item := range group.Items {
			if item.Identity.AuthIndex == "" || item.Identity.Email == "" {
				continue
			}
			emails[item.Identity.AuthIndex] = item.Identity.Email
		}
	}
	return emails
}

func (r *Runtime) setLastProbeOutcomes(outcomes []management.ProbeOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if outcomes == nil {
		r.latestProbeOutcomes = nil
		return
	}
	r.latestProbeOutcomes = append([]management.ProbeOutcome(nil), outcomes...)
}

// LastProbeOutcomes returns the probe outcomes from the most recent probe execution.
func (r *Runtime) LastProbeOutcomes(ctx context.Context) []management.ProbeOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.latestProbeOutcomes) == 0 {
		return nil
	}
	return append([]management.ProbeOutcome(nil), r.latestProbeOutcomes...)
}

// GetScheduleConfig returns the current dynamic schedule configuration.
func (r *Runtime) GetScheduleConfig(ctx context.Context) (config.ScheduleConfig, error) {
	if _, err := r.Config(); err != nil {
		return config.ScheduleConfig{}, err
	}
	return r.state.ScheduleConfig(), nil
}

// SetScheduleConfig updates the dynamic schedule configuration and persists it to the state store.
func (r *Runtime) SetScheduleConfig(ctx context.Context, cfg config.ScheduleConfig) error {
	r.configMu.Lock()
	defer r.configMu.Unlock()
	if _, err := r.Config(); err != nil {
		return err
	}
	if err := r.state.SetScheduleConfig(ctx, cfg); err != nil {
		return fmt.Errorf("persist schedule config: %w", err)
	}
	r.mu.Lock()
	r.cfg.Schedule = cfg
	r.mu.Unlock()
	r.signalAutoScheduleWorker()
	return nil
}

// GetDynamicConfig returns the active dynamic configuration.
func (r *Runtime) GetDynamicConfig(ctx context.Context) (config.DynamicConfig, error) {
	cfg, err := r.Config()
	if err != nil {
		return config.DynamicConfig{}, err
	}
	return r.state.DynamicConfig(cfg), nil
}

// SetDynamicConfig validates, persists, and hot-applies new dynamic configuration without restarting.
func (r *Runtime) SetDynamicConfig(ctx context.Context, dyn config.DynamicConfig) error {
	r.configMu.Lock()
	defer r.configMu.Unlock()
	baseCfg, err := r.Config()
	if err != nil {
		return err
	}
	newCfg, err := dyn.ApplyTo(baseCfg)
	if err != nil {
		return err
	}
	if err := r.state.SetDynamicConfig(ctx, dyn); err != nil {
		return fmt.Errorf("save dynamic config: %w", err)
	}
	return r.replaceConfig(ctx, newCfg)
}

// GetCustomTiers returns the configured custom tiers map.
func (r *Runtime) GetCustomTiers(ctx context.Context) (map[string]int, error) {
	runtimeCfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	return r.state.CustomTiers(runtimeCfg), nil
}

// SetCustomTier updates the custom tier for a credential and recomputes the in-memory routing tables immediately.
func (r *Runtime) SetCustomTier(ctx context.Context, authIndex string, tier int) error {
	cleanIdx := strings.TrimSpace(authIndex)
	if cleanIdx == "" {
		return errors.New("auth_index must not be empty")
	}
	if tier < 0 || tier > 99 {
		return fmt.Errorf("custom tier must be between 1 and 99 (or 0 to reset), got %d", tier)
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	if _, err := r.Config(); err != nil {
		return err
	}
	if err := r.state.SetCustomTier(ctx, cleanIdx, tier); err != nil {
		return fmt.Errorf("persist custom tier: %w", err)
	}

	r.mu.Lock()
	if r.cfg.CustomTiers == nil {
		r.cfg.CustomTiers = make(map[string]int)
	}
	if tier <= 0 {
		delete(r.cfg.CustomTiers, cleanIdx)
	} else {
		r.cfg.CustomTiers[cleanIdx] = tier
	}
	r.mu.Unlock()

	_, _ = r.commitRoutingAndSnapshot(r.clock.Now().UTC(), nil, nil)
	return nil
}

// SetWarmupCredentialState applies one card-icon click. The local state wins
// over the global pool toggle; when the clicked state equals the global state
// the local override is cleared so the credential follows global changes again.
// Turning the credential on clears its persisted 4h30m debounce so it can take
// the next warm-up slot immediately.
func (r *Runtime) SetWarmupCredentialState(ctx context.Context, authIndex, group string, enabled bool) error {
	cleanIdx := strings.TrimSpace(authIndex)
	if cleanIdx == "" {
		return errors.New("auth_index must not be empty")
	}
	modelGroup, err := config.ParseAntigravityModelGroup(group)
	if err != nil {
		return err
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	runtimeCfg, err := r.Config()
	if err != nil {
		return err
	}
	warmupCfg, err := r.state.SetWarmupCredentialState(ctx, runtimeCfg, cleanIdx, modelGroup, enabled)
	if err != nil {
		return fmt.Errorf("persist warm-up switch: %w", err)
	}

	r.mu.Lock()
	r.cfg.Warmup = warmupCfg
	r.mu.Unlock()
	r.syncSnapshotWarmup(warmupCfg)

	r.signalWarmupResetWatcher()
	return nil
}

// SetModelExcluded adds or removes a model from the global Antigravity scheduler
// exclusion list and persists the change to dynamic configuration.
func (r *Runtime) SetModelExcluded(ctx context.Context, model string, excluded bool) ([]string, error) {
	cleanModel := strings.ToLower(strings.TrimSpace(model))
	if cleanModel == "" {
		return nil, errors.New("model must not be empty")
	}
	r.configMu.Lock()
	defer r.configMu.Unlock()
	runtimeCfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	updated, err := r.state.SetModelExcluded(ctx, runtimeCfg, cleanModel, excluded)
	if err != nil {
		return nil, fmt.Errorf("persist excluded model: %w", err)
	}

	r.mu.Lock()
	r.cfg.ExcludedModels = append([]string(nil), updated...)
	r.mu.Unlock()

	return append([]string(nil), updated...), nil
}

func (r *Runtime) currentRunSnapshot() (apply.Result, string) {
	return r.state.RunSnapshot()
}

func (r *Runtime) currentRunHistory() []RunHistoryEntry {
	return r.state.RunHistory()
}

func (r *Runtime) latestAutoApplyEntry() *RunHistoryEntry {
	return r.state.LatestAutoApplyEntry()
}

func registrationResult() RegisterResult {
	return RegisterResult{
		SchemaVersion: 1,
		Metadata:      buildMetadata(),
		Capabilities: map[string]bool{
			"management_api":              true,
			"management":                  true,
			"usage_plugin":                true,
			"scheduler":                   true,
			"scheduler_across_priorities": true,
		},
	}
}

func (r *Runtime) triggerCooldown(ctx context.Context, authIndex, modelGroup, model string) error {
	cfg, err := r.Config()
	if err != nil {
		return err
	}
	cooldownMinutes := cfg.RateLimitCooldownMinutes
	if cooldownMinutes <= 0 {
		cooldownMinutes = config.DefaultRateLimitCooldownMinutes
	}
	return r.triggerFailure(ctx, failureInput{
		AuthIndex:    authIndex,
		ModelGroup:   modelGroup,
		ProbeModel:   model,
		Kind:         state.FailureRateLimit,
		StatusCode:   http.StatusTooManyRequests,
		Reason:       priority.Reason429Cooldown,
		Cooldown:     time.Duration(cooldownMinutes) * time.Minute,
		ProbeEnabled: true,
	})
}

func (r *Runtime) recordCooldownFailure(ctx context.Context, persist bool, authIndex, reason, trigger string, err error) {
	if strings.TrimSpace(trigger) == "" {
		trigger = "health_cooldown"
	}
	redactedErr := host.RedactBytes([]byte(err.Error()))
	message := reason + ": " + redactedErr
	if redactedAuth := redactRuntimeIdentifier(authIndex); redactedAuth != "" {
		message = fmt.Sprintf("%s auth=%s: %s", reason, redactedAuth, redactedErr)
	}
	_ = r.state.RecordRunEvent(ctx, persist, message, RunHistoryEntry{
		Kind:      KindCooldown,
		Trigger:   trigger,
		Attempted: 1,
		Failed:    1,
		Message:   message,
	})
}

func redactRuntimeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 4 {
		return value + "***"
	}
	return value[:4] + "***"
}

func buildMetadata() Metadata {
	return Metadata{
		Name:             "Antigravity Priority",
		Version:          "1.3.3",
		Author:           "ygq-future",
		GitHubRepository: "https://github.com/ygq-future/antigravity-priority",
		Description:      "Intelligent quota pacing and adaptive burn-rate priority scheduler exclusively for Google Antigravity in CLIProxyAPI.",
	}
}

type realRuntimeClock struct{}

func (realRuntimeClock) Now() time.Time {
	return time.Now()
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
