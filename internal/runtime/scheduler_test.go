package runtime_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

type callbackTrackingHost struct {
	mockHost
	callCount int64
}

func (c *callbackTrackingHost) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.ListAuthFiles(ctx)
}

func (c *callbackTrackingHost) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.GetAuth(ctx, authIndex)
}

func (c *callbackTrackingHost) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.GetRuntime(ctx, authIndex)
}

func (c *callbackTrackingHost) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.SaveAuth(ctx, name, doc)
}

func (c *callbackTrackingHost) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.HTTPDo(ctx, req)
}

func (c *callbackTrackingHost) ListAuthModels(ctx context.Context) ([]host.AuthModelsEntry, error) {
	atomic.AddInt64(&c.callCount, 1)
	return c.mockHost.ListAuthModels(ctx)
}

// 1. Acceptance Criterion 1: Capabilities in register/reconfigure
func TestScheduler_RegistrationCapabilities(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	r := runtime.New(runtime.Options{Host: mock, StateCachePath: cachePath})

	regRes, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\n"})
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if !regRes.Capabilities["scheduler"] {
		t.Errorf("expected capability 'scheduler' to be true")
	}
	if !regRes.Capabilities["scheduler_across_priorities"] {
		t.Errorf("expected capability 'scheduler_across_priorities' to be true")
	}

	reconfRes, err := r.Reconfigure(ctx, runtime.ReconfigureRequest{ConfigYAML: "enabled: true\n"})
	if err != nil {
		t.Fatalf("Reconfigure failed: %v", err)
	}
	if !reconfRes.Capabilities["scheduler"] || !reconfRes.Capabilities["scheduler_across_priorities"] {
		t.Errorf("expected capabilities preserved after reconfigure")
	}
}

// 2. Acceptance Criterion 2: Dynamic Model Selection across Quota Pools (Gemini, Argon, Claude)
func TestScheduler_DynamicModelSelection_AcrossQuotaPools(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "cred-gemini-rich", AuthIndex: "idx-gemini", Provider: "antigravity", Status: "active"},
		{Name: "cred-claude-rich", AuthIndex: "idx-claude", Provider: "antigravity", Status: "active"},
		{Name: "cred-argon-rich", AuthIndex: "idx-argon", Provider: "antigravity", Status: "active"},
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}

	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(120 * time.Hour)

	rem90 := int64(90)
	rem0 := int64(0)
	rem30 := int64(30)
	rem95 := int64(95)

	// cred-gemini-rich: Gemini=90%, Claude=0%, Argon=30%
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-gemini", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem90,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-gemini", Provider: core.ProviderAntigravity, ModelGroup: "claude_gpt",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem0,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-gemini", Provider: core.ProviderAntigravity, ModelGroup: "argon",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem30,
	})

	// cred-claude-rich: Gemini=0%, Claude=90%, Argon=0%
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-claude", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem0,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-claude", Provider: core.ProviderAntigravity, ModelGroup: "claude_gpt",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem90,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-claude", Provider: core.ProviderAntigravity, ModelGroup: "argon",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem0,
	})

	// cred-argon-rich: Gemini=30%, Claude=0%, Argon=95%
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-argon", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem30,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-argon", Provider: core.ProviderAntigravity, ModelGroup: "claude_gpt",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem0,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-argon", Provider: core.ProviderAntigravity, ModelGroup: "argon",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem95,
	})

	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "cred-gemini-rich", Provider: "antigravity"},
		{ID: "cred-claude-rich", Provider: "antigravity"},
		{ID: "cred-argon-rich", Provider: "antigravity"},
	}

	// 1. Pick for Gemini model -> cred-gemini-rich has R7d=90% vs Argon-rich's 30%
	geminiReq, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-3.8-flash-high",
		Candidates: candidates,
	})
	geminiRespBytes := r.Handle(ctx, runtime.MethodSchedulerPick, geminiReq)
	var geminiEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(geminiRespBytes, &geminiEnv); err != nil || !geminiEnv.OK {
		t.Fatalf("decode gemini pick: %s", string(geminiRespBytes))
	}
	if !geminiEnv.Result.Handled || geminiEnv.Result.AuthID != "cred-gemini-rich" {
		t.Errorf("expected cred-gemini-rich for Gemini model, got %+v", geminiEnv.Result)
	}

	// 2. Pick for Claude model -> cred-claude-rich has R7d=90% while others are 0%
	claudeReq, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "claude-sonnet-5-5-high",
		Candidates: candidates,
	})
	claudeRespBytes := r.Handle(ctx, runtime.MethodSchedulerPick, claudeReq)
	var claudeEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(claudeRespBytes, &claudeEnv); err != nil || !claudeEnv.OK {
		t.Fatalf("decode claude pick: %s", string(claudeRespBytes))
	}
	if !claudeEnv.Result.Handled || claudeEnv.Result.AuthID != "cred-claude-rich" {
		t.Errorf("expected cred-claude-rich for Claude model, got %+v", claudeEnv.Result)
	}

	// 3. Pick for Argon model -> cred-argon-rich has R7d=95% vs Gemini-rich's 30%
	argonReq, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-4-argon-high",
		Candidates: candidates,
	})
	argonRespBytes := r.Handle(ctx, runtime.MethodSchedulerPick, argonReq)
	var argonEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(argonRespBytes, &argonEnv); err != nil || !argonEnv.OK {
		t.Fatalf("decode argon pick: %s", string(argonRespBytes))
	}
	if !argonEnv.Result.Handled || argonEnv.Result.AuthID != "cred-argon-rich" {
		t.Errorf("expected cred-argon-rich for Argon model, got %+v", argonEnv.Result)
	}
}

