package runtime

import (
	"testing"

	"antigravity-priority/internal/apply"
)

func TestManagementRunHistoryOverlaysFullEmailAtAdapterBoundary(t *testing.T) {
	r := &Runtime{
		runHistory: []RunHistoryEntry{{
			Kind: "apply",
			Snapshot: &apply.PlanSnapshot{Items: []apply.SnapshotItem{{
				Email: "us***om",
			}}},
		}},
		latestDualSnapshot: &apply.DualGroupSnapshot{Groups: map[string]apply.GroupSnapshot{
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

func TestOverlayManagementCooldownEmailsUsesFullEmail(t *testing.T) {
	cooldowns := []map[string]any{{"auth_index": "au***01"}}
	overlayManagementCooldownEmails(cooldowns, map[string]string{
		"auth-account-001": "user@example.com",
	})
	if cooldowns[0]["email"] != "user@example.com" {
		t.Fatalf("cooldown email=%v, want full CPA Host email", cooldowns[0]["email"])
	}
}

func TestManagementRunHistoryDoesNotExposeMaskedIdentityFallback(t *testing.T) {
	r := &Runtime{runHistory: []RunHistoryEntry{{
		Kind: "apply",
		Snapshot: &apply.PlanSnapshot{Items: []apply.SnapshotItem{{
			Email: "us***om",
		}}},
	}}}

	history := r.managementRunHistory()
	snapshot, _ := history[0]["snapshot"].(map[string]any)
	items, _ := snapshot["items"].([]any)
	item, _ := items[0].(map[string]any)
	if item["email"] != "" {
		t.Fatalf("unresolved management email=%v, want empty display fallback", item["email"])
	}
}
