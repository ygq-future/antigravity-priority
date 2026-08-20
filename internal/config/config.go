package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
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
	MaxMaxConcurrency = 64
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
	Enabled             bool
	BoostStartPriority  int
	NormalStartPriority int
}

// PriorityRulesConfig holds priority rule settings for DynamicConfig.
type PriorityRulesConfig struct {
	Enabled             bool `json:"enabled"`
	BoostStartPriority  int  `json:"boost_start_priority"`
	NormalStartPriority int  `json:"normal_start_priority"`
}

// DynamicConfig contains all runtime-customizable configuration parameters
// that can be modified via the UI Config Center without restarting the plugin (REQ-09).
type DynamicConfig struct {
	AutoApply                bool                `json:"auto_apply"`
	Interval                 string              `json:"interval"`                    // e.g. "15m", "30m"
	AntigravityModelGroup    string              `json:"antigravity_model_group"`     // "gemini" or "claude_gpt"
	MaxConcurrency           int                 `json:"max_concurrency"`
	MinChange                int                 `json:"min_change"`
	UrgencyTolerance         float64             `json:"urgency_tolerance"`           // e.g. 0.05
	RateLimitCooldownMinutes int                 `json:"rate_limit_cooldown_minutes"` // e.g. 5
	QuotaSampleCapacity      int                 `json:"quota_sample_capacity"`       // e.g. 6 (range 2..30)
	PriorityRules            PriorityRulesConfig `json:"priority_rules"`
	Schedule                 ScheduleConfig      `json:"schedule"`
}

type rawConfig struct {
	Enabled                  *bool              `json:"enabled"`
	AutoApply                *bool              `json:"auto_apply"`
	Interval                 *rawDuration       `json:"interval"`
	AntigravityModelGroup    *string            `json:"antigravity_model_group"`
	MaxConcurrency           *int               `json:"max_concurrency"`
	MinChange                *int               `json:"min_change"`
	UrgencyTolerance         *float64           `json:"urgency_tolerance"`
	RateLimitCooldownMinutes *int               `json:"rate_limit_cooldown_minutes"`
	QuotaSampleCapacity      *int               `json:"quota_sample_capacity"`
	StateCachePath           *string            `json:"state_cache_path"`
	CachePath                *string            `json:"cache_path"`
	PriorityRules            *rawPriorityRules  `json:"priority_rules"`
	Schedule                 *rawScheduleConfig `json:"schedule"`
}

type rawPriorityRules struct {
	Enabled             *bool `json:"enabled"`
	BoostStartPriority  *int  `json:"boost_start_priority"`
	NormalStartPriority *int  `json:"normal_start_priority"`
}

type rawScheduleConfig struct {
	Paused        *bool   `json:"paused"`
	WindowEnabled *bool   `json:"window_enabled"`
	WindowStart   *string `json:"window_start"`
	WindowEnd     *string `json:"window_end"`
}

type rawDuration string

