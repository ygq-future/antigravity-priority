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
)

func TestProjectDualModelGroups_ControlDirectionsAndIndependentEvidence(t *testing.T) {
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
			readyEvidence("auth-1", core.PlanTypePro, 75, now.Add(5*time.Hour)),
		}},
		config.AntigravityModelGroupClaudeGPT: {Eligible: []priority.QuotaEvidence{
			readyEvidence("auth-1", core.PlanTypeFree, 61, now.Add(4*time.Hour)),
		}},
	}

	tests := []struct {
		name    string
		control config.AntigravityModelGroup
		other   config.AntigravityModelGroup
	}{
		{name: "gemini control", control: config.AntigravityModelGroupGemini, other: config.AntigravityModelGroupClaudeGPT},
		{name: "claude gpt control", control: config.AntigravityModelGroupClaudeGPT, other: config.AntigravityModelGroupGemini},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
				ControlModelGroup: tt.control,
				Credentials:       credentials,
				EvidenceByGroup:   groupEvidence,
				PlanningOptions: priority.Options{
					BoostStartPriority:  999,
					NormalStartPriority: 100,
					MinChange:           1,
				},
				ProjectionTime: now,
			})
			if err != nil {
				t.Fatal(err)
			}
			if projection.ControlPlan.DecidedAt != now {
				t.Fatalf("control plan decided_at = %s, want %s", projection.ControlPlan.DecidedAt, now)
			}
			if projection.Snapshot.ActiveModelGroup != string(tt.control) {
				t.Fatalf("active_model_group = %q, want %q", projection.Snapshot.ActiveModelGroup, tt.control)
			}
			if !projection.Snapshot.ObservedAt.Equal(now) {
				t.Fatalf("observed_at = %s, want %s", projection.Snapshot.ObservedAt, now)
			}
			if len(projection.Snapshot.Groups) != 3 {
				t.Fatalf("groups count = %d, want 3 active canonical groups", len(projection.Snapshot.Groups))
			}

			control := projection.Snapshot.Groups[string(tt.control)]
			other := projection.Snapshot.Groups[string(tt.other)]
			argon := projection.Snapshot.Groups[string(config.AntigravityModelGroupArgon)]

			if len(control.Items) != 1 || control.Items[0].IsPredicted {
				t.Fatalf("control items = %#v, want one non-predicted item", control.Items)
			}
			if len(other.Items) != 1 || other.Items[0].IsPredicted {
				t.Fatalf("other items = %#v, want one non-predicted item", other.Items)
			}
			if len(argon.Items) != 1 || argon.Items[0].IsPredicted {
				t.Fatalf("argon items = %#v, want one non-predicted item", argon.Items)
			}

			// Verify AllQuotaWindows is populated on every item
			qw := control.Items[0].QuotaWindows
			if qw.Gemini5h.Remaining == nil || *qw.Gemini5h.Remaining != 92 {
				t.Errorf("control QuotaWindows Gemini5h = %v, want 92", qw.Gemini5h.Remaining)
			}
			if qw.Claude7d.Remaining == nil || *qw.Claude7d.Remaining != 61 {
				t.Errorf("control QuotaWindows Claude7d = %v, want 61", qw.Claude7d.Remaining)
			}
			if qw.Claude5h.Remaining != nil {
				t.Errorf("control QuotaWindows Claude5h = %v, want nil on Free tier", qw.Claude5h.Remaining)
			}
			if qw.Argon7d.Remaining == nil || *qw.Argon7d.Remaining != 75 {
				t.Errorf("control QuotaWindows Argon7d = %v, want 75", qw.Argon7d.Remaining)
			}
		})
	}
}

func TestProjectDualModelGroups_QuotaWindowsRemainPerCredentialWhenRedactedIndexesCollide(t *testing.T) {
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
			readyEvidence("auth-alpha", core.PlanTypePro, 74, now.Add(5*time.Hour)),
			readyEvidence("auth-beta", core.PlanTypePro, 38, now.Add(6*time.Hour)),
		}},
	}

	projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup:   groupEvidence,
		PlanningOptions: priority.Options{
			BoostStartPriority:  999,
			NormalStartPriority: 100,
			MinChange:           1,
		},
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

