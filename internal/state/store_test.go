package state_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/state"
)

func TestStore_Load_NotExist(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "non_existent_cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("expected no error loading non-existent cache, got %v", err)
	}
	if store == nil {
		t.Fatalf("expected non-nil store")
	}

	rate := store.GetCycleBurnRate("auth-1", "gemini")
	if rate != state.DefaultCycleBurnRate {
		t.Fatalf("expected default rate %v, got %v", state.DefaultCycleBurnRate, rate)
	}

	if _, ok := store.GetEntry("auth-1", "gemini"); ok {
		t.Fatalf("expected store to not have entry")
	}
}

func TestStore_Load_EmptyFile(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "empty_cache.json")

	if err := os.WriteFile(cachePath, []byte("   \n\t  "), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("expected no error loading empty whitespace cache, got %v", err)
	}
	if len(store.Entries()) != 0 {
		t.Fatalf("expected 0 entries in empty store, got %d", len(store.Entries()))
	}
}

func TestStore_Load_Corrupt(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "corrupt_cache.json")

	if err := os.WriteFile(cachePath, []byte("invalid-json{"), 0o600); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	_, err := state.Load(ctx, cachePath)
	if err == nil {
		t.Fatalf("expected error for corrupt cache, got nil")
	}
	if !errors.Is(err, state.ErrCorruptCache) {
		t.Fatalf("expected ErrCorruptCache, got %v", err)
	}
}

func TestStoreLoadNormalizesLegacySampleLearningState(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "legacy-cache.json")
	raw := `{
  "schema_version": 1,
  "entries": {
    "auth-legacy|model_group=gemini": {
      "schema_version": 1,
      "auth_index": "auth-legacy",
      "model_group": "gemini",
      "cycle_burn_rate": 0.15,
      "samples": [
        {"observed_at":"2026-08-22T01:00:00Z","short_window_reset_at":"2026-08-22T05:00:00Z","short_window_rem":100,"long_window_rem":100},
        {"observed_at":"2026-08-22T02:00:00Z","short_window_reset_at":"2026-08-22T05:00:00Z","short_window_rem":98,"long_window_rem":99}
      ]
    }
  }
}`
	if err := os.WriteFile(cachePath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write legacy cache: %v", err)
	}

	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatalf("load legacy cache: %v", err)
	}
	entry, ok := store.GetEntry("auth-legacy", "gemini")
	if !ok {
		t.Fatal("legacy entry missing after load")
	}
	if len(entry.Samples) != 2 || entry.Samples[0].Sequence != 1 || entry.Samples[1].Sequence != 2 {
		t.Fatalf("legacy sample sequences not normalized: %+v", entry.Samples)
	}
	if entry.LearningBaselineSequence != 1 {
		t.Fatalf("legacy baseline = %d, want oldest sequence 1", entry.LearningBaselineSequence)
	}
}

func TestStoreLoadCollapsesAdjacentDuplicateQuotaSamples(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "duplicate-samples-cache.json")
	raw := `{
  "schema_version": 1,
  "entries": {
    "auth-duplicate|model_group=claude_gpt": {
      "schema_version": 1,
      "auth_index": "auth-duplicate",
      "model_group": "claude_gpt",
      "cycle_burn_rate": 0.15,
      "samples": [
        {"sequence":1,"observed_at":"2026-08-23T02:00:00Z","short_window_reset_at":"2026-08-23T07:00:00Z","short_window_rem":100,"long_window_rem":45},
        {"sequence":2,"observed_at":"2026-08-23T02:15:00Z","short_window_reset_at":"2026-08-23T07:15:00Z","short_window_rem":100,"long_window_rem":45},
        {"sequence":3,"observed_at":"2026-08-23T02:30:00Z","short_window_reset_at":"2026-08-23T07:30:00Z","short_window_rem":98,"long_window_rem":44}
      ],
      "learning_baseline_sequence": 2
    }
  }
}`
	if err := os.WriteFile(cachePath, []byte(raw), 0o600); err != nil {
		t.Fatalf("write duplicate cache: %v", err)
	}

	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatalf("load duplicate cache: %v", err)
	}
	entry, ok := store.GetEntry("auth-duplicate", "claude_gpt")
	if !ok {
		t.Fatal("duplicate entry missing after load")
	}
	if len(entry.Samples) != 2 || entry.Samples[0].Sequence != 1 || entry.Samples[1].Sequence != 3 {
		t.Fatalf("adjacent duplicate quotas were not collapsed: %+v", entry.Samples)
	}
	if entry.Samples[0].ObservedAt.Format(time.RFC3339) != "2026-08-23T02:15:00Z" || entry.Samples[0].ShortWindowResetAt.Format(time.RFC3339) != "2026-08-23T07:15:00Z" {
		t.Fatalf("latest duplicate metadata was not retained: %+v", entry.Samples[0])
	}
	if entry.LearningBaselineSequence != entry.Samples[0].Sequence {
		t.Fatalf("baseline did not follow collapsed duplicate: %d", entry.LearningBaselineSequence)
	}
}

