package priority

import (
	"math"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/state"
)

func TestCalculateUrgency(t *testing.T) {
	tests := []struct {
		name     string
		r7d      float64
		t7d      float64
		expected float64
	}{
		{
			name:     "zero remaining weekly quota",
			r7d:      0.0,
			t7d:      10.0,
			expected: 0.0,
		},
		{
			name:     "negative remaining weekly quota",
			r7d:      -0.1,
			t7d:      10.0,
			expected: 0.0,
		},
		{
			name:     "ample time horizon (> 0.5h)",
			r7d:      0.80,
			t7d:      10.0,
			expected: 0.08,
		},
		{
			name:     "sub-30min horizon clamped to 0.5h",
			r7d:      0.80,
			t7d:      0.2,
			expected: 1.60,
		},
		{
			name:     "exact 0.5h boundary",
			r7d:      0.50,
			t7d:      0.5,
			expected: 1.00,
		},
		{
			name:     "zero time remaining clamped to 0.5h",
			r7d:      0.60,
			t7d:      0.0,
			expected: 1.20,
		},
		{
			name:     "negative time remaining clamped to 0.5h",
			r7d:      0.40,
			t7d:      -5.0,
			expected: 0.80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateUrgency(tt.r7d, tt.t7d)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("CalculateUrgency(%v, %v) = %v; want %v", tt.r7d, tt.t7d, got, tt.expected)
			}
		})
	}
}

