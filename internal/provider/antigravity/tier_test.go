package antigravity

import (
	"testing"

	"antigravity-priority/internal/core"
)

func TestParseLoadCodeAssistMapsKnownTiers(t *testing.T) {
	tests := []struct {
		name   string
		paidID string
		want   core.PlanType
	}{
		{name: "ultra", paidID: "g1-ultra-tier", want: core.PlanTypeUltra},
		{name: "ultra lite", paidID: "g1-ultra-lite-tier", want: core.PlanTypeUltraLite},
		{name: "pro", paidID: "g1-pro-tier", want: core.PlanTypePro},
		{name: "free", paidID: "free-tier", want: core.PlanTypeFree},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte(`{"currentTier":{"id":"free-tier"},"paidTier":{"id":"` + test.paidID + `"}}`)
			planType, err := ParseLoadCodeAssist(raw)
			if err != nil {
				t.Fatalf("ParseLoadCodeAssist() error = %v", err)
			}
			if planType != test.want {
				t.Fatalf("plan type = %q; want %q", planType, test.want)
			}
		})
	}
}

func TestParseLoadCodeAssistPrefersPaidTierOverCurrentTier(t *testing.T) {
	raw := []byte(`{
		"currentTier": {"id": "free-tier"},
		"paidTier": {"id": "g1-pro-tier"},
		"allowedTiers": [{"id": "free-tier"}, {"id": "standard-tier"}]
	}`)

	planType, err := ParseLoadCodeAssist(raw)
	if err != nil {
		t.Fatalf("ParseLoadCodeAssist() error = %v", err)
	}
	if planType != core.PlanTypePro {
		t.Fatalf("plan type = %q; want pro", planType)
	}
}

func TestParseLoadCodeAssistFallsBackToCurrentTier(t *testing.T) {
	raw := []byte(`{
		"currentTier": {"id": "standard-tier"}
	}`)

	planType, err := ParseLoadCodeAssist(raw)
	if err != nil {
		t.Fatalf("ParseLoadCodeAssist() error = %v", err)
	}
	if planType != core.PlanTypeTeam {
		t.Fatalf("plan type = %q; want team", planType)
	}
}

func TestParseLoadCodeAssistTreatsFreeTierAsFree(t *testing.T) {
	raw := []byte(`{
		"currentTier": {"id": "free-tier"},
		"paidTier": {"id": "free-tier"},
		"allowedTiers": [{"id": "standard-tier"}]
	}`)

	planType, err := ParseLoadCodeAssist(raw)
	if err != nil {
		t.Fatalf("ParseLoadCodeAssist() error = %v", err)
	}
	if planType != core.PlanTypeFree {
		t.Fatalf("plan type = %q; want free", planType)
	}
}

func TestParseLoadCodeAssistPaidTierWinsEvenWhenCurrentTierIsHigher(t *testing.T) {
	raw := []byte(`{
		"currentTier": {"id": "g1-pro-tier"},
		"paidTier": {"id": "free-tier"}
	}`)

	planType, err := ParseLoadCodeAssist(raw)
	if err != nil {
		t.Fatalf("ParseLoadCodeAssist() error = %v", err)
	}
	if planType != core.PlanTypeFree {
		t.Fatalf("plan type = %q; want paidTier free-tier to win", planType)
	}
}

func TestParseLoadCodeAssistPreservesUnknownPaidTier(t *testing.T) {
	raw := []byte(`{"paidTier":{"id":"g1-enterprise-tier"}}`)

	planType, err := ParseLoadCodeAssist(raw)
	if err != nil {
		t.Fatalf("ParseLoadCodeAssist() error = %v", err)
	}
	if planType != core.PlanType("g1-enterprise-tier") {
		t.Fatalf("plan type = %q; want raw paid tier id", planType)
	}
}

func TestParseLoadCodeAssistRejectsMissingTier(t *testing.T) {
	if _, err := ParseLoadCodeAssist([]byte(`{"allowedTiers":[{"id":"standard-tier"}]}`)); err == nil {
		t.Fatal("expected error when neither paidTier nor currentTier is present")
	}
}

func TestParseLoadCodeAssistRejectsMalformedResponse(t *testing.T) {
	if _, err := ParseLoadCodeAssist([]byte(`{"paidTier":`)); err == nil {
		t.Fatal("expected decode error for malformed response")
	}
}
