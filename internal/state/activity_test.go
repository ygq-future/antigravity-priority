package state_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"antigravity-priority/internal/state"
)

func TestStore_RequestActivity_WindowsPersistenceAndBackfill(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "activity-cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	base := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

	for i := range 6 {
		store.RecordRequestActivity(base.Add(-44*time.Minute + time.Duration(i)*time.Minute))
	}
	// Previous window [11:30, 11:45): 4 requests
	for i := range 4 {
		store.RecordRequestActivity(base.Add(-28*time.Minute + time.Duration(i)*2*time.Minute))
	}
	// Current window [11:45, 12:00]: 3 requests (2 inside burst window [11:55, 12:00])
	store.RecordRequestActivity(base.Add(-10 * time.Minute))
	store.RecordRequestActivity(base.Add(-3 * time.Minute))
	store.RecordRequestActivity(base.Add(-1 * time.Minute))

	summary := store.SummarizeRequestActivity(base, 15*time.Minute, 5*time.Minute, base.Add(-15*time.Minute))
	if summary.PrevWindowCount != 4 {
		t.Fatalf("PrevWindowCount = %d; want 4", summary.PrevWindowCount)
	}
	if summary.CurrWindowCount != 3 {
		t.Fatalf("CurrWindowCount = %d; want 3", summary.CurrWindowCount)
	}
	if summary.SinceLastCount != 3 {
		t.Fatalf("SinceLastCount = %d; want 3", summary.SinceLastCount)
	}
	if summary.BurstCount != 2 {
		t.Fatalf("BurstCount = %d; want 2", summary.BurstCount)
	}
	if len(summary.HistoricalWindowCounts) != 2 || summary.HistoricalWindowCounts[0] != 6 || summary.HistoricalWindowCounts[1] != 4 {
		t.Fatalf("HistoricalWindowCounts = %v; want [6 4]", summary.HistoricalWindowCounts)
	}

	// Persist runtime.json, then append newer requests only to requests.json to
	// simulate an unclean exit after SaveRequestsAtomic.
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("save atomic failed: %v", err)
	}
	store.AppendRequestHistory(state.RequestHistoryEntry{
		ID: "req-after-save-1",
		At: base.Add(30 * time.Second),
	}, 100)
	store.AppendRequestHistory(state.RequestHistoryEntry{
		ID: "req-after-save-2",
		At: base.Add(90 * time.Second),
	}, 100)
	if err := store.SaveRequestsAtomic(ctx); err != nil {
		t.Fatalf("save requests failed: %v", err)
	}

	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	afterReload := reloaded.SummarizeRequestActivity(base.Add(2*time.Minute), 15*time.Minute, 5*time.Minute, base.Add(-15*time.Minute))
	if afterReload.SinceLastCount != 5 {
		t.Fatalf("expected backfilled SinceLastCount = 5 after reload, got %d", afterReload.SinceLastCount)
	}
}

func TestStore_RequestActivity_BackfillSkipsWarmupAndHealthEntries(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "activity-warmup-cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	lastAuto := base.Add(-2 * time.Hour)
	store.SetLastAutoApplyAt(lastAuto)
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("save atomic failed: %v", err)
	}

	// Persist only warm-up and health-probe entries (both explicit Source and legacy ID prefix).
	store.SetRequestHistory([]state.RequestHistoryEntry{
		{ID: "health-auth1-2", Source: state.RequestHistorySourceHealth, At: base.Add(-2 * time.Minute)},
		{ID: "warmup-auth1-2", Source: state.RequestHistorySourceWarmup, At: base.Add(-5 * time.Minute)},
		{ID: "health-auth1-1", At: base.Add(-20 * time.Minute)},
		{ID: "warmup-auth1-1", At: base.Add(-45 * time.Minute)},
	})
	if err := store.SaveRequestsAtomic(ctx); err != nil {
		t.Fatalf("save requests failed: %v", err)
	}

	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	summary := reloaded.SummarizeRequestActivity(base, 15*time.Minute, 5*time.Minute, lastAuto)
	if summary.SinceLastCount != 0 || summary.CurrWindowCount != 0 || summary.PrevWindowCount != 0 || summary.BurstCount != 0 {
		t.Fatalf("expected zero request activity from warmup/health entries, got %+v", summary)
	}
	if !summary.LastRequestAt.IsZero() {
		t.Fatalf("expected zero LastRequestAt when only warmup/health entries exist, got %v", summary.LastRequestAt)
	}

	// Add one real inbound request alongside warm-up/health entries and verify only the real request is seeded.
	realAt := base.Add(-3 * time.Minute)
	reloaded.AppendRequestHistory(state.RequestHistoryEntry{
		ID: "req-real-1",
		At: realAt,
	}, 100)
	if err := reloaded.SaveRequestsAtomic(ctx); err != nil {
		t.Fatalf("save requests with real entry failed: %v", err)
	}

	reloadedAgain, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("second reload failed: %v", err)
	}
	withReal := reloadedAgain.SummarizeRequestActivity(base, 15*time.Minute, 5*time.Minute, lastAuto)
	if withReal.SinceLastCount != 1 || withReal.CurrWindowCount != 1 || withReal.BurstCount != 1 {
		t.Fatalf("expected exactly 1 real request seeded, got %+v", withReal)
	}
	if !withReal.LastRequestAt.Equal(realAt) {
		t.Fatalf("LastRequestAt = %v; want %v", withReal.LastRequestAt, realAt)
	}
}
