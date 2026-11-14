package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

type fixedAdaptiveClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedAdaptiveClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedAdaptiveClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestEvaluateAdaptivePlan_IdleBootstrapLowAndFrequent(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	midpointJitter := func(minDur, maxDur time.Duration) time.Duration {
		return minDur + (maxDur-minDur)/2
	}

	t.Run("bootstrap when uncalibrated and no cached evidence", func(t *testing.T) {
		snap, _ := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now,
			interval:         15 * time.Minute,
			hasQuotaEvidence: false,
			jitterRange:      midpointJitter,
		})
		if snap.Mode != ScheduleModeBootstrap || !snap.ShouldRunNow {
			t.Fatalf("expected bootstrap immediate run, got mode=%s shouldRun=%v", snap.Mode, snap.ShouldRunNow)
		}
	})

	t.Run("idle when zero requests and cached evidence exists", func(t *testing.T) {
		snap, _ := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now,
			interval:         15 * time.Minute,
			lastAutoApplyAt:  now.Add(-20 * time.Minute),
			hasQuotaEvidence: true,
			jitterRange:      midpointJitter,
		})
		if snap.Mode != ScheduleModeIdle || snap.ShouldRunNow || !snap.NextRunAt.IsZero() {
			t.Fatalf("expected idle sleep, got %+v", snap)
		}
	})

	t.Run("low frequency stretches interval and anchors to FirstSinceLastAt after long idle", func(t *testing.T) {
		// Last run was 4h ago; 1 single isolated request arrived just now.
		snap, latched := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now,
			interval:         15 * time.Minute,
			lastAutoApplyAt:  now.Add(-4 * time.Hour),
			hasQuotaEvidence: true,
			summary: state.RequestActivitySummary{
				CurrWindowCount:  1,
				SinceLastCount:   1,
				FirstSinceLastAt: now,
				LastRequestAt:    now,
			},
			jitterRange: midpointJitter,
		})
		if snap.Mode != ScheduleModeLowFrequency || snap.ShouldRunNow {
			t.Fatalf("expected deferred low_frequency mode, got mode=%s shouldRun=%v", snap.Mode, snap.ShouldRunNow)
		}
		if snap.NextWait < 15*time.Minute {
			t.Fatalf("expected stretched wait >= 15m, got %v", snap.NextWait)
		}

		// 30m later with no additional requests: current window is empty, so idle-decay guard sleeps.
		later := now.Add(30 * time.Minute)
		decayed, _ := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              later,
			interval:         15 * time.Minute,
			lastAutoApplyAt:  now.Add(-4 * time.Hour),
			hasQuotaEvidence: true,
			latched:          latched,
			summary: state.RequestActivitySummary{
				PrevWindowCount:  1,
				CurrWindowCount:  0,
				SinceLastCount:   1,
				FirstSinceLastAt: now,
				LastRequestAt:    now,
			},
			jitterRange: midpointJitter,
		})
		if decayed.Mode != ScheduleModeIdle || decayed.ShouldRunNow {
			t.Fatalf("expected idle-decay guard to skip stale sporadic request, got mode=%s shouldRun=%v", decayed.Mode, decayed.ShouldRunNow)
		}
	})

	t.Run("frequent usage schedules strictly within configured interval with jitter", func(t *testing.T) {
		lastRun := now.Add(-5 * time.Minute)
		snap, latched := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now,
			interval:         15 * time.Minute,
			lastAutoApplyAt:  lastRun,
			hasQuotaEvidence: true,
			summary: state.RequestActivitySummary{
				PrevWindowCount: 8,
				CurrWindowCount: 6,
				SinceLastCount:  6,
				BurstCount:      3,
				LastRequestAt:   now,
			},
			jitterRange: midpointJitter,
		})
		if snap.Mode != ScheduleModeFrequent {
			t.Fatalf("expected frequent mode, got %s", snap.Mode)
		}
		if snap.NextRunAt.After(lastRun.Add(15 * time.Minute)) {
			t.Fatalf("frequent NextRunAt %v exceeded 15m ceiling %v", snap.NextRunAt, lastRun.Add(15*time.Minute))
		}
		if snap.NextRunAt.Before(lastRun.Add(11 * time.Minute)) {
			t.Fatalf("frequent NextRunAt %v expected jitter >= 75%% of 15m", snap.NextRunAt)
		}
		// Latched deadline remains stable across re-evaluations before NextRunAt.
		again, _ := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now.Add(1 * time.Minute),
			interval:         15 * time.Minute,
			lastAutoApplyAt:  lastRun,
			hasQuotaEvidence: true,
			latched:          latched,
			summary: state.RequestActivitySummary{
				PrevWindowCount: 8,
				CurrWindowCount: 7,
				SinceLastCount:  7,
				BurstCount:      4,
				LastRequestAt:   now.Add(1 * time.Minute),
			},
			jitterRange: func(minDur, maxDur time.Duration) time.Duration { return minDur },
		})
		if !again.NextRunAt.Equal(snap.NextRunAt) {
			t.Fatalf("expected latched NextRunAt %v to stay stable, got %v", snap.NextRunAt, again.NextRunAt)
		}
	})

	t.Run("historical active baseline adapts reference threshold without hardcoding", func(t *testing.T) {
		// Heavy historical baseline (~40 reqs/window): 4 requests in current window
		// is below the adapted threshold (capped at 3*cadenceFloor ~ 11.6) so it stays low_frequency.
		snap, _ := evaluateAdaptivePlan(adaptiveEvalInput{
			now:              now,
			interval:         15 * time.Minute,
			lastAutoApplyAt:  now.Add(-15 * time.Minute),
			hasQuotaEvidence: true,
			summary: state.RequestActivitySummary{
				CurrWindowCount:        4,
				SinceLastCount:         4,
				HistoricalWindowCounts: []int{38, 42, 40, 44},
				FirstSinceLastAt:       now.Add(-10 * time.Minute),
				LastRequestAt:          now.Add(-2 * time.Minute),
			},
			jitterRange: midpointJitter,
		})
		if snap.Mode != ScheduleModeLowFrequency {
			t.Fatalf("expected heavy-baseline user with 4 sparse requests to classify as low_frequency, got %s (ratio=%.2f ref=%.2f)", snap.Mode, snap.ActivityRatio, snap.ReferenceThreshold)
		}
	})
}

