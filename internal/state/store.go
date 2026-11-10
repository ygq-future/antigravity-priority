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

// SchemaVersion is the current state cache document version.
const SchemaVersion = 1

// ErrCorruptCache indicates the cache file could not be parsed as valid state JSON.
var ErrCorruptCache = errors.New("state: corrupt cache")

// Source identifies the origin of a cache entry.
type Source string

const (
	// SourceFreshProbe indicates the entry originated from a fresh probe.
	SourceFreshProbe Source = "fresh_probe"
)

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
	NextProbeAt              time.Time     `json:"next_probe_at,omitzero"`
	AuthInvalid              bool          `json:"auth_invalid,omitempty"`
	PlanType                 core.PlanType `json:"plan_type,omitempty"`
	Source                   Source        `json:"source,omitempty"`
}

// ProbePolicy defines cache staleness and expiration thresholds.
type ProbePolicy struct {
	TTL             time.Duration
	ResetStaleAfter time.Duration
}

// ProbeCheck specifies the criteria for deciding if a probe is required.
type ProbeCheck struct {
	AuthIndex  string
	Provider   core.Provider
	ModelGroup string
	Now        time.Time
	Policy     ProbePolicy
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
	NextProbeAt          time.Time
	AuthInvalid          bool
	PlanType             core.PlanType
	Source               Source
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
	AuthIndex   string
	Provider    core.Provider
	ModelGroup  string
	ObservedAt  time.Time
	Err         error
	NextProbeAt time.Time
}

// ProbeSchedule contains scheduling timing for a deferred probe.
type ProbeSchedule struct {
	AuthIndex   string
	Provider    core.Provider
	ModelGroup  string
	NextProbeAt time.Time
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
	PreviousDisabled        bool        `json:"previous_disabled,omitempty"`
	AppliedDisabled         bool        `json:"applied_disabled,omitempty"`
}

// AutoDisabledEntry tracks credentials automatically disabled by the plugin
// due to weekly quota depletion (7d exhausted).
type AutoDisabledEntry struct {
	AuthIndex  string    `json:"auth_index"`
	ModelGroup string    `json:"model_group,omitempty"`
	Reason     string    `json:"reason"`
	DisabledAt time.Time `json:"disabled_at"`
}

// PriorityRulesConfig holds priority rule settings for DynamicConfig.
type PriorityRulesConfig = config.PriorityRulesConfig

// Store manages the in-memory and on-disk state cache document.
type Store struct {
	mu              sync.RWMutex
	saveMu          sync.Mutex
	path            string
	entries         map[string]Entry
	latestAudit     string
	latestResult    []byte
	runHistory      []byte
	scheduleConfig  *ScheduleConfig
	dynamicConfig   *DynamicConfig
	lastAutoApplyAt time.Time
	cooldowns       map[string]CooldownEntry
	autoDisabled    map[string]AutoDisabledEntry
	requestHistory  []RequestHistoryEntry
	warmupCompleted map[string]time.Time
}

type document struct {
	SchemaVersion   int                          `json:"schema_version"`
	Entries         map[string]Entry             `json:"entries"`
	LatestAudit     string                       `json:"latest_audit,omitempty"`
	LatestResult    json.RawMessage              `json:"latest_result,omitempty"`
	RunHistory      json.RawMessage              `json:"run_history,omitempty"`
	ScheduleConfig  *ScheduleConfig              `json:"schedule_config,omitempty"`
	DynamicConfig   *DynamicConfig               `json:"app_config,omitempty"`
	LastAutoApplyAt time.Time                    `json:"last_auto_apply_at,omitzero"`
	Cooldowns       map[string]CooldownEntry     `json:"cooldowns,omitempty"`
	AutoDisabled    map[string]AutoDisabledEntry `json:"auto_disabled,omitempty"`
	RequestHistory  []RequestHistoryEntry        `json:"request_history,omitempty"`
	WarmupCompleted map[string]time.Time         `json:"warmup_completed,omitempty"`
}