func TestStore_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cancel_cache.json")

	_, err := state.Load(ctx, cachePath)
	if err == nil {
		t.Fatalf("expected error on canceled context in Load, got nil")
	}

	store, _ := state.Load(context.Background(), cachePath)
	if err := store.SaveAtomic(ctx); err == nil {
		t.Fatalf("expected error on canceled context in SaveAtomic, got nil")
	}
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{}); err == nil {
		t.Fatalf("expected error on canceled context in MarkProbeSuccess, got nil")
	}
	if err := store.MarkProbeFailure(ctx, state.ProbeFailure{}); err == nil {
		t.Fatalf("expected error on canceled context in MarkProbeFailure, got nil")
	}
}

func TestStore_SaveAtomic_And_Reload(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "sub", "refresh-cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	shortReset := now.Add(3 * time.Hour)
	longReset := now.Add(72 * time.Hour)

	// Step 1: Probe success with 100% quota (cold start)
	err = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "account-1",
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: int64Ptr(100),
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  int64Ptr(100),
		PlanType:             core.PlanTypePro,
	})
	if err != nil {
		t.Fatalf("mark probe success failed: %v", err)
	}

	entry, ok := store.GetEntry("account-1", "gemini")
	if !ok {
		t.Fatalf("expected entry to exist")
	}
	if entry.CycleBurnRate != state.DefaultCycleBurnRate {
		t.Fatalf("expected default cycle burn rate %v, got %v", state.DefaultCycleBurnRate, entry.CycleBurnRate)
	}
	if entry.PlanType != core.PlanTypePro {
		t.Fatalf("expected plan type pro, got %v", entry.PlanType)
	}

	// Step 2: Next probe with consumption
	now2 := now.Add(30 * time.Minute)
	err = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "account-1",
		ModelGroup:           "gemini",
		ObservedAt:           now2,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: int64Ptr(80), // -20%
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  int64Ptr(96), // -4%
		PlanType:             core.PlanTypePro,
	})
	if err != nil {
		t.Fatalf("2nd mark probe success failed: %v", err)
	}

	entry2, _ := store.GetEntry("account-1", "gemini")
	expectedRate := 0.165
	if entry2.CycleBurnRate < expectedRate-1e-6 || entry2.CycleBurnRate > expectedRate+1e-6 {
		t.Fatalf("expected rate %v, got %v", expectedRate, entry2.CycleBurnRate)
	}

	// Save to disk
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("save atomic failed: %v", err)
	}

	// Reload from disk
	storeReloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reloading cache failed: %v", err)
	}

	entryReloaded, ok := storeReloaded.GetEntry("account-1", "gemini")
	if !ok {
		t.Fatalf("expected reloaded entry to exist")
	}
	if entryReloaded.CycleBurnRate < expectedRate-1e-6 || entryReloaded.CycleBurnRate > expectedRate+1e-6 {
		t.Fatalf("expected reloaded rate %v, got %v", expectedRate, entryReloaded.CycleBurnRate)
	}
	if *entryReloaded.ShortWindowRemaining != 80 || *entryReloaded.LongWindowRemaining != 96 {
		t.Fatalf("unexpected window values: short=%v, long=%v", *entryReloaded.ShortWindowRemaining, *entryReloaded.LongWindowRemaining)
	}
	if len(entryReloaded.Samples) == 0 {
		t.Fatalf("expected reloaded samples to not be empty")
	}
	samples := storeReloaded.GetSamples("account-1", "gemini")
	if len(samples) == 0 {
		t.Fatalf("expected GetSamples to return non-empty slice")
	}

	allEntries := storeReloaded.Entries()
	if len(allEntries) != 1 {
		t.Fatalf("expected 1 entry in snapshot, got %d", len(allEntries))
	}
}

func TestStore_LastAutoApplyAt_PersistsAcrossReload(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "scheduler-cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if got := store.GetLastAutoApplyAt(); !got.IsZero() {
		t.Fatalf("expected zero last auto apply on empty store, got %v", got)
	}

	zone := time.FixedZone("UTC+8", 8*60*60)
	appliedAt := time.Date(2026, 8, 18, 20, 30, 0, 0, zone)
	store.SetLastAutoApplyAt(appliedAt)
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("save atomic failed: %v", err)
	}

	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if got := reloaded.GetLastAutoApplyAt(); !got.Equal(appliedAt) {
		t.Fatalf("expected reloaded last auto apply %v, got %v", appliedAt, got)
	}
	if got := reloaded.GetLastAutoApplyAt(); got.Location() != time.UTC {
		t.Fatalf("expected reloaded last auto apply in UTC, got %v", got.Location())
	}
}

