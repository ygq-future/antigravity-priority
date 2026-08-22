package core_test

import (
	"testing"

	"antigravity-priority/internal/core"
)

func TestModelGroup(t *testing.T) {
	if core.ModelGroupGemini != "gemini" {
		t.Errorf("expected ModelGroupGemini to be gemini, got %s", core.ModelGroupGemini)
	}
	if core.ModelGroupClaudeGPT != "claude_gpt" {
		t.Errorf("expected ModelGroupClaudeGPT to be claude_gpt, got %s", core.ModelGroupClaudeGPT)
	}
}
