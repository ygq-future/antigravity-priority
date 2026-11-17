package promptguard_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/promptguard"
)

func TestDiffSyncRules_EmptyInitial(t *testing.T) {
	remote := []promptguard.RemotePhrase{
		{Find: "phrase 1", Replace: "safe 1", Mode: "text"},
		{Find: "phrase 2", Replace: "safe 2", Mode: "regex"},
	}

	res, summary := promptguard.DiffSyncRules(nil, remote)
	if len(res) != 2 {
		t.Fatalf("expected 2 synced rules, got %d", len(res))
	}
	if summary.Added != 2 || summary.Deleted != 0 || summary.Updated != 0 || summary.PreservedCustom != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if res[0].Find != "phrase 1" || res[0].Custom != false || !res[0].On {
		t.Fatalf("unexpected rule 0: %+v", res[0])
	}
	if res[1].Find != "phrase 2" || res[1].Mode != "regex" {
		t.Fatalf("unexpected rule 1: %+v", res[1])
	}
}

func TestDiffSyncRules_PreservesCustomRules(t *testing.T) {
	current := []config.PromptGuardRule{
		{
			On:      true,
			Find:    "my custom secret phrase",
			Replace: "clean phrase",
			Scope:   config.PromptGuardScopeSystem,
			Mode:    config.PromptGuardModeText,
			Custom:  true,
		},
		{
			On:      false,
			Find:    "another custom rule",
			Replace: "safe",
			Scope:   config.PromptGuardScopeAll,
			Mode:    config.PromptGuardModeRegex,
			Custom:  true,
		},
	}

	remote := []promptguard.RemotePhrase{
		{Find: "remote phrase 1", Replace: "remote safe 1", Mode: "text"},
	}

	res, summary := promptguard.DiffSyncRules(current, remote)
	if len(res) != 3 {
		t.Fatalf("expected 3 rules (2 custom + 1 remote), got %d", len(res))
	}
	if summary.PreservedCustom != 2 || summary.Added != 1 || summary.Deleted != 0 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	// Custom rules stay at the front with their flags untouched
	if !res[0].Custom || res[0].Find != "my custom secret phrase" {
		t.Fatalf("expected custom rule 0 preserved, got %+v", res[0])
	}
	if !res[1].Custom || res[1].Find != "another custom rule" || res[1].On {
		t.Fatalf("expected custom rule 1 preserved with On=false, got %+v", res[1])
	}
	if res[2].Custom || res[2].Find != "remote phrase 1" {
		t.Fatalf("expected remote rule appended, got %+v", res[2])
	}
}

func TestDiffSyncRules_DeletesRemovedRemoteRules(t *testing.T) {
	current := []config.PromptGuardRule{
		{
			On:      true,
			Find:    "stale remote phrase",
			Replace: "old replacement",
			Custom:  false,
		},
		{
			On:      true,
			Find:    "user custom",
			Replace: "custom safe",
			Custom:  true,
		},
		{
			On:      true,
			Find:    "active remote phrase",
			Replace: "active replacement",
			Custom:  false,
		},
	}

	// Remote no longer has "stale remote phrase", but has "active remote phrase" and "brand new phrase"
	remote := []promptguard.RemotePhrase{
		{Find: "active remote phrase", Replace: "active replacement", Mode: "text"},
		{Find: "brand new phrase", Replace: "new replacement", Mode: "text"},
	}

	res, summary := promptguard.DiffSyncRules(current, remote)
	if len(res) != 3 {
		t.Fatalf("expected 3 rules (1 custom + 2 remote), got %d", len(res))
	}
	if summary.Deleted != 1 {
		t.Fatalf("expected 1 deleted rule, got %d", summary.Deleted)
	}
	if summary.Added != 1 {
		t.Fatalf("expected 1 added rule, got %d", summary.Added)
	}
	if summary.PreservedCustom != 1 {
		t.Fatalf("expected 1 custom preserved, got %d", summary.PreservedCustom)
	}

	// Verify "stale remote phrase" is completely gone
	for _, r := range res {
		if r.Find == "stale remote phrase" {
			t.Fatal("expected stale remote phrase to be deleted")
		}
	}
}

func TestDiffSyncRules_UpdatesRemoteAndPreservesOnToggle(t *testing.T) {
	current := []config.PromptGuardRule{
		{
			On:      false, // User explicitly turned this remote rule off
			Find:    "remote phrase",
			Replace: "old safe",
			Mode:    config.PromptGuardModeText,
			Custom:  false,
		},
	}

	remote := []promptguard.RemotePhrase{
		{Find: "remote phrase", Replace: "better safe", Mode: "text"},
	}

	res, summary := promptguard.DiffSyncRules(current, remote)
	if len(res) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(res))
	}
	if summary.Updated != 1 {
		t.Fatalf("expected 1 updated rule, got %d", summary.Updated)
	}
	// User's On=false must be preserved
	if res[0].On {
		t.Fatalf("expected user's On=false toggle preserved, got %+v", res[0])
	}
	// Replace text must be updated from remote
	if res[0].Replace != "better safe" {
		t.Fatalf("expected replace updated to 'better safe', got %q", res[0].Replace)
	}
}

func TestDiffSyncRules_CustomRuleOverridesRemoteDuplicate(t *testing.T) {
	current := []config.PromptGuardRule{
		{
			On:      true,
			Find:    "shared phrase",
			Replace: "user preferred replacement",
			Custom:  true,
		},
	}

	remote := []promptguard.RemotePhrase{
		{Find: "shared phrase", Replace: "remote default replacement", Mode: "text"},
		{Find: "other phrase", Replace: "safe", Mode: "text"},
	}

	res, summary := promptguard.DiffSyncRules(current, remote)
	if len(res) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(res))
	}
	if res[0].Find != "shared phrase" || res[0].Replace != "user preferred replacement" || !res[0].Custom {
		t.Fatalf("expected custom rule to take precedence, got %+v", res[0])
	}
	if res[1].Find != "other phrase" || res[1].Custom {
		t.Fatalf("expected other remote rule appended, got %+v", res[1])
	}
	if summary.PreservedCustom != 1 || summary.Added != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestFetchRemotePhrases(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/404" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/invalid" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{not valid json`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": 1,
			"updated": "2026-11-17",
			"phrases": [
				{"find": "test find", "replace": "test replace", "mode": "text"}
			]
		}`))
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Success
	file, err := promptguard.FetchRemotePhrases(ctx, ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if file.Version != 1 || len(file.Phrases) != 1 || file.Phrases[0].Find != "test find" {
		t.Fatalf("unexpected fetched file: %+v", file)
	}

	// 2. HTTP error
	_, err = promptguard.FetchRemotePhrases(ctx, ts.Client(), ts.URL+"/404")
	if err == nil {
		t.Fatal("expected error on 404")
	}

	// 3. JSON decode error
	_, err = promptguard.FetchRemotePhrases(ctx, ts.Client(), ts.URL+"/invalid")
	if err == nil {
		t.Fatal("expected error on invalid JSON")
	}
}
