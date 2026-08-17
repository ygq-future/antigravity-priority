package core_test

import (
	"encoding/json"
	"testing"

	"antigravity-priority/internal/core"
)

func TestCredential_WithProbe(t *testing.T) {
	orig := core.Credential{
		Name:            "antigravity-1",
		AuthIndex:       "idx-1",
		Provider:        core.ProviderAntigravity,
		Type:            core.CredentialTypeAntigravity,
		Status:          core.CredentialStatusActive,
		Disabled:        false,
		Unavailable:     false,
		Priority:        100,
		PriorityMissing: false,
		Account:         "user@example.com",
		Email:           "user@example.com",
		PlanType:        core.PlanTypePro,
		Freshness:       core.FreshnessUnknown,
		ProbeStatus:     core.ProbeStatusUnknown,
		RawJSON:         json.RawMessage(`{"auth_index":"idx-1"}`),
	}

	updated := orig.WithProbe(core.FreshnessFresh, core.ProbeStatusReady)

	if updated.Freshness != core.FreshnessFresh {
		t.Errorf("expected FreshnessFresh, got %v", updated.Freshness)
	}
	if updated.ProbeStatus != core.ProbeStatusReady {
		t.Errorf("expected ProbeStatusReady, got %v", updated.ProbeStatus)
	}
	// Verify original is unchanged (immutability)
	if orig.Freshness != core.FreshnessUnknown {
		t.Errorf("expected original FreshnessUnknown, got %v", orig.Freshness)
	}
	if orig.ProbeStatus != core.ProbeStatusUnknown {
		t.Errorf("expected original ProbeStatusUnknown, got %v", orig.ProbeStatus)
	}
	// Verify other fields preserved
	if updated.Name != orig.Name || updated.AuthIndex != orig.AuthIndex || updated.Priority != orig.Priority {
		t.Errorf("expected core fields preserved, got %+v", updated)
	}
}

func TestPromotionFromProbe(t *testing.T) {
	tests := []struct {
		name        string
		freshness   core.Freshness
		probeStatus core.ProbeStatus
		want        core.CanPromote
	}{
		{
			name:        "fresh and ready allows promote",
			freshness:   core.FreshnessFresh,
			probeStatus: core.ProbeStatusReady,
			want:        core.CanPromoteAfterFreshProbe,
		},
		{
			name:        "stale and ready denies promote",
			freshness:   core.FreshnessStale,
			probeStatus: core.ProbeStatusReady,
			want:        core.CannotPromote,
		},
		{
			name:        "fresh and unsupported denies promote",
			freshness:   core.FreshnessFresh,
			probeStatus: core.ProbeStatusUnsupported,
			want:        core.CannotPromote,
		},
		{
			name:        "unknown denies promote",
			freshness:   core.FreshnessUnknown,
			probeStatus: core.ProbeStatusUnknown,
			want:        core.CannotPromote,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := core.PromotionFromProbe(tt.freshness, tt.probeStatus)
			if got != tt.want {
				t.Errorf("PromotionFromProbe(%v, %v) = %v, want %v", tt.freshness, tt.probeStatus, got, tt.want)
			}
		})
	}
}

func TestModelGroup(t *testing.T) {
	if core.ModelGroupGemini != "gemini" {
		t.Errorf("expected ModelGroupGemini to be gemini, got %s", core.ModelGroupGemini)
	}
	if core.ModelGroupClaudeGPT != "claude_gpt" {
		t.Errorf("expected ModelGroupClaudeGPT to be claude_gpt, got %s", core.ModelGroupClaudeGPT)
	}
}
