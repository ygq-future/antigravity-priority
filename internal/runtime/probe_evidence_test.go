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

func TestProbeAndRecord_GeminiTargetRoutesDirectlySingleCall(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"models": {
					"gemini-2.5-pro": {
						"modelProvider": "google",
						"quotaInfo": {
							"remainingFraction": 0.85,
							"resetTime": "2026-08-21T17:00:00Z"
						}
					}
				}
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
		t.Fatalf("HTTP calls = %d; want 1 (Gemini direct routing must not incur serial fallback)", calls)
	}
	if calledURLs[0] != antigravity.FetchAvailableModelsURL {
		t.Fatalf("expected direct call to %q, got %q", antigravity.FetchAvailableModelsURL, calledURLs[0])
	}

	if len(results) != 1 {
		t.Fatalf("results len = %d; want 1 (only Gemini result expected)", len(results))
	}
	if results[0].ModelGroup != antigravity.ModelGroupGemini || results[0].Status != antigravity.StatusReady {
		t.Fatalf("unexpected probe result: %#v", results[0])
	}
	if results[0].Remaining == nil || *results[0].Remaining != 85 {
		t.Fatalf("unexpected remaining: %v", results[0].Remaining)
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

	// Verify ClaudeGPT was NOT recorded as a false probe failure
	if entry, exists := store.GetEntry("auth-gemini-1", string(antigravity.ModelGroupClaudeGPT)); exists {
		if entry.LastError != "" {
			t.Fatalf("ClaudeGPT has unexpected probe failure recorded: %s", entry.LastError)
		}
	}
}

func TestCollectFreshEvidence_GeminiControlGroupOptimizesLatency(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"models": {
					"gemini-2.5-pro": {
						"modelProvider": "google",
						"quotaInfo": {
							"remainingFraction": 0.9,
							"resetTime": "2026-08-21T17:00:00Z"
						}
					}
				}
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

	// 2 credentials probed * 1 direct request = 2 calls (instead of 2 * 2 = 4 sequential calls)
	if calls != 2 {
		t.Fatalf("total HTTP calls = %d; want 2 (1 call per account, ~50%% latency cut)", calls)
	}

	geminiResult := collected.ByGroup[config.AntigravityModelGroupGemini]
	if len(geminiResult.Eligible) != 2 {
		t.Fatalf("eligible credentials = %d; want 2", len(geminiResult.Eligible))
	}
	if collected.Probed != 2 {
		t.Fatalf("probed credentials count = %d; want 2", collected.Probed)
	}

	for _, obs := range geminiResult.Observations {
		if obs.Kind != evidence.ObservationFresh {
			t.Errorf("observation for %s kind = %v; want ObservationFresh", obs.AuthIndex, obs.Kind)
		}
	}
}

func TestCollectFreshEvidence_GeminiFallbackWhenModelsEndpointFails(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		doFn: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			if strings.Contains(req.URL, "fetchAvailableModels") {
				return host.HTTPResponse{StatusCode: http.StatusInternalServerError}, nil
			}
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body: []byte(`{
					"groups": [
						{"displayName": "Gemini Models", "buckets": [{"window": "5h", "remainingFraction": 0.77, "resetTime": "2026-08-21T17:00:00Z"}]}
					]
				}`),
			}, nil
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
