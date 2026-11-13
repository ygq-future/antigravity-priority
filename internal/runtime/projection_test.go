package runtime_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/provider/antigravity"
	"antigravity-priority/internal/runtime"
	"antigravity-priority/internal/state"
)

func TestProjectModelGroups_AllPoolsAreActiveAndIndependent(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{
		{
			Name:      "account-1",
			AuthIndex: "auth-1",
			Provider:  core.ProviderAntigravity,
			Type:      core.CredentialTypeAntigravity,
			Priority:  100,
		},
	}

	groupEvidence := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-1", core.PlanTypePro, 92, now.Add(3*time.Hour)),
		}},
		config.AntigravityModelGroupArgon: {Eligible: []priority.QuotaEvidence{
			readyArgonEvidence("auth-1", core.PlanTypePro, 75, now.Add(5*time.Hour)),
		}},
		config.AntigravityModelGroupClaudeGPT: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-1", core.PlanTypeFree, 61, now.Add(4*time.Hour)),
		}},
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: groupEvidence,
		PlanningOptions: priority.Options{},
		ProjectionTime: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !projection.Snapshot.ObservedAt.Equal(now) {
		t.Fatalf("observed_at = %s, want %s", projection.Snapshot.ObservedAt, now)
	}
	if len(projection.Snapshot.Groups) != 3 {
		t.Fatalf("groups count = %d, want 3 active canonical groups", len(projection.Snapshot.Groups))
	}

	for _, group := range config.AllAntigravityModelGroups {
		pool := projection.Snapshot.Groups[string(group)]
		if len(pool.Items) != 1 {
			t.Fatalf("%s items = %#v, want one item", group, pool.Items)
		}
	}

	gemini := projection.Snapshot.Groups[string(config.AntigravityModelGroupGemini)]
	qw := gemini.Items[0].QuotaWindows
	if qw.Gemini5h.Remaining == nil || *qw.Gemini5h.Remaining != 92 {
		t.Errorf("QuotaWindows Gemini5h = %v, want 92", qw.Gemini5h.Remaining)
	}
	if qw.Claude7d.Remaining == nil || *qw.Claude7d.Remaining != 61 {
		t.Errorf("QuotaWindows Claude7d = %v, want 61", qw.Claude7d.Remaining)
	}
	if qw.Claude5h.Remaining != nil {
		t.Errorf("QuotaWindows Claude5h = %v, want nil on Free tier", qw.Claude5h.Remaining)
	}
	if qw.Argon7d.Remaining == nil || *qw.Argon7d.Remaining != 75 {
		t.Errorf("QuotaWindows Argon7d = %v, want 75", qw.Argon7d.Remaining)
	}
}

