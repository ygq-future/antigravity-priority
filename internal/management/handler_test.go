package management_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/state"
)

type mockRunner struct {
	runFunc         func(ctx context.Context, req management.RunRequest) (apply.Result, error)
	resetFunc       func(ctx context.Context) (map[string]any, error)
	snapshotFunc    func(ctx context.Context) (apply.SnapshotSet, error)
	diagnosticsFunc func(ctx context.Context) (map[string]any, error)
	scheduleConfig  state.ScheduleConfig
	dynamicConfig   state.DynamicConfig
	probeOutcomes   []management.ProbeOutcome
	requestHistory  []state.RequestHistoryEntry
	customTiers     map[string]int
	warmupSwitches  map[string]bool
}

func (m *mockRunner) Run(ctx context.Context, req management.RunRequest) (apply.Result, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, req)
	}
	return apply.Result{}, nil
}

func (m *mockRunner) Reset(ctx context.Context) (map[string]any, error) {
	if m.resetFunc != nil {
		return m.resetFunc(ctx)
	}
	return map[string]any{"ok": true, "reset_count": 0}, nil
}

func (m *mockRunner) LatestSnapshot(ctx context.Context) (apply.SnapshotSet, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx)
	}
	return apply.SnapshotSet{}, nil
}

func (m *mockRunner) SyncHost(ctx context.Context) (apply.SnapshotSet, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx)
	}
	return apply.SnapshotSet{}, nil
}

func (m *mockRunner) Diagnostics(ctx context.Context) (map[string]any, error) {
	if m.diagnosticsFunc != nil {
		return m.diagnosticsFunc(ctx)
	}
	return map[string]any{}, nil
}

func (m *mockRunner) GetScheduleConfig(ctx context.Context) (state.ScheduleConfig, error) {
	return m.scheduleConfig, nil
}

func (m *mockRunner) SetScheduleConfig(ctx context.Context, cfg state.ScheduleConfig) error {
	m.scheduleConfig = cfg
	return nil
}

func (m *mockRunner) GetDynamicConfig(ctx context.Context) (state.DynamicConfig, error) {
	return m.dynamicConfig, nil
}

func (m *mockRunner) SetDynamicConfig(ctx context.Context, cfg state.DynamicConfig) error {
	m.dynamicConfig = cfg
	m.scheduleConfig = cfg.Schedule
	return nil
}

func (m *mockRunner) GetCustomTiers(ctx context.Context) (map[string]int, error) {
	if m.customTiers == nil {
		return map[string]int{}, nil
	}
	return m.customTiers, nil
}

func (m *mockRunner) SetCustomTier(ctx context.Context, authIndex string, tier int) error {
	if m.customTiers == nil {
		m.customTiers = make(map[string]int)
	}
	if tier <= 0 {
		delete(m.customTiers, authIndex)
	} else {
		m.customTiers[authIndex] = tier
	}
	return nil
}

func (m *mockRunner) SetWarmupCredentialState(ctx context.Context, authIndex, group string, enabled bool) error {
	if m.warmupSwitches == nil {
		m.warmupSwitches = make(map[string]bool)
	}
	m.warmupSwitches[authIndex+"|"+group] = enabled
	return nil
}

func (m *mockRunner) SetModelExcluded(ctx context.Context, model string, excluded bool) ([]string, error) {
	norm := strings.ToLower(strings.TrimSpace(model))
	var next []string
	for _, existing := range m.dynamicConfig.ExcludedModels {
		if strings.ToLower(strings.TrimSpace(existing)) != norm {
			next = append(next, existing)
		}
	}
	if excluded && norm != "" {
		next = append(next, norm)
	}
	m.dynamicConfig.ExcludedModels = next
	return append([]string(nil), next...), nil
}

func (m *mockRunner) GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error) {
	return []state.QuotaSample{
		{
			ObservedAt:     time.Now().UTC(),
			ShortWindowRem: 85,
			LongWindowRem:  80,
		},
	}, nil
}

func (m *mockRunner) GetProbeSamples(ctx context.Context, probeRoundID, modelGroup string) ([]state.ProbeSampleRecord, error) {
	return []state.ProbeSampleRecord{{
		AuthIndex:  "auth_test",
		ModelGroup: modelGroup,
		Sample: state.QuotaSample{
			ObservedAt:     time.Now().UTC(),
			ShortWindowRem: 85,
			LongWindowRem:  80,
		},
	}}, nil
}

