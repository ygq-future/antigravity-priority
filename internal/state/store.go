package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/priority"
)

// SchemaVersion is the current state cache document version. Version 2 removed
// legacy probe scheduling and entry source fields.
const SchemaVersion = 2

// ErrCorruptCache indicates the cache file could not be parsed as valid state JSON.
var ErrCorruptCache = errors.New("state: corrupt cache")

// Entry represents the persisted and learned state for a single credential and model group.
type Entry struct {
	SchemaVersion            int           `json:"schema_version"`
	Provider                 core.Provider `json:"provider"`
	ModelGroup               string        `json:"model_group,omitempty"`
	AuthIndex                string        `json:"auth_index"`
	ObservedAt               time.Time     `json:"observed_at"`
	ResetAt                  time.Time     `json:"reset_at,omitzero"`
	Remaining                int64         `json:"remaining"`
	ShortWindowResetAt       time.Time     `json:"short_window_reset_at,omitzero"`
	ShortWindowRemaining     *int64        `json:"short_window_remaining,omitempty"`
	LongWindowResetAt        time.Time     `json:"long_window_reset_at,omitzero"`
	LongWindowRemaining      *int64        `json:"long_window_remaining,omitempty"`
	ArgonWindowResetAt       time.Time     `json:"argon_window_reset_at,omitzero"`
	ArgonWindowRemaining     *int64        `json:"argon_window_remaining,omitempty"`
	CycleBurnRate            float64       `json:"cycle_burn_rate"`
	Samples                  []QuotaSample `json:"samples,omitempty"`
	LearningBaselineSequence uint64        `json:"learning_baseline_sequence,omitempty"`
	PendingWeightedTokens    float64       `json:"pending_weighted_tokens,omitempty"`
	EstimatedShortBurnPct    float64       `json:"estimated_short_burn_pct,omitempty"`
	EstimatedLongBurnPct     float64       `json:"estimated_long_burn_pct,omitempty"`
	TokensPerShortPct        float64       `json:"tokens_per_short_pct,omitempty"`
	LastUsageAt              time.Time     `json:"last_usage_at,omitzero"`
	LastError                string        `json:"last_error,omitempty"`
	LastFailureAt            time.Time     `json:"last_failure_at,omitzero"`
	AuthInvalid              bool          `json:"auth_invalid,omitempty"`
	PlanType                 core.PlanType `json:"plan_type,omitempty"`
	Unavailable              bool          `json:"unavailable,omitempty"`
	UnavailableAt            time.Time     `json:"unavailable_at,omitzero"`
	UnavailableReason        string        `json:"unavailable_reason,omitempty"`
}

// ProbeSuccess contains updated quota and state evidence after a successful probe.
type ProbeSuccess struct {
	AuthIndex            string
	Provider             core.Provider
	ModelGroup           string
	ObservedAt           time.Time
	ResetAt              time.Time
	Remaining            int64
	ShortWindowResetAt   time.Time
	ShortWindowRemaining *int64
	LongWindowResetAt    time.Time
	LongWindowRemaining  *int64
	ArgonWindowResetAt   time.Time
	ArgonWindowRemaining *int64
	AuthInvalid          bool
	PlanType             core.PlanType
	SampleCapacity       int
	ProbeRoundID         string
}

// ProbeSampleRecord identifies a quota sample appended during a probe round.
// It intentionally contains only the sample and its technical credential key;
// management adapters may resolve the key to a display email.
type ProbeSampleRecord struct {
	AuthIndex  string
	ModelGroup string
	Sample     QuotaSample
}

// ProbeFailure contains diagnostic information after a probe failure.
type ProbeFailure struct {
	AuthIndex  string
	Provider   core.Provider
	ModelGroup string
	ObservedAt time.Time
	Err        error
}

// ProbeUnavailable records that the upstream does not expose a model group for
// this account. It is distinct from a probe failure and carries no quota.
type ProbeUnavailable struct {
	AuthIndex  string
	Provider   core.Provider
	ModelGroup string
	ObservedAt time.Time
	Reason     string
}

// DynamicConfig contains all runtime-customizable configuration parameters
// that can be modified via the UI Config Center without restarting the plugin (REQ-09).
type DynamicConfig = config.DynamicConfig

// FailureKind classifies why a credential was temporarily removed from routing.
type FailureKind string

const (
	FailureRateLimit    FailureKind = "rate_limit"
	FailureServerError  FailureKind = "server_error"
	FailureForbidden    FailureKind = "forbidden"
	FailureUnauthorized FailureKind = "unauthorized"
)

// CooldownEntry tracks a temporary health intervention for a credential.
// An empty ModelGroup means account scope; a non-empty ModelGroup with an
// empty Model means pool scope; both set means model scope.
type CooldownEntry struct {
	AuthIndex               string      `json:"auth_index"`
	ModelGroup              string      `json:"model_group,omitempty"`
	Model                   string      `json:"model,omitempty"`
	ProbeModel              string      `json:"probe_model,omitempty"`
	FailureKind             FailureKind `json:"failure_kind,omitempty"`
	StatusCode              int         `json:"status_code,omitempty"`
	TriggeredAt             time.Time   `json:"triggered_at"`
	CooldownUntil           time.Time   `json:"cooldown_until"`
	NextRecoveryAt          time.Time   `json:"next_recovery_at,omitzero"`
	BackoffStep             int         `json:"backoff_step,omitempty"`
	ProbeEnabled            bool        `json:"probe_enabled,omitempty"`
	Reason                  string      `json:"reason"`
	PreviousPriority        int         `json:"previous_priority,omitempty"`
	PreviousPriorityMissing bool        `json:"previous_priority_missing,omitempty"`
}

// PriorityRulesConfig holds priority rule settings for DynamicConfig.
type PriorityRulesConfig = config.PriorityRulesConfig

// Store manages the in-memory and on-disk state cache document.
type Store struct {
	mu              sync.RWMutex
	saveMu          sync.Mutex
	paths           Paths
	entries         map[string]Entry
	latestAudit     string
	latestResult    []byte
	runHistory      []byte
	scheduleConfig  *ScheduleConfig
	dynamicConfig   *DynamicConfig
	lastAutoApplyAt time.Time
	cooldowns       map[string]CooldownEntry
	requestHistory  []RequestHistoryEntry
	warmupCompleted map[string]time.Time
	loadErrors      []string
}