// 3. Acceptance Criterion 3: Round-Robin rotation and Session Affinity
func TestScheduler_RoundRobin_And_SessionAffinity(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "account-alpha", AuthIndex: "idx-alpha", Provider: "antigravity", Status: "active"},
		{Name: "account-beta", AuthIndex: "idx-beta", Provider: "antigravity", Status: "active"},
	}

	store, _ := state.Load(ctx, cachePath)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(120 * time.Hour)
	rem90 := int64(90)
	for _, idx := range []string{"idx-alpha", "idx-beta"} {
		_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
			AuthIndex: idx, Provider: core.ProviderAntigravity, ModelGroup: "gemini",
			ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem90,
		})
	}
	_ = store.SaveAtomic(ctx)

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-alpha", Provider: "antigravity"},
		{ID: "account-beta", Provider: "antigravity"},
	}

	// 1. Without session ID: alternate between alpha and beta
	pickOnce := func(sessionID string) string {
		req := runtime.SchedulerPickRequest{
			Model:      "gemini-3.8-flash-high",
			Candidates: candidates,
		}
		if sessionID != "" {
			req.Options = runtime.SchedulerOptions{
				Metadata: map[string]any{"canonical_session_id": sessionID},
			}
		}
		b, _ := json.Marshal(req)
		res := r.Handle(ctx, runtime.MethodSchedulerPick, b)
		var env struct {
			OK     bool                          `json:"ok"`
			Result runtime.SchedulerPickResponse `json:"result"`
		}
		_ = json.Unmarshal(res, &env)
		return env.Result.AuthID
	}

	first := pickOnce("")
	second := pickOnce("")
	third := pickOnce("")
	if first == second || first != third {
		t.Errorf("expected round-robin alternation, got: %s -> %s -> %s", first, second, third)
	}

	// 2. With canonical_session_id: stick to bound credential
	bound1 := pickOnce("sess-user-42")
	for i := range 5 {
		repeat := pickOnce("sess-user-42")
		if repeat != bound1 {
			t.Fatalf("session affinity broken on call %d: got %s, want %s", i, repeat, bound1)
		}
	}
}

