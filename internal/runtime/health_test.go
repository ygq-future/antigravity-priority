package runtime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/core"
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
