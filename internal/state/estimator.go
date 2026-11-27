package state

import (
	"math"
	"strings"
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
	// MinReplenishmentPoints is the same-window increase required to rebase learning instead of treating it as quota jitter.
	MinReplenishmentPoints int64 = 5
	// MinTokenCalibrationDelta is the minimum 5h percentage drop required to calibrate TokensPerShortPct.
	MinTokenCalibrationDelta int64 = 2
	// MinExternalRebaseDelta is the minimum zero-local-usage drop that triggers an immediate baseline rebase.
	MinExternalRebaseDelta int64 = 2
	// MinAnomalyConfirmationStreak is the number of consecutive consistent local-traffic anomalies
	// required to confirm a genuine upstream pricing/tier shift and re-anchor TokensPerShortPct.
	MinAnomalyConfirmationStreak = 3
	// EMASmoothingAlpha is the exponential moving average smoothing factor.
	EMASmoothingAlpha = 0.3
	// DefaultQuotaSampleCapacity is the default sliding window capacity for multi-sample estimation.
	DefaultQuotaSampleCapacity = 6
	// MinQuotaSampleCapacity is the minimum allowed sample capacity.
	MinQuotaSampleCapacity = 2
	// MaxQuotaSampleCapacity is the maximum allowed sample capacity.
	MaxQuotaSampleCapacity = 30

	// DefaultTokensPerShortPctGemini is default weighted tokens per 1% 5h quota for Gemini.
	DefaultTokensPerShortPctGemini = 80000.0
	// DefaultTokensPerShortPctClaude is default weighted tokens per 1% 5h quota for Claude & GPT.
	DefaultTokensPerShortPctClaude = 40000.0
	// MinTokensPerShortPct is the minimum clamp bound for learned token sensitivity.
	MinTokensPerShortPct = 5000.0
	// MaxTokensPerShortPct is the maximum clamp bound for learned token sensitivity.
	MaxTokensPerShortPct = 500000.0
)

// CalculateWeightedTurnTokens computes weighted equivalent tokens for a single request turn.
func CalculateWeightedTurnTokens(inputTokens, outputTokens, reasoningTokens, cacheReadTokens, totalTokens int64) float64 {
	uncachedInput := max(inputTokens-cacheReadTokens, 0)
	weighted := float64(uncachedInput) + 0.1*float64(cacheReadTokens) + 4.0*float64(outputTokens+reasoningTokens)
	if weighted <= 0 {
		if totalTokens > 2000 {
			return float64(totalTokens)
		}
		return 2000.0
	}
	return weighted
}

func defaultTokensPerShortPct(group string) float64 {
	if strings.Contains(strings.ToLower(group), "gemini") {
		return DefaultTokensPerShortPctGemini
	}
	return DefaultTokensPerShortPctClaude
}

func effectiveTokensPerShortPct(entry Entry) float64 {
	if entry.TokensPerShortPct > 0 {
		return entry.TokensPerShortPct
	}
	return defaultTokensPerShortPct(entry.ModelGroup)
}

func effectiveCycleBurnRate(entry Entry) float64 {
	if entry.CycleBurnRate > 0 {
		return entry.CycleBurnRate
	}
	return DefaultCycleBurnRate
}

// QuotaSample represents a single point-in-time quota observation.
type QuotaSample struct {
	Sequence           uint64    `json:"sequence"`
	ObservedAt         time.Time `json:"observed_at"`
	ShortWindowResetAt time.Time `json:"short_window_reset_at,omitzero"`
	ShortWindowRem     int64     `json:"short_window_rem"`
	LongWindowRem      int64     `json:"long_window_rem"`
	ProbeRoundID       string    `json:"probe_round_id,omitempty"`
	AnomalySkipped     bool      `json:"anomaly_skipped,omitempty"`
}

