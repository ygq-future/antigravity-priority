package runtime

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestExecuteAntigravityQuotaRequest_DirectGeminiRoutingSingleCall(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"models":{"gemini-2.5-pro":{"modelProvider":"google","quotaInfo":{"remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
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
	if requestedURLs[0] != antigravity.FetchAvailableModelsURL {
		t.Fatalf("first HTTP call URL = %q; want %q", requestedURLs[0], antigravity.FetchAvailableModelsURL)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady, got %v", results[antigravity.ModelGroupGemini].Status)
	}
	if *results[antigravity.ModelGroupGemini].Remaining != 88 {
		t.Fatalf("expected Gemini Remaining 88, got %v", *results[antigravity.ModelGroupGemini].Remaining)
	}
	// Unrequested Claude group should be pruned and not present as a false failure
	if _, exists := results[antigravity.ModelGroupClaudeGPT]; exists {
		t.Fatalf("expected ClaudeGPT to be pruned from Gemini-targeted probe, got: %#v", results[antigravity.ModelGroupClaudeGPT])
	}
}

func TestExecuteAntigravityQuotaRequest_GeminiFallsBackWhenPrimaryEndpointFails(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		if strings.Contains(request.URL, "fetchAvailableModels") {
			// All available models endpoints fail with 500
			return host.HTTPResponse{StatusCode: http.StatusInternalServerError}, nil
		}
		// Fallback to retrieveUserQuotaSummary returns Gemini (legacy response)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"groups":[{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.65,"resetTime":"2026-08-21T17:00:00Z"}]}]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
		TargetGroup: antigravity.ModelGroupGemini,
	})

	if len(requestedURLs) != 4 {
		t.Fatalf("HTTP calls = %d; want 4 (3 primary URLs failed + 1 fallback succeeded)", len(requestedURLs))
	}
	if requestedURLs[len(requestedURLs)-1] != antigravity.RetrieveUserQuotaSummaryURL {
		t.Errorf("fallback URL = %q; want retrieveUserQuotaSummary", requestedURLs[len(requestedURLs)-1])
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
			// fetchAvailableModels returns 200, but has only Claude models
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       []byte(`{"models":{"claude-3-5-sonnet":{"quotaInfo":{"remainingFraction":0.9,"resetTime":"2026-08-21T17:00:00Z"}}}}`),
			}, nil
		}
		// Fallback to retrieveUserQuotaSummary provides Gemini
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"groups":[{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.7,"resetTime":"2026-08-21T17:00:00Z"}]}]}`),
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

func TestExecuteAntigravityQuotaRequest_ClaudeDirectRoutingSingleCall(t *testing.T) {
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
	if _, exists := results[antigravity.ModelGroupGemini]; exists {
		t.Fatalf("expected Gemini to be pruned from Claude-targeted probe, got: %#v", results[antigravity.ModelGroupGemini])
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
