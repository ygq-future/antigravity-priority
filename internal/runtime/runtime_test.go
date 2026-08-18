package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

type mockHost struct {
	mu                sync.Mutex
	files             []host.AuthFile
	authDocs          map[string]host.AuthDocument
	patchedPriorities map[string]int
	patchedDisabled   map[string]bool
	httpResponse      host.HTTPResponse
	httpErr           error
	httpCalls         []host.HTTPRequest
}

func newMockHost() *mockHost {
	return &mockHost{
		authDocs:          make(map[string]host.AuthDocument),
		patchedPriorities: make(map[string]int),
		patchedDisabled:   make(map[string]bool),
		httpResponse: host.HTTPResponse{
			StatusCode: http.StatusOK,
			Body: []byte(`{
				"models": {
					"gemini-2.5-pro": {
						"quotaInfo": {
							"remainingFraction": 0.8,
							"resetTime": "2026-08-18T17:00:00Z"
						}
					},
					"gemini-2.5-flash": {
						"quotaInfo": {
							"remainingFraction": 0.9,
							"resetTime": "2026-08-25T12:00:00Z"
						}
					}
				}
			}`),
		},
	}
}

func (m *mockHost) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]host.AuthFile(nil), m.files...), nil
}

func (m *mockHost) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc, ok := m.authDocs[authIndex]; ok {
		return doc, nil
	}
	return host.AuthDocument{
		AuthIndex: authIndex,
		JSON:      json.RawMessage(`{"access_token":"mock_token_123","project_id":"mock-project","account":"test@example.com"}`),
	}, nil
}

func (m *mockHost) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	return host.RuntimeAuth{AuthIndex: authIndex}, nil
}

func (m *mockHost) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	return nil
}

func (m *mockHost) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.httpCalls = append(m.httpCalls, req)
	if m.httpErr != nil {
		return host.HTTPResponse{}, m.httpErr
	}
	return m.httpResponse, nil
}

func (m *mockHost) PatchPriority(ctx context.Context, authIndex string, priority int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchedPriorities[authIndex] = priority
	return nil
}

func (m *mockHost) PatchDisabled(ctx context.Context, name string, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchedDisabled[name] = disabled
	return nil
}

type testClock struct {
	now time.Time
}

func (t *testClock) Now() time.Time {
	return t.now
}

type testSleeper struct{}

func (testSleeper) Sleep(ctx context.Context, d time.Duration) error {
	return nil
}

type mockTicker struct {
	c    chan time.Time
	stop bool
}

func (m *mockTicker) Chan() <-chan time.Time {
	return m.c
}

func (m *mockTicker) Stop() {
	m.stop = true
}

type mockTickerFactory struct {
	lastTicker *mockTicker
}

func (m *mockTickerFactory) NewTicker(interval time.Duration) runtime.Ticker {
	t := &mockTicker{c: make(chan time.Time, 1)}
	m.lastTicker = t
	return t
}

func TestRuntime_Handle_Register(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"enabled: true\nantigravity_model_group: gemini\ninterval: 15m\n"}`)

	respBytes := r.Handle(context.Background(), "plugin.register", req)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result runtime.RegisterResult `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}

	if envelope.Result.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", envelope.Result.SchemaVersion)
	}

	if envelope.Result.Metadata.Name != "Antigravity Priority" {
		t.Errorf("expected metadata name 'Antigravity Priority', got %q", envelope.Result.Metadata.Name)
	}

	if !envelope.Result.Capabilities["management"] {
		t.Errorf("expected management capability to be true")
	}
}

func TestRuntime_Handle_Reconfigure(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"enabled: false\nantigravity_model_group: claude_gpt\n"}`)

	respBytes := r.Handle(context.Background(), "plugin.reconfigure", req)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result runtime.RegisterResult `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}
}