func (m *mockRunner) LastProbeOutcomes(ctx context.Context) []management.ProbeOutcome {
	return m.probeOutcomes
}

func (m *mockRunner) GetRequestHistory(ctx context.Context) ([]state.RequestHistoryEntry, error) {
	return m.requestHistory, nil
}

func (m *mockRunner) ClearRequestHistory(ctx context.Context) error {
	m.requestHistory = nil
	return nil
}

func TestHandler_Run_Probe_Success(t *testing.T) {
	called := false
	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			called = true
			return apply.Result{
				Attempted: 2,
				Succeeded: 2,
			}, nil
		},
		probeOutcomes: []management.ProbeOutcome{
			{AuthIndex: "auth_1", ModelGroup: "claude_gpt", Status: "success", HTTPStatusCode: 200},
			{AuthIndex: "auth_2", ModelGroup: "claude_gpt", Status: "failed", Error: "retrieve quota summary status 403", HTTPStatusCode: 403},
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/run?mode=probe", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Errorf("expected runner.Run to be called")
	}

	var res struct {
		apply.Result
		ProbeOutcomes []management.ProbeOutcome `json:"probe_outcomes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if res.Attempted != 2 || res.Succeeded != 2 {
		t.Errorf("unexpected result: %+v", res.Result)
	}
	if len(res.ProbeOutcomes) != 2 {
		t.Fatalf("expected 2 probe outcomes, got %d", len(res.ProbeOutcomes))
	}
	if res.ProbeOutcomes[1].Status != "failed" || res.ProbeOutcomes[1].HTTPStatusCode != 403 {
		t.Errorf("unexpected outcome: %+v", res.ProbeOutcomes[1])
	}
}

func TestHandler_Run_InvalidMode(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/run?mode=invalid", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
	var errResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal error resp failed: %v", err)
	}
	if errResp["error"] != "invalid mode: must be 'probe'" {
		t.Errorf("unexpected error message: %q", errResp["error"])
	}
}

func TestHandler_Run_ConcurrencyConflict(t *testing.T) {
	blockChan := make(chan struct{})
	doneChan := make(chan struct{})

	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			close(doneChan)
			<-blockChan
			return apply.Result{}, nil
		},
	}

	handler := management.NewHandler(runner)

	// Start first request
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/run?mode=probe", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}()

	<-doneChan

	// Second request should conflict
	req2 := httptest.NewRequest(http.MethodPost, "/run?mode=probe", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	close(blockChan)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected status 409 Conflict, got %d, body=%s", rec2.Code, rec2.Body.String())
	}
}

func TestHandler_Diagnostics_Redaction(t *testing.T) {
	runner := &mockRunner{
		diagnosticsFunc: func(ctx context.Context) (map[string]any, error) {
			return map[string]any{
				"authorization": "Bearer secret_token_123",
				"api_key":       "my-secret-api-key",
				"status":        "ready",
				"nested": map[string]any{
					"token": "sensitive_nested_token",
					"value": "normal_value",
				},
				"list": []any{
					map[string]any{"cookie": "session_secret"},
					"plain_text",
				},
			}, nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/diagnostics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var diag map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &diag); err != nil {
		t.Fatalf("decode diagnostics failed: %v", err)
	}

	if diag["authorization"] != "[REDACTED]" {
		t.Errorf("expected authorization to be redacted, got %v", diag["authorization"])
	}
	if diag["api_key"] != "[REDACTED]" {
		t.Errorf("expected api_key to be redacted, got %v", diag["api_key"])
	}
	if diag["status"] != "ready" {
		t.Errorf("expected status 'ready', got %v", diag["status"])
	}

	nested, _ := diag["nested"].(map[string]any)
	if nested["token"] != "[REDACTED]" {
		t.Errorf("expected nested token to be redacted, got %v", nested["token"])
	}
	if nested["value"] != "normal_value" {
		t.Errorf("expected nested value 'normal_value', got %v", nested["value"])
	}
}

func TestHandler_Snapshot_Latest(t *testing.T) {
	runner := &mockRunner{
		snapshotFunc: func(ctx context.Context) (apply.SnapshotSet, error) {
			primary := apply.PlanSnapshot{
				TotalItems: 1,
				Items: []apply.SnapshotItem{
					{
						Identity:  apply.SnapshotIdentity{Email: "owner@example.com", AuthIndex: "auth_123"},
						Name:      "test-account",
						AuthIndex: "au***23",
						Host:      apply.Target{Priority: 100},
						Target:    apply.Target{Priority: 999},
						Reason:    "fresh boosted",
					},
				},
			}
			predicted := apply.PlanSnapshot{Items: []apply.SnapshotItem{}}
			return testSnapshotSet(time.Now().UTC(), primary, predicted), nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/snapshot/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var snap apply.SnapshotSet
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot failed: %v", err)
	}
	group, ok := snap.Groups["gemini"]
	if !ok {
		t.Fatal("expected 'gemini' group in snapshot")
	}
	if len(group.Items) != 1 {
		t.Errorf("expected 1 item in gemini group, got %d", len(group.Items))
	}
	if group.Items[0].Email != "owner@example.com" || group.Items[0].AuthIndex != "auth_123" {
		t.Errorf("expected full management identity, got email=%q auth_index=%q", group.Items[0].Email, group.Items[0].AuthIndex)
	}
}

func TestHandler_Status_HTML(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set(management.RouteSourceHeader, "resource")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected content-type text/html, got %q", ct)
	}
}

func TestHandler_ResourceRoute_BoundaryProtection(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	req.Header.Set(management.RouteSourceHeader, "resource")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 on resource /run, got %d", rec.Code)
	}
}

func TestHandler_NotFound(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestHandler_RunnerError(t *testing.T) {
	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			return apply.Result{}, errors.New("upstream failure")
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/run?mode=probe", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}

func TestHandler_Reset_Success(t *testing.T) {
	called := false
	runner := &mockRunner{
		resetFunc: func(ctx context.Context) (map[string]any, error) {
			called = true
			return map[string]any{
				"ok":          true,
				"message":     "reset 3 credentials",
				"reset_count": 3,
			}, nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/reset", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected runner.Reset to be called")
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode reset response failed: %v", err)
	}
	if res["ok"] != true {
		t.Errorf("expected ok=true, got %v", res["ok"])
	}
}

func TestHandler_GetScheduleConfig(t *testing.T) {
	runner := &mockRunner{
		scheduleConfig: state.ScheduleConfig{
			Paused: true,
		},
	}
	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/schedule/config", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var cfg state.ScheduleConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode schedule config failed: %v", err)
	}
	if !cfg.Paused {
		t.Error("expected Paused=true")
	}
}

func TestHandler_SetScheduleConfig(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	body := `{"paused":true}`
	req := httptest.NewRequest(http.MethodPost, "/schedule/config", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var cfg state.ScheduleConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !cfg.Paused {
		t.Error("expected Paused=true")
	}

	// Verify it was stored on the mock
	if !runner.scheduleConfig.Paused {
		t.Error("expected mock to store Paused=true")
	}
}

func TestHandler_SetScheduleConfig_InvalidJSON(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/schedule/config", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandler_GetDynamicConfig(t *testing.T) {
	runner := &mockRunner{
		dynamicConfig: state.DynamicConfig{
			AutoApply:                true,
			Interval:                 "30m",
			MaxConcurrency:           8,
			UrgencyTolerance:         0.05,
			RateLimitCooldownMinutes: 5,
			PriorityRules: state.PriorityRulesConfig{
				BoostStartPriority:  990,
				NormalStartPriority: 150,
			},
			Schedule: state.ScheduleConfig{
				Paused: false,
			},
		},
	}
	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/runtime-config", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var cfg state.DynamicConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode dynamic config failed: %v", err)
	}
	if !cfg.AutoApply {
		t.Error("expected AutoApply=true")
	}
	if cfg.Interval != "30m" {
		t.Errorf("expected Interval=30m, got %q", cfg.Interval)
	}
	if cfg.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency=8, got %d", cfg.MaxConcurrency)
	}
	if cfg.PriorityRules.BoostStartPriority != 990 {
		t.Errorf("expected BoostStartPriority=990, got %d", cfg.PriorityRules.BoostStartPriority)
	}
	if cfg.UrgencyTolerance != 0.05 {
		t.Errorf("expected UrgencyTolerance=0.05, got %v", cfg.UrgencyTolerance)
	}
	if cfg.RateLimitCooldownMinutes != 5 {
		t.Errorf("expected RateLimitCooldownMinutes=5, got %d", cfg.RateLimitCooldownMinutes)
	}
	if strings.Contains(rec.Body.String(), `"priority_rules":{"enabled"`) {
		t.Fatalf("GET /runtime-config exposed removed priority_rules.enabled: %s", rec.Body.String())
	}
}

func TestHandler_SetDynamicConfig_Success(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	body := `{"auto_apply":true,"interval":"20m","max_concurrency":4,"urgency_tolerance":0.08,"rate_limit_cooldown_minutes":10,"quota_sample_capacity":6,"priority_rules":{"boost_start_priority":995,"normal_start_priority":120},"schedule":{"paused":false}}`
	req := httptest.NewRequest(http.MethodPost, "/runtime-config", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var cfg state.DynamicConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if cfg.Interval != "20m" {
		t.Errorf("expected Interval=20m, got %q", cfg.Interval)
	}
	if cfg.UrgencyTolerance != 0.08 {
		t.Errorf("expected UrgencyTolerance=0.08, got %v", cfg.UrgencyTolerance)
	}
	if cfg.RateLimitCooldownMinutes != 10 {
		t.Errorf("expected RateLimitCooldownMinutes=10, got %d", cfg.RateLimitCooldownMinutes)
	}
}

func TestHandler_DynamicConfig_WarmupRoundTrip(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	body := `{"auto_apply":true,"interval":"15m","max_concurrency":6,"urgency_tolerance":0.05,"rate_limit_cooldown_minutes":5,"quota_sample_capacity":6,"priority_rules":{"boost_start_priority":999,"normal_start_priority":100},"schedule":{"paused":false},"warmup":{"gemini_enabled":true,"gemini_model":"gemini-2.0-flash","claude_enabled":true,"claude_model":"claude-3-7-sonnet"}}`
	req := httptest.NewRequest(http.MethodPost, "/runtime-config", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var cfg state.DynamicConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !cfg.Warmup.GeminiEnabled || cfg.Warmup.GeminiModel != "gemini-2.0-flash" {
		t.Fatalf("expected gemini enabled with gemini-2.0-flash, got: %+v", cfg.Warmup)
	}
	if !cfg.Warmup.ClaudeEnabled || cfg.Warmup.ClaudeModel != "claude-3-7-sonnet" {
		t.Fatalf("expected claude enabled with claude-3-7-sonnet, got: %+v", cfg.Warmup)
	}

}
func TestHandler_SetDynamicConfig_InvalidJSON(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	req := httptest.NewRequest(http.MethodPost, "/runtime-config", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandler_Sync_Success(t *testing.T) {
	runner := &mockRunner{
		snapshotFunc: func(ctx context.Context) (apply.SnapshotSet, error) {
			primary := apply.PlanSnapshot{
				TotalItems: 1,
				Items: []apply.SnapshotItem{
					{
						Name:      "test-account",
						AuthIndex: "auth_sync",
						Host:      apply.Target{Priority: 100},
						Target:    apply.Target{Priority: 99},
					},
				},
			}
			predicted := apply.PlanSnapshot{}
			return testSnapshotSet(time.Now().UTC(), primary, predicted), nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var snap apply.SnapshotSet
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot failed: %v", err)
	}
}

func testSnapshotSet(observedAt time.Time, primary, predicted apply.PlanSnapshot) apply.SnapshotSet {
	return apply.SnapshotSet{
		ObservedAt: observedAt,
		Groups: map[string]apply.PoolSnapshot{
			"gemini":     {Items: primary.Items},
			"claude_gpt": {Items: predicted.Items},
		},
	}
}

func TestHandler_GetSamples_Success(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/samples?auth_index=auth_test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode samples failed: %v", err)
	}
	if res["auth_index"] != "auth_test" {
		t.Errorf("expected auth_index 'auth_test', got %v", res["auth_index"])
	}
	groups, ok := res["groups"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'groups' object, got %v", res["groups"])
	}
	geminiGroup, ok := groups["gemini"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'gemini' group, got %v", groups["gemini"])
	}
	samples, ok := geminiGroup["samples"].([]any)
	if !ok || len(samples) != 1 {
		t.Fatalf("expected 1 sample for gemini, got %v", geminiGroup["samples"])
	}
}

func TestHandler_GetProbeSamples_Success(t *testing.T) {
	runner := &mockRunner{snapshotFunc: func(context.Context) (apply.SnapshotSet, error) {
		return apply.SnapshotSet{Groups: map[string]apply.PoolSnapshot{
			"gemini": {Items: []apply.SnapshotItem{{Identity: apply.SnapshotIdentity{
				AuthIndex: "auth_test",
				Email:     "probe@example.com",
			}}}},
		}}, nil
	}}
	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/samples?probe_round_id=round-1&model_group=gemini", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode probe samples failed: %v", err)
	}
	if result["probe_round_id"] != "round-1" || result["model_group"] != "gemini" {
		t.Fatalf("unexpected probe sample metadata: %+v", result)
	}
	records, ok := result["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("expected one probe sample record, got %v", result["records"])
	}
	record, _ := records[0].(map[string]any)
	if record["email"] != "probe@example.com" {
		t.Fatalf("probe sample email=%v, want full CPA Host email", record["email"])
	}
}

func TestHandler_RequestHistory_Endpoints(t *testing.T) {
	runner := &mockRunner{
		requestHistory: []state.RequestHistoryEntry{
			{
				ID:           "req-1",
				Model:        "claude-opus-4-6-thinking",
				ModelPool:    "claude_gpt",
				AuthIndex:    "auth-1",
				Email:        "user@example.com",
				InputTokens:  1000,
				OutputTokens: 200,
				TotalTokens:  1200,
				StatusCode:   200,
			},
		},
	}
	handler := management.NewHandler(runner)

	// 1. GET /request-history
	req := httptest.NewRequest(http.MethodGet, "/request-history", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var res struct {
		OK      bool                        `json:"ok"`
		Total   int                         `json:"total"`
		Records []state.RequestHistoryEntry `json:"records"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || !res.OK {
		t.Fatalf("decode failed: %v, body=%s", err, rec.Body.String())
	}
	if res.Total != 1 || len(res.Records) != 1 || res.Records[0].Model != "claude-opus-4-6-thinking" {
		t.Fatalf("unexpected records: %+v", res)
	}

	// 2. POST /request-history/clear
	clearReq := httptest.NewRequest(http.MethodPost, "/request-history/clear", nil)
	clearRec := httptest.NewRecorder()
	handler.ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on clear, got %d", clearRec.Code)
	}

	// 3. Verify history is empty
	req2 := httptest.NewRequest(http.MethodGet, "/request-history", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	var res2 struct {
		Total   int                         `json:"total"`
		Records []state.RequestHistoryEntry `json:"records"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &res2)
	if res2.Total != 0 || len(res2.Records) != 0 {
		t.Fatalf("expected 0 records after clear, got %d", res2.Total)
	}
}

func TestHandler_CustomTier_GetAndSet(t *testing.T) {
	runner := &mockRunner{
		customTiers: map[string]int{
			"auth-1": 1,
		},
	}
	handler := management.NewHandler(runner)

	// 1. GET /custom-tier
	req := httptest.NewRequest(http.MethodGet, "/custom-tier", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var getRes struct {
		OK          bool           `json:"ok"`
		CustomTiers map[string]int `json:"custom_tiers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getRes); err != nil || !getRes.OK {
		t.Fatalf("decode failed: %v, body=%s", err, rec.Body.String())
	}
	if getRes.CustomTiers["auth-1"] != 1 {
		t.Errorf("expected auth-1 to be tier 1, got %v", getRes.CustomTiers)
	}

	// 2. POST /custom-tier
	setBody := `{"auth_index":"auth-2","tier":2}`
	postReq := httptest.NewRequest(http.MethodPost, "/custom-tier", strings.NewReader(setBody))
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on post, got %d, body=%s", postRec.Code, postRec.Body.String())
	}
	if runner.customTiers["auth-2"] != 2 {
		t.Errorf("expected runner customTiers auth-2 to be 2, got %v", runner.customTiers)
	}
}

