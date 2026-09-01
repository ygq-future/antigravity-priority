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
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

const maxRunHistory = 10

type quotaPreview struct {
	ID              string
	ModelGroup      config.AntigravityModelGroup
	AuthScope       string
	HostFingerprint string
	EvidenceByGroup map[config.AntigravityModelGroup]evidence.Result
}

// Runtime manages plugin lifecycle, configuration, ticker worker, and single-flight execution.
type Runtime struct {
	mu                 sync.Mutex
	runMu              sync.Mutex
	tickerFactory      TickerFactory
	runner             TaskRunner
	rootCtx            context.Context
	cancel             context.CancelFunc
	cfg                config.Config
	hostCallbacks      host.HostCallbacks
	clock              Clock
	sleeper            Sleeper
	management         *management.Handler
	latestResult       apply.Result
	latestAudit        string
	latestDualSnapshot *apply.DualGroupSnapshot
	latestQuotaPreview *quotaPreview
	scheduleConfig     state.ScheduleConfig
	stateCacheOverride string
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
	case MethodPluginRegister:
		parsed, err := decodeRegisterRequest(request)
		if err != nil {
			return failure(err)
		}
		result, err := r.Register(ctx, parsed)
		return envelopeRegister(result, err)
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
	case MethodFilterResponse, MethodFilterComplete, MethodFilterError, MethodFilterOutbound, MethodFilterInbound:
		return r.handleFilterEvent(ctx, request)
	default:
		return failure(fmt.Errorf("%w: method %q", ErrInvalidRequest, method))
	}
}

// Register initializes the plugin with configuration received from CPA and starts scheduled workers.
func (r *Runtime) Register(ctx context.Context, req RegisterRequest) (RegisterResult, error) {
	cfg, _, err := config.LoadBytes([]byte(req.ConfigYAML))
	if err != nil {
		return RegisterResult{}, fmt.Errorf("load register config: %w", err)
	}
	cfg = r.applyStateCacheOverride(cfg)
	cfg = r.mergePersistedDynamicConfig(cfg)
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
	cfg = r.applyStateCacheOverride(cfg)
	cfg = r.mergePersistedDynamicConfig(cfg)
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

// ManualApply triggers an immediate priority calculation and host write-back.
func (r *Runtime) ManualApply(ctx context.Context, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	previewID := ""
	if preview := r.currentQuotaPreview(); preview != nil {
		previewID = preview.ID
	}
	return r.runWithPreview(ctx, TriggerManualApply, modelGroup, authIndexes, previewID, true)
}

// ManualApplyWithPreview commits the quota preview identified by previewID
// without issuing another quota request.
func (r *Runtime) ManualApplyWithPreview(ctx context.Context, modelGroup config.AntigravityModelGroup, authIndexes []string, previewID string) error {
	return r.runWithPreview(ctx, TriggerManualApply, modelGroup, authIndexes, previewID, true)
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
	r.mu.Lock()
	cfg := r.cfg
	r.mu.Unlock()
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

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return nil, err
	}
	evidenceByGroup := buildProjectionEvidence(store, credentials)
	now := r.clock.Now().UTC()
	projection, err := ProjectDualModelGroups(ProjectionInput{
		ControlModelGroup: cfg.AntigravityModelGroup,
		Credentials:       credentials,
		EvidenceByGroup:   evidenceByGroup,
		PlanningOptions:   priorityOptions(cfg, store, now),
		ProjectionTime:    now,
	})
	if err != nil {
		return nil, err
	}
	primarySnapshot := projection.ControlSnapshot
	r.clearQuotaPreview()
	r.setDualSnapshot(projection.Snapshot)

	summary := resultSummary("reset", result)
	result.Snapshot = primarySnapshot
	snap := primarySnapshot
	_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
		Kind:     KindReset,
		Trigger:  string(TriggerManualApply),
		Message:  summary,
		Snapshot: &snap,
	})
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