func TestProjectModelGroups_QuotaWindowsRemainPerCredentialWhenRedactedIndexesCollide(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{
		{
			Name:      "account-alpha",
			AuthIndex: "auth-alpha",
			Provider:  core.ProviderAntigravity,
			Type:      core.CredentialTypeAntigravity,
			Priority:  100,
		},
		{
			Name:      "account-beta",
			AuthIndex: "auth-beta",
			Provider:  core.ProviderAntigravity,
			Type:      core.CredentialTypeAntigravity,
			Priority:  99,
		},
	}
	groupEvidence := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-alpha", core.PlanTypePro, 92, now.Add(3*time.Hour)),
			readyEvidence("auth-beta", core.PlanTypePro, 61, now.Add(4*time.Hour)),
		}},
		config.AntigravityModelGroupClaudeGPT: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-alpha", core.PlanTypePro, 83, now.Add(3*time.Hour)),
			readyEvidence("auth-beta", core.PlanTypePro, 47, now.Add(4*time.Hour)),
		}},
		config.AntigravityModelGroupArgon: {Eligible: []priority.QuotaEvidence{
			readyArgonEvidence("auth-alpha", core.PlanTypePro, 74, now.Add(5*time.Hour)),
			readyArgonEvidence("auth-beta", core.PlanTypePro, 38, now.Add(6*time.Hour)),
		}},
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: groupEvidence,
		PlanningOptions: priority.Options{},
		ProjectionTime: now,
	})
	if err != nil {
		t.Fatal(err)
	}

	items := projection.Snapshot.Groups[string(config.AntigravityModelGroupGemini)].Items
	if len(items) != 2 {
		t.Fatalf("gemini items = %d, want 2", len(items))
	}
	byAuth := make(map[string]apply.SnapshotItem, len(items))
	for _, item := range items {
		byAuth[item.Identity.AuthIndex] = item
	}
	alpha, ok := byAuth["auth-alpha"]
	if !ok {
		t.Fatal("missing auth-alpha item")
	}
	beta, ok := byAuth["auth-beta"]
	if !ok {
		t.Fatal("missing auth-beta item")
	}
	// Display indexes are redacted to the same prefix; internal correlation must
	// not collapse the two credentials onto one quota window set.
	if alpha.AuthIndex != beta.AuthIndex {
		t.Fatalf("expected redacted display auth indexes to collide, got %q and %q", alpha.AuthIndex, beta.AuthIndex)
	}
	assertWindow := func(name string, bar apply.QuotaWindowBar, want int64) {
		t.Helper()
		if bar.Remaining == nil || *bar.Remaining != want {
			t.Errorf("%s remaining = %v, want %d", name, bar.Remaining, want)
		}
	}
	assertWindow("auth-alpha gemini5h", alpha.QuotaWindows.Gemini5h, 92)
	assertWindow("auth-beta gemini5h", beta.QuotaWindows.Gemini5h, 61)
	assertWindow("auth-alpha claude7d", alpha.QuotaWindows.Claude7d, 83)
	assertWindow("auth-beta claude7d", beta.QuotaWindows.Claude7d, 47)
	assertWindow("auth-alpha argon7d", alpha.QuotaWindows.Argon7d, 74)
	assertWindow("auth-beta argon7d", beta.QuotaWindows.Argon7d, 38)
}

func TestProjectModelGroups_MissingGroupRemainsStableAndUnknown(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{
		Name:      "account-1",
		AuthIndex: "auth-1",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		PlanType:  core.PlanTypeUnknown,
		Priority:  42,
	}}
	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials: credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{readyEvidence("auth-1", core.PlanTypePlus, 88, now.Add(2*time.Hour))}},
		},
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	missing := projection.Snapshot.Groups[string(config.AntigravityModelGroupClaudeGPT)]
	if len(missing.Items) != 1 {
		t.Fatalf("missing group = %#v, want 1 item", missing)
	}
	if missing.Items[0].PlanType != "unknown" || missing.Items[0].Host.Priority != 42 || missing.Items[0].Target.Priority != 0 || missing.Items[0].RoutingStatus != apply.RoutingStatusColdStart {
		t.Fatalf("missing group borrowed or invented data: %#v", missing.Items[0])
	}
}

func TestProjectModelGroups_HistoricalEvidenceProjectsStoredQuota(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{AuthIndex: "auth-1", Priority: 42}}
	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials: credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: historicalEvidenceResult(readyEvidence("auth-1", core.PlanTypePro, 88, now.Add(100*time.Hour))),
		},
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	control := projection.Snapshot.Groups[string(config.AntigravityModelGroupGemini)]
	if len(control.Items) != 1 || control.Items[0].Target.Priority != 100 {
		t.Fatalf("historical control item = %#v; want stored-quota target 100", control.Items)
	}
}