func TestStore_SaveAtomic_DynamicTempFileSafety(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// Simulate a colliding static temp file left behind by a previous crash.
	staticTmp := filepath.Join(tmpDir, "cache.json.tmp")
	if err := os.WriteFile(staticTmp, []byte("stale collision content"), 0o444); err != nil {
		t.Fatalf("failed to create static tmp collision file: %v", err)
	}

	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "acc-1",
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ResetAt:              now.Add(time.Hour),
		ShortWindowResetAt:   now.Add(time.Hour),
		ShortWindowRemaining: int64Ptr(90),
		LongWindowResetAt:    now.Add(24 * time.Hour),
		LongWindowRemaining:  int64Ptr(95),
	}); err != nil {
		t.Fatalf("mark probe success: %v", err)
	}

	// SaveAtomic must succeed despite the static .tmp collision
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("SaveAtomic failed with collision file present: %v", err)
	}

	// Static file should remain untouched
	staticContent, err := os.ReadFile(staticTmp)
	if err != nil || string(staticContent) != "stale collision content" {
		t.Fatalf("static collision file was unexpectedly overwritten: %v", err)
	}

	// Cache file should be valid and reloadable
	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reloading cache failed: %v", err)
	}
	if _, ok := reloaded.GetEntry("acc-1", "gemini"); !ok {
		t.Fatal("expected entry to be persisted in reloaded cache")
	}

	// Check that no lingering dynamic .tmp files exist
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "cache.json.") && strings.HasSuffix(f.Name(), ".tmp") && f.Name() != "cache.json.tmp" {
			t.Fatalf("unexpected lingering temp file found: %s", f.Name())
		}
	}
}

func TestStore_SaveAtomic_ConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "concurrent_cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	const writers = 8
	var wg sync.WaitGroup
	errCh := make(chan error, writers)

	for i := range writers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			now := time.Date(2026, 8, 18, 12, idx, 0, 0, time.UTC)
			_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
				AuthIndex:            fmt.Sprintf("writer-%d", idx),
				ModelGroup:           "gemini",
				ObservedAt:           now,
				ResetAt:              now.Add(time.Hour),
				ShortWindowResetAt:   now.Add(time.Hour),
				ShortWindowRemaining: new(int64(idx)),
			})
			if err := store.SaveAtomic(ctx); err != nil {
				errCh <- fmt.Errorf("writer %d failed: %w", idx, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for writeErr := range errCh {
		t.Fatalf("concurrent save atomic error: %v", writeErr)
	}

	// Verify reloaded document is healthy
	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("failed to reload after concurrent writes: %v", err)
	}
	if len(reloaded.Entries()) == 0 {
		t.Fatal("expected persisted entries after concurrent writes")
	}
}

func TestStore_ProbeRoundMarksOnlyAppendedSamples(t *testing.T) {
	store, err := state.Load(context.Background(), filepath.Join(t.TempDir(), "round-cache.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	reset5h := now.Add(5 * time.Hour)
	reset7d := now.Add(7 * 24 * time.Hour)
	mark := func(at time.Time, short, long int64, round string) {
		t.Helper()
		err := store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
			AuthIndex:            "round-auth",
			Provider:             core.ProviderAntigravity,
			ModelGroup:           "gemini",
			ObservedAt:           at,
			ResetAt:              reset7d,
			ShortWindowResetAt:   reset5h,
			ShortWindowRemaining: &short,
			LongWindowResetAt:    reset7d,
			LongWindowRemaining:  &long,
			ProbeRoundID:         round,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	mark(now, 100, 100, "round-1")
	mark(now.Add(15*time.Minute), 100, 100, "round-2")
	if records := store.GetSamplesByProbeRound("round-2", "gemini"); len(records) != 0 {
		t.Fatalf("unchanged probe created round-2 samples: %+v", records)
	}
	mark(now.Add(30*time.Minute), 95, 99, "round-3")
	newerShort, newerLong := int64(90), int64(98)
	if err := store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
		AuthIndex:            "round-auth-newer",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now.Add(45 * time.Minute),
		ResetAt:              reset7d,
		ShortWindowResetAt:   reset5h,
		ShortWindowRemaining: &newerShort,
		LongWindowResetAt:    reset7d,
		LongWindowRemaining:  &newerLong,
		ProbeRoundID:         "round-3",
	}); err != nil {
		t.Fatal(err)
	}
	records := store.GetSamplesByProbeRound("round-3", "gemini")
	if len(records) != 2 || records[0].Sample.ProbeRoundID != "round-3" {
		t.Fatalf("appended probe sample not attributable to round-3: %+v", records)
	}
	if records[0].AuthIndex != "round-auth-newer" {
		t.Fatalf("probe samples are not newest-first: %+v", records)
	}
	if samples := store.GetSamples("round-auth", "gemini"); len(samples) != 2 {
		t.Fatalf("expected one cold-start and one changed sample, got %d", len(samples))
	}
}

func TestStore_MarkProbeFailure_Sanitized(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "refresh-cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	t.Run("sensitive token redaction", func(t *testing.T) {
		err = store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:  "acc-fail",
			ModelGroup: "claude_gpt",
			ObservedAt: now,
			Err:        errors.New("request failed with Bearer secret-token-12345: upstream 500"),
		})
		if err != nil {
			t.Fatalf("mark probe failure failed: %v", err)
		}

		entry, ok := store.GetEntry("acc-fail", "claude_gpt")
		if !ok {
			t.Fatalf("expected failure entry to exist")
		}
		if entry.LastError != "probe failed: sensitive upstream error redacted" {
			t.Fatalf("expected redacted error message, got %q", entry.LastError)
		}
	})

	t.Run("normal error truncation", func(t *testing.T) {
		longErr := strings.Repeat("network timeout connecting to upstream endpoint; ", 10)
		err = store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:  "acc-long-fail",
			ModelGroup: "",
			ObservedAt: now,
			Err:        errors.New(longErr),
		})
		if err != nil {
			t.Fatalf("mark probe failure failed: %v", err)
		}

		entry, ok := store.GetEntry("acc-long-fail", "")
		if !ok {
			t.Fatalf("expected entry with empty model group to exist")
		}
		if len(entry.LastError) > 240 {
			t.Fatalf("expected error length <= 240, got %d", len(entry.LastError))
		}
	})

	t.Run("nil error", func(t *testing.T) {
		err = store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:  "acc-nil-err",
			ObservedAt: now,
			Err:        nil,
		})
		if err != nil {
			t.Fatalf("mark probe failure with nil err failed: %v", err)
		}
		entry, _ := store.GetEntry("acc-nil-err", "")
		if entry.LastError != "" {
			t.Fatalf("expected empty last error for nil err, got %q", entry.LastError)
		}
	})
}

