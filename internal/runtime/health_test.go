package runtime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

func writeHealthAuthFile(t *testing.T, priority int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	body := fmt.Sprintf(`{"access_token":"token","project_id":"project","priority":%d,"disabled":false}`, priority)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func newHealthRuntime(t *testing.T, now time.Time, authIndexes ...string) (*runtime.Runtime, *testClock, *mockHost, string, map[string]string) {
	t.Helper()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	clock := &testClock{now: now}
	mock := newMockHost()
	paths := make(map[string]string, len(authIndexes))
	for _, authIndex := range authIndexes {
		authPath := writeHealthAuthFile(t, 100)
		paths[authIndex] = authPath
		mock.files = append(mock.files, host.AuthFile{
			Name:      "account-" + strings.TrimPrefix(authIndex, "idx-"),
			AuthIndex: authIndex,
			Provider:  "antigravity",
			Priority:  100,
		})
		mock.authDocs[authIndex] = host.AuthDocument{AuthIndex: authIndex, Path: authPath}
	}
	r := newTestRuntime(t, runtime.Options{Host: mock, Clock: clock, StateCachePath: cachePath})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	return r, clock, mock, cachePath, paths
}

func usageFailurePayload(authIndex, model string, status int) []byte {
	return fmt.Appendf(nil, `{"Provider":"antigravity","Model":%q,"AuthIndex":%q,"Failed":true,"Failure":{"StatusCode":%d}}`, model, authIndex, status)
}

func usageSuccessPayload(authIndex, model string) []byte {
	return fmt.Appendf(nil, `{"Provider":"antigravity","Model":%q,"AuthIndex":%q,"Failed":false}`, model, authIndex)
}

func mustOK(t *testing.T, response []byte) {
	t.Helper()
	var envelope runtime.Envelope
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, response)
	}
	if !envelope.OK {
		t.Fatalf("expected ok envelope, got %s", response)
	}
}