// stateDocument is the main quota/health document. Run and request history
// live in sibling documents so their writes never rewrite quota evidence.
type stateDocument struct {
	SchemaVersion   int                      `json:"schema_version"`
	Entries         map[string]Entry         `json:"entries"`
	Cooldowns       map[string]CooldownEntry `json:"cooldowns,omitempty"`
	WarmupCompleted map[string]time.Time     `json:"warmup_completed,omitempty"`
}

// legacyStateDocument retains the pre-split single-file fields so an existing
// cache migrates on the next full save without losing history or configuration.
type legacyStateDocument struct {
	stateDocument
	LatestAudit     string                `json:"latest_audit,omitempty"`
	LatestResult    json.RawMessage       `json:"latest_result,omitempty"`
	RunHistory      json.RawMessage       `json:"run_history,omitempty"`
	ScheduleConfig  *ScheduleConfig       `json:"schedule_config,omitempty"`
	DynamicConfig   *DynamicConfig        `json:"app_config,omitempty"`
	LastAutoApplyAt time.Time             `json:"last_auto_apply_at,omitzero"`
	RequestHistory  []RequestHistoryEntry `json:"request_history,omitempty"`
}

// runtimeDocument holds low-frequency scheduling configuration.
type runtimeDocument struct {
	SchemaVersion   int             `json:"schema_version"`
	ScheduleConfig  *ScheduleConfig `json:"schedule_config,omitempty"`
	DynamicConfig   *DynamicConfig  `json:"app_config,omitempty"`
	LastAutoApplyAt time.Time       `json:"last_auto_apply_at,omitzero"`
}

// runsDocument holds the latest execution result and bounded run history.
type runsDocument struct {
	SchemaVersion int             `json:"schema_version"`
	LatestAudit   string          `json:"latest_audit,omitempty"`
	LatestResult  json.RawMessage `json:"latest_result,omitempty"`
	RunHistory    json.RawMessage `json:"run_history,omitempty"`
}

// requestsDocument isolates the high-frequency request history.
type requestsDocument struct {
	SchemaVersion  int                   `json:"schema_version"`
	RequestHistory []RequestHistoryEntry `json:"request_history,omitempty"`
}

// Load loads the split cache documents rooted at path. A missing main file
// yields an empty store. A corrupt main file fails, while corrupt sidecars are
// isolated and reported through LoadErrors so one damaged history file cannot
// block quota scheduling.
//
// Legacy single-file and pre-directory sibling layouts are migrated into the
// current directory, then removed only after every new document is written.
func Load(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("load state context: %w", err)
	}
	paths := ResolvePaths(path)
	legacy := LegacyPaths(path)
	store := &Store{
		paths:           paths,
		entries:         make(map[string]Entry),
		cooldowns:       make(map[string]CooldownEntry),
		warmupCompleted: make(map[string]time.Time),
	}

	var main legacyStateDocument
	newMain, err := readDocument(paths.Main, &main)
	if err != nil {
		return store, fmt.Errorf("decode state cache %s: %w", paths.Main, errors.Join(ErrCorruptCache, err))
	}
	if newMain {
		store.applyStateDocument(main)
	}

	var legacyMain legacyStateDocument
	legacyMainExists := false
	if legacy.Main != paths.Main {
		legacyMainExists, err = readDocument(legacy.Main, &legacyMain)
		if err != nil {
			store.loadErrors = append(store.loadErrors, fmt.Sprintf("legacy state cache %s: %v", legacy.Main, err))
		}
	}
	if !newMain && legacyMainExists {
		store.applyStateDocument(legacyMain)
	}

	var runtimeDoc runtimeDocument
	newRuntime, err := readDocument(paths.Runtime, &runtimeDoc)
	if err != nil {
		store.loadErrors = append(store.loadErrors, fmt.Sprintf("runtime cache %s: %v", paths.Runtime, err))
	}
	var legacyRuntimeDoc runtimeDocument
	legacyRuntimeExists := false
	if legacy.Runtime != paths.Runtime {
		legacyRuntimeExists, err = readDocument(legacy.Runtime, &legacyRuntimeDoc)
		if err != nil {
			store.loadErrors = append(store.loadErrors, fmt.Sprintf("legacy runtime cache %s: %v", legacy.Runtime, err))
		}
	}
	usedLegacyRuntime := false
	switch {
	case newRuntime:
		store.applyRuntimeDocument(runtimeDoc)
	case legacyRuntimeExists:
		store.applyRuntimeDocument(legacyRuntimeDoc)
		usedLegacyRuntime = true
	case legacyMainExists:
		store.applyLegacyRuntime(legacyMain)
		usedLegacyRuntime = true
	}

	var runsDoc runsDocument
	newRuns, err := readDocument(paths.Runs, &runsDoc)
	if err != nil {
		store.loadErrors = append(store.loadErrors, fmt.Sprintf("runs cache %s: %v", paths.Runs, err))
	}
	var legacyRunsDoc runsDocument
	legacyRunsExists := false
	if legacy.Runs != paths.Runs {
		legacyRunsExists, err = readDocument(legacy.Runs, &legacyRunsDoc)
		if err != nil {
			store.loadErrors = append(store.loadErrors, fmt.Sprintf("legacy runs cache %s: %v", legacy.Runs, err))
		}
	}
	usedLegacyRuns := false
	switch {
	case newRuns:
		store.applyRunsDocument(runsDoc)
	case legacyRunsExists:
		store.applyRunsDocument(legacyRunsDoc)
		usedLegacyRuns = true
	case legacyMainExists:
		store.applyLegacyRuns(legacyMain)
		usedLegacyRuns = true
	}

	var requestsDoc requestsDocument
	newRequests, err := readDocument(paths.Requests, &requestsDoc)
	if err != nil {
		store.loadErrors = append(store.loadErrors, fmt.Sprintf("request history cache %s: %v", paths.Requests, err))
	}
	var legacyRequestsDoc requestsDocument
	legacyRequestsExists := false
	if legacy.Requests != paths.Requests {
		legacyRequestsExists, err = readDocument(legacy.Requests, &legacyRequestsDoc)
		if err != nil {
			store.loadErrors = append(store.loadErrors, fmt.Sprintf("legacy request history cache %s: %v", legacy.Requests, err))
		}
	}
	usedLegacyRequests := false
	switch {
	case newRequests:
		store.requestHistory = append([]RequestHistoryEntry(nil), requestsDoc.RequestHistory...)
	case legacyRequestsExists:
		store.requestHistory = append([]RequestHistoryEntry(nil), legacyRequestsDoc.RequestHistory...)
		usedLegacyRequests = true
	case legacyMainExists:
		store.applyLegacyRequests(legacyMain)
		usedLegacyRequests = true
	}

	if legacyMainExists || legacyRuntimeExists || legacyRunsExists || legacyRequestsExists {
		usedLegacy := (!newMain && legacyMainExists) || usedLegacyRuntime || usedLegacyRuns || usedLegacyRequests
		if usedLegacy {
			if err := store.SaveAtomic(ctx); err != nil {
				store.loadErrors = append(store.loadErrors, fmt.Sprintf("migrate legacy cache: %v", err))
				return store, nil
			}
		}
		store.removeLegacy(ctx, legacy, paths)
	}

	return store, nil
}

