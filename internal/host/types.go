package host

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// ManagementAuthStatusPath is the CPA management API path to update credential disabled status.
	ManagementAuthStatusPath = "/v0/management/auth-files/status"
	// RedactedValue is the placeholder string for sensitive headers and fields.
	RedactedValue = "[REDACTED]"
)

// ErrInvalidRequest indicates an invalid host request.
var ErrInvalidRequest = errors.New("host: invalid request")

// Header is a lightweight HTTP header map.
type Header map[string][]string

// Get returns the first value associated with the given key, case-insensitively.
func (h Header) Get(key string) string {
	if h == nil {
		return ""
	}
	if v, ok := h[key]; ok && len(v) > 0 {
		return v[0]
	}
	keyLower := strings.ToLower(key)
	for k, v := range h {
		if strings.ToLower(k) == keyLower && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

// AuthFile represents a minimal credential record returned by host.auth.list.
type AuthFile struct {
	Name            string          `json:"name"`
	AuthIndex       string          `json:"auth_index"`
	Type            string          `json:"type,omitempty"`
	Provider        string          `json:"provider,omitempty"`
	Status          string          `json:"status,omitempty"`
	Disabled        bool            `json:"disabled"`
	Unavailable     bool            `json:"unavailable"`
	Priority        int             `json:"priority"`
	PriorityMissing bool            `json:"-"`
	Account         string          `json:"account,omitempty"`
	Email           string          `json:"email,omitempty"`
	RawJSON         json.RawMessage `json:"-"`
}

// UnmarshalJSON retains the raw JSON bytes while parsing the typed fields.
func (f *AuthFile) UnmarshalJSON(data []byte) error {
	type authFile AuthFile
	var decoded authFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*f = AuthFile(decoded)
	_, priorityPresent := fields["priority"]
	f.PriorityMissing = !priorityPresent
	f.RawJSON = append(json.RawMessage(nil), data...)
	return nil
}

// RuntimeAuth represents the runtime auth view returned by host.auth.get_runtime.
type RuntimeAuth struct {
	AuthIndex string          `json:"auth_index"`
	Name      string          `json:"name"`
	Provider  string          `json:"provider,omitempty"`
	Disabled  bool            `json:"disabled"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

// AuthDocument represents the physical auth JSON document returned by host.auth.get.
type AuthDocument struct {
	AuthIndex string          `json:"auth_index"`
	Name      string          `json:"name,omitempty"`
	Path      string          `json:"path,omitempty"`
	JSON      json.RawMessage `json:"json"`
}

// HTTPRequest represents a host-mediated HTTP request.
type HTTPRequest struct {
	AuthIndex string `json:"auth_index,omitempty"`
	Method    string `json:"Method"`
	URL       string `json:"URL"`
	Headers   Header `json:"Headers,omitempty"`
	Body      []byte `json:"Body,omitempty"`
}

// HTTPResponse represents a host-mediated HTTP response.
type HTTPResponse struct {
	StatusCode int    `json:"StatusCode"`
	Headers    Header `json:"Headers,omitempty"`
	Body       []byte `json:"Body,omitempty"`
}

// UnmarshalJSON supports both uppercase and lowercase field names and base64 body decoding.
func (r *HTTPResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		StatusCode      *int    `json:"StatusCode"`
		StatusCodeSnake *int    `json:"status_code"`
		Headers         Header  `json:"Headers"`
		HeadersLower    Header  `json:"headers"`
		Body            *string `json:"Body"`
		BodyLower       *string `json:"body"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.StatusCode != nil {
		r.StatusCode = *raw.StatusCode
	} else if raw.StatusCodeSnake != nil {
		r.StatusCode = *raw.StatusCodeSnake
	}
	if raw.Headers != nil {
		r.Headers = raw.Headers
	} else {
		r.Headers = raw.HeadersLower
	}
	body, err := decodeBodyString(raw.Body, raw.BodyLower)
	if err != nil {
		return err
	}
	r.Body = body
	return nil
}

func decodeBodyString(official *string, legacy *string) ([]byte, error) {
	if official != nil {
		if *official == "" {
			return nil, nil
		}
		decoded, err := base64.StdEncoding.DecodeString(*official)
		if err != nil {
			return nil, fmt.Errorf("decode Body base64: %w", err)
		}
		return decoded, nil
	}
	if legacy == nil {
		return nil, nil
	}
	if *legacy == "" {
		return nil, nil
	}
	if looksLikeBase64(*legacy) {
		if decoded, err := base64.StdEncoding.DecodeString(*legacy); err == nil {
			return decoded, nil
		}
	}
	return []byte(*legacy), nil
}

func looksLikeBase64(value string) bool {
	trimmed := strings.TrimSpace(value)
	return len(trimmed)%4 == 0 && !strings.ContainsAny(trimmed, "{}<>\n\r")
}

// RecordedHTTPRequest is the redacted audit view of an HTTPDo call.
type RecordedHTTPRequest struct {
	AuthIndex string `json:"auth_index,omitempty"`
	Method    string `json:"method"`
	URL       string `json:"url"`
	Headers   Header `json:"headers,omitempty"`
	Body      string `json:"body,omitempty"`
}

// HostCallbacks defines the host callback interface required by the runtime.
type HostCallbacks interface {
	ListAuthFiles(ctx context.Context) ([]AuthFile, error)
	GetAuth(ctx context.Context, authIndex string) (AuthDocument, error)
	GetRuntime(ctx context.Context, authIndex string) (RuntimeAuth, error)
	SaveAuth(ctx context.Context, name string, doc json.RawMessage) error
	HTTPDo(ctx context.Context, req HTTPRequest) (HTTPResponse, error)
}

// API defines the stable host adapter interface.
type API interface {
	ListAuthFiles(ctx context.Context) ([]AuthFile, error)
	GetAuth(ctx context.Context, authIndex string) (AuthDocument, error)
	GetRuntime(ctx context.Context, authIndex string) (RuntimeAuth, error)
	SaveAuth(ctx context.Context, name string, doc json.RawMessage) error
	PatchPriority(ctx context.Context, authIndex string, priority int) error
	PatchDisabled(ctx context.Context, name string, disabled bool) error
	HTTPDo(ctx context.Context, req HTTPRequest) (HTTPResponse, error)
}