// SyncHost re-reads credentials from CPA host, re-evaluates cached evidence, and updates the dual-group snapshot.
func (r *Runtime) SyncHost(ctx context.Context, modelGroup config.AntigravityModelGroup) (apply.DualGroupSnapshot, error) {
	if !r.runMu.TryLock() {
		return apply.DualGroupSnapshot{}, ErrRunInProgress
	}
	defer r.runMu.Unlock()

	r.mu.Lock()
	cfg := r.cfg
	r.mu.Unlock()

	if r.hostCallbacks == nil {
		return apply.DualGroupSnapshot{}, errMissingHostCallbacks
	}
	// The dashboard selector is view-only; Dynamic Config is the control authority.
	_ = modelGroup

	client := host.NewClient(r.hostCallbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return apply.DualGroupSnapshot{}, err
	}

	credentials := credentialsFromAuthFiles(files)
	credentials, _, err = enrichCredentialsFromAuthDocuments(ctx, client, credentials)
	if err != nil {
		return apply.DualGroupSnapshot{}, err
	}

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return apply.DualGroupSnapshot{}, err
	}
	evidenceByGroup := buildProjectionEvidence(store, credentials)

	now := r.clock.Now().UTC()
	projection, err := ProjectDualModelGroups(ProjectionInput{
		ControlModelGroup: cfg.AntigravityModelGroup,
		Credentials:       credentials,
		EvidenceByGroup:   evidenceByGroup,
		PlanningOptions:   priorityOptions(cfg, store, now),
		ProjectionTime:    now,
	})
	if err != nil {
		return apply.DualGroupSnapshot{}, err
	}
	// Host synchronization only refreshes the projection. It must not consume
	// an unused Fresh Evidence preview; ManualApplyWithPreview still validates
	// the preview's host fingerprint before any transition is executed.
	if preview := r.currentQuotaPreview(); preview != nil {
		projection.Snapshot.PreviewID = preview.ID
	}
	r.setDualSnapshot(projection.Snapshot)
	return cloneDualGroupSnapshot(projection.Snapshot), nil
}

// GetSamples returns the historical quota samples for a specific credential and model group.
func (r *Runtime) GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error) {
	r.mu.Lock()
	cfg := r.cfg
	r.mu.Unlock()

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return nil, err
	}
	return store.GetSamples(authIndex, modelGroup), nil
}

// GetProbeSamples returns only samples appended by one probe round.
func (r *Runtime) GetProbeSamples(ctx context.Context, probeRoundID, modelGroup string) ([]state.ProbeSampleRecord, error) {
	r.mu.Lock()
	cfg := r.cfg
	r.mu.Unlock()

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return nil, err
	}
	return store.GetSamplesByProbeRound(probeRoundID, modelGroup), nil
}

// AutoApply executes a background scheduled run respecting interval cooldown.
func (r *Runtime) AutoApply(ctx context.Context) error {
	return r.runAuto(ctx)
}

func (r *Runtime) run(ctx context.Context, trigger Trigger, modelGroup config.AntigravityModelGroup, authIndexes []string) error {
	return r.runWithPreview(ctx, trigger, modelGroup, authIndexes, "", false)
}

func (r *Runtime) runWithPreview(ctx context.Context, trigger Trigger, modelGroup config.AntigravityModelGroup, authIndexes []string, previewID string, previewRequired bool) error {
	if !r.runMu.TryLock() {
		return ErrRunInProgress
	}
	defer r.runMu.Unlock()

	taskCtx, cleanup, cfg, runner, err := r.taskContext(ctx)
	if err != nil {
		return err
	}
	defer cleanup()

	if !cfg.Enabled && trigger != TriggerManualApply {
		return errors.New("plugin is disabled")
	}

	if err := runner(taskCtx, TaskRequest{
		Config:          cfg,
		Trigger:         trigger,
		AuthIndexes:     append([]string(nil), authIndexes...),
		PreviewID:       strings.TrimSpace(previewID),
		PreviewRequired: previewRequired,
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

	// Schedule window values are user-facing local wall-clock times. Keep the
	// clock's location here so the backend evaluates the same local time shown
	// by the management UI; persisted timestamps remain UTC elsewhere.
	now := r.clock.Now()

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
				Kind:    KindAutoApply,
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
		return cloneDualGroupSnapshot(*snap), nil
	}
	// Startup fallback: generate the stable empty shape through the same
	// projection seam until a shared projection is available.
	cfg, _ := r.Config()
	now := r.clock.Now().UTC()
	projection, err := ProjectDualModelGroups(ProjectionInput{
		ControlModelGroup: cfg.AntigravityModelGroup,
		ProjectionTime:    now,
	})
	if err != nil {
		return apply.DualGroupSnapshot{}, err
	}
	r.setDualSnapshot(projection.Snapshot)
	return cloneDualGroupSnapshot(projection.Snapshot), nil
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
	sched := r.scheduleConfig
	r.mu.Unlock()
	nextWait := r.nextAutoApplyWait(cfg.Interval)
	nextRunAt := r.clock.Now().UTC().Add(nextWait)

	activeCooldowns := make([]map[string]any, 0)
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(ctx, cachePath); err == nil {
		now := r.clock.Now().UTC()
		for _, c := range store.GetCooldowns() {
			if now.Before(c.CooldownUntil) {
				activeCooldowns = append(activeCooldowns, map[string]any{
					"auth_index":     redactRuntimeIdentifier(c.AuthIndex),
					"model_group":    c.ModelGroup,
					"triggered_at":   c.TriggeredAt.Format(time.RFC3339),
					"cooldown_until": c.CooldownUntil.Format(time.RFC3339),
					"reason":         c.Reason,
				})
			}
		}
	}

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
		"active_cooldowns": activeCooldowns,
		"latest_audit":     audit,
		"last_result":      result,
		"latest_apply":     r.latestApplyEntry(),
		"run_history":      r.currentRunHistory(),
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
		entry.Kind = KindApply
	}
	history := make([]RunHistoryEntry, 0, maxRunHistory)
	history = append(history, entry)
	for i := 0; i < len(r.runHistory) && len(history) < maxRunHistory; i++ {
		history = append(history, r.runHistory[i])
	}
	r.runHistory = history
}

