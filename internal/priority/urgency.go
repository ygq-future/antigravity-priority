package priority

import (
	"math"
	"time"
)

// MinUrgencyTimeHorizonHours is the minimum denominator in hours for Weekly Urgency calculation.
const MinUrgencyTimeHorizonHours = 0.5

// HoursInWeeklyCycle represents total hours in a standard 7-day week.
const HoursInWeeklyCycle = 168.0

// QuotaMetrics encapsulates extracted and computed double-window quota metrics.
type QuotaMetrics struct {
	R7d              float64
	T7d              float64
	R5h              float64
	T5h              float64
	CycleBurnRate    float64
	TRequired        float64
	Urgency          float64
	IsBoosted        bool
	IsWeeklyDepleted bool
	IsShortDepleted  bool
}

// CalculateUrgency computes the Weekly Urgency Index (per-hour burn rate): R_7d / max(T_7d, 0.5).
func CalculateUrgency(r7d float64, t7d float64) float64 {
	if r7d <= 0 {
		return 0
	}
	denom := math.Max(t7d, MinUrgencyTimeHorizonHours)
	if denom <= 0 {
		denom = MinUrgencyTimeHorizonHours
	}
	return r7d / denom
}

// CalculateWeeklyPacingRatio scales per-hour urgency to a 168h week: (R_7d / max(T_7d, 0.5)) * 168.0.
// A value of 1.0 represents nominal pacing; >1.0 indicates elevated burn pressure.
func CalculateWeeklyPacingRatio(r7d float64, t7d float64) float64 {
	return CalculateUrgency(r7d, t7d) * HoursInWeeklyCycle
}

// CalculateCompositeScore computes the multi-dimensional composite health and scheduling score:
// - P_weekly: Weekly pacing ratio (weight 0.40)
// - R_7d: Weekly remaining ratio (weight 0.30)
// - R_5h: Short-window remaining ratio (weight 0.20)
// - A_5h: Short-window reset advantage (1 - T_5h/5.0, weight 0.10)
func CalculateCompositeScore(r7d float64, t7d float64, r5h float64, t5h float64) float64 {
	pWeekly := CalculateWeeklyPacingRatio(r7d, t7d)

	clampedR7d := math.Max(0.0, math.Min(1.0, r7d))
	clampedR5h := math.Max(0.0, math.Min(1.0, r5h))
	clampedT5h := math.Max(0.0, math.Min(5.0, t5h))
	a5h := 1.0 - (clampedT5h / 5.0)

	return 0.40*pWeekly + 0.30*clampedR7d + 0.20*clampedR5h + 0.10*a5h
}

// ExtractQuotaMetrics extracts normalized fractions and computes pacing metrics from probe evidence.
func ExtractQuotaMetrics(evidence QuotaEvidence, now time.Time) QuotaMetrics {
	r7d := extractR7d(evidence)
	t7d := extractT7d(evidence, now)
	r5h := extractR5h(evidence, r7d)
	t5h := extractT5h(evidence, now)

	cCycle := evidence.CycleBurnRate
	if cCycle <= 0 {
		cCycle = DefaultCycleBurnRate
	}

	tRequired := CalculateTRequired(r7d, cCycle)
	urgency := CalculateCompositeScore(r7d, t7d, r5h, t5h)

	isWeeklyDepleted := isWeeklyDepletedEvidence(evidence, r7d)
	isShortDepleted := !isWeeklyDepleted && isShortDepletedEvidence(evidence, r5h)
	isBoosted := !isWeeklyDepleted && !isShortDepleted && IsBoostEligible(t7d, tRequired, r7d)

	return QuotaMetrics{
		R7d:              r7d,
		T7d:              t7d,
		R5h:              r5h,
		T5h:              t5h,
		CycleBurnRate:    cCycle,
		TRequired:        tRequired,
		Urgency:          urgency,
		IsBoosted:        isBoosted,
		IsWeeklyDepleted: isWeeklyDepleted,
		IsShortDepleted:  isShortDepleted,
	}
}

func extractR7d(evidence QuotaEvidence) float64 {
	if evidence.LongWindowRemaining != nil {
		if *evidence.LongWindowRemaining <= 0 {
			return 0
		}
		return float64(*evidence.LongWindowRemaining) / 100.0
	}
	if evidence.Remaining != nil {
		if *evidence.Remaining <= 0 {
			return 0
		}
		return float64(*evidence.Remaining) / 100.0
	}
	return 0
}

func extractT7d(evidence QuotaEvidence, now time.Time) float64 {
	resetAt := evidence.LongWindowResetAt
	if resetAt == nil {
		resetAt = evidence.ResetAt
	}
	if resetAt == nil || resetAt.IsZero() || !resetAt.After(now) {
		return 0
	}
	return resetAt.Sub(now).Hours()
}

func extractR5h(evidence QuotaEvidence, fallbackR7d float64) float64 {
	if evidence.ShortWindowRemaining != nil {
		if *evidence.ShortWindowRemaining <= 0 {
			return 0
		}
		return float64(*evidence.ShortWindowRemaining) / 100.0
	}
	if evidence.Remaining != nil {
		if *evidence.Remaining <= 0 {
			return 0
		}
		return float64(*evidence.Remaining) / 100.0
	}
	return fallbackR7d
}

func extractT5h(evidence QuotaEvidence, now time.Time) float64 {
	resetAt := evidence.ShortWindowResetAt
	if resetAt == nil {
		resetAt = evidence.ResetAt
	}
	if resetAt == nil || resetAt.IsZero() || !resetAt.After(now) {
		return 0
	}
	return resetAt.Sub(now).Hours()
}

func isWeeklyDepletedEvidence(evidence QuotaEvidence, r7d float64) bool {
	if evidence.LongWindowRemaining != nil && *evidence.LongWindowRemaining <= 0 {
		return true
	}
	if evidence.LongWindowRemaining == nil && evidence.Remaining != nil && *evidence.Remaining <= 0 {
		return true
	}
	return r7d <= 0 && (evidence.LongWindowRemaining != nil || evidence.Remaining != nil)
}

func isShortDepletedEvidence(evidence QuotaEvidence, r5h float64) bool {
	if evidence.ShortWindowRemaining != nil && *evidence.ShortWindowRemaining <= 0 {
		return true
	}
	return r5h <= 0 && evidence.ShortWindowRemaining != nil
}
