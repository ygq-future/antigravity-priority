package runtime

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/provider/antigravity"
	"antigravity-priority/internal/state"
)

type planTierMockDoer struct {
	mu       sync.Mutex
	calls    []host.HTTPRequest
	response host.HTTPResponse
	err      error
}

func (d *planTierMockDoer) HTTPDoRaw(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	d.mu.Lock()
	d.calls = append(d.calls, req)
	response := d.response
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return host.HTTPResponse{}, err
	}
	return response, nil
}

func (d *planTierMockDoer) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func TestRefreshPlanTiersFetchesAndCachesAuthoritativeTier(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	doer := &planTierMockDoer{response: host.HTTPResponse{
		StatusCode: http.StatusOK,
		Body:       []byte(`{"currentTier":{"id":"free-tier","name":"Antigravity"},"paidTier":{"id":"g1-ultra-tier","name":"Google AI Ultra"}}`),
	}}
	credentials := []core.Credential{{AuthIndex: "auth-ultra", Provider: core.ProviderAntigravity}}
	materials := map[string]authMaterial{"auth-ultra": {accessToken: "secret-token"}}

	changed, err := refreshPlanTiers(ctx, doer, credentials, materials, store, now, false, 4)
	if err != nil {
		t.Fatalf("refreshPlanTiers() error = %v", err)
	}
	if !changed {
		t.Fatal("expected plan tier refresh to report a change")
	}
	if doer.callCount() != 1 {
		t.Fatalf("HTTP calls = %d; want 1", doer.callCount())
	}

	call := doer.calls[0]
	if call.Method != http.MethodPost || call.URL != antigravity.LoadCodeAssistDailyURL {
		t.Fatalf("request = %s %s; want POST %s", call.Method, call.URL, antigravity.LoadCodeAssistDailyURL)
	}
	if string(call.Body) != `{"metadata":{"ideType":"ANTIGRAVITY"}}` {
		t.Fatalf("request body = %s", call.Body)
	}
	if call.AuthIndex != "auth-ultra" {
		t.Fatalf("auth index = %q; want auth-ultra", call.AuthIndex)
	}
	if got := call.Headers["Authorization"]; len(got) != 1 || got[0] != "Bearer secret-token" {
		t.Fatalf("authorization header = %v", got)
	}
	if got := call.Headers["User-Agent"]; len(got) != 1 || got[0] != antigravityHubUserAgent {
		t.Fatalf("user agent = %v; want %q", got, antigravityHubUserAgent)
	}

	record, ok := store.GetPlanTier("auth-ultra")
	if !ok {
		t.Fatal("expected persisted plan tier record")
	}
	if record.PlanType != core.PlanTypeUltra {
		t.Fatalf("record = %#v; want ultra", record)
	}
	if !record.ObservedAt.Equal(now) || !record.AttemptedAt.Equal(now) {
		t.Fatalf("record timestamps = observed %v attempted %v; want %v", record.ObservedAt, record.AttemptedAt, now)
	}
	if got := store.GetPlanType("auth-ultra", "gemini"); got != core.PlanTypeUltra {
		t.Fatalf("GetPlanType() = %q; want ultra", got)
	}
}

func TestRefreshPlanTiersSkipsFreshCache(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.SetPlanTier("auth-pro", state.PlanTierRecord{
		PlanType:   core.PlanTypePro,
		ObservedAt: now.Add(-time.Hour),
	})
	doer := &planTierMockDoer{response: host.HTTPResponse{StatusCode: http.StatusOK}}
	credentials := []core.Credential{{AuthIndex: "auth-pro", Provider: core.ProviderAntigravity}}

	changed, err := refreshPlanTiers(ctx, doer, credentials, map[string]authMaterial{
		"auth-pro": {accessToken: "token"},
	}, store, now, false, 4)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("fresh cache must not be refreshed")
	}
	if doer.callCount() != 0 {
		t.Fatalf("HTTP calls = %d; want 0", doer.callCount())
	}
}

func TestRefreshPlanTiersKeepsKnownTierOnFailureAndThrottlesRetry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	observedAt := now.Add(-25 * time.Hour)
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.SetPlanTier("auth-pro", state.PlanTierRecord{
		PlanType:   core.PlanTypePro,
		ObservedAt: observedAt,
	})
	doer := &planTierMockDoer{response: host.HTTPResponse{StatusCode: http.StatusInternalServerError}}
	credentials := []core.Credential{{AuthIndex: "auth-pro", Provider: core.ProviderAntigravity}}
	materials := map[string]authMaterial{"auth-pro": {accessToken: "token"}}

	changed, err := refreshPlanTiers(ctx, doer, credentials, materials, store, now, false, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected failed refresh to record attempt metadata")
	}
	record, _ := store.GetPlanTier("auth-pro")
	if record.PlanType != core.PlanTypePro || !record.ObservedAt.Equal(observedAt) {
		t.Fatalf("failure discarded last known tier: %#v", record)
	}
	if !record.AttemptedAt.Equal(now) {
		t.Fatalf("attempted_at = %v; want %v", record.AttemptedAt, now)
	}

	if _, err := refreshPlanTiers(ctx, doer, credentials, materials, store, now.Add(10*time.Minute), false, 4); err != nil {
		t.Fatal(err)
	}
	if doer.callCount() != 1 {
		t.Fatalf("HTTP calls = %d; want retry throttled at 1", doer.callCount())
	}

	if _, err := refreshPlanTiers(ctx, doer, credentials, materials, store, now.Add(61*time.Minute), false, 4); err != nil {
		t.Fatal(err)
	}
	if doer.callCount() != 2 {
		t.Fatalf("HTTP calls = %d; want retry after 1h", doer.callCount())
	}
}

