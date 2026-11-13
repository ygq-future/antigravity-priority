package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
)

const (
	// PluginID is the identifier recognized by the CPA host.
	PluginID = "antigravity-priority"
	// DirectoryName is the directory name convention for the plugin.
	DirectoryName = "antigravity-priority"
	// DynamicLibraryBaseName is the base name of the dynamic shared library.
	DynamicLibraryBaseName = "antigravity-priority"
	// CPAConfigKey is the configuration key under plugins.configs in CPA.
	CPAConfigKey = "antigravity-priority"
	// DefaultStateCachePath is the main state file inside the plugin's dedicated cache directory.
	DefaultStateCachePath = "data/antigravity-priority/state.json"
	// LegacyDefaultStateCachePath is the pre-directory cache file migrated automatically.
	LegacyDefaultStateCachePath = "data/antigravity-priority-cache.json"

	// KeyEnabled is the JSON/YAML key for enabled.
	KeyEnabled = "enabled"
	// KeyAutoApply is the JSON key for auto_apply.
	KeyAutoApply = "auto_apply"
	// KeyInterval is the JSON key for interval.
	KeyInterval = "interval"
	// KeyMaxConcurrency is the JSON key for max_concurrency.
	KeyMaxConcurrency = "max_concurrency"
	// KeyUrgencyTolerance is the JSON key for urgency_tolerance.
	KeyUrgencyTolerance = "urgency_tolerance"
	// KeyRateLimitCooldownMinutes is the JSON key for rate_limit_cooldown_minutes.
	KeyRateLimitCooldownMinutes = "rate_limit_cooldown_minutes"
	// KeyQuotaSampleCapacity is the JSON key for quota_sample_capacity.
	KeyQuotaSampleCapacity = "quota_sample_capacity"
	// KeyRequestHistoryLimit is the JSON key for request_history_limit.
	KeyRequestHistoryLimit = "request_history_limit"
	// KeyStateCachePath is the JSON/YAML key for state_cache_path.
	KeyStateCachePath = "state_cache_path"
	// KeySchedule is the JSON key for schedule.
	KeySchedule = "schedule"
	// KeyPaused is the JSON key for paused.
	KeyPaused = "paused"
	// KeyCustomTiers is the JSON key for custom_tiers.
	KeyCustomTiers = "custom_tiers"
	// DefaultGeminiWarmupModel is the default lightweight model used for Gemini 5h warm-up.
	DefaultGeminiWarmupModel = "gemini-3-flash"
	// DefaultClaudeWarmupModel is the default lightweight model used for Claude 5h warm-up.
	DefaultClaudeWarmupModel = "claude-sonnet-5-5-high"

	// DefaultUrgencyTolerance is the default numerical delta threshold below which adjacent credentials share identical priority.
	DefaultUrgencyTolerance = 0.05
	// DefaultRateLimitCooldownMinutes is the default 429 rate limit reactive cooldown duration in minutes.
	DefaultRateLimitCooldownMinutes = 5
	// DefaultQuotaSampleCapacity is the default sliding window capacity for multi-sample estimation.
	DefaultQuotaSampleCapacity = 6
	// DefaultRequestHistoryLimit is the default number of recent requests retained for display.
	DefaultRequestHistoryLimit = 100
	// MinRequestHistoryLimit is the minimum allowed request history retention.
	MinRequestHistoryLimit = 1
	// MaxRequestHistoryLimit is the maximum allowed request history retention.
	MaxRequestHistoryLimit = 1000
	// MinQuotaSampleCapacity is the minimum allowed sample capacity.
	MinQuotaSampleCapacity = 2
	// MaxQuotaSampleCapacity is the maximum allowed sample capacity.
	MaxQuotaSampleCapacity = 30
	// MinMaxConcurrency is the minimum allowed concurrency.
	MinMaxConcurrency = 1
	// MaxMaxConcurrency is the maximum allowed concurrency.
	MaxMaxConcurrency = 32
	// MaxUrgencyTolerance is the maximum allowed urgency clustering tolerance.
	MaxUrgencyTolerance = 0.5
	// MinRateLimitCooldownMinutes is the minimum 429 cooldown duration.
	MinRateLimitCooldownMinutes = 1
	// MaxRateLimitCooldownMinutes is the maximum 429 cooldown duration.
	MaxRateLimitCooldownMinutes = 1440
)

// ErrInvalidConfig indicates configuration parsing or validation failure.
var ErrInvalidConfig = errors.New("config: invalid")

// Config represents the validated complete configuration of the antigravity-priority plugin.
type Config struct {
	Enabled                  bool
	AutoApply                bool
	Interval                 time.Duration
	MaxConcurrency           int
	UrgencyTolerance         float64
	RateLimitCooldownMinutes int
	QuotaSampleCapacity      int
	RequestHistoryLimit      int
	StateCachePath           string
	Schedule                 ScheduleConfig
	CustomTiers              map[string]int
	Warmup                   WarmupConfig
	ExcludedModels           []string
}

