package runtime

import (
	"math"
	"math/rand/v2"
	"time"

	"antigravity-priority/internal/state"
)

// ScheduleMode classifies the adaptive auto-calibration cadence.
type ScheduleMode string

const (
	ScheduleModeIdle         ScheduleMode = "idle"
	ScheduleModeLowFrequency ScheduleMode = "low_frequency"
	ScheduleModeFrequent     ScheduleMode = "frequent"
	ScheduleModeBootstrap    ScheduleMode = "bootstrap"
)

const (
	frequentMinFraction = 0.75
	frequentMaxFraction = 0.98
	lowMinStretchFactor = 1.0
	lowMaxStretchFactor = 4.0
	lowMaxCapDuration   = 6 * time.Hour
	lowMinCapDuration   = 2 * time.Hour
	lowJitterFraction   = 0.15
	burstWakeMinDelay   = 15 * time.Second
	burstWakeMaxDelay   = 45 * time.Second
)

type adaptiveScheduleState struct {
	mode           ScheduleMode
	interval       time.Duration
	lastAutoApply  time.Time
	anchor         time.Time
	targetInterval time.Duration
	nextRunAt      time.Time
}

// AdaptiveScheduleSnapshot describes the current adaptive schedule evaluation
// for worker pacing and diagnostics reporting.
type AdaptiveScheduleSnapshot struct {
	Mode               ScheduleMode
	ShouldRunNow       bool
	NextRunAt          time.Time
	NextWait           time.Duration
	TargetInterval     time.Duration
	PrevWindowCount    int
	PrevWindowRateRPM  float64
	CurrWindowCount    int
	CurrWindowRateRPM  float64
	SinceLastCount     int
	BaselineRequests   float64
	ReferenceThreshold float64
	ActivityRatio      float64
	LastRequestAt      time.Time
}

type adaptiveEvalInput struct {
	now               time.Time
	interval          time.Duration
	lastAutoApplyAt   time.Time
	hasQuotaEvidence  bool
	summary           state.RequestActivitySummary
	latched           adaptiveScheduleState
	jitterRange       func(minDur, maxDur time.Duration) time.Duration
	forDirectAutoCall bool
}

func defaultJitterRange(minDur, maxDur time.Duration) time.Duration {
	if maxDur <= minDur {
		return minDur
	}
	span := maxDur - minDur
	return minDur + rand.N(span+1)
}

func computeActiveWindowBaseline(historicalCounts []int, prevWindowCount int) float64 {
	active := make([]int, 0, len(historicalCounts))
	for _, c := range historicalCounts {
		if c >= 2 {
			active = append(active, c)
		}
	}
	if len(active) == 0 {
		if prevWindowCount >= 2 {
			return float64(prevWindowCount)
		}
		return 0
	}
	const alpha = 0.25
	ema := float64(active[0])
	for i := 1; i < len(active); i++ {
		ema = alpha*float64(active[i]) + (1-alpha)*ema
	}
	return ema
}