func (duration *rawDuration) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		return nil
	}
	var text string
	if len(trimmed) > 0 && trimmed[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
	} else {
		text = string(trimmed)
	}
	*duration = rawDuration(text)
	return nil
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
		PriorityRules: PriorityRules{
			Enabled:             true,
			BoostStartPriority:  999,
			NormalStartPriority: 100,
		},
		Schedule: ScheduleConfig{
			Paused:        false,
			WindowEnabled: false,
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
			Enabled:             cfg.PriorityRules.Enabled,
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
	if dyn.MinChange < 0 || dyn.MinChange > 1000 {
		return fmt.Errorf("min_change must be between 0 and 1000, got %d", dyn.MinChange)
	}
	if dyn.PriorityRules.BoostStartPriority < MinPriorityValue || dyn.PriorityRules.BoostStartPriority > MaxPriorityValue {
		return fmt.Errorf("boost_start_priority must be between %d and %d, got %d", MinPriorityValue, MaxPriorityValue, dyn.PriorityRules.BoostStartPriority)
	}
	if dyn.PriorityRules.NormalStartPriority < MinPriorityValue || dyn.PriorityRules.NormalStartPriority > MaxPriorityValue {
		return fmt.Errorf("normal_start_priority must be between %d and %d, got %d", MinPriorityValue, MaxPriorityValue, dyn.PriorityRules.NormalStartPriority)
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
	if dyn.UrgencyTolerance <= 0 {
		dyn.UrgencyTolerance = DefaultUrgencyTolerance
	}
	if dyn.RateLimitCooldownMinutes <= 0 {
		dyn.RateLimitCooldownMinutes = DefaultRateLimitCooldownMinutes
	}
	if dyn.QuotaSampleCapacity <= 0 {
		dyn.QuotaSampleCapacity = DefaultQuotaSampleCapacity
	}

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
	res.PriorityRules.Enabled = dyn.PriorityRules.Enabled
	res.PriorityRules.BoostStartPriority = dyn.PriorityRules.BoostStartPriority
	res.PriorityRules.NormalStartPriority = dyn.PriorityRules.NormalStartPriority
	res.Schedule = dyn.Schedule
	return res, nil
}

// LoadBytes parses raw YAML or JSON bytes into a validated Config.
// Field-level validation issues produce warnings with smooth fallback to defaults
// rather than hard errors. Only structurally unparseable input returns an error.
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
		var generic map[string]any
		if err := json.Unmarshal(trimmed, &generic); err != nil {
			return rawConfig{}, invalid("config", "json", "must be valid JSON")
		}
		normalizePriorityRulesKeys(generic)
		encoded, err := json.Marshal(generic)
		if err != nil {
			return rawConfig{}, invalid("config", "json", "must be encodable")
		}
		if err := json.Unmarshal(encoded, &raw); err != nil {
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
	var warnings []string
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	}
	if raw.AutoApply != nil {
		cfg.AutoApply = *raw.AutoApply
	}
	if raw.AntigravityModelGroup != nil {
		modelGroup, err := ParseAntigravityModelGroup(*raw.AntigravityModelGroup)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"antigravity_model_group=%q is invalid, falling back to default 'gemini'",
				*raw.AntigravityModelGroup))
		} else {
			cfg.AntigravityModelGroup = modelGroup
		}
	}
	if raw.Interval != nil {
		parsed, normalized, err := parseDurationTolerant("interval", string(*raw.Interval))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf(
				"interval=%q is invalid, falling back to default '15m'",
				string(*raw.Interval)))
		} else {
			cfg.Interval = parsed
			if normalized != "" {
				warnings = append(warnings, fmt.Sprintf(
					"interval=%q normalized to '%s'",
					string(*raw.Interval), normalized))
			}
		}
	}
	if raw.MaxConcurrency != nil {
		if *raw.MaxConcurrency < MinMaxConcurrency || *raw.MaxConcurrency > MaxMaxConcurrency {
			warnings = append(warnings, fmt.Sprintf(
				"max_concurrency=%d is invalid (must be %d..%d), falling back to default '6'",
				*raw.MaxConcurrency, MinMaxConcurrency, MaxMaxConcurrency))
		} else {
			cfg.MaxConcurrency = *raw.MaxConcurrency
		}
	}
	if raw.MinChange != nil {
		if *raw.MinChange < 0 {
			warnings = append(warnings, fmt.Sprintf(
				"min_change=%d is invalid (<0), falling back to default '1'",
				*raw.MinChange))
		} else {
			cfg.MinChange = *raw.MinChange
		}
	}
	if raw.UrgencyTolerance != nil {
		if *raw.UrgencyTolerance <= 0 {
			warnings = append(warnings, fmt.Sprintf(
				"urgency_tolerance=%.4f is invalid (<=0), falling back to default '%.2f'",
				*raw.UrgencyTolerance, DefaultUrgencyTolerance))
		} else {
			cfg.UrgencyTolerance = *raw.UrgencyTolerance
		}
	}
	if raw.RateLimitCooldownMinutes != nil {
		if *raw.RateLimitCooldownMinutes <= 0 {
			warnings = append(warnings, fmt.Sprintf(
				"rate_limit_cooldown_minutes=%d is invalid (<=0), falling back to default '%d'",
				*raw.RateLimitCooldownMinutes, DefaultRateLimitCooldownMinutes))
		} else {
			cfg.RateLimitCooldownMinutes = *raw.RateLimitCooldownMinutes
		}
	}
	if raw.QuotaSampleCapacity != nil {
		if *raw.QuotaSampleCapacity < MinQuotaSampleCapacity || *raw.QuotaSampleCapacity > MaxQuotaSampleCapacity {
			warnings = append(warnings, fmt.Sprintf(
				"quota_sample_capacity=%d is invalid (must be %d..%d), falling back to default '%d'",
				*raw.QuotaSampleCapacity, MinQuotaSampleCapacity, MaxQuotaSampleCapacity, DefaultQuotaSampleCapacity))
		} else {
			cfg.QuotaSampleCapacity = *raw.QuotaSampleCapacity
		}
	}
	if raw.StateCachePath != nil && strings.TrimSpace(*raw.StateCachePath) != "" {
		cfg.StateCachePath = strings.TrimSpace(*raw.StateCachePath)
	} else if raw.CachePath != nil && strings.TrimSpace(*raw.CachePath) != "" {
		cfg.StateCachePath = strings.TrimSpace(*raw.CachePath)
	}
	if raw.PriorityRules != nil {
		rules, ruleWarnings := raw.PriorityRules.applyTolerant(cfg.PriorityRules)
		cfg.PriorityRules = rules
		warnings = append(warnings, ruleWarnings...)
	}
	if raw.Schedule != nil {
		sched, schedWarnings := raw.Schedule.applyTolerant(cfg.Schedule)
		cfg.Schedule = sched
		warnings = append(warnings, schedWarnings...)
	}
	return cfg, warnings
}

