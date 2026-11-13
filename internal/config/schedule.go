package config

// ScheduleConfig holds dynamic schedule control state persisted across restarts.
type ScheduleConfig struct {
	Paused bool `json:"paused"`
}