func evaluateAdaptivePlan(in adaptiveEvalInput) (AdaptiveScheduleSnapshot, adaptiveScheduleState) {
	now := in.now.UTC()
	interval := in.interval
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if interval < time.Minute {
		interval = time.Minute
	}
	jitter := in.jitterRange
	if jitter == nil {
		jitter = defaultJitterRange
	}

	windowMinutes := interval.Minutes()
	cadenceFloor := math.Max(2.0, math.Sqrt(windowMinutes))
	baseline := computeActiveWindowBaseline(in.summary.HistoricalWindowCounts, in.summary.PrevWindowCount)

	refThreshold := cadenceFloor
	if baseline > 0 {
		refThreshold = math.Max(cadenceFloor, math.Min(3.0*cadenceFloor, 0.45*baseline))
	}

	burstWindow := min(5*time.Minute, max(time.Minute, interval/3))
	var burstProj float64
	if in.summary.BurstCount > 1 {
		conf := float64(in.summary.BurstCount-1) / float64(in.summary.BurstCount+1)
		burstProj = float64(in.summary.BurstCount) * (windowMinutes / burstWindow.Minutes()) * conf
	}

	snap := AdaptiveScheduleSnapshot{
		PrevWindowCount:    in.summary.PrevWindowCount,
		PrevWindowRateRPM:  float64(in.summary.PrevWindowCount) / windowMinutes,
		CurrWindowCount:    in.summary.CurrWindowCount,
		CurrWindowRateRPM:  float64(in.summary.CurrWindowCount) / windowMinutes,
		SinceLastCount:     in.summary.SinceLastCount,
		BaselineRequests:   baseline,
		ReferenceThreshold: refThreshold,
		LastRequestAt:      in.summary.LastRequestAt,
	}

	if in.summary.SinceLastCount == 0 {
		if in.lastAutoApplyAt.IsZero() && !in.hasQuotaEvidence {
			if !in.forDirectAutoCall && in.latched.mode == ScheduleModeBootstrap && in.latched.nextRunAt.After(now) {
				snap.Mode = ScheduleModeBootstrap
				snap.TargetInterval = interval
				snap.NextRunAt = in.latched.nextRunAt
				snap.NextWait = in.latched.nextRunAt.Sub(now)
				return snap, in.latched
			}
			snap.Mode = ScheduleModeBootstrap
			snap.TargetInterval = interval
			snap.NextRunAt = now
			snap.ShouldRunNow = true
			return snap, adaptiveScheduleState{
				mode:           ScheduleModeBootstrap,
				interval:       interval,
				targetInterval: interval,
				nextRunAt:      now,
			}
		}
		snap.Mode = ScheduleModeIdle
		return snap, adaptiveScheduleState{
			mode:     ScheduleModeIdle,
			interval: interval,
		}
	}

	// Idle-decay guard: if the current window has zero traffic and the total
	// uncalibrated requests since the last probe haven't reached the active
	// threshold, the trickle has gone silent — stay idle instead of probing.
	if in.summary.CurrWindowCount == 0 && float64(in.summary.SinceLastCount) < refThreshold {
		snap.Mode = ScheduleModeIdle
		return snap, adaptiveScheduleState{
			mode:     ScheduleModeIdle,
			interval: interval,
		}
	}

	momentum := 0.45*float64(in.summary.PrevWindowCount) + 0.55*float64(in.summary.CurrWindowCount)
	effectiveDemand := max(float64(in.summary.CurrWindowCount), momentum, burstProj)
	ratio := effectiveDemand / refThreshold
	snap.ActivityRatio = ratio

	last := in.lastAutoApplyAt.UTC()
	anchor := last
	if anchor.IsZero() {
		anchor = in.summary.FirstSinceLastAt.UTC()
		if anchor.IsZero() {
			anchor = now
		}
	}

	if ratio >= 1.0 {
		snap.Mode = ScheduleModeFrequent
		snap.TargetInterval = interval

		minCycle := max(min(30*time.Second, interval/2), time.Duration(float64(interval)*frequentMinFraction))
		maxCycle := max(min(interval, time.Duration(float64(interval)*frequentMaxFraction)), minCycle)

		// Direct AutoApply calls enforce the hard interval ceiling deterministically
		// once frequent traffic is established and >= interval has elapsed.
		if in.forDirectAutoCall && (last.IsZero() || !now.Before(last.Add(interval))) {
			snap.ShouldRunNow = true
			snap.NextRunAt = now
			return snap, in.latched
		}

		var nextRunAt time.Time
		reuseLatched := in.latched.mode == ScheduleModeFrequent &&
			in.latched.interval == interval &&
			in.latched.lastAutoApply.Equal(last) &&
			!in.latched.nextRunAt.IsZero()

		if reuseLatched {
			nextRunAt = in.latched.nextRunAt
			// Guard against backward clock jumps pushing the deadline past one interval.
			if nextRunAt.Sub(now) > interval {
				nextRunAt = now.Add(jitter(minCycle, maxCycle))
			}
		} else if last.IsZero() || !now.Before(last.Add(interval)) {
			// Transitioned from idle/low-frequency into frequent usage while cached
			// quota is stale: trigger soon with a short anti-fingerprint jitter.
			wakeMin := min(burstWakeMinDelay, interval/4)
			wakeMax := max(min(burstWakeMaxDelay, interval/2), wakeMin)
			nextRunAt = now.Add(jitter(wakeMin, wakeMax))
		} else {
			nextRunAt = last.Add(jitter(minCycle, maxCycle))
			if !nextRunAt.After(now) {
				ceiling := last.Add(interval)
				if ceiling.After(now) {
					nextRunAt = minTime(ceiling, now.Add(time.Second))
				} else {
					nextRunAt = now.Add(time.Second)
				}
			}
		}

		snap.NextRunAt = nextRunAt
		if !now.Before(nextRunAt) {
			snap.ShouldRunNow = true
			snap.NextWait = 0
		} else {
			snap.NextWait = nextRunAt.Sub(now)
		}

		nextState := adaptiveScheduleState{
			mode:           ScheduleModeFrequent,
			interval:       interval,
			lastAutoApply:  last,
			anchor:         anchor,
			targetInterval: interval,
			nextRunAt:      nextRunAt,
		}
		return snap, nextState
	}

	// Low-frequency / sparse traffic: stretch the calibration horizon and apply
	// jitter so sporadic usage avoids fixed-cadence signatures.
	snap.Mode = ScheduleModeLowFrequency
	stretch := min(lowMaxStretchFactor, max(lowMinStretchFactor, 1.0/max(ratio, 1.0/lowMaxStretchFactor)))
	maxStretched := min(max(time.Duration(lowMaxStretchFactor)*interval, lowMinCapDuration), lowMaxCapDuration)
	targetStretched := min(maxStretched, max(interval, time.Duration(float64(interval)*stretch)))
	snap.TargetInterval = targetStretched

	minStretched := max(interval, time.Duration(float64(targetStretched)*(1.0-lowJitterFraction)))
	maxJitterStretched := max(min(maxStretched, time.Duration(float64(targetStretched)*(1.0+lowJitterFraction))), minStretched)

	lowAnchor := anchor
	if !in.summary.FirstSinceLastAt.IsZero() && in.summary.FirstSinceLastAt.UTC().After(lowAnchor) {
		lowAnchor = in.summary.FirstSinceLastAt.UTC()
	}

	var nextRunAt time.Time
	reuseLatched := in.latched.mode == ScheduleModeLowFrequency &&
		in.latched.interval == interval &&
		in.latched.lastAutoApply.Equal(last) &&
		in.latched.anchor.Equal(lowAnchor) &&
		!in.latched.nextRunAt.IsZero()

	if reuseLatched {
		nextRunAt = in.latched.nextRunAt
		// If rising low-frequency traffic shortened the target significantly,
		// tighten the latched deadline without re-rolling upward.
		if targetStretched < time.Duration(float64(in.latched.targetInterval)*0.8) {
			tightened := lowAnchor.Add(jitter(minStretched, maxJitterStretched))
			if tightened.Before(nextRunAt) {
				nextRunAt = tightened
			}
		}
		if nextRunAt.Sub(now) > maxStretched {
			nextRunAt = now.Add(jitter(minStretched, maxJitterStretched))
		}
	} else {
		nextRunAt = lowAnchor.Add(jitter(minStretched, maxJitterStretched))
	}

	if in.forDirectAutoCall && !now.Before(lowAnchor.Add(targetStretched)) {
		snap.ShouldRunNow = true
		snap.NextRunAt = now
		return snap, in.latched
	}

	snap.NextRunAt = nextRunAt
	if !now.Before(nextRunAt) {
		snap.ShouldRunNow = true
		snap.NextWait = 0
	} else {
		snap.NextWait = nextRunAt.Sub(now)
	}

	nextState := adaptiveScheduleState{
		mode:           ScheduleModeLowFrequency,
		interval:       interval,
		lastAutoApply:  last,
		anchor:         lowAnchor,
		targetInterval: targetStretched,
		nextRunAt:      nextRunAt,
	}
	return snap, nextState
}

func minTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.Before(b) {
		return a
	}
	return b
}
