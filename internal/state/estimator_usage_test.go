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

	// 1. Initial successful probe for Gemini
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

func TestProbeCalibration_RejectsBlankPeriodAndMixedExternalDrain(t *testing.T) {
	ctx := context.Background()
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatalf("state.Load() error: %v", err)
	}

	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(72 * time.Hour)
	probe := func(at time.Time, shortRem, longRem int64) {
		t.Helper()
		if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
			AuthIndex:            "auth-ext",
			Provider:             core.ProviderAntigravity,
			ModelGroup:           "gemini",
			ObservedAt:           at,
			ResetAt:              shortReset,
			Remaining:            shortRem,
			ShortWindowResetAt:   shortReset,
			ShortWindowRemaining: &shortRem,
			LongWindowResetAt:    longReset,
			LongWindowRemaining:  &longRem,
			PlanType:             core.PlanTypePro,
		}); err != nil {
			t.Fatalf("MarkProbeSuccess(%v) error: %v", at, err)
		}
	}

	// Initial baseline + legitimate calibration to establish EMA K = 80000 and rate > 0.15.
	probe(now, 100, 100)
	store.RecordUsageConsumption("auth-ext", "gemini", 400000, now.Add(5*time.Minute)) // 5% expected
	probe(now.Add(15*time.Minute), 95, 99)                                             // 5% short, 1% long -> rate = 0.3*0.2 + 0.7*0.15 = 0.165
	entry, _ := store.GetEntry("auth-ext", "gemini")
	if entry.TokensPerShortPct != 80000 || entry.CycleBurnRate < 0.165-1e-6 || entry.CycleBurnRate > 0.165+1e-6 {
		t.Fatalf("unexpected calibrated baseline: k=%v rate=%v", entry.TokensPerShortPct, entry.CycleBurnRate)
	}

	// 1. Slow external 1%-per-probe trickle with zero local usage across 5 probes -> span gate must reject & rebase.
	for i := 1; i <= 5; i++ {
		probe(now.Add(time.Duration(15+i*15)*time.Minute), int64(95-i), int64(99-i))
	}
	entry, _ = store.GetEntry("auth-ext", "gemini")
	if entry.CycleBurnRate < 0.165-1e-6 || entry.CycleBurnRate > 0.165+1e-6 {
		t.Fatalf("zero-usage external trickle polluted CycleBurnRate: got %v want 0.165", entry.CycleBurnRate)
	}
	if entry.TokensPerShortPct != 80000 {
		t.Fatalf("zero-usage trickle changed TokensPerShortPct: got %v want 80000", entry.TokensPerShortPct)
	}
	if entry.LearningBaselineSequence != entry.Samples[len(entry.Samples)-1].Sequence {
		t.Fatalf("expected baseline rebased to latest sample %d, got %d", entry.Samples[len(entry.Samples)-1].Sequence, entry.LearningBaselineSequence)
	}

	// 2. Mixed blank-period external drain + multiple small consecutive harness turns:
	// 10 consecutive cached turns totaling 40,000 tokens (0.5% EMA expected), while external drained 30% (90 -> 60).
	for i := range 10 {
		store.RecordUsageConsumption("auth-ext", "gemini", 4000, now.Add(time.Duration(100+i)*time.Minute))
	}
	probe(now.Add(115*time.Minute), 60, 89)
	entry, _ = store.GetEntry("auth-ext", "gemini")
	if entry.TokensPerShortPct != 80000 {
		t.Fatalf("mixed external drain polluted TokensPerShortPct: got %v want 80000", entry.TokensPerShortPct)
	}
	if entry.CycleBurnRate < 0.165-1e-6 || entry.CycleBurnRate > 0.165+1e-6 {
		t.Fatalf("mixed external drain polluted CycleBurnRate: got %v want 0.165", entry.CycleBurnRate)
	}
	if !entry.Samples[len(entry.Samples)-1].AnomalySkipped {
		t.Fatalf("expected AnomalySkipped=true on mixed external drain sample")
	}
	if entry.LearningBaselineSequence != entry.Samples[len(entry.Samples)-1].Sequence {
		t.Fatalf("expected baseline rebased after anomaly, got %d", entry.LearningBaselineSequence)
	}

	// 3. Subsequent clean normal harness session resumes learning from the rebased sample (60 -> 55).
	for i := range 20 {
		store.RecordUsageConsumption("auth-ext", "gemini", 20000, now.Add(time.Duration(120+i)*time.Minute))
	}
	probe(now.Add(145*time.Minute), 55, 88)
	entry, _ = store.GetEntry("auth-ext", "gemini")
	if entry.Samples[len(entry.Samples)-1].AnomalySkipped {
		t.Fatalf("clean post-rebase session should not be flagged as anomaly")
	}
	// delta5h = 5%, delta7d = 1% -> obs = 0.20 -> newRate = 0.3*0.20 + 0.7*0.165 = 0.1755
	if entry.CycleBurnRate < 0.1755-1e-6 || entry.CycleBurnRate > 0.1755+1e-6 {
		t.Fatalf("expected clean post-rebase CycleBurnRate=0.1755, got %v", entry.CycleBurnRate)
	}
}