func (raw *rawPriorityRules) applyTolerant(rules PriorityRules) (PriorityRules, []string) {
	var warnings []string
	if raw.Enabled != nil {
		rules.Enabled = *raw.Enabled
	}
	if raw.BoostStartPriority != nil {
		if *raw.BoostStartPriority < MinPriorityValue || *raw.BoostStartPriority > MaxPriorityValue {
			warnings = append(warnings, fmt.Sprintf(
				"priority_rules.boost_start_priority=%d is invalid (must be %d..%d), falling back to default '999'",
				*raw.BoostStartPriority, MinPriorityValue, MaxPriorityValue))
		} else {
			rules.BoostStartPriority = *raw.BoostStartPriority
		}
	}
	if raw.NormalStartPriority != nil {
		if *raw.NormalStartPriority < MinPriorityValue || *raw.NormalStartPriority > MaxPriorityValue {
			warnings = append(warnings, fmt.Sprintf(
				"priority_rules.normal_start_priority=%d is invalid (must be %d..%d), falling back to default '100'",
				*raw.NormalStartPriority, MinPriorityValue, MaxPriorityValue))
		} else {
			rules.NormalStartPriority = *raw.NormalStartPriority
		}
	}
	return rules, warnings
}

func (raw *rawScheduleConfig) applyTolerant(sched ScheduleConfig) (ScheduleConfig, []string) {
	var warnings []string
	if raw.Paused != nil {
		sched.Paused = *raw.Paused
	}
	if raw.WindowEnabled != nil {
		sched.WindowEnabled = *raw.WindowEnabled
	}
	if raw.WindowStart != nil {
		sched.WindowStart = strings.TrimSpace(*raw.WindowStart)
	}
	if raw.WindowEnd != nil {
		sched.WindowEnd = strings.TrimSpace(*raw.WindowEnd)
	}
	if err := ValidateScheduleWindow(sched.WindowStart, sched.WindowEnd); err != nil {
		warnings = append(warnings, fmt.Sprintf("schedule window %q-%q is invalid (%v), falling back to disabled", sched.WindowStart, sched.WindowEnd, err))
		sched.WindowEnabled = false
	}
	return sched, warnings
}