func TestRuntime_AdaptiveTicker_IdleSkipsAndBurstWakes(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "adaptive-worker-cache.json")
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	clock := &fixedAdaptiveClock{now: now}

	// Pre-populate cached evidence so startup respects traffic activity.
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	rem := int64(80)
	if err := store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
		AuthIndex:            "auth-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now.Add(-2 * time.Hour),
		ShortWindowResetAt:   now.Add(3 * time.Hour),
		ShortWindowRemaining: &rem,
		LongWindowResetAt:    now.Add(5 * 24 * time.Hour),
		LongWindowRemaining:  &rem,
	}); err != nil {
		t.Fatal(err)
	}
	store.SetLastAutoApplyAt(now.Add(-2 * time.Hour))
	if err := store.SaveAtomic(context.Background()); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	r := New(Options{
		Clock:          clock,
		Sleeper:        realSleeper{},
		StateCachePath: cachePath,
		Runner: func(ctx context.Context, request TaskRequest) error {
			calls.Add(1)
			return nil
		},
	})
	defer func() { _ = r.Shutdown(context.Background()) }()

	// Register with auto_apply enabled and short 1m interval.
	resp := r.Handle(context.Background(), MethodPluginRegister, []byte(`{"config_yaml":"enabled: true\nstate_cache_path: `+filepath.ToSlash(cachePath)+`\n"}`))
	var regEnv Envelope
	if err := json.Unmarshal(resp, &regEnv); err != nil || !regEnv.OK {
		t.Fatalf("register failed: %s", string(resp))
	}
	dyn, err := r.GetDynamicConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dyn.AutoApply = true
	dyn.Interval = "1m"
	if err := r.SetDynamicConfig(context.Background(), dyn); err != nil {
		t.Fatal(err)
	}

	// Wait past the 2s startup delay while idle: zero AutoApply calls should run.
	time.Sleep(2400 * time.Millisecond)
	if got := calls.Load(); got != 0 {
		t.Fatalf("expected 0 AutoApply calls on idle startup with cached evidence, got %d", got)
	}

	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sched := diag["scheduler"].(map[string]any)
	if sched["mode"] != string(ScheduleModeIdle) {
		t.Fatalf("expected scheduler mode=idle, got %v", sched["mode"])
	}
	if sched["next_run_at"] != "" {
		t.Fatalf("expected empty next_run_at while idle, got %v", sched["next_run_at"])
	}
	for range 4 {
		r.Handle(context.Background(), MethodUsageHandle, []byte(`{"Provider":"antigravity","Model":"gemini-3-flash","AuthIndex":"auth-1","Failed":false}`))
	}
	diagBurst, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	schedBurst := diagBurst["scheduler"].(map[string]any)
	if schedBurst["mode"] != string(ScheduleModeFrequent) {
		t.Fatalf("expected scheduler mode=frequent after burst, got %v", schedBurst["mode"])
	}
	if schedBurst["curr_window_requests"] != 4 {
		t.Fatalf("expected curr_window_requests=4, got %v", schedBurst["curr_window_requests"])
	}

	// Advance clock by 35s and trigger worker wake so it sees the elapsed deadline without waiting real wall time.
	clock.Advance(35 * time.Second)
	r.signalAutoScheduleWorker()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected burst wake to execute 1 AutoApply call, got %d", got)
	}

	// Verify Host callbacks aren't required when custom Runner is injected, and
	// confirm scheduler transitions back to idle right after calibration.
	diagAfter, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	schedAfter := diagAfter["scheduler"].(map[string]any)
	if schedAfter["mode"] != string(ScheduleModeIdle) {
		t.Fatalf("expected scheduler to return to idle after calibrating burst, got %v", schedAfter["mode"])
	}
	_ = host.AuthFile{}
}

