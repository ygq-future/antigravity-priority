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
)

// ErrInvalidConfig indicates configuration parsing or validation failure.
var ErrInvalidConfig = errors.New("config: invalid")

// Config represents the validated configuration of the antigravity-priority plugin.
type Config struct {
	Enabled               bool
	AutoApply             bool
	Interval              time.Duration
	AntigravityModelGroup AntigravityModelGroup
	MaxConcurrency        int
	MinChange             int
	StateCachePath        string
	PriorityRules         PriorityRules
}

// PriorityRules contains the priority scoring configurations.
type PriorityRules struct {
	Enabled             bool
	BoostStartPriority  int
	NormalStartPriority int
}

type rawConfig struct {
	Enabled               *bool             `json:"enabled"`
	AutoApply             *bool             `json:"auto_apply"`
	Interval              *rawDuration      `json:"interval"`
	AntigravityModelGroup *string           `json:"antigravity_model_group"`
	MaxConcurrency        *int              `json:"max_concurrency"`
	MinChange             *int              `json:"min_change"`
	StateCachePath        *string           `json:"state_cache_path"`
	CachePath             *string           `json:"cache_path"`
	PriorityRules         *rawPriorityRules `json:"priority_rules"`
}

type rawPriorityRules struct {
	Enabled             *bool `json:"enabled"`
	BoostStartPriority  *int  `json:"boost_start_priority"`
	NormalStartPriority *int  `json:"normal_start_priority"`
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
		Enabled:               true,
		AutoApply:             false,
		Interval:              15 * time.Minute,
		AntigravityModelGroup: AntigravityModelGroupGemini,
		MaxConcurrency:        6,
		MinChange:             1,
		StateCachePath:        DefaultStateCachePath,
		PriorityRules: PriorityRules{
			Enabled:             true,
			BoostStartPriority:  999,
			NormalStartPriority: 100,
		},
	}
}

// LoadBytes parses raw YAML or JSON bytes into a validated Config.
func LoadBytes(data []byte) (Config, error) {
	raw, err := decodeRaw(data)
	if err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return raw.apply(Default())
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

func (raw rawConfig) apply(cfg Config) (Config, error) {
	if raw.Enabled != nil {
		cfg.Enabled = *raw.Enabled
	}
	if raw.AutoApply != nil {
		cfg.AutoApply = *raw.AutoApply
	}
	if raw.AntigravityModelGroup != nil {
		modelGroup, err := ParseAntigravityModelGroup(*raw.AntigravityModelGroup)
		if err != nil {
			return Config{}, err
		}
		cfg.AntigravityModelGroup = modelGroup
	}
	if raw.Interval != nil {
		parsed, err := parseDuration("interval", string(*raw.Interval))
		if err != nil {
			return Config{}, err
		}
		cfg.Interval = parsed
	}
	if raw.MaxConcurrency != nil {
		if *raw.MaxConcurrency < 1 {
			return Config{}, invalid("max_concurrency", fmt.Sprint(*raw.MaxConcurrency), "must be at least 1")
		}
		cfg.MaxConcurrency = *raw.MaxConcurrency
	}
	if raw.MinChange != nil {
		if *raw.MinChange < 0 {
			return Config{}, invalid("min_change", fmt.Sprint(*raw.MinChange), "must be at least 0")
		}
		cfg.MinChange = *raw.MinChange
	}
	if raw.StateCachePath != nil && strings.TrimSpace(*raw.StateCachePath) != "" {
		cfg.StateCachePath = strings.TrimSpace(*raw.StateCachePath)
	} else if raw.CachePath != nil && strings.TrimSpace(*raw.CachePath) != "" {
		cfg.StateCachePath = strings.TrimSpace(*raw.CachePath)
	}
	if raw.PriorityRules != nil {
		priorityRules, err := raw.PriorityRules.apply(cfg.PriorityRules)
		if err != nil {
			return Config{}, err
		}
		cfg.PriorityRules = priorityRules
	}
	return cfg, nil
}

func (raw *rawPriorityRules) apply(rules PriorityRules) (PriorityRules, error) {
	if raw.Enabled != nil {
		rules.Enabled = *raw.Enabled
	}
	if raw.BoostStartPriority != nil {
		if *raw.BoostStartPriority < 1 {
			return PriorityRules{}, invalid("priority_rules.boost_start_priority", fmt.Sprint(*raw.BoostStartPriority), "must be at least 1")
		}
		rules.BoostStartPriority = *raw.BoostStartPriority
	}
	if raw.NormalStartPriority != nil {
		if *raw.NormalStartPriority < 1 {
			return PriorityRules{}, invalid("priority_rules.normal_start_priority", fmt.Sprint(*raw.NormalStartPriority), "must be at least 1")
		}
		rules.NormalStartPriority = *raw.NormalStartPriority
	}
	return rules, nil
}