func (s *Store) applyLegacyRuntime(doc legacyStateDocument) {
	s.applyConfigDocuments(doc.ScheduleConfig, doc.DynamicConfig)
	s.lastAutoApplyAt = utcOrZero(doc.LastAutoApplyAt)
}

func (s *Store) applyLegacyRuns(doc legacyStateDocument) {
	s.applyRunsFields(doc.LatestAudit, doc.LatestResult, doc.RunHistory)
}

func (s *Store) applyLegacyRequests(doc legacyStateDocument) {
	if len(doc.RequestHistory) > 0 {
		s.requestHistory = append([]RequestHistoryEntry(nil), doc.RequestHistory...)
	}
}

// removeLegacy deletes pre-directory files after migration. Files that also
// belong to the current layout are never removed.
func (s *Store) removeLegacy(ctx context.Context, legacy, current Paths) {
	if err := ctx.Err(); err != nil {
		return
	}
	for _, path := range []string{legacy.Main, legacy.Runtime, legacy.Runs, legacy.Requests} {
		if path == "" || path == current.Main || path == current.Runtime || path == current.Runs || path == current.Requests {
			continue
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.loadErrors = append(s.loadErrors, fmt.Sprintf("remove legacy cache %s: %v", path, err))
		}
	}
}

// readDocument reads one cache document. It reports false for a missing or
// empty file and returns the decode error for a corrupt document.
func readDocument(path string, target any) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return false, nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return true, err
	}
	return true, nil
}

func (s *Store) applyStateDocument(doc legacyStateDocument) {
	if doc.Entries != nil {
		s.entries = doc.Entries
		for key, entry := range s.entries {
			entry.Samples, entry.LearningBaselineSequence = normalizeSampleHistory(entry.Samples, entry.LearningBaselineSequence)
			// Removed probe scheduling/source fields were optional, so older
			// entries remain valid after normalization. SaveAtomic will persist
			// them under the current schema version.
			if entry.SchemaVersion < SchemaVersion {
				entry.SchemaVersion = SchemaVersion
			}
			s.entries[key] = entry
		}
		cleanseClonedArgonEntries(s.entries)
	}
	s.applyRunsFields(doc.LatestAudit, doc.LatestResult, doc.RunHistory)
	if len(doc.RequestHistory) > 0 {
		s.requestHistory = append([]RequestHistoryEntry(nil), doc.RequestHistory...)
	}
	s.applyConfigDocuments(doc.ScheduleConfig, doc.DynamicConfig)
	s.lastAutoApplyAt = utcOrZero(doc.LastAutoApplyAt)
	if doc.Cooldowns != nil {
		s.cooldowns = make(map[string]CooldownEntry, len(doc.Cooldowns))
		for k, v := range doc.Cooldowns {
			key := k
			if !strings.Contains(k, "::") && v.ModelGroup != "" {
				key = CooldownKey(v.AuthIndex, v.ModelGroup)
			}
			s.cooldowns[key] = v
		}
	}
	if doc.WarmupCompleted != nil {
		s.warmupCompleted = make(map[string]time.Time, len(doc.WarmupCompleted))
		for key, at := range doc.WarmupCompleted {
			s.warmupCompleted[key] = utcOrZero(at)
		}
	}
}

func (s *Store) applyRuntimeDocument(doc runtimeDocument) {
	s.applyConfigDocuments(doc.ScheduleConfig, doc.DynamicConfig)
	s.lastAutoApplyAt = utcOrZero(doc.LastAutoApplyAt)
}

func (s *Store) applyRunsDocument(doc runsDocument) {
	s.applyRunsFields(doc.LatestAudit, doc.LatestResult, doc.RunHistory)
}

// applyRunsFields replaces the persisted execution projection. An empty
// sidecar field clears legacy main-document values so the split stays authoritative.
func (s *Store) applyRunsFields(audit string, latestResult, runHistory []byte) {
	s.latestAudit = audit
	s.latestResult = nil
	if len(latestResult) > 0 {
		s.latestResult = append([]byte(nil), latestResult...)
	}
	s.runHistory = nil
	if len(runHistory) > 0 {
		s.runHistory = append([]byte(nil), runHistory...)
	}
}

// applyConfigDocuments merges persisted scheduling configuration. An explicit
// schedule_config always wins over the schedule embedded in app_config.
func (s *Store) applyConfigDocuments(schedule *ScheduleConfig, dynamic *DynamicConfig) {
	if schedule != nil {
		sc := *schedule
		s.scheduleConfig = &sc
	}
	if dynamic == nil {
		return
	}
	dc := *dynamic
	if s.scheduleConfig != nil {
		dc.Schedule = *s.scheduleConfig
	} else {
		sc := dc.Schedule
		s.scheduleConfig = &sc
	}
	s.dynamicConfig = &dc
}

// LoadErrors reports sidecar documents that could not be decoded. Main state
// corruption remains fatal and is returned by Load instead.
func (s *Store) LoadErrors() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.loadErrors...)
}

