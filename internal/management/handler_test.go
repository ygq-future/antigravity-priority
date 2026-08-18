package management_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/management"
)

type mockRunner struct {
	runFunc         func(ctx context.Context, req management.RunRequest) (apply.Result, error)
	statusFunc      func(ctx context.Context) (management.StatusInfo, error)
	snapshotFunc    func(ctx context.Context) (apply.PlanSnapshot, error)
	diagnosticsFunc func(ctx context.Context) (map[string]any, error)
}

func (m *mockRunner) Run(ctx context.Context, req management.RunRequest) (apply.Result, error) {
	if m.runFunc != nil {
		return m.runFunc(ctx, req)
	}
	return apply.Result{}, nil
}

func (m *mockRunner) Status(ctx context.Context) (management.StatusInfo, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return management.StatusInfo{}, nil
}

func (m *mockRunner) LatestSnapshot(ctx context.Context) (apply.PlanSnapshot, error) {
	if m.snapshotFunc != nil {
		return m.snapshotFunc(ctx)
	}
	return apply.PlanSnapshot{}, nil
}

func (m *mockRunner) Diagnostics(ctx context.Context) (map[string]any, error) {
	if m.diagnosticsFunc != nil {
		return m.diagnosticsFunc(ctx)
	}
	return map[string]any{}, nil
}

func TestHandler_Run_DryRun_Success(t *testing.T) {
	called := false
	runner := &mockRunner{
		runFunc: func(ctx context.Context, req management.RunRequest) (apply.Result, error) {
			called = true
			if req.Mode != "dry-run" {
				t.Errorf("expected mode 'dry-run', got %q", req.Mode)
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
	req := httptest.NewRequest(http.MethodPost, "/run?mode=dry-run&antigravity_model_group=claude_gpt", nil)
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
	if errResp["error"] != "invalid mode: must be 'dry-run' or 'apply'" {
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
		req := httptest.NewRequest(http.MethodPost, "/run?mode=dry-run", nil)
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
		snapshotFunc: func(ctx context.Context) (apply.PlanSnapshot, error) {
			return apply.PlanSnapshot{
				TotalItems:   1,
				TotalChanges: 1,
				Items: []apply.SnapshotItem{
					{
						Name:      "test-account",
						AuthIndex: "auth_123",
						Current:   apply.Target{Priority: 100},
						Target:    apply.Target{Priority: 999},
						Reason:    "fresh boosted",
					},
				},
			}, nil
		},
	}

	handler := management.NewHandler(runner)
	req := httptest.NewRequest(http.MethodGet, "/snapshot/latest", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var snap apply.PlanSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot failed: %v", err)
	}
	if snap.TotalItems != 1 {
		t.Errorf("expected 1 item, got %d", snap.TotalItems)
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
