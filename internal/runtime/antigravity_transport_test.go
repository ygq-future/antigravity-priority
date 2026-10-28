package runtime

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/provider/antigravity"
)

func TestExecuteAntigravityQuotaRequestOwnsNetworkIOAndFallback(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		calls++
		if calls == 1 {
			return host.HTTPResponse{StatusCode: http.StatusServiceUnavailable}, nil
		}
		if request.Method != http.MethodPost || request.AuthIndex != "auth-1" {
			t.Fatalf("unexpected request: %#v", request)
		}
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"groups":[{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}]},{"displayName":"Claude Models","buckets":[{"window":"5h","remainingFraction":0.8,"resetTime":"2026-08-21T17:00:00Z"}]}]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex: "auth-1", AccessToken: "secret", ProjectID: "project-1", ObservedAt: now,
	})
	if calls != 2 {
		t.Fatalf("HTTP calls = %d; want 2", calls)
	}
	for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
		if results[group].ObservedAt != now || results[group].AuthIndex != "auth-1" {
			t.Fatalf("result[%s] = %#v", group, results[group])
		}
	}
}

type quotaHTTPDoerFunc func(context.Context, host.HTTPRequest) (host.HTTPResponse, error)

func (fn quotaHTTPDoerFunc) HTTPDoRaw(ctx context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
	return fn(ctx, request)
}

