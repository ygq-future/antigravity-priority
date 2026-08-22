package runtime_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/priority"
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
			if len(projection.Snapshot.Groups) != 2 {
				t.Fatalf("groups = %#v, want both canonical groups", projection.Snapshot.Groups)
			}

			control := projection.Snapshot.Groups[string(tt.control)]
			predicted := projection.Snapshot.Groups[string(tt.other)]
			if len(control.Items) != 1 || control.Items[0].IsPredicted {
				t.Fatalf("control items = %#v, want one non-predicted item", control.Items)
			}
			if len(predicted.Items) != 1 || !predicted.Items[0].IsPredicted {
				t.Fatalf("predicted items = %#v, want one predicted item", predicted.Items)
			}
			if control.Items[0].PlanType == predicted.Items[0].PlanType {
				t.Fatalf("group evidence was copied: control=%q predicted=%q", control.Items[0].PlanType, predicted.Items[0].PlanType)
			}
			if len(predicted.Changes) == 0 || predicted.Changes[0].Reason[:len("predicted: ")] != "predicted: " {
				t.Fatalf("predicted changes = %#v, want predicted reason", predicted.Changes)
			}
		})
	}
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
	if len(missing.Items) != 1 || !missing.Items[0].IsPredicted {
		t.Fatalf("missing group = %#v, want stable predicted item", missing)
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
	r := runtime.New(runtime.Options{Clock: fixedClock{now: now}})
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
	if len(first.Groups) != 2 || len(first.Groups["gemini"].Items) != 0 || len(first.Groups["claude_gpt"].Items) != 0 {
		t.Fatalf("fallback = %#v, want two stable empty groups", first)
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