func TestStore_Auxiliary_Coverage(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, _ := state.Load(ctx, filepath.Join(tmpDir, "aux_cache.json"))

	// Test fallback when CycleBurnRate <= 0 in stored entry
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:  "zero-rate-acc",
		ModelGroup: "gemini",
		ObservedAt: now,
		ResetAt:    now.Add(2 * time.Hour), // only ResetAt set, no short window
	})

	rate := store.GetCycleBurnRate("zero-rate-acc", "gemini")
	if rate != state.DefaultCycleBurnRate {
		t.Fatalf("expected default rate %v for zero-rate entry, got %v", state.DefaultCycleBurnRate, rate)
	}
}

func TestStore_GetHistoricalEvidence_And_BuildHistoricalEvidence(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, _ := state.Load(ctx, filepath.Join(tmpDir, "evidence_cache.json"))
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	// 1. Non-existent entry returns false
	if _, ok := store.GetHistoricalEvidence("non-existent", "gemini"); ok {
		t.Errorf("expected false for non-existent entry")
	}

	// 2. Successful probe entry
	shortRem := int64(80)
	longRem := int64(90)
	shortReset := now.Add(3 * time.Hour)
	longReset := now.Add(70 * time.Hour)
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "acc-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ResetAt:              shortReset,
		Remaining:            80,
		ShortWindowResetAt:   shortReset,
		ShortWindowRemaining: &shortRem,
		LongWindowResetAt:    longReset,
		LongWindowRemaining:  &longRem,
	})

	ev, ok := store.GetHistoricalEvidence("acc-1", "gemini")
	if !ok {
		t.Fatalf("expected true for cached ready evidence")
	}
	if ev.AuthIndex != "acc-1" || !ev.ObservedAt.Equal(now) || ev.CycleBurnRate != state.DefaultCycleBurnRate {
		t.Errorf("unexpected cached evidence: %+v", ev)
	}

	// A later failure records its own diagnostic time without overwriting the
	// last successful observation used for read-only history.
	failureAt := now.Add(30 * time.Minute)
	_ = store.MarkProbeFailure(ctx, state.ProbeFailure{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		ObservedAt: failureAt,
		Err:        errors.New("transient upstream failure"),
	})
	entry, ok := store.GetEntry("acc-1", "gemini")
	if !ok || !entry.ObservedAt.Equal(now) || !entry.LastFailureAt.Equal(failureAt) {
		t.Fatalf("failure overwrote historical observation: %#v", entry)
	}
	historical, ok := store.GetHistoricalEvidence("acc-1", "gemini")
	if !ok || !historical.ObservedAt.Equal(now) {
		t.Fatalf("historical observation disappeared after failure: %#v", historical)
	}

	// 3. Failure-only entry has no historical quota authority
	_ = store.MarkProbeFailure(ctx, state.ProbeFailure{
		AuthIndex:  "acc-failed",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		ObservedAt: now,
		Err:        errors.New("upstream failed"),
	})

	if _, ok := store.GetHistoricalEvidence("acc-failed", "gemini"); ok {
		t.Fatalf("failure-only entry must not be returned as quota evidence")
	}
	failedEntry, failedOK := store.GetEntry("acc-failed", "gemini")
	if !failedOK || failedEntry.LastError == "" || !failedEntry.LastFailureAt.Equal(now) {
		t.Fatalf("failure diagnostic was not preserved separately: %#v", failedEntry)
	}

	// 4. BuildHistoricalEvidence
	creds := []core.Credential{
		{AuthIndex: "acc-1"},
		{AuthIndex: "acc-failed"},
		{AuthIndex: "acc-missing"},
	}
	groupEv := store.BuildHistoricalEvidence(creds, "gemini")
	if len(groupEv) != 1 {
		t.Fatalf("expected only historical success from 3 credentials, got %d", len(groupEv))
	}
}