func TestRuntime_AdaptiveSchedule_IgnoresWarmupRequestsLiveAndAfterRestart(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "adaptive-warmup-isolation.json")
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	clock := &fixedAdaptiveClock{now: now}

	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	rem := int64(100)
	if err := store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
		AuthIndex:            "auth-1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           now.Add(-2 * time.Hour),
		ShortWindowResetAt:   now.Add(5 * time.Hour),
		ShortWindowRemaining: &rem,
		LongWindowResetAt:    now.Add(5 * 24 * time.Hour),
		LongWindowRemaining:  &rem,
	}); err != nil {
		t.Fatal(err)
	}
	store.SetLastAutoApplyAt(now.Add(-2 * time.Hour))
	if err := store.SaveAtomic(context.Background()); err != nil {
		t.Fatal(err)
	}

	rt := New(Options{
		Clock:                    clock,
		StateCachePath:           cachePath,
		DisableBackgroundWorkers: true,
	})
	doer := quotaHTTPDoerFunc(func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
		if strings.Contains(req.URL, "generateContent") {
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"response":{"usageMetadata":{"promptTokenCount":50,"candidatesTokenCount":650,"totalTokenCount":700}}}`),
			}, nil
		}
		return host.HTTPResponse{StatusCode: http.StatusOK}, nil
	})
	resetAt := now.Add(5 * time.Hour)
	evidenceByGroup := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {
			Eligible: []priority.QuotaEvidence{{
				AuthIndex:            "auth-1",
				ShortWindowRemaining: &rem,
				ShortWindowResetAt:   &resetAt,
			}},
		},
	}
	materials := map[string]authMaterial{
		"auth-1": {accessToken: "token-1", projectID: "proj-1", email: "user@example.com"},
	}
	warmCfg := config.WarmupConfig{
		GeminiEnabled: true,
		GeminiModel:   "gemini-3-flash",
	}
	rtStore, err := rt.state.Store(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if warmed := rt.triggerWarmupForEligibleCredentials(context.Background(), doer, warmCfg, "", evidenceByGroup, materials, nil, rtStore, now); warmed != 1 {
		t.Fatalf("expected 1 warmup execution, got %d", warmed)
	}

	assertIdle := func(label string, target *Runtime) {
		t.Helper()
		diag, err := target.Diagnostics(context.Background())
		if err != nil {
			t.Fatalf("%s: diagnostics failed: %v", label, err)
		}
		sched := diag["scheduler"].(map[string]any)
		if sched["mode"] != string(ScheduleModeIdle) {
			t.Fatalf("%s: expected mode=idle, got %v", label, sched["mode"])
		}
		if sched["requests_since_last"] != 0 {
			t.Fatalf("%s: expected requests_since_last=0, got %v", label, sched["requests_since_last"])
		}
		if sched["curr_window_requests"] != 0 {
			t.Fatalf("%s: expected curr_window_requests=0, got %v", label, sched["curr_window_requests"])
		}
		if sched["last_request_at"] != nil {
			t.Fatalf("%s: expected nil last_request_at, got %v", label, sched["last_request_at"])
		}
		if sched["next_run_at"] != "" {
			t.Fatalf("%s: expected empty next_run_at, got %v", label, sched["next_run_at"])
		}
	}

	assertIdle("live", rt)
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Reload a fresh Runtime from disk; seedActivityFromRequestHistory must ignore the persisted warmup entry.
	restarted := New(Options{
		Clock:                    clock,
		StateCachePath:           cachePath,
		DisableBackgroundWorkers: true,
	})
	defer func() { _ = restarted.Shutdown(context.Background()) }()

	history, err := restarted.GetRequestHistory(context.Background())
	if err != nil || len(history) != 1 {
		t.Fatalf("expected 1 persisted warmup history entry, got len=%d err=%v", len(history), err)
	}
	if history[0].Source != state.RequestHistorySourceWarmup {
		t.Fatalf("expected source=%q, got %q", state.RequestHistorySourceWarmup, history[0].Source)
	}
	assertIdle("restarted", restarted)
}
