package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

func TestGenerateNaturalWarmupPrompt(t *testing.T) {
	// Test that generated prompts are natural, non-empty, and vary across iterations
	seen := make(map[string]bool)
	for range 20 {
		p := GenerateNaturalWarmupPrompt()
		if len(p) < 20 {
			t.Fatalf("expected substantial natural prompt, got %q", p)
		}
		seen[p] = true
	}
	if len(seen) < 15 {
		t.Fatalf("expected high diversity among 20 generated prompts, got %d unique", len(seen))
	}

	// RenderWarmupPrompt wrapper
	p2 := RenderWarmupPrompt(config.DefaultWarmupConfig(), "")
	if len(p2) == 0 {
		t.Fatal("expected non-empty RenderWarmupPrompt output")
	}
}

func TestIsWarmupEligible(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	remaining100 := int64(100)
	remaining80 := int64(80)

	resetIn5h := now.Add(5 * time.Hour)
	resetIn2h := now.Add(2 * time.Hour)
	resetInPast := now.Add(-10 * time.Minute)

	// 1. Full quota, unstarted 5h countdown -> Eligible
	evFullUnstarted := &priority.QuotaEvidence{
		ShortWindowRemaining: &remaining100,
		ShortWindowResetAt:   &resetIn5h,
	}
	if !isWarmupEligible(evFullUnstarted, now, time.Time{}) {
		t.Fatal("expected eligible for full quota and unstarted reset time")
	}

	// 2. Full quota, but in debounce cooldown (warmed up 1 hour ago) -> Not eligible
	recentWarmup := now.Add(-1 * time.Hour)
	if isWarmupEligible(evFullUnstarted, now, recentWarmup) {
		t.Fatal("expected ineligible due to debounce cooldown")
	}

	// 3. Debounce cooldown expired (warmed up 5 hours ago) -> Eligible
	oldWarmup := now.Add(-5 * time.Hour)
	if !isWarmupEligible(evFullUnstarted, now, oldWarmup) {
		t.Fatal("expected eligible when debounce cooldown expired")
	}

	// 4. Quota depleted / not full -> Not eligible
	evPartial := &priority.QuotaEvidence{
		ShortWindowRemaining: &remaining80,
		ShortWindowResetAt:   &resetIn5h,
	}
	if isWarmupEligible(evPartial, now, time.Time{}) {
		t.Fatal("expected ineligible when quota is not full")
	}

	// 5. Countdown is already midway through (e.g. 2 hours remaining) -> Not eligible
	evMidway := &priority.QuotaEvidence{
		ShortWindowRemaining: &remaining100,
		ShortWindowResetAt:   &resetIn2h,
	}
	if isWarmupEligible(evMidway, now, time.Time{}) {
		t.Fatal("expected ineligible when 5h countdown is already halfway running")
	}

	// 6. Reset time is in the past (previous window elapsed and returned to idle) -> Eligible
	evPastReset := &priority.QuotaEvidence{
		ShortWindowRemaining: &remaining100,
		ShortWindowResetAt:   &resetInPast,
	}
	if !isWarmupEligible(evPastReset, now, time.Time{}) {
		t.Fatal("expected eligible when previous window elapsed into past")
	}

	// 7. Nil evidence -> Ineligible
	if isWarmupEligible(nil, now, time.Time{}) {
		t.Fatal("expected ineligible for nil evidence")
	}
}

