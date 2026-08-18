package config_test

import (
	"errors"
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
	cfg, err := config.LoadBytes(nil)
	if err != nil {
		t.Fatalf("unexpected error on nil: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config on nil, got %+v", cfg)
	}

	cfg, err = config.LoadBytes([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error on empty bytes: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config on empty bytes, got %+v", cfg)
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

	cfg, err := config.LoadBytes(jsonData)
	if err != nil {
		t.Fatalf("unexpected error loading json: %v", err)
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

	cfg, err := config.LoadBytes(jsonData)
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

	cfg, err := config.LoadBytes(yamlData)
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

	cfg, err := config.LoadBytes(fullCPAConfig)
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
	tests := []struct {
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
		{
			name: "invalid model group",
			raw:  `{"antigravity_model_group": "invalid_group"}`,
		},
		{
			name: "negative interval",
			raw:  `{"interval": "-5m"}`,
		},
		{
			name: "zero max concurrency",
			raw:  `{"max_concurrency": 0}`,
		},
		{
			name: "negative min change",
			raw:  `{"min_change": -1}`,
		},
		{
			name: "zero boost start priority",
			raw:  `{"priority_rules": {"boost_start_priority": 0}}`,
		},
		{
			name: "zero normal start priority",
			raw:  `{"priority_rules": {"normal_start_priority": 0}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := config.LoadBytes([]byte(tt.raw))
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

	cfg, err := config.LoadBytes(hostConfig)
	if err != nil {
		t.Fatalf("unexpected error parsing host config with lists: %v", err)
	}
	if cfg != config.Default() {
		t.Errorf("expected default config when plugin block absent from host config, got %+v", cfg)
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
