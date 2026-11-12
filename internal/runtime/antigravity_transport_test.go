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

func TestExecuteAntigravityQuotaRequestReportsUpstreamFailure(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	calls := 0
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		calls++
		if request.Method != http.MethodPost || request.AuthIndex != "auth-1" || request.URL != antigravity.RetrieveUserQuotaSummaryDailyURL {
			t.Fatalf("unexpected request: %#v", request)
		}
		return host.HTTPResponse{StatusCode: http.StatusServiceUnavailable}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex: "auth-1", AccessToken: "secret", ProjectID: "project-1", ObservedAt: now,
	})
	if calls != 1 {
		t.Fatalf("HTTP calls = %d; want a single summary request", calls)
	}
	for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
		if results[group].Status != antigravity.StatusProbeFailed {
			t.Fatalf("result[%s] = %#v; want a reported probe failure", group, results[group])
		}
		if results[group].Error != "retrieve quota summary status 503" {
			t.Fatalf("result[%s] error = %q; want the upstream status", group, results[group].Error)
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
	if len(results) != 2 {
		t.Fatalf("failure results = %d; want the two universal pools", len(results))
	}
	for _, result := range results {
		if result.Status != antigravity.StatusProbeFailed || result.Error != "host http do failed" {
			t.Fatalf("unexpected safe failure result: %#v", result)
		}
	}
}
func TestExecuteAntigravityQuotaRequest_SingleSummaryCoversAllPools(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[{"window":"5h","remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"}]},
				{"displayName":"Gemini 4 Argon","buckets":[{"bucketId":"gemini-argon-wtu-weekly","window":"weekly","remainingFraction":0.71,"resetTime":"2026-08-27T00:00:00Z"}]},
				{"displayName":"Claude and GPT models","buckets":[{"window":"5h","remainingFraction":0.8,"resetTime":"2026-08-21T17:00:00Z"}]}
			]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
	})

	if len(requestedURLs) != 1 {
		t.Fatalf("HTTP calls = %d; want 1 (avoid serial fallback)", len(requestedURLs))
	}
	if requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryDailyURL {
		t.Fatalf("first HTTP call URL = %q; want %q", requestedURLs[0], antigravity.RetrieveUserQuotaSummaryDailyURL)
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
	if results[antigravity.ModelGroupArgon].Status != antigravity.StatusReady {
		t.Fatalf("expected Argon StatusReady, got %#v", results[antigravity.ModelGroupArgon])
	}
}

func TestExecuteAntigravityQuotaRequest_MissingGroupIsNotFabricated(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		// A summary can answer 200 with only one pool. The other pools were not
		// observed in this round and must not be filled from another endpoint.
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
	})

	if len(requestedURLs) != 1 || requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryDailyURL {
		t.Fatalf("HTTP calls = %v; want a single summary request", requestedURLs)
	}
	if results[antigravity.ModelGroupGemini].Status == antigravity.StatusReady {
		t.Fatalf("Gemini = %#v; an omitted pool must not become ready", results[antigravity.ModelGroupGemini])
	}
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Fatalf("ClaudeGPT = %#v; want the observed pool", results[antigravity.ModelGroupClaudeGPT])
	}
}

// TestExecuteAntigravityQuotaRequest_GeminiProbePreservesDualWindowEvidence guards the
// dashboard evidence contract: a Gemini-targeted probe must surface the canonical
// 5h/weekly windows (and therefore the reset countdowns and plan type) carried by the
// quota summary.
func TestExecuteAntigravityQuotaRequest_GeminiProbePreservesDualWindowEvidence(t *testing.T) {
	now := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[
					{"window":"5h","remainingFraction":0.88,"resetTime":"2026-08-21T17:00:00Z"},
					{"window":"7d","remainingFraction":0.64,"resetTime":"2026-08-27T00:00:00Z"}
				]},
				{"displayName":"Gemini 4 Argon","buckets":[
					{"bucketId":"gemini-argon-wtu-weekly","window":"weekly","remainingFraction":0.71,"resetTime":"2026-08-29T00:00:00Z"}
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

// Consumer Antigravity traffic is measured on the daily host, so that is the only
// host the probe may ask.
func TestExecuteAntigravityQuotaRequest_PrefersDailySummaryHost(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(quotaSummaryWithArgon)}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
	})

	if len(requestedURLs) != 1 {
		t.Fatalf("HTTP calls = %v; want exactly one summary call", requestedURLs)
	}
	if requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryDailyURL {
		t.Fatalf("first call URL = %q; want %q", requestedURLs[0], antigravity.RetrieveUserQuotaSummaryDailyURL)
	}
	if !strings.Contains(requestedURLs[0], "daily-") {
		t.Fatalf("first call URL = %q; daily host must be asked before production", requestedURLs[0])
	}
	if argon := results[antigravity.ModelGroupGemini].ArgonWindowRemaining; argon == nil || *argon != 71 {
		t.Fatalf("ArgonWindowRemaining = %v; want 71", argon)
	}
}

