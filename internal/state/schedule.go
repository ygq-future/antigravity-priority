package state

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// IsInScheduleWindow reports whether the given time falls within the active
// schedule window defined by start and end in "HH:MM" format.
// Supports cross-midnight windows (e.g. "22:00" to "06:00").
// Returns true if the window is disabled, if start/end are empty, or if start == end (full day).
func IsInScheduleWindow(now time.Time, cfg ScheduleConfig) bool {
	if !cfg.WindowEnabled {
		return true
	}
	start := strings.TrimSpace(cfg.WindowStart)
	end := strings.TrimSpace(cfg.WindowEnd)
	if start == "" || end == "" {
		return true
	}
	startMin, err := parseHHMM(start)
	if err != nil {
		return true
	}
	endMin, err := parseHHMM(end)
	if err != nil {
		return true
	}
	// Full day: 00:00-24:00 or same start and end
	if startMin == endMin {
		return true
	}
	if startMin == 0 && endMin == 24*60 {
		return true
	}

	nowMin := now.Hour()*60 + now.Minute()

	if startMin < endMin {
		// Normal window: e.g. 09:00-23:00
		return nowMin >= startMin && nowMin < endMin
	}
	// Cross-midnight window: e.g. 22:00-06:00
	return nowMin >= startMin || nowMin < endMin
}

// NextWindowStart returns the next time the schedule window opens, relative to now.
// Returns zero duration if the window is not enabled or if already inside the window.
func NextWindowStart(now time.Time, cfg ScheduleConfig) time.Duration {
	if !cfg.WindowEnabled {
		return 0
	}
	if IsInScheduleWindow(now, cfg) {
		return 0
	}
	start := strings.TrimSpace(cfg.WindowStart)
	if start == "" {
		return 0
	}
	startMin, err := parseHHMM(start)
	if err != nil {
		return 0
	}
	nowMin := now.Hour()*60 + now.Minute()
	nowSec := now.Second()

	var deltaMin int
	if startMin > nowMin {
		deltaMin = startMin - nowMin
	} else {
		// Next day
		deltaMin = (24*60 - nowMin) + startMin
	}

	return time.Duration(deltaMin)*time.Minute - time.Duration(nowSec)*time.Second
}

// parseHHMM parses a "HH:MM" string into minutes since midnight.
// Accepts "24:00" as 1440 (end-of-day sentinel).
func parseHHMM(s string) (int, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid time format %q: expected HH:MM", s)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || h < 0 || h > 24 {
		return 0, fmt.Errorf("invalid hour in %q", s)
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute in %q", s)
	}
	if h == 24 && m != 0 {
		return 0, fmt.Errorf("invalid time %q: 24:XX only allows 24:00", s)
	}
	return h*60 + m, nil
}

// ValidateScheduleWindow validates window_start and window_end format.
func ValidateScheduleWindow(start, end string) error {
	if start != "" {
		if _, err := parseHHMM(start); err != nil {
			return fmt.Errorf("window_start: %w", err)
		}
	}
	if end != "" {
		if _, err := parseHHMM(end); err != nil {
			return fmt.Errorf("window_end: %w", err)
		}
	}
	return nil
}
