package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	// DefaultStateCachePath is the default path for persisting probe state and burn rate metrics inside CPA's persistent data directory.
	DefaultStateCachePath = "data/antigravity-priority-cache.json"

	// KeyEnabled is the JSON/YAML key for enabled.
	KeyEnabled = "enabled"
	// KeyAutoApply is the JSON key for auto_apply.
	KeyAutoApply = "auto_apply"
	// KeyInterval is the JSON key for interval.
	KeyInterval = "interval"
	// KeyAntigravityModelGroup is the JSON key for antigravity_model_group.
	KeyAntigravityModelGroup = "antigravity_model_group"
	// KeyMaxConcurrency is the JSON key for max_concurrency.
	KeyMaxConcurrency = "max_concurrency"
	// KeyMinChange is the JSON key for min_change.
	KeyMinChange = "min_change"
	// KeyUrgencyTolerance is the JSON key for urgency_tolerance.
	KeyUrgencyTolerance = "urgency_tolerance"
	// KeyRateLimitCooldownMinutes is the JSON key for rate_limit_cooldown_minutes.
	KeyRateLimitCooldownMinutes = "rate_limit_cooldown_minutes"
	// KeyQuotaSampleCapacity is the JSON key for quota_sample_capacity.
	KeyQuotaSampleCapacity = "quota_sample_capacity"
	// KeyStateCachePath is the JSON/YAML key for state_cache_path.
	KeyStateCachePath = "state_cache_path"
	// KeyPriorityRules is the JSON key for priority_rules.
	KeyPriorityRules = "priority_rules"
	// KeyBoostStartPriority is the JSON key for boost_start_priority.
	KeyBoostStartPriority = "boost_start_priority"
	// KeyNormalStartPriority is the JSON key for normal_start_priority.
	KeyNormalStartPriority = "normal_start_priority"
	// KeySchedule is the JSON key for schedule.
	KeySchedule = "schedule"
	// KeyPaused is the JSON key for paused.
	KeyPaused = "paused"
	// KeyWindowEnabled is the JSON key for window_enabled.
	KeyWindowEnabled = "window_enabled"
	// KeyWindowStart is the JSON key for window_start.
	KeyWindowStart = "window_start"
	// KeyWindowEnd is the JSON key for window_end.
	KeyWindowEnd = "window_end"

	// DefaultUrgencyTolerance is the default numerical delta threshold below which adjacent credentials share identical priority.
	DefaultUrgencyTolerance = 0.05
	// DefaultRateLimitCooldownMinutes is the default 429 rate limit reactive cooldown duration in minutes.
	DefaultRateLimitCooldownMinutes = 5
	// DefaultQuotaSampleCapacity is the default sliding window capacity for multi-sample estimation.
	DefaultQuotaSampleCapacity = 6
	// MinQuotaSampleCapacity is the minimum allowed sample capacity.
	MinQuotaSampleCapacity = 2
	// MaxQuotaSampleCapacity is the maximum allowed sample capacity.
	MaxQuotaSampleCapacity = 30
	// MinMaxConcurrency is the minimum allowed concurrency.
	MinMaxConcurrency = 1
	// MaxMaxConcurrency is the maximum allowed concurrency.
	MaxMaxConcurrency = 32
	// MaxMinChange is the maximum allowed priority change threshold.
	MaxMinChange = 100
	// MaxUrgencyTolerance is the maximum allowed urgency clustering tolerance.
	MaxUrgencyTolerance = 0.5
	// MinRateLimitCooldownMinutes is the minimum 429 cooldown duration.
	MinRateLimitCooldownMinutes = 1
	// MaxRateLimitCooldownMinutes is the maximum 429 cooldown duration.
	MaxRateLimitCooldownMinutes = 1440
	// MinPriorityValue is the minimum allowed priority setting.
	MinPriorityValue = 1
	// MaxPriorityValue is the maximum allowed priority setting.
	MaxPriorityValue = 999
)

// ErrInvalidConfig indicates configuration parsing or validation failure.
var ErrInvalidConfig = errors.New("config: invalid")

// Config represents the validated complete configuration of the antigravity-priority plugin.
type Config struct {
	Enabled                  bool
	AutoApply                bool
	Interval                 time.Duration
	AntigravityModelGroup    AntigravityModelGroup
	MaxConcurrency           int
	MinChange                int
	UrgencyTolerance         float64
	RateLimitCooldownMinutes int
	QuotaSampleCapacity      int
	StateCachePath           string
	PriorityRules            PriorityRules
	Schedule                 ScheduleConfig
}