func TestProjectDualModelGroups_MissingGroupRemainsStableAndUnknown(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{
		Name:      "account-1",
		AuthIndex: "auth-1",
		Provider:  core.ProviderAntigravity,
		Type:      core.CredentialTypeAntigravity,
		PlanType:  core.PlanTypeUnknown,
		Priority:  42,
	}}
	projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{readyEvidence("auth-1", core.PlanTypePlus, 88, now.Add(2*time.Hour))}},
		},
		PlanningOptions: priority.Options{NormalStartPriority: 100, MinChange: 1},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	missing := projection.Snapshot.Groups[string(config.AntigravityModelGroupClaudeGPT)]
	if len(missing.Items) != 1 {
		t.Fatalf("missing group = %#v, want 1 item", missing)
	}
	if missing.Items[0].PlanType != "unknown" || missing.Items[0].Target.Priority != 42 {
		t.Fatalf("missing group borrowed or invented data: %#v", missing.Items[0])
	}
	if len(missing.Changes) != 0 {
		t.Fatalf("missing group changes = %#v, want none", missing.Changes)
	}
}

func TestProjectDualModelGroups_HistoricalEvidenceIsReadOnly(t *testing.T) {
	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	credentials := []core.Credential{{AuthIndex: "auth-1", Priority: 42}}
	projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: historicalEvidenceResult(readyEvidence("auth-1", core.PlanTypePro, 88, now.Add(100*time.Hour))),
		},
		PlanningOptions: priority.Options{NormalStartPriority: 100, MinChange: 1},
		ProjectionTime:  now,
	})
	if err != nil {
		t.Fatal(err)
	}
	control := projection.Snapshot.Groups[string(config.AntigravityModelGroupGemini)]
	if len(control.Items) != 1 || control.Items[0].EvidenceFresh {
		t.Fatalf("historical control item = %#v; want read-only evidence", control.Items)
	}
	if len(control.Changes) != 0 || control.Items[0].Target.Priority != 100 {
		t.Fatalf("historical control projection = %#v; want predicted target without write change", control)
	}
}

func TestProjectDualModelGroups_DoesNotMutateInputsOrReturnedSnapshots(t *testing.T) {
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
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup: map[config.AntigravityModelGroup]evidence.Result{
			config.AntigravityModelGroupGemini: {Eligible: []priority.QuotaEvidence{fresh}},
		},
		PlanningOptions: priority.Options{NormalStartPriority: 100, MinChange: 1},
		ProjectionTime:  now,
	}
	first, err := runtime.ProjectDualModelGroups(input)
	if err != nil {
		t.Fatal(err)
	}
	first.Snapshot.Groups["gemini"] = apply.GroupSnapshot{}
	first.ControlPlan.Items[0].Credential.Priority = 999
	if credentials[0].Priority != 42 || fresh.Remaining == nil || *fresh.Remaining != 88 {
		t.Fatalf("projection mutated input: credentials=%#v evidence=%#v", credentials, fresh)
	}
	second, err := runtime.ProjectDualModelGroups(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Snapshot.Groups["gemini"].Items) != 1 || second.ControlPlan.Items[0].Credential.Priority != 42 {
		t.Fatalf("projection result was not independent: %#v / %#v", second.Snapshot, second.ControlPlan.Items)
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
	first.Groups["gemini"] = apply.GroupSnapshot{Items: []apply.SnapshotItem{{AuthIndex: "mutated"}}}

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

func TestProjectDualModelGroups_ThreePoolIndependentPlansAndQuotaWindows(t *testing.T) {
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

	projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup:   evidenceByGroup,
		PlanningOptions:   priority.Options{NormalStartPriority: 100},
		ProjectionTime:    now,
	})
	if err != nil {
		t.Fatalf("ProjectDualModelGroups failed: %v", err)
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

func TestProjectDualModelGroups_NoArgonBucketDoesNotFabricateArgon7d(t *testing.T) {
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

	projection, err := runtime.ProjectDualModelGroups(runtime.ProjectionInput{
		ControlModelGroup: config.AntigravityModelGroupGemini,
		Credentials:       credentials,
		EvidenceByGroup:   evidenceByGroup,
		PlanningOptions:   priority.Options{NormalStartPriority: 100},
		ProjectionTime:    now,
	})
	if err != nil {
		t.Fatalf("ProjectDualModelGroups failed: %v", err)
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
	}
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