// 4. Acceptance Criterion 4: Pool-scoped 429 cooldown isolates only the rate-limited model pool
func TestScheduler_PoolScoped429Cooldown_Isolation(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "account-1", AuthIndex: "idx-1", Provider: "antigravity", Status: "active", Priority: 100},
		{Name: "account-2", AuthIndex: "idx-2", Provider: "antigravity", Status: "active", Priority: 100},
	}

	store, _ := state.Load(ctx, cachePath)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(120 * time.Hour)
	rem90 := int64(90)
	for _, idx := range []string{"idx-1", "idx-2"} {
		for _, grp := range []string{"gemini", "argon", "claude_gpt"} {
			_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
				AuthIndex: idx, Provider: core.ProviderAntigravity, ModelGroup: grp,
				ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem90, LongWindowResetAt: longReset, LongWindowRemaining: &rem90,
			})
		}
	}
	_ = store.SaveAtomic(ctx)

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	// Trigger 429 on account-1 for claude-sonnet-5-5-high
	eventPayload := []byte(`{"Provider":"antigravity","Model":"claude-sonnet-5-5-high","AuthIndex":"idx-1","Failed":true,"Failure":{"StatusCode":429}}`)
	_ = r.Handle(ctx, runtime.MethodUsageHandle, eventPayload)
	_ = r.Handle(ctx, runtime.MethodUsageHandle, eventPayload)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-1", Provider: "antigravity"},
		{ID: "account-2", Provider: "antigravity"},
	}

	pickModel := func(model string) string {
		req, _ := json.Marshal(runtime.SchedulerPickRequest{
			Model:      model,
			Candidates: candidates,
		})
		res := r.Handle(ctx, runtime.MethodSchedulerPick, req)
		var env struct {
			OK     bool                          `json:"ok"`
			Result runtime.SchedulerPickResponse `json:"result"`
		}
		_ = json.Unmarshal(res, &env)
		return env.Result.AuthID
	}

	// For Claude: account-1 is in 429 cooldown, so account-2 MUST be selected
	claudePicked := pickModel("claude-sonnet-5-5-high")
	if claudePicked != "account-2" {
		t.Errorf("expected account-2 for cooling Claude model, got %s", claudePicked)
	}

	// For Gemini and Argon: account-1 MUST NOT be penalized and should participate normally
	geminiPicks := make(map[string]int)
	for range 4 {
		geminiPicks[pickModel("gemini-3.8-flash-high")]++
	}
	if geminiPicks["account-1"] == 0 {
		t.Errorf("account-1 incorrectly cooling for Gemini model: %+v", geminiPicks)
	}

	argonPicks := make(map[string]int)
	for range 4 {
		argonPicks[pickModel("gemini-4-argon-high")]++
	}
	if argonPicks["account-1"] == 0 {
		t.Errorf("account-1 incorrectly cooling for Argon model: %+v", argonPicks)
	}
}

// 5. Acceptance Criterion 5: Non-Antigravity scheduler.pick returns handled: false
func TestScheduler_NonAntigravityRequest_ReturnsHandledFalse(t *testing.T) {
	ctx := context.Background()
	r := runtime.New(runtime.Options{})

	// No antigravity candidates present
	req, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model: "gpt-4o",
		Candidates: []runtime.SchedulerAuthCandidate{
			{ID: "openai-key-1", Provider: "openai"},
			{ID: "anthropic-key-1", Provider: "anthropic"},
		},
	})
	res := r.Handle(ctx, runtime.MethodSchedulerPick, req)
	var env struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(res, &env); err != nil || !env.OK {
		t.Fatalf("decode pick: %s", string(res))
	}
	if env.Result.Handled {
		t.Errorf("expected Handled: false for non-antigravity request")
	}
}

// 6. Acceptance Criterion 6: Failover in Mixed Route vs Antigravity-Only Route
func TestScheduler_DepletedFailover_MixedProvider_And_AntigravityOnly(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "ag-depleted", AuthIndex: "idx-depleted", Provider: "antigravity", Status: "active"},
	}

	store, _ := state.Load(ctx, cachePath)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(120 * time.Hour)
	rem0 := int64(0)
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-depleted", Provider: core.ProviderAntigravity, ModelGroup: "claude_gpt",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem0,
	})
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.SyncHost(ctx, config.AntigravityModelGroupGemini); err != nil {
		t.Fatalf("SyncHost failed: %v", err)
	}

	// Case 1: Mixed route (Antigravity depleted + Vertex & OpenAI candidates)
	mixedReq, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model: "claude-sonnet-5-5-high",
		Candidates: []runtime.SchedulerAuthCandidate{
			{ID: "ag-depleted", Provider: "antigravity", Priority: 100},
			{ID: "vertex-claude", Provider: "vertex", Priority: 50},
			{ID: "anthropic-direct", Provider: "anthropic", Priority: 90},
		},
	})
	mixedRes := r.Handle(ctx, runtime.MethodSchedulerPick, mixedReq)
	var mixedEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(mixedRes, &mixedEnv)
	t.Logf("mixed result: %+v, raw: %s", mixedEnv.Result, string(mixedRes))
	if !mixedEnv.Result.Handled || mixedEnv.Result.AuthID != "anthropic-direct" {
		t.Errorf("expected failover to highest-priority other candidate 'anthropic-direct', got %+v", mixedEnv.Result)
	}

	// Case 2: Antigravity-only route and depleted
	agOnlyReq, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model: "claude-sonnet-5-5-high",
		Candidates: []runtime.SchedulerAuthCandidate{
			{ID: "ag-depleted", Provider: "antigravity", Priority: 100},
		},
	})
	agOnlyRes := r.Handle(ctx, runtime.MethodSchedulerPick, agOnlyReq)
	var agOnlyEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(agOnlyRes, &agOnlyEnv)
	if agOnlyEnv.Result.Handled {
		t.Errorf("expected Handled: false for depleted antigravity-only route, got %+v", agOnlyEnv.Result)
	}
}

