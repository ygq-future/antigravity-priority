package state_test

import (
	"testing"
	"time"

	"antigravity-priority/internal/state"
)

func int64Ptr(v int64) *int64 { return &v }

type learningState struct {
	rate     float64
	samples  []state.QuotaSample
	baseline uint64
}

func observe(s learningState, at, reset time.Time, short, long *int64, capacity int) learningState {
	s.rate, s.samples, s.baseline = state.UpdateSamplesAndCycleBurnRate(
		s.rate, s.samples, s.baseline, at, reset, short, long, capacity,
	)
	return s
}

func TestEstimatorColdStartAndInvalidObservationPreserveState(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset := now.Add(4 * time.Hour)
	s := observe(learningState{}, now, reset, int64Ptr(100), int64Ptr(100), 6)
	if s.rate != state.DefaultCycleBurnRate || len(s.samples) != 1 || s.baseline != 1 || s.samples[0].Sequence != 1 {
		t.Fatalf("unexpected cold-start state: %+v", s)
	}

	preserved := observe(learningState{rate: 0.25, samples: s.samples, baseline: s.baseline}, now.Add(time.Minute), reset, nil, int64Ptr(90), 6)
	if preserved.rate != 0.25 || len(preserved.samples) != 1 || preserved.baseline != 1 {
		t.Fatalf("invalid observation changed learning state: %+v", preserved)
	}
}

func TestEstimatorLearningAdvancesBaselineAndPreservesHistory(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset := now.Add(4 * time.Hour)
	s := learningState{rate: state.DefaultCycleBurnRate}
	s = observe(s, now, reset, int64Ptr(100), int64Ptr(100), 6)
	s = observe(s, now.Add(15*time.Minute), reset, int64Ptr(98), int64Ptr(100), 6)
	s = observe(s, now.Add(30*time.Minute), reset, int64Ptr(96), int64Ptr(99), 6)
	s = observe(s, now.Add(45*time.Minute), reset, int64Ptr(94), int64Ptr(98), 6)

	if s.rate < 0.195-1e-6 || s.rate > 0.195+1e-6 {
		t.Fatalf("expected learned rate 0.195, got %v", s.rate)
	}
	if len(s.samples) != 4 {
		t.Fatalf("learning removed sample history: len=%d", len(s.samples))
	}
	if s.baseline != s.samples[3].Sequence || s.samples[3].ShortWindowRem != 94 {
		t.Fatalf("baseline did not advance to current sample: baseline=%d samples=%+v", s.baseline, s.samples)
	}
}

func TestEstimatorUnchangedObservationOnlyRefreshesTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset := now.Add(4 * time.Hour)
	s := observe(learningState{rate: 0.18}, now, reset, int64Ptr(90), int64Ptr(95), 6)
	sequence := s.samples[0].Sequence
	s = observe(s, now.Add(15*time.Minute), reset, int64Ptr(90), int64Ptr(95), 6)

	if len(s.samples) != 1 || s.samples[0].Sequence != sequence || !s.samples[0].ObservedAt.Equal(now.Add(15*time.Minute)) {
		t.Fatalf("unchanged observation was not deduplicated: %+v", s.samples)
	}
	if s.rate != 0.18 || s.baseline != sequence {
		t.Fatalf("unchanged observation changed estimator state: %+v", s)
	}
}

func TestEstimatorWindowResetRebasesWithoutDeletingHistory(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset1 := now.Add(2 * time.Hour)
	reset2 := now.Add(7 * time.Hour)
	s := learningState{rate: 0.18}
	s = observe(s, now, reset1, int64Ptr(50), int64Ptr(80), 6)
	s = observe(s, now.Add(15*time.Minute), reset1, int64Ptr(48), int64Ptr(80), 6)
	s = observe(s, now.Add(2*time.Hour), reset2, int64Ptr(100), int64Ptr(80), 6)

	if len(s.samples) != 3 || s.samples[2].ShortWindowRem != 100 || !s.samples[2].ShortWindowResetAt.Equal(reset2) {
		t.Fatalf("window reset did not preserve history: %+v", s.samples)
	}
	if s.rate != 0.18 || s.baseline != s.samples[2].Sequence {
		t.Fatalf("window reset did not rebase estimator: %+v", s)
	}
}

func TestEstimatorChangedResetTimeIsADistinctObservation(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset1 := now.Add(2 * time.Hour)
	reset2 := now.Add(7 * time.Hour)
	s := observe(learningState{rate: 0.18}, now, reset1, int64Ptr(100), int64Ptr(80), 6)
	s = observe(s, now.Add(2*time.Hour), reset2, int64Ptr(100), int64Ptr(80), 6)

	if len(s.samples) != 2 || s.samples[0].Sequence == s.samples[1].Sequence {
		t.Fatalf("changed reset time was deduplicated: %+v", s.samples)
	}
	if s.baseline != s.samples[1].Sequence {
		t.Fatalf("changed reset time did not rebase learning: %+v", s)
	}
}

func TestEstimatorFIFOUsesOldestRetainedSampleWhenBaselineEvicted(t *testing.T) {
	now := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reset := now.Add(4 * time.Hour)
	s := learningState{rate: 0.15}
	for i, quota := range []int64{100, 99, 98, 94} {
		long := int64(100)
		if quota == 94 {
			long = 98
		}
		s = observe(s, now.Add(time.Duration(i)*10*time.Minute), reset, &quota, &long, 3)
	}

	if len(s.samples) != 3 || s.samples[0].ShortWindowRem != 99 || s.samples[2].ShortWindowRem != 94 {
		t.Fatalf("unexpected FIFO history: %+v", s.samples)
	}
	if s.baseline != s.samples[2].Sequence {
		t.Fatalf("expected successful learning to advance baseline, got %d", s.baseline)
	}
	if s.rate <= 0.15 {
		t.Fatalf("expected learning from oldest retained sample, got rate=%v", s.rate)
	}
}