// Load loads the state document from path. If the file does not exist, an empty store is returned.
func Load(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("load state context: %w", err)
	}
	store := &Store{
		path:            path,
		entries:         make(map[string]Entry),
		cooldowns:       make(map[string]CooldownEntry),
		autoDisabled:    make(map[string]AutoDisabledEntry),
		warmupCompleted: make(map[string]time.Time),
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return store, nil
		}
		return nil, fmt.Errorf("read state cache %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return store, nil
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return store, fmt.Errorf("decode state cache %s: %w", path, errors.Join(ErrCorruptCache, err))
	}
	if doc.Entries != nil {
		store.entries = doc.Entries
		for key, entry := range store.entries {
			entry.Samples, entry.LearningBaselineSequence = normalizeSampleHistory(entry.Samples, entry.LearningBaselineSequence)
			store.entries[key] = entry
		}
		cleanseClonedArgonEntries(store.entries)
	}
	store.latestAudit = doc.LatestAudit
	if len(doc.LatestResult) > 0 {
		store.latestResult = append([]byte(nil), doc.LatestResult...)
	}
	if len(doc.RunHistory) > 0 {
		store.runHistory = append([]byte(nil), doc.RunHistory...)
	}
	if len(doc.RequestHistory) > 0 {
		store.requestHistory = append([]RequestHistoryEntry(nil), doc.RequestHistory...)
	}
	if doc.ScheduleConfig != nil {
		sc := *doc.ScheduleConfig
		store.scheduleConfig = &sc
	}
	if doc.DynamicConfig != nil {
		dc := *doc.DynamicConfig
		if store.scheduleConfig != nil {
			dc.Schedule = *store.scheduleConfig
		} else {
			sc := dc.Schedule
			store.scheduleConfig = &sc
		}
		store.dynamicConfig = &dc
	}
	store.lastAutoApplyAt = utcOrZero(doc.LastAutoApplyAt)
	if doc.Cooldowns != nil {
		store.cooldowns = make(map[string]CooldownEntry, len(doc.Cooldowns))
		for k, v := range doc.Cooldowns {
			key := k
			if !strings.Contains(k, "::") && v.ModelGroup != "" {
				key = CooldownKey(v.AuthIndex, v.ModelGroup)
			}
			store.cooldowns[key] = v
		}
	}
	if doc.AutoDisabled != nil {
		store.autoDisabled = doc.AutoDisabled
	}
	if doc.WarmupCompleted != nil {
		store.warmupCompleted = make(map[string]time.Time, len(doc.WarmupCompleted))
		for key, at := range doc.WarmupCompleted {
			store.warmupCompleted[key] = utcOrZero(at)
		}
	}
	return store, nil
}

// SaveAtomic writes the store document to disk using a temporary file and rename.
func (s *Store) SaveAtomic(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save state context: %w", err)
	}
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	s.mu.RLock()
	path := s.path
	doc := document{
		SchemaVersion:   SchemaVersion,
		Entries:         s.entries,
		LatestAudit:     s.latestAudit,
		LatestResult:    s.latestResult,
		RunHistory:      s.runHistory,
		ScheduleConfig:  s.scheduleConfig,
		DynamicConfig:   s.dynamicConfig,
		LastAutoApplyAt: s.lastAutoApplyAt,
		Cooldowns:       s.cooldowns,
		AutoDisabled:    s.autoDisabled,
		RequestHistory:  s.requestHistory,
		WarmupCompleted: s.warmupCompleted,
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("encode state cache: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create state cache dir %s: %w", dir, err)
	}
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create state cache temp: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err = tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write state cache temp: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("close state cache temp: %w", err)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename state cache temp: %w", err)
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
		NextProbeAt:              utcOrZero(success.NextProbeAt),
		AuthInvalid:              success.AuthInvalid,
		PlanType:                 success.PlanType,
		Source:                   success.Source,
	}
	s.entries[key] = entry
	return nil
}

// MarkProbeFailure records a probe failure and sets the next probe time and sanitized error message.
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
	entry.NextProbeAt = utcOrZero(failure.NextProbeAt)

	s.entries[key] = entry
	return nil
}

// MarkProbeScheduled records the next scheduled probe time without modifying existing evidence.
func (s *Store) MarkProbeScheduled(ctx context.Context, schedule ProbeSchedule) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("mark probe scheduled context: %w", err)
	}
	key := entryKey(schedule.AuthIndex, schedule.ModelGroup)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.entries[key]
	entry.SchemaVersion = SchemaVersion
	entry.Provider = schedule.Provider
	entry.ModelGroup = entryModelGroup(schedule.ModelGroup)
	entry.AuthIndex = authIndexKey(schedule.AuthIndex)
	entry.NextProbeAt = utcOrZero(schedule.NextProbeAt)

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