// 7. Acceptance Criterion 7: Lazy in-memory transition refresh when reset time passes
func TestScheduler_LazyTransitionRefresh(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "account-resetting", AuthIndex: "idx-reset", Provider: "antigravity", Status: "active"},
	}

	store, _ := state.Load(ctx, cachePath)
	shortReset := now.Add(10 * time.Minute)
	longReset := now.Add(120 * time.Hour)
	rem0 := int64(0)
	rem90 := int64(90)

	// Soft-depleted: R5h=0, but R7d=90%
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-reset", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem0, LongWindowResetAt: longReset, LongWindowRemaining: &rem90,
	})
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-resetting", Provider: "antigravity"},
	}

	// 1. Before reset time -> account is soft-depleted (-1), Handled: false
	req, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-3.8-flash-high",
		Candidates: candidates,
	})
	beforeRes := r.Handle(ctx, runtime.MethodSchedulerPick, req)
	var beforeEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(beforeRes, &beforeEnv)
	if beforeEnv.Result.Handled {
		t.Errorf("expected soft-depleted account to not be picked before reset, got %+v", beforeEnv.Result)
	}

	// 2. Advance time past 10 minutes reset deadline
	clock.Advance(11 * time.Minute)

	// 3. Lazy transition automatically recognizes reset has passed and promotes candidate without HTTP probe!
	afterRes := r.Handle(ctx, runtime.MethodSchedulerPick, req)
	var afterEnv struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(afterRes, &afterEnv)
	if !afterEnv.Result.Handled || afterEnv.Result.AuthID != "account-resetting" {
		t.Errorf("expected lazy refresh to recover account after reset time, got %+v", afterEnv.Result)
	}
}

