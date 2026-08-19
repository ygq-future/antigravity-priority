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
	// DefaultQuotaSampleCapacity is the default sliding window capacity for multi-sample estimation.
	DefaultQuotaSampleCapacity = 6
	// MinQuotaSampleCapacity is the minimum allowed sample capacity.
	MinQuotaSampleCapacity = 2
	// MaxQuotaSampleCapacity is the maximum allowed sample capacity.
	MaxQuotaSampleCapacity = 30
)

// QuotaSample represents a single point-in-time quota observation.
type QuotaSample struct {
	ObservedAt         time.Time `json:"observed_at"`
	ShortWindowResetAt time.Time `json:"short_window_reset_at,omitempty"`
	ShortWindowRem     int64     `json:"short_window_rem"`
	LongWindowRem      int64     `json:"long_window_rem"`
}

// UpdateSamplesAndCycleBurnRate updates the sliding window sample queue and adaptively estimates
// the cycle burn rate across the multi-sample span.
func UpdateSamplesAndCycleBurnRate(
	prevRate float64,
	samples []QuotaSample,
	currObservedAt time.Time,
	currShortReset time.Time,
	currShortRem, currLongRem *int64,
	capacity int,
) (float64, []QuotaSample) {
	if prevRate <= 0 {
		prevRate = DefaultCycleBurnRate
	}
	if capacity < MinQuotaSampleCapacity || capacity > MaxQuotaSampleCapacity {
		capacity = DefaultQuotaSampleCapacity
	}
	if currShortRem == nil || currLongRem == nil || currShortReset.IsZero() {
		return prevRate, samples
	}

	currSample := QuotaSample{
		ObservedAt:         currObservedAt.UTC(),
		ShortWindowResetAt: currShortReset.UTC(),
		ShortWindowRem:     *currShortRem,
		LongWindowRem:      *currLongRem,
	}

	// 1. Initial sample on cold start
	if len(samples) == 0 {
		return prevRate, []QuotaSample{currSample}
	}

	lastSample := samples[len(samples)-1]

	// 2. Window reset boundary or replenishment detection:
	// If short window reset time changed or 5h quota increased, 5h cycle has reset -> start fresh window.
	if !currSample.ShortWindowResetAt.Equal(lastSample.ShortWindowResetAt) || currSample.ShortWindowRem > lastSample.ShortWindowRem {
		return prevRate, []QuotaSample{currSample}
	}

	// 3. Zero-consumption deduplication:
	// If neither 5h nor 7d quota changed, update the timestamp of the latest sample without appending duplicates.
	if currSample.ShortWindowRem == lastSample.ShortWindowRem && currSample.LongWindowRem == lastSample.LongWindowRem {
		updatedSamples := append([]QuotaSample(nil), samples...)
		updatedSamples[len(updatedSamples)-1].ObservedAt = currSample.ObservedAt
		return prevRate, updatedSamples
	}

	// 4. Append new sample and maintain sliding window capacity (FIFO)
	updatedSamples := append([]QuotaSample(nil), samples...)
	updatedSamples = append(updatedSamples, currSample)
	if len(updatedSamples) > capacity {
		updatedSamples = updatedSamples[len(updatedSamples)-capacity:]
	}

	// 5. Multi-sample span delta calculation against baseline sample (earliest in current window)
	baseSample := updatedSamples[0]
	delta5h := float64(baseSample.ShortWindowRem-currSample.ShortWindowRem) / 100.0
	delta7d := float64(baseSample.LongWindowRem-currSample.LongWindowRem) / 100.0

	// Must consume at least 5% of 5h quota and positive weekly quota
	if delta5h < MinDeltaThreshold || delta7d <= 0 {
		return prevRate, updatedSamples
	}

	// 6. Compute observed rate, clamp, and apply EMA smoothing
	obs := delta7d / delta5h
	clamped := clamp(obs, MinCycleBurnRate, MaxCycleBurnRate)
	newRate := EMASmoothingAlpha*clamped + (1.0-EMASmoothingAlpha)*prevRate

	// 7. Advance baseline: reset samples to start from currSample so the learned delta is not double-counted
	return newRate, []QuotaSample{currSample}
}

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