func TestProjectModelGroups_DoesNotMutateInputsOrReturnedSnapshots(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{
		Name:      "account-1",
		AuthIndex: "auth-1",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		Priority:  42,
	}}
	fresh := readyEvidence("auth-1", core.PlanTypePro, 88, now.Add(2*time.Hour))
	input := runtime.ProjectionInput{
		Credentials: credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{fresh}},
		},
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	}
	first, err := runtime.ProjectModelGroups(input)
	if err != nil {
		t.Fatal(err)
	}
	originalTarget := first.Snapshot.Groups["gemini"].Items[0].Target.Priority
	mutated := first.Snapshot.Groups["gemini"]
	mutated.Items[0].Target.Priority = 999
	first.Snapshot.Groups["gemini"] = mutated
	if credentials[0].Priority != 42 || fresh.Remaining == nil || *fresh.Remaining != 88 {
		t.Fatalf("projection mutated input: credentials=%#v evidence=%#v", credentials, fresh)
	}
	second, err := runtime.ProjectModelGroups(input)
	if err != nil {
		t.Fatal(err)
	}
	secondGemini := second.Snapshot.Groups["gemini"]
	if len(secondGemini.Items) != 1 || secondGemini.Items[0].Target.Priority != originalTarget {
		t.Fatalf("projection result was not independent: %#v", second.Snapshot)
	}
}

func TestRuntime_LatestSnapshotFallbackIsStableAndComplete(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	r := runtime.New(runtime.Options{
		Clock:          fixedClock{now: now},
		StateCachePath: filepath.Join(t.TempDir(), "startup-cache.json"),
	})
	cachePath := filepath.Join(t.TempDir(), "projection-fallback.json")
	if _, err := r.Register(context.Background(), runtime.RegisterRequest{
		ConfigYAML: fmt.Sprintf("state_cache_path: %q\n", cachePath),
	}); err != nil {
		t.Fatal(err)
	}

	first, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Groups) != 3 || len(first.Groups["gemini"].Items) != 0 || len(first.Groups["claude_gpt"].Items) != 0 || len(first.Groups["argon"].Items) != 0 {
		t.Fatalf("fallback = %#v, want three stable empty groups", first)
	}
	first.Groups["gemini"] = apply.PoolSnapshot{Items: []apply.SnapshotItem{{AuthIndex: "mutated"}}}

	second, err := r.LatestSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !second.ObservedAt.Equal(first.ObservedAt) {
		t.Fatalf("fallback observed_at changed from %s to %s", first.ObservedAt, second.ObservedAt)
	}
	if len(second.Groups["gemini"].Items) != 0 {
		t.Fatalf("stored fallback was exposed for mutation: %#v", second.Groups["gemini"])
	}
}