// 8. Acceptance Criterion 8: Real-time token usage deduction updates routing immediately
func TestScheduler_UsageDeduction_RealTimeUpdate(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "account-A", AuthIndex: "idx-A", Provider: "antigravity", Status: "active"},
		{Name: "account-B", AuthIndex: "idx-B", Provider: "antigravity", Status: "active"},
	}

	store, _ := state.Load(ctx, cachePath)
	shortReset := now.Add(4 * time.Hour)
	longReset := now.Add(120 * time.Hour)
	rem80 := int64(80)
	rem79 := int64(79)

	// A: 80%, B: 79% (both 5h and 7d)
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-A", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem80, LongWindowResetAt: longReset, LongWindowRemaining: &rem80,
	})
	_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "idx-B", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: shortReset, ShortWindowRemaining: &rem79, LongWindowResetAt: longReset, LongWindowRemaining: &rem79,
	})

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nurgency_tolerance: 0.0\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-A", Provider: "antigravity"},
		{ID: "account-B", Provider: "antigravity"},
	}

	req, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-3.8-flash-high",
		Candidates: candidates,
	})

	// 1. Initial pick selects account-A (80% remaining beats 79%)
	pick1 := r.Handle(ctx, runtime.MethodSchedulerPick, req)
	var env1 struct {
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(pick1, &env1)
	if env1.Result.AuthID != "account-A" {
		t.Fatalf("expected account-A initial pick, got %s", env1.Result.AuthID)
	}

	// 2. Report massive usage on account-A (1,600,000 weighted tokens -> burns ~20% of 5h window)
	usagePayload := []byte(`{
		"Provider": "antigravity",
		"Model": "gemini-3.8-flash-high",
		"AuthIndex": "idx-A",
		"Failed": false,
		"Detail": {
			"InputTokens": 400000,
			"OutputTokens": 300000,
			"ReasoningTokens": 0,
			"TotalTokens": 700000
		}
	}`)
	usageRes := r.Handle(ctx, runtime.MethodUsageHandle, usagePayload)
	var usageEnv struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(usageRes, &usageEnv); err != nil || !usageEnv.OK {
		t.Fatalf("usage report failed: %s", string(usageRes))
	}

	// 3. Next pick immediately chooses account-B because account-A has burned quota in real-time
	pick2 := r.Handle(ctx, runtime.MethodSchedulerPick, req)
	var env2 struct {
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	_ = json.Unmarshal(pick2, &env2)
	if env2.Result.AuthID != "account-B" {
		t.Errorf("expected account-B to be chosen after account-A real-time deduction, got %s", env2.Result.AuthID)
	}
}

// 9. Acceptance Criterion 9: Zero host.* C ABI callbacks during scheduler.pick execution
func TestScheduler_ZeroHostCallbacksDuringPick(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := &callbackTrackingHost{
		mockHost: *newMockHost(),
	}
	mock.files = []host.AuthFile{
		{Name: "account-1", AuthIndex: "idx-1", Provider: "antigravity", Status: "active"},
	}

	r := runtime.New(runtime.Options{Host: mock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)

	// Reset host call counter
	atomic.StoreInt64(&mock.callCount, 0)

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-1", Provider: "antigravity"},
	}
	req, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-3.8-flash-high",
		Candidates: candidates,
	})

	// Execute 100 pick requests in pure memory
	for i := range 100 {
		res := r.Handle(ctx, runtime.MethodSchedulerPick, req)
		if !strings.Contains(string(res), `"handled":true`) {
			t.Fatalf("call %d failed: %s", i, string(res))
		}
	}
	callsMade := atomic.LoadInt64(&mock.callCount)
	if callsMade != 0 {
		t.Errorf("expected 0 host.* callbacks during scheduler.pick, but %d were recorded", callsMade)
	}
}

func TestScheduler_CustomTier_PrioritizesTier1_And_BalancesTier2(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "account-a", AuthIndex: "idx-a", Provider: "antigravity", Status: "active"},
		{Name: "account-b", AuthIndex: "idx-b", Provider: "antigravity", Status: "active"},
		{Name: "account-c", AuthIndex: "idx-c", Provider: "antigravity", Status: "active"},
	}

	r := runtime.New(runtime.Options{Host: mock, StateCachePath: cachePath, Clock: clock})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	_, _ = r.SyncHost(ctx, config.AntigravityModelGroupGemini)


	// Setup healthy quotas for all three in state store
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	rem := int64(80)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(72 * time.Hour)
	for _, idx := range []string{"idx-a", "idx-b", "idx-c"} {
		_ = store.MarkProbeSuccess(ctx, state.ProbeSuccess{
			AuthIndex:            idx,
			Provider:             core.ProviderAntigravity,
			ModelGroup:           "gemini",
			ObservedAt:           now,
			ResetAt:              reset7d,
			Remaining:            rem,
			ShortWindowRemaining: &rem,
			ShortWindowResetAt:   reset5h,
			LongWindowRemaining:  &rem,
			LongWindowResetAt:    reset7d,
			PlanType:             core.PlanTypePro,
		})
	}
	_ = store.SaveAtomic(ctx)

	// Configure Custom Tiers: c: 1, a: 2, b: 2
	if err := r.SetCustomTier(ctx, "idx-c", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-a", 2); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-b", 2); err != nil {
		t.Fatal(err)
	}

	candidates := []runtime.SchedulerAuthCandidate{
		{ID: "account-a", Provider: "antigravity"},
		{ID: "account-b", Provider: "antigravity"},
		{ID: "account-c", Provider: "antigravity"},
	}
	req, _ := json.Marshal(runtime.SchedulerPickRequest{
		Model:      "gemini-3.8-flash-high",
		Candidates: candidates,
	})

	// 1. With all 3 healthy, Account C (Tier 1) MUST always win
	for i := range 5 {
		resBytes := r.Handle(ctx, runtime.MethodSchedulerPick, req)
		var env struct {
			OK     bool                          `json:"ok"`
			Result runtime.SchedulerPickResponse `json:"result"`
		}
		if err := json.Unmarshal(resBytes, &env); err != nil || !env.OK {
			t.Fatalf("call %d unmarshal failed: %s", i, string(resBytes))
		}
		if env.Result.AuthID != "account-c" {
			t.Fatalf("call %d expected account-c (Tier 1), got %s", i, env.Result.AuthID)
		}
	}

	// 2. Put Account C into 429 rate limit cooldown (temporary bypass)
	store, _ = state.Load(ctx, cachePath)
	store.SetCooldown(state.CooldownEntry{
		AuthIndex:     "idx-c",
		ModelGroup:    "gemini",
		TriggeredAt:   now,
		CooldownUntil: now.Add(5 * time.Minute),
		Reason:        "429 cooldown",
	})
	_ = store.SaveAtomic(ctx)

	// Advance time by 65s (>60s routing table refresh TTL, but well within 5min cooldown)
	clock.now = now.Add(65 * time.Second)
	// 3. Requests must now rotate between Account A and Account B (both Tier 2, load balancing)
	picks := make([]string, 4)
	for i := range 4 {
		resBytes := r.Handle(ctx, runtime.MethodSchedulerPick, req)
		var env struct {
			OK     bool                          `json:"ok"`
			Result runtime.SchedulerPickResponse `json:"result"`
		}
		if err := json.Unmarshal(resBytes, &env); err != nil || !env.OK {
			t.Fatalf("call %d unmarshal failed: %s", i, string(resBytes))
		}
		picks[i] = env.Result.AuthID
	}

	// Expected round-robin alternation between account-a and account-b
	if picks[0] == picks[1] || picks[1] == picks[2] || picks[2] == picks[3] {
		t.Errorf("expected alternating load balancing between account-a and account-b, got %v", picks)
	}
	hasA := false
	hasB := false
	for _, p := range picks {
		if p == "account-a" {
			hasA = true
		}
		if p == "account-b" {
			hasB = true
		}
	}
	if !hasA || !hasB {
		t.Errorf("expected both account-a and account-b to participate in load balancing, got %v", picks)
	}
}

