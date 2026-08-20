package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
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
	SchemaVersion        int           `json:"schema_version"`
	Provider             core.Provider `json:"provider"`
	ModelGroup           string        `json:"model_group,omitempty"`
	AuthIndex            string        `json:"auth_index"`
	ObservedAt           time.Time     `json:"observed_at"`
	ResetAt              time.Time     `json:"reset_at,omitempty"`
	Remaining            int64         `json:"remaining"`
	ShortWindowResetAt   time.Time     `json:"short_window_reset_at,omitempty"`
	ShortWindowRemaining *int64        `json:"short_window_remaining,omitempty"`
	LongWindowResetAt    time.Time     `json:"long_window_reset_at,omitempty"`
	LongWindowRemaining  *int64        `json:"long_window_remaining,omitempty"`
	CycleBurnRate        float64       `json:"cycle_burn_rate"`
	Samples              []QuotaSample `json:"samples,omitempty"`
	LastError            string        `json:"last_error,omitempty"`
	NextProbeAt          time.Time     `json:"next_probe_at,omitempty"`
	AuthInvalid          bool          `json:"auth_invalid,omitempty"`
	PlanType             core.PlanType `json:"plan_type,omitempty"`
	Source               Source        `json:"source,omitempty"`
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
	NextProbeAt          time.Time
	AuthInvalid          bool
	PlanType             core.PlanType
	Source               Source
	SampleCapacity       int
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

// CooldownEntry tracks temporary 429 rate limit circuit breaking for a credential.
type CooldownEntry struct {
	AuthIndex     string    `json:"auth_index"`
	ModelGroup    string    `json:"model_group,omitempty"`
	TriggeredAt   time.Time `json:"triggered_at"`
	CooldownUntil time.Time `json:"cooldown_until"`
	Reason        string    `json:"reason"`
}

// PriorityRulesConfig holds priority rule settings for DynamicConfig.
type PriorityRulesConfig = config.PriorityRulesConfig

// Store manages the in-memory and on-disk state cache document.
type Store struct {
	mu             sync.RWMutex
	path           string
	entries        map[string]Entry
	latestAudit    string
	latestResult   []byte
	runHistory     []byte
	scheduleConfig *ScheduleConfig
	dynamicConfig  *DynamicConfig
	cooldowns      map[string]CooldownEntry
}

type document struct {
	SchemaVersion  int                      `json:"schema_version"`
	Entries        map[string]Entry         `json:"entries"`
	LatestAudit    string                   `json:"latest_audit,omitempty"`
	LatestResult   json.RawMessage          `json:"latest_result,omitempty"`
	RunHistory     json.RawMessage          `json:"run_history,omitempty"`
	ScheduleConfig *ScheduleConfig          `json:"schedule_config,omitempty"`
	DynamicConfig  *DynamicConfig           `json:"app_config,omitempty"`
	Cooldowns      map[string]CooldownEntry `json:"cooldowns,omitempty"`
}

// Load loads the state document from path. If the file does not exist, an empty store is returned.
func Load(ctx context.Context, path string) (*Store, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("load state context: %w", err)
	}
	store := &Store{path: path, entries: make(map[string]Entry), cooldowns: make(map[string]CooldownEntry)}
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
	}
	store.latestAudit = doc.LatestAudit
	if len(doc.LatestResult) > 0 {
		store.latestResult = append([]byte(nil), doc.LatestResult...)
	}
	if len(doc.RunHistory) > 0 {
		store.runHistory = append([]byte(nil), doc.RunHistory...)
	}
	if doc.ScheduleConfig != nil {
		sc := *doc.ScheduleConfig
		store.scheduleConfig = &sc
	}
	if doc.DynamicConfig != nil {
		dc := *doc.DynamicConfig
		store.dynamicConfig = &dc
	}
	if doc.Cooldowns != nil {
		store.cooldowns = doc.Cooldowns
	}
	return store, nil
}

