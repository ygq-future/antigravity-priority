package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/state"
)

func TestManagementRunHistoryOverlaysFullEmailAtAdapterBoundary(t *testing.T) {
	r := &Runtime{
		state: &stateRepository{
			runHistory: []RunHistoryEntry{{
				Kind: "apply",
				Snapshot: &apply.PlanSnapshot{Items: []apply.SnapshotItem{{
					Email: "user***",
				}}},
			}},
		},
		latestSnapshotSet: &apply.SnapshotSet{Groups: map[string]apply.PoolSnapshot{
			"gemini": {Items: []apply.SnapshotItem{{
				Identity: apply.SnapshotIdentity{
					AuthIndex: "auth-account-001",
					Email:     "user@example.com",
				},
			}}},
		}},
	}

	history := r.managementRunHistory()
	if len(history) != 1 || history[0].Snapshot == nil || len(history[0].Snapshot.Items) != 1 {
		t.Fatalf("unexpected management history: %#v", history)
	}
	if got := history[0].Snapshot.Items[0].Email; got != "user@example.com" {
		t.Fatalf("management email=%q, want full CPA Host email", got)
	}
	encoded, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal management history: %v", err)
	}
	if !strings.Contains(string(encoded), "user@example.com") {
		t.Fatalf("expected encoded management history to include full email, got %s", encoded)
	}
	// Persisted repository copy must remain redacted.
	if got := r.state.RunHistory()[0].Snapshot.Items[0].Email; got != "user***" {
		t.Fatalf("persisted run history email mutated to %q, want redacted", got)
	}
}

func TestOverlayManagementRequestHistoryEmailsUsesFullEmail(t *testing.T) {
	entries := []state.RequestHistoryEntry{
		{AuthIndex: "auth-account-001", Email: "auth***"},
		{AuthIndex: "auth-account-002", Email: "known***"},
	}
	overlayManagementRequestHistoryEmails(entries, map[string]string{
		"auth-account-001": "user@example.com",
	})

	if entries[0].Email != "user@example.com" {
		t.Fatalf("request history email=%v, want full CPA Host email", entries[0].Email)
	}
	if entries[1].Email != "known***" {
		t.Fatalf("unresolved request history email=%v, want persisted masked fallback", entries[1].Email)
	}
}

func TestRateLimitDiagnosticsUsesAccountEmail(t *testing.T) {
	r := &Runtime{pending429: map[string]pending429{
		"key": {AuthIndex: "auth-account-001", ModelGroup: "gemini", Model: "gemini-3-flash", Count: 2},
	}}

	pending, _ := r.rateLimitDiagnostics(map[string]string{
		"auth-account-001": "user@example.com",
	})

	if len(pending) != 1 {
		t.Fatalf("pending count=%d, want 1", len(pending))
	}
	if pending[0]["email"] != "user@example.com" {
		t.Fatalf("pending email=%v, want full CPA Host email", pending[0]["email"])
	}
}

func TestManagementRunHistoryDoesNotExposeMaskedIdentityFallback(t *testing.T) {
	r := &Runtime{state: &stateRepository{runHistory: []RunHistoryEntry{{
		Kind: "apply",
		Snapshot: &apply.PlanSnapshot{Items: []apply.SnapshotItem{{
			Email: "user***",
		}}},
	}}}}

	history := r.managementRunHistory()
	if len(history) != 1 || history[0].Snapshot == nil || len(history[0].Snapshot.Items) != 1 {
		t.Fatalf("unexpected management history: %#v", history)
	}
	if got := history[0].Snapshot.Items[0].Email; got != "" {
		t.Fatalf("unresolved management email=%q, want empty display fallback", got)
	}
	encoded, err := json.Marshal(history)
	if err != nil {
		t.Fatalf("marshal management history: %v", err)
	}
	if strings.Contains(string(encoded), "user***") {
		t.Fatalf("unresolved management history leaked masked fallback: %s", encoded)
	}
}