func TestExecuteAntigravityQuotaRequestPropagatesSafeFailure(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	doer := quotaHTTPDoerFunc(func(context.Context, host.HTTPRequest) (host.HTTPResponse, error) {
		return host.HTTPResponse{}, errors.New("token=secret-token network down")
	})
	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{AuthIndex: "auth-1", ObservedAt: now})
	for _, result := range results {
		if result.Status != antigravity.StatusProbeFailed || result.Error != "host http do failed" {
			t.Fatalf("unexpected safe failure result: %#v", result)
		}
	}
}
func TestExecuteAntigravityQuotaRequest_SupplementsGeminiFromAvailableModels(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		calls++
		if calls == 1 {
			// 1. Summary returns only Claude and GPT models (new Google behavior)
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"groups":[{"displayName":"Claude and GPT models","buckets":[{"window":"5h","remainingFraction":0.75,"resetTime":"2026-08-21T17:00:00Z"}]}]}`),
			}, nil
		}
		// 2. Fallback to available models queries per-model quotas for Gemini
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"models":{"gemini-2.5-pro":{"modelProvider":"google","quotaInfo":{"remainingFraction":0.92,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex: "auth-1", AccessToken: "secret", ProjectID: "project-1", ObservedAt: now,
	})

	if calls != 2 {
		t.Fatalf("HTTP calls = %d; want 2", calls)
	}
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Errorf("expected ClaudeGPT StatusReady, got %v", results[antigravity.ModelGroupClaudeGPT].Status)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Errorf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
	if *results[antigravity.ModelGroupGemini].Remaining != 92 {
		t.Errorf("expected Gemini Remaining 92, got %v", *results[antigravity.ModelGroupGemini].Remaining)
	}
}

func TestExecuteAntigravityQuotaRequest_GeminiTargetedProbeKeepsPeerGroup(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"}]},
				{"displayName":"Claude and GPT models","buckets":[{"window":"5h","remainingFraction":0.8,"resetTime":"2026-08-21T17:00:00Z"}]}
			]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupGemini,
	})

	if len(requestedURLs) != 1 {
		t.Fatalf("HTTP calls = %d; want 1 (avoid serial fallback)", len(requestedURLs))
	}
	if requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryURL {
		t.Fatalf("first HTTP call URL = %q; want %q", requestedURLs[0], antigravity.RetrieveUserQuotaSummaryURL)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
	if *results[antigravity.ModelGroupGemini].Remaining != 88 {
		t.Fatalf("expected Gemini Remaining 88, got %v", *results[antigravity.ModelGroupGemini].Remaining)
	}
	// The peer group travels in the same response and must keep its fresh evidence.
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Fatalf("expected ClaudeGPT StatusReady, got %#v", results[antigravity.ModelGroupClaudeGPT])
	}
}

func TestExecuteAntigravityQuotaRequest_GeminiFallsBackWhenSummaryUnavailable(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		if strings.Contains(request.URL, "fetchAvailableModels") {
			// Available models still reports per-model Gemini quota
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"models":{"gemini-2.5-pro":{"modelProvider":"google","quotaInfo":{"remainingFraction":0.65,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
			}, nil
		}
		// All quota summary endpoints fail with 500
		return host.HTTPResponse{StatusCode: http.StatusInternalServerError}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupGemini,
	})

	if len(requestedURLs) != 4 {
		t.Fatalf("HTTP calls = %d; want 4 (3 summary URLs failed + 1 fallback succeeded)", len(requestedURLs))
	}
	if requestedURLs[len(requestedURLs)-1] != antigravity.FetchAvailableModelsURL {
		t.Errorf("fallback URL = %q; want fetchAvailableModels", requestedURLs[len(requestedURLs)-1])
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
	if *results[antigravity.ModelGroupGemini].Remaining != 65 {
		t.Fatalf("expected Gemini Remaining 65, got %v", *results[antigravity.ModelGroupGemini].Remaining)
	}
}

func TestExecuteAntigravityQuotaRequest_GeminiFallsBackWhenGeminiQuotaMissing(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		calls++
		if calls == 1 {
			// Quota summary returns 200, but has only Claude models
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"models":{"claude-3-5-sonnet":{"quotaInfo":{"remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
			}, nil
		}
		// Fallback to available models provides Gemini
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"models":{"gemini-2.5-pro":{"quotaInfo":{"remainingFraction":0.7,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupGemini,
	})

	if calls != 2 {
		t.Fatalf("HTTP calls = %d; want 2", calls)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
}

func TestExecuteAntigravityQuotaRequest_ClaudeTargetedProbeSingleSummaryCall(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"groups":[{"displayName":"Claude and GPT models","buckets":[{"window":"5h","remainingFraction":0.8,"resetTime":"2026-08-21T17:00:00Z"}]}]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupClaudeGPT,
	})

	if len(requestedURLs) != 1 {
		t.Fatalf("HTTP calls = %d; want 1", len(requestedURLs))
	}
	if requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryURL {
		t.Fatalf("call URL = %q; want %q", requestedURLs[0], antigravity.RetrieveUserQuotaSummaryURL)
	}
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Fatalf("expected ClaudeGPT StatusReady, got %v", results[antigravity.ModelGroupClaudeGPT].Status)
	}
	// A group the summary omits is reported as a probe failure rather than silently
	// dropped, so its historical evidence is never mistaken for a fresh observation.
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusProbeFailed {
		t.Fatalf("expected Gemini StatusProbeFailed, got %#v", results[antigravity.ModelGroupGemini])
	}
}

func TestExecuteAntigravityQuotaRequest_BothModelsReadyAvoidsSecondRequest(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		calls++
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}]},
				{"displayName":"Claude Models","buckets":[{"window":"5h","remainingFraction":0.8,"resetTime":"2026-08-21T17:00:00Z"}]}
			]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: "", // Unspecified: requests all models
	})

	if calls != 1 {
		t.Fatalf("HTTP calls = %d; want 1 when all requested models are ready", calls)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Fatalf("expected ClaudeGPT StatusReady, got %v", results[antigravity.ModelGroupClaudeGPT].Status)
	}
}

// TestExecuteAntigravityQuotaRequest_GeminiProbePreservesDualWindowEvidence guards the
// dashboard evidence contract: a Gemini-targeted probe must surface the canonical
// 5h/weekly windows (and therefore the reset countdowns and plan type) carried by the
// quota summary. The per-model available-models payload has no window names, so
// preferring it degrades evidence into one unclassified window.
func TestExecuteAntigravityQuotaRequest_GeminiProbePreservesDualWindowEvidence(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		if strings.Contains(request.URL, "fetchAvailableModels") {
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body: []byte(`{"models":{
					"gemini-2.5-pro":{"modelProvider":"google","quotaInfo":{"remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"}},
					"claude-3-5-sonnet":{"quotaInfo":{"remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}}
				}}`),
			}, nil
		}
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[
					{"window":"5h","remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"},
					{"window":"7d","remainingFraction":0.64,"resetTime":"2026-08-27T00:00:00Z"}
				]},
				{"displayName":"Claude and GPT models","buckets":[
					{"window":"5h","remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}
				]}
			]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupGemini,
	})

	if len(requestedURLs) != 1 {
		t.Fatalf("HTTP calls = %v; want exactly the quota summary", requestedURLs)
	}
	gemini := results[antigravity.ModelGroupGemini]
	if gemini.ShortWindowResetAt == nil || gemini.LongWindowResetAt == nil {
		t.Fatalf("Gemini dual-window evidence lost: %#v", gemini)
	}
	if gemini.PlanType != core.PlanTypePro {
		t.Fatalf("Gemini plan type = %q; want %q", gemini.PlanType, core.PlanTypePro)
	}
	if claude := results[antigravity.ModelGroupClaudeGPT]; claude.Status != antigravity.StatusReady {
		t.Fatalf("ClaudeGPT status = %v; want ready", claude.Status)
	}
}
