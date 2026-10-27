package priority

import (
	"cmp"
	"math"
	"slices"

	"antigravity-priority/internal/core"
)

const floatEpsilon = 1e-9

// CompareHealthyCandidates compares two healthy active candidates based on the scheduling rules:
// 0. Non-Free (PRO) before Free.
// 1. Weekly Urgency descending (higher urgency first).
// 2. 5h reset countdown T_5h ascending (earliest reset first).
// 3. AuthIndex ascending (lexicographical deterministic tie-breaking).
func CompareHealthyCandidates(left PlanItem, right PlanItem) int {
	// 0. Non-Free (PRO) before Free
	if left.PlanType != right.PlanType {
		if left.PlanType != core.PlanTypeFree && right.PlanType == core.PlanTypeFree {
			return -1
		}
		if left.PlanType == core.PlanTypeFree && right.PlanType != core.PlanTypeFree {
			return 1
		}
	}
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