// SaveAtomic writes every split document. Startup and migration use it to
// rewrite a legacy single-file cache into the split layout.
func (s *Store) SaveAtomic(ctx context.Context) error {
	if err := s.SaveRuntimeAtomic(ctx); err != nil {
		return err
	}
	if err := s.SaveRunsAtomic(ctx); err != nil {
		return err
	}
	if err := s.SaveRequestsAtomic(ctx); err != nil {
		return err
	}
	return s.SaveStateAtomic(ctx)
}

// SaveStateAtomic writes only quota evidence and health state.
func (s *Store) SaveStateAtomic(ctx context.Context) error {
	return s.saveDocument(ctx, "state cache", s.paths.Main, func() any {
		return stateDocument{
			SchemaVersion:   SchemaVersion,
			Entries:         s.entries,
			Cooldowns:       s.cooldowns,
			WarmupCompleted: s.warmupCompleted,
		}
	})
}

// SaveRuntimeAtomic writes only scheduling configuration.
func (s *Store) SaveRuntimeAtomic(ctx context.Context) error {
	return s.saveDocument(ctx, "runtime cache", s.paths.Runtime, func() any {
		return runtimeDocument{
			SchemaVersion:   SchemaVersion,
			ScheduleConfig:  s.scheduleConfig,
			DynamicConfig:   s.dynamicConfig,
			LastAutoApplyAt: s.lastAutoApplyAt,
		}
	})
}

// SaveRunsAtomic writes only the latest execution result and run history.
func (s *Store) SaveRunsAtomic(ctx context.Context) error {
	return s.saveDocument(ctx, "runs cache", s.paths.Runs, func() any {
		return runsDocument{
			SchemaVersion: SchemaVersion,
			LatestAudit:   s.latestAudit,
			LatestResult:  s.latestResult,
			RunHistory:    s.runHistory,
		}
	})
}

// SaveRequestsAtomic writes only the request history document.
func (s *Store) SaveRequestsAtomic(ctx context.Context) error {
	return s.saveDocument(ctx, "request history cache", s.paths.Requests, func() any {
		return requestsDocument{
			SchemaVersion:  SchemaVersion,
			RequestHistory: s.requestHistory,
		}
	})
}

// saveDocument snapshots one document under the read lock, then atomically
// replaces only its file. The write lock is held only while encoding so
// concurrent readers never observe a half-mutated document.
func (s *Store) saveDocument(ctx context.Context, label, path string, build func() any) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save %s context: %w", label, err)
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.RLock()
	data, err := json.MarshalIndent(build(), "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("encode %s: %w", label, err)
	}
	return writeFileAtomic(path, data)
}

// WriteRequestHistoryAtomic persists only the request-history sidecar. The
// hot request path calls it directly so it never reads or rewrites quota state.
func WriteRequestHistoryAtomic(ctx context.Context, mainPath string, entries []RequestHistoryEntry) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save request history context: %w", err)
	}
	paths := ResolvePaths(mainPath)
	data, err := json.MarshalIndent(requestsDocument{
		SchemaVersion:  SchemaVersion,
		RequestHistory: entries,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode request history cache: %w", err)
	}
	return writeFileAtomic(paths.Requests, data)
}

func writeFileAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create cache dir %s: %w", dir, err)
	}
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create cache temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write cache temp: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("close cache temp: %w", err)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename cache temp: %w", err)
	}
	return nil
}

// SetRuntimeSnapshot updates the persistent snapshot and run history.
func (s *Store) SetRuntimeSnapshot(audit string, resultJSON []byte, historyJSON []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestAudit = audit
	s.latestResult = append([]byte(nil), resultJSON...)
	s.runHistory = append([]byte(nil), historyJSON...)
}

// GetRuntimeSnapshot returns the persisted runtime snapshot and run history.
func (s *Store) GetRuntimeSnapshot() (audit string, resultJSON []byte, historyJSON []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestAudit, append([]byte(nil), s.latestResult...), append([]byte(nil), s.runHistory...)
}

// GetRequestHistory returns a copy of the recorded request history entries.
func (s *Store) GetRequestHistory() []RequestHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]RequestHistoryEntry, len(s.requestHistory))
	copy(out, s.requestHistory)
	return out
}

// SetRequestHistory sets the persistent request history entries.
func (s *Store) SetRequestHistory(entries []RequestHistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requestHistory = append([]RequestHistoryEntry(nil), entries...)
}

// GetLastWarmupAt returns the last successful 5h warm-up time for a credential
// pool. The zero time means the credential has never been warmed up.
func (s *Store) GetLastWarmupAt(authIndex, modelGroup string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.warmupCompleted[entryKey(authIndex, modelGroup)]
}

// SetWarmupCompleted records a successful 5h warm-up for a credential pool.
// Warm-up state is persisted separately from quota evidence so a probe write
// never clears the debounce window and a restart never forgets it.
func (s *Store) SetWarmupCompleted(authIndex, modelGroup string, at time.Time) {
	key := entryKey(authIndex, modelGroup)
	if key == "" || at.IsZero() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.warmupCompleted == nil {
		s.warmupCompleted = make(map[string]time.Time)
	}
	s.warmupCompleted[key] = at.UTC()
}