func TestProjectModelGroups_ThreePoolIndependentPlansAndQuotaWindows(t *testing.T) {
	raw := []byte(`{"groups":[
		{"displayName":"Gemini Models","description":"Shared quota for Gemini models","buckets":[
			{"bucketId":"gemini-5h","displayName":"Five Hour Limit","window":"5h","remainingFraction":0.99,"resetTime":"2026-10-28T17:00:00Z"},
			{"bucketId":"gemini-weekly","displayName":"Weekly Limit","window":"weekly","remainingFraction":0.79,"resetTime":"2026-10-31T00:00:00Z"}
		]},
		{"displayName":"Gemini 4 Argon","description":"For paid Pro plans, Argon quota is separate.","buckets":[
			{"bucketId":"gemini-argon-wtu-weekly","displayName":"Gemini 4 Argon Weekly Limit","window":"weekly","remainingFraction":0.71,"resetTime":"2026-10-29T23:24:33Z"}
		]},
		{"displayName":"Claude and GPT Models","description":"Shared quota for Claude and GPT models","buckets":[
			{"bucketId":"claude-5h","displayName":"Five Hour Limit","window":"5h","remainingFraction":0.80,"resetTime":"2026-10-28T17:00:00Z"},
			{"bucketId":"claude-weekly","displayName":"Weekly Limit","window":"weekly","remainingFraction":0.45,"resetTime":"2026-10-31T00:00:00Z"}
		]}
	]}`)
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	parsed := antigravity.ParseAllModelGroups(raw, now)
	credentials := []core.Credential{{
		Name:      "pro-account",
		AuthIndex: "auth-pro",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		Priority:  100,
	}}

	evidenceByGroup := make(map[config.AntigravityModelGroup]evidence.Result)
	for group, res := range parsed {
		evidenceByGroup[group] = evidence.Result{
			Eligible: []priority.QuotaEvidence{{
				AuthIndex:            "auth-pro",
				Provider:             core.ProviderAntigravity,
				ModelGroup:           group,
				ObservedAt:           now,
				ResetAt:              res.ResetAt,
				Remaining:            res.Remaining,
				ShortWindowResetAt:   res.ShortWindowResetAt,
				ShortWindowRemaining: res.ShortWindowRemaining,
				LongWindowResetAt:    res.LongWindowResetAt,
				LongWindowRemaining:  res.LongWindowRemaining,
				ArgonWindowResetAt:   res.ArgonWindowResetAt,
				ArgonWindowRemaining: res.ArgonWindowRemaining,
				PlanType:             res.PlanType,
			}},
		}
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: evidenceByGroup,
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatalf("ProjectModelGroups failed: %v", err)
	}

	// Verify independent plans
	geminiItem := projection.Snapshot.Groups["gemini"].Items[0]
	if math.Abs(geminiItem.R5h-0.99) > 1e-4 || math.Abs(geminiItem.R7d-0.79) > 1e-4 {
		t.Fatalf("gemini plan R5h=%v R7d=%v, want R5h=0.99 R7d=0.79", geminiItem.R5h, geminiItem.R7d)
	}

	argonItem := projection.Snapshot.Groups["argon"].Items[0]
	if math.Abs(argonItem.R5h-0.99) > 1e-4 || math.Abs(argonItem.R7d-0.71) > 1e-4 {
		t.Fatalf("argon plan R5h=%v R7d=%v, want R5h=0.99 R7d=0.71", argonItem.R5h, argonItem.R7d)
	}

	claudeItem := projection.Snapshot.Groups["claude_gpt"].Items[0]
	if math.Abs(claudeItem.R5h-0.80) > 1e-4 || math.Abs(claudeItem.R7d-0.45) > 1e-4 {
		t.Fatalf("claude plan R5h=%v R7d=%v, want R5h=0.80 R7d=0.45", claudeItem.R5h, claudeItem.R7d)
	}

	// Verify populated 5-bar QuotaWindows across all 3 groups
	for _, group := range []string{"gemini", "argon", "claude_gpt"} {
		item := projection.Snapshot.Groups[group].Items[0]
		qw := item.QuotaWindows
		if qw.Gemini5h.Remaining == nil || *qw.Gemini5h.Remaining != 99 {
			t.Errorf("[%s] QuotaWindows Gemini5h = %v, want 99", group, qw.Gemini5h.Remaining)
		}
		if qw.Gemini7d.Remaining == nil || *qw.Gemini7d.Remaining != 79 {
			t.Errorf("[%s] QuotaWindows Gemini7d = %v, want 79", group, qw.Gemini7d.Remaining)
		}
		if qw.Claude5h.Remaining == nil || *qw.Claude5h.Remaining != 80 {
			t.Errorf("[%s] QuotaWindows Claude5h = %v, want 80", group, qw.Claude5h.Remaining)
		}
		if qw.Claude7d.Remaining == nil || *qw.Claude7d.Remaining != 45 {
			t.Errorf("[%s] QuotaWindows Claude7d = %v, want 45", group, qw.Claude7d.Remaining)
		}
		if qw.Argon7d.Remaining == nil || *qw.Argon7d.Remaining != 71 {
			t.Errorf("[%s] QuotaWindows Argon7d = %v, want 71", group, qw.Argon7d.Remaining)
		}
	}
}

