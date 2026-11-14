package state

import (
	"math"
	"slices"
	"time"
)

const (
	// RequestActivityRetention bounds how long minute-level request activity buckets
	// are retained for adaptive scheduling baseline estimation.
	RequestActivityRetention = 7 * 24 * time.Hour

	recentRequestTimeRetention = 4 * time.Hour
	maxRecentRequestTimes      = 2048
	maxHistoricalWindows       = 256
)

// MinuteBucket stores aggregated Antigravity request volume for one UTC minute.
type MinuteBucket struct {
	Minute int64 `json:"minute"`
	Count  int   `json:"count"`
}

// RequestActivitySummary provides the windowed volume and historical baseline
// inputs consumed by the adaptive scheduler.
type RequestActivitySummary struct {
	PrevWindowCount        int
	CurrWindowCount        int
	SinceLastCount         int
	BurstCount             int
	FirstSinceLastAt       time.Time
	LastRequestAt          time.Time
	HistoricalWindowCounts []int
}

// RecordRequestActivity records one inbound Antigravity request timestamp for
// adaptive frequency-driven scheduling.
func (s *Store) RecordRequestActivity(at time.Time) {
	if at.IsZero() {
		return
	}
	utc := at.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordRequestActivityLocked(utc)
}

func (s *Store) recordRequestActivityLocked(utc time.Time) {
	minute := utc.Unix() / 60
	n := len(s.requestActivity)
	switch {
	case n == 0 || s.requestActivity[n-1].Minute < minute:
		s.requestActivity = append(s.requestActivity, MinuteBucket{Minute: minute, Count: 1})
	case s.requestActivity[n-1].Minute == minute:
		s.requestActivity[n-1].Count++
	default:
		idx, found := slices.BinarySearchFunc(s.requestActivity, minute, func(b MinuteBucket, target int64) int {
			if b.Minute < target {
				return -1
			}
			if b.Minute > target {
				return 1
			}
			return 0
		})
		if found {
			s.requestActivity[idx].Count++
		} else {
			s.requestActivity = slices.Insert(s.requestActivity, idx, MinuteBucket{Minute: minute, Count: 1})
		}
	}

	rn := len(s.recentRequestTimes)
	if rn == 0 || !utc.Before(s.recentRequestTimes[rn-1]) {
		s.recentRequestTimes = append(s.recentRequestTimes, utc)
	} else {
		idx, _ := slices.BinarySearchFunc(s.recentRequestTimes, utc, func(a, b time.Time) int {
			return a.Compare(b)
		})
		s.recentRequestTimes = slices.Insert(s.recentRequestTimes, idx, utc)
	}

	s.pruneRequestActivityLocked(utc)
}

func (s *Store) pruneRequestActivityLocked(now time.Time) {
	minMinute := now.Add(-RequestActivityRetention).Unix() / 60
	if len(s.requestActivity) > 0 && s.requestActivity[0].Minute < minMinute {
		cut := 0
		for cut < len(s.requestActivity) && s.requestActivity[cut].Minute < minMinute {
			cut++
		}
		if cut > 0 {
			s.requestActivity = append([]MinuteBucket(nil), s.requestActivity[cut:]...)
		}
	}

	cutoff := now.Add(-recentRequestTimeRetention)
	if len(s.recentRequestTimes) > 0 && s.recentRequestTimes[0].Before(cutoff) {
		cut := 0
		for cut < len(s.recentRequestTimes) && s.recentRequestTimes[cut].Before(cutoff) {
			cut++
		}
		cut = alignCutToMinuteBoundary(s.recentRequestTimes, cut)
		if cut > 0 {
			s.recentRequestTimes = append([]time.Time(nil), s.recentRequestTimes[cut:]...)
		}
	}
	if len(s.recentRequestTimes) > maxRecentRequestTimes {
		excess := len(s.recentRequestTimes) - maxRecentRequestTimes
		cut := alignCutToMinuteBoundary(s.recentRequestTimes, excess)
		if cut > 0 {
			s.recentRequestTimes = append([]time.Time(nil), s.recentRequestTimes[cut:]...)
		}
	}
}

func alignCutToMinuteBoundary(times []time.Time, cut int) int {
	if cut <= 0 || cut >= len(times) {
		return cut
	}
	cutMin := times[cut-1].Unix() / 60
	for cut < len(times) && times[cut].Unix()/60 == cutMin {
		cut++
	}
	return cut
}

func normalizeMinuteBuckets(raw []MinuteBucket) []MinuteBucket {
	if len(raw) == 0 {
		return nil
	}
	sorted := make([]MinuteBucket, 0, len(raw))
	for _, b := range raw {
		if b.Count <= 0 || b.Minute <= 0 {
			continue
		}
		sorted = append(sorted, b)
	}
	if len(sorted) == 0 {
		return nil
	}
	slices.SortFunc(sorted, func(a, b MinuteBucket) int {
		if a.Minute < b.Minute {
			return -1
		}
		if a.Minute > b.Minute {
			return 1
		}
		return 0
	})
	merged := make([]MinuteBucket, 0, len(sorted))
	for _, b := range sorted {
		if len(merged) > 0 && merged[len(merged)-1].Minute == b.Minute {
			merged[len(merged)-1].Count += b.Count
			continue
		}
		merged = append(merged, b)
	}
	return merged
}

