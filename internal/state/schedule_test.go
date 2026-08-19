package state_test

import (
	"context"
	"testing"
	"time"

	"antigravity-priority/internal/state"
)

func TestIsInScheduleWindow(t *testing.T) {
	tests := []struct {
		name   string
		now    time.Time
		cfg    state.ScheduleConfig
		expect bool
	}{
		{
			name:   "window disabled returns true",
			now:    time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: false, WindowStart: "09:00", WindowEnd: "23:00"},
			expect: true,
		},
		{
			name:   "empty start/end returns true",
			now:    time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true},
			expect: true,
		},
		{
			name:   "inside normal window",
			now:    time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expect: true,
		},
		{
			name:   "at window start boundary",
			now:    time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expect: true,
		},
		{
			name:   "at window end boundary excluded",
			now:    time.Date(2026, 8, 19, 23, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expect: false,
		},
		{
			name:   "before normal window",
			now:    time.Date(2026, 8, 19, 5, 30, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expect: false,
		},
		{
			name:   "cross-midnight window: inside evening",
			now:    time.Date(2026, 8, 19, 23, 30, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "22:00", WindowEnd: "06:00"},
			expect: true,
		},
		{
			name:   "cross-midnight window: inside morning",
			now:    time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "22:00", WindowEnd: "06:00"},
			expect: true,
		},
		{
			name:   "cross-midnight window: outside during day",
			now:    time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "22:00", WindowEnd: "06:00"},
			expect: false,
		},
		{
			name:   "full day 00:00-24:00",
			now:    time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "00:00", WindowEnd: "24:00"},
			expect: true,
		},
		{
			name:   "same start and end treated as full day",
			now:    time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "09:00"},
			expect: true,
		},
		{
			name:   "invalid format returns true (safe fallback)",
			now:    time.Date(2026, 8, 19, 14, 0, 0, 0, time.UTC),
			cfg:    state.ScheduleConfig{WindowEnabled: true, WindowStart: "bad", WindowEnd: "23:00"},
			expect: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := state.IsInScheduleWindow(tc.now, tc.cfg)
			if got != tc.expect {
				t.Errorf("IsInScheduleWindow() = %v, want %v", got, tc.expect)
			}
		})
	}
}

func TestNextWindowStart(t *testing.T) {
	tests := []struct {
		name      string
		now       time.Time
		cfg       state.ScheduleConfig
		expectMin int // expected minutes approximately
	}{
		{
			name:      "window disabled returns 0",
			now:       time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC),
			cfg:       state.ScheduleConfig{WindowEnabled: false, WindowStart: "09:00", WindowEnd: "23:00"},
			expectMin: 0,
		},
		{
			name:      "already inside window returns 0",
			now:       time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
			cfg:       state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expectMin: 0,
		},
		{
			name:      "before window start same day",
			now:       time.Date(2026, 8, 19, 7, 0, 0, 0, time.UTC),
			cfg:       state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expectMin: 120, // 2 hours
		},
		{
			name:      "after window end wraps to next day",
			now:       time.Date(2026, 8, 19, 23, 30, 0, 0, time.UTC),
			cfg:       state.ScheduleConfig{WindowEnabled: true, WindowStart: "09:00", WindowEnd: "23:00"},
			expectMin: 570, // 9.5 hours
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := state.NextWindowStart(tc.now, tc.cfg)
			gotMin := int(got.Minutes())
			if gotMin != tc.expectMin {
				t.Errorf("NextWindowStart() = %v (%d min), want ~%d min", got, gotMin, tc.expectMin)
			}
		})
	}
}

func TestValidateScheduleWindow(t *testing.T) {
	tests := []struct {
		name    string
		start   string
		end     string
		wantErr bool
	}{
		{"valid normal", "09:00", "23:00", false},
		{"valid cross-midnight", "22:00", "06:00", false},
		{"valid full day", "00:00", "24:00", false},
		{"empty values", "", "", false},
		{"invalid start", "25:00", "23:00", true},
		{"invalid end", "09:00", "abc", true},
		{"invalid 24:30", "24:30", "06:00", true},
		{"negative hour", "-1:00", "23:00", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := state.ValidateScheduleWindow(tc.start, tc.end)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateScheduleWindow(%q, %q) error = %v, wantErr %v", tc.start, tc.end, err, tc.wantErr)
			}
		})
	}
}

func TestScheduleConfig_Persistence(t *testing.T) {
	store, err := state.Load(context.Background(), t.TempDir()+"/test-schedule.json")
	if err != nil {
		t.Fatal(err)
	}

	// Default should be zero-value
	cfg := store.GetScheduleConfig()
	if cfg.Paused || cfg.WindowEnabled {
		t.Errorf("expected zero-value ScheduleConfig, got %+v", cfg)
	}

	// Set and read back
	store.SetScheduleConfig(state.ScheduleConfig{
		Paused:        true,
		WindowEnabled: true,
		WindowStart:   "09:00",
		WindowEnd:     "23:00",
	})

	cfg = store.GetScheduleConfig()
	if !cfg.Paused {
		t.Error("expected Paused=true")
	}
	if !cfg.WindowEnabled {
		t.Error("expected WindowEnabled=true")
	}
	if cfg.WindowStart != "09:00" {
		t.Errorf("expected WindowStart=09:00, got %q", cfg.WindowStart)
	}

	// Save and reload
	if err := store.SaveAtomic(context.Background()); err != nil {
		t.Fatal(err)
	}

	store2, err := state.Load(context.Background(), store.Path())
	if err != nil {
		t.Fatal(err)
	}
	cfg2 := store2.GetScheduleConfig()
	if !cfg2.Paused || !cfg2.WindowEnabled || cfg2.WindowStart != "09:00" || cfg2.WindowEnd != "23:00" {
		t.Errorf("persisted ScheduleConfig mismatch: %+v", cfg2)
	}
}