func TestStore_MarkProbeUnavailableSuppressesHistoricalEvidence(t *testing.T) {
	ctx := context.Background()
	store, _ := state.Load(ctx, filepath.Join(t.TempDir(), "unavailable_cache.json"))
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)
	shortRem := int64(70)
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "acc-argon",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "argon",
		ObservedAt:           now,
		ResetAt:              now.Add(5 * 24 * time.Hour),
		Remaining:            70,
		ArgonWindowResetAt:   now.Add(5 * 24 * time.Hour),
		ArgonWindowRemaining: &shortRem,
	})
	if _, ok := store.GetHistoricalEvidence("acc-argon", "argon"); !ok {
		t.Fatal("expected Argon evidence before it becomes unavailable")
	}

	err := store.MarkProbeUnavailable(ctx, state.ProbeUnavailable{
		AuthIndex:  "acc-argon",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "argon",
		ObservedAt: now.Add(time.Hour),
		Reason:     "target model group not entitled",
	})
	if err != nil {
		t.Fatalf("MarkProbeUnavailable failed: %v", err)
	}
	entry, ok := store.GetEntry("acc-argon", "argon")
	if !ok || !entry.Unavailable || entry.LastError != "" || !entry.LastFailureAt.IsZero() {
		t.Fatalf("unavailable entry must not carry a failure diagnostic: %#v", entry)
	}
	if _, ok := store.GetHistoricalEvidence("acc-argon", "argon"); ok {
		t.Fatal("stale Argon evidence must be suppressed while unavailable")
	}

	// A later successful probe clears the unavailable marker.
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "acc-argon",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "argon",
		ObservedAt:           now.Add(2 * time.Hour),
		ResetAt:              now.Add(5 * 24 * time.Hour),
		Remaining:            60,
		ArgonWindowResetAt:   now.Add(5 * 24 * time.Hour),
		ArgonWindowRemaining: &shortRem,
	})
	entry, ok = store.GetEntry("acc-argon", "argon")
	if !ok || entry.Unavailable {
		t.Fatalf("successful probe must clear unavailable state: %#v", entry)
	}
	if _, ok := store.GetHistoricalEvidence("acc-argon", "argon"); !ok {
		t.Fatal("successful probe must restore Argon evidence")
	}
}

func TestStore_GetPlanType(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, _ := state.Load(ctx, filepath.Join(tmpDir, "plan_cache.json"))
	now := time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC)

	// Unknown when empty
	if pt := store.GetPlanType("acc-none", "gemini"); pt != core.PlanTypeUnknown {
		t.Fatalf("expected unknown, got %v", pt)
	}

	// Record success with Pro plan
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:  "acc-pro",
		ModelGroup: "gemini",
		ObservedAt: now,
		ResetAt:    now.Add(time.Hour),
		PlanType:   core.PlanTypePro,
	})

	if pt := store.GetPlanType("acc-pro", "gemini"); pt != core.PlanTypePro {
		t.Fatalf("expected pro, got %v", pt)
	}
	// Falls back to other model groups
	if pt := store.GetPlanType("acc-pro", "claude_gpt"); pt != core.PlanTypePro {
		t.Fatalf("expected fallback to pro, got %v", pt)
	}
}