func normalizeRequestTimes(raw []time.Time) []time.Time {
	if len(raw) == 0 {
		return nil
	}
	out := make([]time.Time, 0, len(raw))
	for _, t := range raw {
		if t.IsZero() {
			continue
		}
		out = append(out, t.UTC())
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, func(a, b time.Time) int {
		return a.Compare(b)
	})
	if len(out) > maxRecentRequestTimes {
		out = append([]time.Time(nil), out[len(out)-maxRecentRequestTimes:]...)
	}
	return out
}

// seedActivityFromRequestHistory backfills any request timestamps from
// requests.json that are newer than the latest persisted activity watermark
// (e.g. after an upgrade or unclean process termination).
func (s *Store) seedActivityFromRequestHistory() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requestHistory) == 0 {
		return
	}
	var lastBucketMinute int64
	if len(s.requestActivity) > 0 {
		lastBucketMinute = s.requestActivity[len(s.requestActivity)-1].Minute
	}
	var lastExactAt time.Time
	if len(s.recentRequestTimes) > 0 {
		lastExactAt = s.recentRequestTimes[len(s.recentRequestTimes)-1]
	}

	// requestHistory is newest-first; iterate oldest-first for clean append.
	for _, entry := range slices.Backward(s.requestHistory) {
		if entry.IsBackgroundSynthetic() {
			continue
		}
		at := entry.At
		if at.IsZero() {
			continue
		}
		utc := at.UTC()
		minute := utc.Unix() / 60
		hasSavedExact := len(s.recentRequestTimes) > 0
		needBucket := len(s.requestActivity) == 0 || minute > lastBucketMinute || (hasSavedExact && minute == lastBucketMinute && utc.After(lastExactAt))
		needExact := !hasSavedExact || utc.After(lastExactAt)
		if !needBucket && !needExact {
			continue
		}
		if needBucket && needExact {
			s.recordRequestActivityLocked(utc)
			continue
		}
		if needBucket {
			n := len(s.requestActivity)
			if n > 0 && s.requestActivity[n-1].Minute == minute {
				s.requestActivity[n-1].Count++
			} else {
				s.requestActivity = append(s.requestActivity, MinuteBucket{Minute: minute, Count: 1})
			}
		}
		if needExact {
			s.recentRequestTimes = append(s.recentRequestTimes, utc)
		}
	}
	if len(s.recentRequestTimes) > 0 {
		s.recentRequestTimes = normalizeRequestTimes(s.recentRequestTimes)
		latest := s.recentRequestTimes[len(s.recentRequestTimes)-1]
		s.pruneRequestActivityLocked(latest)
	} else if len(s.requestActivity) > 0 {
		latest := time.Unix(s.requestActivity[len(s.requestActivity)-1].Minute*60, 0).UTC()
		s.pruneRequestActivityLocked(latest)
	}
}

// HasQuotaEvidence reports whether the store holds at least one persisted quota entry.
func (s *Store) HasQuotaEvidence() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries) > 0
}

// SummarizeRequestActivity computes windowed counts and historical active-window
// counts relative to now.
func (s *Store) SummarizeRequestActivity(now time.Time, window, burstWindow time.Duration, lastAutoApplyAt time.Time) RequestActivitySummary {
	if window <= 0 {
		window = 15 * time.Minute
	}
	if burstWindow <= 0 {
		burstWindow = min(5*time.Minute, max(time.Minute, window/3))
	}
	utcNow := now.UTC()
	utcLast := utcOrZero(lastAutoApplyAt)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var summary RequestActivitySummary
	if len(s.recentRequestTimes) > 0 {
		summary.LastRequestAt = s.recentRequestTimes[len(s.recentRequestTimes)-1]
	} else if len(s.requestActivity) > 0 {
		summary.LastRequestAt = time.Unix(s.requestActivity[len(s.requestActivity)-1].Minute*60, 0).UTC()
	}

	currStart := utcNow.Add(-window)
	prevStart := utcNow.Add(-2 * window)
	burstStart := utcNow.Add(-burstWindow)

	summary.PrevWindowCount = s.countRequestsRangeLocked(prevStart, currStart, true, false)
	summary.CurrWindowCount = s.countRequestsRangeLocked(currStart, utcNow, true, true)
	summary.BurstCount = s.countRequestsRangeLocked(burstStart, utcNow, true, true)

	if utcLast.IsZero() {
		earliest := utcNow.Add(-RequestActivityRetention)
		summary.SinceLastCount = s.countRequestsRangeLocked(earliest, utcNow, true, true)
		summary.FirstSinceLastAt = s.firstRequestAfterLocked(time.Time{}, utcNow)
	} else if utcNow.After(utcLast) {
		summary.SinceLastCount = s.countRequestsRangeLocked(utcLast, utcNow, false, true)
		summary.FirstSinceLastAt = s.firstRequestAfterLocked(utcLast, utcNow)
	}

	// Collect completed non-zero windows prior to currStart, oldest-to-newest.
	oldestLimit := utcNow.Add(-RequestActivityRetention)
	if len(s.requestActivity) > 0 {
		bucketStart := time.Unix(s.requestActivity[0].Minute*60, 0).UTC()
		if bucketStart.After(oldestLimit) {
			oldestLimit = bucketStart
		}
	}
	var countsDesc []int
	winEnd := currStart
	for i := 0; i < maxHistoricalWindows && winEnd.After(oldestLimit); i++ {
		winStart := winEnd.Add(-window)
		c := s.countRequestsRangeLocked(winStart, winEnd, true, false)
		if c > 0 {
			countsDesc = append(countsDesc, c)
		}
		winEnd = winStart
	}
	if len(countsDesc) > 0 {
		slices.Reverse(countsDesc)
		summary.HistoricalWindowCounts = countsDesc
	}
	return summary
}