// MarkProbeSuccess records a successful probe, adaptively updates cycle burn rate, and resets errors.
func (s *Store) MarkProbeSuccess(ctx context.Context, success ProbeSuccess) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mark probe success context: %w", err)
	}
	key := entryKey(success.AuthIndex, success.ModelGroup)

	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.entries[key]
	capacity := success.SampleCapacity
	if capacity <= 0 {
		if s.dynamicConfig != nil && s.dynamicConfig.QuotaSampleCapacity > 0 {
			capacity = s.dynamicConfig.QuotaSampleCapacity
		} else {
			capacity = DefaultQuotaSampleCapacity
		}
	}
	newRate, newSamples, newBaseline := UpdateSamplesAndCycleBurnRate(
		prev.CycleBurnRate,
		prev.Samples,
		prev.LearningBaselineSequence,
		success.ObservedAt,
		success.ShortWindowResetAt,
		success.ShortWindowRemaining,
		success.LongWindowRemaining,
		capacity,
	)
	if sampleAppended(prev.Samples, newSamples) && strings.TrimSpace(success.ProbeRoundID) != "" && len(newSamples) > 0 {
		newSamples[len(newSamples)-1].ProbeRoundID = strings.TrimSpace(success.ProbeRoundID)
	}
	// Probe calibration for real-time usage estimator
	tokensPerShortPct := prev.TokensPerShortPct
	if prev.PendingWeightedTokens > 0 && prev.ShortWindowRemaining != nil && success.ShortWindowRemaining != nil {
		if !prev.ShortWindowResetAt.IsZero() && !success.ShortWindowResetAt.IsZero() && prev.ShortWindowResetAt.Equal(success.ShortWindowResetAt) {
			deltaShort := *prev.ShortWindowRemaining - *success.ShortWindowRemaining
			if deltaShort >= 2 {
				kObs := prev.PendingWeightedTokens / float64(deltaShort)
				prevK := prev.TokensPerShortPct
				if prevK <= 0 {
					prevK = defaultTokensPerShortPct(success.ModelGroup)
				}
				newK := 0.3*kObs + 0.7*prevK
				tokensPerShortPct = clamp(newK, MinTokensPerShortPct, MaxTokensPerShortPct)
			}
		}
	}

	entry := Entry{
		SchemaVersion:            SchemaVersion,
		Provider:                 success.Provider,
		ModelGroup:               entryModelGroup(success.ModelGroup),
		AuthIndex:                authIndexKey(success.AuthIndex),
		ObservedAt:               success.ObservedAt.UTC(),
		ResetAt:                  utcOrZero(success.ResetAt),
		Remaining:                success.Remaining,
		ShortWindowResetAt:       utcOrZero(success.ShortWindowResetAt),
		ShortWindowRemaining:     cloneInt64Ptr(success.ShortWindowRemaining),
		LongWindowResetAt:        utcOrZero(success.LongWindowResetAt),
		LongWindowRemaining:      cloneInt64Ptr(success.LongWindowRemaining),
		ArgonWindowResetAt:       utcOrZero(success.ArgonWindowResetAt),
		ArgonWindowRemaining:     cloneInt64Ptr(success.ArgonWindowRemaining),
		CycleBurnRate:            newRate,
		Samples:                  newSamples,
		LearningBaselineSequence: newBaseline,
		PendingWeightedTokens:    0,
		EstimatedShortBurnPct:    0,
		EstimatedLongBurnPct:     0,
		TokensPerShortPct:        tokensPerShortPct,
		LastUsageAt:              time.Time{},
		LastError:                "",
		LastFailureAt:            time.Time{},
		AuthInvalid:              success.AuthInvalid,
		PlanType:                 success.PlanType,
	}
	s.entries[key] = entry
	return nil
}

// MarkProbeFailure records a probe failure and sanitized error message.
func (s *Store) MarkProbeFailure(ctx context.Context, failure ProbeFailure) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mark probe failure context: %w", err)
	}
	key := entryKey(failure.AuthIndex, failure.ModelGroup)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.entries[key]
	entry.SchemaVersion = SchemaVersion
	entry.Provider = failure.Provider
	entry.ModelGroup = entryModelGroup(failure.ModelGroup)
	entry.AuthIndex = authIndexKey(failure.AuthIndex)
	entry.LastError = sanitizeProbeError(failure.Err)
	entry.LastFailureAt = failure.ObservedAt.UTC()
	entry.Unavailable = false
	entry.UnavailableAt = time.Time{}
	entry.UnavailableReason = ""

	s.entries[key] = entry
	return nil
}

// MarkProbeUnavailable records that the upstream does not expose a model group
// for this account. It is not a probe failure: the group has no quota because
// the account is not entitled to it. Stale quota evidence is cleared and
// GetHistoricalEvidence suppresses the entry until a successful probe returns.
func (s *Store) MarkProbeUnavailable(ctx context.Context, unavailable ProbeUnavailable) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mark probe unavailable context: %w", err)
	}
	key := entryKey(unavailable.AuthIndex, unavailable.ModelGroup)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.entries[key]
	entry.SchemaVersion = SchemaVersion
	entry.Provider = unavailable.Provider
	entry.ModelGroup = entryModelGroup(unavailable.ModelGroup)
	entry.AuthIndex = authIndexKey(unavailable.AuthIndex)
	entry.ObservedAt = unavailable.ObservedAt.UTC()
	entry.Unavailable = true
	entry.UnavailableAt = unavailable.ObservedAt.UTC()
	entry.UnavailableReason = strings.TrimSpace(unavailable.Reason)
	entry.LastError = ""
	entry.LastFailureAt = time.Time{}
	entry.ResetAt = time.Time{}
	entry.Remaining = 0
	entry.ShortWindowResetAt = time.Time{}
	entry.ShortWindowRemaining = nil
	entry.LongWindowResetAt = time.Time{}
	entry.LongWindowRemaining = nil
	entry.ArgonWindowResetAt = time.Time{}
	entry.ArgonWindowRemaining = nil
	entry.CycleBurnRate = 0
	entry.Samples = nil
	entry.LearningBaselineSequence = 0
	entry.PendingWeightedTokens = 0
	entry.EstimatedShortBurnPct = 0
	entry.EstimatedLongBurnPct = 0
	entry.TokensPerShortPct = 0
	entry.PlanType = core.PlanTypeUnknown

	s.entries[key] = entry
	return nil
}