func TestScheduler_UpdateModelInventory_FiltersNonAntigravityModels(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{Name: "ag-account", AuthIndex: "idx-ag", Provider: "antigravity", Status: "active"},
		{Name: "vx-account", AuthIndex: "idx-vx", Provider: "vertex", Status: "active"},
		{Name: "cx-account", AuthIndex: "idx-cx", Provider: "codex", Status: "active"},
	}
	mock.modelsEntries = []host.AuthModelsEntry{
		{
			AuthIndex: "idx-ag",
			Provider:  "antigravity",
			Models:    []string{"gemini-3.8-flash-high", "gemini-4-argon-high", "claude-sonnet-5-5-high"},
		},
		{
			AuthIndex: "idx-vx",
			Provider:  "vertex",
			Models:    []string{"gemini-omni-1.2-flash", "gemini-4-argon"},
		},
		{
			AuthIndex: "idx-cx",
			Provider:  "codex",
			Models:    []string{"gpt-6-chat-thinking", "gpt-image-2.5"},
		},
	}

	r := runtime.New(runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(ctx, runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}

	snap, err := r.SyncHost(ctx, config.AntigravityModelGroupGemini)
	if err != nil {
		t.Fatalf("SyncHost failed: %v", err)
	}

	// 1. Verify models returned in snapshot contain only Antigravity models
	forbiddenModels := []string{
		"gemini-omni-1.2-flash",
		"gemini-4-argon",
		"gpt-6-chat-thinking",
		"gpt-image-2.5",
	}
	for _, m := range snap.Models {
		for _, forbidden := range forbiddenModels {
			if m.ID == forbidden {
				t.Errorf("snap.Models contains non-antigravity model %q from another provider", forbidden)
			}
		}
	}

	// 2. Verify all models in snap.Models are indeed Antigravity models
	agMap := make(map[string]bool)
	for _, m := range snap.Models {
		agMap[m.ID] = true
	}
	expectedAgModels := []string{"gemini-3.8-flash-high", "gemini-4-argon-high", "claude-sonnet-5-5-high"}
	for _, expected := range expectedAgModels {
		if !agMap[expected] {
			t.Errorf("snap.Models missing expected Antigravity model %q", expected)
		}
	}
}
