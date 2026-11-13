package state_test

import (
	"context"
	"testing"

	"antigravity-priority/internal/state"
)

func TestScheduleConfig_Persistence(t *testing.T) {
	cachePath := t.TempDir() + "/test-schedule.json"
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}

	// Default should be zero-value
	cfg := store.GetScheduleConfig()
	if cfg.Paused {
		t.Errorf("expected zero-value ScheduleConfig, got %+v", cfg)
	}

	// Set and read back
	store.SetScheduleConfig(state.ScheduleConfig{
		Paused: true,
	})

	cfg = store.GetScheduleConfig()
	if !cfg.Paused {
		t.Error("expected Paused=true")
	}

	// Save and reload
	if err := store.SaveAtomic(context.Background()); err != nil {
		t.Fatal(err)
	}

	store2, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	cfg2 := store2.GetScheduleConfig()
	if !cfg2.Paused {
		t.Errorf("persisted ScheduleConfig mismatch: %+v", cfg2)
	}
}