func TestExecuteAntigravityWarmup(t *testing.T) {
	ctx := context.Background()

	// 1. Success on first try
	var calledURLs []string
	doerSuccess := quotaHTTPDoerFunc(func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
		calledURLs = append(calledURLs, req.URL)
		return host.HTTPResponse{StatusCode: http.StatusOK}, nil
	})

	res, err := executeAntigravityWarmup(ctx, doerSuccess, "auth-1", "token-1", "proj-1", "gemini-3-flash", "Hello")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got code=%d err=%v", res.StatusCode, err)
	}
	if len(calledURLs) != 1 || calledURLs[0] != antigravityWarmupURLs[0] {
		t.Fatalf("unexpected called URLs: %v", calledURLs)
	}

	// 2. 403 Forbidden with fallback to legacy user agent
	var calledUserAgents []string
	doerFallback := quotaHTTPDoerFunc(func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
		ua := req.Headers.Get("User-Agent")
		calledUserAgents = append(calledUserAgents, ua)
		if ua == antigravityLegacyHubUserAgent {
			return host.HTTPResponse{StatusCode: http.StatusOK}, nil
		}
		return host.HTTPResponse{StatusCode: http.StatusForbidden}, nil
	})

	res, err = executeAntigravityWarmup(ctx, doerFallback, "auth-1", "token-1", "proj-1", "gemini-3-flash", "Hello")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("expected fallback 200 OK, got code=%d err=%v", res.StatusCode, err)
	}
	if len(calledUserAgents) < 2 || calledUserAgents[1] != antigravityLegacyHubUserAgent {
		t.Fatalf("expected legacy user agent fallback, got: %v", calledUserAgents)
	}
}

func TestRuntime_TriggerWarmupForEligibleCredentials(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt := New(Options{})
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "cache.json"))
	if err != nil {
		t.Fatalf("load state store failed: %v", err)
	}

	warmupCount := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
		if strings.Contains(req.URL, "generateContent") {
			warmupCount++
			return host.HTTPResponse{StatusCode: http.StatusOK}, nil
		}
		return host.HTTPResponse{StatusCode: http.StatusOK}, nil
	})

	cfg := config.WarmupConfig{
		GeminiEnabled: true,
		ClaudeEnabled: true,
		GeminiModel:   "gemini-3-flash",
		ClaudeModel:   "claude-sonnet-5-5-high",
	}
	remaining100 := int64(100)
	resetIn5h := now.Add(5 * time.Hour)

	evidenceByGroup := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {
			Eligible: []priority.QuotaEvidence{
				{
					AuthIndex:            "auth-gemini-1",
					ShortWindowRemaining: &remaining100,
					ShortWindowResetAt:   &resetIn5h,
				},
			},
		},
		config.AntigravityModelGroupClaudeGPT: {
			Eligible: []priority.QuotaEvidence{
				{
					AuthIndex:            "auth-claude-1",
					ShortWindowRemaining: &remaining100,
					ShortWindowResetAt:   &resetIn5h,
				},
			},
		},
	}

	materials := map[string]authMaterial{
		"auth-gemini-1": {accessToken: "token-g", projectID: "proj-g"},
		"auth-claude-1": {accessToken: "token-c", projectID: "proj-c"},
	}

	// 1. Initial run: both should be warmed up
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, nil, store, now)
	if warmupCount != 2 {
		t.Fatalf("expected 2 warmup calls (gemini and claude), got %d", warmupCount)
	}
	history, err := rt.GetRequestHistory(ctx)
	if err != nil || len(history) < 2 {
		t.Fatalf("expected at least 2 request history entries, got %d err=%v", len(history), err)
	}
	for _, entry := range history {
		if strings.Contains(entry.Email, "@") {
			t.Fatalf("expected masked email, got unmasked: %s", entry.Email)
		}
	}

	// 2. Immediate second run: debounced, no calls should happen
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, nil, store, now.Add(10*time.Minute))
	if warmupCount != 2 {
		t.Fatalf("expected still 2 warmup calls due to debounce, got %d", warmupCount)
	}

	// 3. After debounce expiration (5 hours later): should trigger again
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, nil, store, now.Add(5*time.Hour))
	if warmupCount != 4 {
		t.Fatalf("expected 4 warmup calls after debounce expired, got %d", warmupCount)
	}

	// 4. Disabled credentials must never be warmed up
	disabledAuth := map[string]struct{}{
		"auth-gemini-1": {},
		"auth-claude-1": {},
	}
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, disabledAuth, store, now.Add(11*time.Hour))
	if warmupCount != 4 {
		t.Fatalf("expected warmup calls to remain 4 for disabled credentials, got %d", warmupCount)
	}
}

