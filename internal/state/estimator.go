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
	Sequence           uint64    `json:"sequence"`
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
	learningBaselineSequence uint64,
	currObservedAt time.Time,
	currShortReset time.Time,
	currShortRem, currLongRem *int64,
	capacity int,
) (float64, []QuotaSample, uint64) {
	if prevRate <= 0 {
		prevRate = DefaultCycleBurnRate
	}
	if capacity < MinQuotaSampleCapacity || capacity > MaxQuotaSampleCapacity {
		capacity = DefaultQuotaSampleCapacity
	}
	samples, learningBaselineSequence = normalizeSampleHistory(samples, learningBaselineSequence)
	if currShortRem == nil || currLongRem == nil || currShortReset.IsZero() {
		return prevRate, samples, learningBaselineSequence
	}

	currSample := QuotaSample{
		Sequence:           nextSampleSequence(samples),
		ObservedAt:         currObservedAt.UTC(),
		ShortWindowResetAt: currShortReset.UTC(),
		ShortWindowRem:     *currShortRem,
		LongWindowRem:      *currLongRem,
	}

	// 1. Initial sample on cold start
	if len(samples) == 0 {
		return prevRate, []QuotaSample{currSample}, currSample.Sequence
	}

	lastSample := samples[len(samples)-1]

	// 2. Zero-consumption deduplication:
	// If both quota values are unchanged, refresh the latest observation metadata
	// without appending a duplicate sample. Rolling windows may move their reset
	// timestamp between probes even when no quota has been consumed.
	if currSample.ShortWindowRem == lastSample.ShortWindowRem &&
		currSample.LongWindowRem == lastSample.LongWindowRem {
		updatedSamples := append([]QuotaSample(nil), samples...)
		updatedSamples[len(updatedSamples)-1].ObservedAt = currSample.ObservedAt
		updatedSamples[len(updatedSamples)-1].ShortWindowResetAt = currSample.ShortWindowResetAt
		return prevRate, updatedSamples, learningBaselineSequence
	}

	// 3. Append new sample and maintain sliding window capacity (FIFO).
	updatedSamples := append([]QuotaSample(nil), samples...)
	updatedSamples = append(updatedSamples, currSample)
	if len(updatedSamples) > capacity {
		updatedSamples = updatedSamples[len(updatedSamples)-capacity:]
	}

	// 4. A reset boundary or replenishment starts a new learning span without deleting history.
	if !currSample.ShortWindowResetAt.Equal(lastSample.ShortWindowResetAt) || currSample.ShortWindowRem > lastSample.ShortWindowRem {
		return prevRate, updatedSamples, currSample.Sequence
	}

	// 5. Resolve the learning cursor, using the oldest retained observation after FIFO rotation.
	baseSample, found := sampleBySequence(updatedSamples, learningBaselineSequence)
	if !found {
		baseSample = updatedSamples[0]
		learningBaselineSequence = baseSample.Sequence
	}
	delta5h := float64(baseSample.ShortWindowRem-currSample.ShortWindowRem) / 100.0
	delta7d := float64(baseSample.LongWindowRem-currSample.LongWindowRem) / 100.0

	// Must consume at least 5% of 5h quota and positive weekly quota
	if delta5h < MinDeltaThreshold || delta7d <= 0 {
		return prevRate, updatedSamples, learningBaselineSequence
	}

	// 6. Compute observed rate, clamp, and apply EMA smoothing
	obs := delta7d / delta5h
	clamped := clamp(obs, MinCycleBurnRate, MaxCycleBurnRate)
	newRate := EMASmoothingAlpha*clamped + (1.0-EMASmoothingAlpha)*prevRate

	// 7. Advance only the learning cursor so history remains available for trend inspection.
	return newRate, updatedSamples, currSample.Sequence
}

func normalizeSampleHistory(samples []QuotaSample, baseline uint64) ([]QuotaSample, uint64) {
	if len(samples) == 0 {
		return nil, 0
	}
	normalized := make([]QuotaSample, 0, len(samples))
	var previous uint64
	for _, sample := range samples {
		if sample.Sequence == 0 || sample.Sequence <= previous {
			sample.Sequence = previous + 1
		}
		previous = sample.Sequence

		if len(normalized) > 0 {
			last := &normalized[len(normalized)-1]
			if sample.ShortWindowRem == last.ShortWindowRem && sample.LongWindowRem == last.LongWindowRem {
				last.ObservedAt = sample.ObservedAt
				last.ShortWindowResetAt = sample.ShortWindowResetAt
				if baseline == sample.Sequence {
					baseline = last.Sequence
				}
				continue
			}
		}
		normalized = append(normalized, sample)
	}
	if baseline == 0 {
		baseline = normalized[0].Sequence
	} else if _, found := sampleBySequence(normalized, baseline); !found {
		baseline = normalized[0].Sequence
	}
	return normalized, baseline
}

func nextSampleSequence(samples []QuotaSample) uint64 {
	if len(samples) == 0 {
		return 1
	}
	return samples[len(samples)-1].Sequence + 1
}

func sampleBySequence(samples []QuotaSample, sequence uint64) (QuotaSample, bool) {
	for _, sample := range samples {
		if sample.Sequence == sequence {
			return sample, true
		}
	}
	return QuotaSample{}, false
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