func TestRuntime_Handle_Shutdown(t *testing.T) {
	r := runtime.New(runtime.Options{})

	respBytes := r.Handle(context.Background(), "plugin.shutdown", nil)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result map[string]any         `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}

	afterShutdownResp := r.Handle(context.Background(), "plugin.register", nil)
	var afterEnvelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	_ = json.Unmarshal(afterShutdownResp, &afterEnvelope)
	if afterEnvelope.OK {
		t.Errorf("expected failure after shutdown, got OK=true")
	}
	if afterEnvelope.Error == nil || afterEnvelope.Error.Code != "shutdown" {
		t.Errorf("expected error code 'shutdown', got %+v", afterEnvelope.Error)
	}
}

func TestRuntime_Handle_UnknownMethod(t *testing.T) {
	r := runtime.New(runtime.Options{})
	respBytes := r.Handle(context.Background(), "unknown.method", nil)

	var envelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}
	if envelope.OK {
		t.Errorf("expected OK=false for unknown method")
	}
	if envelope.Error == nil || envelope.Error.Code != "invalid_request" {
		t.Errorf("expected error code 'invalid_request', got %+v", envelope.Error)
	}
}

func TestRuntime_Handle_InvalidConfig(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"interval: -5m"}`)

	respBytes := r.Handle(context.Background(), "plugin.register", req)

	var envelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}
	if envelope.OK {
		t.Errorf("expected OK=false for invalid config")
	}
	if envelope.Error == nil || envelope.Error.Code != "invalid_config" {
		t.Errorf("expected error code 'invalid_config', got %+v", envelope.Error)
	}
}

func TestRuntime_Handle_ManagementRegister(t *testing.T) {
	r := runtime.New(runtime.Options{})
	respBytes := r.Handle(context.Background(), "management.register", nil)

	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Routes []struct {
				Method string `json:"Method"`
				Path   string `json:"Path"`
			} `json:"routes"`
			Resources []struct {
				Path string `json:"Path"`
				Menu string `json:"Menu"`
			} `json:"resources"`
		} `json:"result"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if !envelope.OK {
		t.Fatalf("expected OK=true")
	}
	if len(envelope.Result.Routes) < 3 {
		t.Errorf("expected at least 3 routes, got %d", len(envelope.Result.Routes))
	}
	if len(envelope.Result.Resources) < 1 {
		t.Errorf("expected at least 1 resource, got %d", len(envelope.Result.Resources))
	}
}

func TestRuntime_Handle_ManagementHandle(t *testing.T) {
	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-auth",
			AuthIndex: "auth_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	// 1. POST /v0/management/plugins/antigravity-priority/run with mode=dry-run
	mgmtReq := map[string]any{
		"Method": "POST",
		"Path":   "/v0/management/plugins/antigravity-priority/run",
		"Query":  "mode=dry-run",
	}
	reqBytes, _ := json.Marshal(mgmtReq)
	respBytes := r.Handle(context.Background(), "management.handle", reqBytes)

	var envelope struct {
		OK     bool                       `json:"ok"`
		Result runtime.ManagementResponse `json:"result"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal envelope failed: %v, raw: %s", err, string(respBytes))
	}
	if !envelope.OK {
		t.Fatalf("expected OK=true")
	}
	if envelope.Result.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", envelope.Result.StatusCode)
	}

	// 2. POST /plugins/antigravity-priority/run with mode=apply
	mgmtReqApply := map[string]any{
		"Method": "POST",
		"Path":   "/plugins/antigravity-priority/run",
		"query":  "mode=apply",
	}
	reqApplyBytes, _ := json.Marshal(mgmtReqApply)
	respApplyBytes := r.Handle(context.Background(), "management.handle", reqApplyBytes)

	var envApply struct {
		OK     bool                       `json:"ok"`
		Result runtime.ManagementResponse `json:"result"`
	}
	if err := json.Unmarshal(respApplyBytes, &envApply); err != nil || !envApply.OK || envApply.Result.StatusCode != http.StatusOK {
		t.Errorf("apply request failed: %s", string(respApplyBytes))
	}

	// 3. GET /status under resource route
	mgmtReqStatus := map[string]any{
		"Method": "GET",
		"Path":   "/v0/resource/plugins/antigravity-priority/status",
	}
	reqStatusBytes, _ := json.Marshal(mgmtReqStatus)
	respStatusBytes := r.Handle(context.Background(), "management.handle", reqStatusBytes)

	var envStatus struct {
		OK     bool                       `json:"ok"`
		Result runtime.ManagementResponse `json:"result"`
	}
	if err := json.Unmarshal(respStatusBytes, &envStatus); err != nil || !envStatus.OK || envStatus.Result.StatusCode != http.StatusOK {
		t.Errorf("status request failed: %s", string(respStatusBytes))
	}
}

