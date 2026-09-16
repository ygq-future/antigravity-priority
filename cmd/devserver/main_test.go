package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/provider/antigravity"
)

type devTestClock struct {
	now time.Time
}

func (c *devTestClock) Now() time.Time {
	return c.now
}

func TestDevHostSeedsAndReadsCPAAuthFiles(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	hostAdapter, err := newDevHost(devHostOptions{
		AuthDir:        t.TempDir(),
		AccountCount:   10,
		QuotaStatePath: filepath.Join(t.TempDir(), "quota.json"),
		Seed:           7,
		NowFn:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	files, err := hostAdapter.ListAuthFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 10 {
		t.Fatalf("auth file count = %d, want 10", len(files))
	}
	if files[0].AuthIndex == "" || files[0].Email == "" || files[0].Type != "antigravity" {
		t.Fatalf("auth file identity = %+v", files[0])
	}

	document, err := hostAdapter.GetAuth(context.Background(), files[0].AuthIndex)
	if err != nil {
		t.Fatal(err)
	}
	if document.Path == "" || !strings.HasSuffix(document.Path, ".json") {
		t.Fatalf("auth document path = %q", document.Path)
	}
	var raw map[string]any
	if err := json.Unmarshal(document.JSON, &raw); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"access_token", "disabled", "email", "expired", "expires_in", "priority", "project_id", "refresh_token", "timestamp", "type"} {
		if _, ok := raw[field]; !ok {
			t.Fatalf("auth file is missing %q: %s", field, document.JSON)
		}
	}

	data, err := os.ReadFile(document.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(document.JSON) {
		t.Fatalf("GetAuth did not expose the physical document")
	}
	raw["disabled"] = true
	raw["priority"] = float64(12)
	updated, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(document.Path, updated, 0o600); err != nil {
		t.Fatal(err)
	}
	updatedFiles, err := hostAdapter.ListAuthFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !updatedFiles[0].Disabled || updatedFiles[0].Priority != 12 {
		t.Fatalf("host list did not reread disabled/priority edits: %+v", updatedFiles[0])
	}
	runtimeAuth, err := hostAdapter.GetRuntime(context.Background(), updatedFiles[0].AuthIndex)
	if err != nil {
		t.Fatal(err)
	}
	if !runtimeAuth.Disabled {
		t.Fatalf("runtime auth did not reflect disabled edit: %+v", runtimeAuth)
	}
}

func TestDevHostReturnsBothModelGroupsAndQuotaLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	hostAdapter, err := newDevHost(devHostOptions{
		AuthDir:        t.TempDir(),
		AccountCount:   1,
		QuotaStatePath: filepath.Join(t.TempDir(), "quota.json"),
		Seed:           1,
		NowFn:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	files, err := hostAdapter.ListAuthFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	readQuota := func() map[antigravity.ModelGroup]antigravity.ProbeResult {
		t.Helper()
		response, err := hostAdapter.HTTPDo(context.Background(), host.HTTPRequest{
			AuthIndex: files[0].AuthIndex,
			Method:    http.MethodPost,
			URL:       antigravity.RetrieveUserQuotaSummaryURL,
			Body:      []byte(`{"project":"dev-project-001"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("simulated quota status = %d", response.StatusCode)
		}
		var contract struct {
			Groups []struct {
				DisplayName string `json:"displayName"`
				Buckets     []struct {
					Window string `json:"window"`
				} `json:"buckets"`
			} `json:"groups"`
		}
		if err := json.Unmarshal(response.Body, &contract); err != nil {
			t.Fatalf("decode simulated quota contract: %v", err)
		}
		if len(contract.Groups) != 2 || len(contract.Groups[0].Buckets) != 2 || len(contract.Groups[1].Buckets) != 2 {
			t.Fatalf("simulated quota response must expose two official group/bucket pairs: %+v", contract.Groups)
		}
		return antigravity.ParseAllModelGroups(response.Body, now)
	}

	previous := readQuota()
	if previous[antigravity.ModelGroupGemini].ShortWindowRemaining == nil || previous[antigravity.ModelGroupClaudeGPT].LongWindowRemaining == nil {
		t.Fatalf("simulated response did not contain both complete groups: %+v", previous)
	}
	depleted := false
	for round := 0; round < 40; round++ {
		now = now.Add(time.Minute)
		current := readQuota()
		previouslyExhausted := false
		for _, group := range simulatedModelGroups {
			old := previous[antigravity.ModelGroup(group)]
			if old.ShortWindowRemaining != nil && old.LongWindowRemaining != nil && (*old.ShortWindowRemaining == 0 || *old.LongWindowRemaining == 0) {
				previouslyExhausted = true
			}
		}
		for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
			old := previous[group]
			item := current[group]
			if old.ShortWindowRemaining == nil || old.LongWindowRemaining == nil || item.ShortWindowRemaining == nil || item.LongWindowRemaining == nil {
				t.Fatalf("round %d group %s lost a window", round, group)
			}
			if previouslyExhausted {
				if *item.ShortWindowRemaining != 100 || *item.LongWindowRemaining != 100 {
					t.Fatalf("round %d group %s did not recover after credential exhaustion: short=%d/%d long=%d/%d", round, group, *old.ShortWindowRemaining, *item.ShortWindowRemaining, *old.LongWindowRemaining, *item.LongWindowRemaining)
				}
				depleted = true
			} else if *item.ShortWindowRemaining >= *old.ShortWindowRemaining || *item.LongWindowRemaining >= *old.LongWindowRemaining {
				t.Fatalf("round %d group %s did not consume both windows: old=%+v current=%+v", round, group, old, item)
			}
		}
		previous = current
		if depleted {
			break
		}
	}
	if !depleted {
		t.Fatal("deterministic quota simulation did not reach exhaustion")
	}
}

func TestDevRuntimeUsesProductionPathForProbeApplyAndManagement(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	clock := &devTestClock{now: now}
	dev, err := newDevServer(devServerOptions{
		AuthDir:        t.TempDir(),
		QuotaStatePath: filepath.Join(t.TempDir(), "quota.json"),
		StateCachePath: filepath.Join(t.TempDir(), "cache.json"),
		AccountCount:   10,
		Seed:           3,
		Clock:          clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := dev.runtime.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown runtime: %v", err)
		}
	}()

	if err := dev.runtime.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	snapshot, err := dev.runtime.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []string{"gemini", "claude_gpt"} {
		if len(snapshot.Groups[group].Items) != 10 {
			t.Fatalf("%s item count = %d, want 10", group, len(snapshot.Groups[group].Items))
		}
	}
	if snapshot.PreviewID == "" {
		t.Fatal("probe did not publish a preview id")
	}
	if err := dev.runtime.ManualApplyWithPreview(context.Background(), config.AntigravityModelGroupGemini, nil, snapshot.PreviewID); err != nil {
		t.Fatalf("manual apply with preview failed: %v", err)
	}
	postApplySnapshot, err := dev.runtime.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if postApplySnapshot.PreviewID != "" || len(postApplySnapshot.Groups["gemini"].Changes) != 0 {
		t.Fatalf("post-apply snapshot = %#v; want consumed preview and no pending changes", postApplySnapshot)
	}

	samples, err := dev.runtime.GetSamples(context.Background(), "dev-auth-001", "gemini")
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("initial Gemini samples = %d, want 1", len(samples))
	}
	claudeSamples, err := dev.runtime.GetSamples(context.Background(), "dev-auth-001", "claude_gpt")
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeSamples) != 1 {
		t.Fatalf("initial Claude/GPT samples = %d, want 1", len(claudeSamples))
	}

	dynamic, err := dev.runtime.GetDynamicConfig(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	dynamic.AutoApply = true
	dynamic.Interval = "1m"
	if err := dev.runtime.SetDynamicConfig(context.Background(), dynamic); err != nil {
		t.Fatal(err)
	}
	if err := dev.runtime.AutoApply(context.Background()); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := dev.runtime.Diagnostics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	scheduler, ok := diagnostics["scheduler"].(map[string]any)
	if !ok {
		t.Fatalf("scheduler diagnostics = %#v", diagnostics["scheduler"])
	}
	if last, ok := scheduler["last_auto_apply_at"].(time.Time); !ok || last.IsZero() {
		t.Fatalf("auto scheduler did not record a run: %#v", scheduler["last_auto_apply_at"])
	}

	if err := dev.runtime.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	previewSnapshot, err := dev.runtime.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.runtime.ManualApplyWithPreview(context.Background(), config.AntigravityModelGroupGemini, nil, previewSnapshot.PreviewID); err != nil {
		t.Fatal(err)
	}
	files, err := dev.host.ListAuthFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		document, err := dev.host.GetAuth(context.Background(), file.AuthIndex)
		if err != nil {
			t.Fatal(err)
		}
		var raw struct {
			Priority *int `json:"priority"`
		}
		if err := json.Unmarshal(document.JSON, &raw); err != nil {
			t.Fatal(err)
		}
		if raw.Priority == nil {
			t.Fatalf("apply did not write priority for %s: %s", file.AuthIndex, document.JSON)
		}
	}

	server := httptest.NewServer(dev.runtime.ManagementHandler())
	defer server.Close()
	response, err := http.Get(server.URL + management.PathDiagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("diagnostics status = %d", response.StatusCode)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}

	clock.now = clock.now.Add(time.Minute)
	if err := dev.runtime.Probe(context.Background(), config.AntigravityModelGroupGemini, nil); err != nil {
		t.Fatal(err)
	}
	updatedSamples, err := dev.runtime.GetSamples(context.Background(), "dev-auth-001", "gemini")
	if err != nil {
		t.Fatal(err)
	}
	if len(updatedSamples) < 2 {
		t.Fatalf("second probe did not append changed quota sample: %+v", updatedSamples)
	}
}

func TestDevServer_ManuallyDisabledAccountSimulation(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	clock := &devTestClock{now: now}
	dev, err := newDevServer(devServerOptions{
		AuthDir:        t.TempDir(),
		QuotaStatePath: filepath.Join(t.TempDir(), "quota.json"),
		StateCachePath: filepath.Join(t.TempDir(), "cache.json"),
		AccountCount:   2,
		Seed:           42,
		Clock:          clock,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dev.runtime.Shutdown(context.Background()) }()

	// 1. Manually disable account 0 on host
	files, err := dev.host.ListAuthFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	targetAuth := files[0].AuthIndex
	doc, err := dev.host.GetAuth(context.Background(), files[0].Name)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(doc.JSON, &raw); err != nil {
		t.Fatal(err)
	}
	raw["disabled"] = true
	updated, _ := json.Marshal(raw)
	if err := dev.host.SaveAuth(context.Background(), files[0].Name, updated); err != nil {
		t.Fatal(err)
	}

	// 2. Perform single-credential probe via management endpoint
	handler := dev.runtime.ManagementHandler()
	req := httptest.NewRequest(http.MethodPost, "/v0/management/plugins/antigravity-priority/run?mode=probe&auth_index="+targetAuth+"&antigravity_model_group=gemini", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("single probe failed: %d, body: %s", rec.Code, rec.Body.String())
	}

	// 3. Snapshot inspection: disabled on host, full quota visible, changes empty
	snapshot, err := dev.runtime.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	group := snapshot.Groups["gemini"]
	if len(group.Items) != 2 {
		t.Fatalf("expected all 2 accounts in snapshot after single-credential probe, got %d", len(group.Items))
	}
	found := false
	for _, it := range group.Items {
		if it.Identity.AuthIndex == targetAuth {
			found = true
			if it.Reason != "disabled on host" {
				t.Fatalf("expected reason %q, got %q", "disabled on host", it.Reason)
			}
			if !it.Current.Disabled || !it.Target.Disabled {
				t.Fatalf("expected item disabled on both current and target: %+v", it)
			}
			if it.R7d <= 0 || it.R5h <= 0 {
				t.Fatalf("expected valid non-zero quota for probed manually disabled item, got R7d=%v, R5h=%v", it.R7d, it.R5h)
			}
			break
		}
	}
	if !found {
		t.Fatalf("target credential %q not found in snapshot", targetAuth)
	}
	if len(group.Changes) != 0 {
		t.Fatalf("manually disabled account must not produce changes, got %+v", group.Changes)
	}
}