func TestStore_CleanseClonedArgonEntriesOnLoad(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-cache.json")
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	reset7d := now.Add(48 * time.Hour)
	reset5h := now.Add(3 * time.Hour)

	// Write raw JSON simulating a legacy state document with cloned Argon entries
	rawDoc := fmt.Sprintf(`{
		"schema_version": 1,
		"entries": {
			"auth-cloned|model_group=gemini": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "gemini",
				"auth_index": "auth-cloned",
				"observed_at": %q,
				"remaining": 89,
				"short_window_remaining": 100,
				"short_window_reset_at": %q,
				"long_window_remaining": 89,
				"long_window_reset_at": %q,
				"cycle_burn_rate": 0.15
			},
			"auth-cloned|model_group=argon": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "argon",
				"auth_index": "auth-cloned",
				"observed_at": %q,
				"remaining": 89,
				"short_window_remaining": 100,
				"short_window_reset_at": %q,
				"long_window_remaining": 89,
				"long_window_reset_at": %q,
				"cycle_burn_rate": 0.15
			},
			"auth-valid|model_group=gemini": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "gemini",
				"auth_index": "auth-valid",
				"observed_at": %q,
				"remaining": 95,
				"long_window_remaining": 95,
				"long_window_reset_at": %q,
				"argon_window_remaining": 70,
				"argon_window_reset_at": %q
			},
			"auth-valid|model_group=argon": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "argon",
				"auth_index": "auth-valid",
				"observed_at": %q,
				"remaining": 70,
				"long_window_remaining": 70,
				"long_window_reset_at": %q,
				"argon_window_remaining": 70,
				"argon_window_reset_at": %q
			}
		}
	}`,
		now.Format(time.RFC3339), reset5h.Format(time.RFC3339), reset7d.Format(time.RFC3339),
		now.Format(time.RFC3339), reset5h.Format(time.RFC3339), reset7d.Format(time.RFC3339),
		now.Format(time.RFC3339), reset7d.Format(time.RFC3339), reset7d.Add(12*time.Hour).Format(time.RFC3339),
		now.Format(time.RFC3339), reset7d.Add(12*time.Hour).Format(time.RFC3339), reset7d.Add(12*time.Hour).Format(time.RFC3339),
	)
	if err := os.WriteFile(path, []byte(rawDoc), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := state.Load(ctx, path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Cloned Argon entry must be cleansed on Load
	if _, ok := store.GetEntry("auth-cloned", "argon"); ok {
		t.Errorf("expected cloned argon entry to be cleansed")
	}
	if _, ok := store.GetHistoricalEvidence("auth-cloned", "argon"); ok {
		t.Errorf("expected GetHistoricalEvidence for cloned argon to return false")
	}
	// Gemini entry for auth-cloned must remain intact
	if _, ok := store.GetEntry("auth-cloned", "gemini"); !ok {
		t.Errorf("expected gemini entry for auth-cloned to remain")
	}
	if entry, ok := store.GetEntry("auth-cloned", "gemini"); !ok || entry.SchemaVersion != state.SchemaVersion {
		t.Errorf("expected legacy entry to migrate to schema %d, got %d", state.SchemaVersion, entry.SchemaVersion)
	}
	if ev, ok := store.GetHistoricalEvidence("auth-cloned", "gemini"); !ok || ev.Remaining == nil || *ev.Remaining != 89 {
		t.Errorf("expected valid gemini evidence for auth-cloned")
	}

	// Valid Argon entry must be preserved
	if _, ok := store.GetEntry("auth-valid", "argon"); !ok {
		t.Errorf("expected valid argon entry to be preserved")
	}
	if ev, ok := store.GetHistoricalEvidence("auth-valid", "argon"); !ok || ev.Remaining == nil || *ev.Remaining != 70 {
		t.Errorf("expected valid argon evidence for auth-valid")
	}
}

func TestStore_ScheduleConfig_DynamicConfig_Sync(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "schedule_sync_test.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// 1. Initially set dynamic config with paused = false
	dyn := state.DynamicConfig{
		AutoApply: true,
		Interval:  "15m",
		Schedule: state.ScheduleConfig{
			Paused:        false,
			WindowEnabled: true,
			WindowStart:   "08:00",
			WindowEnd:     "22:00",
		},
	}
	store.SetDynamicConfig(dyn)
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("SaveAtomic failed: %v", err)
	}

	// Verify scheduleConfig is in sync
	sched := store.GetScheduleConfig()
	if sched.Paused || !sched.WindowEnabled {
		t.Fatalf("expected scheduleConfig to match dynamic config, got %+v", sched)
	}

	// 2. Pause schedule via SetScheduleConfig
	store.SetScheduleConfig(state.ScheduleConfig{
		Paused:        true,
		WindowEnabled: true,
		WindowStart:   "08:00",
		WindowEnd:     "22:00",
	})
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("SaveAtomic failed: %v", err)
	}

	// Verify GetDynamicConfig has paused = true
	dynGot, ok := store.GetDynamicConfig()
	if !ok || !dynGot.Schedule.Paused {
		t.Fatalf("expected GetDynamicConfig to reflect paused=true, got %+v", dynGot.Schedule)
	}

	// 3. Simulate legacy corrupted cache with desynchronized schedule_config vs app_config
	legacyDesynced := `{
		"schema_version": 1,
		"schedule_config": {
			"paused": true,
			"window_enabled": true,
			"window_start": "09:00",
			"window_end": "21:00"
		},
		"app_config": {
			"auto_apply": true,
			"interval": "15m",
			"schedule": {
				"paused": false,
				"window_enabled": false,
				"window_start": "00:00",
				"window_end": "23:59"
			}
		}
	}`
	legacyPath := filepath.Join(t.TempDir(), "legacy_desynced.json")
	if err := os.WriteFile(legacyPath, []byte(legacyDesynced), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err := state.Load(ctx, legacyPath)
	if err != nil {
		t.Fatalf("Load legacy desynced failed: %v", err)
	}

	if !loaded.GetScheduleConfig().Paused {
		t.Fatalf("expected loaded scheduleConfig to be paused=true")
	}
	loadedDyn, ok := loaded.GetDynamicConfig()
	if !ok || !loadedDyn.Schedule.Paused {
		t.Fatalf("expected loaded dynamicConfig to be healed with paused=true, got %+v", loadedDyn.Schedule)
	}
}

func TestResolvePaths_DerivesSiblingDocuments(t *testing.T) {
	mainPath := filepath.Join("data", "antigravity-priority", "state.json")
	paths := state.ResolvePaths(mainPath)
	if paths.Main != mainPath {
		t.Fatalf("main path = %q; want %q", paths.Main, mainPath)
	}
	wantRuntime := filepath.Join("data", "antigravity-priority", "runtime.json")
	wantRuns := filepath.Join("data", "antigravity-priority", "runs.json")
	wantRequests := filepath.Join("data", "antigravity-priority", "requests.json")
	if paths.Runtime != wantRuntime || paths.Runs != wantRuns || paths.Requests != wantRequests {
		t.Fatalf("derived paths = %+v; want runtime=%q runs=%q requests=%q", paths, wantRuntime, wantRuns, wantRequests)
	}

	fallback := state.ResolvePaths("")
	if fallback.Main != config.DefaultStateCachePath {
		t.Fatalf("empty path fallback = %q; want %q", fallback.Main, config.DefaultStateCachePath)
	}
	legacyMapped := state.ResolvePaths(config.LegacyDefaultStateCachePath)
	if legacyMapped.Main != config.DefaultStateCachePath {
		t.Fatalf("legacy default must map into the dedicated directory, got %q", legacyMapped.Main)
	}
	legacy := state.LegacyPaths(config.LegacyDefaultStateCachePath)
	wantLegacyRuntime := filepath.Join("data", "antigravity-priority-cache.runtime.json")
	wantLegacyRuns := filepath.Join("data", "antigravity-priority-cache.runs.json")
	wantLegacyRequests := filepath.Join("data", "antigravity-priority-cache.requests.json")
	if legacy.Main != config.LegacyDefaultStateCachePath || legacy.Runtime != wantLegacyRuntime || legacy.Runs != wantLegacyRuns || legacy.Requests != wantLegacyRequests {
		t.Fatalf("legacy paths = %+v", legacy)
	}
	fromNewDefault := state.LegacyPaths(config.DefaultStateCachePath)
	if fromNewDefault.Main != config.LegacyDefaultStateCachePath {
		t.Fatalf("new default must still locate the legacy cache for migration, got %q", fromNewDefault.Main)
	}
}

func TestStore_SplitDocumentsPersistIndependently(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	paths := state.ResolvePaths(cachePath)

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "split-auth",
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ShortWindowResetAt:   now.Add(3 * time.Hour),
		ShortWindowRemaining: int64Ptr(80),
		LongWindowResetAt:    now.Add(72 * time.Hour),
		LongWindowRemaining:  int64Ptr(90),
	}); err != nil {
		t.Fatal(err)
	}
	store.SetScheduleConfig(state.ScheduleConfig{Paused: true})
	store.SetRuntimeSnapshot("split audit", []byte(`{"ok":true}`), []byte(`[{"kind":"probe"}]`))
	store.SetRequestHistory([]state.RequestHistoryEntry{{ID: "split-req", At: now, Model: "m", ModelPool: "gemini"}})

	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{paths.Main, paths.Runtime, paths.Runs, paths.Requests} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected split document %s: %v", path, err)
		}
	}

	mainRaw, err := os.ReadFile(paths.Main)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacyKey := range []string{"request_history", "run_history", "latest_audit", "latest_result", "schedule_config", "app_config"} {
		if strings.Contains(string(mainRaw), `"`+legacyKey+`"`) {
			t.Fatalf("main state document still contains %q: %s", legacyKey, mainRaw)
		}
	}

	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.GetEntry("split-auth", "gemini"); !ok {
		t.Fatal("expected quota entry in main state document")
	}
	if !reloaded.GetScheduleConfig().Paused {
		t.Fatal("expected schedule config in runtime document")
	}
	audit, resultJSON, historyJSON := reloaded.GetRuntimeSnapshot()
	if audit != "split audit" || len(resultJSON) == 0 || len(historyJSON) == 0 {
		t.Fatalf("unexpected runs document: audit=%q result=%s history=%s", audit, resultJSON, historyJSON)
	}
	history := reloaded.GetRequestHistory()
	if len(history) != 1 || history[0].ID != "split-req" {
		t.Fatalf("unexpected request history: %+v", history)
	}
}