func TestProjectModelGroups_NoArgonBucketDoesNotFabricateArgon7d(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{
		Name:      "pro-no-argon",
		AuthIndex: "auth-no-argon",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		Priority:  100,
	}}

	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(120 * time.Hour)
	rem5h := int64(95)
	rem7d := int64(88)

	evidenceByGroup := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {
			Eligible: []priority.QuotaEvidence{{
				AuthIndex:            "auth-no-argon",
				Provider:             core.ProviderAntigravity,
				ModelGroup:           config.AntigravityModelGroupGemini,
				ObservedAt:           now,
				ResetAt:              &reset5h,
				Remaining:            &rem5h,
				ShortWindowResetAt:   &reset5h,
				ShortWindowRemaining: &rem5h,
				LongWindowResetAt:    &reset7d,
				LongWindowRemaining:  &rem7d,
				PlanType:             core.PlanTypePro,
			}},
		},
		config.AntigravityModelGroupClaudeGPT: {
			Eligible: []priority.QuotaEvidence{{
				AuthIndex:            "auth-no-argon",
				Provider:             core.ProviderAntigravity,
				ModelGroup:           config.AntigravityModelGroupClaudeGPT,
				ObservedAt:           now,
				ResetAt:              &reset5h,
				Remaining:            &rem5h,
				ShortWindowResetAt:   &reset5h,
				ShortWindowRemaining: &rem5h,
				LongWindowResetAt:    &reset7d,
				LongWindowRemaining:  &rem7d,
				PlanType:             core.PlanTypePro,
			}},
		},
		// Argon has NO quota evidence because parseArgonGroup found no Argon windows
		config.AntigravityModelGroupArgon: {},
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: evidenceByGroup,
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatalf("ProjectModelGroups failed: %v", err)
	}

	for _, group := range []string{"gemini", "argon", "claude_gpt"} {
		item := projection.Snapshot.Groups[group].Items[0]
		qw := item.QuotaWindows
		if qw.Gemini5h.Remaining == nil || *qw.Gemini5h.Remaining != 95 {
			t.Errorf("[%s] QuotaWindows Gemini5h = %v, want 95", group, qw.Gemini5h.Remaining)
		}
		if qw.Gemini7d.Remaining == nil || *qw.Gemini7d.Remaining != 88 {
			t.Errorf("[%s] QuotaWindows Gemini7d = %v, want 88", group, qw.Gemini7d.Remaining)
		}
		if qw.Argon7d.Remaining != nil {
			t.Errorf("[%s] QuotaWindows Argon7d.Remaining = %v, want nil when no Argon bucket exists", group, *qw.Argon7d.Remaining)
		}
		if qw.ArgonSharedWithGemini {
			t.Errorf("[%s] QuotaWindows ArgonSharedWithGemini = true, want false for a Pro account", group)
		}
	}
}

func TestProjectModelGroups_UltraSharedArgonMarksGeminiWindow(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{
		Name:      "ultra-shared-argon",
		AuthIndex: "auth-ultra",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		Priority:  100,
	}}

	reset5h := now.Add(4 * time.Hour)
	reset7d := now.Add(120 * time.Hour)
	rem5h := int64(90)
	rem7d := int64(88)
	sharedEvidence := func(group config.AntigravityModelGroup) priority.QuotaEvidence {
		return priority.QuotaEvidence{
			AuthIndex:            "auth-ultra",
			Provider:             core.ProviderAntigravity,
			ModelGroup:           group,
			ObservedAt:           now,
			ResetAt:              &reset5h,
			Remaining:            &rem5h,
			ShortWindowResetAt:   &reset5h,
			ShortWindowRemaining: &rem5h,
			LongWindowResetAt:    &reset7d,
			LongWindowRemaining:  &rem7d,
			PlanType:             core.PlanTypeUltra,
		}
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials: credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini:    {Eligible: []priority.QuotaEvidence{sharedEvidence(config.AntigravityModelGroupGemini)}},
			config.AntigravityModelGroupClaudeGPT: {Eligible: []priority.QuotaEvidence{sharedEvidence(config.AntigravityModelGroupClaudeGPT)}},
			// Argon has no independent bucket; the probe copied the Gemini pool.
			config.AntigravityModelGroupArgon: {Eligible: []priority.QuotaEvidence{sharedEvidence(config.AntigravityModelGroupArgon)}},
		},
		PlanningOptions: priority.Options{},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatalf("ProjectModelGroups failed: %v", err)
	}

	argon := projection.Snapshot.Groups["argon"].Items[0]
	qw := argon.QuotaWindows
	if !qw.ArgonSharedWithGemini {
		t.Fatal("argon_shared_with_gemini = false; want true for an Ultra shared pool")
	}
	if qw.Argon7d.Remaining != nil {
		t.Fatalf("Argon7d.Remaining = %v; want nil for an independent-bucket-free Ultra account", qw.Argon7d.Remaining)
	}
	if qw.Gemini7d.Remaining == nil || *qw.Gemini7d.Remaining != 88 {
		t.Fatalf("Gemini7d.Remaining = %v; want 88 as the governing Argon weekly window", qw.Gemini7d.Remaining)
	}
	if argon.Target.Priority != 100 {
		t.Fatalf("argon priority = %d; want 100 from shared Gemini evidence", argon.Target.Priority)
	}
}