// HasEntry returns true if an entry exists for authIndex and modelGroup.
func (s *Store) HasEntry(authIndex, modelGroup string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[entryKey(authIndex, modelGroup)]
	return ok
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

// NeedsProbe evaluates whether the entry requires a fresh probe.
func (s *Store) NeedsProbe(ctx context.Context, check ProbeCheck) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("needs probe context: %w", err)
	}
	key := entryKey(check.AuthIndex, check.ModelGroup)

	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok || entry.SchemaVersion != SchemaVersion {
		return true, nil
	}
	if check.Provider != "" && entry.Provider != check.Provider {
		return true, nil
	}
	if entry.ModelGroup != entryModelGroup(check.ModelGroup) {
		return true, nil
	}
	// Window reset reached -> must re-probe
	if isResetReached(entry, check.Now) {
		return true, nil
	}
	if isResetTooOld(entry, check) {
		return true, nil
	}
	if isTTLExpired(entry, check) {
		return true, nil
	}
	if !entry.NextProbeAt.IsZero() && check.Now.Before(entry.NextProbeAt) {
		return false, nil
	}
	if !entry.NextProbeAt.IsZero() && !check.Now.Before(entry.NextProbeAt) {
		return true, nil
	}
	return false, nil
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

func isTTLExpired(entry Entry, check ProbeCheck) bool {
	return !entry.ObservedAt.IsZero() && check.Policy.TTL > 0 && !check.Now.Before(entry.ObservedAt.Add(check.Policy.TTL))
}

func isResetReached(entry Entry, now time.Time) bool {
	if !entry.ShortWindowResetAt.IsZero() && !now.Before(entry.ShortWindowResetAt) {
		return true
	}
	if !entry.ResetAt.IsZero() && !now.Before(entry.ResetAt) {
		return true
	}
	return false
}

func isResetTooOld(entry Entry, check ProbeCheck) bool {
	if check.Policy.ResetStaleAfter <= 0 {
		return false
	}
	if !entry.ShortWindowResetAt.IsZero() && check.Now.Sub(entry.ShortWindowResetAt) > check.Policy.ResetStaleAfter {
		return true
	}
	if !entry.ResetAt.IsZero() && check.Now.Sub(entry.ResetAt) > check.Policy.ResetStaleAfter {
		return true
	}
	return false
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

// GetCustomTier returns the custom tier configured for an auth index (0 if not set).
func (s *Store) GetCustomTier(authIndex string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dynamicConfig == nil || s.dynamicConfig.CustomTiers == nil {
		return 0
	}
	return s.dynamicConfig.CustomTiers[strings.TrimSpace(authIndex)]
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

// GetActiveCooldownsForGroup returns a map of auth_index -> CooldownUntil for unexpired
// account- or pool-scoped interventions affecting the specified model group.
func (s *Store) GetActiveCooldownsForGroup(now time.Time, modelGroup string) map[string]time.Time {
	entries := s.GetActiveCooldownEntriesForGroup(now, modelGroup)
	active := make(map[string]time.Time, len(entries))
	for authIndex, entry := range entries {
		active[authIndex] = entry.CooldownUntil
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

// GetActiveCooldowns returns a map of auth_index -> CooldownUntil for unexpired interventions.
// Expired entries remain available until Runtime has restored the Host state.
func (s *Store) GetActiveCooldowns(now time.Time) map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := make(map[string]time.Time)
	for k, v := range s.cooldowns {
		if now.Before(v.CooldownUntil) {
			active[k] = v.CooldownUntil
			if v.AuthIndex != "" {
				active[v.AuthIndex] = v.CooldownUntil
			}
		}
	}
	return active
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

// ClearExpiredCooldowns removes cooldowns whose CooldownUntil time has passed.
func (s *Store) ClearExpiredCooldowns(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.cooldowns {
		if !now.Before(v.CooldownUntil) {
			delete(s.cooldowns, k)
		}
	}
}

// GetAutoDisabled returns a copy of all current auto-disabled entries.
func (s *Store) GetAutoDisabled() map[string]AutoDisabledEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make(map[string]AutoDisabledEntry, len(s.autoDisabled))
	maps.Copy(copied, s.autoDisabled)
	return copied
}

// IsAutoDisabled checks if a credential is automatically disabled by the plugin.
func (s *Store) IsAutoDisabled(authIndex string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.autoDisabled[authIndex]
	return ok
}

// SetAutoDisabled records an auto-disabled state for a credential.
func (s *Store) SetAutoDisabled(entry AutoDisabledEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.autoDisabled == nil {
		s.autoDisabled = make(map[string]AutoDisabledEntry)
	}
	s.autoDisabled[entry.AuthIndex] = entry
}

// DeleteAutoDisabled removes an auto-disabled record for a credential.
func (s *Store) DeleteAutoDisabled(authIndex string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.autoDisabled, authIndex)
}

// Path returns the file path of the state cache.
func (s *Store) Path() string {
	return s.path
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
