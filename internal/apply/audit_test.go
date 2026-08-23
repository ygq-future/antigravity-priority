package apply_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
)

func TestSnapshot_And_AuditEvent_Redaction(t *testing.T) {
	now := time.Now().UTC()
	resetAt := now.Add(2 * time.Hour)
	rem := int64(80)

	plan := priority.Plan{
		Items: []priority.PlanItem{
			{
				Credential: core.Credential{
					Name:      "user-cred",
					AuthIndex: "idx-auth-123",
					Provider:  core.ProviderAntigravity,
					Type:      "antigravity",
					Status:    core.CredentialStatusActive,
					Account:   "acc-token=secret-acc",
					Email:     "user-bearer-xyz@example.com",
					Priority:  20,
					Disabled:  false,
				},
				Priority:      90,
				Disabled:      false,
				EvidenceFresh: true,
				ResetAt:       &resetAt,
				Remaining:     &rem,
				Reason:        "reason containing token=supersecret",
			},
			{
				Credential: core.Credential{
					Name:      "stale-cred",
					AuthIndex: "idx-stale-456",
					Priority:  10,
					Disabled:  false,
				},
				Priority:      10,
				Disabled:      false,
				EvidenceFresh: false,
				Reason:        "keep current state",
			},
		},
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Name:      "user-cred",
					AuthIndex: "idx-auth-123",
					Account:   "acc-token=secret-acc",
					Email:     "user-bearer-xyz@example.com",
					Priority:  20,
					Disabled:  false,
				},
				Priority:      90,
				Disabled:      false,
				EvidenceFresh: true,
				Reason:        "change with token=supersecret",
			},
			{
				Credential: core.Credential{
					Name:      "stale-cred",
					AuthIndex: "idx-stale-456",
					Priority:  10,
				},
				Priority:      10,
				EvidenceFresh: false,
				Reason:        "stale change",
			},
		},
	}

	// 1. Snapshot test
	snapshot := apply.Snapshot(plan)
	if snapshot.TotalItems != 2 || snapshot.TotalChanges != 2 {
		t.Fatalf("unexpected snapshot counts: %+v", snapshot)
	}

	snapshotJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	snapshotStr := string(snapshotJSON)
	if snapshot.Items[0].Identity.Email != "user-bearer-xyz@example.com" || snapshot.Items[0].Identity.AuthIndex != "idx-auth-123" {
		t.Fatalf("trusted in-memory identity was not preserved: %+v", snapshot.Items[0].Identity)
	}
	if strings.Contains(snapshotStr, `"Identity"`) || strings.Contains(snapshotStr, "idx-auth-123") || strings.Contains(snapshotStr, "user-bearer-xyz@example.com") {
		t.Errorf("audit JSON contains trusted in-memory identity: %s", snapshotStr)
	}

	if strings.Contains(snapshotStr, "secret-acc") || strings.Contains(snapshotStr, "supersecret") {
		t.Errorf("snapshot contains unredacted secrets: %s", snapshotStr)
	}
	if !strings.Contains(snapshotStr, host.RedactedValue) {
		t.Errorf("snapshot expected to contain REDACTED: %s", snapshotStr)
	}

	// 2. Transition result projection test
	_, transitionStore := newDocumentFixture(t, map[string]string{
		"idx-auth-123":  `{"priority":20,"disabled":false}`,
		"idx-stale-456": `{"priority":10,"disabled":false}`,
	})
	result, err := apply.Apply(context.Background(), apply.Request{
		Transition:        apply.NewHostTransition(transitionStore),
		Plan:              plan,
		ReportSkippedPlan: true,
	})
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}

	event := result.Event
	if event.Action != "host.transition" {
		t.Errorf("expected action 'host.transition', got %s", event.Action)
	}
	if event.TotalChanges != 1 {
		t.Errorf("expected TotalChanges=1 for executable transitions, got %d", event.TotalChanges)
	}
	if event.FreshChanges != 1 {
		t.Errorf("expected FreshChanges=1, got %d", event.FreshChanges)
	}
	if event.SkippedChanges != 0 {
		t.Errorf("expected SkippedChanges=0 for executable transitions, got %d", event.SkippedChanges)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}
	eventStr := string(eventJSON)
	if strings.Contains(eventStr, "supersecret") {
		t.Errorf("audit event contains unredacted secrets: %s", eventStr)
	}
	if !strings.Contains(eventStr, host.RedactedValue) {
		t.Errorf("audit event expected to contain REDACTED: %s", eventStr)
	}
}