func TestRuntime_SingleFlight_Conflict(t *testing.T) {
	blockChan := make(chan struct{})
	enteredChan := make(chan struct{})

	r := runtime.New(runtime.Options{
		Runner: func(ctx context.Context, request runtime.TaskRequest) error {
			close(enteredChan)
			<-blockChan
			return nil
		},
	})

	go func() {
		_ = r.DryRun(context.Background(), config.AntigravityModelGroupGemini, nil)
	}()

	<-enteredChan

	if err := r.DryRun(context.Background(), config.AntigravityModelGroupGemini, nil); !errors.Is(err, runtime.ErrRunInProgress) {
		t.Errorf("expected ErrRunInProgress on DryRun, got %v", err)
	}
	if err := r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, nil); !errors.Is(err, runtime.ErrRunInProgress) {
		t.Errorf("expected ErrRunInProgress on ManualApply, got %v", err)
	}
	if err := r.AutoApply(context.Background()); !errors.Is(err, runtime.ErrRunInProgress) {
		t.Errorf("expected ErrRunInProgress on AutoApply, got %v", err)
	}

	close(blockChan)
}

func TestRuntime_AutoApply_Cooldown(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)}
	calls := 0

	r := runtime.New(runtime.Options{
		Clock: clock,
		Runner: func(ctx context.Context, request runtime.TaskRequest) error {
			calls++
			return nil
		},
	})

	// First AutoApply
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("first auto apply failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}

	// Immediate second AutoApply should be throttled by interval cooldown without error
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("second auto apply failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected cooldown to prevent second call, got %d calls", calls)
	}

	// Advance clock past 15m interval
	clock.now = clock.now.Add(16 * time.Minute)
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("third auto apply failed: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls after interval advance, got %d", calls)
	}
}

func TestRuntime_ProductionRunner_ConcurrentProbes(t *testing.T) {
	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-account-1",
			AuthIndex: "auth_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
		{
			Name:      "test-account-2",
			AuthIndex: "auth_2",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  90,
		},
		{
			Name:      "test-account-3",
			AuthIndex: "auth_3",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  80,
		},
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	err := r.DryRun(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	snap, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatalf("latest snapshot failed: %v", err)
	}
	if snap.TotalItems != 3 {
		t.Errorf("expected 3 total items, got %d", snap.TotalItems)
	}
}

func TestRuntime_ProductionRunner_FilteredAuthIndexes(t *testing.T) {
	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-account-1",
			AuthIndex: "auth_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
		{
			Name:      "test-account-2",
			AuthIndex: "auth_2",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  90,
		},
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	// Run with only auth_1
	err := r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, []string{"auth_1"})
	if err != nil {
		t.Fatalf("manual apply failed: %v", err)
	}

	snap, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatalf("latest snapshot failed: %v", err)
	}
	if snap.TotalItems != 1 {
		t.Errorf("expected 1 item when filtered, got %d", snap.TotalItems)
	}
}

func TestRuntime_ProductionRunner_CachedEvidence(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "refresh-cache.json")

	// Pre-populate state cache with fresh entry
	store, _ := state.Load(context.Background(), cachePath)
	_ = store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
		AuthIndex:            "auth_cached",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		ResetAt:              time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		Remaining:            80,
		ShortWindowResetAt:   time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC),
		ShortWindowRemaining: ptrInt64(90),
		LongWindowResetAt:    time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC),
		LongWindowRemaining:  ptrInt64(80),
		NextProbeAt:          time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC),
		Source:               state.SourceFreshProbe,
	})
	_ = store.SaveAtomic(context.Background())

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-account-cached",
			AuthIndex: "auth_cached",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 5, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	// AutoApply checks store.NeedsProbe -> false (fresh cached) -> uses cached evidence without HTTP call
	err := r.AutoApply(context.Background())
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
}