func warmupQuotaBody(reset5h, reset7d time.Time) []byte {
	return fmt.Appendf(nil,
		`{"models":{"gemini-2.5-pro":{"quotaInfo":{"windows":[{"name":"5h","remainingFraction":1.0,"resetTime":%q}]}},`+
			`"gemini-2.5-flash":{"quotaInfo":{"windows":[{"name":"7d","remainingFraction":1.0,"resetTime":%q}]}}}}`,
		reset5h.UTC().Format(time.RFC3339),
		reset7d.UTC().Format(time.RFC3339),
	)
}

// warmupStubHost is the minimal host surface the warm-up sweep needs: auth
// inventory, auth material, and a single canned quota response.
type warmupStubHost struct {
	mu        sync.Mutex
	files     []host.AuthFile
	body      []byte
	httpCalls []host.HTTPRequest
}

func (h *warmupStubHost) ListAuthFiles(context.Context) ([]host.AuthFile, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]host.AuthFile(nil), h.files...), nil
}

func (h *warmupStubHost) GetAuth(_ context.Context, authIndex string) (host.AuthDocument, error) {
	return host.AuthDocument{
		AuthIndex: authIndex,
		JSON:      json.RawMessage(`{"access_token":"test-token","project_id":"test-project","account":"test@example.com"}`),
	}, nil
}

func (h *warmupStubHost) GetRuntime(_ context.Context, authIndex string) (host.RuntimeAuth, error) {
	return host.RuntimeAuth{AuthIndex: authIndex}, nil
}

func (h *warmupStubHost) SaveAuth(context.Context, string, json.RawMessage) error { return nil }

func (h *warmupStubHost) HTTPDo(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.httpCalls = append(h.httpCalls, request)
	return host.HTTPResponse{StatusCode: http.StatusOK, Body: h.body}, nil
}

func (h *warmupStubHost) ListAuthModels(context.Context) ([]host.AuthModelsEntry, error) {
	return nil, nil
}

func (h *warmupStubHost) setBody(body []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.body = body
}

func (h *warmupStubHost) addFile(file host.AuthFile) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.files = append(h.files, file)
}

func (h *warmupStubHost) countCalls(needle string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	count := 0
	for _, request := range h.httpCalls {
		if strings.Contains(request.URL, needle) {
			count++
		}
	}
	return count
}

func (h *warmupStubHost) quotaAuthIndexes() map[string]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	indexes := make(map[string]struct{})
	for _, request := range h.httpCalls {
		if strings.Contains(request.URL, "retrieveUserQuota") {
			indexes[request.AuthIndex] = struct{}{}
		}
	}
	return indexes
}

type warmupTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *warmupTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *warmupTestClock) advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta)
}

// seedReadyWindow stores the exact evidence shape of a card showing "Ready":
// the recorded 5h reset time already elapsed while the quota still reads full.
func seedReadyWindow(t *testing.T, cachePath string, now time.Time) {
	t.Helper()
	seedWindow(t, cachePath, now, now.Add(-2*time.Hour), now.Add(-30*time.Minute), 100)
}

func seedWindow(t *testing.T, cachePath string, now, observedAt, shortReset time.Time, remaining int64) {
	t.Helper()
	seedGroupWindow(t, cachePath, now, config.AntigravityModelGroupGemini, observedAt, shortReset, remaining)
}

func seedGroupWindow(t *testing.T, cachePath string, now time.Time, group config.AntigravityModelGroup, observedAt, shortReset time.Time, remaining int64) {
	t.Helper()
	ctx := context.Background()
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	longReset := now.Add(7 * 24 * time.Hour)
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            "idx-a",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           string(group),
		ObservedAt:           observedAt,
		ResetAt:              longReset,
		Remaining:            remaining,
		ShortWindowRemaining: &remaining,
		ShortWindowResetAt:   shortReset,
		LongWindowRemaining:  &remaining,
		LongWindowResetAt:    longReset,
		PlanType:             core.PlanTypePro,
	}); err != nil {
		t.Fatalf("seed ready window failed: %v", err)
	}
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatalf("save seeded evidence failed: %v", err)
	}
}