func readyArgonEvidence(authIndex string, planType core.PlanType, remaining int64, resetAt time.Time) priority.QuotaEvidence {
	evidence := readyEvidence(authIndex, planType, remaining, resetAt)
	evidence.ArgonWindowRemaining = &remaining
	evidence.ArgonWindowResetAt = &resetAt
	evidence.LongWindowRemaining = &remaining
	evidence.LongWindowResetAt = &resetAt
	return evidence
}

func readyEvidence(authIndex string, planType core.PlanType, remaining int64, resetAt time.Time) priority.QuotaEvidence {
	return priority.QuotaEvidence{
		AuthIndex:  authIndex,
		Provider:   core.ProviderAntigravity,
		ObservedAt: resetAt.Add(-time.Hour),
		ResetAt:    &resetAt,
		Remaining:  &remaining,
		PlanType:   planType,
	}
}

func historicalEvidenceResult(values ...priority.QuotaEvidence) evidence.Result {
	observations := make([]evidence.Observation, 0, len(values))
	for _, value := range values {
		copy := value
		observations = append(observations, evidence.Observation{
			AuthIndex:  copy.AuthIndex,
			ModelGroup: copy.ModelGroup,
			Kind:       evidence.ObservationHistorical,
			ObservedAt: copy.ObservedAt,
			Evidence:   &copy,
		})
	}
	return evidence.Result{Observations: observations}
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestProjectModelGroups_CarriesPerCredentialWarmupState(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{
		{Name: "account-1", AuthIndex: "auth-1", Provider: core.ProviderAntigravity, Type: core.CredentialTypeAntigravity, Priority: 100},
		{Name: "account-2", AuthIndex: "auth-2", Provider: core.ProviderAntigravity, Type: core.CredentialTypeAntigravity, Priority: 100},
		{Name: "account-3", AuthIndex: "auth-3", Provider: core.ProviderAntigravity, Type: core.CredentialTypeAntigravity, Priority: 100, PlanType: core.PlanTypeFree},
	}
	groupEvidence := map[config.AntigravityModelGroup]evidence.Result{
		config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-1", core.PlanTypePro, 90, now.Add(3*time.Hour)),
			readyEvidence("auth-2", core.PlanTypePro, 80, now.Add(3*time.Hour)),
			readyEvidence("auth-3", core.PlanTypeFree, 70, now.Add(3*time.Hour)),
		}},
	}

	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials:     credentials,
		EvidenceByGroup: groupEvidence,
		PlanningOptions: priority.Options{},
		ProjectionTime: now,
		Warmup: config.WarmupConfig{
			GeminiEnabled: true,
			Overrides:     map[string]bool{"auth-2": false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, group := range config.AllAntigravityModelGroups {
		geminiToggle := group == config.AntigravityModelGroupGemini || group == config.AntigravityModelGroupArgon
		pool := projection.Snapshot.Groups[string(group)]
		if len(pool.Items) != 3 {
			t.Fatalf("%s items = %d, want 3", group, len(pool.Items))
		}
		for _, item := range pool.Items {
			switch item.Identity.AuthIndex {
			case "auth-1":
				if item.WarmupOverridden {
					t.Errorf("%s auth-1 warmup should follow the global toggle", group)
				}
				if item.WarmupActive != geminiToggle {
					t.Errorf("%s auth-1 warmup active = %v, want %v", group, item.WarmupActive, geminiToggle)
				}
			case "auth-2":
				if !item.WarmupOverridden {
					t.Errorf("%s auth-2 warmup should carry a local override", group)
				}
				if item.WarmupActive {
					t.Errorf("%s auth-2 warmup active = true, want false for a local off override", group)
				}
			case "auth-3":
				if item.WarmupOverridden {
					t.Errorf("%s auth-3 must not expose a warm-up state for a Free plan", group)
				}
				if item.WarmupActive {
					t.Errorf("%s auth-3 warmup active = true, want false for a Free plan", group)
				}
			default:
				t.Fatalf("%s unexpected credential %q", group, item.Identity.AuthIndex)
			}
		}
	}
}

func TestProjectModelGroups_LocalWarmupOverrideBeatsGlobalOff(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{
		{Name: "account-1", AuthIndex: "auth-1", Provider: core.ProviderAntigravity, Type: core.CredentialTypeAntigravity, Priority: 100},
	}
	projection, err := runtime.ProjectModelGroups(runtime.ProjectionInput{
		Credentials: credentials,
		PlanningOptions: priority.Options{},
		ProjectionTime: now,
		Warmup:         config.WarmupConfig{Overrides: map[string]bool{"auth-1": true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range config.AllAntigravityModelGroups {
		pool := projection.Snapshot.Groups[string(group)]
		if len(pool.Items) != 1 {
			t.Fatalf("%s items = %d, want 1", group, len(pool.Items))
		}
		item := pool.Items[0]
		if !item.WarmupOverridden || !item.WarmupActive {
			t.Errorf("%s local override = true must stay active while the global toggle is off (overridden=%v active=%v)", group, item.WarmupOverridden, item.WarmupActive)
		}
	}
}

func TestEvaluateRoutingAndSnapshot_UnifiesScoresAndStatusesAcrossTablesAndSnapshot(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	store, err := state.Load(ctx, filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	rem80 := int64(80)
	reset5h := now.Add(3 * time.Hour)
	reset7d := now.Add(72 * time.Hour)

	// Healthy active credential in Gemini; unavailable in Argon
	if err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex: "auth-active", Provider: core.ProviderAntigravity, ModelGroup: "gemini",
		ObservedAt: now, ShortWindowResetAt: reset5h, ShortWindowRemaining: &rem80,
		LongWindowResetAt: reset7d, LongWindowRemaining: &rem80, PlanType: core.PlanTypePro,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkProbeUnavailable(ctx, state.ProbeUnavailable{
		AuthIndex: "auth-active", Provider: core.ProviderAntigravity, ModelGroup: "argon",
		ObservedAt: now, Reason: "no argon bucket",
	}); err != nil {
		t.Fatal(err)
	}

	// Cold-start credential with stale high Host priority=999, currently in 429 cooldown in Gemini only
	store.SetCooldown(state.CooldownEntry{
		AuthIndex:     "auth-cold-cool",
		ModelGroup:    "gemini",
		FailureKind:   state.FailureRateLimit,
		StatusCode:    429,
		TriggeredAt:   now,
		CooldownUntil: now.Add(5 * time.Minute),
		Reason:        priority.Reason429Cooldown,
	})

	credentials := []core.Credential{
		{AuthIndex: "auth-active", Name: "active", Provider: core.ProviderAntigravity, Priority: 50},
		{AuthIndex: "auth-cold-cool", Name: "cold-cool", Provider: core.ProviderAntigravity, Priority: 999},
		{AuthIndex: "auth-disabled", Name: "disabled", Provider: core.ProviderAntigravity, Priority: 900, Disabled: true},
	}

	out, err := runtime.EvaluateRoutingAndSnapshot(runtime.RoutingEngineInput{
		Credentials: credentials,
		Store:       store,
		Config:      config.Default(),
		Now:         now,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify SnapshotSet items match PoolRoutingTable candidates identically in every pool.
	for _, group := range config.AllAntigravityModelGroups {
		table := out.Tables[group]
		snapItems := out.Snapshot.Groups[string(group)].Items
		if len(snapItems) != len(credentials) {
			t.Fatalf("%s snapshot items = %d, want %d", group, len(snapItems), len(credentials))
		}
		for _, item := range snapItems {
			cand, ok := table.ByAuthIndex[item.Identity.AuthIndex]
			if !ok {
				t.Fatalf("%s missing routing table candidate for %s", group, item.Identity.AuthIndex)
			}
			if item.Target.Priority != cand.Priority {
				t.Errorf("%s %s snapshot priority=%d != table priority=%d", group, item.Identity.AuthIndex, item.Target.Priority, cand.Priority)
			}
			if item.RoutingStatus != cand.RoutingStatus {
				t.Errorf("%s %s snapshot status=%q != table status=%q", group, item.Identity.AuthIndex, item.RoutingStatus, cand.RoutingStatus)
			}
			if item.PoolEligible == nil || *item.PoolEligible != cand.PoolEligible {
				t.Errorf("%s %s snapshot pool_eligible=%v != table=%v", group, item.Identity.AuthIndex, item.PoolEligible, cand.PoolEligible)
			}
		}
	}

	// Specific assertions for Gemini pool
	geminiByAuth := make(map[string]apply.SnapshotItem)
	for _, item := range out.Snapshot.Groups["gemini"].Items {
		geminiByAuth[item.Identity.AuthIndex] = item
	}
	if geminiByAuth["auth-active"].RoutingStatus != apply.RoutingStatusActive || geminiByAuth["auth-active"].Target.Priority != 100 {
		t.Errorf("auth-active gemini = %+v, want active priority 100", geminiByAuth["auth-active"])
	}
	if geminiByAuth["auth-cold-cool"].RoutingStatus != apply.RoutingStatusCooldown || geminiByAuth["auth-cold-cool"].Target.Priority != -1 || geminiByAuth["auth-cold-cool"].Host.Priority != 999 {
		t.Errorf("auth-cold-cool gemini = %+v, want cooldown -1 with Host.Priority=999 preserved", geminiByAuth["auth-cold-cool"])
	}
	if geminiByAuth["auth-disabled"].RoutingStatus != apply.RoutingStatusDisabledOnHost || geminiByAuth["auth-disabled"].Target.Priority != -2 || !geminiByAuth["auth-disabled"].Target.Disabled {
		t.Errorf("auth-disabled gemini = %+v, want disabled -2", geminiByAuth["auth-disabled"])
	}

	// In Claude pool, auth-cold-cool is NOT cooling, so it must be cold_start with score 0 (not stale 999)
	claudeByAuth := make(map[string]apply.SnapshotItem)
	for _, item := range out.Snapshot.Groups["claude_gpt"].Items {
		claudeByAuth[item.Identity.AuthIndex] = item
	}
	if claudeByAuth["auth-cold-cool"].RoutingStatus != apply.RoutingStatusColdStart || claudeByAuth["auth-cold-cool"].Target.Priority != 0 || claudeByAuth["auth-cold-cool"].Host.Priority != 999 {
		t.Errorf("auth-cold-cool claude_gpt = %+v, want cold_start 0 with Host.Priority=999 preserved", claudeByAuth["auth-cold-cool"])
	}

	// In Argon pool, auth-active is unavailable -> pool_unavailable, pool_eligible=false, priority=-1
	argonByAuth := make(map[string]apply.SnapshotItem)
	for _, item := range out.Snapshot.Groups["argon"].Items {
		argonByAuth[item.Identity.AuthIndex] = item
	}
	if argonByAuth["auth-active"].RoutingStatus != apply.RoutingStatusPoolUnavailable || argonByAuth["auth-active"].Target.Priority != -1 || argonByAuth["auth-active"].PoolEligible == nil || *argonByAuth["auth-active"].PoolEligible {
		t.Errorf("auth-active argon = %+v, want pool_unavailable -1 with pool_eligible=false", argonByAuth["auth-active"])
	}
}
