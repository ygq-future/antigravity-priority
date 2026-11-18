package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/state"
)

func argonQuotaBody(now time.Time, window string, fraction float64) []byte {
	return fmt.Appendf(nil, `{"groups":[
		{"displayName":"Gemini Models","buckets":[{"bucketId":"gemini-5h","window":"5h","remainingFraction":0.8,"resetTime":%q}]},
		{"displayName":"Gemini 4 Argon","buckets":[{"bucketId":"argon","window":%q,"remainingFraction":%g,"resetTime":%q}]}]}`,
		now.Add(5*time.Hour).Format(time.RFC3339), window, fraction, now.Add(argonWarmupDebounce).Format(time.RFC3339))
}

func TestArgonManualBatchWhileResetLaneIsClosed(t *testing.T) {
	rt, stub, now, _ := newArgonRuntime(t)
	ctx := t.Context()
	for _, idx := range []string{"idx-b", "idx-c", "idx-d"} {
		stub.addFile(host.AuthFile{Name: idx, AuthIndex: idx, Provider: "antigravity", Status: "active"})
	}
	// The last observation had four full weeks; no generation has been sent yet.
	if err := rt.runTaskRequest(ctx, TaskRequest{Trigger: TriggerProbe, Warmup: WarmupScope{Suppress: true}}); err != nil {
		t.Fatal(err)
	}
	lane := state.WarmupLane{LastAuth: "idx-b", LastAt: now, NextAuth: "idx-a", NextAt: now.Add(time.Hour)}
	if err := rt.state.SetWarmupLane(ctx, lane); err != nil {
		t.Fatal(err)
	}
	stub.quotaBodyFor = func(index string) []byte {
		if index == "idx-a" {
			return argonQuotaBody(now, "weekly", 0.9999)
		}
		return argonQuotaBody(now, "weekly", 1)
	}
	stub.failGenerationFor = "idx-b"
	sweepAndDrain(t, rt, ctx)
	if got := stub.countCalls("generateContent"); got != 0 {
		t.Fatalf("closed reset lane sent %d generations", got)
	}
	if err := rt.Probe(ctx, nil); err != nil {
		t.Fatal(err)
	}
	waitForWarmups(t, rt)
	got := make(map[string]int)
	for _, req := range stub.httpCalls {
		if strings.Contains(req.URL, "generateContent") {
			got[req.AuthIndex]++
		}
	}
	if !maps.Equal(got, map[string]int{"idx-b": 1, "idx-c": 1, "idx-d": 1}) {
		t.Fatalf("generation counts = %v; want all three full weeks, even after a failure", got)
	}
	if got := rt.state.StoreOrNil().GetWarmupLane(); got != lane {
		t.Fatalf("weekly batch changed the 5h lane: %+v", got)
	}
}

func TestArgonAndGeminiResetWarmupsUseSeparateSlots(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, clock, _ := newWarmupSweepRuntime(t, now)
	ctx := t.Context()
	stub.addFile(host.AuthFile{Name: "account-b", AuthIndex: "idx-b", Provider: "antigravity", Status: "active"})
	dyn := config.Default().Dynamic()
	dyn.Warmup.GeminiEnabled = true
	dyn.Warmup.ArgonEnabled = true
	if err := rt.SetDynamicConfig(ctx, dyn); err != nil {
		t.Fatal(err)
	}
	stub.setBody(bytes.Replace(argonQuotaBody(now, "weekly", 1), []byte(`"remainingFraction":0.8`), []byte(`"remainingFraction":1`), 1))
	if err := rt.runTaskRequest(ctx, TaskRequest{Trigger: TriggerProbe, Warmup: WarmupScope{Suppress: true}}); err != nil {
		t.Fatal(err)
	}
	sweepAndDrain(t, rt, ctx)
	sweepAndDrain(t, rt, ctx)
	if got := stub.countCalls("generateContent"); got != 1 {
		t.Fatalf("first shared slot sent %d generations, want one", got)
	}
	lane := rt.state.StoreOrNil().GetWarmupLane()
	gap := lane.NextAt.Sub(now)
	if lane.LastAuth != "idx-a" || lane.NextAuth != "idx-a" || gap < WarmupLaneSameCredentialMinGap || gap >= WarmupLaneSameCredentialMaxGap {
		t.Fatalf("expected sibling pool of idx-a in next slot, got %+v", lane)
	}
	clock.advance(gap)
	sweepAndDrain(t, rt, ctx)
	got := make(map[string]int)
	for _, req := range stub.httpCalls {
		if !strings.Contains(req.URL, "generateContent") {
			continue
		}
		var payload antigravityGenerateEnvelope
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			t.Fatal(err)
		}
		got[req.AuthIndex+":"+payload.Model]++
	}
	want := map[string]int{"idx-a:" + config.DefaultArgonWarmupModel: 1, "idx-a:" + config.DefaultGeminiWarmupModel: 1}
	if !maps.Equal(got, want) {
		t.Fatalf("mixed week/5h generations = %v, want %v", got, want)
	}
}

