package host_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"antigravity-priority/internal/host"
)

type mockHostCallbacks struct {
	listAuthFilesFunc func(ctx context.Context) ([]host.AuthFile, error)
	getAuthFunc       func(ctx context.Context, authIndex string) (host.AuthDocument, error)
	getRuntimeFunc    func(ctx context.Context, authIndex string) (host.RuntimeAuth, error)
	saveAuthFunc      func(ctx context.Context, name string, doc json.RawMessage) error
	httpDoFunc        func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error)
}

func (m *mockHostCallbacks) ListAuthFiles(ctx context.Context) ([]host.AuthFile, error) {
	if m.listAuthFilesFunc != nil {
		return m.listAuthFilesFunc(ctx)
	}
	return nil, nil
}

func (m *mockHostCallbacks) GetAuth(ctx context.Context, authIndex string) (host.AuthDocument, error) {
	if m.getAuthFunc != nil {
		return m.getAuthFunc(ctx, authIndex)
	}
	return host.AuthDocument{}, nil
}

func (m *mockHostCallbacks) GetRuntime(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
	if m.getRuntimeFunc != nil {
		return m.getRuntimeFunc(ctx, authIndex)
	}
	return host.RuntimeAuth{}, nil
}

func (m *mockHostCallbacks) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	if m.saveAuthFunc != nil {
		return m.saveAuthFunc(ctx, name, doc)
	}
	return nil
}

func (m *mockHostCallbacks) HTTPDo(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
	if m.httpDoFunc != nil {
		return m.httpDoFunc(ctx, req)
	}
	return host.HTTPResponse{}, nil
}

func TestClient_ListAuthFiles(t *testing.T) {
	ctx := context.Background()
	expected := []host.AuthFile{
		{Name: "c1", AuthIndex: "idx-1"},
	}
	callbacks := &mockHostCallbacks{
		listAuthFilesFunc: func(ctx context.Context) ([]host.AuthFile, error) {
			return expected, nil
		},
	}
	client := host.NewClient(callbacks)
	got, err := client.ListAuthFiles(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].AuthIndex != "idx-1" {
		t.Fatalf("unexpected result: %+v", got)
	}

	callbacks.listAuthFilesFunc = func(ctx context.Context) ([]host.AuthFile, error) {
		return nil, errors.New("list failure")
	}
	if _, err := client.ListAuthFiles(ctx); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_GetAuth_And_GetRuntime(t *testing.T) {
	ctx := context.Background()
	callbacks := &mockHostCallbacks{
		getAuthFunc: func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
			if authIndex == "idx-1" {
				return host.AuthDocument{AuthIndex: "idx-1", Name: "c1"}, nil
			}
			return host.AuthDocument{}, errors.New("not found")
		},
		getRuntimeFunc: func(ctx context.Context, authIndex string) (host.RuntimeAuth, error) {
			if authIndex == "idx-1" {
				return host.RuntimeAuth{AuthIndex: "idx-1", Name: "c1"}, nil
			}
			return host.RuntimeAuth{}, errors.New("not found")
		},
	}
	client := host.NewClient(callbacks)

	doc, err := client.GetAuth(ctx, "idx-1")
	if err != nil || doc.AuthIndex != "idx-1" {
		t.Fatalf("unexpected GetAuth: %v, doc: %+v", err, doc)
	}
	if _, err := client.GetAuth(ctx, "unknown"); err == nil {
		t.Fatal("expected GetAuth error, got nil")
	}

	rt, err := client.GetRuntime(ctx, "idx-1")
	if err != nil || rt.AuthIndex != "idx-1" {
		t.Fatalf("unexpected GetRuntime: %v, rt: %+v", err, rt)
	}
	if _, err := client.GetRuntime(ctx, "unknown"); err == nil {
		t.Fatal("expected GetRuntime error, got nil")
	}
}