func TestHandler_WarmupCredential_EnableDisable(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	// 1. Turn one credential's card state off for the Gemini pool.
	body := `{"auth_index":"auth-1","group":"gemini","enabled":false}`
	req := httptest.NewRequest(http.MethodPost, "/warmup-credential", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if enabled, ok := runner.warmupSwitches["auth-1|gemini"]; !ok || enabled {
		t.Fatalf("expected auth-1 gemini state stored as disabled, got %v", runner.warmupSwitches)
	}

	// 2. Re-enable it through the management prefix.
	body = `{"auth_index":"auth-1","group":"gemini","enabled":true}`
	req = httptest.NewRequest(http.MethodPost, "/v0/management/plugins/antigravity-priority/warmup-credential", strings.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if enabled := runner.warmupSwitches["auth-1|gemini"]; !enabled {
		t.Fatalf("expected auth-1 gemini state stored as enabled, got %v", runner.warmupSwitches)
	}

	// 3. Missing enabled flag is a client error.
	req = httptest.NewRequest(http.MethodPost, "/warmup-credential", strings.NewReader(`{"auth_index":"auth-1","group":"gemini"}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when enabled is omitted, got %d", rec.Code)
	}

	// 4. Unknown model group is a client error.
	req = httptest.NewRequest(http.MethodPost, "/warmup-credential", strings.NewReader(`{"auth_index":"auth-1","group":"nope","enabled":true}`))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown group, got %d", rec.Code)
	}
}

func TestHandler_ExcludedModels_GetAndSet(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	// 1. Initial GET /excluded-models returns empty array
	getReq := httptest.NewRequest(http.MethodGet, "/excluded-models", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d, body=%s", getRec.Code, getRec.Body.String())
	}
	var getRes struct {
		OK             bool     `json:"ok"`
		ExcludedModels []string `json:"excluded_models"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &getRes); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if !getRes.OK || len(getRes.ExcludedModels) != 0 {
		t.Fatalf("expected ok=true and empty excluded_models, got %+v", getRes)
	}

	// 2. POST /v0/management/plugins/antigravity-priority/excluded-models excludes a model
	postReq := httptest.NewRequest(
		http.MethodPost,
		"/v0/management/plugins/antigravity-priority/excluded-models",
		strings.NewReader(`{"model":"Gemini-3-Flash","excluded":true}`),
	)
	postRec := httptest.NewRecorder()
	handler.ServeHTTP(postRec, postReq)
	if postRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST exclude, got %d, body=%s", postRec.Code, postRec.Body.String())
	}
	var postRes struct {
		OK             bool     `json:"ok"`
		Model          string   `json:"model"`
		Excluded       bool     `json:"excluded"`
		ExcludedModels []string `json:"excluded_models"`
	}
	if err := json.Unmarshal(postRec.Body.Bytes(), &postRes); err != nil {
		t.Fatalf("decode POST response: %v", err)
	}
	if !postRes.OK || postRes.Model != "gemini-3-flash" || !postRes.Excluded || len(postRes.ExcludedModels) != 1 || postRes.ExcludedModels[0] != "gemini-3-flash" {
		t.Fatalf("unexpected POST exclude response: %+v", postRes)
	}

	// 3. POST restore removes the model
	restoreReq := httptest.NewRequest(http.MethodPost, "/excluded-models", strings.NewReader(`{"model":"gemini-3-flash","excluded":false}`))
	restoreRec := httptest.NewRecorder()
	handler.ServeHTTP(restoreRec, restoreReq)
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on POST restore, got %d, body=%s", restoreRec.Code, restoreRec.Body.String())
	}
	if len(runner.dynamicConfig.ExcludedModels) != 0 {
		t.Fatalf("expected empty excluded_models after restore, got %v", runner.dynamicConfig.ExcludedModels)
	}

	// 4. Invalid payloads return 400
	for _, badBody := range []string{`{"model":"","excluded":true}`, `{"model":"gemini-3-flash"}`, `{bad`} {
		badReq := httptest.NewRequest(http.MethodPost, "/excluded-models", strings.NewReader(badBody))
		badRec := httptest.NewRecorder()
		handler.ServeHTTP(badRec, badReq)
		if badRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for body %s, got %d", badBody, badRec.Code)
		}
	}
}
