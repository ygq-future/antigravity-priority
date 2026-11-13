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
				Priority:  90,
				Disabled:  false,
				ResetAt:   &resetAt,
				Remaining: &rem,
				Reason:    "reason containing token=supersecret",
			},
			{
				Credential: core.Credential{
					Name:      "stale-cred",
					AuthIndex: "idx-stale-456",
					Priority:  10,
					Disabled:  false,
				},
				Priority: 10,
				Disabled: false,
				Reason:   "keep current state",
			},
		},
	}

	// 1. Snapshot test
	snapshot := apply.Snapshot(plan)
	if snapshot.TotalItems != 2 {
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
	transitionResult, err := apply.NewHostTransition(transitionStore).Execute(context.Background(), apply.TransitionRound{
		Intents: []apply.TransitionIntent{
			apply.ResetIntent(core.Credential{Name: "secret-acc", AuthIndex: "idx-auth-123", Priority: 20}, "reset token=supersecret"),
		},
	})
	if err != nil {
		t.Fatalf("execute transition error: %v", err)
	}
	result := apply.ResultFromTransition(transitionResult)
	event := result.Event
	if event.Action != "host.transition" {
		t.Errorf("expected action 'host.transition', got %s", event.Action)
	}
	if event.TotalChanges != 1 || len(event.Changes) != 1 {
		t.Errorf("expected one transitioned change, got %#v", event)
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

func TestRedactedErrors(t *testing.T) {
	// Test error redaction helper
	res := apply.RedactedError(errors.New("authorization: Bearer super-secret-key-123"))

	if strings.Contains(res, "super-secret-key-123") {
		t.Errorf("error contains secret key: %s", res)
	}
	if !strings.Contains(res, host.RedactedValue) {
		t.Errorf("error expected to contain REDACTED: %s", res)
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

func TestSnapshot_ExposesCooldownDeadline(t *testing.T) {
	until := time.Date(2026, 11, 12, 6, 30, 0, 0, time.UTC)
	plan := priority.Plan{
		Items: []priority.PlanItem{
			{
				Credential:    core.Credential{AuthIndex: "idx-cool", Provider: core.ProviderAntigravity},
				Priority:      -1,
				Reason:        "health probe HTTP 404",
				CooldownUntil: &until,
			},
			{
				Credential: core.Credential{AuthIndex: "idx-active", Provider: core.ProviderAntigravity},
				Priority:   100,
				Reason:     "fresh remaining positive",
			},
		},
	}

	snapshot := apply.Snapshot(plan)
	if snapshot.Items[0].CooldownUntil == nil || !snapshot.Items[0].CooldownUntil.Equal(until) {
		t.Fatalf("snapshot cooldown deadline = %v; want %v", snapshot.Items[0].CooldownUntil, until)
	}
	if snapshot.Items[1].CooldownUntil != nil {
		t.Fatalf("active snapshot item must not carry a cooldown deadline: %v", snapshot.Items[1].CooldownUntil)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if !strings.Contains(string(encoded), `"cooldown_until":"2026-11-12T06:30:00Z"`) {
		t.Fatalf("snapshot JSON is missing the cooldown deadline: %s", encoded)
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
	}

	snapshot := apply.Snapshot(plan)
	if len(snapshot.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(snapshot.Items))
	}

	// 1. Unset credential: Priority 0 and PriorityMissing true
	if !snapshot.Items[0].Host.PriorityMissing {
		t.Errorf("expected Items[0].Host.PriorityMissing=true, got false")
	}
	if snapshot.Items[0].Host.Priority != 0 {
		t.Errorf("expected Items[0].Host.Priority=0, got %d", snapshot.Items[0].Host.Priority)
	}

	// 2. Explicit zero credential: Priority 0 and PriorityMissing false
	if snapshot.Items[1].Host.PriorityMissing {
		t.Errorf("expected Items[1].Host.PriorityMissing=false, got true")
	}
	if snapshot.Items[1].Host.Priority != 0 {
		t.Errorf("expected Items[1].Host.Priority=0, got %d", snapshot.Items[1].Host.Priority)
	}

	// 3. Depleted credential: Priority -1 and PriorityMissing false
	if snapshot.Items[2].Host.PriorityMissing {
		t.Errorf("expected Items[2].Host.PriorityMissing=false, got true")
	}
	if snapshot.Items[2].Host.Priority != -1 {
		t.Errorf("expected Items[2].Host.Priority=-1, got %d", snapshot.Items[2].Host.Priority)
	}
}

func TestSnapshotSet_And_PlanSnapshot_ManagementView(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	orig := apply.SnapshotSet{
		ObservedAt: now,
		Models: []apply.ModelOption{{
			ID:          "gemini-3-flash",
			DisplayName: "Bearer secret_token_display",
			QuotaPool:   "gemini",
		}},
		Groups: map[string]apply.PoolSnapshot{
			"gemini": {Items: []apply.SnapshotItem{{
				Identity:        apply.SnapshotIdentity{Email: "owner@example.com", AuthIndex: "auth_123"},
				Name:            "account",
				AuthIndex:       "au***23",
				Email:           "ow***com",
				Reason:          "token=dummy_val_123",
				SupportedModels: []string{"gemini-3-flash"},
			}}},
		},
	}

	view := orig.ManagementView()
	gotItem := view.Groups["gemini"].Items[0]
	if gotItem.Email != "owner@example.com" || gotItem.AuthIndex != "auth_123" {
		t.Fatalf("ManagementView identity = (%q, %q), want full identity", gotItem.Email, gotItem.AuthIndex)
	}
	if strings.Contains(gotItem.Reason, "dummy_val_123") || strings.Contains(view.Models[0].DisplayName, "secret_token_display") {
		t.Fatalf("ManagementView did not redact sensitive substrings: reason=%q display=%q", gotItem.Reason, view.Models[0].DisplayName)
	}
	// Original audit snapshot must remain untouched and redacted.
	if orig.Groups["gemini"].Items[0].Email != "ow***com" || orig.Groups["gemini"].Items[0].AuthIndex != "au***23" {
		t.Fatalf("ManagementView mutated original snapshot: %+v", orig.Groups["gemini"].Items[0])
	}

	rawJSON, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal management view: %v", err)
	}
	if strings.Contains(string(rawJSON), `"Identity"`) || strings.Contains(string(rawJSON), "dummy_val_123") {
		t.Fatalf("unexpected leakage in management view JSON: %s", rawJSON)
	}

	plan := apply.PlanSnapshot{
		DecidedAt:  now,
		TotalItems: 1,
		Items: []apply.SnapshotItem{{
			Identity:  apply.SnapshotIdentity{Email: "owner@example.com", AuthIndex: "auth_123"},
			AuthIndex: "au***23",
			Email:     "ow***com",
			Reason:    "Authorization: Bearer abc123",
		}},
	}
	planView := plan.ManagementView(func(item apply.SnapshotItem) string {
		return item.Identity.Email
	})
	if planView.Items[0].Email != "owner@example.com" || planView.Items[0].AuthIndex != "au***23" {
		t.Fatalf("PlanSnapshot.ManagementView = %+v, want full email and preserved auth_index", planView.Items[0])
	}
	if strings.Contains(planView.Items[0].Reason, "abc123") {
		t.Fatalf("PlanSnapshot.ManagementView did not redact reason: %q", planView.Items[0].Reason)
	}
}
