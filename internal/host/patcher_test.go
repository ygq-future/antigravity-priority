package host_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"antigravity-priority/internal/host"
)

func TestDocumentPatcher_DirectPatching(t *testing.T) {
	ctx := context.Background()
	patcher := host.NewDocumentPatcher()

	t.Run("PatchPriority and ResetPriority", func(t *testing.T) {
		dir := t.TempDir()
		authPath := filepath.Join(dir, "auth_direct.json")
		initial := []byte(`{"name":"test","priority":10}`)
		if err := os.WriteFile(authPath, initial, 0o600); err != nil {
			t.Fatal(err)
		}

		// Update priority
		if err := patcher.PatchPriority(ctx, authPath, 999); err != nil {
			t.Fatalf("unexpected PatchPriority error: %v", err)
		}
		raw, err := os.ReadFile(authPath)
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		if parsed["priority"] != float64(999) {
			t.Errorf("expected priority 999, got %v", parsed["priority"])
		}

		// Reset priority
		if err := patcher.ResetPriority(ctx, authPath); err != nil {
			t.Fatalf("unexpected ResetPriority error: %v", err)
		}
		raw, _ = os.ReadFile(authPath)
		parsed = nil
		_ = json.Unmarshal(raw, &parsed)
		if _, ok := parsed["priority"]; ok {
			t.Errorf("expected priority to be removed, but present: %v", parsed["priority"])
		}
	})

	t.Run("PatchDisabled", func(t *testing.T) {
		dir := t.TempDir()
		authPath := filepath.Join(dir, "auth_dis.json")
		initial := []byte(`{"name":"test","disabled":false}`)
		if err := os.WriteFile(authPath, initial, 0o600); err != nil {
			t.Fatal(err)
		}

		if err := patcher.PatchDisabled(ctx, authPath, true); err != nil {
			t.Fatalf("unexpected PatchDisabled error: %v", err)
		}
		raw, _ := os.ReadFile(authPath)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		if parsed["disabled"] != true {
			t.Errorf("expected disabled true, got %v", parsed["disabled"])
		}
	})

	t.Run("Empty path produces error", func(t *testing.T) {
		if err := patcher.PatchPriority(ctx, "", 100); err == nil {
			t.Errorf("expected error for empty path in PatchPriority")
		}
		if err := patcher.ResetPriority(ctx, ""); err == nil {
			t.Errorf("expected error for empty path in ResetPriority")
		}
		if err := patcher.PatchDisabled(ctx, "", true); err == nil {
			t.Errorf("expected error for empty path in PatchDisabled")
		}
	})

	t.Run("Nonexistent file produces error", func(t *testing.T) {
		if err := patcher.PatchPriority(ctx, filepath.Join(t.TempDir(), "none.json"), 100); err == nil {
			t.Errorf("expected error for nonexistent file in PatchPriority")
		}
		if err := patcher.ResetPriority(ctx, filepath.Join(t.TempDir(), "none.json")); err == nil {
			t.Errorf("expected error for nonexistent file in ResetPriority")
		}
		if err := patcher.PatchDisabled(ctx, filepath.Join(t.TempDir(), "none.json"), true); err == nil {
			t.Errorf("expected error for nonexistent file in PatchDisabled")
		}
	})
}