func (r *Runtime) snapshotLatestResult(result apply.Result, audit string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latestResult = result
	r.latestAudit = audit
}

func (r *Runtime) setDualSnapshot(snap apply.DualGroupSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := cloneDualGroupSnapshot(snap)
	r.latestDualSnapshot = &cloned
}

func (r *Runtime) currentSnapshotEmails() map[string]string {
	r.mu.Lock()
	snapshot := r.latestDualSnapshot
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

func (r *Runtime) setQuotaPreview(preview quotaPreview) {
	cloned := cloneQuotaPreview(preview)
	r.mu.Lock()
	r.latestQuotaPreview = &cloned
	r.mu.Unlock()
}

func (r *Runtime) currentQuotaPreview() *quotaPreview {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latestQuotaPreview == nil {
		return nil
	}
	cloned := cloneQuotaPreview(*r.latestQuotaPreview)
	return &cloned
}

func (r *Runtime) clearQuotaPreview() {
	r.mu.Lock()
	r.latestQuotaPreview = nil
	r.mu.Unlock()
}

func cloneQuotaPreview(preview quotaPreview) quotaPreview {
	cloned := preview
	cloned.EvidenceByGroup = make(map[config.AntigravityModelGroup]evidence.Result, len(preview.EvidenceByGroup))
	for group, result := range preview.EvidenceByGroup {
		cloned.EvidenceByGroup[group] = cloneEvidence(result)
	}
	return cloned
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

func (r *Runtime) latestApplyEntry() *RunHistoryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.runHistory {
		if entry.Kind == KindApply || entry.Kind == KindAutoApply {
			copy := entry
			return &copy
		}
	}
	return nil
}

func registrationResult() RegisterResult {
	return RegisterResult{
		SchemaVersion: 1,
		Metadata:      buildMetadata(),
		Capabilities: map[string]bool{
			"management_api": true,
			"management":     true,
			"usage_plugin":   true,
		},
	}
}

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type usageEvent struct {
	Provider  string       `json:"Provider"`
	Model     string       `json:"Model"`
	AuthID    string       `json:"AuthID"`
	AuthIndex string       `json:"AuthIndex"`
	Failed    bool         `json:"Failed"`
	Failure   usageFailure `json:"Failure"`
}

func (r *Runtime) handleUsageEvent(ctx context.Context, raw []byte) []byte {
	var payload usageEvent
	if err := json.Unmarshal(raw, &payload); err != nil {
		return failure(fmt.Errorf("decode usage event: %w", err))
	}
	if !strings.EqualFold(strings.TrimSpace(payload.Provider), string(core.ProviderAntigravity)) ||
		!payload.Failed || payload.Failure.StatusCode != 429 {
		return mustMarshal(Envelope{OK: true})
	}
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthID)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}

	r.runMu.Lock()
	err := r.triggerCooldown(ctx, authIndex, modelGroupForUsage(payload.Model), "429 rate limit detected")
	r.runMu.Unlock()
	if err != nil {
		return failure(err)
	}
	return mustMarshal(Envelope{OK: true})
}

func modelGroupForUsage(model string) string {
	if strings.Contains(strings.ToLower(model), "gemini") {
		return string(config.AntigravityModelGroupGemini)
	}
	return string(config.AntigravityModelGroupClaudeGPT)
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
		r.runMu.Lock()
		err := r.triggerCooldown(ctx, authIndex, payload.ModelGroup, "429 rate limit detected")
		r.runMu.Unlock()
		if err != nil {
			return failure(err)
		}
	}

	return mustMarshal(Envelope{OK: true})
}

