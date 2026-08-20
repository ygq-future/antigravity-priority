package state

import (
	"time"

	"antigravity-priority/internal/config"
)

// ScheduleConfig holds dynamic schedule control state persisted across restarts.
type ScheduleConfig = config.ScheduleConfig

// IsInScheduleWindow reports whether the given time falls within the active
// schedule window defined by start and end in "HH:MM" format.
// Supports cross-midnight windows (e.g. "22:00" to "06:00").
// Returns true if the window is disabled, if start/end are empty, or if start == end (full day).
func IsInScheduleWindow(now time.Time, cfg ScheduleConfig) bool {
	return config.IsInScheduleWindow(now, cfg)
}

// NextWindowStart returns the next time the schedule window opens, relative to now.
// Returns zero duration if the window is not enabled or if already inside the window.
func NextWindowStart(now time.Time, cfg ScheduleConfig) time.Duration {
	return config.NextWindowStart(now, cfg)
}

// ValidateScheduleWindow validates window_start and window_end format.
func ValidateScheduleWindow(start, end string) error {
	return config.ValidateScheduleWindow(start, end)
}