// RecordUsageConsumption records estimated token consumption for real-time quota deduction.
func (s *Store) RecordUsageConsumption(authIndex, modelGroup string, weightedTokens float64, now time.Time) {
	if weightedTokens <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := entryKey(authIndex, modelGroup)
	entry, ok := s.entries[key]
	if !ok {
		return
	}

	entry.PendingWeightedTokens += weightedTokens
	entry.LastUsageAt = now.UTC()
	k5h := effectiveTokensPerShortPct(entry)
	deltaShortPct := weightedTokens / k5h
	cCycle := effectiveCycleBurnRate(entry)

	entry.EstimatedShortBurnPct += deltaShortPct
	entry.EstimatedLongBurnPct += deltaShortPct * cCycle
	s.entries[key] = entry

	// Gemini and Argon share the physical 5h short window while retaining independent 7d weekly pools.
	// Accumulate the exact short burn percentage onto the sibling group without mutating its EstimatedLongBurnPct.
	normGroup := strings.ToLower(strings.TrimSpace(modelGroup))
	var siblingGroup string
	if normGroup == string(config.AntigravityModelGroupGemini) {
		siblingGroup = string(config.AntigravityModelGroupArgon)
	} else if normGroup == string(config.AntigravityModelGroupArgon) {
		siblingGroup = string(config.AntigravityModelGroupGemini)
	}

	if siblingGroup != "" {
		sibKey := entryKey(authIndex, siblingGroup)
		if sibEntry, sibOk := s.entries[sibKey]; sibOk {
			sibEntry.EstimatedShortBurnPct += deltaShortPct
			sibEntry.LastUsageAt = now.UTC()
			s.entries[sibKey] = sibEntry
		}
	}
}

// GetEntry retrieves a copy of the cached entry for authIndex and modelGroup.
func (s *Store) GetEntry(authIndex, modelGroup string) (Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[entryKey(authIndex, modelGroup)]
	return entry, ok
}

// GetPlanType returns the cached PlanType for authIndex across known model groups.
// It checks the specific modelGroup if provided, or looks up in known groups.
func (s *Store) GetPlanType(authIndex, preferredGroup string) core.PlanType {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if preferredGroup != "" {
		if entry, ok := s.entries[entryKey(authIndex, preferredGroup)]; ok && entry.PlanType != "" && entry.PlanType != core.PlanTypeUnknown {
			return entry.PlanType
		}
	}
	for _, group := range config.AllAntigravityModelGroups {
		groupStr := string(group)
		if groupStr == preferredGroup {
			continue
		}
		if entry, ok := s.entries[entryKey(authIndex, groupStr)]; ok && entry.PlanType != "" && entry.PlanType != core.PlanTypeUnknown {
			return entry.PlanType
		}
	}
	if entry, ok := s.entries[authIndexKey(authIndex)]; ok && entry.PlanType != "" && entry.PlanType != core.PlanTypeUnknown {
		return entry.PlanType
	}
	return core.PlanTypeUnknown
}

// GetCycleBurnRate returns the learned cycle burn rate for the credential, falling back to 0.15.
func (s *Store) GetCycleBurnRate(authIndex, modelGroup string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[entryKey(authIndex, modelGroup)]
	if !ok || entry.CycleBurnRate <= 0 {
		return DefaultCycleBurnRate
	}
	return entry.CycleBurnRate
}

// GetSamples returns a copy of the recorded quota samples for authIndex and modelGroup.
func (s *Store) GetSamples(authIndex, modelGroup string) []QuotaSample {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[entryKey(authIndex, modelGroup)]
	if !ok || len(entry.Samples) == 0 {
		return nil
	}
	return append([]QuotaSample(nil), entry.Samples...)
}

// GetSamplesByProbeRound returns only samples appended by the specified probe
// round. Unchanged quota observations are deliberately absent.
func (s *Store) GetSamplesByProbeRound(probeRoundID, modelGroup string) []ProbeSampleRecord {
	probeRoundID = strings.TrimSpace(probeRoundID)
	modelGroup = entryModelGroup(modelGroup)
	if probeRoundID == "" || modelGroup == "" {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	records := make([]ProbeSampleRecord, 0)
	for _, entry := range s.entries {
		if entry.ModelGroup != modelGroup {
			continue
		}
		for _, sample := range entry.Samples {
			if sample.ProbeRoundID != probeRoundID {
				continue
			}
			records = append(records, ProbeSampleRecord{
				AuthIndex:  entry.AuthIndex,
				ModelGroup: modelGroup,
				Sample:     sample,
			})
		}
	}
	sort.Slice(records, func(left, right int) bool {
		leftSample := records[left].Sample
		rightSample := records[right].Sample
		if !leftSample.ObservedAt.Equal(rightSample.ObservedAt) {
			return leftSample.ObservedAt.After(rightSample.ObservedAt)
		}
		if leftSample.Sequence != rightSample.Sequence {
			return leftSample.Sequence > rightSample.Sequence
		}
		return records[left].AuthIndex < records[right].AuthIndex
	})
	return records
}

// Entries returns a shallow copy of all cached entries.
func (s *Store) Entries() map[string]Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]Entry, len(s.entries))
	maps.Copy(res, s.entries)
	return res
}

func entryKey(authIndex, modelGroup string) string {
	authKey := authIndexKey(authIndex)
	group := entryModelGroup(modelGroup)
	if group == "" {
		return authKey
	}
	return authKey + "|model_group=" + group
}

func authIndexKey(authIndex string) string {
	return strings.TrimSpace(authIndex)
}

func entryModelGroup(modelGroup string) string {
	return strings.TrimSpace(modelGroup)
}

func utcOrZero(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func sanitizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	text := strings.TrimSpace(err.Error())
	lower := strings.ToLower(text)
	for _, word := range []string{"authorization", "bearer", "token", "api_key", "apikey", "secret", "credential", "raw-auth", "raw auth", "auth json"} {
		if strings.Contains(lower, word) {
			return "probe failed: sensitive upstream error redacted"
		}
	}
	if len(text) > 240 {
		return text[:240]
	}
	return text
}

// GetScheduleConfig returns a copy of the persisted schedule configuration.
// Returns a zero-value ScheduleConfig if none has been stored.
func (s *Store) GetScheduleConfig() ScheduleConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.scheduleConfig != nil {
		return *s.scheduleConfig
	}
	if s.dynamicConfig != nil {
		return s.dynamicConfig.Schedule
	}
	return ScheduleConfig{}
}

// SetScheduleConfig updates the persisted schedule configuration.
func (s *Store) SetScheduleConfig(cfg ScheduleConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := cfg
	s.scheduleConfig = &sc
	if s.dynamicConfig != nil {
		s.dynamicConfig.Schedule = cfg
	}
}

// GetLastAutoApplyAt returns the completion time of the most recent automatic
// scheduling round. It lets a restarted Runtime honor the remaining interval
// instead of starting the schedule over.
func (s *Store) GetLastAutoApplyAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastAutoApplyAt
}