func (r *Runtime) triggerCooldown(ctx context.Context, authIndex, modelGroup, reason string) error {
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
		r.recordCooldownFailure(ctx, nil, authIndex, reason, err)
		return fmt.Errorf("record cooldown state: %w", err)
	}
	if _, active := store.GetActiveCooldowns(now)[authIndex]; active {
		return nil
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
	if err := store.SaveAtomic(ctx); err != nil {
		r.recordCooldownFailure(ctx, store, authIndex, reason, err)
		return fmt.Errorf("persist cooldown state: %w", err)
	}
	if r.hostCallbacks == nil {
		err := errMissingHostCallbacks
		r.recordCooldownFailure(ctx, store, authIndex, reason, err)
		return err
	}

	client := host.NewClient(r.hostCallbacks)
	credential := core.Credential{
		Name:      authIndex,
		AuthIndex: authIndex,
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
	}
	files, listErr := client.ListAuthFiles(ctx)
	if listErr != nil {
		r.recordCooldownFailure(ctx, store, authIndex, reason, listErr)
		return fmt.Errorf("synchronize cooldown credential: %w", listErr)
	}
	found := false
	for _, candidate := range credentialsFromAuthFiles(files) {
		if candidate.AuthIndex == authIndex || candidate.Name == authIndex {
			credential = candidate
			found = true
			break
		}
	}
	if !found {
		err := errors.New("cooldown credential not found in Host inventory")
		r.recordCooldownFailure(ctx, store, authIndex, reason, err)
		return err
	}
	if enriched, _, enrichErr := enrichCredentialsFromAuthDocuments(ctx, client, []core.Credential{credential}); enrichErr != nil {
		r.recordCooldownFailure(ctx, store, authIndex, reason, enrichErr)
		return fmt.Errorf("synchronize cooldown Host state: %w", enrichErr)
	} else if len(enriched) == 1 {
		credential = enriched[0]
	}
	transition := apply.NewHostTransition(client)
	result, applyErr := apply.ExecuteRound(ctx, transition, apply.TransitionRound{
		Intents: []apply.TransitionIntent{apply.CooldownIntent(credential, reason)},
	})
	result.Snapshot = apply.Snapshot(priority.Plan{
		DecidedAt: now,
		Items: []priority.PlanItem{{
			Credential: credential,
			Priority:   priority.DepletedPriority,
			Disabled:   false,
			Reason:     priority.Reason429Cooldown,
		}},
	})
	if applyErr != nil {
		r.recordCooldownFailure(ctx, store, authIndex, reason, applyErr)
		return applyErr
	}
	summary := resultSummary("429 cooldown", result)
	_, projectErr := r.projectRun(ctx, store, result, summary, RunHistoryEntry{
		Kind:     KindCooldown,
		Trigger:  "rate_limit_429",
		Message:  summary,
		Snapshot: &result.Snapshot,
	})
	if projectErr != nil {
		return projectErr
	}
	if result.Transitions.Totals.Failed > 0 || result.Transitions.Totals.Conflicts > 0 || result.Transitions.Totals.Uncertain > 0 {
		return errors.New("429 cooldown host mutation failed")
	}
	return nil
}

func (r *Runtime) recordCooldownFailure(ctx context.Context, store *state.Store, authIndex, reason string, err error) {
	message := reason + ": " + host.RedactBytes([]byte(err.Error()))
	result := apply.ResultFromTransition(apply.TransitionRoundResult{Details: []apply.TransitionResult{{
		AuthIndex: redactRuntimeIdentifier(authIndex),
		Outcome:   apply.OutcomeFailed,
		Reason:    apply.ReasonCommitFailed,
		Cause:     reason,
		Error:     host.RedactBytes([]byte(err.Error())),
	}}})
	entry := RunHistoryEntry{
		Kind:      KindCooldown,
		Trigger:   "rate_limit_429",
		Attempted: result.Attempted,
		Succeeded: result.Succeeded,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
		NoChange:  result.NoChange,
		Conflicts: result.Conflicts,
		Uncertain: result.Uncertain,
		Message:   message,
	}
	if store == nil {
		r.snapshotRunEntry(result, message, entry)
		return
	}
	_, _ = r.projectRun(ctx, store, result, message, entry)
}

func redactRuntimeIdentifier(value string) string {
	if len(value) <= 4 {
		return "***"
	}
	return value[:2] + "***" + value[len(value)-2:]
}

func buildMetadata() Metadata {
	return Metadata{
		Name:             "Antigravity Priority",
		Version:          "1.2.12",
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
