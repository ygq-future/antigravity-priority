package priority

import (
	"antigravity-priority/internal/state"
)

const (
	// DefaultBoostStartPriority is the default starting priority for boosted credentials.
	DefaultBoostStartPriority = 999
	// DefaultNormalStartPriority is the default starting priority for regular active credentials.
	DefaultNormalStartPriority = 100
	// MaxPriority is the upper bound for any assignable positive priority.
	MaxPriority = 999
	// MinPriority is the lower bound for active positive priorities.
	MinPriority = 1
	// DepletedPriority is the negative priority assigned to depleted credentials.
	DepletedPriority = -1
	// ShortWindowHours represents the duration in hours of a single Antigravity short rolling window.
	ShortWindowHours = 5.0
)

// CalculateTRequired computes the physical time in hours required to burn remaining weekly quota:
// T_required = (R_7d / C_cycle) * 5.0.
func CalculateTRequired(r7d float64, cycleBurnRate float64) float64 {
	if r7d <= 0 {
		return 0
	}
	if cycleBurnRate <= 0 {
		cycleBurnRate = state.DefaultCycleBurnRate
	}
	return (r7d / cycleBurnRate) * ShortWindowHours
}

// IsBoostEligible determines if a credential qualifies for the dynamic 999 priority boost tier:
// IsBoosted = (T_7d <= T_required) && (R_7d > 0).
func IsBoostEligible(t7d float64, tRequired float64, r7d float64) bool {
	return r7d > 0 && t7d <= tRequired
}
