package priority

import (
	"math"
	"testing"

	"antigravity-priority/internal/state"
)

func TestCalculateTRequired(t *testing.T) {
	tests := []struct {
		name          string
		r7d           float64
		cycleBurnRate float64
		expected      float64
	}{
		{
			name:          "zero remaining quota requires 0 hours",
			r7d:           0.0,
			cycleBurnRate: 0.15,
			expected:      0.0,
		},
		{
			name:          "negative remaining quota requires 0 hours",
			r7d:           -0.5,
			cycleBurnRate: 0.15,
			expected:      0.0,
		},
		{
			name:          "80 percent balance with default 0.15 rate (26.67h)",
			r7d:           0.80,
			cycleBurnRate: 0.15,
			expected:      (0.80 / 0.15) * 5.0,
		},
		{
			name:          "82 percent balance with default 0.15 rate (27.33h)",
			r7d:           0.82,
			cycleBurnRate: 0.15,
			expected:      (0.82 / 0.15) * 5.0,
		},
		{
			name:          "zero/negative rate falls back to default 0.15",
			r7d:           0.30,
			cycleBurnRate: 0.0,
			expected:      (0.30 / state.DefaultCycleBurnRate) * 5.0,
		},
		{
			name:          "high burn rate 0.30 reduces required horizon",
			r7d:           0.60,
			cycleBurnRate: 0.30,
			expected:      (0.60 / 0.30) * 5.0, // 10 hours
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTRequired(tt.r7d, tt.cycleBurnRate)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("CalculateTRequired(%v, %v) = %v; want %v", tt.r7d, tt.cycleBurnRate, got, tt.expected)
			}
		})
	}
}

func TestIsBoostEligible(t *testing.T) {
	tests := []struct {
		name      string
		t7d       float64
		tRequired float64
		r7d       float64
		expected  bool
	}{
		{
			name:      "within dynamic boost horizon (T_7d < T_required)",
			t7d:       20.0,
			tRequired: 26.67,
			r7d:       0.80,
			expected:  true,
		},
		{
			name:      "exact boundary (T_7d == T_required)",
			t7d:       26.67,
			tRequired: 26.67,
			r7d:       0.80,
			expected:  true,
		},
		{
			name:      "outside boost horizon (T_7d > T_required)",
			t7d:       30.0,
			tRequired: 26.67,
			r7d:       0.80,
			expected:  false,
		},
		{
			name:      "zero remaining quota is never boosted",
			t7d:       1.0,
			tRequired: 10.0,
			r7d:       0.0,
			expected:  false,
		},
		{
			name:      "negative remaining quota is never boosted",
			t7d:       1.0,
			tRequired: 10.0,
			r7d:       -0.2,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBoostEligible(tt.t7d, tt.tRequired, tt.r7d)
			if got != tt.expected {
				t.Errorf("IsBoostEligible(%v, %v, %v) = %v; want %v", tt.t7d, tt.tRequired, tt.r7d, got, tt.expected)
			}
		})
	}
}