func readPriority(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Priority int `json:"priority"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document.Priority
}

func TestHealth_429CooldownsArePoolScoped(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, paths := newHealthRuntime(t, now, "idx-a")

	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusTooManyRequests)))
	}
	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "claude-sonnet-5-5-high", http.StatusTooManyRequests)))
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	entries := store.GetCooldowns()
	geminiKey := state.CooldownKey("idx-a", "gemini")
	claudeKey := state.CooldownKey("idx-a", "claude_gpt")
	if _, ok := entries[geminiKey]; !ok {
		t.Fatalf("gemini cooldown missing: %#v", entries)
	}
	if _, ok := entries[claudeKey]; !ok {
		t.Fatalf("claude cooldown was suppressed by gemini cooldown: %#v", entries)
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
}

func TestHealth_401DemotesAccountWithoutProbe(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, mock, cachePath, paths := newHealthRuntime(t, now, "idx-a", "idx-b")
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatal(err)
	}
	seedHealthEvidence(t, ctx, cachePath, now, "idx-a", "idx-b")
	if err := r.SetCustomTier(ctx, "idx-a", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-b", 2); err != nil {
		t.Fatal(err)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusUnauthorized)))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := store.GetCooldowns()["idx-a"]
	if !ok {
		t.Fatalf("account-level 401 entry missing: %#v", store.GetCooldowns())
	}
	if entry.FailureKind != state.FailureUnauthorized || entry.ProbeEnabled {
		t.Fatalf("401 entry = %#v; want unauthorized without probe", entry)
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	if len(mock.httpCalls) != 0 {
		t.Fatalf("401 must not trigger an active refresh/probe, got %d HTTP calls", len(mock.httpCalls))
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-b" {
		t.Fatalf("401 failed over to %q; want account-b", got)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageSuccessPayload("idx-a", "gemini-3.8-flash-high")))
	store, err = state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := store.GetCooldowns()["idx-a"]; exists {
		t.Fatalf("successful request did not clear 401 quarantine: %#v", store.GetCooldowns())
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-a" {
		t.Fatalf("recovered account picked %q; want account-a", got)
	}
}

func TestHealth_403DemotesAccountWithoutProbe(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, mock, cachePath, paths := newHealthRuntime(t, now, "idx-a", "idx-b")
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatal(err)
	}
	seedHealthEvidence(t, ctx, cachePath, now, "idx-a", "idx-b")
	if err := r.SetCustomTier(ctx, "idx-a", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-b", 2); err != nil {
		t.Fatal(err)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusForbidden)))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := store.GetCooldowns()["idx-a"]
	if !ok {
		t.Fatalf("account-level 403 entry missing: %#v", store.GetCooldowns())
	}
	if entry.FailureKind != state.FailureForbidden || entry.ProbeEnabled {
		t.Fatalf("403 entry = %#v; want forbidden without probe", entry)
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	if len(mock.httpCalls) != 0 {
		t.Fatalf("403 must not trigger an active refresh/probe, got %d HTTP calls", len(mock.httpCalls))
	}

	// 403 demotes the entire account across all models in pool
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-b" {
		t.Fatalf("failed model picked %q; want account-b", got)
	}
	if got := pickModel(t, r, "gemini-3.5-flash-lite"); got != "account-b" {
		t.Fatalf("sibling model picked %q; want account-b because account is demoted", got)
	}

	// Recovery on subsequent successful request
	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageSuccessPayload("idx-a", "gemini-3.8-flash-high")))
	store, err = state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := store.GetCooldowns()["idx-a"]; exists {
		t.Fatalf("successful request did not clear 403 quarantine: %#v", store.GetCooldowns())
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-a" {
		t.Fatalf("recovered account picked %q; want account-a", got)
	}
}

func TestHealth_Probe403EscalatesToAccountQuarantineWithoutRetry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, clock, mock, cachePath, paths := newHealthRuntime(t, now, "idx-a")
	mock.httpResponse = host.HTTPResponse{StatusCode: http.StatusForbidden, Body: []byte(`{"error":{"code":403,"message":"permission denied"}}`)}

	// Trigger 5xx to start probe backoff
	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusServiceUnavailable)))
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	first := store.GetCooldowns()[state.CooldownKey("idx-a", "gemini")]
	if first.CooldownUntil.IsZero() {
		t.Fatalf("missing initial cooldown: %#v", first)
	}

	// Advance clock to trigger scheduled probe and signal worker
	clock.Set(first.CooldownUntil)
	if _, err := r.Reconfigure(ctx, runtime.ReconfigureRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}

	// When probe receives 403, it must escalate to account-level quarantine with ProbeEnabled: false
	waitForHealthEntry(t, cachePath, "idx-a", func(e state.CooldownEntry) bool {
		return e.FailureKind == state.FailureForbidden && !e.ProbeEnabled && strings.TrimSpace(e.ModelGroup) == ""
	})
}

func TestHealth_5xxDemotesPoolAndFailsOver(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, paths := newHealthRuntime(t, now, "idx-a", "idx-b")
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatal(err)
	}
	seedHealthEvidence(t, ctx, cachePath, now, "idx-a", "idx-b")
	if err := r.SetCustomTier(ctx, "idx-a", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-b", 2); err != nil {
		t.Fatal(err)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusServiceUnavailable)))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := store.GetCooldowns()[state.CooldownKey("idx-a", "gemini")]
	if !ok {
		t.Fatalf("pool-level 5xx entry missing: %#v", store.GetCooldowns())
	}
	if entry.FailureKind != state.FailureServerError || !entry.ProbeEnabled {
		t.Fatalf("5xx entry = %#v; want server_error with probe", entry)
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-b" {
		t.Fatalf("5xx failover picked %q; want account-b", got)
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
}

func TestHealth_499IsIgnored(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, paths := newHealthRuntime(t, now, "idx-a")

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", 499)))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.GetCooldowns()) != 0 {
		t.Fatalf("499 must not create health state: %#v", store.GetCooldowns())
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want unchanged 100", got)
	}
}

func TestHealth_NestedCooldownsRestorePerPoolIndependently(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, paths := newHealthRuntime(t, now, "idx-a", "idx-b")
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatal(err)
	}
	seedHealthEvidence(t, ctx, cachePath, now, "idx-a", "idx-b")
	if err := r.SetCustomTier(ctx, "idx-a", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCustomTier(ctx, "idx-b", 2); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusTooManyRequests)))
	}
	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "claude-sonnet-5-5-high", http.StatusTooManyRequests)))
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-b" {
		t.Fatalf("cooling gemini picked %q; want account-b", got)
	}
	if got := pickModel(t, r, "claude-sonnet-5-5-high"); got != "account-b" {
		t.Fatalf("cooling claude picked %q; want account-b", got)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageSuccessPayload("idx-a", "gemini-3.8-flash-high")))
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.GetCooldowns()[state.CooldownKey("idx-a", "gemini")]; ok {
		t.Fatalf("gemini cooldown was not cleared: %#v", store.GetCooldowns())
	}
	if _, ok := store.GetCooldowns()[state.CooldownKey("idx-a", "claude_gpt")]; !ok {
		t.Fatalf("claude cooldown was cleared by gemini success: %#v", store.GetCooldowns())
	}
	if got := pickModel(t, r, "gemini-3.8-flash-high"); got != "account-a" {
		t.Fatalf("recovered gemini picked %q; want account-a", got)
	}
	if got := pickModel(t, r, "claude-sonnet-5-5-high"); got != "account-b" {
		t.Fatalf("still-cooling claude picked %q; want account-b", got)
	}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageSuccessPayload("idx-a", "claude-sonnet-5-5-high")))
	store, err = state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.GetCooldowns()) != 0 {
		t.Fatalf("health entries remain after full recovery: %#v", store.GetCooldowns())
	}
	if got := pickModel(t, r, "claude-sonnet-5-5-high"); got != "account-a" {
		t.Fatalf("recovered claude picked %q; want account-a", got)
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d after full recovery; want untouched 100", got)
	}
}

func TestHealth_ProbeBackoffAndRecovery(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r, clock, mock, cachePath, paths := newHealthRuntime(t, now, "idx-a")
	mock.httpResponse = host.HTTPResponse{StatusCode: http.StatusServiceUnavailable, Body: []byte(`{"error":{"code":503}}`)}

	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, usageFailurePayload("idx-a", "gemini-3.8-flash-high", http.StatusServiceUnavailable)))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	first := store.GetCooldowns()[state.CooldownKey("idx-a", "gemini")]
	if first.CooldownUntil.IsZero() {
		t.Fatalf("missing initial cooldown: %#v", first)
	}
	clock.Set(first.CooldownUntil)
	if _, err := r.Reconfigure(ctx, runtime.ReconfigureRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	waitForHealthEntry(t, cachePath, "idx-a", func(entry state.CooldownEntry) bool {
		return entry.BackoffStep > 0 && entry.CooldownUntil.After(first.CooldownUntil)
	})

	store, err = state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	second := store.GetCooldowns()[state.CooldownKey("idx-a", "gemini")]
	mock.httpResponse = host.HTTPResponse{StatusCode: http.StatusOK, Body: []byte(`{}`)}
	clock.Set(second.CooldownUntil)
	if _, err := r.Reconfigure(ctx, runtime.ReconfigureRequest{ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n"}); err != nil {
		t.Fatal(err)
	}
	waitForHealthClear(t, cachePath, "idx-a")
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
	diag, err := r.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	history, _ := diag["run_history"].([]runtime.RunHistoryEntry)
	if len(history) == 0 || history[0].Kind != runtime.KindCooldownRecovery {
		t.Fatalf("expected cooldown recovery audit after probe success, got %#v", history)
	}
}

func seedHealthEvidence(t *testing.T, ctx context.Context, cachePath string, now time.Time, authIndexes ...string) {
	t.Helper()
	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	remaining := int64(80)
	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(72 * time.Hour)
	for _, authIndex := range authIndexes {
		for _, modelGroup := range []string{"gemini", "claude_gpt"} {
			if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
				AuthIndex:            authIndex,
				Provider:             core.ProviderAntigravity,
				ModelGroup:           modelGroup,
				ObservedAt:           now,
				ResetAt:              reset7d,
				Remaining:            remaining,
				ShortWindowRemaining: &remaining,
				ShortWindowResetAt:   reset5h,
				LongWindowRemaining:  &remaining,
				LongWindowResetAt:    reset7d,
				PlanType:             core.PlanTypePro,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := store.SaveAtomic(ctx); err != nil {
		t.Fatal(err)
	}
}

func pickModel(t *testing.T, r *runtime.Runtime, model string) string {
	t.Helper()
	request, err := json.Marshal(runtime.SchedulerPickRequest{
		Model: model,
		Candidates: []runtime.SchedulerAuthCandidate{
			{ID: "account-a", Provider: "antigravity"},
			{ID: "account-b", Provider: "antigravity"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	response := r.Handle(context.Background(), runtime.MethodSchedulerPick, request)
	var envelope struct {
		OK     bool                          `json:"ok"`
		Result runtime.SchedulerPickResponse `json:"result"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		t.Fatalf("decode scheduler pick: %v (%s)", err, response)
	}
	if !envelope.OK || !envelope.Result.Handled {
		t.Fatalf("scheduler pick not handled: %s", response)
	}
	return envelope.Result.AuthID
}