// SetLastAutoApplyAt records the completion time of the most recent automatic
// scheduling round.
func (s *Store) SetLastAutoApplyAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAutoApplyAt = utcOrZero(at)
}

// GetDynamicConfig returns a copy of the persisted dynamic configuration and true if present.
func (s *Store) GetDynamicConfig() (DynamicConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dynamicConfig == nil {
		return DynamicConfig{}, false
	}
	cfg := *s.dynamicConfig
	if s.scheduleConfig != nil {
		cfg.Schedule = *s.scheduleConfig
	}
	return cfg, true
}

// SetDynamicConfig updates the persisted dynamic configuration.
func (s *Store) SetDynamicConfig(cfg DynamicConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dc := cfg
	sc := cfg.Schedule
	s.dynamicConfig = &dc
	s.scheduleConfig = &sc
}

// GetCustomTiers returns a copy of the configured custom tiers map.
func (s *Store) GetCustomTiers() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dynamicConfig == nil || len(s.dynamicConfig.CustomTiers) == 0 {
		return nil
	}
	res := make(map[string]int, len(s.dynamicConfig.CustomTiers))
	for k, v := range s.dynamicConfig.CustomTiers {
		if v > 0 {
			res[k] = v
		}
	}
	return res
}

// SetCustomTier updates or clears the custom tier for an auth index.
func (s *Store) SetCustomTier(authIndex string, tier int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dynamicConfig == nil {
		def := config.Default().Dynamic()
		s.dynamicConfig = &def
	}
	if s.dynamicConfig.CustomTiers == nil {
		s.dynamicConfig.CustomTiers = make(map[string]int)
	}
	cleanIdx := strings.TrimSpace(authIndex)
	if cleanIdx == "" {
		return
	}
	if tier <= 0 {
		delete(s.dynamicConfig.CustomTiers, cleanIdx)
	} else {
		s.dynamicConfig.CustomTiers[cleanIdx] = tier
	}
}

// GetKnownAuthIndexes returns all unique auth indexes tracked in the store entries.
func (s *Store) GetKnownAuthIndexes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	var indexes []string
	for _, entry := range s.entries {
		if entry.AuthIndex != "" && !seen[entry.AuthIndex] {
			seen[entry.AuthIndex] = true
			indexes = append(indexes, entry.AuthIndex)
		}
	}
	return indexes
}

// GetHistoricalEvidence constructs a historical quota observation from the
// persisted state entry. Loading a successful cache entry never grants Fresh
// Evidence; the Evidence authority decides that only from current probe
// outcomes.
func (s *Store) GetHistoricalEvidence(authIndex, modelGroup string) (priority.QuotaEvidence, bool) {
	normGroup := entryModelGroup(modelGroup)
	s.mu.RLock()
	entry, ok := s.entries[entryKey(authIndex, normGroup)]
	var geminiEntry Entry
	var hasGemini bool
	if normGroup == string(config.AntigravityModelGroupArgon) {
		geminiEntry, hasGemini = s.entries[entryKey(authIndex, string(config.AntigravityModelGroupGemini))]
	}
	s.mu.RUnlock()
	if !ok || entry.SchemaVersion != SchemaVersion {
		return priority.QuotaEvidence{}, false
	}
	if entry.Unavailable {
		return priority.QuotaEvidence{}, false
	}

	if entry.ObservedAt.IsZero() || (entry.ResetAt.IsZero() && entry.ShortWindowResetAt.IsZero() && entry.LongWindowResetAt.IsZero()) {
		return priority.QuotaEvidence{}, false
	}
	if normGroup == string(config.AntigravityModelGroupArgon) {
		if hasGemini && geminiEntry.ArgonWindowRemaining == nil &&
			entry.LongWindowRemaining != nil && geminiEntry.LongWindowRemaining != nil &&
			*entry.LongWindowRemaining == *geminiEntry.LongWindowRemaining &&
			!entry.LongWindowResetAt.IsZero() && entry.LongWindowResetAt.Equal(geminiEntry.LongWindowResetAt) {
			return priority.QuotaEvidence{}, false
		}
	}

	var resetAt *time.Time
	if !entry.ResetAt.IsZero() {
		r := entry.ResetAt
		resetAt = &r
	} else if !entry.ShortWindowResetAt.IsZero() {
		r := entry.ShortWindowResetAt
		resetAt = &r
	} else if !entry.LongWindowResetAt.IsZero() {
		r := entry.LongWindowResetAt
		resetAt = &r
	}
	var shortResetAt *time.Time
	if !entry.ShortWindowResetAt.IsZero() {
		r := entry.ShortWindowResetAt
		shortResetAt = &r
	}
	var longResetAt *time.Time
	if !entry.LongWindowResetAt.IsZero() {
		r := entry.LongWindowResetAt
		longResetAt = &r
	}
	var argonResetAt *time.Time
	if !entry.ArgonWindowResetAt.IsZero() {
		r := entry.ArgonWindowResetAt
		argonResetAt = &r
	}

	remaining := entry.Remaining
	shortRemaining := cloneInt64Ptr(entry.ShortWindowRemaining)
	longRemaining := cloneInt64Ptr(entry.LongWindowRemaining)
	argonRemaining := cloneInt64Ptr(entry.ArgonWindowRemaining)

	if entry.EstimatedShortBurnPct > 0 && shortRemaining != nil {
		deducted := max(*shortRemaining-int64(math.Round(entry.EstimatedShortBurnPct)), 0)
		shortRemaining = &deducted
	}
	if entry.EstimatedLongBurnPct > 0 && longRemaining != nil {
		deducted := max(*longRemaining-int64(math.Round(entry.EstimatedLongBurnPct)), 0)
		longRemaining = &deducted
	}
	if entry.EstimatedLongBurnPct > 0 && argonRemaining != nil {
		deducted := max(*argonRemaining-int64(math.Round(entry.EstimatedLongBurnPct)), 0)
		argonRemaining = &deducted
	}
	if shortRemaining != nil && remaining > *shortRemaining {
		remaining = *shortRemaining
	}
	if longRemaining != nil && remaining > *longRemaining {
		remaining = *longRemaining
	}
	cycleBurnRate := entry.CycleBurnRate
	if cycleBurnRate <= 0 {
		cycleBurnRate = DefaultCycleBurnRate
	}

	return priority.QuotaEvidence{
		Provider:             core.ProviderAntigravity,
		AuthIndex:            entry.AuthIndex,
		ModelGroup:           config.AntigravityModelGroup(entry.ModelGroup),
		ObservedAt:           entry.ObservedAt,
		ResetAt:              resetAt,
		Remaining:            &remaining,
		ShortWindowResetAt:   shortResetAt,
		ShortWindowRemaining: shortRemaining,
		LongWindowResetAt:    longResetAt,
		LongWindowRemaining:  longRemaining,
		ArgonWindowResetAt:   argonResetAt,
		ArgonWindowRemaining: argonRemaining,
		PlanType:             entry.PlanType,
		CycleBurnRate:        cycleBurnRate,
	}, true
}

