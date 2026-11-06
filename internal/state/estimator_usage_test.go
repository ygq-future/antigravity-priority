package state_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/state"
)

func TestCalculateWeightedTurnTokens(t *testing.T) {
	// 1000 input, 200 cache read (uncached = 800), 100 output, 50 reasoning
	// weighted = 800 + 0.1*200 + 4.0*(100+50) = 800 + 20 + 600 = 1420
	weighted := state.CalculateWeightedTurnTokens(1000, 100, 50, 200, 1150)
	if weighted != 1420 {
		t.Errorf("CalculateWeightedTurnTokens() = %v, want 1420", weighted)
	}

	// Zero / negative tokens floor fallback (at least 2000)
	floor := state.CalculateWeightedTurnTokens(0, 0, 0, 0, 0)
	if floor != 2000 {
		t.Errorf("CalculateWeightedTurnTokens() floor = %v, want 2000", floor)
	}
}

func TestRecordUsageConsumption_AndProbeCalibration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cache.json")
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("state.Load() error: %v", err)
	}

	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(3 * time.Hour)
	longReset := now.Add(72 * time.Hour)
	remShort := int64(80)
	remLong := int64(90)

	// 1. Initial successful probe for Gemini and Argon
	err = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "auth-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ResetAt:              shortReset,
		Remaining:            80,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: &remShort,
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  &remLong,
		PlanType:             core.PlanTypePro,
	})
	if err != nil {
		t.Fatalf("MarkProbeSuccess error: %v", err)
	}

	remArgonLong := int64(85)
	err = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "auth-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "argon",
		ObservedAt:           now,
		ResetAt:              shortReset,
		Remaining:            80,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: &remShort,
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  &remArgonLong,
		PlanType:             core.PlanTypePro,
	})
	if err != nil {
		t.Fatalf("MarkProbeSuccess argon error: %v", err)
	}

	// 2. Record usage on Gemini: 160,000 weighted tokens
	// For Gemini, defaultTokensPerShortPct = 80,000
	// 160,000 / 80,000 = 2% short burn
	// Long burn = 2% * 0.15 = 0.3% -> round to 0
	store.RecordUsageConsumption("auth-1", "gemini", 160000, now.Add(5*time.Minute))

	// Verify Gemini entry has estimated deductions
	evGemini, ok := store.GetHistoricalEvidence("auth-1", "gemini")
	if !ok {
		t.Fatalf("expected historical evidence for gemini")
	}
	if evGemini.ShortWindowRemaining == nil || *evGemini.ShortWindowRemaining != 78 {
		t.Errorf("expected ShortWindowRemaining=78 (80 - 2), got %v", evGemini.ShortWindowRemaining)
	}

	// Verify Argon entry inherited the shared 5h short burn deduction but kept 7d weekly intact
	evArgon, ok := store.GetHistoricalEvidence("auth-1", "argon")
	if !ok {
		t.Fatalf("expected historical evidence for argon")
	}
	if evArgon.ShortWindowRemaining == nil || *evArgon.ShortWindowRemaining != 78 {
		t.Errorf("expected Argon ShortWindowRemaining=78 (shared from Gemini), got %v", *evArgon.ShortWindowRemaining)
	}
	if evArgon.LongWindowRemaining == nil || *evArgon.LongWindowRemaining != 85 {
		t.Errorf("expected Argon LongWindowRemaining=85 (unaffected by Gemini 7d), got %v", *evArgon.LongWindowRemaining)
	}

	// Record usage on Argon: 40,000 weighted tokens -> 1% short burn on shared 5h window
	store.RecordUsageConsumption("auth-1", "argon", 40000, now.Add(10*time.Minute))
	evGeminiAfterArgon, _ := store.GetHistoricalEvidence("auth-1", "gemini")
	if evGeminiAfterArgon.ShortWindowRemaining == nil || *evGeminiAfterArgon.ShortWindowRemaining != 77 {
		t.Errorf("expected Gemini ShortWindowRemaining=77 (shared 5h synced from Argon), got %v", *evGeminiAfterArgon.ShortWindowRemaining)
	}
	if evGeminiAfterArgon.LongWindowRemaining == nil || *evGeminiAfterArgon.LongWindowRemaining != 90 {
		t.Errorf("expected Gemini LongWindowRemaining=90 (unaffected by Argon 7d), got %v", *evGeminiAfterArgon.LongWindowRemaining)
	}
	// 3. Subsequent probe arrives with actual server drop of 4% (from 80 to 76)
	// Pending tokens were 160,000. Actual drop = 4%.
	// Observed sensitivity kObs = 160,000 / 4 = 40,000.
	// Previous K was default 80,000.
	// New K = 0.3 * 40000 + 0.7 * 80000 = 12000 + 56000 = 68,000.
	newRemShort := int64(76)
	newRemLong := int64(89)
	err = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "auth-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now.Add(15 * time.Minute),
		ResetAt:              shortReset,
		Remaining:            76,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: &newRemShort,
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  &newRemLong,
		PlanType:             core.PlanTypePro,
	})
	if err != nil {
		t.Fatalf("MarkProbeSuccess calibration error: %v", err)
	}

	// Verify pending tokens reset to 0 and learned K updated
	entry, ok := store.GetEntry("auth-1", "gemini")
	if !ok {
		t.Fatalf("GetEntry error")
	}
	if entry.PendingWeightedTokens != 0 {
		t.Errorf("expected PendingWeightedTokens=0, got %v", entry.PendingWeightedTokens)
	}
	if entry.EstimatedShortBurnPct != 0 {
		t.Errorf("expected EstimatedShortBurnPct=0, got %v", entry.EstimatedShortBurnPct)
	}
	if entry.TokensPerShortPct != 68000 {
		t.Errorf("expected learned TokensPerShortPct=68000, got %v", entry.TokensPerShortPct)
	}

	// Historical evidence now reflects exact server probe truth
	evCalibrated, ok := store.GetHistoricalEvidence("auth-1", "gemini")
	if !ok {
		t.Fatalf("GetHistoricalEvidence error")
	}
	if evCalibrated.ShortWindowRemaining == nil || *evCalibrated.ShortWindowRemaining != 76 {
		t.Errorf("expected ShortWindowRemaining=76, got %v", evCalibrated.ShortWindowRemaining)
	}
}
