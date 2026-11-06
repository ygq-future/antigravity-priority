package runtime_test

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

func TestRuntime_RequestHistory_RecordsOnUsage(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	mock := newMockHost()
	mock.files = []host.AuthFile{{
		Name:      "dev-account-001",
		AuthIndex: "auth-1",
		Email:     "user1@example.com",
		Provider:  string(core.ProviderAntigravity),
	}}

	clock := &testClock{now: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	r := newTestRuntime(t, runtime.Options{
		Host:    mock,
		Clock:   clock,
		Sleeper: testSleeper{},
	})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Shutdown(context.Background()) }()

	successPayload := `{"Provider":"antigravity","Model":"claude-opus-4-6-thinking","AuthIndex":"auth-1","Failed":false,"Detail":{"InputTokens":1500,"OutputTokens":500,"TotalTokens":2000}}`
	resp1 := r.Handle(context.Background(), runtime.MethodUsageHandle, []byte(successPayload))
	var env1 struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp1, &env1); err != nil || !env1.OK {
		t.Fatalf("usage handle failed: %s", string(resp1))
	}

	// 2. Send 429 rate limit usage
	clock.Advance(5 * time.Second)
	failPayload := `{"Provider":"antigravity","Model":"gemini-3.8-flash-high","AuthIndex":"auth-1","Failed":true,"Failure":{"StatusCode":429,"Body":"RESOURCE_EXHAUSTED"}}`
	resp2 := r.Handle(context.Background(), runtime.MethodUsageHandle, []byte(failPayload))
	var env2 struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp2, &env2); err != nil || !env2.OK {
		t.Fatalf("usage handle 429 failed: %s", string(resp2))
	}

	// 3. Inspect request history
	records, err := r.GetRequestHistory(context.Background())
	if err != nil {
		t.Fatalf("get request history failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Newest should be first (the 429 error)
	rec0 := records[0]
	if rec0.Model != "gemini-3.8-flash-high" || rec0.StatusCode != 429 || !rec0.Failed {
		t.Errorf("record 0 mismatch: %+v", rec0)
	}
	if rec0.ModelPool != "gemini" {
		t.Errorf("expected model pool gemini, got %q", rec0.ModelPool)
	}

	// Oldest should be second (the 200 success)
	rec1 := records[1]
	if rec1.Model != "claude-opus-4-6-thinking" || rec1.StatusCode != 200 || rec1.Failed {
		t.Errorf("record 1 mismatch: %+v", rec1)
	}
	if rec1.ModelPool != "claude_gpt" {
		t.Errorf("expected model pool claude_gpt, got %q", rec1.ModelPool)
	}
	if rec1.InputTokens != 1500 || rec1.OutputTokens != 500 || rec1.TotalTokens != 2000 {
		t.Errorf("record 1 tokens mismatch: %+v", rec1)
	}

	// 4. Verify Diagnostics contains request_history
	diag, err := r.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	slice, okSlice := diag["request_history"].([]state.RequestHistoryEntry)
	if !okSlice {
		t.Fatalf("expected []state.RequestHistoryEntry in diagnostics, got %T", diag["request_history"])
	}
	if len(slice) != 2 {
		t.Errorf("expected 2 diag records, got %d", len(slice))
	}

	// 5. Clear Request History
	if err := r.ClearRequestHistory(context.Background()); err != nil {
		t.Fatalf("clear failed: %v", err)
	}
	recordsAfterClear, err := r.GetRequestHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(recordsAfterClear) != 0 {
		t.Fatalf("expected 0 records after clear, got %d", len(recordsAfterClear))
	}
}

func TestRuntime_RequestHistory_CapacityLimit100(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	clock := &testClock{now: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	r := newTestRuntime(t, runtime.Options{
		Clock:   clock,
		Sleeper: testSleeper{},
	})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Shutdown(context.Background()) }()

	// Feed 120 requests
	for i := 1; i <= 120; i++ {
		clock.Advance(time.Second)
		payload := fmt.Sprintf(`{"Provider":"antigravity","Model":"model-%03d","AuthIndex":"auth-1","Failed":false,"Detail":{"TotalTokens":%d}}`, i, i*10)
		_ = r.Handle(context.Background(), runtime.MethodUsageHandle, []byte(payload))
	}

	records, err := r.GetRequestHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 100 {
		t.Fatalf("expected exactly 100 records (max capacity), got %d", len(records))
	}

	// Newest should be model-120
	if records[0].Model != "model-120" {
		t.Errorf("expected newest to be model-120, got %s", records[0].Model)
	}
	// Oldest should be model-021 (120 - 100 + 1 = 21)
	if records[99].Model != "model-021" {
		t.Errorf("expected oldest to be model-021, got %s", records[99].Model)
	}
}

func TestRuntime_RequestHistory_ConcurrentSafety(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "cache.json")
	clock := &testClock{now: time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	r := newTestRuntime(t, runtime.Options{
		Clock:   clock,
		Sleeper: testSleeper{},
	})
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: "enabled: true\nstate_cache_path: " + filepath.ToSlash(cachePath) + "\n",
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Shutdown(context.Background()) }()

	var wg sync.WaitGroup
	for g := range 10 {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for range 20 {
				payload := fmt.Sprintf(`{"Provider":"antigravity","Model":"test-model","AuthIndex":"auth-%d","Failed":false,"Detail":{"TotalTokens":100}}`, workerID)
				_ = r.Handle(context.Background(), runtime.MethodUsageHandle, []byte(payload))
				_, _ = r.GetRequestHistory(context.Background())
			}
		}(g)
	}
	wg.Wait()

	records, err := r.GetRequestHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) > 100 {
		t.Errorf("expected at most 100 records, got %d", len(records))
	}
}