// WarmupConfig encapsulates 5-hour short-window reset countdown warm-up settings.
type WarmupConfig struct {
	GeminiEnabled bool   `json:"gemini_enabled"`
	ClaudeEnabled bool   `json:"claude_enabled"`
	GeminiModel   string `json:"gemini_model"`
	ClaudeModel   string `json:"claude_model"`
	// Overrides holds card-level warm-up states keyed by auth_index. Presence
	// means a local override of the global pool toggle; absence means the card
	// follows the global switch. A local value equal to the global is cleared by
	// the setter so the card rejoins the global state.
	Overrides map[string]bool `json:"overrides,omitempty"`
}

// DefaultWarmupConfig returns the default warm-up configuration.
func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{
		GeminiEnabled: false,
		ClaudeEnabled: false,
		GeminiModel:   DefaultGeminiWarmupModel,
		ClaudeModel:   DefaultClaudeWarmupModel,
	}
}

// Normalize ensures empty model names fall back to canonical defaults and the
// per-credential override list holds only unique, non-empty auth indexes.
func (w *WarmupConfig) Normalize() {
	if strings.TrimSpace(w.GeminiModel) == "" {
		w.GeminiModel = DefaultGeminiWarmupModel
	} else {
		w.GeminiModel = strings.TrimSpace(w.GeminiModel)
	}
	if strings.TrimSpace(w.ClaudeModel) == "" {
		w.ClaudeModel = DefaultClaudeWarmupModel
	} else {
		w.ClaudeModel = strings.TrimSpace(w.ClaudeModel)
	}
	w.Overrides = normalizeWarmupOverrides(w.Overrides)
}

func normalizeWarmupOverrides(src map[string]bool) map[string]bool {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]bool, len(src))
	for raw, enabled := range src {
		idx := strings.TrimSpace(raw)
		if idx == "" {
			continue
		}
		out[idx] = enabled
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Clone returns a deep copy so configuration snapshots never share the
// override map with the persisted document.
func (w WarmupConfig) Clone() WarmupConfig {
	clone := w
	if len(w.Overrides) == 0 {
		clone.Overrides = nil
		return clone
	}
	clone.Overrides = make(map[string]bool, len(w.Overrides))
	maps.Copy(clone.Overrides, w.Overrides)
	return clone
}

// HasEnabledOverride reports whether any credential is locally forced on, which
// keeps warm-up active even when both global pool toggles are off.
func (w WarmupConfig) HasEnabledOverride() bool {
	for _, enabled := range w.Overrides {
		if enabled {
			return true
		}
	}
	return false
}

// WarmupOverride returns the card-level warm-up state of one credential.
func (w WarmupConfig) WarmupOverride(authIndex string) (bool, bool) {
	idx := strings.TrimSpace(authIndex)
	if idx == "" || len(w.Overrides) == 0 {
		return false, false
	}
	enabled, ok := w.Overrides[idx]
	return enabled, ok
}

// SetWarmupOverride records one credential's local warm-up state.
func (w *WarmupConfig) SetWarmupOverride(authIndex string, enabled bool) {
	idx := strings.TrimSpace(authIndex)
	if idx == "" {
		return
	}
	if w.Overrides == nil {
		w.Overrides = make(map[string]bool, 1)
	}
	w.Overrides[idx] = enabled
}

// ClearWarmupOverride removes one credential's local override so it follows
// the global pool toggles again.
func (w *WarmupConfig) ClearWarmupOverride(authIndex string) {
	idx := strings.TrimSpace(authIndex)
	if idx == "" || len(w.Overrides) == 0 {
		return
	}
	delete(w.Overrides, idx)
	if len(w.Overrides) == 0 {
		w.Overrides = nil
	}
}


// DynamicConfig contains all runtime-customizable configuration parameters
// that can be modified via the UI Config Center without restarting the plugin (REQ-09).
type DynamicConfig struct {
	AutoApply                bool                `json:"auto_apply"`
	Interval                 string              `json:"interval"` // e.g. "15m", "30m"
	MaxConcurrency           int                 `json:"max_concurrency"`
	UrgencyTolerance         float64             `json:"urgency_tolerance"`           // e.g. 0.05
	RateLimitCooldownMinutes int                 `json:"rate_limit_cooldown_minutes"` // e.g. 5
	QuotaSampleCapacity      int                 `json:"quota_sample_capacity"`       // e.g. 6 (range 2..30)
	RequestHistoryLimit      int                 `json:"request_history_limit"`       // e.g. 100 (range 1..1000)
	Schedule                 ScheduleConfig      `json:"schedule"`
	CustomTiers              map[string]int      `json:"custom_tiers,omitempty"`
	Warmup                   WarmupConfig        `json:"warmup"`
	ExcludedModels           []string            `json:"excluded_models,omitempty"`
}

// UnmarshalJSON seeds omitted fields from the canonical defaults before
// applying persisted dynamic overrides. This keeps older documents round-trip valid.
func (dyn *DynamicConfig) UnmarshalJSON(data []byte) error {
	type dynamicConfigAlias DynamicConfig
	seeded := dynamicConfigAlias(Default().Dynamic())
	if err := json.Unmarshal(data, &seeded); err != nil {
		return err
	}
	*dyn = DynamicConfig(seeded)
	return nil
}

type rawConfig struct {
	Enabled        *bool   `json:"enabled"`
	StateCachePath *string `json:"state_cache_path"`
}

// Default returns the standard default configuration values.
func Default() Config {
	return Config{
		Enabled:                  true,
		AutoApply:                false,
		Interval:                 15 * time.Minute,
		MaxConcurrency:           6,
		UrgencyTolerance:         DefaultUrgencyTolerance,
		RateLimitCooldownMinutes: DefaultRateLimitCooldownMinutes,
		QuotaSampleCapacity:      DefaultQuotaSampleCapacity,
		RequestHistoryLimit:      DefaultRequestHistoryLimit,
		StateCachePath:           DefaultStateCachePath,
		Schedule: ScheduleConfig{
			Paused: false,
		},
		Warmup: DefaultWarmupConfig(),
	}
}

// Dynamic returns the DynamicConfig view of the current Config.
func (cfg Config) Dynamic() DynamicConfig {
	return DynamicConfig{
		AutoApply:                cfg.AutoApply,
		Interval:                 cfg.Interval.String(),
		MaxConcurrency:           cfg.MaxConcurrency,
		UrgencyTolerance:         cfg.UrgencyTolerance,
		RateLimitCooldownMinutes: cfg.RateLimitCooldownMinutes,
		QuotaSampleCapacity:      cfg.QuotaSampleCapacity,
		RequestHistoryLimit:      cfg.RequestHistoryLimit,
		Schedule:       cfg.Schedule,
		CustomTiers:    cloneCustomTiers(cfg.CustomTiers),
		Warmup:         cfg.Warmup.Clone(),
		ExcludedModels: NormalizeExcludedModels(cfg.ExcludedModels),
	}
}

func cloneCustomTiers(src map[string]int) map[string]int {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]int, len(src))
	for k, v := range src {
		if v > 0 {
			dst[k] = v
		}
	}
	return dst
}

