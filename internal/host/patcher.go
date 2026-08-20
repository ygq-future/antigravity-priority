package host

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DocumentPatcher performs atomic in-place AST patching on physical auth JSON documents.
type DocumentPatcher struct{}

// NewDocumentPatcher creates a new DocumentPatcher instance.
func NewDocumentPatcher() *DocumentPatcher {
	return &DocumentPatcher{}
}

// PatchPriority updates the priority field in the physical credential document at path.
func (p *DocumentPatcher) PatchPriority(ctx context.Context, path string, priority int) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%w: auth document path is required", ErrInvalidRequest)
	}
	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return fmt.Errorf("read auth document: %w", err)
	}
	patched, err := patchPriorityBytes(raw, priority)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(ctx, trimmedPath, patched); err != nil {
		return err
	}
	return ctx.Err()
}

// ResetPriority removes the priority field from the physical credential document at path.
func (p *DocumentPatcher) ResetPriority(ctx context.Context, path string) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%w: auth document path is required", ErrInvalidRequest)
	}
	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return fmt.Errorf("read auth document: %w", err)
	}
	patched, err := deleteTopLevelFieldBytes(raw, "priority")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(ctx, trimmedPath, patched); err != nil {
		return err
	}
	return ctx.Err()
}

// PatchDisabled updates the disabled field in the physical credential document at path.
func (p *DocumentPatcher) PatchDisabled(ctx context.Context, path string, disabled bool) error {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return fmt.Errorf("%w: auth document path is required", ErrInvalidRequest)
	}
	raw, err := os.ReadFile(trimmedPath)
	if err != nil {
		return fmt.Errorf("read auth document: %w", err)
	}
	patched, err := patchDisabledBytes(raw, disabled)
	if err != nil {
		return err
	}
	if err := writeFileAtomic(ctx, trimmedPath, patched); err != nil {
		return err
	}
	return ctx.Err()
}

func patchPriorityBytes(raw []byte, priority int) ([]byte, error) {
	encodedPriority, err := json.Marshal(priority)
	if err != nil {
		return nil, fmt.Errorf("encode priority: %w", err)
	}
	return patchTopLevelFieldBytes(raw, "priority", encodedPriority)
}

func deleteTopLevelFieldBytes(raw []byte, field string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("decode auth document for delete: %w", err)
	}
	if _, ok := m[field]; !ok {
		return raw, nil
	}
	delete(m, field)
	return json.MarshalIndent(m, "", "  ")
}

func patchDisabledBytes(raw []byte, disabled bool) ([]byte, error) {
	encodedDisabled, err := json.Marshal(disabled)
	if err != nil {
		return nil, fmt.Errorf("encode disabled: %w", err)
	}
	return patchTopLevelFieldBytes(raw, "disabled", encodedDisabled)
}

func patchTopLevelFieldBytes(raw []byte, field string, encodedValue []byte) ([]byte, error) {
	rangeStart, rangeEnd, found, err := topLevelFieldValueRange(raw, field)
	if err != nil {
		return nil, err
	}
	if found {
		patched := make([]byte, 0, len(raw)-rangeEnd+rangeStart+len(encodedValue))
		patched = append(patched, raw[:rangeStart]...)
		patched = append(patched, encodedValue...)
		patched = append(patched, raw[rangeEnd:]...)
		return patched, nil
	}
	return appendTopLevelField(raw, field, encodedValue)
}

func topLevelFieldValueRange(raw []byte, field string) (int, int, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return 0, 0, false, fmt.Errorf("decode auth document: %w", err)
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return 0, 0, false, fmt.Errorf("%w: auth document must be a JSON object", ErrInvalidRequest)
	}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, 0, false, fmt.Errorf("decode auth document key: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, 0, false, fmt.Errorf("%w: auth document key must be a string", ErrInvalidRequest)
		}
		valueScanStart := int(decoder.InputOffset())
		var value json.RawMessage
		if valErr := decoder.Decode(&value); valErr != nil {
			return 0, 0, false, fmt.Errorf("decode auth document value: %w", valErr)
		}
		valueEnd := int(decoder.InputOffset())
		if key != field {
			continue
		}
		valueStart, err := jsonValueStart(raw, valueScanStart, valueEnd)
		if err != nil {
			return 0, 0, false, err
		}
		return valueStart, valueEnd, true, nil
	}
	if _, err := decoder.Token(); err != nil {
		return 0, 0, false, fmt.Errorf("decode auth document close: %w", err)
	}
	return 0, 0, false, nil
}

func jsonValueStart(raw []byte, start int, end int) (int, error) {
	for index := start; index < end; index++ {
		switch raw[index] {
		case ' ', '\n', '\r', '\t':
			continue
		case ':':
			for valueStart := index + 1; valueStart < end; valueStart++ {
				switch raw[valueStart] {
				case ' ', '\n', '\r', '\t':
					continue
				default:
					return valueStart, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("%w: field value offset not found", ErrInvalidRequest)
}

func appendTopLevelField(raw []byte, field string, value []byte) ([]byte, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: auth document contains invalid JSON", ErrInvalidRequest)
	}
	insertAt := len(raw)
	for insertAt > 0 {
		insertAt--
		switch raw[insertAt] {
		case ' ', '\n', '\r', '\t':
			continue
		case '}':
			fieldBytes, err := json.Marshal(field)
			if err != nil {
				return nil, fmt.Errorf("encode field: %w", err)
			}
			prefix := []byte{','}
			if objectIsEmpty(raw[:insertAt]) {
				prefix = nil
			}
			patched := make([]byte, 0, len(raw)+len(prefix)+len(fieldBytes)+1+len(value))
			patched = append(patched, raw[:insertAt]...)
			patched = append(patched, prefix...)
			patched = append(patched, fieldBytes...)
			patched = append(patched, ':')
			patched = append(patched, value...)
			patched = append(patched, raw[insertAt:]...)
			return patched, nil
		default:
			return nil, fmt.Errorf("%w: auth document must end with object close", ErrInvalidRequest)
		}
	}
	return nil, fmt.Errorf("%w: auth document object close not found", ErrInvalidRequest)
}

func objectIsEmpty(rawBeforeClose []byte) bool {
	for index := len(rawBeforeClose) - 1; index >= 0; index-- {
		switch rawBeforeClose[index] {
		case ' ', '\n', '\r', '\t':
			continue
		case '{':
			return true
		default:
			return false
		}
	}
	return false
}

func writeFileAtomic(ctx context.Context, path string, data []byte) error {
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
