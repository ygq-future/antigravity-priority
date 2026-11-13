package runtime

import (
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
	if len(history) != 1 {
		t.Fatalf("management history length=%d, want 1", len(history))
	}
	snapshot, _ := history[0]["snapshot"].(map[string]any)
	items, _ := snapshot["items"].([]any)
	item, _ := items[0].(map[string]any)
	if item["email"] != "user@example.com" {
		t.Fatalf("management email=%v, want full CPA Host email", item["email"])
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
	snapshot, _ := history[0]["snapshot"].(map[string]any)
	items, _ := snapshot["items"].([]any)
	item, _ := items[0].(map[string]any)
	if item["email"] != "" {
		t.Fatalf("unresolved management email=%v, want empty display fallback", item["email"])
	}
}
