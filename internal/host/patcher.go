package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DocumentPatcher is the low-level complete-document replacement primitive.
// Field-specific patch methods intentionally do not exist: the Apply-layer
// Host Transition owns expected-state checks and constructs the full document
// before calling this primitive.
type DocumentPatcher struct{}

// NewDocumentPatcher creates a complete-document replacement primitive.
func NewDocumentPatcher() *DocumentPatcher {
	return &DocumentPatcher{}
}

// Replace atomically replaces the physical credential document at path.
func (p *DocumentPatcher) Replace(ctx context.Context, path string, data []byte) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%w: auth document path is required", ErrInvalidRequest)
	}
	return writeFileAtomic(ctx, trimmedPath, data)
}

func writeFileAtomic(ctx context.Context, path string, data []byte) (err error) {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write auth document context: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat auth document: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write auth document context: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create auth document temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := ctx.Err(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write auth document context: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write auth document temp: %w", err)
	}
	if err := ctx.Err(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write auth document context: %w", err)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod auth document temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close auth document temp: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("write auth document context: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace auth document: %w", err)
	}
	cleanup = false
	return nil
}