// BuildHistoricalEvidence constructs historical observations for all given
// credentials under the specified model group.
func (s *Store) BuildHistoricalEvidence(credentials []core.Credential, modelGroup string) []priority.QuotaEvidence {
	evidence := make([]priority.QuotaEvidence, 0, len(credentials))
	for _, cred := range credentials {
		if ev, ok := s.GetHistoricalEvidence(cred.AuthIndex, modelGroup); ok {
			evidence = append(evidence, ev)
		}
	}
	return evidence
}

// CooldownKey formats the map key for account- or pool-scoped health tracking.
func CooldownKey(authIndex, modelGroup string) string {
	group := strings.ToLower(strings.TrimSpace(modelGroup))
	if group == "" {
		return authIndex
	}
	return authIndex + "::" + group
}

// FailureKey formats the map key for account-, pool-, or model-scoped health tracking.
func FailureKey(authIndex, modelGroup, model string) string {
	key := CooldownKey(authIndex, modelGroup)
	if trimmed := strings.TrimSpace(model); trimmed != "" {
		return key + "::" + trimmed
	}
	return key
}

// GetActiveCooldownEntriesForGroup returns the active account- or pool-scoped
// intervention for each credential in the specified model group.
func (s *Store) GetActiveCooldownEntriesForGroup(now time.Time, modelGroup string) map[string]CooldownEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normGroup := strings.ToLower(strings.TrimSpace(modelGroup))
	active := make(map[string]CooldownEntry)
	for _, v := range s.cooldowns {
		if !now.Before(v.CooldownUntil) || strings.TrimSpace(v.Model) != "" {
			continue
		}
		entryGroup := strings.ToLower(strings.TrimSpace(v.ModelGroup))
		if entryGroup != "" && entryGroup != normGroup {
			continue
		}
		existing, ok := active[v.AuthIndex]
		if !ok {
			active[v.AuthIndex] = v
			continue
		}
		if existing.ModelGroup == "" {
			continue
		}
		if v.ModelGroup == "" || v.CooldownUntil.After(existing.CooldownUntil) {
			active[v.AuthIndex] = v
		}
	}
	return active
}

// GetActiveModelCooldownsForGroup returns auth_index -> model -> CooldownUntil
// for unexpired model-scoped interventions in the specified model group.
func (s *Store) GetActiveModelCooldownsForGroup(now time.Time, modelGroup string) map[string]map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	normGroup := strings.ToLower(strings.TrimSpace(modelGroup))
	active := make(map[string]map[string]time.Time)
	for _, v := range s.cooldowns {
		if !now.Before(v.CooldownUntil) {
			continue
		}
		model := strings.ToLower(strings.TrimSpace(v.Model))
		if model == "" || strings.ToLower(strings.TrimSpace(v.ModelGroup)) != normGroup {
			continue
		}
		models := active[v.AuthIndex]
		if models == nil {
			models = make(map[string]time.Time)
			active[v.AuthIndex] = models
		}
		if existing, ok := models[model]; !ok || v.CooldownUntil.After(existing) {
			models[model] = v.CooldownUntil
		}
	}
	return active
}

// GetActiveCooldownsForPlanning returns the active account- and pool-scoped
// entries keyed by CooldownKey, plus each entry's display reason.
func (s *Store) GetActiveCooldownsForPlanning(now time.Time) (map[string]time.Time, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := make(map[string]time.Time)
	reasons := make(map[string]string)
	for _, v := range s.cooldowns {
		if !now.Before(v.CooldownUntil) || strings.TrimSpace(v.Model) != "" {
			continue
		}
		key := CooldownKey(v.AuthIndex, v.ModelGroup)
		active[key] = v.CooldownUntil
		if strings.TrimSpace(v.Reason) != "" {
			reasons[key] = v.Reason
		}
	}
	return active, reasons
}

// DeleteCooldownExact removes only the specified intervention entry.
func (s *Store) DeleteCooldownExact(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cooldowns, key)
}

// GetCooldowns returns a copy of all current cooldown entries.
func (s *Store) GetCooldowns() map[string]CooldownEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make(map[string]CooldownEntry, len(s.cooldowns))
	maps.Copy(copied, s.cooldowns)
	return copied
}

// SetCooldown records or updates a health intervention for a credential.
func (s *Store) SetCooldown(entry CooldownEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cooldowns == nil {
		s.cooldowns = make(map[string]CooldownEntry)
	}
	key := FailureKey(entry.AuthIndex, entry.ModelGroup, entry.Model)
	s.cooldowns[key] = entry
}

func cleanseClonedArgonEntries(entries map[string]Entry) {
	for key, entry := range entries {
		if entryModelGroup(entry.ModelGroup) != string(config.AntigravityModelGroupArgon) {
			continue
		}
		geminiKey := entryKey(entry.AuthIndex, string(config.AntigravityModelGroupGemini))
		geminiEntry, hasGemini := entries[geminiKey]
		if !hasGemini || geminiEntry.ArgonWindowRemaining != nil {
			continue
		}
		if entry.LongWindowRemaining != nil && geminiEntry.LongWindowRemaining != nil &&
			*entry.LongWindowRemaining == *geminiEntry.LongWindowRemaining &&
			!entry.LongWindowResetAt.IsZero() && entry.LongWindowResetAt.Equal(geminiEntry.LongWindowResetAt) {
			delete(entries, key)
		}
	}
}