// Untouched buckets are reported at full quota with a reset time that every probe
// pushes back by a whole window. That reset is the only signal warm-up has, so the
// probe must keep the buckets as measured evidence instead of rejecting them.
func TestExecuteAntigravityQuotaRequest_UntouchedBucketsStayReady(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	var requestedURLs []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		requestedURLs = append(requestedURLs, request.URL)
		return host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[
					{"bucketId":"gemini-5h","window":"5h","remainingFraction":1,"resetTime":"2026-11-11T17:00:00Z"},
					{"bucketId":"gemini-weekly","window":"weekly","remainingFraction":1,"resetTime":"2026-11-18T12:00:00Z"}
				]},
				{"displayName":"Claude and GPT models","buckets":[
					{"bucketId":"3p-5h","window":"5h","remainingFraction":1,"resetTime":"2026-11-11T17:00:00Z"},
					{"bucketId":"3p-weekly","window":"weekly","remainingFraction":1,"resetTime":"2026-11-18T12:00:00Z"}
				]}
			]}`),
		}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex: "auth-1", AccessToken: "secret", ProjectID: "project-1", ObservedAt: now,
	})

	if len(requestedURLs) != 1 || requestedURLs[0] != antigravity.RetrieveUserQuotaSummaryDailyURL {
		t.Fatalf("HTTP calls = %v; want a single daily summary call", requestedURLs)
	}
	gemini := results[antigravity.ModelGroupGemini]
	if gemini.Status != antigravity.StatusReady {
		t.Fatalf("Gemini status = %v; untouched buckets are still measured evidence", gemini.Status)
	}
	if gemini.ShortWindowRemaining == nil || *gemini.ShortWindowRemaining != 100 {
		t.Fatalf("Gemini 5h remaining = %v; want 100", gemini.ShortWindowRemaining)
	}
	if gemini.ShortWindowResetAt == nil || gemini.ShortWindowResetAt.Format(time.RFC3339) != "2026-11-11T17:00:00Z" {
		t.Fatalf("Gemini 5h reset = %v; want the upstream untouched-window reset", gemini.ShortWindowResetAt)
	}
}

// Google gates the separate Gemini 4 Argon weekly bucket on the client identity, so the
// probe must advertise the hub identity that receives it.
func TestExecuteAntigravityQuotaRequest_UsesHubIdentityThatReceivesArgonBucket(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	var userAgent string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		userAgent = strings.Join(request.Headers["User-Agent"], ",")
		body := quotaSummaryWithArgon
		if userAgent != antigravityHubUserAgent {
			body = quotaSummaryWithoutArgon
		}
		return host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(body)}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
	})

	if userAgent != antigravityHubUserAgent {
		t.Fatalf("User-Agent = %q; want %q", userAgent, antigravityHubUserAgent)
	}
	if !strings.Contains(userAgent, "/2.24.0 ") {
		t.Fatalf("User-Agent = %q; hub 2.24.0 is required for measured Gemini buckets", userAgent)
	}
	if argon := results[antigravity.ModelGroupGemini].ArgonWindowRemaining; argon == nil || *argon != 71 {
		t.Fatalf("ArgonWindowRemaining = %v; want 71 from the identity-gated Argon bucket", argon)
	}
}

// A hard rejection of the current identity must fall back to the legacy identity instead
// of erasing every window from the dashboard.
func TestExecuteAntigravityQuotaRequest_RetriesLegacyIdentityOnForbidden(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	var userAgents []string
	doer := quotaHTTPDoerFunc(func(_ context.Context, request host.HTTPRequest) (host.HTTPResponse, error) {
		userAgent := strings.Join(request.Headers["User-Agent"], ",")
		userAgents = append(userAgents, userAgent)
		if userAgent == antigravityHubUserAgent {
			return host.HTTPResponse{StatusCode: http.StatusForbidden}, nil
		}
		return host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(quotaSummaryWithoutArgon)}, nil
	})

	results := executeAntigravityQuotaRequest(context.Background(), doer, antigravityQuotaRequest{
		AuthIndex:   "auth-1",
		AccessToken: "secret",
		ProjectID:   "project-1",
		ObservedAt:  now,
	})

	if len(userAgents) != 2 {
		t.Fatalf("HTTP calls = %v; want the rejected identity retried once", userAgents)
	}
	if userAgents[1] != antigravityLegacyHubUserAgent {
		t.Fatalf("retry User-Agent = %q; want %q", userAgents[1], antigravityLegacyHubUserAgent)
	}
	if results[antigravity.ModelGroupGemini].Status != antigravity.StatusReady {
		t.Fatalf("expected Gemini StatusReady after legacy fallback, got %#v", results[antigravity.ModelGroupGemini])
	}
	if results[antigravity.ModelGroupGemini].ShortWindowRemaining == nil || *results[antigravity.ModelGroupGemini].ShortWindowRemaining != 99 {
		t.Fatalf("ShortWindowRemaining = %v; want 99", results[antigravity.ModelGroupGemini].ShortWindowRemaining)
	}
	if results[antigravity.ModelGroupClaudeGPT].Status != antigravity.StatusReady {
		t.Fatalf("legacy identity retry must restore the Claude/GPT pool: %#v", results[antigravity.ModelGroupClaudeGPT])
	}
}

// quotaSummaryWithArgon mirrors the live RetrieveUserQuotaSummary shape: the Argon
// weekly limit travels in its own "Gemini 4 Argon" group, not inside the shared pool.
const quotaSummaryWithArgon = `{"groups":[
	{"displayName":"Gemini Models","description":"Shared quota for Gemini models","buckets":[
		{"bucketId":"gemini-5h","displayName":"Five Hour Limit","window":"5h","remainingFraction":0.99,"resetTime":"2026-10-28T17:00:00Z"},
		{"bucketId":"gemini-weekly","displayName":"Weekly Limit","window":"weekly","remainingFraction":0.79,"resetTime":"2026-10-31T00:00:00Z"}
	]},
	{"displayName":"Gemini 4 Argon","description":"For paid Pro plans, Argon quota is separate.","buckets":[
		{"bucketId":"gemini-argon-wtu-weekly","displayName":"Gemini 4 Argon Weekly Limit","window":"weekly","remainingFraction":0.71,"resetTime":"2026-10-29T23:24:33Z"}
	]},
	{"displayName":"Claude and GPT models","buckets":[
		{"bucketId":"3p-5h","window":"5h","remainingFraction":0.93,"resetTime":"2026-10-28T17:00:00Z"}
	]}
]}`

const quotaSummaryWithoutArgon = `{"groups":[
	{"displayName":"Gemini Models","description":"Shared quota for Gemini models","buckets":[
		{"bucketId":"gemini-5h","displayName":"Five Hour Limit","window":"5h","remainingFraction":0.99,"resetTime":"2026-10-28T17:00:00Z"},
		{"bucketId":"gemini-weekly","displayName":"Weekly Limit","window":"weekly","remainingFraction":0.79,"resetTime":"2026-10-31T00:00:00Z"}
	]},
	{"displayName":"Claude and GPT models","buckets":[
		{"bucketId":"3p-5h","window":"5h","remainingFraction":0.93,"resetTime":"2026-10-28T17:00:00Z"}
	]}
]}`
