package host_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"antigravity-priority/internal/host"
)

func TestDocumentPatcher_ReplaceIsAtomicAndPreservesMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credential.json")
	if err := os.WriteFile(path, []byte(`{"priority":10}`), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	patcher := host.NewDocumentPatcher()
	if err := patcher.Replace(context.Background(), path, []byte(`{"priority":90,"disabled":true}`)); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode().Perm() != before.Mode().Perm() {
		t.Fatalf("replacement changed file mode: before=%o after=%o", before.Mode().Perm(), after.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"priority":90,"disabled":true}` {
		t.Fatalf("unexpected replacement: %s", data)
	}
}

func TestDocumentPatcher_ReplaceRejectsInvalidContextAndPath(t *testing.T) {
	patcher := host.NewDocumentPatcher()
	if err := patcher.Replace(context.Background(), "", []byte(`{}`)); err == nil {
		t.Fatal("expected empty path to fail")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := patcher.Replace(ctx, filepath.Join(t.TempDir(), "missing.json"), []byte(`{}`)); err == nil {
		t.Fatal("expected canceled replacement to fail")
	}
}
