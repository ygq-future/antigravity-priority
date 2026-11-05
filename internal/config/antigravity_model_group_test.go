package config_test

import (
	"testing"

	"antigravity-priority/internal/config"
)


func TestClassifyModelQuotaPool(t *testing.T) {
	tests := []struct {
		model    string
		expected config.AntigravityModelGroup
	}{
		// Gemini models
		{"gemini-3.8-flash-high", config.AntigravityModelGroupGemini},
		{"gemini-3-flash", config.AntigravityModelGroupGemini},
		{"gemini-3.5-flash-lite", config.AntigravityModelGroupGemini},
		{"google/gemini-2.5-pro", config.AntigravityModelGroupGemini},

		// Argon models
		{"gemini-4-argon-high", config.AntigravityModelGroupArgon},
		{"teamA/gemini-4-argon-high", config.AntigravityModelGroupArgon},
		{"argon-experimental", config.AntigravityModelGroupArgon},

		// Claude & GPT models
		{"claude-sonnet-5-5-high", config.AntigravityModelGroupClaudeGPT},
		{"claude-sonnet-5-5-high(16384)", config.AntigravityModelGroupClaudeGPT},
		{"claude-opus-5-5-high", config.AntigravityModelGroupClaudeGPT},
		{"claude-opus-4-6-thinking(32768)", config.AntigravityModelGroupClaudeGPT},
		{"anthropic/claude-3-5-haiku", config.AntigravityModelGroupClaudeGPT},
		{"gpt-oss-120b-medium", config.AntigravityModelGroupClaudeGPT},
		{"o3-mini", config.AntigravityModelGroupClaudeGPT},
		{"o1", config.AntigravityModelGroupClaudeGPT},
		{"openai/gpt-4o", config.AntigravityModelGroupClaudeGPT},
	}

	for _, tt := range tests {
		got := config.ClassifyModelQuotaPool(tt.model)
		if got != tt.expected {
			t.Errorf("ClassifyModelQuotaPool(%q) = %q, want %q", tt.model, got, tt.expected)
		}
	}
}