// PriorityRules contains the priority scoring configurations.
type PriorityRules struct {
	BoostStartPriority  int
	NormalStartPriority int
}

// PriorityRulesConfig holds priority rule settings for DynamicConfig.
type PriorityRulesConfig struct {
	BoostStartPriority  int `json:"boost_start_priority"`
	NormalStartPriority int `json:"normal_start_priority"`
}

// UnmarshalJSON migrates legacy priority_rules.enabled documents. Custom
// values from enabled=false configurations were previously inactive, so they
// resolve to the canonical defaults instead of becoming active unexpectedly.
func (cfg *PriorityRulesConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled             *bool `json:"enabled"`
		BoostStartPriority  *int  `json:"boost_start_priority"`
		NormalStartPriority *int  `json:"normal_start_priority"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	defaults := defaultPriorityRules()
	cfg.BoostStartPriority = defaults.BoostStartPriority
	cfg.NormalStartPriority = defaults.NormalStartPriority
	if raw.Enabled != nil && !*raw.Enabled {
		return nil
	}
	if raw.BoostStartPriority != nil {
		cfg.BoostStartPriority = *raw.BoostStartPriority
	}
	if raw.NormalStartPriority != nil {
		cfg.NormalStartPriority = *raw.NormalStartPriority
	}
	return nil
}

func defaultPriorityRules() PriorityRules {
	return PriorityRules{BoostStartPriority: 999, NormalStartPriority: 100}
}

// DynamicConfig contains all runtime-customizable configuration parameters
// that can be modified via the UI Config Center without restarting the plugin (REQ-09).
type DynamicConfig struct {
	AutoApply                bool                `json:"auto_apply"`
	Interval                 string              `json:"interval"`                // e.g. "15m", "30m"
	AntigravityModelGroup    string              `json:"antigravity_model_group"` // "gemini" or "claude_gpt"
	MaxConcurrency           int                 `json:"max_concurrency"`
	MinChange                int                 `json:"min_change"`
	UrgencyTolerance         float64             `json:"urgency_tolerance"`           // e.g. 0.05
	RateLimitCooldownMinutes int                 `json:"rate_limit_cooldown_minutes"` // e.g. 5
	QuotaSampleCapacity      int                 `json:"quota_sample_capacity"`       // e.g. 6 (range 2..30)
	PriorityRules            PriorityRulesConfig `json:"priority_rules"`
	Schedule                 ScheduleConfig      `json:"schedule"`
}

// UnmarshalJSON seeds omitted fields from the canonical defaults before
// applying persisted dynamic overrides. This keeps older documents that lack
// priority_rules (or newer fields) round-trip valid.
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
		AntigravityModelGroup:    AntigravityModelGroupGemini,
		MaxConcurrency:           6,
		MinChange:                1,
		UrgencyTolerance:         DefaultUrgencyTolerance,
		RateLimitCooldownMinutes: DefaultRateLimitCooldownMinutes,
		QuotaSampleCapacity:      DefaultQuotaSampleCapacity,
		StateCachePath:           DefaultStateCachePath,
		PriorityRules:            defaultPriorityRules(),
		Schedule: ScheduleConfig{
			Paused:        false,
			WindowEnabled: false,
			WindowStart:   "00:00",
			WindowEnd:     "23:59",
		},
	}
}

// Dynamic returns the DynamicConfig view of the current Config.
func (cfg Config) Dynamic() DynamicConfig {
	return DynamicConfig{
		AutoApply:                cfg.AutoApply,
		Interval:                 cfg.Interval.String(),
		AntigravityModelGroup:    string(cfg.AntigravityModelGroup),
		MaxConcurrency:           cfg.MaxConcurrency,
		MinChange:                cfg.MinChange,
		UrgencyTolerance:         cfg.UrgencyTolerance,
		RateLimitCooldownMinutes: cfg.RateLimitCooldownMinutes,
		QuotaSampleCapacity:      cfg.QuotaSampleCapacity,
		PriorityRules: PriorityRulesConfig{
			BoostStartPriority:  cfg.PriorityRules.BoostStartPriority,
			NormalStartPriority: cfg.PriorityRules.NormalStartPriority,
		},
		Schedule: cfg.Schedule,
	}
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
	if _, err := ParseAntigravityModelGroup(dyn.AntigravityModelGroup); err != nil {
		return fmt.Errorf("invalid antigravity_model_group %q: must be 'gemini' or 'claude_gpt'", dyn.AntigravityModelGroup)
	}
	if dyn.MaxConcurrency < MinMaxConcurrency || dyn.MaxConcurrency > MaxMaxConcurrency {
		return fmt.Errorf("max_concurrency must be between %d and %d, got %d", MinMaxConcurrency, MaxMaxConcurrency, dyn.MaxConcurrency)
	}
	if dyn.MinChange < 0 || dyn.MinChange > MaxMinChange {
		return fmt.Errorf("min_change must be between 0 and %d, got %d", MaxMinChange, dyn.MinChange)
	}
	if math.IsNaN(dyn.UrgencyTolerance) || math.IsInf(dyn.UrgencyTolerance, 0) || dyn.UrgencyTolerance < 0 || dyn.UrgencyTolerance > MaxUrgencyTolerance {
		return fmt.Errorf("urgency_tolerance must be between 0 and %.1f, got %v", MaxUrgencyTolerance, dyn.UrgencyTolerance)
	}
	if dyn.RateLimitCooldownMinutes < MinRateLimitCooldownMinutes || dyn.RateLimitCooldownMinutes > MaxRateLimitCooldownMinutes {
		return fmt.Errorf("rate_limit_cooldown_minutes must be between %d and %d, got %d", MinRateLimitCooldownMinutes, MaxRateLimitCooldownMinutes, dyn.RateLimitCooldownMinutes)
	}
	if dyn.PriorityRules.BoostStartPriority < MinPriorityValue || dyn.PriorityRules.BoostStartPriority > MaxPriorityValue {
		return fmt.Errorf("boost_start_priority must be between %d and %d, got %d", MinPriorityValue, MaxPriorityValue, dyn.PriorityRules.BoostStartPriority)
	}
	if dyn.PriorityRules.NormalStartPriority < MinPriorityValue || dyn.PriorityRules.NormalStartPriority > MaxPriorityValue {
		return fmt.Errorf("normal_start_priority must be between %d and %d, got %d", MinPriorityValue, MaxPriorityValue, dyn.PriorityRules.NormalStartPriority)
	}
	if dyn.PriorityRules.NormalStartPriority > dyn.PriorityRules.BoostStartPriority {
		return fmt.Errorf("normal_start_priority must not exceed boost_start_priority, got %d > %d", dyn.PriorityRules.NormalStartPriority, dyn.PriorityRules.BoostStartPriority)
	}
	if dyn.QuotaSampleCapacity < MinQuotaSampleCapacity || dyn.QuotaSampleCapacity > MaxQuotaSampleCapacity {
		return fmt.Errorf("quota_sample_capacity must be between %d and %d, got %d", MinQuotaSampleCapacity, MaxQuotaSampleCapacity, dyn.QuotaSampleCapacity)
	}
	if err := ValidateScheduleWindow(dyn.Schedule.WindowStart, dyn.Schedule.WindowEnd); err != nil {
		return err
	}
	return nil
}

// ApplyTo validates and applies dynamic configuration overrides on top of a base Config.
func (dyn DynamicConfig) ApplyTo(base Config) (Config, error) {
	if err := dyn.Validate(); err != nil {
		return base, err
	}

	interval, _ := time.ParseDuration(dyn.Interval)
	modelGroup, _ := ParseAntigravityModelGroup(dyn.AntigravityModelGroup)

	res := base
	res.AutoApply = dyn.AutoApply
	res.Interval = interval
	res.AntigravityModelGroup = modelGroup
	res.MaxConcurrency = dyn.MaxConcurrency
	res.MinChange = dyn.MinChange
	res.UrgencyTolerance = dyn.UrgencyTolerance
	res.RateLimitCooldownMinutes = dyn.RateLimitCooldownMinutes
	res.QuotaSampleCapacity = dyn.QuotaSampleCapacity
	res.PriorityRules.BoostStartPriority = dyn.PriorityRules.BoostStartPriority
	res.PriorityRules.NormalStartPriority = dyn.PriorityRules.NormalStartPriority
	res.Schedule = dyn.Schedule
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