func TestStore_TargetedSaveLeavesOtherDocumentsUntouched(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	paths := state.ResolvePaths(cachePath)

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	store.SetScheduleConfig(state.ScheduleConfig{Paused: false})
	store.SetRuntimeSnapshot("audit", []byte(`{"ok":true}`), []byte(`[]`))
	store.SetRequestHistory([]state.RequestHistoryEntry{{ID: "first", At: now, Model: "m", ModelPool: "gemini"}})
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	beforeMain, err := os.ReadFile(paths.Main)
	if err != nil {
		t.Fatal(err)
	}
	beforeRuntime, err := os.ReadFile(paths.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	beforeRuns, err := os.ReadFile(paths.Runs)
	if err != nil {
		t.Fatal(err)
	}

	store.SetRequestHistory([]state.RequestHistoryEntry{{ID: "second", At: now, Model: "m", ModelPool: "gemini"}})
	if err := store.SaveRequestsAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	afterMain, _ := os.ReadFile(paths.Main)
	afterRuntime, _ := os.ReadFile(paths.Runtime)
	afterRuns, _ := os.ReadFile(paths.Runs)
	if string(beforeMain) != string(afterMain) {
		t.Fatal("request history save rewrote the main state document")
	}
	if string(beforeRuntime) != string(afterRuntime) {
		t.Fatal("request history save rewrote the runtime document")
	}
	if string(beforeRuns) != string(afterRuns) {
		t.Fatal("request history save rewrote the runs document")
	}
	if err := state.WriteRequestHistoryAtomic(ctx, cachePath, []state.RequestHistoryEntry{{ID: "third", At: now, Model: "m", ModelPool: "gemini"}}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	history := reloaded.GetRequestHistory()
	if len(history) != 1 || history[0].ID != "third" {
		t.Fatalf("direct request history write not loaded: %+v", history)
	}
}

func TestStore_LoadMigratesLegacySingleFileCache(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "legacy-cache.json")
	legacy := `{
		"schema_version": 1,
		"entries": {
			"legacy-auth|model_group=gemini": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "gemini",
				"auth_index": "legacy-auth",
				"observed_at": "2026-11-10T12:00:00Z",
				"remaining": 80,
				"cycle_burn_rate": 0.15
			}
		},
		"latest_audit": "legacy audit",
		"run_history": [{"kind":"probe","at":"2026-11-10T12:00:00Z"}],
		"request_history": [{"id":"legacy-req","at":"2026-11-10T12:00:00Z","model":"m","model_pool":"gemini"}],
		"schedule_config": {"paused": true},
		"app_config": {"auto_apply": true, "interval": "15m"},
		"last_auto_apply_at": "2026-11-10T12:00:00Z"
	}`
	if err := os.WriteFile(cachePath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GetEntry("legacy-auth", "gemini"); !ok {
		t.Fatal("expected legacy quota entry")
	}
	if audit, _, _ := store.GetRuntimeSnapshot(); audit != "legacy audit" {
		t.Fatalf("legacy audit = %q", audit)
	}
	if history := store.GetRequestHistory(); len(history) != 1 || history[0].ID != "legacy-req" {
		t.Fatalf("legacy request history = %+v", history)
	}
	if !store.GetScheduleConfig().Paused {
		t.Fatal("expected legacy schedule config")
	}

	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}
	paths := state.ResolvePaths(cachePath)
	for _, path := range []string{paths.Runtime, paths.Runs, paths.Requests} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migrated document %s: %v", path, err)
		}
	}
	mainRaw, err := os.ReadFile(paths.Main)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacyKey := range []string{"request_history", "run_history", "latest_audit", "schedule_config", "app_config"} {
		if strings.Contains(string(mainRaw), `"`+legacyKey+`"`) {
			t.Fatalf("migrated main document still contains %q", legacyKey)
		}
	}
	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.GetEntry("legacy-auth", "gemini"); !ok {
		t.Fatal("expected migrated quota entry")
	}
	if history := reloaded.GetRequestHistory(); len(history) != 1 || history[0].ID != "legacy-req" {
		t.Fatalf("migrated request history = %+v", history)
	}
}