func newWarmupSweepRuntime(t *testing.T, now time.Time) (*Runtime, *warmupStubHost, *warmupTestClock, string) {
	t.Helper()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	stub := &warmupStubHost{
		files: []host.AuthFile{{Name: "account-a", AuthIndex: "idx-a", Provider: "antigravity", Status: "active"}},
		body:  warmupQuotaBody(now.Add(5*time.Hour), now.Add(7*24*time.Hour)),
	}
	clock := &warmupTestClock{now: now}
	rt := New(Options{Host: stub, StateCachePath: cachePath, Clock: clock, DisableBackgroundWorkers: true})
	t.Cleanup(func() { _ = rt.Shutdown(context.Background()) })
	if _, err := rt.Register(context.Background(), RegisterRequest{ConfigYAML: "enabled: true"}); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	return rt, stub, clock, cachePath
}

func enableGeminiWarmup(t *testing.T, rt *Runtime) {
	t.Helper()
	dyn := config.Default().Dynamic()
	dyn.Warmup = config.WarmupConfig{GeminiEnabled: true, GeminiModel: "gemini-3-flash"}
	if err := rt.SetDynamicConfig(context.Background(), dyn); err != nil {
		t.Fatalf("enable warm-up failed: %v", err)
	}
}

func TestRuntime_WarmupSweepWarmsReadyWindowWithoutSchedulingRun(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, _, cachePath := newWarmupSweepRuntime(t, now)
	enableGeminiWarmup(t, rt)
	seedReadyWindow(t, cachePath, now)

	rt.sweepWarmupResets(ctx)

	if got := stub.countCalls("generateContent"); got != 1 {
		t.Fatalf("expected exactly 1 warm-up without a scheduling run, got %d", got)
	}
	persisted, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload state failed: %v", err)
	}
	if persisted.GetLastWarmupAt("idx-a", string(config.AntigravityModelGroupGemini)).IsZero() {
		t.Fatal("expected the warm-up timestamp to be persisted")
	}

	rt.sweepWarmupResets(ctx)
	if got := stub.countCalls("generateContent"); got != 1 {
		t.Fatalf("expected the debounce window to block a second warm-up, got %d", got)
	}
}

func TestRuntime_ResetSweepProbesOnlyTheResetCredentialWithoutWarmup(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, _, cachePath := newWarmupSweepRuntime(t, now)
	stub.addFile(host.AuthFile{Name: "account-b", AuthIndex: "idx-b", Provider: "antigravity", Status: "active"})
	seedReadyWindow(t, cachePath, now)

	rt.sweepWarmupResets(ctx)

	if got := stub.countCalls("generateContent"); got != 0 {
		t.Fatalf("warm-up is disabled; expected no warm-up request, got %d", got)
	}
	probed := stub.quotaAuthIndexes()
	if _, ok := probed["idx-a"]; !ok {
		t.Fatalf("reset refresh must probe the credential whose 5h reset elapsed; probed %v", probed)
	}
	if _, ok := probed["idx-b"]; ok {
		t.Fatalf("reset refresh must not probe credentials without an elapsed 5h reset; probed %v", probed)
	}
	quotaCalls := stub.countCalls("retrieveUserQuota")
	persisted, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload state failed: %v", err)
	}
	entry, ok := persisted.GetEntry("idx-a", string(config.AntigravityModelGroupGemini))
	if !ok {
		t.Fatal("expected the refreshed credential entry")
	}
	if !entry.ShortWindowResetAt.After(now) {
		t.Fatalf("expected the refresh to persist the rolled 5h reset, got %v", entry.ShortWindowResetAt)
	}

	rt.sweepWarmupResets(ctx)
	if got := stub.countCalls("retrieveUserQuota"); got != quotaCalls {
		t.Fatalf("expected one refresh per reset boundary, quota requests grew from %d to %d", quotaCalls, got)
	}
}