func TestProbeCalibration_ConsecutiveConsistentShiftReanchorsWithoutLockout(t *testing.T) {
	ctx := context.Background()
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "reanchor.json"))
	if err != nil {
		t.Fatalf("state.Load() error: %v", err)
	}

	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(72 * time.Hour)
	probe := func(at time.Time, shortRem, longRem int64) {
		t.Helper()
		if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
			AuthIndex:            "auth-shift",
			Provider:             core.ProviderAntigravity,
			ModelGroup:           "gemini",
			ObservedAt:           at,
			ResetAt:              shortReset,
			Remaining:            shortRem,
			ShortWindowResetAt:   shortReset,
			ShortWindowRemaining: &shortRem,
			LongWindowResetAt:    longReset,
			LongWindowRemaining:  &longRem,
			PlanType:             core.PlanTypePro,
		}); err != nil {
			t.Fatalf("MarkProbeSuccess error: %v", err)
		}
	}

	// Establish EMA K = 80,000.
	probe(now, 100, 100)
	store.RecordUsageConsumption("auth-shift", "gemini", 400000, now.Add(5*time.Minute))
	probe(now.Add(15*time.Minute), 95, 99)

	// Genuine 4x stricter upstream sensitivity: 1% now costs 20,000 tokens (within [5000, 500000]).
	// Round 1 & 2: rejected while confirming consistency (streak=1, streak=2).
	store.RecordUsageConsumption("auth-shift", "gemini", 160000, now.Add(20*time.Minute)) // EMA est=2%, actual=8% (kObs=20000)
	probe(now.Add(30*time.Minute), 87, 98)
	entry, _ := store.GetEntry("auth-shift", "gemini")
	if entry.TokensPerShortPct != 80000 || entry.SensitivityAnomalyStreak != 1 {
		t.Fatalf("round 1 should hold K=80000 with streak=1, got k=%v streak=%d", entry.TokensPerShortPct, entry.SensitivityAnomalyStreak)
	}

	store.RecordUsageConsumption("auth-shift", "gemini", 160000, now.Add(35*time.Minute)) // kObs=20000
	probe(now.Add(45*time.Minute), 79, 97)
	entry, _ = store.GetEntry("auth-shift", "gemini")
	if entry.TokensPerShortPct != 80000 || entry.SensitivityAnomalyStreak != 2 {
		t.Fatalf("round 2 should hold K=80000 with streak=2, got k=%v streak=%d", entry.TokensPerShortPct, entry.SensitivityAnomalyStreak)
	}

	// Round 3: confirmed -> re-anchors TokensPerShortPct to 20,000 and resets streak.
	store.RecordUsageConsumption("auth-shift", "gemini", 160000, now.Add(50*time.Minute)) // kObs=20000
	probe(now.Add(60*time.Minute), 71, 96)
	entry, _ = store.GetEntry("auth-shift", "gemini")
	if entry.TokensPerShortPct != 20000 || entry.SensitivityAnomalyStreak != 0 {
		t.Fatalf("round 3 should re-anchor K=20000 and reset streak, got k=%v streak=%d", entry.TokensPerShortPct, entry.SensitivityAnomalyStreak)
	}
}