func TestRefreshPlanTiersFailureFallsBackToWindowInference(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	doer := &planTierMockDoer{response: host.HTTPResponse{StatusCode: http.StatusForbidden}}
	credentials := []core.Credential{{AuthIndex: "auth-pro", Provider: core.ProviderAntigravity}}
	materials := map[string]authMaterial{"auth-pro": {accessToken: "token"}}

	if _, err := refreshPlanTiers(ctx, doer, credentials, materials, store, now, false, 4); err != nil {
		t.Fatal(err)
	}
	record, ok := store.GetPlanTier("auth-pro")
	if !ok || record.PlanType != "" {
		t.Fatalf("failed refresh record = %#v; want no plan type", record)
	}

	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[
					{"window":"5h","remainingFraction":0.8,"resetTime":"2026-11-11T17:00:00Z"},
					{"window":"weekly","remainingFraction":0.6,"resetTime":"2026-11-18T00:00:00Z"}
				]},
				{"displayName":"Claude and GPT models","buckets":[
					{"window":"5h","remainingFraction":0.5,"resetTime":"2026-11-11T17:00:00Z"},
					{"window":"weekly","remainingFraction":0.4,"resetTime":"2026-11-18T00:00:00Z"}
				]}
			]}`),
		},
	}
	results, err := probeAndRecord(ctx, host.NewClient(mock), store, probeJob{
		credential:   core.Credential{AuthIndex: "auth-pro", Provider: core.ProviderAntigravity},
		authMaterial: authMaterial{accessToken: "token", projectID: "project"},
	}, now, 10, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected probe results")
	}
	for _, result := range results {
		if result.Status != antigravity.StatusReady {
			continue
		}
		if result.PlanType != core.PlanTypePro {
			t.Fatalf("fallback plan type = %q; want pro from 5h window inference", result.PlanType)
		}
	}
}

func TestRefreshPlanTiersSkipsDisabledAccountsInBatch(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	doer := &planTierMockDoer{response: host.HTTPResponse{StatusCode: http.StatusOK}}
	credentials := []core.Credential{{
		AuthIndex: "auth-disabled",
		Provider:  core.ProviderAntigravity,
		Disabled:  true,
	}}

	changed, err := refreshPlanTiers(ctx, doer, credentials, map[string]authMaterial{
		"auth-disabled": {accessToken: "token"},
	}, store, now, false, 4)
	if err != nil {
		t.Fatal(err)
	}
	if changed || doer.callCount() != 0 {
		t.Fatalf("disabled account refreshed: changed=%v calls=%d", changed, doer.callCount())
	}
}

func TestProbeAndRecordUsesCachedAuthoritativeTier(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	mock := &probeEvidenceMockHost{
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{"groups":[
				{"displayName":"Gemini Models","buckets":[
					{"window":"5h","remainingFraction":0.8,"resetTime":"2026-11-11T17:00:00Z"},
					{"window":"weekly","remainingFraction":0.6,"resetTime":"2026-11-18T00:00:00Z"}
				]},
				{"displayName":"Claude and GPT models","buckets":[
					{"window":"5h","remainingFraction":0.5,"resetTime":"2026-11-11T17:00:00Z"},
					{"window":"weekly","remainingFraction":0.4,"resetTime":"2026-11-18T00:00:00Z"}
				]}
			]}`),
		},
	}
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "plan-tier.json"))
	if err != nil {
		t.Fatal(err)
	}
	store.SetPlanTier("auth-ultra", state.PlanTierRecord{
		PlanType:   core.PlanTypeUltra,
		ObservedAt: now,
	})

	results, err := probeAndRecord(ctx, host.NewClient(mock), store, probeJob{
		credential:   core.Credential{AuthIndex: "auth-ultra", Provider: core.ProviderAntigravity},
		authMaterial: authMaterial{accessToken: "token", projectID: "project"},
	}, now, 10, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected probe results")
	}
	for _, result := range results {
		if result.PlanType != core.PlanTypeUltra {
			t.Fatalf("probe result plan type = %q; want ultra", result.PlanType)
		}
	}
	if len(mock.httpCalls) != 1 || mock.httpCalls[0].URL != antigravity.RetrieveUserQuotaSummaryDailyURL {
		t.Fatalf("probe made unexpected HTTP calls: %+v", mock.httpCalls)
	}
}
