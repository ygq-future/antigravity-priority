package state

import (
	"time"
)

const (
	// DefaultCycleBurnRate is the initial cold-start baseline fraction of weekly quota burnable per 5h cycle.
	DefaultCycleBurnRate = 0.15
	// MinCycleBurnRate is the lower clamp bound for observed cycle burn rate.
	MinCycleBurnRate = 0.08
	// MaxCycleBurnRate is the upper clamp bound for observed cycle burn rate.
	MaxCycleBurnRate = 0.30
	// MinDeltaThreshold is the minimum 5h quota consumption required to trigger a burn rate estimation update (5%).
	MinDeltaThreshold = 0.05
	// EMASmoothingAlpha is the exponential moving average smoothing factor.
	EMASmoothingAlpha = 0.3
)

// CalculateCycleBurnRate adaptively estimates the cycle burn rate from in-window quota consumption deltas.
func CalculateCycleBurnRate(
	prevRate float64,
	prevShortRem, prevLongRem *int64,
	prevShortReset time.Time,
	currShortRem, currLongRem *int64,
	currShortReset time.Time,
) float64 {
	if prevRate <= 0 {
		prevRate = DefaultCycleBurnRate
	}

	if prevShortRem == nil || prevLongRem == nil || currShortRem == nil || currLongRem == nil {
		return prevRate
	}

	if prevShortReset.IsZero() || currShortReset.IsZero() {
		return prevRate
	}

	// If the 5-hour window reset time has changed or remaining 5h quota increased, a window reset occurred.
	if !prevShortReset.Equal(currShortReset) || *currShortRem > *prevShortRem {
		return prevRate
	}

	delta5h := float64(*prevShortRem-*currShortRem) / 100.0
	delta7d := float64(*prevLongRem-*currLongRem) / 100.0

	// Must consume at least 5% of 5h quota and positive weekly quota.
	if delta5h < MinDeltaThreshold || delta7d <= 0 {
		return prevRate
	}

	obs := delta7d / delta5h
	clamped := clamp(obs, MinCycleBurnRate, MaxCycleBurnRate)

	return EMASmoothingAlpha*clamped + (1.0-EMASmoothingAlpha)*prevRate
}

func clamp(val, minVal, maxVal float64) float64 {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
