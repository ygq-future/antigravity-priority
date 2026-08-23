package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"antigravity-priority/internal/management"
)

func TestDevServerHTTPExposesFullIdentityAndStatefulSamples(t *testing.T) {
	now := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	runner := newDevRunnerWithClock(func() time.Time { return now })
	server := httptest.NewServer(management.NewHandler(runner))
	defer server.Close()

	response, err := http.Get(server.URL + "/snapshot/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var snapshot struct {
		Groups map[string]struct {
			Items []struct {
				Email     string `json:"email"`
				AuthIndex string `json:"auth_index"`
			} `json:"items"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatal(err)
	}
	item := snapshot.Groups["gemini"].Items[0]
	if item.Email != "work-gemini-pro@corp.com" || item.AuthIndex != "auth_ag_001" {
		t.Fatalf("HTTP snapshot identity = %+v", item)
	}

	for range 5 {
		now = now.Add(time.Minute)
		request, err := http.NewRequest(http.MethodPost, server.URL+"/run?mode=probe", nil)
		if err != nil {
			t.Fatal(err)
		}
		probeResponse, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		probeResponse.Body.Close()
		if probeResponse.StatusCode != http.StatusOK {
			t.Fatalf("probe status = %d", probeResponse.StatusCode)
		}
	}

	samplesResponse, err := http.Get(server.URL + "/samples?auth_index=auth_ag_001")
	if err != nil {
		t.Fatal(err)
	}
	defer samplesResponse.Body.Close()
	var samplesPayload struct {
		Groups map[string]struct {
			Samples []struct {
				Sequence uint64 `json:"sequence"`
			} `json:"samples"`
		} `json:"groups"`
	}
	if err := json.NewDecoder(samplesResponse.Body).Decode(&samplesPayload); err != nil {
		t.Fatal(err)
	}
	samples := samplesPayload.Groups["gemini"].Samples
	if len(samples) != 4 || samples[0].Sequence != 1 || samples[3].Sequence != 4 {
		t.Fatalf("HTTP samples do not reflect probe lifecycle: %+v", samples)
	}
}

func TestDevRunnerSnapshotCarriesFullCPAIdentity(t *testing.T) {
	now := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	runner := newDevRunnerWithClock(func() time.Time { return now })

	snapshot, err := runner.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	items := snapshot.Groups["gemini"].Items
	if len(items) == 0 {
		t.Fatal("dev snapshot has no credentials")
	}
	if items[0].Identity.Email != "work-gemini-pro@corp.com" || items[0].Identity.AuthIndex != "auth_ag_001" {
		t.Fatalf("dev identity = %+v; want full CPA email and auth index", items[0].Identity)
	}
	changes := snapshot.Groups["gemini"].Changes
	if len(changes) == 0 || changes[0].Identity.AuthIndex == "" {
		t.Fatalf("dev change identity is missing: %+v", changes)
	}
}

func TestDevRunnerProbeScenarioExercisesSampleLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 23, 9, 0, 0, 0, time.UTC)
	runner := newDevRunnerWithClock(func() time.Time { return now })
	runner.dynamicConfig.QuotaSampleCapacity = 3
	ctx := context.Background()
	const authIndex = "auth_ag_001"

	initial, err := runner.GetSamples(ctx, authIndex, "gemini")
	if err != nil || len(initial) != 1 || initial[0].Sequence != 1 {
		t.Fatalf("initial samples = %+v, err=%v", initial, err)
	}

	now = now.Add(time.Minute)
	if _, err := runner.Run(ctx, management.RunRequest{Mode: "probe"}); err != nil {
		t.Fatal(err)
	}
	unchanged, _ := runner.GetSamples(ctx, authIndex, "gemini")
	if len(unchanged) != 1 || unchanged[0].Sequence != 1 || !unchanged[0].ObservedAt.Equal(now) {
		t.Fatalf("unchanged probe did not refresh in place: %+v", unchanged)
	}

	now = now.Add(time.Minute)
	_, _ = runner.Run(ctx, management.RunRequest{Mode: "probe"})
	changed, _ := runner.GetSamples(ctx, authIndex, "gemini")
	if len(changed) != 2 || changed[1].Sequence != 2 {
		t.Fatalf("changed quota did not append: %+v", changed)
	}

	now = now.Add(time.Minute)
	_, _ = runner.Run(ctx, management.RunRequest{Mode: "probe"})
	deduplicated, _ := runner.GetSamples(ctx, authIndex, "gemini")
	if len(deduplicated) != 2 || !deduplicated[1].ObservedAt.Equal(now) {
		t.Fatalf("second unchanged probe was not deduplicated: %+v", deduplicated)
	}

	now = now.Add(time.Minute)
	_, _ = runner.Run(ctx, management.RunRequest{Mode: "probe"})
	reset, _ := runner.GetSamples(ctx, authIndex, "gemini")
	if len(reset) != 3 || reset[2].Sequence != 3 || reset[2].ShortWindowResetAt.Equal(reset[1].ShortWindowResetAt) {
		t.Fatalf("window reset did not append distinct history: %+v", reset)
	}
	entry := runner.sampleStates[devSampleKey(authIndex, "gemini")]
	if entry.baseline != reset[2].Sequence {
		t.Fatalf("window reset baseline = %d, want %d", entry.baseline, reset[2].Sequence)
	}

	now = now.Add(time.Minute)
	_, _ = runner.Run(ctx, management.RunRequest{Mode: "probe"})
	rotated, _ := runner.GetSamples(ctx, authIndex, "gemini")
	if len(rotated) != 3 || rotated[0].Sequence != 2 || rotated[2].Sequence != 4 {
		t.Fatalf("FIFO rotation did not retain the newest three samples: %+v", rotated)
	}
}
