package runtime

import (
	"context"
	"encoding/json"
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
	"antigravity-priority/internal/provider/antigravity"
	"antigravity-priority/internal/state"
)

type probeEvidenceMockHost struct {
	mu           sync.Mutex
	httpResponse host.HTTPResponse
	httpCalls    []host.HTTPRequest
	doFn         func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error)
}

func (m *probeEvidenceMockHost) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	return nil, nil
}

func (m *probeEvidenceMockHost) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	return host.AuthDocument{
		AuthIndex: authIndex,
		JSON:      json.RawMessage(`{"access_token":"secret-token","project_id":"mock-project"}`),
	}, nil
}

func (m *probeEvidenceMockHost) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	return host.RuntimeAuth{AuthIndex: authIndex}, nil
}

func (m *probeEvidenceMockHost) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	return nil
}

func (m *probeEvidenceMockHost) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	m.mu.Lock()
	m.httpCalls = append(m.httpCalls, req)
	doFn := m.doFn
	resp := m.httpResponse
	m.mu.Unlock()

	if doFn != nil {
		return doFn(ctx, req)
	}
	return resp, nil
}

func TestProbeAndRecord_GeminiTargetKeepsDualWindowEvidence(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"groups": [
					{"displayName": "Gemini Models", "buckets": [
						{"window": "5h", "remainingFraction": 0.85, "resetTime": "2026-08-21T17:00:00Z"},
						{"window": "weekly", "remainingFraction": 0.62, "resetTime": "2026-08-28T00:00:00Z"}
					]},
					{"displayName": "Claude and GPT models", "buckets": [
						{"window": "5h", "remainingFraction": 0.4, "resetTime": "2026-08-21T16:00:00Z"}
					]}
				]
			}`),
		},
	}

	client := host.NewClient(mock)
	store, err := state.Load(context.Background(), filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}

	job := probeJob{
		credential: core.Credential{
			AuthIndex: "auth-gemini-1",
			Provider:  core.ProviderAntigravity,
		},
		authMaterial: authMaterial{
			accessToken: "secret-token",
			projectID:   "test-project",
		},
		targetGroup: antigravity.ModelGroupGemini,
	}

	results, err := probeAndRecord(context.Background(), client, store, job, now, 10, "round-1")
	if err != nil {
		t.Fatalf("probeAndRecord failed: %v", err)
	}

	mock.mu.Lock()
	calls := len(mock.httpCalls)
	var calledURLs []string
	for _, call := range mock.httpCalls {
		calledURLs = append(calledURLs, call.URL)
	}
	mock.mu.Unlock()

	if calls != 1 {
		t.Fatalf("HTTP calls = %d; want 1 quota summary request", calls)
	}
	if calledURLs[0] != antigravity.RetrieveUserQuotaSummaryURL {
		t.Fatalf("expected call to %q, got %q", antigravity.RetrieveUserQuotaSummaryURL, calledURLs[0])
	}

	if len(results) != 2 {
		t.Fatalf("results len = %d; want both model groups from one response", len(results))
	}
	var gemini antigravity.ProbeResult
	for _, result := range results {
		if result.ModelGroup == antigravity.ModelGroupGemini {
			gemini = result
		}
	}
	if gemini.Status != antigravity.StatusReady {
		t.Fatalf("unexpected probe result: %#v", gemini)
	}
	if gemini.ShortWindowResetAt == nil || gemini.LongWindowResetAt == nil {
		t.Fatalf("dual-window evidence lost: short=%v long=%v", gemini.ShortWindowResetAt, gemini.LongWindowResetAt)
	}
	if gemini.PlanType != core.PlanTypePro {
		t.Fatalf("plan type = %q; want %q", gemini.PlanType, core.PlanTypePro)
	}
	if gemini.Remaining == nil || *gemini.Remaining != 85 {
		t.Fatalf("unexpected remaining: %v", gemini.Remaining)
	}

	// Verify Gemini quota is recorded and fresh in state store
	needsProbe, err := store.NeedsProbe(context.Background(), state.ProbeCheck{
		AuthIndex:  "auth-gemini-1",
		Provider:   core.ProviderAntigravity,
		ModelGroup: string(antigravity.ModelGroupGemini),
		Now:        now,
		Policy:     probePolicy(15 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	if needsProbe {
		t.Fatalf("store still needs probe for Gemini after successful probe")
	}
}

func TestCollectFreshEvidence_GeminiControlGroupUsesOneSummaryCallPerAccount(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"groups": [
					{"displayName": "Gemini Models", "buckets": [
						{"window": "5h", "remainingFraction": 0.9, "resetTime": "2026-08-21T17:00:00Z"},
						{"window": "weekly", "remainingFraction": 0.7, "resetTime": "2026-08-28T00:00:00Z"}
					]},
					{"displayName": "Claude and GPT models", "buckets": [
						{"window": "5h", "remainingFraction": 0.6, "resetTime": "2026-08-21T17:00:00Z"}
					]}
				]
			}`),
		},
	}

	client := host.NewClient(mock)
	store, err := state.Load(context.Background(), filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}

	credentials := []core.Credential{
		{AuthIndex: "auth-1", Provider: core.ProviderAntigravity},
		{AuthIndex: "auth-2", Provider: core.ProviderAntigravity},
	}
	authMaterials := map[string]authMaterial{
		"auth-1": {accessToken: "tok-1", projectID: "proj-1"},
		"auth-2": {accessToken: "tok-2", projectID: "proj-2"},
	}

	collected, err := collectFreshEvidence(context.Background(), collectInput{
		client:         client,
		store:          store,
		credentials:    credentials,
		authMaterials:  authMaterials,
		now:            now,
		cacheTTL:       15 * time.Minute,
		forceProbe:     true,
		maxConcurrency: 2,
		modelGroup:     config.AntigravityModelGroupGemini,
		sampleCapacity: 10,
	})
	if err != nil {
		t.Fatalf("collectFreshEvidence failed: %v", err)
	}

	mock.mu.Lock()
	calls := len(mock.httpCalls)
	mock.mu.Unlock()

	// 2 credentials probed * 1 quota summary request = 2 calls; the summary already
	// carries both model groups, so no second endpoint is needed.
	if calls != 2 {
		t.Fatalf("total HTTP calls = %d; want 2 (1 summary call per account)", calls)
	}

	geminiResult := collected.ByGroup[config.AntigravityModelGroupGemini]
	if len(geminiResult.Eligible) != 2 {
		t.Fatalf("eligible credentials = %d; want 2", len(geminiResult.Eligible))
	}
	claudeResult := collected.ByGroup[config.AntigravityModelGroupClaudeGPT]
	if len(claudeResult.Eligible) != 2 {
		t.Fatalf("claude/gpt eligible credentials = %d; want 2 from the same response", len(claudeResult.Eligible))
	}
	if collected.Probed != 2 {
		t.Fatalf("probed credentials count = %d; want 2", collected.Probed)
	}

	// Each account carries one fresh Gemini observation; the peer group arrives in the
	// same response and is reported separately for the Gemini round.
	fresh := 0
	for _, obs := range geminiResult.Observations {
		if obs.Kind == evidence.ObservationFresh {
			fresh++
		}
	}
	if fresh != 2 {
		t.Fatalf("fresh gemini observations = %d; want 2", fresh)
	}
}

