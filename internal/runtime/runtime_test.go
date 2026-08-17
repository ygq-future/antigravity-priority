package runtime_test

import (
	"context"
	"encoding/json"
	"testing"

	"antigravity-priority/internal/runtime"
)

func TestRuntime_Handle_Register(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"enabled: true\nantigravity_model_group: gemini\ninterval: 15m\n"}`)

	respBytes := r.Handle(context.Background(), "plugin.register", req)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result runtime.RegisterResult `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}

	if envelope.Result.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", envelope.Result.SchemaVersion)
	}

	if envelope.Result.Metadata.Name != "Antigravity Priority" {
		t.Errorf("expected metadata name 'Antigravity Priority', got %q", envelope.Result.Metadata.Name)
	}

	if !envelope.Result.Capabilities["management"] {
		t.Errorf("expected management capability to be true")
	}
}

func TestRuntime_Handle_Reconfigure(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"enabled: false\nantigravity_model_group: claude_gpt\n"}`)

	respBytes := r.Handle(context.Background(), "plugin.reconfigure", req)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result runtime.RegisterResult `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}
}

func TestRuntime_Handle_Shutdown(t *testing.T) {
	r := runtime.New(runtime.Options{})

	respBytes := r.Handle(context.Background(), "plugin.shutdown", nil)

	var envelope struct {
		OK     bool                   `json:"ok"`
		Result map[string]any         `json:"result"`
		Error  *runtime.EnvelopeError `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}

	if !envelope.OK {
		t.Fatalf("expected OK=true, got error: %+v", envelope.Error)
	}

	// Calling register after shutdown should fail
	afterShutdownResp := r.Handle(context.Background(), "plugin.register", nil)
	var afterEnvelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	_ = json.Unmarshal(afterShutdownResp, &afterEnvelope)
	if afterEnvelope.OK {
		t.Errorf("expected failure after shutdown, got OK=true")
	}
	if afterEnvelope.Error == nil || afterEnvelope.Error.Code != "shutdown" {
		t.Errorf("expected error code 'shutdown', got %+v", afterEnvelope.Error)
	}
}

func TestRuntime_Handle_UnknownMethod(t *testing.T) {
	r := runtime.New(runtime.Options{})
	respBytes := r.Handle(context.Background(), "unknown.method", nil)

	var envelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}
	if envelope.OK {
		t.Errorf("expected OK=false for unknown method")
	}
	if envelope.Error == nil || envelope.Error.Code != "invalid_request" {
		t.Errorf("expected error code 'invalid_request', got %+v", envelope.Error)
	}
}

func TestRuntime_Handle_InvalidConfig(t *testing.T) {
	r := runtime.New(runtime.Options{})
	req := []byte(`{"config_yaml":"interval: -5m"}`)

	respBytes := r.Handle(context.Background(), "plugin.register", req)

	var envelope struct {
		OK    bool                   `json:"ok"`
		Error *runtime.EnvelopeError `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		t.Fatalf("unmarshal response envelope failed: %v", err)
	}
	if envelope.OK {
		t.Errorf("expected OK=false for invalid config")
	}
	if envelope.Error == nil || envelope.Error.Code != "invalid_config" {
		t.Errorf("expected error code 'invalid_config', got %+v", envelope.Error)
	}
}
