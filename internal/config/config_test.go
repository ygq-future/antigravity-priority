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
		"auto_apply": true,
		"interval": "30m",
		"antigravity_model_group": "claude_gpt",
		"max_concurrency": 10,
		"min_change": 2,
		"priority_rules": {
			"enabled": true,
			"boost_start_priority": 990,
			"normal_start_priority": 200
		}
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
	if !cfg.AutoApply {
		t.Errorf("expected AutoApply=true, got %v", cfg.AutoApply)
	}
	if cfg.Interval != 30*time.Minute {
		t.Errorf("expected Interval=30m, got %v", cfg.Interval)
	}
	if cfg.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
		t.Errorf("expected AntigravityModelGroup=claude_gpt, got %v", cfg.AntigravityModelGroup)
	}
	if cfg.MaxConcurrency != 10 {
		t.Errorf("expected MaxConcurrency=10, got %v", cfg.MaxConcurrency)
	}
	if cfg.MinChange != 2 {
		t.Errorf("expected MinChange=2, got %v", cfg.MinChange)
	}
	if cfg.PriorityRules.BoostStartPriority != 990 {
		t.Errorf("expected BoostStartPriority=990, got %v", cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.PriorityRules.NormalStartPriority != 200 {
		t.Errorf("expected NormalStartPriority=200, got %v", cfg.PriorityRules.NormalStartPriority)
	}
}

func TestLoadBytes_JSON_FlatKeys(t *testing.T) {
	jsonData := []byte(`{
		"enabled": true,
		"priority_rules.boost_start_priority": 980,
		"priority_rules.normal_start_priority": 120
	}`)

	cfg, _, err := config.LoadBytes(jsonData)
	if err != nil {
		t.Fatalf("unexpected error loading flat keys json: %v", err)
	}

	if cfg.PriorityRules.BoostStartPriority != 980 {
		t.Errorf("expected BoostStartPriority=980, got %v", cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.PriorityRules.NormalStartPriority != 120 {
		t.Errorf("expected NormalStartPriority=120, got %v", cfg.PriorityRules.NormalStartPriority)
	}
}

func TestLoadBytes_YAML(t *testing.T) {
	yamlData := []byte(`
# Plugin Configuration
enabled: true
auto_apply: true
interval: 10m
antigravity_model_group: claude-gpt
max_concurrency: 4
min_change: 0
priority_rules:
  enabled: false
  boost_start_priority: 950
  normal_start_priority: 50
`)

	cfg, _, err := config.LoadBytes(yamlData)
	if err != nil {
		t.Fatalf("unexpected error loading yaml: %v", err)
	}

	if !cfg.Enabled {
		t.Errorf("expected Enabled=true, got %v", cfg.Enabled)
	}
	if !cfg.AutoApply {
		t.Errorf("expected AutoApply=true, got %v", cfg.AutoApply)
	}
	if cfg.Interval != 10*time.Minute {
		t.Errorf("expected Interval=10m, got %v", cfg.Interval)
	}
	if cfg.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
		t.Errorf("expected AntigravityModelGroup=claude_gpt, got %v", cfg.AntigravityModelGroup)
	}
	if cfg.MaxConcurrency != 4 {
		t.Errorf("expected MaxConcurrency=4, got %v", cfg.MaxConcurrency)
	}
	if cfg.MinChange != 0 {
		t.Errorf("expected MinChange=0, got %v", cfg.MinChange)
	}
	if cfg.PriorityRules.Enabled {
		t.Errorf("expected PriorityRules.Enabled=false, got %v", cfg.PriorityRules.Enabled)
	}
	if cfg.PriorityRules.BoostStartPriority != 950 {
		t.Errorf("expected BoostStartPriority=950, got %v", cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.PriorityRules.NormalStartPriority != 50 {
		t.Errorf("expected NormalStartPriority=50, got %v", cfg.PriorityRules.NormalStartPriority)
	}
}

func TestLoadBytes_CPAPluginConfigPath(t *testing.T) {
	fullCPAConfig := []byte(`
plugins:
  configs:
    antigravity-priority:
      enabled: true
      auto_apply: true
      interval: 20m
      antigravity_model_group: gemini
      max_concurrency: 8
`)

	cfg, _, err := config.LoadBytes(fullCPAConfig)
	if err != nil {
		t.Fatalf("unexpected error parsing CPA plugin config path: %v", err)
	}

	if !cfg.AutoApply {
		t.Errorf("expected AutoApply=true, got %v", cfg.AutoApply)
	}
	if cfg.Interval != 20*time.Minute {
		t.Errorf("expected Interval=20m, got %v", cfg.Interval)
	}
	if cfg.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency=8, got %v", cfg.MaxConcurrency)
	}
}

func TestLoadBytes_Invalid(t *testing.T) {
	// Only structurally unparseable input should produce hard errors
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

	// Field-level issues should produce warnings with smooth fallback to defaults
	warningCases := []struct {
		name           string
		raw            string
		wantWarnings   int
		checkField     string
		checkValue     any
	}{
		{
			name:         "invalid model group falls back to gemini",
			raw:          `{"antigravity_model_group": "invalid_group"}`,
			wantWarnings: 1,
			checkField:   "model_group",
			checkValue:   config.AntigravityModelGroupGemini,
		},
		{
			name:         "negative interval falls back to 15m",
			raw:          `{"interval": "-5m"}`,
			wantWarnings: 1,
			checkField:   "interval",
			checkValue:   15 * time.Minute,
		},
		{
			name:         "zero max concurrency falls back to 6",
			raw:          `{"max_concurrency": 0}`,
			wantWarnings: 1,
			checkField:   "max_concurrency",
			checkValue:   6,
		},
		{
			name:         "negative min change falls back to 1",
			raw:          `{"min_change": -1}`,
			wantWarnings: 1,
			checkField:   "min_change",
			checkValue:   1,
		},
		{
			name:         "zero boost start priority falls back to 999",
			raw:          `{"priority_rules": {"boost_start_priority": 0}}`,
			wantWarnings: 1,
			checkField:   "boost_start_priority",
			checkValue:   999,
		},
		{
			name:         "zero normal start priority falls back to 100",
			raw:          `{"priority_rules": {"normal_start_priority": 0}}`,
			wantWarnings: 1,
			checkField:   "normal_start_priority",
			checkValue:   100,
		},
	}

	for _, tt := range warningCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg, warnings, err := config.LoadBytes([]byte(tt.raw))
			if err != nil {
				t.Fatalf("expected no error with smooth fallback, got %v", err)
			}
			if len(warnings) != tt.wantWarnings {
				t.Errorf("expected %d warning(s), got %d: %v", tt.wantWarnings, len(warnings), warnings)
			}
			switch tt.checkField {
			case "model_group":
				if cfg.AntigravityModelGroup != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.AntigravityModelGroup)
				}
			case "interval":
				if cfg.Interval != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.Interval)
				}
			case "max_concurrency":
				if cfg.MaxConcurrency != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.MaxConcurrency)
				}
			case "min_change":
				if cfg.MinChange != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.MinChange)
				}
			case "boost_start_priority":
				if cfg.PriorityRules.BoostStartPriority != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.PriorityRules.BoostStartPriority)
				}
			case "normal_start_priority":
				if cfg.PriorityRules.NormalStartPriority != tt.checkValue {
					t.Errorf("expected %v, got %v", tt.checkValue, cfg.PriorityRules.NormalStartPriority)
				}
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