func waitForHealthEntry(t *testing.T, cachePath, authIndex string, predicate func(state.CooldownEntry) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		store, err := state.Load(context.Background(), cachePath)
		if err == nil {
			for _, entry := range store.GetCooldowns() {
				if entry.AuthIndex == authIndex && predicate(entry) {
					return
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for health entry %s; state=%#v", authIndex, store)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForHealthClear(t *testing.T, cachePath, authIndex string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		store, err := state.Load(context.Background(), cachePath)
		if err == nil && len(store.GetCooldowns()) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for health entry %s to clear; state=%#v", authIndex, store)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type hostWithCooldownReset struct {
	*mockHost
	mu         sync.Mutex
	resetCalls []string
}

func (h *hostWithCooldownReset) ResetRoutingCooldown(_ context.Context, authIndex string) error {
	h.mu.Lock()
	h.resetCalls = append(h.resetCalls, authIndex)
	h.mu.Unlock()
	return nil
}

func TestRuntime_ClearCooldown_SingleAndAll429_CallsHostResetCooldown(t *testing.T) {
	ctx := context.Background()
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	clock := &testClock{now: now}
	seedHealthEvidence(t, ctx, cachePath, now, "account-a", "account-b")

	baseHost := newMockHost()
	baseHost.files = []host.AuthFile{
		{Name: "account-a", AuthIndex: "account-a", Provider: "antigravity", Status: "active"},
		{Name: "account-b", AuthIndex: "account-b", Provider: "antigravity", Status: "active"},
	}
	hostStub := &hostWithCooldownReset{mockHost: baseHost}

	r := runtime.New(runtime.Options{Host: hostStub, Clock: clock, StateCachePath: cachePath, DisableBackgroundWorkers: true})
	defer func() { _ = r.Shutdown(context.Background()) }()
	regRes := r.Handle(ctx, runtime.MethodPluginRegister, marshalHealthTestJSON(t, runtime.RegisterRequest{
		ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\nrate_limit_cooldown_minutes: 5\n",
	}))
	var regEnv struct {
		OK     bool                   `json:"ok"`
		Result runtime.RegisterResult `json:"result"`
	}
	if err := json.Unmarshal(regRes, &regEnv); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	if regEnv.Result.SchemaVersion < 6 {
		t.Fatalf("expected SchemaVersion >= 6 for raw management JSON, got %d", regEnv.Result.SchemaVersion)
	}
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatalf("SyncHost: %v", err)
	}

	// Trigger two 429 failures for account-a in gemini pool to put it into 429 cooldown.
	for range 2 {
		payload := marshalHealthTestJSON(t, map[string]any{
			"Model":     "gemini-3.8-flash-high",
			"AuthIndex": "account-a",
			"Provider":  "antigravity",
			"Failed":    true,
			"Failure": map[string]any{
				"StatusCode": 429,
				"Body":       `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`,
			},
		})
		_ = r.Handle(ctx, runtime.MethodUsageHandle, payload)
	}

	// Diagnostics must expose cooldown_id while keeping auth_index redacted.
	diag, err := r.Diagnostics(ctx)
	if err != nil {
		t.Fatalf("Diagnostics: %v", err)
	}
	activeCooldowns, _ := diag["active_cooldowns"].([]map[string]any)
	if len(activeCooldowns) != 1 {
		t.Fatalf("expected 1 active cooldown, got %d", len(activeCooldowns))
	}
	cooldownID, _ := activeCooldowns[0]["cooldown_id"].(string)
	if cooldownID == "" {
		t.Fatalf("expected non-empty cooldown_id in diagnostics entry: %#v", activeCooldowns[0])
	}
	if gotIdx, _ := activeCooldowns[0]["auth_index"].(string); gotIdx == "account-a" {
		t.Fatalf("expected redacted auth_index in diagnostics, got raw %q", gotIdx)
	}

	// Clear by cooldown_id.
	res, err := r.ClearCooldown(ctx, management.ClearCooldownRequest{CooldownID: cooldownID})
	if err != nil {
		t.Fatalf("ClearCooldown by cooldown_id: %v", err)
	}
	if res.ClearedCount != 1 || res.HostResetCount != 1 {
		t.Fatalf("expected ClearedCount=1 HostResetCount=1, got %+v", res)
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	if len(store.GetCooldowns()) != 0 {
		t.Fatalf("expected empty cooldowns after clear, got %#v", store.GetCooldowns())
	}

	// Trigger 429 cooldowns on both accounts and also a 5xx cooldown, then test ClearCooldown(all=true, kind="rate_limit").
	for _, idx := range []string{"account-a", "account-b"} {
		for range 2 {
			_ = r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
				"Model":     "gemini-3.8-flash-high",
				"AuthIndex": idx,
				"Provider":  "antigravity",
				"Failed":    true,
				"Failure":   map[string]any{"StatusCode": 429},
			}))
		}
	}
	_ = r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
		"Model":     "claude-sonnet-5-5-high",
		"AuthIndex": "account-b",
		"Provider":  "antigravity",
		"Failed":    true,
		"Failure":   map[string]any{"StatusCode": 503},
	}))

	allRes, err := r.ClearCooldown(ctx, management.ClearCooldownRequest{All: true, Kind: "rate_limit"})
	if err != nil {
		t.Fatalf("ClearCooldown all 429: %v", err)
	}
	if allRes.ClearedCount != 2 || allRes.HostResetCount != 2 {
		t.Fatalf("expected 2 rate_limit cooldowns cleared and 2 host resets, got %+v", allRes)
	}

	storeAfter, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatalf("reload store after all 429 clear: %v", err)
	}
	if remaining := storeAfter.GetCooldowns(); len(remaining) != 1 {
		t.Fatalf("expected only non-429 (5xx) cooldown to remain, got %#v", remaining)
	}

	hostStub.mu.Lock()
	totalHostResets := len(hostStub.resetCalls)
	hostStub.mu.Unlock()
	if totalHostResets != 3 {
		t.Fatalf("expected 3 host.routing.reset_cooldown calls total, got %d", totalHostResets)
	}
}