func TestExtractQuotaMetrics(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	longReset := now.Add(20 * time.Hour)
	shortReset := now.Add(2 * time.Hour)

	t.Run("full double-window healthy metrics", func(t *testing.T) {
		r7dVal := int64(80)
		r5hVal := int64(90)
		ev := ProbeEvidence{
			AuthIndex:            "auth-1",
			Provider:             core.ProviderAntigravity,
			LongWindowRemaining:  &r7dVal,
			LongWindowResetAt:    &longReset,
			ShortWindowRemaining: &r5hVal,
			ShortWindowResetAt:   &shortReset,
			CycleBurnRate:        0.15,
			EvidenceFresh:        true,
			Freshness:            core.FreshnessFresh,
			ProbeStatus:          core.ProbeStatusReady,
			Status:               EvidenceStatusReady,
		}

		metrics := ExtractQuotaMetrics(ev, now)

		if math.Abs(metrics.R7d-0.80) > 1e-9 {
			t.Errorf("R7d = %v; want 0.80", metrics.R7d)
		}
		if math.Abs(metrics.T7d-20.0) > 1e-9 {
			t.Errorf("T7d = %v; want 20.0", metrics.T7d)
		}
		if math.Abs(metrics.R5h-0.90) > 1e-9 {
			t.Errorf("R5h = %v; want 0.90", metrics.R5h)
		}
		if math.Abs(metrics.T5h-2.0) > 1e-9 {
			t.Errorf("T5h = %v; want 2.0", metrics.T5h)
		}
		if math.Abs(metrics.CycleBurnRate-0.15) > 1e-9 {
			t.Errorf("CycleBurnRate = %v; want 0.15", metrics.CycleBurnRate)
		}
		// TRequired = (0.80 / 0.15) * 5 = 26.6666...
		expectedTReq := (0.80 / 0.15) * 5.0
		if math.Abs(metrics.TRequired-expectedTReq) > 1e-9 {
			t.Errorf("TRequired = %v; want %v", metrics.TRequired, expectedTReq)
		}
		// T7d (20h) <= TRequired (26.67h) -> IsBoosted should be true
		if !metrics.IsBoosted {
			t.Errorf("IsBoosted = false; want true")
		}
		if metrics.IsWeeklyDepleted {
			t.Errorf("IsWeeklyDepleted = true; want false")
		}
		if metrics.IsShortDepleted {
			t.Errorf("IsShortDepleted = true; want false")
		}
	})

	t.Run("weekly depleted overrides short window", func(t *testing.T) {
		r7dVal := int64(0)
		r5hVal := int64(50)
		ev := ProbeEvidence{
			AuthIndex:            "auth-2",
			LongWindowRemaining:  &r7dVal,
			LongWindowResetAt:    &longReset,
			ShortWindowRemaining: &r5hVal,
			ShortWindowResetAt:   &shortReset,
		}

		metrics := ExtractQuotaMetrics(ev, now)

		if !metrics.IsWeeklyDepleted {
			t.Errorf("IsWeeklyDepleted = false; want true")
		}
		if metrics.IsShortDepleted {
			t.Errorf("IsShortDepleted = true; want false (weekly takes precedence)")
		}
		if metrics.IsBoosted {
			t.Errorf("IsBoosted = true; want false")
		}
	})

	t.Run("short window soft depleted when weekly positive", func(t *testing.T) {
		r7dVal := int64(50)
		r5hVal := int64(0)
		ev := ProbeEvidence{
			AuthIndex:            "auth-3",
			LongWindowRemaining:  &r7dVal,
			LongWindowResetAt:    &longReset,
			ShortWindowRemaining: &r5hVal,
			ShortWindowResetAt:   &shortReset,
		}

		metrics := ExtractQuotaMetrics(ev, now)

		if metrics.IsWeeklyDepleted {
			t.Errorf("IsWeeklyDepleted = true; want false")
		}
		if !metrics.IsShortDepleted {
			t.Errorf("IsShortDepleted = false; want true")
		}
		if metrics.IsBoosted {
			t.Errorf("IsBoosted = true; want false")
		}
	})

	t.Run("fallback to single remaining and default cycle burn rate", func(t *testing.T) {
		remVal := int64(60)
		resetAt := now.Add(5 * time.Hour)
		ev := ProbeEvidence{
			AuthIndex:  "auth-4",
			Remaining:  &remVal,
			ResetAt:    &resetAt,
			ObservedAt: now,
		}

		metrics := ExtractQuotaMetrics(ev, now)

		if math.Abs(metrics.R7d-0.60) > 1e-9 {
			t.Errorf("R7d = %v; want 0.60", metrics.R7d)
		}
		if math.Abs(metrics.R5h-0.60) > 1e-9 {
			t.Errorf("R5h fallback = %v; want 0.60", metrics.R5h)
		}
		if math.Abs(metrics.CycleBurnRate-state.DefaultCycleBurnRate) > 1e-9 {
			t.Errorf("CycleBurnRate default = %v; want %v", metrics.CycleBurnRate, state.DefaultCycleBurnRate)
		}
	})

	t.Run("past reset times yield zero duration", func(t *testing.T) {
		remVal := int64(10)
		pastReset := now.Add(-1 * time.Hour)
		ev := ProbeEvidence{
			AuthIndex: "auth-5",
			Remaining: &remVal,
			ResetAt:   &pastReset,
		}

		metrics := ExtractQuotaMetrics(ev, now)
		if metrics.T7d != 0 {
			t.Errorf("T7d past = %v; want 0", metrics.T7d)
		}
		if metrics.T5h != 0 {
			t.Errorf("T5h past = %v; want 0", metrics.T5h)
		}
	})

	t.Run("nil and zero remaining extractions", func(t *testing.T) {
		rZero := int64(0)
		evNil := ProbeEvidence{}
		mNil := ExtractQuotaMetrics(evNil, now)
		if mNil.R7d != 0 || mNil.R5h != 0 {
			t.Errorf("nil remaining should extract 0")
		}

		evZeroRem := ProbeEvidence{
			Remaining: &rZero,
		}
		mZero := ExtractQuotaMetrics(evZeroRem, now)
		if mZero.R7d != 0 || mZero.R5h != 0 || !mZero.IsWeeklyDepleted {
			t.Errorf("zero Remaining should be weekly depleted and 0 fraction")
		}

		evZeroShort := ProbeEvidence{
			ShortWindowRemaining: &rZero,
		}
		mZeroShort := ExtractQuotaMetrics(evZeroShort, now)
		if mZeroShort.R5h != 0 {
			t.Errorf("zero ShortWindowRemaining should be 0")
		}
	})
}
