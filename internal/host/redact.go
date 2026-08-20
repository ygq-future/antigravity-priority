package host

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	sensitiveTextPattern = regexp.MustCompile(`(?i)(authorization|api[_-]?key|token|cookie|set-cookie)(\s*[:=]\s*)([^\s,;\"'}]+)`)
	bearerTextPattern    = regexp.MustCompile(`(?i)(bearer\s+)([^\s,;\"'}]+)`)
)

// RedactHTTPRequest returns a safely logged copy of an HTTPRequest.
func RedactHTTPRequest(req HTTPRequest) RecordedHTTPRequest {
	return RecordedHTTPRequest{
		AuthIndex: req.AuthIndex,
		Method:    req.Method,
		URL:       redactURL(req.URL),
		Headers:   redactHeaders(req.Headers),
		Body:      RedactBytes(req.Body),
	}
}

// String implements fmt.Stringer for RecordedHTTPRequest.
func (r RecordedHTTPRequest) String() string {
	encoded, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf("%s %s", r.Method, r.URL)
	}
	return string(encoded)
}

// RedactBytes removes known sensitive fields from JSON or text payloads.
func RedactBytes(raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	redacted, ok := redactJSON(trimmed)
	if ok {
		return string(redacted)
	}
	return redactText(string(raw))
}

func redactHeaders(headers Header) Header {
	if len(headers) == 0 {
		return nil
	}
	redacted := make(Header, len(headers))
	for key, values := range headers {
		copied := append([]string(nil), values...)
		if sensitiveKey(key) {
			for i := range copied {
				copied[i] = RedactedValue
			}
		}
		redacted[key] = copied
	}
	return redacted
}

func redactJSON(raw []byte) ([]byte, bool) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err == nil {
		redacted := make(map[string]json.RawMessage, len(object))
		for key, value := range object {
			if sensitiveKey(key) {
				redacted[key] = json.RawMessage(`"[REDACTED]"`)
				continue
			}
			if child, ok := redactJSON(bytes.TrimSpace(value)); ok {
				redacted[key] = child
				continue
			}
			redacted[key] = append(json.RawMessage(nil), value...)
		}
		encoded, err := json.Marshal(redacted)
		return encoded, err == nil
	}

	var array []json.RawMessage
	if err := json.Unmarshal(raw, &array); err == nil {
		redacted := make([]json.RawMessage, len(array))
		for i, value := range array {
			if child, ok := redactJSON(bytes.TrimSpace(value)); ok {
				redacted[i] = child
				continue
			}
			redacted[i] = append(json.RawMessage(nil), value...)
		}
		encoded, err := json.Marshal(redacted)
		return encoded, err == nil
	}
	return nil, false
}

func redactText(value string) string {
	redacted := bearerTextPattern.ReplaceAllString(value, "${1}"+RedactedValue)
	return sensitiveTextPattern.ReplaceAllString(redacted, "${1}${2}"+RedactedValue)
}

func redactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}
	query := parsed.Query()
	for key := range query {
		if sensitiveKey(key) {
			query.Set(key, RedactedValue)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	return normalized == "authorization" ||
		normalized == "cookie" ||
		normalized == "set_cookie" ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "secret")
}
