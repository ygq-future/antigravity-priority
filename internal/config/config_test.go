package config_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()

	if !cfg.Enabled {
		t.Errorf("expected Enabled=true, got %v", cfg.Enabled)
	}
	if cfg.AutoApply {
		t.Errorf("expected AutoApply=false, got %v", cfg.AutoApply)
	}
	if cfg.Interval != 15*time.Minute {
		t.Errorf("expected Interval=15m, got %v", cfg.Interval)
	}
	if cfg.AntigravityModelGroup != config.AntigravityModelGroupGemini {
		t.Errorf("expected AntigravityModelGroup=gemini, got %v", cfg.AntigravityModelGroup)
	}
	if cfg.MaxConcurrency != 6 {
		t.Errorf("expected MaxConcurrency=6, got %v", cfg.MaxConcurrency)
	}
	if cfg.MinChange != 1 {
		t.Errorf("expected MinChange=1, got %v", cfg.MinChange)
	}
	if cfg.QuotaSampleCapacity != 6 {
		t.Errorf("expected QuotaSampleCapacity=6, got %v", cfg.QuotaSampleCapacity)
	}
	if cfg.StateCachePath != config.DefaultStateCachePath {
		t.Errorf("expected StateCachePath=%s, got %s", config.DefaultStateCachePath, cfg.StateCachePath)
	}
	if !cfg.PriorityRules.Enabled {
		t.Errorf("expected PriorityRules.Enabled=true, got %v", cfg.PriorityRules.Enabled)
	}
	if cfg.PriorityRules.BoostStartPriority != 999 {
		t.Errorf("expected BoostStartPriority=999, got %v", cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.PriorityRules.NormalStartPriority != 100 {
		t.Errorf("expected NormalStartPriority=100, got %v", cfg.PriorityRules.NormalStartPriority)
	}
}

func TestLoadBytes_Empty(t *testing.T) {
	cfg, warnings, err := config.LoadBytes(nil)
	if err != nil {
		t.Fatalf("unexpected error on nil: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config on nil, got %+v", cfg)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings on nil, got %v", warnings)
	}

	cfg, warnings, err = config.LoadBytes([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error on empty bytes: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config on empty bytes, got %+v", cfg)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings on empty bytes, got %v", warnings)
	}
}

func TestLoadBytes_JSON(t *testing.T) {
	jsonData := []byte(`{
		"enabled": false,
		"state_cache_path": "custom/path.json",
		"ignored_business_field": "some_value"
	}`)

	cfg, warnings, err := config.LoadBytes(jsonData)
	if err != nil {
		t.Fatalf("unexpected error loading json: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}

	if cfg.Enabled {
		t.Errorf("expected Enabled=false, got %v", cfg.Enabled)
	}
	if cfg.StateCachePath != "custom/path.json" {
		t.Errorf("expected StateCachePath='custom/path.json', got %v", cfg.StateCachePath)
	}
	// Defaults preserved for business fields
	if cfg.Interval != 15*time.Minute {
		t.Errorf("expected default Interval=15m, got %v", cfg.Interval)
	}
}

func TestLoadBytes_YAML(t *testing.T) {
	yamlData := []byte(`
# Plugin Configuration
enabled: false
state_cache_path: data/custom-cache.json
`)

	cfg, _, err := config.LoadBytes(yamlData)
	if err != nil {
		t.Fatalf("unexpected error loading yaml: %v", err)
	}

	if cfg.Enabled {
		t.Errorf("expected Enabled=false, got %v", cfg.Enabled)
	}
	if cfg.StateCachePath != "data/custom-cache.json" {
		t.Errorf("expected StateCachePath='data/custom-cache.json', got %v", cfg.StateCachePath)
	}
}

func TestLoadBytes_CPAPluginConfigPath(t *testing.T) {
	fullCPAConfig := []byte(`
plugins:
  configs:
    antigravity-priority:
      enabled: true
      state_cache_path: data/cpa-cache.json
`)

	cfg, _, err := config.LoadBytes(fullCPAConfig)
	if err != nil {
		t.Fatalf("unexpected error parsing CPA plugin config path: %v", err)
	}

	if !cfg.Enabled {
		t.Errorf("expected Enabled=true, got %v", cfg.Enabled)
	}
	if cfg.StateCachePath != "data/cpa-cache.json" {
		t.Errorf("expected StateCachePath='data/cpa-cache.json', got %v", cfg.StateCachePath)
	}
}

func TestLoadBytes_Invalid(t *testing.T) {
	structuralErrors := []struct {
		name string
		raw  string
	}{
		{
			name: "invalid json syntax",
			raw:  `{invalid-json`,
		},
		{
			name: "invalid yaml syntax",
			raw:  "keyWithoutColon",
		},
	}

	for _, tt := range structuralErrors {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := config.LoadBytes([]byte(tt.raw))
			if err == nil {
				t.Errorf("expected error, got nil")
			}
			if !errors.Is(err, config.ErrInvalidConfig) {
				t.Errorf("expected ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestLoadBytes_HostConfigWithListsAndNoPluginBlock(t *testing.T) {
	hostConfig := []byte(`
plugins:
  enabled: true
  dir: "plugins"
  store-sources:
    - "https://raw.githubusercontent.com/ygq-future/antigravity-priority/main/registry.json"
    - "https://example.com/store.json"
  excluded-providers:
    - antigravity
`)

	cfg, warnings, err := config.LoadBytes(hostConfig)
	if err != nil {
		t.Fatalf("unexpected error parsing host config with lists: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config when plugin block absent from host config, got %+v", cfg)
	}
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

func TestParseAntigravityModelGroup(t *testing.T) {
	valid := map[string]config.AntigravityModelGroup{
		"":               config.AntigravityModelGroupGemini,
		"gemini":         config.AntigravityModelGroupGemini,
		"GEMINI":         config.AntigravityModelGroupGemini,
		"claude_gpt":     config.AntigravityModelGroupClaudeGPT,
		"claude-gpt":     config.AntigravityModelGroupClaudeGPT,
		"claudegpt":      config.AntigravityModelGroupClaudeGPT,
		"claude gpt":     config.AntigravityModelGroupClaudeGPT,
		"claude_and_gpt": config.AntigravityModelGroupClaudeGPT,
	}

	for input, expected := range valid {
		got, err := config.ParseAntigravityModelGroup(input)
		if err != nil {
			t.Errorf("ParseAntigravityModelGroup(%q) error: %v", input, err)
		}
		if got != expected {
			t.Errorf("ParseAntigravityModelGroup(%q) = %v, want %v", input, got, expected)
		}
	}

	invalid := []string{"openai", "codex", "xai", "unknown", "gemini_pro"}
	for _, input := range invalid {
		_, err := config.ParseAntigravityModelGroup(input)
		if err == nil {
			t.Errorf("ParseAntigravityModelGroup(%q) expected error, got nil", input)
		}
	}
}

func TestDynamicConfig_ValidateAndApplyTo(t *testing.T) {
	base := config.Default()

	t.Run("valid dynamic config applies cleanly", func(t *testing.T) {
		dyn := config.DynamicConfig{
			AutoApply:                true,
			Interval:                 "30m",
			AntigravityModelGroup:    "claude_gpt",
			MaxConcurrency:           12,
			MinChange:                5,
			UrgencyTolerance:         0.10,
			RateLimitCooldownMinutes: 10,
			QuotaSampleCapacity:      15,
			PriorityRules: config.PriorityRulesConfig{
				Enabled:             true,
				BoostStartPriority:  990,
				NormalStartPriority: 250,
			},
			Schedule: config.ScheduleConfig{
				Paused:        false,
				WindowEnabled: true,
				WindowStart:   "09:00",
				WindowEnd:     "23:00",
			},
		}

		merged, err := dyn.ApplyTo(base)
		if err != nil {
			t.Fatalf("expected valid ApplyTo, got %v", err)
		}

		if !merged.AutoApply {
			t.Errorf("expected AutoApply=true")
		}
		if merged.Interval != 30*time.Minute {
			t.Errorf("expected Interval=30m, got %v", merged.Interval)
		}
		if merged.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
			t.Errorf("expected ModelGroup=claude_gpt, got %v", merged.AntigravityModelGroup)
		}
		if merged.MaxConcurrency != 12 {
			t.Errorf("expected MaxConcurrency=12, got %d", merged.MaxConcurrency)
		}
		if merged.MinChange != 5 {
			t.Errorf("expected MinChange=5, got %d", merged.MinChange)
		}
		if merged.UrgencyTolerance != 0.10 {
			t.Errorf("expected UrgencyTolerance=0.10, got %f", merged.UrgencyTolerance)
		}
		if merged.RateLimitCooldownMinutes != 10 {
			t.Errorf("expected RateLimitCooldownMinutes=10, got %d", merged.RateLimitCooldownMinutes)
		}
		if merged.QuotaSampleCapacity != 15 {
			t.Errorf("expected QuotaSampleCapacity=15, got %d", merged.QuotaSampleCapacity)
		}
		if merged.PriorityRules.BoostStartPriority != 990 {
			t.Errorf("expected BoostStartPriority=990, got %d", merged.PriorityRules.BoostStartPriority)
		}
		if merged.PriorityRules.NormalStartPriority != 250 {
			t.Errorf("expected NormalStartPriority=250, got %d", merged.PriorityRules.NormalStartPriority)
		}
		if !merged.Schedule.WindowEnabled || merged.Schedule.WindowStart != "09:00" || merged.Schedule.WindowEnd != "23:00" {
			t.Errorf("expected Schedule window enabled with 09:00-23:00, got %+v", merged.Schedule)
		}

		// Exporting dynamic view matches applied values
		exportedDyn := merged.Dynamic()
		if exportedDyn.Interval != "30m0s" && exportedDyn.Interval != "30m" {
			t.Errorf("expected exported interval 30m, got %s", exportedDyn.Interval)
		}
	})

	t.Run("invalid boundary checks produce errors", func(t *testing.T) {
		invalidCases := []struct {
			name    string
			mutate  func(dyn *config.DynamicConfig)
			wantErr string
		}{
			{
				name:    "invalid interval duration string",
				mutate:  func(dyn *config.DynamicConfig) { dyn.Interval = "invalid" },
				wantErr: "invalid interval",
			},
			{
				name:    "interval duration too short (< 1m)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.Interval = "30s" },
				wantErr: "too short",
			},
			{
				name:    "invalid model group",
				mutate:  func(dyn *config.DynamicConfig) { dyn.AntigravityModelGroup = "unknown" },
				wantErr: "invalid antigravity_model_group",
			},
			{
				name:    "max concurrency out of range (0)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.MaxConcurrency = 0 },
				wantErr: "max_concurrency must be between",
			},
			{
				name:    "max concurrency out of range (> 64)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.MaxConcurrency = 65 },
				wantErr: "max_concurrency must be between",
			},
			{
				name:    "min change out of range (< 0)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.MinChange = -1 },
				wantErr: "min_change must be between",
			},
			{
				name:    "boost start priority out of range (0)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.PriorityRules.BoostStartPriority = 0 },
				wantErr: "boost_start_priority must be between",
			},
			{
				name:    "normal start priority out of range (> 999)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.PriorityRules.NormalStartPriority = 1000 },
				wantErr: "normal_start_priority must be between",
			},
			{
				name:    "quota sample capacity out of range (1)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.QuotaSampleCapacity = 1 },
				wantErr: "quota_sample_capacity must be between",
			},
			{
				name:    "quota sample capacity out of range (31)",
				mutate:  func(dyn *config.DynamicConfig) { dyn.QuotaSampleCapacity = 31 },
				wantErr: "quota_sample_capacity must be between",
			},
			{
				name:    "invalid schedule window format",
				mutate:  func(dyn *config.DynamicConfig) { dyn.Schedule.WindowStart = "25:00" },
				wantErr: "window_start",
			},
		}

		for _, tt := range invalidCases {
			t.Run(tt.name, func(t *testing.T) {
				validDyn := base.Dynamic()
				tt.mutate(&validDyn)
				_, err := validDyn.ApplyTo(base)
				if err == nil {
					t.Fatalf("expected error for %s, got nil", tt.name)
				}
				if tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			})
		}
	})
}