func TestHealth_GenericResourceExhausted429IsRequestScopedExempt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, paths := newHealthRuntime(t, now, "idx-a", "idx-b", "idx-c")
	if _, err := r.SyncHost(ctx); err != nil {
		t.Fatal(err)
	}

	genericBodies := []string{
		`{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`,
		`[{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED","details":[]}}]`,
		`{"error":{"code":429,"message":"Resource has been exhausted","status":"RESOURCE_EXHAUSTED"}}`,
	}

	for i, authIdx := range []string{"idx-a", "idx-b", "idx-c"} {
		for range 3 {
			mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
				"Provider":  "antigravity",
				"Model":     "claude-opus-4-6-thinking",
				"AuthIndex": authIdx,
				"Failed":    true,
				"Failure": map[string]any{
					"StatusCode": http.StatusTooManyRequests,
					"Body":       genericBodies[i%len(genericBodies)],
				},
			})))
		}
	}
	// Even without explicit StatusCode (StatusCode=0), generic JSON 429 body is still request-scoped exempt.
	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
		"Provider":  "antigravity",
		"Model":     "gemini-3.8-flash-high",
		"AuthIndex": "idx-a",
		"Failed":    true,
		"Failure": map[string]any{
			"StatusCode": 0,
			"Body":       genericBodies[0],
		},
	})))
	// Filter error event with generic 429 body must also remain exempt.
	mustOK(t, r.Handle(ctx, runtime.MethodFilterError, marshalHealthTestJSON(t, map[string]any{
		"auth_index":  "idx-a",
		"status_code": http.StatusTooManyRequests,
		"model":       "claude-opus-4-6-thinking",
		"error":       genericBodies[0],
	})))

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.GetCooldowns()) != 0 {
		t.Fatalf("generic RESOURCE_EXHAUSTED must not create cooldowns, got %#v", store.GetCooldowns())
	}

	diag, err := r.Diagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pending, ok := diag["pending_429"].([]map[string]any); ok && len(pending) > 0 {
		t.Fatalf("generic RESOURCE_EXHAUSTED must not enter pending_429, got %#v", pending)
	}
	if storm, ok := diag["rate_limit_storm"].(map[string]any); ok && len(storm) > 0 {
		t.Fatalf("generic RESOURCE_EXHAUSTED must not trigger rate_limit_storm, got %#v", storm)
	}

	history, err := r.GetRequestHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 10 {
		t.Fatalf("expected 10 request history entries, got %d", len(history))
	}
	for _, rec := range history {
		if !rec.Failed || rec.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("unexpected history status: %+v", rec)
		}
		if rec.FailureScope != string(state.FailureRequestScoped) {
			t.Fatalf("expected failure_scope=%q, got %q (%+v)", state.FailureRequestScoped, rec.FailureScope, rec)
		}
		if !strings.Contains(rec.ErrorMessage, "RESOURCE_EXHAUSTED") {
			t.Fatalf("expected raw error payload preserved in ErrorMessage, got %q", rec.ErrorMessage)
		}
	}
	if got := readPriority(t, paths["idx-a"]); got != 100 {
		t.Fatalf("host priority = %d; want untouched 100", got)
	}
}