func TestClient_SaveAuth(t *testing.T) {
	ctx := context.Background()
	var savedName string
	callbacks := &mockHostCallbacks{
		saveAuthFunc: func(ctx context.Context, name string, doc json.RawMessage) error {
			savedName = name
			if name == "fail" {
				return errors.New("save fail")
			}
			return nil
		},
	}
	client := host.NewClient(callbacks)

	if err := client.SaveAuth(ctx, "   ", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
	if err := client.SaveAuth(ctx, "c1", json.RawMessage(`{}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if savedName != "c1" {
		t.Fatalf("expected savedName c1, got %s", savedName)
	}
	if err := client.SaveAuth(ctx, "fail", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for fail, got nil")
	}
}

func TestClient_PatchPriority(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth1.json")
	initialJSON := []byte(`{
		"name": "c1",
		"auth_index": "idx-1",
		"priority": 10
	}`)
	if err := os.WriteFile(authPath, initialJSON, 0600); err != nil {
		t.Fatal(err)
	}

	callbacks := &mockHostCallbacks{
		getAuthFunc: func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
			if authIndex == "idx-1" {
				return host.AuthDocument{AuthIndex: "idx-1", Path: authPath}, nil
			}
			return host.AuthDocument{}, errors.New("auth not found")
		},
	}
	client := host.NewClient(callbacks)

	// Empty auth index
	if err := client.PatchPriority(ctx, "  ", 100); err == nil {
		t.Fatal("expected error for empty authIndex")
	}

	// Successful priority patch
	if err := client.PatchPriority(ctx, "idx-1", 99); err != nil {
		t.Fatalf("unexpected patch priority error: %v", err)
	}
	content, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatal(err)
	}
	if int(parsed["priority"].(float64)) != 99 {
		t.Fatalf("expected priority 99, got %v", parsed["priority"])
	}

	// Patch file without priority field (appending field)
	noPriPath := filepath.Join(dir, "auth2.json")
	if err := os.WriteFile(noPriPath, []byte(`{"name":"c2"}`), 0600); err != nil {
		t.Fatal(err)
	}
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: noPriPath}, nil
	}
	if err := client.PatchPriority(ctx, "idx-2", 42); err != nil {
		t.Fatalf("unexpected patch priority error: %v", err)
	}
	content, err = os.ReadFile(noPriPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatal(err)
	}
	if int(parsed["priority"].(float64)) != 42 {
		t.Fatalf("expected priority 42, got %v", parsed["priority"])
	}

	// Empty JSON object `{}`
	emptyObjPath := filepath.Join(dir, "auth3.json")
	if err := os.WriteFile(emptyObjPath, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: emptyObjPath}, nil
	}
	if err := client.PatchPriority(ctx, "idx-3", 7); err != nil {
		t.Fatalf("unexpected patch priority error on empty obj: %v", err)
	}
	content, _ = os.ReadFile(emptyObjPath)
	_ = json.Unmarshal(content, &parsed)
	if int(parsed["priority"].(float64)) != 7 {
		t.Fatalf("expected priority 7, got %v", parsed["priority"])
	}

	// Path is empty
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: ""}, nil
	}
	if err := client.PatchPriority(ctx, "idx-4", 10); err == nil {
		t.Fatal("expected error for empty document path")
	}

	// File does not exist
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: filepath.Join(dir, "nonexistent.json")}, nil
	}
	if err := client.PatchPriority(ctx, "idx-5", 10); err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	// Invalid JSON content
	invPath := filepath.Join(dir, "inv.json")
	_ = os.WriteFile(invPath, []byte(`not json`), 0600)
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: invPath}, nil
	}
	if err := client.PatchPriority(ctx, "idx-6", 10); err == nil {
		t.Fatal("expected error for invalid json file")
	}

	// Context cancellation
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: authPath}, nil
	}
	if err := client.PatchPriority(canceledCtx, "idx-1", 50); err == nil {
		t.Fatal("expected error with canceled context")
	}

	// HTTPStatusError with empty body
	emptyStatusErr := &host.HTTPStatusError{StatusCode: 500, Body: ""}
	if !strings.Contains(emptyStatusErr.Error(), "host http status 500") {
		t.Errorf("expected status 500 in error, got %s", emptyStatusErr.Error())
	}

	// GetAuth error in PatchPriority
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{}, errors.New("get auth failed")
	}
	if err := client.PatchPriority(ctx, "idx-err", 10); err == nil {
		t.Fatal("expected error when GetAuth fails in PatchPriority")
	}

	// PatchDisabled errors: empty doc path, nonexistent file
	callbacks.listAuthFilesFunc = func(ctx context.Context) ([]host.AuthFile, error) {
		return []host.AuthFile{{Name: "err-doc", AuthIndex: "idx-err-doc"}}, nil
	}
	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: ""}, nil
	}
	if err := client.PatchDisabled(ctx, "err-doc", true); err == nil {
		t.Fatal("expected error for empty document path in PatchDisabled")
	}

	callbacks.getAuthFunc = func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
		return host.AuthDocument{AuthIndex: authIndex, Path: filepath.Join(dir, "nonexistent.json")}, nil
	}
	if err := client.PatchDisabled(ctx, "err-doc", true); err == nil {
		t.Fatal("expected error for nonexistent file in PatchDisabled")
	}
}

func TestClient_PatchDisabled(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth_dis.json")
	if err := os.WriteFile(authPath, []byte(`{"name":"c1","auth_index":"idx-1","disabled":false}`), 0600); err != nil {
		t.Fatal(err)
	}

	callbacks := &mockHostCallbacks{
		listAuthFilesFunc: func(ctx context.Context) ([]host.AuthFile, error) {
			return []host.AuthFile{
				{Name: "c1", AuthIndex: "idx-1"},
			}, nil
		},
		getAuthFunc: func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
			if authIndex == "idx-1" {
				return host.AuthDocument{AuthIndex: "idx-1", Path: authPath}, nil
			}
			return host.AuthDocument{}, errors.New("not found")
		},
	}
	client := host.NewClient(callbacks)

	// Empty name
	if err := client.PatchDisabled(ctx, "  ", true); err == nil {
		t.Fatal("expected error for empty name")
	}

	// Successful patch by Name
	if err := client.PatchDisabled(ctx, "c1", true); err != nil {
		t.Fatalf("unexpected patch disabled error: %v", err)
	}
	content, _ := os.ReadFile(authPath)
	var parsed map[string]interface{}
	_ = json.Unmarshal(content, &parsed)
	if parsed["disabled"] != true {
		t.Fatalf("expected disabled true, got %v", parsed["disabled"])
	}

	// Successful patch by AuthIndex
	if err := client.PatchDisabled(ctx, "idx-1", false); err != nil {
		t.Fatalf("unexpected patch disabled error: %v", err)
	}
	content, _ = os.ReadFile(authPath)
	_ = json.Unmarshal(content, &parsed)
	if parsed["disabled"] != false {
		t.Fatalf("expected disabled false, got %v", parsed["disabled"])
	}

	// Not found in list
	if err := client.PatchDisabled(ctx, "unknown-auth", true); err == nil {
		t.Fatal("expected error for unknown auth name")
	}

	// ListAuthFiles fails
	callbacks.listAuthFilesFunc = func(ctx context.Context) ([]host.AuthFile, error) {
		return nil, errors.New("list failed")
	}
	if err := client.PatchDisabled(ctx, "c1", true); err == nil {
		t.Fatal("expected error when ListAuthFiles fails")
	}
}

func TestClient_ResetPriority(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth_reset.json")
	if err := os.WriteFile(authPath, []byte(`{"name":"c1","auth_index":"idx-1","priority":999}`), 0600); err != nil {
		t.Fatal(err)
	}

	callbacks := &mockHostCallbacks{
		getAuthFunc: func(ctx context.Context, authIndex string) (host.AuthDocument, error) {
			if authIndex == "idx-1" {
				return host.AuthDocument{AuthIndex: "idx-1", Path: authPath}, nil
			}
			return host.AuthDocument{}, errors.New("not found")
		},
	}
	client := host.NewClient(callbacks)

	if err := client.ResetPriority(ctx, "idx-1"); err != nil {
		t.Fatalf("unexpected reset priority error: %v", err)
	}

	content, err := os.ReadFile(authPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["priority"]; ok {
		t.Fatalf("expected priority to be removed, but still present: %v", parsed["priority"])
	}
}

func TestClient_HTTPDo(t *testing.T) {
	ctx := context.Background()
	callbacks := &mockHostCallbacks{
		httpDoFunc: func(ctx context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			if strings.HasPrefix(req.URL, "https://api.example.com/ok") {
				return host.HTTPResponse{
					StatusCode: 200,
					Body:       []byte(`{"status":"ok"}`),
				}, nil
			}
			if strings.HasPrefix(req.URL, "https://api.example.com/error") {
				return host.HTTPResponse{
					StatusCode: 429,
					Body:       []byte(`{"error":"rate limit exceeded","token":"secret123"}`),
				}, nil
			}
			return host.HTTPResponse{}, errors.New("network error")
		},
	}
	client := host.NewClient(callbacks)

	// Missing method or url
	if _, err := client.HTTPDo(ctx, host.HTTPRequest{Method: "", URL: "https://example.com"}); err == nil {
		t.Fatal("expected error for empty method")
	}
	if _, err := client.HTTPDo(ctx, host.HTTPRequest{Method: "GET", URL: ""}); err == nil {
		t.Fatal("expected error for empty url")
	}

	// 200 OK
	resp, err := client.HTTPDo(ctx, host.HTTPRequest{Method: "GET", URL: "https://api.example.com/ok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// 429 Error with redacted error message
	_, err = client.HTTPDo(ctx, host.HTTPRequest{Method: "POST", URL: "https://api.example.com/error?token=mysecret"})
	if err == nil {
		t.Fatal("expected error for 429, got nil")
	}
	if strings.Contains(err.Error(), "secret123") || strings.Contains(err.Error(), "mysecret") {
		t.Fatalf("error should not contain secret tokens, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), host.RedactedValue) {
		t.Fatalf("error should contain REDACTED, got: %s", err.Error())
	}

	// Network error
	_, err = client.HTTPDo(ctx, host.HTTPRequest{Method: "GET", URL: "https://api.example.com/fail"})
	if err == nil {
		t.Fatal("expected network error, got nil")
	}
}

func TestRedactBytes_And_RedactHTTPRequest(t *testing.T) {
	// Empty bytes
	if got := host.RedactBytes(nil); got != "" {
		t.Errorf("expected empty string for nil bytes, got %q", got)
	}

	// Redact empty headers
	if got := host.RedactHTTPRequest(host.HTTPRequest{}).Headers; got != nil {
		t.Errorf("expected nil headers for empty headers, got %v", got)
	}

	// JSON object with sensitive keys and nested structures
	jsonInput := []byte(`{
		"authorization": "Bearer secret-token-123",
		"api_key": "key-xyz",
		"token": "tok-abc",
		"cookie": "sess=123",
		"safe_field": "hello world",
		"nested": {
			"api-key": "sub-key",
			"normal": 123
		}
	}`)
	redactedJSON := host.RedactBytes(jsonInput)
	if strings.Contains(redactedJSON, "secret-token-123") || strings.Contains(redactedJSON, "key-xyz") || strings.Contains(redactedJSON, "tok-abc") {
		t.Errorf("sensitive JSON was not redacted: %s", redactedJSON)
	}
	if !strings.Contains(redactedJSON, `"safe_field":"hello world"`) && !strings.Contains(redactedJSON, `"safe_field": "hello world"`) {
		t.Errorf("safe field missing from redacted JSON: %s", redactedJSON)
	}

	// JSON array with nested sensitive keys
	jsonArrInput := []byte(`[
		{"token": "arr-secret-1"},
		{"name": "test", "apiKey": "arr-secret-2"}
	]`)
	redactedArr := host.RedactBytes(jsonArrInput)
	if strings.Contains(redactedArr, "arr-secret-1") || strings.Contains(redactedArr, "arr-secret-2") {
		t.Errorf("sensitive array was not redacted: %s", redactedArr)
	}

	// Plain text with bearer and tokens
	textInput := "Error: bearer secret-token-value occurred with token: abc-123 and authorization=def-456"
	redactedText := host.RedactBytes([]byte(textInput))
	if strings.Contains(redactedText, "secret-token-value") || strings.Contains(redactedText, "abc-123") || strings.Contains(redactedText, "def-456") {
		t.Errorf("sensitive text was not redacted: %s", redactedText)
	}

	// RedactHTTPRequest
	req := host.HTTPRequest{
		AuthIndex: "idx-1",
		Method:    "POST",
		URL:       "https://api.example.com/v1?api_key=secret-key-in-query&other=safe",
		Headers: host.Header{
			"Authorization": []string{"Bearer supersecret"},
			"Cookie":        []string{"session=abc"},
			"Content-Type":  []string{"application/json"},
		},
		Body: []byte(`{"token":"bodysecret"}`),
	}
	rec := host.RedactHTTPRequest(req)
	if strings.Contains(rec.URL, "secret-key-in-query") {
		t.Errorf("query param not redacted: %s", rec.URL)
	}
	if rec.Headers.Get("Authorization") != host.RedactedValue {
		t.Errorf("Authorization header not redacted: %s", rec.Headers.Get("Authorization"))
	}
	if rec.Headers.Get("Cookie") != host.RedactedValue {
		t.Errorf("Cookie header not redacted: %s", rec.Headers.Get("Cookie"))
	}
	if rec.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type should not be redacted: %s", rec.Headers.Get("Content-Type"))
	}
	if strings.Contains(rec.Body, "bodysecret") {
		t.Errorf("body not redacted: %s", rec.Body)
	}
	if !strings.Contains(rec.String(), host.RedactedValue) {
		t.Errorf("String representation should contain REDACTED: %s", rec.String())
	}
}
