package state_test

import (
	"testing"
	"time"

	"antigravity-priority/internal/state"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func TestEstimator_ColdStart(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)

	rate := state.CalculateCycleBurnRate(
		0.0, // cold start
		nil,
		nil,
		shortReset,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
	)

	if rate != state.DefaultCycleBurnRate {
		t.Fatalf("expected default rate %v for cold start, got %v", state.DefaultCycleBurnRate, rate)
	}

	// Test negative rate fallback
	rateNeg := state.CalculateCycleBurnRate(
		-0.5,
		nil,
		nil,
		shortReset,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
	)
	if rateNeg != state.DefaultCycleBurnRate {
		t.Fatalf("expected default rate for negative input, got %v", rateNeg)
	}
}

func TestEstimator_NilAndZeroPointers(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	learnedRate := 0.22

	// Nil pointers
	rate := state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(100),
		nil,
		shortReset,
		int64Ptr(80),
		int64Ptr(90),
		shortReset,
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate on nil prevLong, got %v", rate)
	}

	rate = state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
		int64Ptr(80),
		nil,
		shortReset,
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate on nil currLong, got %v", rate)
	}

	// Zero reset timestamps
	rate = state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(100),
		int64Ptr(100),
		time.Time{},
		int64Ptr(80),
		int64Ptr(90),
		shortReset,
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate on zero prevShortReset, got %v", rate)
	}

	rate = state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
		int64Ptr(80),
		int64Ptr(90),
		time.Time{},
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate on zero currShortReset, got %v", rate)
	}
}

func TestEstimator_ZeroConsumption(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	learnedRate := 0.22

	// Zero consumption: remaining 5h and 7d unchanged
	rate := state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(80),
		int64Ptr(90),
		shortReset,
		int64Ptr(80),
		int64Ptr(90),
		shortReset,
	)

	if rate != learnedRate {
		t.Fatalf("expected preserved rate %v on zero consumption, got %v", learnedRate, rate)
	}

	// Negative weekly delta (e.g. 7d quota increased / corrected)
	rate = state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(80),
		int64Ptr(90),
		shortReset,
		int64Ptr(70), // consumed 10% of 5h
		int64Ptr(95), // but 7d increased from 90 to 95 -> delta7d <= 0
		shortReset,
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate when delta 7d <= 0, got %v", rate)
	}
}

func TestEstimator_UnderThreshold(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	learnedRate := 0.22

	// Consumption < 5% (delta 5h = 4%, e.g. 100 -> 96)
	rate := state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
		int64Ptr(96),
		int64Ptr(99),
		shortReset,
	)

	if rate != learnedRate {
		t.Fatalf("expected preserved rate %v when delta 5h < 5%%, got %v", learnedRate, rate)
	}
}

func TestEstimator_WindowResetBoundary(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	prevShortReset := now.Add(1 * time.Hour)
	currShortReset := now.Add(5 * time.Hour) // window refreshed
	learnedRate := 0.25

	// 5h window reset boundary: reset time changed
	rate := state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(10),
		int64Ptr(70),
		prevShortReset,
		int64Ptr(100),
		int64Ptr(70),
		currShortReset,
	)

	if rate != learnedRate {
		t.Fatalf("expected preserved rate %v across window reset boundary, got %v", learnedRate, rate)
	}

	// 5h quota refreshed (increased) within same reset time (anomaly/replenishment)
	rate = state.CalculateCycleBurnRate(
		learnedRate,
		int64Ptr(50),
		int64Ptr(70),
		prevShortReset,
		int64Ptr(80), // increased from 50 to 80
		int64Ptr(70),
		prevShortReset,
	)
	if rate != learnedRate {
		t.Fatalf("expected preserved rate when 5h quota increased, got %v", rate)
	}
}

func TestEstimator_ValidUpdateAndEMA(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	prevRate := 0.15

	// delta 5h = 20% (100 -> 80), delta 7d = 4% (100 -> 96)
	// C_obs = 0.04 / 0.20 = 0.20
	// C_new = 0.3 * 0.20 + 0.7 * 0.15 = 0.06 + 0.105 = 0.165
	rate := state.CalculateCycleBurnRate(
		prevRate,
		int64Ptr(100),
		int64Ptr(100),
		shortReset,
		int64Ptr(80),
		int64Ptr(96),
		shortReset,
	)

	expected := 0.165
	if rate < expected-1e-6 || rate > expected+1e-6 {
		t.Fatalf("expected EMA rate %v, got %v", expected, rate)
	}

	// Step 2: another observation with delta 5h = 30% (80 -> 50), delta 7d = 6% (96 -> 90)
	// C_obs = 0.06 / 0.30 = 0.20
	// C_new = 0.3 * 0.20 + 0.7 * 0.165 = 0.06 + 0.1155 = 0.1755
	rate2 := state.CalculateCycleBurnRate(
		rate,
		int64Ptr(80),
		int64Ptr(96),
		shortReset,
		int64Ptr(50),
		int64Ptr(90),
		shortReset,
	)

	expected2 := 0.1755
	if rate2 < expected2-1e-6 || rate2 > expected2+1e-6 {
		t.Fatalf("expected 2nd step EMA rate %v, got %v", expected2, rate2)
	}
}

func TestEstimator_Clamping(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	shortReset := now.Add(4 * time.Hour)
	prevRate := 0.15

	t.Run("upper clamp 0.30", func(t *testing.T) {
		// delta 5h = 10% (100 -> 90), delta 7d = 50% (100 -> 50)
		// C_obs = 0.50 / 0.10 = 5.0 -> clamped to 0.30
		// C_new = 0.3 * 0.30 + 0.7 * 0.15 = 0.09 + 0.105 = 0.195
		rate := state.CalculateCycleBurnRate(
			prevRate,
			int64Ptr(100),
			int64Ptr(100),
			shortReset,
			int64Ptr(90),
			int64Ptr(50),
			shortReset,
		)
		expected := 0.195
		if rate < expected-1e-6 || rate > expected+1e-6 {
			t.Fatalf("expected upper clamped rate %v, got %v", expected, rate)
		}
	})

	t.Run("lower clamp 0.08", func(t *testing.T) {
		// delta 5h = 50% (100 -> 50), delta 7d = 1% (100 -> 99)
		// C_obs = 0.01 / 0.50 = 0.02 -> clamped to 0.08
		// C_new = 0.3 * 0.08 + 0.7 * 0.20 = 0.024 + 0.140 = 0.164
		rate := state.CalculateCycleBurnRate(
			0.20,
			int64Ptr(100),
			int64Ptr(100),
			shortReset,
			int64Ptr(50),
			int64Ptr(99),
			shortReset,
		)
		expected := 0.164
		if rate < expected-1e-6 || rate > expected+1e-6 {
			t.Fatalf("expected lower clamped rate %v, got %v", expected, rate)
		}
	})
}