func TestStore_CorruptSidecarIsIsolated(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	paths := state.ResolvePaths(cachePath)

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "isolated-auth",
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ShortWindowResetAt:   now.Add(3 * time.Hour),
		ShortWindowRemaining: int64Ptr(70),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStateAtomic(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Requests, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}

	reloaded, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("corrupt request sidecar must not fail the load: %v", err)
	}
	if _, ok := reloaded.GetEntry("isolated-auth", "gemini"); !ok {
		t.Fatal("main quota state was lost because a sidecar was corrupt")
	}
	if len(reloaded.GetRequestHistory()) != 0 {
		t.Fatal("corrupt request history should load as empty")
	}
	if len(reloaded.LoadErrors()) == 0 {
		t.Fatal("expected the corrupt sidecar to be reported")
	}
}

func TestStore_MigratesLegacyDefaultIntoDedicatedDirectory(t *testing.T) {
	ctx := context.Background()
	t.Chdir(t.TempDir())

	writeFile := func(path, content string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	legacyMain := `{
		"schema_version": 1,
		"entries": {
			"legacy-auth|model_group=gemini": {
				"schema_version": 1,
				"provider": "antigravity",
				"model_group": "gemini",
				"auth_index": "legacy-auth",
				"observed_at": "2026-11-10T12:00:00Z",
				"remaining": 80,
				"cycle_burn_rate": 0.15
			}
		},
		"latest_audit": "legacy main audit"
	}`
	legacyRuntime := `{"schema_version":1,"schedule_config":{"paused":true},"app_config":{"auto_apply":true,"interval":"15m"}}`
	legacyRuns := `{"schema_version":1,"latest_audit":"legacy runs"}`
	legacyRequests := `{"schema_version":1,"request_history":[{"id":"legacy-req","at":"2026-11-10T12:00:00Z","model":"m","model_pool":"gemini"}]}`

	legacy := state.LegacyPaths(config.LegacyDefaultStateCachePath)
	writeFile(legacy.Main, legacyMain)
	writeFile(legacy.Runtime, legacyRuntime)
	writeFile(legacy.Runs, legacyRuns)
	writeFile(legacy.Requests, legacyRequests)

	store, err := state.Load(ctx, config.LegacyDefaultStateCachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GetEntry("legacy-auth", "gemini"); !ok {
		t.Fatal("expected legacy quota entry")
	}
	if audit, _, _ := store.GetRuntimeSnapshot(); audit != "legacy runs" {
		t.Fatalf("legacy runs audit = %q", audit)
	}
	if history := store.GetRequestHistory(); len(history) != 1 || history[0].ID != "legacy-req" {
		t.Fatalf("legacy request history = %+v", history)
	}
	if !store.GetScheduleConfig().Paused {
		t.Fatal("expected legacy schedule config")
	}

	paths := state.ResolvePaths(config.DefaultStateCachePath)
	for _, path := range []string{paths.Main, paths.Runtime, paths.Runs, paths.Requests} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected migrated document %s: %v", path, err)
		}
	}
	for _, path := range []string{legacy.Main, legacy.Runtime, legacy.Runs, legacy.Requests} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("legacy file %s must be removed after migration, got %v", path, err)
		}
	}

	reloaded, err := state.Load(ctx, config.LegacyDefaultStateCachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.GetEntry("legacy-auth", "gemini"); !ok {
		t.Fatal("expected migrated quota entry after reload")
	}
	if history := reloaded.GetRequestHistory(); len(history) != 1 || history[0].ID != "legacy-req" {
		t.Fatalf("migrated request history = %+v", history)
	}
}