// QuotaLearningInput carries the persisted estimator state and local usage
// accumulators needed to evaluate one probe observation.
type QuotaLearningInput struct {
	ModelGroup                   string
	PrevCycleBurnRate            float64
	PrevTokensPerShortPct        float64
	PrevSamples                  []QuotaSample
	LearningBaselineSequence     uint64
	PrevShortResetAt             time.Time
	PrevShortRemaining           *int64
	StepPendingTokens            float64
	StepEstimatedShortPct        float64
	LearningPendingTokens        float64
	LearningEstimatedShortPct    float64
	SensitivityAnomalyStreak     int
	LastAnomalyTokensPerShortPct float64
	CurrObservedAt               time.Time
	CurrShortResetAt             time.Time
	CurrShortRemaining           *int64
	CurrLongRemaining            *int64
	SampleCapacity               int
}

// QuotaLearningOutput returns the updated sample history and learned parameters
// after applying EMA-gated attribution checks.
type QuotaLearningOutput struct {
	CycleBurnRate                float64
	TokensPerShortPct            float64
	Samples                      []QuotaSample
	LearningBaselineSequence     uint64
	LearningPendingTokens        float64
	LearningEstimatedShortPct    float64
	SensitivityAnomalyStreak     int
	LastAnomalyTokensPerShortPct float64
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

	// 4. A reset boundary or material replenishment starts a new learning span without deleting history.
	// Small same-window increases can occur because the upstream estimate is rounded.
	if !currSample.ShortWindowResetAt.Equal(lastSample.ShortWindowResetAt) || currSample.ShortWindowRem-lastSample.ShortWindowRem >= MinReplenishmentPoints {
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

// CalibrateQuotaLearning updates sample history, TokensPerShortPct, and CycleBurnRate
// while filtering out unattributed external quota drops that deviate from the current
// EMA-predicted consumption envelope.
func CalibrateQuotaLearning(in QuotaLearningInput) QuotaLearningOutput {
	prevRate := in.PrevCycleBurnRate
	if prevRate <= 0 {
		prevRate = DefaultCycleBurnRate
	}
	capacity := in.SampleCapacity
	if capacity < MinQuotaSampleCapacity || capacity > MaxQuotaSampleCapacity {
		capacity = DefaultQuotaSampleCapacity
	}
	samples, baseline := normalizeSampleHistory(in.PrevSamples, in.LearningBaselineSequence)
	out := QuotaLearningOutput{
		CycleBurnRate:                prevRate,
		TokensPerShortPct:            in.PrevTokensPerShortPct,
		Samples:                      samples,
		LearningBaselineSequence:     baseline,
		LearningPendingTokens:        in.LearningPendingTokens,
		LearningEstimatedShortPct:    in.LearningEstimatedShortPct,
		SensitivityAnomalyStreak:     in.SensitivityAnomalyStreak,
		LastAnomalyTokensPerShortPct: in.LastAnomalyTokensPerShortPct,
	}
	if in.CurrShortRemaining == nil || in.CurrLongRemaining == nil || in.CurrShortResetAt.IsZero() {
		return out
	}

	currSample := QuotaSample{
		Sequence:           nextSampleSequence(samples),
		ObservedAt:         in.CurrObservedAt.UTC(),
		ShortWindowResetAt: in.CurrShortResetAt.UTC(),
		ShortWindowRem:     *in.CurrShortRemaining,
		LongWindowRem:      *in.CurrLongRemaining,
	}

	// 1. Cold start: record initial observation and anchor learning baseline.
	if len(samples) == 0 {
		out.Samples = []QuotaSample{currSample}
		out.LearningBaselineSequence = currSample.Sequence
		out.LearningPendingTokens = 0
		out.LearningEstimatedShortPct = 0
		out.SensitivityAnomalyStreak = 0
		out.LastAnomalyTokensPerShortPct = 0
		return out
	}

	lastSample := samples[len(samples)-1]

	// 2. Zero-consumption deduplication: preserve accumulated local usage while refreshing metadata.
	if currSample.ShortWindowRem == lastSample.ShortWindowRem &&
		currSample.LongWindowRem == lastSample.LongWindowRem {
		updated := append([]QuotaSample(nil), samples...)
		updated[len(updated)-1].ObservedAt = currSample.ObservedAt
		updated[len(updated)-1].ShortWindowResetAt = currSample.ShortWindowResetAt
		out.Samples = updated
		return out
	}

	// 3. Append new sample and enforce FIFO window capacity.
	updated := append([]QuotaSample(nil), samples...)
	updated = append(updated, currSample)
	if len(updated) > capacity {
		updated = updated[len(updated)-capacity:]
	}
	out.Samples = updated

	// 4. Short-window reset or replenishment starts a fresh learning span.
	if !currSample.ShortWindowResetAt.Equal(lastSample.ShortWindowResetAt) ||
		currSample.ShortWindowRem-lastSample.ShortWindowRem >= MinReplenishmentPoints {
		out.LearningBaselineSequence = currSample.Sequence
		out.LearningPendingTokens = 0
		out.LearningEstimatedShortPct = 0
		out.SensitivityAnomalyStreak = 0
		out.LastAnomalyTokensPerShortPct = 0
		return out
	}

	// 5. Resolve baseline sample for the active learning span.
	baseSample, found := sampleBySequence(updated, baseline)
	if !found {
		baseSample = updated[0]
		baseline = baseSample.Sequence
		out.LearningBaselineSequence = baseline
	}

	prevShortRem := lastSample.ShortWindowRem
	if in.PrevShortRemaining != nil && !in.PrevShortResetAt.IsZero() && in.PrevShortResetAt.Equal(currSample.ShortWindowResetAt) {
		prevShortRem = *in.PrevShortRemaining
	}
	deltaShort := prevShortRem - currSample.ShortWindowRem

	isColdStartK := in.PrevTokensPerShortPct <= 0
	prevK := in.PrevTokensPerShortPct
	if isColdStartK {
		prevK = defaultTokensPerShortPct(in.ModelGroup)
	}

	maxRatio := 2.5
	minAnomalyDrop := 4.0
	if isColdStartK {
		maxRatio = 4.0
		minAnomalyDrop = 6.0
	}

	// 6. Step-level EMA consistency check and TokensPerShortPct calibration.
	stepAnomalous := false
	reanchored := false
	if deltaShort > 0 {
		stepEstPct := in.StepEstimatedShortPct
		if stepEstPct <= 0 && in.StepPendingTokens > 0 {
			stepEstPct = in.StepPendingTokens / prevK
		}
		priorEstPct := max(0.0, in.LearningEstimatedShortPct-stepEstPct)
		alreadyObservedDrop := max(baseSample.ShortWindowRem-prevShortRem, 0)
		unexplainedPriorEstPct := max(0.0, priorEstPct-float64(alreadyObservedDrop))
		effectiveStepEstPct := stepEstPct + unexplainedPriorEstPct
		maxAllowedStepDrop := math.Max(maxRatio*effectiveStepEstPct+2.0, minAnomalyDrop)

		switch {
		case in.StepPendingTokens <= 0 && effectiveStepEstPct <= 0:
			if deltaShort >= MinExternalRebaseDelta {
				stepAnomalous = true
				out.SensitivityAnomalyStreak = 0
				out.LastAnomalyTokensPerShortPct = 0
			}
		case in.StepPendingTokens <= 0:
			if float64(deltaShort) > maxAllowedStepDrop {
				stepAnomalous = true
				out.SensitivityAnomalyStreak = 0
				out.LastAnomalyTokensPerShortPct = 0
			}
		default:
			exceedsEnvelope := float64(deltaShort) > maxAllowedStepDrop
			if deltaShort >= MinTokenCalibrationDelta {
				kObs := in.StepPendingTokens / float64(deltaShort)
				switch {
				case kObs < MinTokensPerShortPct || kObs > MaxTokensPerShortPct:
					stepAnomalous = true
					out.SensitivityAnomalyStreak = 0
					out.LastAnomalyTokensPerShortPct = 0
				case exceedsEnvelope:
					stepAnomalous = true
					if in.StepPendingTokens >= MinTokensPerShortPct {
						if in.SensitivityAnomalyStreak > 0 && in.LastAnomalyTokensPerShortPct > 0 &&
							kObs >= 0.5*in.LastAnomalyTokensPerShortPct && kObs <= 2.0*in.LastAnomalyTokensPerShortPct {
							out.SensitivityAnomalyStreak = in.SensitivityAnomalyStreak + 1
						} else {
							out.SensitivityAnomalyStreak = 1
						}
						out.LastAnomalyTokensPerShortPct = kObs
						if out.SensitivityAnomalyStreak >= MinAnomalyConfirmationStreak {
							out.TokensPerShortPct = clamp(kObs, MinTokensPerShortPct, MaxTokensPerShortPct)
							out.SensitivityAnomalyStreak = 0
							out.LastAnomalyTokensPerShortPct = 0
							reanchored = true
						}
					} else {
						out.SensitivityAnomalyStreak = 0
						out.LastAnomalyTokensPerShortPct = 0
					}
				default:
					out.SensitivityAnomalyStreak = 0
					out.LastAnomalyTokensPerShortPct = 0
					newK := EMASmoothingAlpha*kObs + (1.0-EMASmoothingAlpha)*prevK
					out.TokensPerShortPct = clamp(newK, MinTokensPerShortPct, MaxTokensPerShortPct)
				}
			} else if exceedsEnvelope {
				stepAnomalous = true
				out.SensitivityAnomalyStreak = 0
				out.LastAnomalyTokensPerShortPct = 0
			} else {
				out.SensitivityAnomalyStreak = 0
				out.LastAnomalyTokensPerShortPct = 0
			}
		}
	}

	if stepAnomalous {
		if !reanchored {
			updated[len(updated)-1].AnomalySkipped = true
		}
		out.LearningBaselineSequence = currSample.Sequence
		out.LearningPendingTokens = 0
		out.LearningEstimatedShortPct = 0
		return out
	}

	// 7. Span-level CycleBurnRate update when cumulative 5h burn reaches MinDeltaThreshold.
	delta5hPct := float64(baseSample.ShortWindowRem - currSample.ShortWindowRem)
	delta5h := delta5hPct / 100.0
	delta7d := float64(baseSample.LongWindowRem-currSample.LongWindowRem) / 100.0
	if delta5h < MinDeltaThreshold || delta7d <= 0 {
		return out
	}

	spanEstPct := in.LearningEstimatedShortPct
	if spanEstPct <= 0 && in.LearningPendingTokens > 0 {
		spanEstPct = in.LearningPendingTokens / prevK
	}
	maxAllowedSpanDrop := math.Max(maxRatio*spanEstPct+2.0, minAnomalyDrop)
	if in.LearningPendingTokens <= 0 || delta5hPct > maxAllowedSpanDrop {
		updated[len(updated)-1].AnomalySkipped = true
		out.LearningBaselineSequence = currSample.Sequence
		out.LearningPendingTokens = 0
		out.LearningEstimatedShortPct = 0
		return out
	}

	obs := delta7d / delta5h
	clamped := clamp(obs, MinCycleBurnRate, MaxCycleBurnRate)
	out.CycleBurnRate = EMASmoothingAlpha*clamped + (1.0-EMASmoothingAlpha)*prevRate
	out.LearningBaselineSequence = currSample.Sequence
	out.LearningPendingTokens = 0
	out.LearningEstimatedShortPct = 0
	return out
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

func sampleAppended(previous, current []QuotaSample) bool {
	if len(current) == 0 {
		return false
	}
	if len(previous) == 0 {
		return true
	}
	return current[len(current)-1].Sequence > previous[len(previous)-1].Sequence
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