func TestRuntime_ResetSweepProbesCredentialOnceWhenBothPoolsExpire(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, _, cachePath := newWarmupSweepRuntime(t, now)
	shortReset := now.Add(-30 * time.Minute)
	seedGroupWindow(t, cachePath, now, config.AntigravityModelGroupGemini, now.Add(-2*time.Hour), shortReset, 100)
	seedGroupWindow(t, cachePath, now, config.AntigravityModelGroupClaudeGPT, now.Add(-2*time.Hour), shortReset, 100)
	stub.setBody(fmt.Appendf(nil,
		`{"models":{"gemini-2.5-pro":{"quotaInfo":{"windows":[{"name":"5h","remainingFraction":1.0,"resetTime":%q},{"name":"7d","remainingFraction":1.0,"resetTime":%q}]}},`+
			`"claude-3-5-sonnet":{"quotaInfo":{"windows":[{"name":"5h","remainingFraction":1.0,"resetTime":%q},{"name":"7d","remainingFraction":1.0,"resetTime":%q}]}}}}`,
		now.Add(5*time.Hour).UTC().Format(time.RFC3339),
		now.Add(7*24*time.Hour).UTC().Format(time.RFC3339),
		now.Add(5*time.Hour).UTC().Format(time.RFC3339),
		now.Add(7*24*time.Hour).UTC().Format(time.RFC3339),
	))

	rt.sweepWarmupResets(ctx)

	if got := stub.countCalls("retrieveUserQuota"); got != 1 {
		t.Fatalf("expected one targeted probe for a credential with both 5h pools due, got %d quota requests", got)
	}
	if got := stub.countCalls("generateContent"); got != 0 {
		t.Fatalf("warm-up is disabled; expected no warm-up request, got %d", got)
	}
}

func TestRuntime_ResetSweepWakesAtResetGraceDeadline(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, _, cachePath := newWarmupSweepRuntime(t, now)
	seedWindow(t, cachePath, now, now, now.Add(20*time.Second), 100)

	wait := rt.sweepWarmupResets(ctx)

	if want := 20*time.Second + WarmupResetGraceBaseDelay; wait != want {
		t.Fatalf("expected the watcher to wake at the reset grace deadline (%v), got %v", want, wait)
	}
	if got := stub.countCalls("retrieveUserQuota"); got != 0 {
		t.Fatalf("expected no probe before the reset boundary, got %d", got)
	}
}

func TestRuntime_WarmupSweepBoundsRetriesWhenWindowDoesNotConverge(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, clock, cachePath := newWarmupSweepRuntime(t, now)
	enableGeminiWarmup(t, rt)
	seedReadyWindow(t, cachePath, now)

	// Upstream keeps reporting an exhausted 5h window whose reset already passed,
	// so the warm-up cannot land until the window actually converges.
	stub.setBody(fmt.Appendf(nil,
		`{"models":{"gemini-2.5-pro":{"quotaInfo":{"windows":[{"name":"5h","remainingFraction":0.0,"resetTime":%q}]}},`+
			`"gemini-2.5-flash":{"quotaInfo":{"windows":[{"name":"7d","remainingFraction":1.0,"resetTime":%q}]}},`+
			`"gemini-argon":{"quotaInfo":{"windows":[{"name":"7d","remainingFraction":1.0,"resetTime":%q}]}},`+
			`"claude-3-5-sonnet":{"quotaInfo":{"windows":[{"name":"5h","remainingFraction":1.0,"resetTime":%q}]}}}}`,
		now.Add(-30*time.Minute).UTC().Format(time.RFC3339),
		now.Add(7*24*time.Hour).UTC().Format(time.RFC3339),
		now.Add(7*24*time.Hour).UTC().Format(time.RFC3339),
		now.Add(5*time.Hour).UTC().Format(time.RFC3339),
	))

	for range 7 {
		rt.sweepWarmupResets(ctx)
		clock.advance(20 * time.Minute)
	}

	if got := stub.countCalls("generateContent"); got != 0 {
		t.Fatalf("expected no warm-up while the window is not full, got %d", got)
	}
	if got := stub.countCalls("retrieveUserQuota"); got != warmupResetMaxAttempts {
		t.Fatalf("expected retries to stop after %d probes, got %d", warmupResetMaxAttempts, got)
	}
}
