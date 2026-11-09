package runtime

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/evidence"
)

func TestGenerateNaturalWarmupPrompt(t *testing.T) {
	// Test that generated prompts are natural, non-empty, and vary across iterations
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
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

	warmupCount := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
		if strings.Contains(req.URL, "generateContent") {
			warmupCount++
			return host.HTTPResponse{StatusCode: http.StatusOK}, nil
		}
		return host.HTTPResponse{StatusCode: http.StatusOK}, nil
	})

	cfg := config.WarmupConfig{
		GeminiEnabled:  true,
		ClaudeEnabled:  true,
		GeminiModel:    "gemini-3-flash",
		ClaudeModel:    "claude-sonnet-5-5-high",
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
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, now)
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
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, now.Add(10*time.Minute))
	if warmupCount != 2 {
		t.Fatalf("expected still 2 warmup calls due to debounce, got %d", warmupCount)
	}

	// 3. After debounce expiration (5 hours later): should trigger again
	rt.triggerWarmupForEligibleCredentials(ctx, doer, cfg, evidenceByGroup, materials, now.Add(5*time.Hour))
	if warmupCount != 4 {
		t.Fatalf("expected 4 warmup calls after debounce expired, got %d", warmupCount)
	}
}
