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

func TestClient_ReadAndSave(t *testing.T) {
	ctx := context.Background()
	callbacks := &mockHostCallbacks{
		listAuthFilesFunc: func(context.Context) ([]host.AuthFile, error) {
			return []host.AuthFile{{Name: "c1", AuthIndex: "idx-1"}}, nil
		},
		getAuthFunc: func(_ context.Context, authIndex string) (host.AuthDocument, error) {
			if authIndex != "idx-1" {
				return host.AuthDocument{}, errors.New("not found")
			}
			return host.AuthDocument{AuthIndex: authIndex, Name: "c1", JSON: json.RawMessage(`{"priority":10}`)}, nil
		},
		getRuntimeFunc: func(_ context.Context, authIndex string) (host.RuntimeAuth, error) {
			return host.RuntimeAuth{AuthIndex: authIndex}, nil
		},
	}
	client := host.NewClient(callbacks)
	files, err := client.ListAuthFiles(ctx)
	if err != nil || len(files) != 1 || files[0].AuthIndex != "idx-1" {
		t.Fatalf("unexpected ListAuthFiles result: %#v, %v", files, err)
	}
	doc, err := client.GetAuth(ctx, "idx-1")
	if err != nil || doc.Name != "c1" {
		t.Fatalf("unexpected GetAuth result: %#v, %v", doc, err)
	}
	runtimeAuth, err := client.GetRuntime(ctx, "idx-1")
	if err != nil || runtimeAuth.AuthIndex != "idx-1" {
		t.Fatalf("unexpected GetRuntime result: %#v, %v", runtimeAuth, err)
	}
	if err := client.SaveAuth(ctx, "  ", json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected empty save name to fail")
	}
}

func TestClient_GetAuthLoadsPhysicalDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"access_token":"secret","priority":10}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := host.NewClient(&mockHostCallbacks{
		getAuthFunc: func(context.Context, string) (host.AuthDocument, error) {
			return host.AuthDocument{AuthIndex: "idx-1", Path: path}, nil
		},
	})
	doc, err := client.GetAuth(context.Background(), "idx-1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc.JSON), "priority") {
		t.Fatalf("expected physical JSON to be loaded, got %s", doc.JSON)
	}
}

func TestClient_ReplaceAuth_UsesOneCompleteDocumentReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	initial := []byte(`{"access_token":"secret","priority":10,"disabled":false,"metadata":{"keep":true}}`)
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	client := host.NewClient(&mockHostCallbacks{
		getAuthFunc: func(context.Context, string) (host.AuthDocument, error) {
			return host.AuthDocument{AuthIndex: "idx-1", Name: "credential", Path: path}, nil
		},
	})
	doc, err := client.GetAuth(context.Background(), "idx-1")
	if err != nil {
		t.Fatal(err)
	}
	replacement := json.RawMessage(`{"access_token":"secret","priority":90,"disabled":true,"metadata":{"keep":true}}`)
	if err := client.ReplaceAuth(context.Background(), doc, replacement); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["priority"] != float64(90) || decoded["disabled"] != true || decoded["metadata"].(map[string]any)["keep"] != true {
		t.Fatalf("unexpected replaced document: %s", data)
	}
}

func TestClient_ReplaceAuth_UsesHostCallbackWithoutPath(t *testing.T) {
	var savedName string
	var saved json.RawMessage
	client := host.NewClient(&mockHostCallbacks{
		saveAuthFunc: func(_ context.Context, name string, doc json.RawMessage) error {
			savedName, saved = name, append(json.RawMessage(nil), doc...)
			return nil
		},
	})
	if err := client.ReplaceAuth(context.Background(), host.AuthDocument{AuthIndex: "idx-1", Name: "credential"}, json.RawMessage(`{"priority":1}`)); err != nil {
		t.Fatal(err)
	}
	if savedName != "credential" || string(saved) != `{"priority":1}` {
		t.Fatalf("unexpected callback replacement: name=%q doc=%s", savedName, saved)
	}
}

func TestClient_HTTPDo(t *testing.T) {
	client := host.NewClient(&mockHostCallbacks{
		httpDoFunc: func(_ context.Context, req host.HTTPRequest) (host.HTTPResponse, error) {
			if strings.HasSuffix(req.URL, "/ok") {
				return host.HTTPResponse{StatusCode: 200, Body: []byte(`{"ok":true}`)}, nil
			}
			return host.HTTPResponse{StatusCode: 429, Body: []byte(`{"token":"secret"}`)}, nil
		},
	})
	if _, err := client.HTTPDo(context.Background(), host.HTTPRequest{Method: "", URL: "https://example.com"}); err == nil {
		t.Fatal("expected invalid HTTP request")
	}
	if _, err := client.HTTPDo(context.Background(), host.HTTPRequest{Method: "GET", URL: "https://example.com/ok"}); err != nil {
		t.Fatal(err)
	}
	_, err := client.HTTPDo(context.Background(), host.HTTPRequest{Method: "GET", URL: "https://example.com/error"})
	if err == nil || strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "429") {
		t.Fatalf("unexpected redacted HTTP error: %v", err)
	}
}
