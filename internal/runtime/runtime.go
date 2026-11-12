package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

const maxRunHistory = 10

// Runtime manages plugin lifecycle, configuration, ticker worker, and single-flight execution.
type Runtime struct {
	mu                      sync.Mutex
	runMu                   sync.Mutex
	runner                  TaskRunner
	rootCtx                 context.Context
	cancel                  context.CancelFunc
	cfg                     config.Config
	hostCallbacks           host.HostCallbacks
	clock                   Clock
	sleeper                 Sleeper
	management              *management.Handler
	latestResult            apply.Result
	latestAudit             string
	latestSnapshotSet       *apply.SnapshotSet
	scheduleConfig          state.ScheduleConfig
	stateCacheOverride      string
	runHistory              []RunHistoryEntry
	requestHistory          []state.RequestHistoryEntry
	requestHistoryPersistMu sync.Mutex
	latestProbeOutcomes     []management.ProbeOutcome
	lastAutoApplyAt         time.Time
	worker                  *tickerWorker
	shutdown                bool
	rateLimitMu             sync.Mutex
	pending429              map[string]pending429
	stormUntil              time.Time
	stormAccounts           map[string]bool
	activeCooldowns         map[string]state.CooldownEntry
	rateLimitWake           chan struct{}
	rateLimitDone           chan struct{}
	routingMu               sync.RWMutex
	plansByPool             map[config.AntigravityModelGroup]PoolRoutingTable
	authIDToIndex           map[string]string
	supportedModelsByAuth   map[string]map[string]struct{}
	modelCatalog            []ModelCatalogItem
	rrCursorByPool          map[config.AntigravityModelGroup]string
	sessionAffinity         map[string]sessionBinding
	probeMu           sync.Mutex
	probeAttempts           map[string]probeAttempt
	warmupResetWake         chan struct{}
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
		rootCtx:               ctx,
		cancel:                cancel,
		cfg:                   config.Default(),
		hostCallbacks:         options.Host,
		clock:                 clock,
		sleeper:               sleeper,
		pending429:            make(map[string]pending429),
		stormAccounts:         make(map[string]bool),
		activeCooldowns:       make(map[string]state.CooldownEntry),
		rateLimitWake:         make(chan struct{}, 1),
		rateLimitDone:         make(chan struct{}),
		plansByPool:           make(map[config.AntigravityModelGroup]PoolRoutingTable),
		authIDToIndex:         make(map[string]string),
		supportedModelsByAuth: make(map[string]map[string]struct{}),
		modelCatalog:          append([]ModelCatalogItem(nil), builtInAntigravityModels...),
		rrCursorByPool:        make(map[config.AntigravityModelGroup]string),
		sessionAffinity:       make(map[string]sessionBinding),
		probeAttempts:         make(map[string]probeAttempt),
		warmupResetWake:       make(chan struct{}, 1),
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
		maps.Copy(rt.activeCooldowns, store.GetCooldowns())
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
		if reqHist := store.GetRequestHistory(); len(reqHist) > 0 {
			rt.requestHistory = reqHist
		}
		if audit != "" {
			rt.latestAudit = audit
		}
		schedCfg := store.GetScheduleConfig()
		rt.scheduleConfig = schedCfg
		rt.lastAutoApplyAt = store.GetLastAutoApplyAt()
		if dynCfg, ok := store.GetDynamicConfig(); ok {
			if merged, err := dynCfg.ApplyTo(rt.cfg); err == nil {
				merged.Schedule = schedCfg
				rt.cfg = merged
				rt.scheduleConfig = schedCfg
			}
		}
		historyLimit := rt.cfg.RequestHistoryLimit
		if historyLimit <= 0 {
			historyLimit = state.DefaultMaxRequestHistory
		}
		if len(rt.requestHistory) > historyLimit {
			rt.requestHistory = rt.requestHistory[:historyLimit]
			store.SetRequestHistory(rt.requestHistory)
		}
		// Unconditionally ensure cache file exists on disk upon initialization
		_ = store.SaveAtomic(context.Background())
	}
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
	credentials = enrichCredentialsPlanTypeFromStore(store, credentials)
	evidenceByGroup := buildProjectionEvidence(store, credentials)
	now := r.clock.Now().UTC()
	projection, err := ProjectModelGroups(ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: evidenceByGroup,
		PlanningOptions: priorityOptions(cfg, store, now),
		ProjectionTime:  now,
		Warmup:          cfg.Warmup,
	})
	if err != nil {
		return nil, err
	}
	// Reset execution details still render a single plan snapshot. Use the
	// Gemini pool as the stable legacy view; the three-pool SnapshotSet remains
	// the authoritative snapshot for every other consumer.
	geminiPool := projection.Snapshot.Groups[string(config.AntigravityModelGroupGemini)]
	legacySnapshot := apply.PlanSnapshot{
		DecidedAt:  projection.Snapshot.ObservedAt,
		TotalItems: len(geminiPool.Items),
		Items:      geminiPool.Items,
	}
	r.setSnapshotSet(projection.Snapshot)

	summary := resultSummary("reset", result)
	result.Snapshot = legacySnapshot
	snap := legacySnapshot
	_, projectErr := r.projectRun(ctx, store, result, summary, withTransitionStats(result, RunHistoryEntry{
		Kind:     KindReset,
		Trigger:  string(TriggerManualReset),
		Message:  summary,
		Snapshot: &snap,
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

	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(now, credentials)
	r.routingMu.Unlock()

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

// SyncHost re-reads credentials from CPA host, re-evaluates cached evidence, and updates the three-pool snapshot.
func (r *Runtime) SyncHost(ctx context.Context) (apply.SnapshotSet, error) {
	if !r.runMu.TryLock() {
		return apply.SnapshotSet{}, ErrRunInProgress
	}
	defer r.runMu.Unlock()

	r.mu.Lock()
	cfg := r.cfg
	r.mu.Unlock()

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

	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	credentials = enrichCredentialsPlanTypeFromStore(store, credentials)
	evidenceByGroup := buildProjectionEvidence(store, credentials)

	now := r.clock.Now().UTC()
	projection, err := ProjectModelGroups(ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: evidenceByGroup,
		PlanningOptions: priorityOptions(cfg, store, now),
		ProjectionTime:  now,
		Warmup:          cfg.Warmup,
	})
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	authModels, _ := client.ListAuthModels(ctx)
	r.routingMu.Lock()
	r.updateModelInventoryLocked(files, authModels)
	r.recomputeRoutingTablesLocked(now)
	r.routingMu.Unlock()
	r.setSnapshotSet(projection.Snapshot)
	return r.LatestSnapshot(ctx)
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

	completedAt := r.clock.Now().UTC()
	r.mu.Lock()
	r.lastAutoApplyAt = completedAt
	r.mu.Unlock()
	if err := r.persistLastAutoApplyAt(taskCtx, cfg, completedAt); err != nil {
		return err
	}
	return nil
}

// persistLastAutoApplyAt stores the automatic round completion time so a
// restarted Runtime resumes the remaining interval instead of scheduling a
// fresh round from scratch.
func (r *Runtime) persistLastAutoApplyAt(ctx context.Context, cfg config.Config, at time.Time) error {
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return fmt.Errorf("load state for auto apply schedule: %w", err)
	}
	store.SetLastAutoApplyAt(at)
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
		return fmt.Errorf("persist auto apply schedule: %w", err)
	}
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
	// A persisted timestamp ahead of the local clock (manual clock correction)
	// must not postpone scheduling for longer than one interval.
	remaining = min(remaining, interval)
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

// LatestSnapshot returns the most recently generated three-pool plan snapshot.
func (r *Runtime) LatestSnapshot(ctx context.Context) (apply.SnapshotSet, error) {
	if _, err := r.Config(); err != nil {
		return apply.SnapshotSet{}, err
	}
	r.mu.Lock()
	snap := r.latestSnapshotSet
	if snap != nil {
		r.enrichSnapshotWithModelsLocked(snap)
		cloned := cloneSnapshotSet(*snap)
		r.mu.Unlock()
		return cloned, nil
	}
	r.mu.Unlock()
	// projection seam until a shared projection is available.
	now := r.clock.Now().UTC()
	projection, err := ProjectModelGroups(ProjectionInput{
		ProjectionTime: now,
	})
	if err != nil {
		return apply.SnapshotSet{}, err
	}
	r.setSnapshotSet(projection.Snapshot)
	return cloneSnapshotSet(projection.Snapshot), nil
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

	emailsByAuthIndex := r.currentSnapshotEmails()
	activeCooldowns := make([]map[string]any, 0)
	var loadErrors []string
	cachePath := cfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(ctx, cachePath); err == nil {
		loadErrors = store.LoadErrors()
		now := r.clock.Now().UTC()
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
	// discard tables built from the previous values. The next pick rebuilds
	// them with the new rules instead of waiting for the 60-second TTL.
	r.routingMu.Lock()
	r.plansByPool = make(map[config.AntigravityModelGroup]PoolRoutingTable)
	r.routingMu.Unlock()

	r.trimRequestHistory(ctx)
	r.signalRateLimitWorker()
	r.signalWarmupResetWatcher()

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

func (r *Runtime) snapshotRunEntry(result apply.Result, audit string, entry RunHistoryEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.latestResult = result
	r.latestAudit = audit
	if entry.At.IsZero() {
		entry.At = r.clock.Now().UTC()
	}
	history := make([]RunHistoryEntry, 0, maxRunHistory)
	history = append(history, entry)
	for i := 0; i < len(r.runHistory) && len(history) < maxRunHistory; i++ {
		history = append(history, r.runHistory[i])
	}
	r.runHistory = history
}

func (r *Runtime) enrichSnapshotWithModelsLocked(snap *apply.SnapshotSet) {
	if snap == nil {
		return
	}
	r.routingMu.RLock()
	catalog := r.modelCatalog
	if len(catalog) == 0 {
		catalog = builtInAntigravityModels
	}
	models := make([]apply.ModelOption, len(catalog))
	for i, item := range catalog {
		models[i] = apply.ModelOption{
			ID:          item.ID,
			DisplayName: item.DisplayName,
			QuotaPool:   string(item.QuotaPool),
		}
	}
	supportedMap := make(map[string][]string, len(r.supportedModelsByAuth))
	for authIdx, set := range r.supportedModelsByAuth {
		if len(set) > 0 {
			supported := make([]string, 0, len(set))
			for m := range set {
				supported = append(supported, m)
			}
			sort.Strings(supported)
			supportedMap[authIdx] = supported
		}
	}
	r.routingMu.RUnlock()

	snap.Models = models
	for groupKey, group := range snap.Groups {
		for i, item := range group.Items {
			// Supported-model lookup keys come from the Host inventory and are
			// unredacted; the display AuthIndex may be truncated and collide.
			authIdx := snapshotAuthIndex(item)
			if supported, ok := supportedMap[authIdx]; ok && len(supported) > 0 {
				group.Items[i].SupportedModels = supported
			}
		}
		snap.Groups[groupKey] = group
	}
}

func (r *Runtime) setSnapshotSet(snap apply.SnapshotSet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := cloneSnapshotSet(snap)
	r.enrichSnapshotWithModelsLocked(&cloned)
	r.latestSnapshotSet = &cloned
}

func (r *Runtime) patchSnapshotCooldown(authIndex string, inCooldown bool, reason string, restoredPriority int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.latestSnapshotSet == nil {
		return
	}
	for groupKey, group := range r.latestSnapshotSet.Groups {
		for i, item := range group.Items {
			if item.Identity.AuthIndex == authIndex || item.Name == authIndex || item.Identity.Email == authIndex || item.Email == authIndex || strings.EqualFold(item.AuthIndex, authIndex) {
				if inCooldown {
					group.Items[i].Host.Priority = priority.DepletedPriority
					group.Items[i].Target.Priority = priority.DepletedPriority
					group.Items[i].Reason = reason
					group.Items[i].IsBoosted = false
				} else {
					group.Items[i].Host.Priority = restoredPriority
					group.Items[i].Target.Priority = restoredPriority
					group.Items[i].Reason = reason
				}
			}
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
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
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
			dyn.Schedule = sched
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
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
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
	r.mu.Lock()
	sched := r.scheduleConfig
	r.mu.Unlock()
	storeSched := store.GetScheduleConfig()
	if storeSched != (config.ScheduleConfig{}) {
		sched = storeSched
	}
	merged.Schedule = sched
	r.mu.Lock()
	r.scheduleConfig = sched
	r.mu.Unlock()
	return merged
}

// GetCustomTiers returns the configured custom tiers map.
func (r *Runtime) GetCustomTiers(ctx context.Context) (map[string]int, error) {
	runtimeCfg, err := r.Config()
	if err != nil {
		return nil, err
	}
	cachePath := runtimeCfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err == nil {
		if tiers := store.GetCustomTiers(); len(tiers) > 0 {
			return tiers, nil
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cfg.CustomTiers) > 0 {
		cloned := make(map[string]int, len(r.cfg.CustomTiers))
		maps.Copy(cloned, r.cfg.CustomTiers)
		return cloned, nil
	}
	return map[string]int{}, nil
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
		return fmt.Errorf("load state for custom tier: %w", err)
	}
	store.SetCustomTier(cleanIdx, tier)
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
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
	r.latestSnapshotSet = nil
	r.mu.Unlock()

	r.routingMu.Lock()
	r.recomputeRoutingTablesLocked(r.clock.Now().UTC())
	r.routingMu.Unlock()

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
	runtimeCfg, err := r.Config()
	if err != nil {
		return err
	}
	dyn, err := r.GetDynamicConfig(ctx)
	if err != nil {
		return err
	}
	dyn.Warmup.Normalize()
	globalEnabled := warmupEnabledForGroup(dyn.Warmup, modelGroup)
	dyn.Warmup.ClearWarmupOverride(cleanIdx)
	if enabled != globalEnabled {
		dyn.Warmup.SetWarmupOverride(cleanIdx, enabled)
	}
	if err := dyn.Validate(); err != nil {
		return err
	}

	cachePath := runtimeCfg.StateCachePath
	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		return fmt.Errorf("load state for warm-up switch: %w", err)
	}
	store.SetDynamicConfig(dyn)
	if err := store.SaveRuntimeAtomic(ctx); err != nil {
		return fmt.Errorf("persist warm-up switch: %w", err)
	}
	if enabled {
		if store.ClearWarmupCompleted(cleanIdx) > 0 {
			if err := store.SaveStateAtomic(ctx); err != nil {
				return fmt.Errorf("clear warm-up debounce: %w", err)
			}
		}
	}

	r.mu.Lock()
	r.cfg.Warmup = dyn.Warmup.Clone()
	r.latestSnapshotSet = nil
	r.mu.Unlock()

	r.signalWarmupResetWatcher()
	return nil
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

func (r *Runtime) latestAutoApplyEntry() *RunHistoryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.runHistory {
		if entry.Kind == KindAutoApply {
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
			"management_api":              true,
			"management":                  true,
			"usage_plugin":                true,
			"scheduler":                   true,
			"scheduler_across_priorities": true,
		},
	}
}

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type usageDetail struct {
	InputTokens         int64 `json:"InputTokens"`
	OutputTokens        int64 `json:"OutputTokens"`
	ReasoningTokens     int64 `json:"ReasoningTokens"`
	CachedTokens        int64 `json:"CachedTokens"`
	CacheReadTokens     int64 `json:"CacheReadTokens"`
	CacheCreationTokens int64 `json:"CacheCreationTokens"`
	TotalTokens         int64 `json:"TotalTokens"`
}

type usageEvent struct {
	Provider  string       `json:"Provider"`
	Model     string       `json:"Model"`
	AuthID    string       `json:"AuthID"`
	AuthIndex string       `json:"AuthIndex"`
	Failed    bool         `json:"Failed"`
	Failure   usageFailure `json:"Failure"`
	Detail    usageDetail  `json:"Detail"`
}

func (r *Runtime) handleUsageEvent(ctx context.Context, raw []byte) []byte {
	var payload usageEvent
	if err := json.Unmarshal(raw, &payload); err != nil {
		return failure(fmt.Errorf("decode usage event: %w", err))
	}
	if !isAntigravityUsageProvider(payload.Provider) {
		return mustMarshal(Envelope{OK: true})
	}
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthID)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}
	resolvedIndex := r.resolveAuthIndex(authIndex, nil)
	email := r.resolveAuthEmail(resolvedIndex)
	pool := config.ClassifyModelQuotaPool(payload.Model)
	reasoningTokens := payload.Detail.ReasoningTokens
	cacheTokens := payload.Detail.CacheReadTokens
	if cacheTokens == 0 {
		cacheTokens = payload.Detail.CachedTokens
	}
	totalTokens := payload.Detail.TotalTokens
	if totalTokens == 0 {
		totalTokens = payload.Detail.InputTokens + payload.Detail.OutputTokens + reasoningTokens + cacheTokens
	} else if reasoningTokens == 0 && cacheTokens == 0 {
		diff := totalTokens - (payload.Detail.InputTokens + payload.Detail.OutputTokens)
		if diff > 0 {
			reasoningTokens = diff
		}
	}
	kind := state.FailureKind("")
	if payload.Failed {
		kind = failureKindFor(payload.Failure.StatusCode, payload.Failure.Body)
	}
	statusCode := 200
	if payload.Failed {
		switch {
		case payload.Failure.StatusCode != 0:
			statusCode = payload.Failure.StatusCode
		case kind == state.FailureUnauthorized:
			statusCode = http.StatusUnauthorized
		case kind == state.FailureForbidden:
			statusCode = http.StatusForbidden
		case kind == state.FailureRateLimit:
			statusCode = http.StatusTooManyRequests
		default:
			statusCode = http.StatusInternalServerError
		}
	}
	r.appendRequestHistory(state.RequestHistoryEntry{
		ID:                  fmt.Sprintf("req-%d", r.clock.Now().UTC().UnixNano()),
		At:                  r.clock.Now().UTC(),
		Model:               payload.Model,
		ModelPool:           string(pool),
		AuthIndex:           resolvedIndex,
		Email:               email,
		InputTokens:         payload.Detail.InputTokens,
		OutputTokens:        payload.Detail.OutputTokens,
		ReasoningTokens:     reasoningTokens,
		CachedTokens:        payload.Detail.CachedTokens,
		CacheReadTokens:     payload.Detail.CacheReadTokens,
		CacheCreationTokens: payload.Detail.CacheCreationTokens,
		TotalTokens:         totalTokens,
		Failed:              payload.Failed,
		StatusCode:          statusCode,
		ErrorMessage:        requestHistoryErrorMessage(payload.Failed, statusCode, payload.Failure.Body),
	})

	if !payload.Failed {
		if err := r.observeUsageSuccess(ctx, resolvedIndex, string(pool), payload.Model, payload.Detail); err != nil {
			return failure(err)
		}
		return mustMarshal(Envelope{OK: true})
	}

	switch kind {
	case state.FailureRateLimit:
		if err := r.observe429(ctx, resolvedIndex, string(pool), payload.Model, true); err != nil {
			return failure(err)
		}
	case state.FailureUnauthorized:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureUnauthorized,
			StatusCode:   http.StatusUnauthorized,
			Reason:       priority.ReasonUnauthorizedCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureForbidden:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureForbidden,
			StatusCode:   http.StatusForbidden,
			Reason:       priority.ReasonForbiddenCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureServerError:
		status := payload.Failure.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			ModelGroup:   string(pool),
			ProbeModel:   payload.Model,
			Kind:         state.FailureServerError,
			StatusCode:   status,
			Reason:       fmt.Sprintf("%d server error cooldown", status),
			Cooldown:     serverErrorBaseCooldown,
			ProbeEnabled: true,
		}, true); err != nil {
			return failure(err)
		}
	}
	return mustMarshal(Envelope{OK: true})
}

func isAntigravityUsageProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "antigravity" || p == "google" || p == "gemini" || p == "google-antigravity" || strings.Contains(p, "antigravity")
}

func failureKindFor(statusCode int, body string) state.FailureKind {
	switch statusCode {
	case http.StatusUnauthorized:
		return state.FailureUnauthorized
	case http.StatusForbidden:
		return state.FailureForbidden
	case http.StatusTooManyRequests:
		return state.FailureRateLimit
	}
	if statusCode >= 500 && statusCode <= 599 {
		return state.FailureServerError
	}
	if statusCode != 0 {
		return ""
	}
	upper := strings.ToUpper(body)
	switch {
	case strings.Contains(upper, "UNAUTHENTICATED") || strings.Contains(upper, "401"):
		return state.FailureUnauthorized
	case strings.Contains(upper, "PERMISSION_DENIED") || strings.Contains(upper, "403"):
		return state.FailureForbidden
	case strings.Contains(upper, "RESOURCE_EXHAUSTED") || strings.Contains(upper, "RATE_LIMIT") || strings.Contains(upper, "429"):
		return state.FailureRateLimit
	case strings.Contains(upper, "UNAVAILABLE") || strings.Contains(upper, "500") || strings.Contains(upper, "502") ||
		strings.Contains(upper, "503") || strings.Contains(upper, "504"):
		return state.FailureServerError
	default:
		return ""
	}
}