// SaveAtomic writes the store document to disk using a temporary file and rename.
func (s *Store) SaveAtomic(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("save state context: %w", err)
	}
	s.mu.RLock()
	path := s.path
	doc := document{
		SchemaVersion:  SchemaVersion,
		Entries:        s.entries,
		LatestAudit:    s.latestAudit,
		LatestResult:   s.latestResult,
		RunHistory:     s.runHistory,
		ScheduleConfig: s.scheduleConfig,
		DynamicConfig:  s.dynamicConfig,
		Cooldowns:      s.cooldowns,
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
	tmpPath := filepath.Join(dir, filepath.Base(path)+".tmp")
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if err = os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write state cache temp: %w", err)
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
	newRate, newSamples := UpdateSamplesAndCycleBurnRate(
		prev.CycleBurnRate,
		prev.Samples,
		success.ObservedAt,
		success.ShortWindowResetAt,
		success.ShortWindowRemaining,
		success.LongWindowRemaining,
		capacity,
	)

	entry := Entry{
		SchemaVersion:        SchemaVersion,
		Provider:             success.Provider,
		ModelGroup:           entryModelGroup(success.ModelGroup),
		AuthIndex:            authIndexKey(success.AuthIndex),
		ObservedAt:           success.ObservedAt.UTC(),
		ResetAt:              utcOrZero(success.ResetAt),
		Remaining:            success.Remaining,
		ShortWindowResetAt:   utcOrZero(success.ShortWindowResetAt),
		ShortWindowRemaining: cloneInt64Ptr(success.ShortWindowRemaining),
		LongWindowResetAt:    utcOrZero(success.LongWindowResetAt),
		LongWindowRemaining:  cloneInt64Ptr(success.LongWindowRemaining),
		CycleBurnRate:        newRate,
		Samples:              newSamples,
		LastError:            "",
		NextProbeAt:          utcOrZero(success.NextProbeAt),
		AuthInvalid:          success.AuthInvalid,
		PlanType:             success.PlanType,
		Source:               success.Source,
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
	entry.ObservedAt = failure.ObservedAt.UTC()
	entry.LastError = sanitizeProbeError(failure.Err)
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
	for k, v := range s.entries {
		res[k] = v
	}
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
	if s.scheduleConfig == nil {
		return ScheduleConfig{}
	}
	return *s.scheduleConfig
}

// SetScheduleConfig updates the persisted schedule configuration.
func (s *Store) SetScheduleConfig(cfg ScheduleConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduleConfig = &cfg
}

// GetDynamicConfig returns a copy of the persisted dynamic configuration and true if present.
func (s *Store) GetDynamicConfig() (DynamicConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.dynamicConfig == nil {
		return DynamicConfig{}, false
	}
	return *s.dynamicConfig, true
}

// SetDynamicConfig updates the persisted dynamic configuration.
func (s *Store) SetDynamicConfig(cfg DynamicConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dynamicConfig = &cfg
}

// GetActiveCooldowns returns a map of auth_index -> CooldownUntil for unexpired 429 rate limit cooldowns.
func (s *Store) GetActiveCooldowns(now time.Time) map[string]time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := make(map[string]time.Time)
	for k, v := range s.cooldowns {
		if now.Before(v.CooldownUntil) {
			active[k] = v.CooldownUntil
		}
	}
	return active
}

// GetCooldowns returns a copy of all current cooldown entries.
func (s *Store) GetCooldowns() map[string]CooldownEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make(map[string]CooldownEntry, len(s.cooldowns))
	for k, v := range s.cooldowns {
		copied[k] = v
	}
	return copied
}

// SetCooldown records or updates a 429 rate limit cooldown for a credential.
func (s *Store) SetCooldown(entry CooldownEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cooldowns == nil {
		s.cooldowns = make(map[string]CooldownEntry)
	}
	s.cooldowns[entry.AuthIndex] = entry
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

// Path returns the file path of the state cache.
func (s *Store) Path() string {
	return s.path
}
