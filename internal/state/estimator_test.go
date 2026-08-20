package state_test

import (
	"testing"
	"time"

	"antigravity-priority/internal/state"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func TestEstimator_MultiSample_ColdStartAndPreservation(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)

	// Cold start with 0 or negative rate falls back to default 0.15
	rate, samples := state.UpdateSamplesAndCycleBurnRate(
		0.0, nil, now, shortReset, int64Ptr(100), int64Ptr(100), 6,
	)
	if rate != state.DefaultCycleBurnRate || len(samples) != 1 {
		t.Fatalf("expected default rate and 1 sample, got rate=%v, len=%d", rate, len(samples))
	}

	// Nil pointers return preserved rate and samples
	rate2, samples2 := state.UpdateSamplesAndCycleBurnRate(
		0.25, samples, now.Add(time.Minute), shortReset, nil, int64Ptr(90), 6,
	)
	if rate2 != 0.25 || len(samples2) != len(samples) {
		t.Fatalf("expected preserved rate and samples on nil pointer")
	}
}

func TestEstimator_MultiSample_SlowConsumptionAccumulation(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	rate := state.DefaultCycleBurnRate
	var samples []state.QuotaSample
	capacity := 6

	// Step 1: Initial probe (100%, 100%)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, now, shortReset, int64Ptr(100), int64Ptr(100), capacity,
	)
	if rate != state.DefaultCycleBurnRate || len(samples) != 1 {
		t.Fatalf("step 1: expected default rate and 1 sample, got %v, len=%d", rate, len(samples))
	}

	// Step 2: Small consumption 2% (100 -> 98, 100 -> 100). Delta 2% < 5% -> no rate change
	t2 := now.Add(15 * time.Minute)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, t2, shortReset, int64Ptr(98), int64Ptr(100), capacity,
	)
	if rate != state.DefaultCycleBurnRate || len(samples) != 2 {
		t.Fatalf("step 2: expected default rate and 2 samples, got %v, len=%d", rate, len(samples))
	}

	// Step 3: Another small consumption 2% (98 -> 96, 100 -> 99). Total delta 4% < 5% -> no rate change
	t3 := now.Add(30 * time.Minute)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, t3, shortReset, int64Ptr(96), int64Ptr(99), capacity,
	)
	if rate != state.DefaultCycleBurnRate || len(samples) != 3 {
		t.Fatalf("step 3: expected default rate and 3 samples, got %v, len=%d", rate, len(samples))
	}

	// Step 4: Another small consumption 2% (96 -> 94, 99 -> 98). Total delta from baseline (100 -> 94) = 6% >= 5%
	// delta 5h = 6%, delta 7d = 2% (100 -> 98)
	// obs = 0.02 / 0.06 = 0.3333 -> clamped to 0.30
	// newRate = 0.3 * 0.30 + 0.7 * 0.15 = 0.09 + 0.105 = 0.195
	t4 := now.Add(45 * time.Minute)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, t4, shortReset, int64Ptr(94), int64Ptr(98), capacity,
	)
	expectedRate := 0.195
	if rate < expectedRate-1e-6 || rate > expectedRate+1e-6 {
		t.Fatalf("step 4: expected accumulated rate %v, got %v", expectedRate, rate)
	}
	// After learning, baseline advances to current sample
	if len(samples) != 1 || samples[0].ShortWindowRem != 94 {
		t.Fatalf("step 4: expected samples reset to current baseline (len=1, rem=94), got len=%d", len(samples))
	}
}

func TestEstimator_MultiSample_ZeroConsumptionDeduplication(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	rate := 0.18
	var samples []state.QuotaSample
	capacity := 6

	// Step 1: Initial probe
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, now, shortReset, int64Ptr(90), int64Ptr(95), capacity,
	)
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}

	// Step 2 & 3: 0 consumption probes (same 90%, 95%)
	t2 := now.Add(15 * time.Minute)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, t2, shortReset, int64Ptr(90), int64Ptr(95), capacity,
	)
	if len(samples) != 1 || !samples[0].ObservedAt.Equal(t2) {
		t.Fatalf("expected 1 sample with updated observedAt, got len=%d, at=%v", len(samples), samples[0].ObservedAt)
	}

	t3 := now.Add(30 * time.Minute)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, t3, shortReset, int64Ptr(90), int64Ptr(95), capacity,
	)
	if len(samples) != 1 || !samples[0].ObservedAt.Equal(t3) {
		t.Fatalf("expected 1 sample with updated observedAt, got len=%d", len(samples))
	}
}

func TestEstimator_MultiSample_WindowResetEviction(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset1 := now.Add(2 * time.Hour)
	shortReset2 := now.Add(7 * time.Hour) // window refreshed
	rate := 0.18
	var samples []state.QuotaSample
	capacity := 6

	// Add 2 samples in cycle 1
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, now, shortReset1, int64Ptr(50), int64Ptr(80), capacity,
	)
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, now.Add(15*time.Minute), shortReset1, int64Ptr(48), int64Ptr(80), capacity,
	)
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}

	// Reset occurs: shortReset2 with 100% 5h quota
	rate, samples = state.UpdateSamplesAndCycleBurnRate(
		rate, samples, now.Add(2*time.Hour), shortReset2, int64Ptr(100), int64Ptr(80), capacity,
	)
	if len(samples) != 1 || samples[0].ShortWindowRem != 100 || !samples[0].ShortWindowResetAt.Equal(shortReset2) {
		t.Fatalf("expected samples cleared on reset, got len=%d, rem=%v", len(samples), samples[0].ShortWindowRem)
	}
	if rate != 0.18 {
		t.Fatalf("expected preserved rate across reset, got %v", rate)
	}
}

func TestEstimator_MultiSample_CapacityFIFO(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	rate := 0.15
	var samples []state.QuotaSample
	capacity := 3

	// Push 4 samples (100 -> 99 -> 98 -> 97)
	for i := 0; i < 4; i++ {
		rem5h := int64(100 - i)
		rate, samples = state.UpdateSamplesAndCycleBurnRate(
			rate, samples, now.Add(time.Duration(i*10)*time.Minute), shortReset, &rem5h, int64Ptr(100), capacity,
		)
	}

	if len(samples) != 3 {
		t.Fatalf("expected samples capped at %d, got %d", capacity, len(samples))
	}
	// Oldest sample (100) should be dropped; oldest in slice should now be 99
	if samples[0].ShortWindowRem != 99 || samples[len(samples)-1].ShortWindowRem != 97 {
		t.Fatalf("expected FIFO sliding from 99 to 97, got first=%d, last=%d", samples[0].ShortWindowRem, samples[len(samples)-1].ShortWindowRem)
	}
}
