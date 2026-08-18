package priority

import (
	"cmp"
	"math"
	"slices"
)

const floatEpsilon = 1e-9

// CompareHealthyCandidates compares two healthy active candidates based on the 3-tier rules:
// 1. Weekly Urgency descending (higher urgency first).
// 2. 5h reset countdown T_5h ascending (earliest reset first).
// 3. AuthIndex ascending (lexicographical deterministic tie-breaking).
func CompareHealthyCandidates(left PlanItem, right PlanItem) int {
	// 1. Weekly Urgency descending
	if math.Abs(left.Urgency-right.Urgency) > floatEpsilon {
		if left.Urgency > right.Urgency {
			return -1
		}
		return 1
	}

	// 2. 5h Reset countdown ascending (earliest reset first)
	if math.Abs(left.T5h-right.T5h) > floatEpsilon {
		if left.T5h < right.T5h {
			return -1
		}
		return 1
	}

	// 3. Deterministic tie-breaker: AuthIndex ascending
	return cmp.Compare(left.Credential.AuthIndex, right.Credential.AuthIndex)
}

// CompareUniquenessCandidates orders candidate credentials for non-colliding priority allocation.
func CompareUniquenessCandidates(left PlanItem, right PlanItem) int {
	leftFreshPositive := left.EvidenceFresh && isPositiveRemaining(left)
	rightFreshPositive := right.EvidenceFresh && isPositiveRemaining(right)

	switch {
	case leftFreshPositive && rightFreshPositive:
		if left.IsBoosted && !right.IsBoosted {
			return -1
		}
		if !left.IsBoosted && right.IsBoosted {
			return 1
		}
		return CompareHealthyCandidates(left, right)
	case leftFreshPositive:
		return -1
	case rightFreshPositive:
		return 1
	}

	// Unprobed peers without fresh positive evidence: higher existing priority first, then AuthIndex
	if left.Priority != right.Priority {
		return right.Priority - left.Priority
	}
	return cmp.Compare(left.Credential.AuthIndex, right.Credential.AuthIndex)
}

// SortPlanItems orders items deterministically for the final Plan representation.
func SortPlanItems(items []PlanItem) {
	slices.SortStableFunc(items, func(left PlanItem, right PlanItem) int {
		if left.EvidenceFresh && !right.EvidenceFresh {
			return -1
		}
		if !left.EvidenceFresh && right.EvidenceFresh {
			return 1
		}
		if left.Priority != right.Priority {
			return right.Priority - left.Priority
		}
		if left.Disabled != right.Disabled {
			if !left.Disabled {
				return -1
			}
			return 1
		}
		return cmp.Compare(left.Credential.AuthIndex, right.Credential.AuthIndex)
	})
}

func isPositiveRemaining(item PlanItem) bool {
	if item.LongWindowRemaining != nil && *item.LongWindowRemaining > 0 {
		return true
	}
	if item.Remaining != nil && *item.Remaining > 0 {
		return true
	}
	return item.R7d > 0
}