// NormalizeExcludedModels canonicalizes a list of excluded model IDs by
// trimming whitespace, lowercasing, removing empties/duplicates, and sorting.
func NormalizeExcludedModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, m := range models {
		norm := strings.ToLower(strings.TrimSpace(m))
		if norm == "" {
			continue
		}
		if _, ok := seen[norm]; ok {
			continue
		}
		seen[norm] = struct{}{}
		out = append(out, norm)
	}
	if len(out) == 0 {
		return nil
	}
	slices.Sort(out)
	return out
}

// IsModelExcluded reports whether model matches an entry in excludedModels.
func IsModelExcluded(excludedModels []string, model string) bool {
	if len(excludedModels) == 0 {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(model))
	if target == "" {
		return false
	}
	for _, m := range excludedModels {
		if strings.ToLower(strings.TrimSpace(m)) == target {
			return true
		}
	}
	return false
}

// Validate validates all field boundaries in DynamicConfig.
func (dyn DynamicConfig) Validate() error {
	interval, err := time.ParseDuration(dyn.Interval)
	if err != nil || interval <= 0 {
		return fmt.Errorf("invalid interval %q: must be positive duration (e.g. 15m)", dyn.Interval)
	}
	if interval < time.Minute {
		return fmt.Errorf("interval %s too short: minimum is 1m", dyn.Interval)
	}
	if dyn.MaxConcurrency < MinMaxConcurrency || dyn.MaxConcurrency > MaxMaxConcurrency {
		return fmt.Errorf("max_concurrency must be between %d and %d, got %d", MinMaxConcurrency, MaxMaxConcurrency, dyn.MaxConcurrency)
	}
	if math.IsNaN(dyn.UrgencyTolerance) || math.IsInf(dyn.UrgencyTolerance, 0) || dyn.UrgencyTolerance < 0 || dyn.UrgencyTolerance > MaxUrgencyTolerance {
		return fmt.Errorf("urgency_tolerance must be between 0 and %.1f, got %v", MaxUrgencyTolerance, dyn.UrgencyTolerance)
	}
	if dyn.RateLimitCooldownMinutes < MinRateLimitCooldownMinutes || dyn.RateLimitCooldownMinutes > MaxRateLimitCooldownMinutes {
		return fmt.Errorf("rate_limit_cooldown_minutes must be between %d and %d, got %d", MinRateLimitCooldownMinutes, MaxRateLimitCooldownMinutes, dyn.RateLimitCooldownMinutes)
	}
	if dyn.QuotaSampleCapacity < MinQuotaSampleCapacity || dyn.QuotaSampleCapacity > MaxQuotaSampleCapacity {
		return fmt.Errorf("quota_sample_capacity must be between %d and %d, got %d", MinQuotaSampleCapacity, MaxQuotaSampleCapacity, dyn.QuotaSampleCapacity)
	}
	if dyn.RequestHistoryLimit < MinRequestHistoryLimit || dyn.RequestHistoryLimit > MaxRequestHistoryLimit {
		return fmt.Errorf("request_history_limit must be between %d and %d, got %d", MinRequestHistoryLimit, MaxRequestHistoryLimit, dyn.RequestHistoryLimit)
	}
	for authIdx, tier := range dyn.CustomTiers {
		if strings.TrimSpace(authIdx) == "" {
			return fmt.Errorf("custom_tiers contains empty auth_index")
		}
		if tier < 0 || tier > 99 {
			return fmt.Errorf("custom_tiers[%q] must be between 1 and 99 (0 to remove), got %d", authIdx, tier)
		}
	}
	for _, m := range dyn.ExcludedModels {
		if strings.TrimSpace(m) == "" {
			return fmt.Errorf("excluded_models contains empty model id")
		}
	}
	if dyn.Warmup.GeminiEnabled && strings.TrimSpace(dyn.Warmup.GeminiModel) == "" {
		return fmt.Errorf("warmup.gemini_model cannot be empty when gemini warm-up is enabled")
	}
	if dyn.Warmup.ClaudeEnabled && strings.TrimSpace(dyn.Warmup.ClaudeModel) == "" {
		return fmt.Errorf("warmup.claude_model cannot be empty when claude warm-up is enabled")
	}
	return nil
}

