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
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/state"
)

type mockRunner struct {
	runFunc         func(ctx context.Context, req management.RunRequest) (apply.Result, error)
	resetFunc       func(ctx context.Context) (map[string]any, error)
	statusFunc      func(ctx context.Context) (management.StatusInfo, error)
	snapshotFunc    func(ctx context.Context) (apply.DualGroupSnapshot, error)
	diagnosticsFunc func(ctx context.Context) (map[string]any, error)
	scheduleConfig  state.ScheduleConfig
	dynamicConfig   state.DynamicConfig
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

func (m *mockRunner) Status(ctx context.Context) (management.StatusInfo, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return management.StatusInfo{}, nil
}

func (m *mockRunner) LatestSnapshot(ctx context.Context) (apply.DualGroupSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx)
	}
	return apply.DualGroupSnapshot{}, nil
}

func (m *mockRunner) SyncHost(ctx context.Context, modelGroup config.AntigravityModelGroup) (apply.DualGroupSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx)
	}
	return apply.DualGroupSnapshot{}, nil
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

func (m *mockRunner) GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error) {
	return []state.QuotaSample{
		{
			ObservedAt:     time.Now().UTC(),
			ShortWindowRem: 85,
			LongWindowRem:  80,
		},
	}, nil
}

func TestHandler_Run_Probe_Success(t *testing.T) {
	called := false
	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			called = true
			if req.Mode != "probe" {
				t.Errorf("expected mode 'probe', got %q", req.Mode)
			}
			if req.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
				t.Errorf("expected model group 'claude_gpt', got %q", req.AntigravityModelGroup)
			}
			return apply.Result{
				Attempted: 2,
				Succeeded: 2,
			}, nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/run?mode=probe&antigravity_model_group=claude_gpt", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}
	if !called {
		t.Errorf("expected runner.Run to be called")
	}

	var res apply.Result
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if res.Attempted != 2 || res.Succeeded != 2 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestHandler_Run_Apply_Success(t *testing.T) {
	called := false
	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			called = true
			if req.Mode != "apply" {
				t.Errorf("expected mode 'apply', got %q", req.Mode)
			}
			return apply.Result{
				Attempted: 1,
				Succeeded: 1,
			}, nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/run?mode=apply&auth_index=auth_1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if !called {
		t.Errorf("expected runner.Run to be called")
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
	if errResp["error"] != "invalid mode: must be 'apply' or 'probe'" {
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
	req2 := httptest.NewRequest(http.MethodPost, "/run?mode=apply", nil)
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
		snapshotFunc: func(ctx context.Context) (apply.DualGroupSnapshot, error) {
			primary := apply.PlanSnapshot{
				TotalItems:   1,
				TotalChanges: 1,
				Items: []apply.SnapshotItem{
					{
						Identity:  apply.SnapshotIdentity{Email: "owner@example.com", AuthIndex: "auth_123"},
						Name:      "test-account",
						AuthIndex: "au***23",
						Current:   apply.Target{Priority: 100},
						Target:    apply.Target{Priority: 999},
						Reason:    "fresh boosted",
					},
				},
			}
			predicted := apply.PlanSnapshot{Items: []apply.SnapshotItem{}, Changes: []apply.SnapshotChange{}}
			return testDualGroupSnapshot(time.Now().UTC(), primary, predicted), nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/snapshot/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var snap apply.DualGroupSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot failed: %v", err)
	}
	if snap.ActiveModelGroup != "gemini" {
		t.Errorf("expected active_model_group 'gemini', got %q", snap.ActiveModelGroup)
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
	req := httptest.NewRequest(http.MethodPost, "/run?mode=apply", nil)
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
			Paused:        true,
			WindowEnabled: true,
			WindowStart:   "09:00",
			WindowEnd:     "23:00",
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
	if cfg.WindowStart != "09:00" {
		t.Errorf("expected WindowStart=09:00, got %q", cfg.WindowStart)
	}
}

func TestHandler_SetScheduleConfig(t *testing.T) {
	runner := &mockRunner{}
	handler := management.NewHandler(runner)

	body := `{"paused":false,"window_enabled":true,"window_start":"08:30","window_end":"23:30"}`
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
	if !cfg.WindowEnabled {
		t.Error("expected WindowEnabled=true")
	}
	if cfg.WindowStart != "08:30" || cfg.WindowEnd != "23:30" {
		t.Errorf("unexpected window: %q-%q", cfg.WindowStart, cfg.WindowEnd)
	}

	// Verify it was stored on the mock
	if runner.scheduleConfig.WindowStart != "08:30" {
		t.Errorf("expected mock to store WindowStart=08:30, got %q", runner.scheduleConfig.WindowStart)
	}
}

func TestHandler_SetScheduleConfig_InvalidWindow(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})
	body := `{"window_enabled":true,"window_start":"25:00","window_end":"23:00"}`
	req := httptest.NewRequest(http.MethodPost, "/schedule/config", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
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
			AntigravityModelGroup:    "claude_gpt",
			MaxConcurrency:           8,
			MinChange:                2,
			UrgencyTolerance:         0.05,
			RateLimitCooldownMinutes: 5,
			PriorityRules: state.PriorityRulesConfig{
				BoostStartPriority:  990,
				NormalStartPriority: 150,
			},
			Schedule: state.ScheduleConfig{
				Paused:        false,
				WindowEnabled: true,
				WindowStart:   "08:00",
				WindowEnd:     "22:00",
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
	if cfg.AntigravityModelGroup != "claude_gpt" {
		t.Errorf("expected AntigravityModelGroup=claude_gpt, got %q", cfg.AntigravityModelGroup)
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

	body := `{"auto_apply":true,"interval":"20m","antigravity_model_group":"gemini","max_concurrency":4,"min_change":3,"urgency_tolerance":0.08,"rate_limit_cooldown_minutes":10,"quota_sample_capacity":6,"priority_rules":{"boost_start_priority":995,"normal_start_priority":120},"schedule":{"paused":false,"window_enabled":true,"window_start":"09:00","window_end":"18:00"}}`
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
	if cfg.MinChange != 3 {
		t.Errorf("expected MinChange=3, got %d", cfg.MinChange)
	}
	if cfg.UrgencyTolerance != 0.08 {
		t.Errorf("expected UrgencyTolerance=0.08, got %v", cfg.UrgencyTolerance)
	}
	if cfg.RateLimitCooldownMinutes != 10 {
		t.Errorf("expected RateLimitCooldownMinutes=10, got %d", cfg.RateLimitCooldownMinutes)
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
		snapshotFunc: func(ctx context.Context) (apply.DualGroupSnapshot, error) {
			primary := apply.PlanSnapshot{
				TotalItems: 1,
				Items: []apply.SnapshotItem{
					{
						Name:      "test-account",
						AuthIndex: "auth_sync",
						Current:   apply.Target{Priority: 100},
						Target:    apply.Target{Priority: 99},
					},
				},
			}
			predicted := apply.PlanSnapshot{}
			return testDualGroupSnapshot(time.Now().UTC(), primary, predicted), nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var snap apply.DualGroupSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot failed: %v", err)
	}
	if snap.ActiveModelGroup != "gemini" {
		t.Errorf("expected active_model_group 'gemini', got %q", snap.ActiveModelGroup)
	}
}

func testDualGroupSnapshot(observedAt time.Time, primary, predicted apply.PlanSnapshot) apply.DualGroupSnapshot {
	return apply.DualGroupSnapshot{
		ActiveModelGroup: "gemini",
		ObservedAt:       observedAt,
		Groups: map[string]apply.GroupSnapshot{
			"gemini":     {Items: primary.Items, Changes: primary.Changes},
			"claude_gpt": {Items: predicted.Items, Changes: predicted.Changes},
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
