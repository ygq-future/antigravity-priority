package runtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	listResponses     [][]host.AuthFile
	authDocs          map[string]host.AuthDocument
	patchedPriorities map[string]int
	patchedDisabled   map[string]bool
	httpResponse      host.HTTPResponse
	httpErr           error
	httpCalls         []host.HTTPRequest
	operations        []string
	afterHTTP         func(*mockHost)
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
							"windows": [{
								"name": "5h",
								"remainingFraction": 0.8,
								"resetTime": "2026-08-18T17:00:00Z"
							}]
						}
					},
					"gemini-2.5-flash": {
						"quotaInfo": {
							"windows": [{
								"name": "7d",
								"remainingFraction": 0.9,
								"resetTime": "2026-08-25T12:00:00Z"
							}]
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
	m.operations = append(m.operations, "host-sync")
	if len(m.listResponses) > 0 {
		files := m.listResponses[0]
		m.listResponses = m.listResponses[1:]
		return append([]host.AuthFile(nil), files...), nil
	}
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
	m.httpCalls = append(m.httpCalls, req)
	m.operations = append(m.operations, "google-probe")
	hook := m.afterHTTP
	m.afterHTTP = nil
	m.mu.Unlock()
	if hook != nil {
		hook(m)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.httpErr != nil {
		return host.HTTPResponse{}, m.httpErr
	}
	return m.httpResponse, nil
}

func (m *mockHost) PatchPriority(ctx context.Context, authIndex string, priority int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchedPriorities[authIndex] = priority
	m.operations = append(m.operations, "host-patch")
	for i := range m.files {
		if m.files[i].AuthIndex == authIndex {
			m.files[i].Priority = priority
			m.files[i].PriorityMissing = false
		}
	}
	return nil
}

func (m *mockHost) PatchDisabled(ctx context.Context, name string, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patchedDisabled[name] = disabled
	for i := range m.files {
		if m.files[i].Name == name || m.files[i].AuthIndex == name {
			m.files[i].Disabled = disabled
		}
	}
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

	if len(envelope.Result.Metadata.ConfigFields) != 0 {
		t.Errorf("expected Metadata.ConfigFields to remain empty, got %+v", envelope.Result.Metadata.ConfigFields)
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
	// Only structurally unparseable input should produce hard errors
	req := []byte(`{"config_yaml":"{invalid-json"}`)

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

func TestRuntime_Handle_Diagnostics(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"enabled: true\n"}`)

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

	// Diagnostics should surface runtime status
	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	mgmt, ok := diag["management_api"].(map[string]any)
	if !ok || mgmt["status"] != "ready" {
		t.Errorf("expected management_api status ready, got %+v", diag["management_api"])
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

	// 1. POST /v0/management/plugins/antigravity-priority/run with mode=probe
	mgmtReq := map[string]any{
		"Method": "POST",
		"Path":   "/v0/management/plugins/antigravity-priority/run",
		"Query":  "mode=probe",
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
		_ = r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil)
	}()

	<-enteredChan

	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); !errors.Is(err, runtime.ErrRunInProgress) {
		t.Errorf("expected ErrRunInProgress on Probe, got %v", err)
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

	err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}

	snap, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatalf("latest snapshot failed: %v", err)
	}
	if len(snap.Groups[snap.ActiveModelGroup].Items) != 3 {
		t.Errorf("expected 3 total items, got %d", len(snap.Groups[snap.ActiveModelGroup].Items))
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
	if len(snap.Groups[snap.ActiveModelGroup].Items) != 1 {
		t.Errorf("expected 1 item when filtered, got %d", len(snap.Groups[snap.ActiveModelGroup].Items))
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
	_, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"})
	if err != nil {
		t.Fatal(err)
	}
	dyn, _ := r.GetDynamicConfig(context.Background())
	dyn.AutoApply = true
	if err := r.SetDynamicConfig(context.Background(), dyn); err != nil {
		t.Fatal(err)
	}

	// AutoApply must probe even though the cached observation is still within TTL.
	err = r.AutoApply(context.Background())
	if err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if len(mock.httpCalls) == 0 {
		t.Fatal("expected AutoApply to force a current-round Google probe")
	}
	if got := strings.Join(mock.operations, ","); !strings.HasPrefix(got, "host-sync,google-probe,host-sync") {
		t.Fatalf("operation order = %s; want host-sync,google-probe,host-sync before planning/apply", got)
	}
}

func TestRuntime_ProbeReconcilesHostChangesBeforePlanning(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "deleted", AuthIndex: "auth-deleted", Provider: "antigravity", Priority: 50}}
	mock.afterHTTP = func(m *mockHost) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.files = []host.AuthFile{{Name: "added", AuthIndex: "auth-added", Provider: "antigravity", Priority: 77}}
	}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}, Sleeper: testSleeper{}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := snapshot.Groups["gemini"].Items
	if len(items) != 1 || items[0].AuthIndex != "auth-added" || items[0].Target.Priority != 77 {
		t.Fatalf("post-probe snapshot items = %#v; want newly added credential unchanged", items)
	}
	if _, patched := mock.patchedPriorities["auth-deleted"]; patched {
		t.Fatal("deleted credential received a write")
	}
}

func TestRuntime_ProbeUsesPostProbePriorityAndDisabledState(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "changed", AuthIndex: "auth-changed", Provider: "antigravity", Priority: 50}}
	mock.afterHTTP = func(m *mockHost) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.files[0].Priority = 77
		m.files[0].Disabled = true
	}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}, Sleeper: testSleeper{}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}

	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}

	snapshot, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := snapshot.Groups["gemini"].Items
	if len(items) != 1 || items[0].Current.Priority != 77 || !items[0].Current.Disabled {
		t.Fatalf("post-probe current state = %#v; want priority=77 disabled=true", items)
	}
	if strings.Contains(strings.Join(mock.operations, ","), "host-patch") {
		t.Fatalf("Probe must not patch Host: %v", mock.operations)
	}
}

func TestRuntime_ProbeStillPerformsSecondHostSyncWhenInitialInventoryIsEmpty(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.listResponses = [][]host.AuthFile{
		{},
		{{Name: "late-addition", AuthIndex: "auth-added", Provider: "antigravity", Priority: 77}},
	}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}, Sleeper: testSleeper{}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}

	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}

	snapshot, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := snapshot.Groups["gemini"].Items
	if len(items) != 1 || items[0].AuthIndex != "auth-added" || items[0].Target.Priority != 77 {
		t.Fatalf("post-probe snapshot items = %#v; want late addition unchanged", items)
	}
	if got := strings.Join(mock.operations, ","); got != "host-sync,host-sync" {
		t.Fatalf("operation order = %s; want two host syncs even with empty initial inventory", got)
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

	err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("probe with auth path failed: %v", err)
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

	cachePath := filepath.Join(tempDir, "state_apply_full.json")
	req := []byte(fmt.Sprintf(`{"config_yaml":"enabled: true\nstate_cache_path: %q\n"}`, cachePath))
	r.Handle(context.Background(), "plugin.register", req)

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
	latestApply, ok := diag["latest_apply"].(*runtime.RunHistoryEntry)
	if !ok || latestApply == nil || latestApply.Succeeded == 0 {
		t.Fatalf("expected latest_apply diagnostics record, got %#v", diag["latest_apply"])
	}
	applyAt := latestApply.At
	if err := r.Probe(context.Background(), config.AntigravityModelGroupClaudeGPT, nil); err != nil {
		t.Fatal(err)
	}
	diagAfterProbe, _ := r.Diagnostics(context.Background())
	latestAfterProbe := diagAfterProbe["latest_apply"].(*runtime.RunHistoryEntry)
	if !latestAfterProbe.At.Equal(applyAt) || latestAfterProbe.Succeeded != latestApply.Succeeded || latestAfterProbe.Message != latestApply.Message {
		t.Fatalf("probe replaced latest Apply health: before=%#v after=%#v", latestApply, latestAfterProbe)
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

func TestRuntime_DiagnosticsPreservesFailedApplyAfterProbe(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "broken", AuthIndex: "auth-broken", Provider: "antigravity", Priority: 50}}
	// Authentication can be probed, but the missing physical path makes the
	// subsequent priority write fail inside the Apply engine.
	mock.authDocs["auth-broken"] = host.AuthDocument{
		AuthIndex: "auth-broken",
		JSON:      json.RawMessage(`{"access_token":"token","project_id":"project"}`),
	}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}, Sleeper: testSleeper{}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	if err := r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	diagnostics, _ := r.Diagnostics(context.Background())
	failedApply := diagnostics["latest_apply"].(*runtime.RunHistoryEntry)
	if failedApply == nil || failedApply.Failed != 1 {
		t.Fatalf("latest_apply = %#v; want one failed write", failedApply)
	}
	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	afterProbe, _ := r.Diagnostics(context.Background())
	latest := afterProbe["latest_apply"].(*runtime.RunHistoryEntry)
	if latest == nil || latest.Failed != failedApply.Failed || !latest.At.Equal(failedApply.At) || latest.Message != failedApply.Message {
		t.Fatalf("Probe replaced failed Apply health: before=%#v after=%#v", failedApply, latest)
	}
}

func TestRuntime_DiagnosticsWithProbeOnlyHistoryHasNoLatestApply(t *testing.T) {
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "probe-only", AuthIndex: "auth-probe", Provider: "antigravity", Priority: 100}}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}, Sleeper: testSleeper{}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	diagnostics, _ := r.Diagnostics(context.Background())
	latest, ok := diagnostics["latest_apply"].(*runtime.RunHistoryEntry)
	if !ok || latest != nil {
		t.Fatalf("probe-only latest_apply = %#v; want typed nil", diagnostics["latest_apply"])
	}
}

func TestRuntime_SyncHostPreservesConfiguredControlGroup(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "control", AuthIndex: "auth-control", Provider: "antigravity", Priority: 100}}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	httpCallsBefore := len(mock.httpCalls)
	snapshot, err := r.SyncHost(context.Background(), config.AntigravityModelGroupClaudeGPT)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ActiveModelGroup != "gemini" {
		t.Fatalf("active_model_group = %q; want configured gemini", snapshot.ActiveModelGroup)
	}
	if len(mock.httpCalls) != httpCallsBefore {
		t.Fatal("overview synchronization unexpectedly called Google")
	}
}

func TestRuntime_ProductionRunner_ZeroChange_Apply_Omitted(t *testing.T) {
	tempDir := t.TempDir()
	authFilePath := filepath.Join(tempDir, "auth_zero_change.json")
	// This low-urgency weekly account remains at the regular starting priority.
	_ = os.WriteFile(authFilePath, []byte(`{"access_token":"token_123","project_id":"proj_123","priority":100}`), 0o600)

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:            "test-account-zero-1",
			AuthIndex:       "auth_zero_1",
			Provider:        string(core.ProviderAntigravity),
			Type:            string(core.CredentialTypeAntigravity),
			Priority:        100,
			PriorityMissing: false,
		},
	}
	mock.authDocs["auth_zero_1"] = host.AuthDocument{
		AuthIndex: "auth_zero_1",
		Path:      authFilePath,
		JSON:      json.RawMessage(`{"access_token":"token_123","project_id":"proj_123","priority":100}`),
	}

	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Sleeper: testSleeper{},
	})

	cachePath := filepath.Join(tempDir, "state_zero.json")
	store, _ := state.Load(context.Background(), cachePath)
	_ = store.MarkProbeSuccess(context.Background(), state.ProbeSuccess{
		AuthIndex:            "auth_zero_1",
		Provider:             core.ProviderAntigravity,
		ModelGroup:           "gemini",
		ObservedAt:           time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC),
		ResetAt:              time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
		Remaining:            80,
		ShortWindowResetAt:   time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC),
		ShortWindowRemaining: ptrInt64(90),
		LongWindowResetAt:    time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC),
		LongWindowRemaining:  ptrInt64(80),
		NextProbeAt:          time.Date(2026, 8, 18, 13, 0, 0, 0, time.UTC),
		Source:               state.SourceFreshProbe,
	})
	_ = store.SaveAtomic(context.Background())

	req := []byte(fmt.Sprintf(`{"config_yaml":"enabled: true\nstate_cache_path: %q\n"}`, cachePath))
	r.Handle(context.Background(), "plugin.register", req)

	initialDiag, _ := r.Diagnostics(context.Background())
	initialHistLen := len(initialDiag["run_history"].([]runtime.RunHistoryEntry))

	// First run: all credentials already in sync (Priority 100 == Target 100)
	err := r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("manual apply failed: %v", err)
	}

	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}

	// Verify audit summary indicates in sync
	latestAudit, _ := diag["latest_audit"].(string)
	if !strings.Contains(latestAudit, "in sync") {
		t.Errorf("expected latest_audit to contain 'in sync', got %q", latestAudit)
	}

	// Verify no host patches were executed
	if len(mock.patchedPriorities) != 0 {
		t.Errorf("expected 0 host patches for zero-change apply, got %+v", mock.patchedPriorities)
	}

	// Verify runHistory does NOT contain useless 0-change apply entries
	runHistory, ok := diag["run_history"].([]runtime.RunHistoryEntry)
	if !ok {
		t.Fatalf("expected run_history in diagnostics")
	}
	if len(runHistory) != initialHistLen {
		t.Errorf("expected run_history length %d for zero-change apply, got %d entries: %+v", initialHistLen, len(runHistory), runHistory)
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

	err := r.Probe(context.Background(), config.AntigravityModelGroupGemini, nil)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}

	snap, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatalf("latest snapshot failed: %v", err)
	}

	activeGroup := snap.Groups[snap.ActiveModelGroup]
	if len(activeGroup.Items) != 1 {
		t.Fatalf("expected 1 item in snapshot, got %d", len(activeGroup.Items))
	}
	if !activeGroup.Items[0].Target.Disabled {
		t.Errorf("expected failing probe to be temporarily disabled")
	}
	if activeGroup.Items[0].Reason != "failedQuotaFetch" {
		t.Errorf("expected reason 'failedQuotaFetch', got %q", activeGroup.Items[0].Reason)
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

func TestRuntime_AutoApply_Paused(t *testing.T) {
	clock := &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)}
	calls := 0
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	r := runtime.New(runtime.Options{
		Clock: clock,
		Runner: func(ctx context.Context, request runtime.TaskRequest) error {
			calls++
			return nil
		},
	})

	// Register with temp cache path to enable SetScheduleConfig
	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pause the scheduler
	if err := r.SetScheduleConfig(context.Background(), state.ScheduleConfig{Paused: true}); err != nil {
		t.Fatal(err)
	}

	// AutoApply should be skipped when paused
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls when paused, got %d", calls)
	}

	// Resume and verify it runs
	if err := r.SetScheduleConfig(context.Background(), state.ScheduleConfig{Paused: false}); err != nil {
		t.Fatal(err)
	}
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("auto apply after resume failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call after resume, got %d", calls)
	}
}

func TestRuntime_AutoApply_OutsideScheduleWindow(t *testing.T) {
	// Clock is at 03:00 UTC, window is 09:00-23:00 — should skip
	clock := &testClock{now: time.Date(2026, 8, 19, 3, 0, 0, 0, time.UTC)}
	calls := 0
	cachePath := filepath.Join(t.TempDir(), "cache.json")

	r := runtime.New(runtime.Options{
		Clock: clock,
		Runner: func(ctx context.Context, request runtime.TaskRequest) error {
			calls++
			return nil
		},
	})

	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set schedule window 09:00-23:00
	if err := r.SetScheduleConfig(context.Background(), state.ScheduleConfig{
		WindowEnabled: true,
		WindowStart:   "09:00",
		WindowEnd:     "23:00",
	}); err != nil {
		t.Fatal(err)
	}

	// AutoApply at 03:00 should be skipped (outside window)
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls outside window, got %d", calls)
	}

	// Move clock into the window (12:00)
	clock.now = time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("auto apply inside window failed: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call inside window, got %d", calls)
	}
}

func TestRuntime_GetSetScheduleConfig(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	r := runtime.New(runtime.Options{})

	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default schedule config should be zero-value
	cfg, err := r.GetScheduleConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Paused || cfg.WindowEnabled {
		t.Errorf("expected zero schedule config, got %+v", cfg)
	}

	// Set and read back
	if err := r.SetScheduleConfig(context.Background(), state.ScheduleConfig{
		Paused:        true,
		WindowEnabled: true,
		WindowStart:   "08:30",
		WindowEnd:     "23:30",
	}); err != nil {
		t.Fatal(err)
	}

	cfg, err = r.GetScheduleConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Paused {
		t.Error("expected Paused=true")
	}
	if !cfg.WindowEnabled {
		t.Error("expected WindowEnabled=true")
	}
	if cfg.WindowStart != "08:30" || cfg.WindowEnd != "23:30" {
		t.Errorf("unexpected window: %q-%q", cfg.WindowStart, cfg.WindowEnd)
	}

	// Verify persisted to disk and survives reload
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	diskCfg := store.GetScheduleConfig()
	if !diskCfg.Paused || diskCfg.WindowStart != "08:30" {
		t.Errorf("expected disk config to match, got %+v", diskCfg)
	}

	// Verify schedule config shows in diagnostics
	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	sched, ok := diag["scheduler"].(map[string]any)
	if !ok {
		t.Fatal("expected scheduler in diagnostics")
	}
	if sched["paused"] != true {
		t.Errorf("expected paused=true in diagnostics, got %v", sched["paused"])
	}
	if sched["window_start"] != "08:30" {
		t.Errorf("expected window_start=08:30 in diagnostics, got %v", sched["window_start"])
	}
}

func TestRuntime_GetSetDynamicConfig(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	r := runtime.New(runtime.Options{})

	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Read default dynamic config
	dynCfg, err := r.GetDynamicConfig(context.Background())
	if err != nil {
		t.Fatalf("get dynamic config failed: %v", err)
	}
	if dynCfg.Interval != "15m0s" {
		t.Errorf("expected default Interval=15m0s, got %q", dynCfg.Interval)
	}
	if dynCfg.AntigravityModelGroup != "gemini" {
		t.Errorf("expected default model group=gemini, got %q", dynCfg.AntigravityModelGroup)
	}
	if dynCfg.QuotaSampleCapacity != 6 {
		t.Errorf("expected default QuotaSampleCapacity=6, got %d", dynCfg.QuotaSampleCapacity)
	}

	// 2. Set updated dynamic config
	update := state.DynamicConfig{
		AutoApply:                true,
		Interval:                 "30m",
		AntigravityModelGroup:    "claude_gpt",
		MaxConcurrency:           8,
		MinChange:                3,
		UrgencyTolerance:         0.08,
		RateLimitCooldownMinutes: 5,
		QuotaSampleCapacity:      10,
		PriorityRules: state.PriorityRulesConfig{
			BoostStartPriority:  990,
			NormalStartPriority: 120,
		},
		Schedule: state.ScheduleConfig{
			Paused:        false,
			WindowEnabled: true,
			WindowStart:   "08:00",
			WindowEnd:     "22:00",
		},
	}
	if err := r.SetDynamicConfig(context.Background(), update); err != nil {
		t.Fatalf("set dynamic config failed: %v", err)
	}

	// 3. Verify in-memory config updated
	runtimeCfg, err := r.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeCfg.AutoApply {
		t.Error("expected AutoApply=true")
	}
	if runtimeCfg.Interval != 30*time.Minute {
		t.Errorf("expected Interval=30m, got %v", runtimeCfg.Interval)
	}
	if runtimeCfg.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
		t.Errorf("expected claude_gpt, got %v", runtimeCfg.AntigravityModelGroup)
	}
	if runtimeCfg.MaxConcurrency != 8 {
		t.Errorf("expected MaxConcurrency=8, got %d", runtimeCfg.MaxConcurrency)
	}
	if runtimeCfg.QuotaSampleCapacity != 10 {
		t.Errorf("expected QuotaSampleCapacity=10, got %d", runtimeCfg.QuotaSampleCapacity)
	}

	// 4. Verify disk persistence and survival across reloads
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	diskDyn, ok := store.GetDynamicConfig()
	if !ok {
		t.Fatal("expected dynamic config on disk")
	}
	if diskDyn.Interval != "30m" || diskDyn.AntigravityModelGroup != "claude_gpt" || diskDyn.QuotaSampleCapacity != 10 {
		t.Errorf("unexpected disk dynamic config: %+v", diskDyn)
	}

	// 5. Test validation failures
	badInterval := update
	badInterval.Interval = "invalid"
	if err := r.SetDynamicConfig(context.Background(), badInterval); err == nil {
		t.Error("expected error for invalid interval")
	}

	badGroup := update
	badGroup.AntigravityModelGroup = "unknown_group"
	if err := r.SetDynamicConfig(context.Background(), badGroup); err == nil {
		t.Error("expected error for invalid model group")
	}

	invalidCases := []struct {
		name   string
		mutate func(*state.DynamicConfig)
	}{
		{"max_concurrency=33", func(cfg *state.DynamicConfig) { cfg.MaxConcurrency = 33 }},
		{"min_change=101", func(cfg *state.DynamicConfig) { cfg.MinChange = 101 }},
		{"urgency_tolerance=0.51", func(cfg *state.DynamicConfig) { cfg.UrgencyTolerance = 0.51 }},
		{"cooldown=1441", func(cfg *state.DynamicConfig) { cfg.RateLimitCooldownMinutes = 1441 }},
		{"inverted priorities", func(cfg *state.DynamicConfig) {
			cfg.PriorityRules.BoostStartPriority, cfg.PriorityRules.NormalStartPriority = 100, 101
		}},
	}
	for _, testCase := range invalidCases {
		t.Run(testCase.name, func(t *testing.T) {
			candidate := update
			testCase.mutate(&candidate)
			if err := r.SetDynamicConfig(context.Background(), candidate); err == nil {
				t.Fatal("expected backend validation error")
			}
		})
	}

	zeroTolerance := update
	zeroTolerance.UrgencyTolerance = 0
	if err := r.SetDynamicConfig(context.Background(), zeroTolerance); err != nil {
		t.Fatalf("zero urgency tolerance rejected: %v", err)
	}
	gotZero, _ := r.GetDynamicConfig(context.Background())
	if gotZero.UrgencyTolerance != 0 {
		t.Fatalf("zero urgency tolerance was normalized to %v", gotZero.UrgencyTolerance)
	}
}

func TestRuntime_DynamicConfig_SurvivesReconfigure(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	r := runtime.New(runtime.Options{})

	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Set dynamic config from UI
	if err := r.SetDynamicConfig(context.Background(), state.DynamicConfig{
		AutoApply:                true,
		Interval:                 "45m",
		AntigravityModelGroup:    "claude_gpt",
		MaxConcurrency:           10,
		MinChange:                5,
		UrgencyTolerance:         0.05,
		RateLimitCooldownMinutes: 5,
		QuotaSampleCapacity:      6,
		PriorityRules: state.PriorityRulesConfig{
			BoostStartPriority:  980,
			NormalStartPriority: 200,
		},
		Schedule: state.ScheduleConfig{
			Paused: false,
		},
	}); err != nil {
		t.Fatal(err)
	}

	// CPA calls reconfigure with minimal YAML (e.g. enabled: true)
	_, err = r.Reconfigure(context.Background(), runtime.ReconfigureRequest{
		ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify runtime config preserved dynamic settings from disk
	cfg, err := r.Config()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoApply {
		t.Error("expected AutoApply=true to be preserved after Reconfigure")
	}
	if cfg.Interval != 45*time.Minute {
		t.Errorf("expected Interval=45m, got %v", cfg.Interval)
	}
	if cfg.AntigravityModelGroup != config.AntigravityModelGroupClaudeGPT {
		t.Errorf("expected claude_gpt, got %v", cfg.AntigravityModelGroup)
	}
	if cfg.MaxConcurrency != 10 {
		t.Errorf("expected MaxConcurrency=10, got %d", cfg.MaxConcurrency)
	}
}

func TestRuntime_AutoApply_PluginDisabled(t *testing.T) {
	calls := 0
	r := runtime.New(runtime.Options{
		Clock: &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)},
		Runner: func(ctx context.Context, request runtime.TaskRequest) error {
			calls++
			return nil
		},
	})

	// Register with enabled: false
	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "enabled: false\nauto_apply: true\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// AutoApply must be skipped when plugin is disabled
	if err := r.AutoApply(context.Background()); err != nil {
		t.Fatalf("auto apply failed: %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls when plugin is disabled, got %d", calls)
	}
}

func TestRuntime_ProductionRunner_RespectsManuallyDisabledAccounts(t *testing.T) {
	tempDir := t.TempDir()
	authFilePath := filepath.Join(tempDir, "auth_disabled.json")
	_ = os.WriteFile(authFilePath, []byte(`{"access_token":"token_123","project_id":"proj_123","priority":50,"disabled":true}`), 0o600)

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "manually-disabled-account",
			AuthIndex: "auth_dis",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  50,
			Disabled:  true,
		},
	}
	mock.authDocs["auth_dis"] = host.AuthDocument{
		AuthIndex: "auth_dis",
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

	// Verify PatchDisabled was NOT called with disabled=false
	if val, patched := mock.patchedDisabled["manually-disabled-account"]; patched && !val {
		t.Errorf("expected manually disabled account to NOT be re-enabled, but got PatchDisabled(false)")
	}

	// Verify file remains disabled
	updatedData, err := os.ReadFile(authFilePath)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	var updatedMap map[string]any
	_ = json.Unmarshal(updatedData, &updatedMap)
	if updatedMap["disabled"] != true {
		t.Errorf("expected disabled to remain true in physical file, got %v", updatedMap["disabled"])
	}
}

func TestRuntime_FilterEvent_429Cooldown(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	authFilePath := filepath.Join(t.TempDir(), "auth_429.json")
	_ = os.WriteFile(authFilePath, []byte(`{"access_token":"token_123","project_id":"proj_123","priority":100}`), 0o600)

	mock := newMockHost()
	mock.files = []host.AuthFile{
		{
			Name:      "rate-limited-account",
			AuthIndex: "auth_429",
			Provider:  string(core.ProviderAntigravity),
			Type:      string(core.CredentialTypeAntigravity),
			Priority:  100,
		},
	}
	mock.authDocs["auth_429"] = host.AuthDocument{
		AuthIndex: "auth_429",
		Path:      authFilePath,
	}

	clock := &testClock{now: time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)}
	r := runtime.New(runtime.Options{
		Host:    mock,
		Clock:   clock,
		Sleeper: testSleeper{},
	})

	_, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Simulate CPA calling filter.response with 429
	eventPayload := []byte(`{"auth_index":"auth_429","status_code":429,"error":"RESOURCE_EXHAUSTED: quota rate limit exceeded"}`)
	resBytes := r.Handle(context.Background(), "filter.response", eventPayload)

	var env struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resBytes, &env); err != nil || !env.OK {
		t.Fatalf("expected filter event to return OK, got %s", string(resBytes))
	}

	// 2. Verify priority was immediately patched to -1
	updatedData, err := os.ReadFile(authFilePath)
	if err != nil {
		t.Fatal(err)
	}
	var updatedMap map[string]any
	_ = json.Unmarshal(updatedData, &updatedMap)
	if updatedMap["priority"] != float64(-1) {
		t.Errorf("expected priority to be patched to -1 on 429, got %v", updatedMap["priority"])
	}

	// 3. Verify cooldown persisted in store
	store, err := state.Load(context.Background(), cachePath)
	if err != nil {
		t.Fatal(err)
	}
	cooldowns := store.GetActiveCooldowns(clock.now)
	until, inCooldown := cooldowns["auth_429"]
	if !inCooldown {
		t.Fatal("expected auth_429 to be in active cooldown")
	}
	expectedUntil := clock.now.Add(5 * time.Minute)
	if !until.Equal(expectedUntil) {
		t.Errorf("cooldown until = %v; want %v", until, expectedUntil)
	}

	// 4. Verify cooldown shows up in Diagnostics
	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	activeCD, ok := diag["active_cooldowns"].([]map[string]any)
	if !ok || len(activeCD) != 1 {
		t.Fatalf("expected 1 active cooldown in diagnostics, got %+v", diag["active_cooldowns"])
	}
	if activeCD[0]["auth_index"] != "auth_429" {
		t.Errorf("expected auth_index=auth_429, got %v", activeCD[0]["auth_index"])
	}
	if _, hasWarnings := diag["config_warnings"]; hasWarnings {
		t.Errorf("expected config_warnings to be purged from diagnostics")
	}
	history := diag["run_history"].([]runtime.RunHistoryEntry)
	if len(history) == 0 || history[0].Kind != runtime.KindCooldown || history[0].Succeeded != 1 {
		t.Fatalf("expected successful cooldown audit record, got %#v", history)
	}
}

func TestRuntime_FilterEvent_429FailureIsReportedAndAudited(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "broken", AuthIndex: "auth-broken", Provider: "antigravity", Priority: 100}}
	mock.authDocs["auth-broken"] = host.AuthDocument{AuthIndex: "auth-broken", Path: filepath.Join(t.TempDir(), "missing", "auth.json")}
	r := runtime.New(runtime.Options{Host: mock, Clock: &testClock{now: time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	var envelope runtime.Envelope
	response := r.Handle(context.Background(), runtime.MethodFilterResponse, []byte(`{"auth_index":"auth-broken","status_code":429}`))
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || envelope.Error == nil {
		t.Fatalf("expected qualified failure envelope, got %s", response)
	}
	diagnostics, _ := r.Diagnostics(context.Background())
	history := diagnostics["run_history"].([]runtime.RunHistoryEntry)
	if len(history) == 0 || history[0].Kind != runtime.KindCooldown || history[0].Failed != 1 {
		t.Fatalf("expected failed cooldown audit record, got %#v", history)
	}
}

func TestRuntime_FilterEvent_429WaitsForSingleFlightBoundary(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	authPath := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(authPath, []byte(`{"priority":100,"disabled":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mock := newMockHost()
	mock.files = []host.AuthFile{{Name: "serial", AuthIndex: "auth-serial", Provider: "antigravity", Priority: 100}}
	mock.authDocs["auth-serial"] = host.AuthDocument{AuthIndex: "auth-serial", Path: authPath}
	started := make(chan struct{})
	release := make(chan struct{})
	r := runtime.New(runtime.Options{Host: mock, Runner: func(context.Context, runtime.TaskRequest) error {
		close(started)
		<-release
		if err := os.WriteFile(authPath, []byte(`{"priority":50,"disabled":false}`), 0o600); err != nil {
			return err
		}
		return nil
	}})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "state_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	applyDone := make(chan error, 1)
	go func() { applyDone <- r.ManualApply(context.Background(), config.AntigravityModelGroupGemini, nil) }()
	<-started
	filterDone := make(chan []byte, 1)
	go func() {
		filterDone <- r.Handle(context.Background(), runtime.MethodFilterResponse, []byte(`{"auth_index":"auth-serial","status_code":429}`))
	}()
	select {
	case <-filterDone:
		t.Fatal("429 mutation escaped the Runtime single-flight boundary")
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
	response := <-filterDone
	var envelope runtime.Envelope
	_ = json.Unmarshal(response, &envelope)
	if !envelope.OK {
		t.Fatalf("cooldown failed after serialized apply: %s", response)
	}
	finalDocument, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	var finalState struct {
		Priority int  `json:"priority"`
		Disabled bool `json:"disabled"`
	}
	if err := json.Unmarshal(finalDocument, &finalState); err != nil {
		t.Fatal(err)
	}
	if finalState.Priority != -1 || finalState.Disabled {
		t.Fatalf("final Host state is not deterministic cooldown state: %s", finalDocument)
	}
}
