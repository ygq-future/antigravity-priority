package antigravity

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
)

type mockClock struct {
	now time.Time
}

func (m mockClock) Now() time.Time {
	return m.now
}

type mockHTTPDoer struct {
	handler func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error)
}

func (m mockHTTPDoer) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	return m.handler(ctx, req)
}

func TestProber_Probe_Success(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	validJSON := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "5h",
							"remainingFraction": 0.80,
							"resetTime": "2026-08-17T17:00:00Z"
						},
						{
							"name": "weekly",
							"remainingFraction": 0.90,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			}
		}
	}`)

	mockDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			if req.AuthIndex != "auth-123" {
				t.Errorf("expected AuthIndex auth-123, got %s", req.AuthIndex)
			}
			if req.Headers.Get("Authorization") != "Bearer secret-token" {
				t.Errorf("expected Authorization Bearer secret-token, got %s", req.Headers.Get("Authorization"))
			}
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       validJSON,
			}, nil
		},
	}

	prober := NewProber(mockDoer, mockClock{now: fixedNow})
	result := prober.Probe(context.Background(), ProbeRequest{
		AuthIndex:   "auth-123",
		AccessToken: "secret-token",
		ProjectID:   "my-project",
		ModelGroup:  ModelGroupGemini,
	})

	if result.Status != StatusReady {
		t.Fatalf("expected StatusReady, got %v (err: %v)", result.Status, result.Error)
	}
	if result.AuthIndex != "auth-123" {
		t.Errorf("expected AuthIndex auth-123, got %s", result.AuthIndex)
	}
	if result.Provider != core.ProviderAntigravity {
		t.Errorf("expected ProviderAntigravity, got %v", result.Provider)
	}
	if result.ShortWindowRemaining == nil || *result.ShortWindowRemaining != 80 {
		t.Errorf("expected ShortWindowRemaining=80, got %v", result.ShortWindowRemaining)
	}
	if result.LongWindowRemaining == nil || *result.LongWindowRemaining != 90 {
		t.Errorf("expected LongWindowRemaining=90, got %v", result.LongWindowRemaining)
	}
	if result.ObservedAt != fixedNow {
		t.Errorf("expected ObservedAt %v, got %v", fixedNow, result.ObservedAt)
	}
}

func TestProber_Probe_Failover(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	validJSON := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "5h",
							"remainingFraction": 0.75,
							"resetTime": "2026-08-17T17:00:00Z"
						}
					]
				}
			}
		}
	}`)

	callCount := 0
	mockDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			callCount++
			if callCount == 1 {
				// First URL returns 503
				return host.HTTPResponse{StatusCode: http.StatusServiceUnavailable}, nil
			}
			// Second URL succeeds
			return host.HTTPResponse{
				StatusCode: http.StatusOK,
				Body:       validJSON,
			}, nil
		},
	}

	prober := NewProber(mockDoer, mockClock{now: fixedNow})
	result := prober.Probe(context.Background(), ProbeRequest{
		AuthIndex:  "auth-failover",
		ModelGroup: ModelGroupGemini,
	})

	if result.Status != StatusReady {
		t.Fatalf("expected StatusReady after failover, got %v (err: %v)", result.Status, result.Error)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

func TestProber_Probe_Errors(t *testing.T) {
	fixedNow := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	// Upstream 401 Unauthorized
	unauthorizedDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			return host.HTTPResponse{StatusCode: http.StatusUnauthorized}, nil
		},
	}
	p1 := NewProber(unauthorizedDoer, mockClock{now: fixedNow})
	r1 := p1.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-1", ModelGroup: ModelGroupGemini})
	if r1.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on 401, got %v", r1.Status)
	}
	if r1.AuthIndex != "auth-1" {
		t.Errorf("expected AuthIndex auth-1, got %s", r1.AuthIndex)
	}

	// Upstream 403 Forbidden
	forbiddenDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			return host.HTTPResponse{StatusCode: http.StatusForbidden}, nil
		},
	}
	p2 := NewProber(forbiddenDoer, mockClock{now: fixedNow})
	r2 := p2.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-2", ModelGroup: ModelGroupGemini})
	if r2.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on 403, got %v", r2.Status)
	}

	// Upstream 429 Too Many Requests
	rateLimitDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			return host.HTTPResponse{StatusCode: http.StatusTooManyRequests}, nil
		},
	}
	p3 := NewProber(rateLimitDoer, mockClock{now: fixedNow})
	r3 := p3.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-3", ModelGroup: ModelGroupGemini})
	if r3.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on 429, got %v", r3.Status)
	}

	// Network error on all endpoints
	netErrDoer := mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			return host.HTTPResponse{}, errors.New("connection reset by peer")
		},
	}
	p4 := NewProber(netErrDoer, mockClock{now: fixedNow})
	r4 := p4.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-4", ModelGroup: ModelGroupGemini})
	if r4.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on network error, got %v", r4.Status)
	}

	// Nil host HTTP doer
	p5 := NewProber(nil, mockClock{now: fixedNow})
	r5 := p5.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-5", ModelGroup: ModelGroupGemini})
	if r5.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on nil host, got %v", r5.Status)
	}
}

func TestProber_RequestFormatting(t *testing.T) {
	// Probe with empty token should default to $TOKEN$
	headers := probeHeaders(ProbeRequest{AccessToken: ""})
	if headers.Get("Authorization") != "Bearer $TOKEN$" {
		t.Errorf("expected default Bearer $TOKEN$, got %s", headers.Get("Authorization"))
	}

	// Probe with empty ProjectID should produce nil body
	bodyEmpty := probeBody(ProbeRequest{ProjectID: ""})
	if bodyEmpty != nil {
		t.Errorf("expected nil body for empty project ID, got %s", string(bodyEmpty))
	}

	// Probe with non-empty ProjectID
	bodyProject := probeBody(ProbeRequest{ProjectID: "proj-abc"})
	if bodyProject == nil || string(bodyProject) != `{"project":"proj-abc"}` {
		t.Errorf("expected JSON project body, got %s", string(bodyProject))
	}

	// Prober with nil clock should use real clock without panic
	pReal := NewProber(mockHTTPDoer{
		handler: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			return host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(`{}`)}, nil
		},
	}, nil)
	rReal := pReal.Probe(context.Background(), ProbeRequest{AuthIndex: "auth-real", ModelGroup: ModelGroupGemini})
	if rReal.ObservedAt.IsZero() {
		t.Errorf("expected non-zero ObservedAt")
	}
}