func TestHealth_Real429QuotaAndRateLimitTriggerCooldown(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	r, _, _, cachePath, _ := newHealthRuntime(t, now, "idx-quota", "idx-rate", "idx-custom")

	quotaExhaustedBody := `{"error":{"code":429,"message":"You have exhausted your capacity on this model. Your quota will reset after 2h.","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"QUOTA_EXHAUSTED","domain":"cloudcode-pa.googleapis.com","metadata":{"quotaResetDelay":"2h"}}]}}`
	rateLimitExceededBody := `{"error":{"code":429,"message":"Too many requests","status":"RESOURCE_EXHAUSTED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"RATE_LIMIT_EXCEEDED"},{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":"10s"}]}}`
	customNoDetailsBody := `{"error":{"code":429,"message":"Quota exceeded for quota metric 'Generate Content API requests per minute'.","status":"RESOURCE_EXHAUSTED"}}`
	genericExemptBody := `{"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`

	// 1. QUOTA_EXHAUSTED x2 -> triggers cooldown.
	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
			"Provider":  "antigravity",
			"Model":     "claude-opus-4-6-thinking",
			"AuthIndex": "idx-quota",
			"Failed":    true,
			"Failure":   map[string]any{"StatusCode": http.StatusTooManyRequests, "Body": quotaExhaustedBody},
		})))
	}

	// 2. RATE_LIMIT_EXCEEDED x1 + generic exempt 429 x2 -> must NOT escalate yet.
	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
		"Provider":  "antigravity",
		"Model":     "gemini-3.8-flash-high",
		"AuthIndex": "idx-rate",
		"Failed":    true,
		"Failure":   map[string]any{"StatusCode": http.StatusTooManyRequests, "Body": rateLimitExceededBody},
	})))
	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
			"Provider":  "antigravity",
			"Model":     "gemini-3.8-flash-high",
			"AuthIndex": "idx-rate",
			"Failed":    true,
			"Failure":   map[string]any{"StatusCode": http.StatusTooManyRequests, "Body": genericExemptBody},
		})))
	}
	storeMid, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := storeMid.GetCooldowns()[state.CooldownKey("idx-rate", "gemini")]; ok {
		t.Fatalf("generic 429 must not escalate pending real 429 into cooldown: %#v", storeMid.GetCooldowns())
	}

	// 3. Second real RATE_LIMIT_EXCEEDED -> now escalates to cooldown.
	mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
		"Provider":  "antigravity",
		"Model":     "gemini-3.8-flash-high",
		"AuthIndex": "idx-rate",
		"Failed":    true,
		"Failure":   map[string]any{"StatusCode": http.StatusTooManyRequests, "Body": rateLimitExceededBody},
	})))

	// 4. Custom quota exceeded message without details x2 -> fail-safe cooldown.
	for range 2 {
		mustOK(t, r.Handle(ctx, runtime.MethodUsageHandle, marshalHealthTestJSON(t, map[string]any{
			"Provider":  "antigravity",
			"Model":     "gemini-3.8-flash-high",
			"AuthIndex": "idx-custom",
			"Failed":    true,
			"Failure":   map[string]any{"StatusCode": http.StatusTooManyRequests, "Body": customNoDetailsBody},
		})))
	}

	store, err := state.Load(ctx, cachePath)
	if err != nil {
		t.Fatal(err)
	}
	cooldowns := store.GetCooldowns()
	for _, key := range []string{
		state.CooldownKey("idx-quota", "claude_gpt"),
		state.CooldownKey("idx-rate", "gemini"),
		state.CooldownKey("idx-custom", "gemini"),
	} {
		if _, ok := cooldowns[key]; !ok {
			t.Fatalf("expected cooldown %q to be recorded, got %#v", key, cooldowns)
		}
	}

	history, err := r.GetRequestHistory(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var exemptCount, realCount int
	for _, rec := range history {
		if rec.FailureScope == string(state.FailureRequestScoped) {
			exemptCount++
		} else {
			realCount++
		}
	}
	if exemptCount != 2 || realCount != 6 {
		t.Fatalf("expected 2 exempt and 6 real 429 history entries, got exempt=%d real=%d", exemptCount, realCount)
	}
}

func marshalHealthTestJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	return data
}