func TestRuntime_ProductionRunner_PhysicalAuthJSONPath(t *testing.T) {
	tempDir := t.TempDir()
	jsonFilePath := filepath.Join(tempDir, "auth.json")
	_ = os.WriteFile(jsonFilePath, []byte(`{"access_token":"from_file_token","project_id":"file_proj"}`), 0o600)

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-file-auth",
			AuthIndex: "auth_file_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
	}
	mock.authDocs["auth_file_1"] = host.AuthDocument{
		AuthIndex: "auth_file_1",
		Path:      jsonFilePath,
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	err := r.DryRun(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("dry run with auth path failed: %v", err)
	}
}

func TestRuntime_ProductionRunner_Apply_Full(t *testing.T) {
	tempDir := t.TempDir()
	authFilePath := filepath.Join(tempDir, "auth_1.json")
	_ = os.WriteFile(authFilePath, []byte(`{"access_token":"token_123","project_id":"proj_123","priority":50}`), 0o600)

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-account-1",
			AuthIndex: "auth_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  50,
		},
	}
	mock.authDocs["auth_1"] = host.AuthDocument{
		AuthIndex: "auth_1",
		Path:      authFilePath,
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	err := r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("manual apply failed: %v", err)
	}

	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	lastResult, ok := diag["last_result"].(apply.Result)
	if !ok {
		t.Fatalf("expected last_result in diagnostics")
	}
	if lastResult.Succeeded == 0 {
		t.Errorf("expected at least 1 succeeded apply change, got %+v", lastResult)
	}

	// Verify file content was patched
	updatedData, err := os.ReadFile(authFilePath)
	if err != nil {
		t.Fatalf("read patched file failed: %v", err)
	}
	var updatedMap map[string]any
	_ = json.Unmarshal(updatedData, &updatedMap)
	if updatedMap["priority"] == float64(50) {
		t.Errorf("expected priority to be changed from 50, got %v", updatedMap["priority"])
	}
}

func TestRuntime_ProductionRunner_ProbeFailure(t *testing.T) {
	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "test-account-fail",
			AuthIndex: "auth_fail_1",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
			Disabled:  false,
		},
	}
	mock.httpResponse = host.HTTPResponse{
		StatusCode: http.StatusUnauthorized,
		Body:       []byte(`{"error": "unauthorized"}`),
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	err := r.DryRun(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	snap, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatalf("latest snapshot failed: %v", err)
	}

	if len(snap.Items) != 1 {
		t.Fatalf("expected 1 item in snapshot, got %d", len(snap.Items))
	}
	if !snap.Items[0].Target.Disabled {
		t.Errorf("expected failing probe to be temporarily disabled")
	}
	if snap.Items[0].Reason != "failedQuotaFetch" {
		t.Errorf("expected reason 'failedQuotaFetch', got %q", snap.Items[0].Reason)
	}
}

func TestRuntime_TickerWorker_StartAndStop(t *testing.T) {
	mockFactory := &mockTickerFactory{}
	r := runtime.New(runtime.Options{
		TickerFactory: mockFactory,
		Clock:         &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper:       testSleeper{},
	})

	// Register with auto_apply = true
	req := []byte(`{"config_yaml":"enabled: true\nauto_apply: true\ninterval: 10m\n"}`)
	respBytes := r.Handle(context.Background(), "plugin.register", req)
	var env struct {
		OK bool `json:"ok"`
	}
	_ = json.Unmarshal(respBytes, &env)
	if !env.OK {
		t.Fatalf("register failed")
	}

	// Reconfigure with auto_apply = false
	reqReconf := []byte(`{"config_yaml":"enabled: true\nauto_apply: false\n"}`)
	r.Handle(context.Background(), "plugin.reconfigure", reqReconf)

	// Shutdown
	_ = r.Shutdown(context.Background())
}

func TestRuntime_Diagnostics_And_Status(t *testing.T) {
	r := runtime.New(runtime.Options{})

	status, err := r.Status(context.Background())
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if status.LatestAudit == "" {
		t.Errorf("expected non-empty latest audit")
	}

	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	mgmtAPI, ok := diag["management_api"].(map[string]any)
	if !ok || mgmtAPI["status"] != "ready" {
		t.Errorf("expected management_api status 'ready', got %+v", diag["management_api"])
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