// ApplyTo validates and applies dynamic configuration overrides on top of a base Config.
func (dyn DynamicConfig) ApplyTo(base Config) (Config, error) {
	if err := dyn.Validate(); err != nil {
		return base, err
	}

	interval, _ := time.ParseDuration(dyn.Interval)

	res := base
	res.AutoApply = dyn.AutoApply
	res.Interval = interval
	res.MaxConcurrency = dyn.MaxConcurrency
	res.UrgencyTolerance = dyn.UrgencyTolerance
	res.RateLimitCooldownMinutes = dyn.RateLimitCooldownMinutes
	res.QuotaSampleCapacity = dyn.QuotaSampleCapacity
	res.RequestHistoryLimit = dyn.RequestHistoryLimit
	res.Schedule = dyn.Schedule
	res.CustomTiers = cloneCustomTiers(dyn.CustomTiers)
	res.Warmup = dyn.Warmup.Clone()
	res.Warmup.Normalize()
	res.ExcludedModels = NormalizeExcludedModels(dyn.ExcludedModels)
	return res, nil
}

// LoadBytes parses raw YAML or JSON bytes into a validated Config.
// In v1.1.0+, CPA host config.yaml only specifies enabled and state_cache_path;
// all operational scheduling parameters are managed dynamically in DynamicConfig.
func LoadBytes(data []byte) (Config, []string, error) {
	raw, err := decodeRaw(data)
	if err != nil {
		return Config{}, nil, fmt.Errorf("parse config: %w", err)
	}
	cfg, warnings := raw.applyTolerant(Default())
	return cfg, warnings, nil
}

func decodeRaw(data []byte) (rawConfig, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return rawConfig{}, nil
	}
	var raw rawConfig
	if trimmed[0] == '{' {
		if err := json.Unmarshal(trimmed, &raw); err != nil {
			return rawConfig{}, invalid("config", err.Error(), "must match config schema")
		}
		return raw, nil
	}
	yamlMap, err := parseYAMLMap(extractPluginConfigYAML(string(trimmed)))
	if err != nil {
		return rawConfig{}, err
	}
	encoded, err := json.Marshal(yamlMap)
	if err != nil {
		return rawConfig{}, invalid("config", "yaml", "must be encodable")
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return rawConfig{}, invalid("config", err.Error(), "must match config schema")
	}
	return raw, nil
}

func (raw rawConfig) applyTolerant(cfg Config) (Config, []string) {
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	}
	if raw.StateCachePath != nil && strings.TrimSpace(*raw.StateCachePath) != "" {
		cfg.StateCachePath = strings.TrimSpace(*raw.StateCachePath)
	}
	return cfg, nil
}