func TestLoadBytes_DurationCaseNormalization(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		expected time.Duration
	}{
		{"uppercase 30M", `{"interval": "30M"}`, 30 * time.Minute},
		{"uppercase 1H", `{"interval": "1H"}`, time.Hour},
		{"uppercase 15S", `{"interval": "15S"}`, 15 * time.Second},
		{"uppercase 500MS", `{"interval": "500MS"}`, 500 * time.Millisecond},
		{"mixed 1h30M", `{"interval": "1h30M"}`, 90 * time.Minute},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg, warnings, err := config.LoadBytes([]byte(tt.raw))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Interval != tt.expected {
				t.Errorf("expected Interval=%v, got %v", tt.expected, cfg.Interval)
			}
			if len(warnings) != 1 {
				t.Errorf("expected 1 normalization warning, got %d: %v", len(warnings), warnings)
			}
		})
	}
}

func TestLoadBytes_MultipleWarnings(t *testing.T) {
	raw := `{
		"interval": "INVALID",
		"antigravity_model_group": "unknown_group",
		"max_concurrency": -5,
		"min_change": -10,
		"priority_rules": {
			"boost_start_priority": 0,
			"normal_start_priority": -1
		}
	}`

	cfg, warnings, err := config.LoadBytes([]byte(raw))
	if err != nil {
		t.Fatalf("expected no error with smooth fallback, got %v", err)
	}
	if len(warnings) != 6 {
		t.Errorf("expected 6 warnings, got %d: %v", len(warnings), warnings)
	}

	// All values should be defaults
	defaults := config.Default()
	if cfg.Interval != defaults.Interval {
		t.Errorf("expected Interval=%v, got %v", defaults.Interval, cfg.Interval)
	}
	if cfg.AntigravityModelGroup != defaults.AntigravityModelGroup {
		t.Errorf("expected AntigravityModelGroup=%v, got %v", defaults.AntigravityModelGroup, cfg.AntigravityModelGroup)
	}
	if cfg.MaxConcurrency != defaults.MaxConcurrency {
		t.Errorf("expected MaxConcurrency=%v, got %v", defaults.MaxConcurrency, cfg.MaxConcurrency)
	}
	if cfg.MinChange != defaults.MinChange {
		t.Errorf("expected MinChange=%v, got %v", defaults.MinChange, cfg.MinChange)
	}
	if cfg.PriorityRules.BoostStartPriority != defaults.PriorityRules.BoostStartPriority {
		t.Errorf("expected BoostStartPriority=%v, got %v", defaults.PriorityRules.BoostStartPriority, cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.PriorityRules.NormalStartPriority != defaults.PriorityRules.NormalStartPriority {
		t.Errorf("expected NormalStartPriority=%v, got %v", defaults.PriorityRules.NormalStartPriority, cfg.PriorityRules.NormalStartPriority)
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