func TestArgonDispatchUsesLatestLocalProbeWithoutExtraQuotaRequest(t *testing.T) {
	rt, stub, now, _ := newArgonRuntime(t)
	release := blockArgonBatch(t, rt, stub)
	// Change the upstream response while b is queued; its local cache stays full.
	stub.setBody(argonQuotaBody(now, "weekly", 0.9999))
	release()
	waitForWarmups(t, rt)
	if got := stub.countCalls("retrieveUserQuota"); got != 2 {
		t.Fatalf("sent %d quota queries, want only the two probe queries", got)
	}
	if got := stub.countCalls("generateContent"); got != 2 {
		t.Fatalf("sent %d generations; both probed weeks were full", got)
	}
}

func newArgonRuntime(t *testing.T) (*Runtime, *warmupStubHost, time.Time, string) {
	t.Helper()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	rt, stub, _, path := newWarmupSweepRuntime(t, now)
	dyn := config.Default().Dynamic()
	dyn.AutoApply = false
	dyn.Warmup.ArgonEnabled = true
	if err := rt.SetDynamicConfig(t.Context(), dyn); err != nil {
		t.Fatal(err)
	}
	stub.setBody(argonQuotaBody(now, "weekly", 1))
	return rt, stub, now, path
}

func TestArgonWarmupRequiresExactlyFullIndependentWeek(t *testing.T) {
	for _, tc := range []struct {
		window   string
		fraction float64
		want     int
	}{
		{"weekly", 1, 1}, {"weekly", 0.99, 0}, {"weekly", 0.999, 0},
		{"5h", 1, 0}, {"unknown", 1, 0},
	} {
		t.Run(fmt.Sprintf("%s-%g", tc.window, tc.fraction), func(t *testing.T) {
			rt, stub, now, _ := newArgonRuntime(t)
			stub.setBody(argonQuotaBody(now, tc.window, tc.fraction))
			if err := rt.Probe(t.Context(), nil); err != nil {
				t.Fatal(err)
			}
			waitForWarmups(t, rt)
			if got := stub.countCalls("generateContent"); got != tc.want {
				t.Fatalf("sent %d generations, want %d", got, tc.want)
			}
			// Fallback and rounded readings still exist for ordinary routing.
			entry, ok := rt.state.StoreOrNil().GetEntry("idx-a", "argon")
			if !ok || entry.ArgonWindowRemaining == nil {
				t.Fatal("lost routing evidence")
			}
			if got := warmupWindowDue(entry, entry.ArgonWindowResetAt, now); got != (tc.want == 1) {
				t.Fatalf("watcher due = %v", got)
			}
		})
	}
}

func blockArgonBatch(t *testing.T, rt *Runtime, stub *warmupStubHost) func() {
	t.Helper()
	stub.addFile(host.AuthFile{Name: "account-b", AuthIndex: "idx-b", Provider: "antigravity", Status: "active"})
	stub.generationStarted = make(chan struct{}, 1)
	stub.generationRelease = make(chan struct{})
	release := sync.OnceFunc(func() { close(stub.generationRelease) })
	t.Cleanup(release)
	if err := rt.Probe(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stub.generationStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first generation did not start")
	}
	return release
}

func TestArgonWarmupDequeuesAgainstLatestProbe(t *testing.T) {
	rt, stub, now, _ := newArgonRuntime(t)
	release := blockArgonBatch(t, rt, stub)
	stub.setBody(argonQuotaBody(now, "weekly", 0.8))
	if err := rt.Probe(t.Context(), []string{"idx-b"}); err != nil {
		t.Fatal(err)
	}
	release()
	waitForWarmups(t, rt)
	if got := stub.countCalls("generateContent"); got != 1 || !rt.state.StoreOrNil().GetLastWarmupAt("idx-b", "argon").IsZero() {
		t.Fatalf("stale queued job sent or debounced: %d total generations", got)
	}
}

func TestArgonWarmupQuiesceDrainsAndRegisterResumes(t *testing.T) {
	rt, stub, _, _ := newArgonRuntime(t)
	release := blockArgonBatch(t, rt, stub)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := rt.Quiesce(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("quiesce during a blocked generation: %v", err)
	}
	release()
	if err := rt.Quiesce(t.Context()); err != nil {
		t.Fatal(err)
	}
	probeB := func(want int) {
		t.Helper()
		if err := rt.Probe(t.Context(), []string{"idx-b"}); err != nil {
			t.Fatal(err)
		}
		waitForWarmups(t, rt)
		if got := stub.countCalls("generateContent"); got != want {
			t.Fatalf("got %d generations, want %d", got, want)
		}
	}
	probeB(1)
	if _, err := rt.Register(t.Context(), RegisterRequest{ConfigYAML: "enabled: true"}); err != nil {
		t.Fatal(err)
	}
	probeB(2)
}

func TestArgonWarmupShutdownPersistsLateSuccess(t *testing.T) {
	rt, stub, now, path := newArgonRuntime(t)
	// The synchronous host callback can return HTTP 200 after root cancellation.
	stub.finishGenerationOnCancel = true
	blockArgonBatch(t, rt, stub)
	if err := rt.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	store, err := state.Load(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !store.GetLastWarmupAt("idx-a", "argon").Equal(now) || !store.GetLastWarmupAt("idx-b", "argon").IsZero() {
		t.Fatal("expected only the completed generation in persisted debounce state")
	}
}