func (r *Runtime) handleFilterEvent(ctx context.Context, raw []byte) []byte {
	var payload struct {
		AuthIndex  string `json:"auth_index"`
		AuthName   string `json:"auth_name"`
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
		Model      string `json:"model"`
		ModelGroup string `json:"model_group"`
	}
	_ = json.Unmarshal(raw, &payload)
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthName)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}
	resolvedIndex := r.resolveAuthIndex(authIndex, nil)
	model := strings.TrimSpace(payload.Model)
	modelGroup := strings.TrimSpace(payload.ModelGroup)
	if modelGroup == "" && model != "" {
		modelGroup = string(config.ClassifyModelQuotaPool(model))
	}
	kind := failureKindFor(payload.StatusCode, payload.Error)
	// Without a pool or model the failure cannot be isolated safely; only 401 and 403
	// are unambiguously account-wide.
	if modelGroup == "" && kind != state.FailureUnauthorized && kind != state.FailureForbidden {
		return mustMarshal(Envelope{OK: true})
	}

	switch kind {
	case state.FailureRateLimit:
		if err := r.observe429(ctx, resolvedIndex, modelGroup, model, true); err != nil {
			return failure(err)
		}
	case state.FailureUnauthorized:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureUnauthorized,
			StatusCode:   http.StatusUnauthorized,
			Reason:       priority.ReasonUnauthorizedCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureForbidden:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureForbidden,
			StatusCode:   http.StatusForbidden,
			Reason:       priority.ReasonForbiddenCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureServerError:
		status := payload.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			ModelGroup:   modelGroup,
			ProbeModel:   model,
			Kind:         state.FailureServerError,
			StatusCode:   status,
			Reason:       fmt.Sprintf("%d server error cooldown", status),
			Cooldown:     serverErrorBaseCooldown,
			ProbeEnabled: true,
		}, true); err != nil {
			return failure(err)
		}
	}

	return mustMarshal(Envelope{OK: true})
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

func (r *Runtime) recordCooldownFailure(ctx context.Context, store *state.Store, authIndex, reason, trigger string, err error) {
	if strings.TrimSpace(trigger) == "" {
		trigger = "health_cooldown"
	}
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
		Trigger:   trigger,
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
		Version:          "1.3.2",
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