func (s *Store) firstRequestAfterLocked(after, upTo time.Time) time.Time {
	upToMin := upTo.Unix() / 60
	afterMin := int64(-1)
	if !after.IsZero() {
		afterMin = after.Unix() / 60
	}
	checkBucketsFirst := len(s.recentRequestTimes) == 0 || after.IsZero() || after.Before(s.recentRequestTimes[0])
	if checkBucketsFirst {
		exactStartMin := int64(math.MaxInt64)
		if len(s.recentRequestTimes) > 0 {
			exactStartMin = s.recentRequestTimes[0].Unix() / 60
		}
		for _, b := range s.requestActivity {
			if b.Minute > upToMin || b.Minute >= exactStartMin {
				break
			}
			if b.Count > 0 && b.Minute > afterMin {
				return time.Unix(b.Minute*60, 0).UTC()
			}
		}
	}
	for _, t := range s.recentRequestTimes {
		if t.After(upTo) {
			break
		}
		if after.IsZero() || t.After(after) {
			return t
		}
	}
	if !checkBucketsFirst {
		for _, b := range s.requestActivity {
			if b.Minute > upToMin {
				break
			}
			if b.Count > 0 && b.Minute > afterMin {
				return time.Unix(b.Minute*60, 0).UTC()
			}
		}
	}
	return time.Time{}
}

func (s *Store) countRequestsRangeLocked(start, end time.Time, startInclusive, endInclusive bool) int {
	if end.Before(start) || (end.Equal(start) && (!startInclusive || !endInclusive)) {
		return 0
	}
	if len(s.recentRequestTimes) > 0 {
		earliestExact := s.recentRequestTimes[0]
		if !start.Before(earliestExact) {
			return countTimesInRange(s.recentRequestTimes, start, end, startInclusive, endInclusive)
		}
	}
	// Fall back to minute buckets for ranges extending past exact recent times,
	// refining the recent tail with exact timestamps when available.
	if len(s.recentRequestTimes) == 0 {
		return countBucketsInRange(s.requestActivity, start, end, startInclusive, endInclusive)
	}
	split := s.recentRequestTimes[0]
	splitMinStart := time.Unix((split.Unix()/60)*60, 0).UTC()
	if splitMinStart.After(start) && !splitMinStart.After(end) {
		headCount := countBucketsInRange(s.requestActivity, start, splitMinStart, startInclusive, false)
		tailCount := countTimesInRange(s.recentRequestTimes, splitMinStart, end, true, endInclusive)
		return headCount + tailCount
	}
	return countBucketsInRange(s.requestActivity, start, end, startInclusive, endInclusive)
}

func countTimesInRange(times []time.Time, start, end time.Time, startInclusive, endInclusive bool) int {
	startIdx, _ := slices.BinarySearchFunc(times, start, func(t, target time.Time) int {
		return t.Compare(target)
	})
	if !startInclusive {
		for startIdx < len(times) && !times[startIdx].After(start) {
			startIdx++
		}
	}
	count := 0
	for i := startIdx; i < len(times); i++ {
		t := times[i]
		if endInclusive {
			if t.After(end) {
				break
			}
		} else {
			if !t.Before(end) {
				break
			}
		}
		count++
	}
	return count
}

func countBucketsInRange(buckets []MinuteBucket, start, end time.Time, startInclusive, endInclusive bool) int {
	if len(buckets) == 0 {
		return 0
	}
	startMin := start.Unix() / 60
	if !startInclusive && start.Unix()%60 == 59 {
		startMin++
	}
	endMin := end.Unix() / 60
	if !endInclusive && end.Unix()%60 == 0 {
		endMin--
	}
	if endMin < startMin {
		return 0
	}
	idx, _ := slices.BinarySearchFunc(buckets, startMin, func(b MinuteBucket, target int64) int {
		if b.Minute < target {
			return -1
		}
		if b.Minute > target {
			return 1
		}
		return 0
	})
	total := 0
	for i := idx; i < len(buckets); i++ {
		if buckets[i].Minute > endMin {
			break
		}
		total += buckets[i].Count
	}
	return total
}
