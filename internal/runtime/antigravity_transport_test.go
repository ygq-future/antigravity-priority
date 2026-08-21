package runtime

import (
	"context"
	"errors"
	"net/http"
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
		return host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(`{"models":[]}`)}, nil
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
