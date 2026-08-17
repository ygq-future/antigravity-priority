package host_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"antigravity-priority/internal/host"
)

func TestAuthFile_UnmarshalJSON(t *testing.T) {
	data := []byte(`{
		"name": "antigravity-auth",
		"auth_index": "idx-123",
		"type": "antigravity",
		"priority": 100,
		"disabled": false
	}`)

	var af host.AuthFile
	if err := json.Unmarshal(data, &af); err != nil {
		t.Fatalf("unmarshal AuthFile error: %v", err)
	}

	if af.Priority != 100 {
		t.Errorf("expected Priority=100, got %d", af.Priority)
	}
	if af.PriorityMissing {
		t.Errorf("expected PriorityMissing=false, got true")
	}
	if len(af.RawJSON) == 0 {
		t.Errorf("expected RawJSON to be preserved")
	}

	missingPriorityData := []byte(`{
		"name": "antigravity-auth",
		"auth_index": "idx-123"
	}`)
	var afMissing host.AuthFile
	if err := json.Unmarshal(missingPriorityData, &afMissing); err != nil {
		t.Fatalf("unmarshal AuthFile missing priority error: %v", err)
	}
	if !afMissing.PriorityMissing {
		t.Errorf("expected PriorityMissing=true, got false")
	}

	invalidJSON := []byte(`{invalid-json`)
	var afInvalid host.AuthFile
	if err := json.Unmarshal(invalidJSON, &afInvalid); err == nil {
		t.Error("expected error for invalid json, got nil")
	}
}

func TestHTTPResponse_UnmarshalJSON(t *testing.T) {
	bodyContent := "hello payload"
	encodedBody := base64.StdEncoding.EncodeToString([]byte(bodyContent))

	// Official uppercase format
	data := []byte(`{
		"StatusCode": 200,
		"Headers": {"Content-Type": ["application/json"]},
		"Body": "` + encodedBody + `"
	}`)

	var resp host.HTTPResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal HTTPResponse error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != bodyContent {
		t.Errorf("expected Body=%q, got %q", bodyContent, string(resp.Body))
	}

	// Legacy lowercase format with plain string
	legacyData := []byte(`{
		"status_code": 404,
		"headers": {"Content-Type": ["text/plain"]},
		"body": "not found plain text"
	}`)
	var respLegacy host.HTTPResponse
	if err := json.Unmarshal(legacyData, &respLegacy); err != nil {
		t.Fatalf("unmarshal legacy HTTPResponse error: %v", err)
	}
	if respLegacy.StatusCode != 404 {
		t.Errorf("expected StatusCode=404, got %d", respLegacy.StatusCode)
	}
	if string(respLegacy.Body) != "not found plain text" {
		t.Errorf("expected Body='not found plain text', got %q", string(respLegacy.Body))
	}

	// Empty body
	emptyData := []byte(`{"StatusCode": 204, "Body": ""}`)
	var respEmpty host.HTTPResponse
	if err := json.Unmarshal(emptyData, &respEmpty); err != nil {
		t.Fatalf("unmarshal empty HTTPResponse error: %v", err)
	}
	if respEmpty.Body != nil {
		t.Errorf("expected nil body, got %v", respEmpty.Body)
	}

	// Invalid base64 official body
	invalidBase64Data := []byte(`{"StatusCode": 500, "Body": "not_valid_base64!!"}`)
	var respInvalid host.HTTPResponse
	if err := json.Unmarshal(invalidBase64Data, &respInvalid); err == nil {
		t.Error("expected error for invalid base64 body, got nil")
	}

	// Invalid overall JSON
	if err := json.Unmarshal([]byte(`{invalid`), &respInvalid); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
