package state_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	if store.HasEntry("auth-1", "gemini") {
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
	if err := store.MarkProbeScheduled(ctx, state.ProbeSchedule{}); err == nil {
		t.Fatalf("expected error on canceled context in MarkProbeScheduled, got nil")
	}
	if _, err := store.NeedsProbe(ctx, state.ProbeCheck{}); err == nil {
		t.Fatalf("expected error on canceled context in NeedsProbe, got nil")
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

func TestStore_MarkProbeScheduled(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "sched_cache.json")

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	next := time.Date(2026, 8, 18, 14, 30, 0, 0, time.UTC)
	err = store.MarkProbeScheduled(ctx, state.ProbeSchedule{
		AuthIndex:   "sched-account",
		Provider:    core.ProviderAntigravity,
		ModelGroup:  "gemini",
		NextProbeAt: next,
	})
	if err != nil {
		t.Fatalf("mark probe scheduled failed: %v", err)
	}

	entry, ok := store.GetEntry("sched-account", "gemini")
	if !ok {
		t.Fatalf("expected scheduled entry to exist")
	}
	if entry.NextProbeAt != next {
		t.Fatalf("expected next probe at %v, got %v", next, entry.NextProbeAt)
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
	nextProbe := now.Add(15 * time.Minute)

	t.Run("sensitive token redaction", func(t *testing.T) {
		err = store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:   "acc-fail",
			ModelGroup:  "claude_gpt",
			ObservedAt:  now,
			NextProbeAt: nextProbe,
			Err:         errors.New("request failed with Bearer secret-token-12345: upstream 500"),
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
			AuthIndex:   "acc-long-fail",
			ModelGroup:  "",
			ObservedAt:  now,
			NextProbeAt: nextProbe,
			Err:         errors.New(longErr),
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
			AuthIndex:   "acc-nil-err",
			ObservedAt:  now,
			NextProbeAt: nextProbe,
			Err:         nil,
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

func TestStore_NeedsProbe(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, _ := state.Load(ctx, filepath.Join(tmpDir, "cache.json"))

	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	policy := state.ProbePolicy{
		TTL:             15 * time.Minute,
		ResetStaleAfter: 1 * time.Hour,
	}

	// 1. Unknown entry -> Needs probe
	needs, err := store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		ModelGroup: "gemini",
		Now:        now,
		Policy:     policy,
	})
	if err != nil || !needs {
		t.Fatalf("expected needs probe for unknown entry, got %v, err=%v", needs, err)
	}

	// 2. Fresh entry -> Does NOT need probe
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "acc-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now,
		ShortWindowResetAt:   now.Add(4 * time.Hour),
		ShortWindowRemaining: int64Ptr(80),
		LongWindowResetAt:    now.Add(48 * time.Hour),
		LongWindowRemaining:  int64Ptr(90),
	})

	needs, err = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(5 * time.Minute), // only 5m passed
		Policy:     policy,
	})
	if err != nil || needs {
		t.Fatalf("expected not needing probe within TTL, got %v, err=%v", needs, err)
	}

	// 3. Different provider / model group -> Needs probe
	needs, _ = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.Provider("other"),
		ModelGroup: "gemini",
		Now:        now.Add(5 * time.Minute),
		Policy:     policy,
	})
	if !needs {
		t.Fatalf("expected needing probe when provider mismatch")
	}

	needs, _ = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "claude_gpt",
		Now:        now.Add(5 * time.Minute),
		Policy:     policy,
	})
	if !needs {
		t.Fatalf("expected needing probe when model group mismatch")
	}

	// 4. TTL Expired -> Needs probe
	needs, err = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(16 * time.Minute), // 16m > 15m TTL
		Policy:     policy,
	})
	if err != nil || !needs {
		t.Fatalf("expected needing probe when TTL expired, got %v, err=%v", needs, err)
	}

	// 5. Short Window Reset reached -> Needs probe
	needs, err = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(4 * time.Hour).Add(1 * time.Second),
		Policy:     policy,
	})
	if err != nil || !needs {
		t.Fatalf("expected needing probe when short reset reached, got %v, err=%v", needs, err)
	}

	// 6. Reset stale after -> Needs probe
	needs, err = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(5 * time.Hour).Add(30 * time.Minute), // > 1h after 4h reset
		Policy:     policy,
	})
	if err != nil || !needs {
		t.Fatalf("expected needing probe when reset too old, got %v, err=%v", needs, err)
	}

	// 7. NextProbeAt in future vs passed
	_ = store.MarkProbeFailure(ctx, state.ProbeFailure{
		AuthIndex:   "acc-cooldown",
		Provider:    core.ProviderAntigravity,
		ModelGroup:  "gemini",
		ObservedAt:  now,
		NextProbeAt: now.Add(10 * time.Minute),
	})
	// before NextProbeAt -> false
	needs, _ = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-cooldown",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(3 * time.Minute),
		Policy:     policy,
	})
	if needs {
		t.Fatalf("expected not needing probe during cooldown")
	}
	// after NextProbeAt -> true
	needs, _ = store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "acc-cooldown",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		Now:        now.Add(12 * time.Minute),
		Policy:     policy,
	})
	if !needs {
		t.Fatalf("expected needing probe after cooldown passed")
	}
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

	// Test isResetReached with ResetAt
	policy := state.ProbePolicy{TTL: 1 * time.Hour}
	needs, _ := store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  "zero-rate-acc",
		ModelGroup: "gemini",
		Now:        now.Add(2 * time.Hour).Add(1 * time.Second),
		Policy:     policy,
	})
	if !needs {
		t.Fatalf("expected needing probe when ResetAt is reached")
	}
}

func TestStore_GetCachedEvidence_And_BuildGroupEvidence(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	store, _ := state.Load(ctx, filepath.Join(tmpDir, "evidence_cache.json"))
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	// 1. Non-existent entry returns false
	if _, ok := store.GetCachedEvidence("non-existent", "gemini"); ok {
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

	ev, ok := store.GetCachedEvidence("acc-1", "gemini")
	if !ok {
		t.Fatalf("expected true for cached ready evidence")
	}
	if ev.AuthIndex != "acc-1" || !ev.EvidenceFresh || ev.CycleBurnRate != state.DefaultCycleBurnRate {
		t.Errorf("unexpected cached evidence: %+v", ev)
	}

	// 3. Failure entry returns EvidenceStatusProbeFailed
	_ = store.MarkProbeFailure(ctx, state.ProbeFailure{
		AuthIndex:  "acc-failed",
		Provider:   core.ProviderAntigravity,
		ModelGroup: "gemini",
		ObservedAt: now,
		Err:        errors.New("upstream failed"),
	})

	evFail, ok := store.GetCachedEvidence("acc-failed", "gemini")
	if !ok {
		t.Fatalf("expected true for cached failure evidence")
	}
	if evFail.Status != "probe_failed" {
		t.Errorf("expected Status probe_failed, got %v", evFail.Status)
	}

	// 4. BuildGroupEvidence
	creds := []core.Credential{
		{AuthIndex: "acc-1"},
		{AuthIndex: "acc-failed"},
		{AuthIndex: "acc-missing"},
	}
	groupEv := store.BuildGroupEvidence(creds, "gemini")
	if len(groupEv) != 2 {
		t.Fatalf("expected 2 evidences from 3 credentials, got %d", len(groupEv))
	}
}