func TestResultName_FallbackHierarchy(t *testing.T) {
	// Test resultName fallback: Account -> Email -> Name -> AuthIndex
	plan := priority.Plan{
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					Account:   "my-account",
					Email:     "my-email@example.com",
					Name:      "my-name",
					AuthIndex: "my-idx",
				},
				Priority:      50,
				EvidenceFresh: true,
			},
			{
				Credential: core.Credential{
					Email:     "my-email2@example.com",
					Name:      "my-name2",
					AuthIndex: "my-idx2",
				},
				Priority:      50,
				EvidenceFresh: true,
			},
			{
				Credential: core.Credential{
					Name:      "my-name3",
					AuthIndex: "my-idx3",
				},
				Priority:      50,
				EvidenceFresh: true,
			},
			{
				Credential: core.Credential{
					AuthIndex: "my-idx4",
				},
				Priority:      50,
				EvidenceFresh: true,
			},
			{
				Credential: core.Credential{},
				Priority:   50,
			},
		},
	}

	res, err := apply.Apply(context.Background(), apply.Request{
		Transition: apply.NewHostTransition(&documentFixture{paths: map[string]string{}}),
		Plan:       plan,
	})
	if err != nil {
		t.Fatalf("apply error: %v", err)
	}

	if res.Changes[0].Name != "my***nt" {
		t.Errorf("expected redacted account identity, got %s", res.Changes[0].Name)
	}
	if res.Changes[1].Name != "my***om" {
		t.Errorf("expected redacted email identity, got %s", res.Changes[1].Name)
	}
	if res.Changes[2].Name != "my***e3" {
		t.Errorf("expected redacted name identity, got %s", res.Changes[2].Name)
	}
	if res.Changes[3].Name != "my***x4" {
		t.Errorf("expected redacted auth identity, got %s", res.Changes[3].Name)
	}
	if res.Changes[4].Name != "" {
		t.Errorf("expected Name '', got %s", res.Changes[4].Name)
	}
}

func TestRedactedErrors(t *testing.T) {
	// Test error redaction helpers through plan error simulation
	res := apply.FailureResult(priority.PlanItem{
		Credential: core.Credential{AuthIndex: "idx-err"},
	}, errors.New("authorization: Bearer super-secret-key-123"))

	if strings.Contains(res.Error, "super-secret-key-123") {
		t.Errorf("error contains secret key: %s", res.Error)
	}
	if !strings.Contains(res.Error, host.RedactedValue) {
		t.Errorf("error expected to contain REDACTED: %s", res.Error)
	}

	// Test nil error
	if got := apply.RedactedError(nil); got != "" {
		t.Errorf("expected empty string for nil error, got %q", got)
	}

	// Test empty errors list
	if got := apply.RedactedErrors(nil); got != "" {
		t.Errorf("expected empty string for nil errors, got %q", got)
	}
	if got := apply.RedactedErrors([]error{}); got != "" {
		t.Errorf("expected empty string for empty errors, got %q", got)
	}

	// Test list with sensitive errors
	errs := []error{
		errors.New("first error token=secret1"),
		errors.New("second error api_key=secret2"),
	}
	encoded := apply.RedactedErrors(errs)
	if strings.Contains(encoded, "secret1") || strings.Contains(encoded, "secret2") {
		t.Errorf("redactedErrors contains secrets: %s", encoded)
	}
	if !strings.Contains(encoded, host.RedactedValue) {
		t.Errorf("redactedErrors expected to contain REDACTED: %s", encoded)
	}
}

func TestSnapshot_PriorityMissing_Preservation(t *testing.T) {
	plan := priority.Plan{
		Items: []priority.PlanItem{
			{
				Credential: core.Credential{
					AuthIndex:       "unset-cred",
					Priority:        0,
					PriorityMissing: true,
					Disabled:        false,
				},
				Priority: 100,
				Disabled: false,
			},
			{
				Credential: core.Credential{
					AuthIndex:       "zero-cred",
					Priority:        0,
					PriorityMissing: false,
					Disabled:        false,
				},
				Priority: 100,
				Disabled: false,
			},
			{
				Credential: core.Credential{
					AuthIndex:       "depleted-cred",
					Priority:        -1,
					PriorityMissing: false,
					Disabled:        false,
				},
				Priority: -1,
				Disabled: false,
			},
		},
		Changes: []priority.Change{
			{
				Credential: core.Credential{
					AuthIndex:       "unset-cred",
					Priority:        0,
					PriorityMissing: true,
				},
				Priority: 100,
			},
		},
	}

	snapshot := apply.Snapshot(plan)
	if len(snapshot.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(snapshot.Items))
	}

	// 1. Unset credential: Priority 0 and PriorityMissing true
	if !snapshot.Items[0].Current.PriorityMissing {
		t.Errorf("expected Items[0].Current.PriorityMissing=true, got false")
	}
	if snapshot.Items[0].Current.Priority != 0 {
		t.Errorf("expected Items[0].Current.Priority=0, got %d", snapshot.Items[0].Current.Priority)
	}

	// 2. Explicit zero credential: Priority 0 and PriorityMissing false
	if snapshot.Items[1].Current.PriorityMissing {
		t.Errorf("expected Items[1].Current.PriorityMissing=false, got true")
	}
	if snapshot.Items[1].Current.Priority != 0 {
		t.Errorf("expected Items[1].Current.Priority=0, got %d", snapshot.Items[1].Current.Priority)
	}

	// 3. Depleted credential: Priority -1 and PriorityMissing false
	if snapshot.Items[2].Current.PriorityMissing {
		t.Errorf("expected Items[2].Current.PriorityMissing=false, got true")
	}
	if snapshot.Items[2].Current.Priority != -1 {
		t.Errorf("expected Items[2].Current.Priority=-1, got %d", snapshot.Items[2].Current.Priority)
	}

	// 4. Changes check
	if !snapshot.Changes[0].Current.PriorityMissing {
		t.Errorf("expected Changes[0].Current.PriorityMissing=true, got false")
	}
}