func TestCollectFreshEvidence_FallsBackToAvailableModelsWhenSummaryFails(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		doFn: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			if strings.Contains(req.URL, "fetchAvailableModels") {
				return host.HTTPResponse{
					StatusCode: http.StatusOK,
					Body: []byte(`{
						"models": {
							"gemini-2.5-pro": {"modelProvider": "google", "quotaInfo": {"remainingFraction": 0.77, "resetTime": "2026-08-21T17:00:00Z"}}
						}
					}`),
				}, nil
			}
			return host.HTTPResponse{StatusCode: http.StatusInternalServerError}, nil
		},
	}

	client := host.NewClient(mock)
	store, err := state.Load(context.Background(), filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}

	credentials := []core.Credential{
		{AuthIndex: "auth-fallback", Provider: core.ProviderAntigravity},
	}
	authMaterials := map[string]authMaterial{
		"auth-fallback": {accessToken: "tok", projectID: "proj"},
	}

	collected, err := collectFreshEvidence(context.Background(), collectInput{
		client:         client,
		store:          store,
		credentials:    credentials,
		authMaterials:  authMaterials,
		now:            now,
		cacheTTL:       15 * time.Minute,
		forceProbe:     true,
		maxConcurrency: 1,
		modelGroup:     config.AntigravityModelGroupGemini,
		sampleCapacity: 10,
	})
	if err != nil {
		t.Fatalf("collectFreshEvidence failed: %v", err)
	}

	geminiResult := collected.ByGroup[config.AntigravityModelGroupGemini]
	if len(geminiResult.Eligible) != 1 {
		t.Fatalf("expected 1 eligible credential after fallback, got %d", len(geminiResult.Eligible))
	}
	if geminiResult.Eligible[0].Remaining == nil || *geminiResult.Eligible[0].Remaining != 77 {
		t.Fatalf("expected remaining 77, got %v", geminiResult.Eligible[0].Remaining)
	}
}

func TestCollectFreshEvidence_ClaudeControlGroupDirectRouting(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"groups": [
					{"displayName": "Claude and GPT models", "buckets": [{"window": "5h", "remainingFraction": 0.82, "resetTime": "2026-08-21T17:00:00Z"}]}
				]
			}`),
		},
	}

	client := host.NewClient(mock)
	store, err := state.Load(context.Background(), filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}

	credentials := []core.Credential{
		{AuthIndex: "auth-claude-1", Provider: core.ProviderAntigravity},
	}
	authMaterials := map[string]authMaterial{
		"auth-claude-1": {accessToken: "tok", projectID: "proj"},
	}

	collected, err := collectFreshEvidence(context.Background(), collectInput{
		client:         client,
		store:          store,
		credentials:    credentials,
		authMaterials:  authMaterials,
		now:            now,
		cacheTTL:       15 * time.Minute,
		forceProbe:     true,
		maxConcurrency: 1,
		modelGroup:     config.AntigravityModelGroupClaudeGPT,
		sampleCapacity: 10,
	})
	if err != nil {
		t.Fatalf("collectFreshEvidence failed: %v", err)
	}

	mock.mu.Lock()
	calls := len(mock.httpCalls)
	calledURL := mock.httpCalls[0].URL
	mock.mu.Unlock()

	if calls != 1 {
		t.Fatalf("total HTTP calls = %d; want 1 for Claude direct routing", calls)
	}
	if calledURL != antigravity.RetrieveUserQuotaSummaryURL {
		t.Fatalf("expected call to %q, got %q", antigravity.RetrieveUserQuotaSummaryURL, calledURL)
	}

	claudeResult := collected.ByGroup[config.AntigravityModelGroupClaudeGPT]
	if len(claudeResult.Eligible) != 1 {
		t.Fatalf("expected 1 eligible credential, got %d", len(claudeResult.Eligible))
	}
	if claudeResult.Eligible[0].Remaining == nil || *claudeResult.Eligible[0].Remaining != 82 {
		t.Fatalf("expected remaining 82, got %v", claudeResult.Eligible[0].Remaining)
	}
}
