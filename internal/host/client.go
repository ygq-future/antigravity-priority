package host

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Client adapts CPA host callbacks to the host.API interface.
type Client struct {
	callbacks HostCallbacks
	patcher   *DocumentPatcher
}

// NewClient creates a new host callbacks adapter.
func NewClient(callbacks HostCallbacks) *Client {
	return &Client{
		callbacks: callbacks,
		patcher:   NewDocumentPatcher(),
	}
}

// HTTPStatusError represents a non-2xx HTTP status from host management.
type HTTPStatusError struct {
	StatusCode int
	Body       string
}

// Error implements error.
func (e *HTTPStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("host http status %d", e.StatusCode)
	}
	return fmt.Sprintf("host http status %d: %s", e.StatusCode, e.Body)
}

// ListAuthFiles lists host credentials via host.auth.list.
func (c *Client) ListAuthFiles(ctx context.Context) ([]AuthFile, error) {
	files, err := c.callbacks.ListAuthFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("host.auth.list: %w", err)
	}
	return files, nil
}

// GetAuth reads physical auth JSON document via host.auth.get.
func (c *Client) GetAuth(ctx context.Context, authIndex string) (AuthDocument, error) {
	document, err := c.callbacks.GetAuth(ctx, authIndex)
	if err != nil {
		return AuthDocument{}, fmt.Errorf("host.auth.get: %w", err)
	}
	return document, nil
}

// GetRuntime reads runtime credentials via host.auth.get_runtime.
func (c *Client) GetRuntime(ctx context.Context, authIndex string) (RuntimeAuth, error) {
	runtime, err := c.callbacks.GetRuntime(ctx, authIndex)
	if err != nil {
		return RuntimeAuth{}, fmt.Errorf("host.auth.get_runtime: %w", err)
	}
	return runtime, nil
}

// SaveAuth saves the auth JSON document via host.auth.save.
func (c *Client) SaveAuth(ctx context.Context, name string, doc json.RawMessage) error {
	trimmed, err := stableName(name)
	if err != nil {
		return err
	}
	if err := c.callbacks.SaveAuth(ctx, trimmed, doc); err != nil {
		return fmt.Errorf("host.auth.save: %w", err)
	}
	return nil
}

// PatchPriority updates the priority field in the physical credential document.
func (c *Client) PatchPriority(ctx context.Context, authIndex string, priority int) error {
	trimmed, err := stableName(authIndex)
	if err != nil {
		return err
	}
	document, err := c.GetAuth(ctx, trimmed)
	if err != nil {
		return err
	}
	if err := c.patcher.PatchPriority(ctx, document.Path, priority); err != nil {
		return fmt.Errorf("patch priority: %w", err)
	}
	return nil
}

// ResetPriority removes the priority field from the physical credential document, returning it to default unset state.
func (c *Client) ResetPriority(ctx context.Context, authIndex string) error {
	trimmed, err := stableName(authIndex)
	if err != nil {
		return err
	}
	document, err := c.GetAuth(ctx, trimmed)
	if err != nil {
		return err
	}
	if err := c.patcher.ResetPriority(ctx, document.Path); err != nil {
		return fmt.Errorf("reset priority: %w", err)
	}
	return nil
}

// PatchDisabled updates the disabled field in the physical credential document.
func (c *Client) PatchDisabled(ctx context.Context, name string, disabled bool) error {
	trimmed, err := stableName(name)
	if err != nil {
		return err
	}
	document, err := c.authDocumentByName(ctx, trimmed)
	if err != nil {
		return err
	}
	if err := c.patcher.PatchDisabled(ctx, document.Path, disabled); err != nil {
		return fmt.Errorf("patch disabled: %w", err)
	}
	return nil
}

func (c *Client) authDocumentByName(ctx context.Context, name string) (AuthDocument, error) {
	files, err := c.ListAuthFiles(ctx)
	if err != nil {
		return AuthDocument{}, err
	}
	for _, file := range files {
		if file.Name == name || file.AuthIndex == name {
			return c.GetAuth(ctx, file.AuthIndex)
		}
	}
	return AuthDocument{}, fmt.Errorf("%w: auth document not found", ErrInvalidRequest)
}

// HTTPDo sends an external HTTP request via host.http.do and rejects non-2xx responses.
func (c *Client) HTTPDo(ctx context.Context, req HTTPRequest) (HTTPResponse, error) {
	resp, err := c.HTTPDoRaw(ctx, req)
	if err != nil {
		return HTTPResponse{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return resp, fmt.Errorf("host.http.do %s %s: %w", req.Method, redactURL(req.URL), &HTTPStatusError{StatusCode: resp.StatusCode, Body: RedactBytes(resp.Body)})
	}
	return resp, nil
}

// HTTPDoRaw sends an external HTTP request via host.http.do, retaining non-2xx response bodies.
func (c *Client) HTTPDoRaw(ctx context.Context, req HTTPRequest) (HTTPResponse, error) {
	method := strings.TrimSpace(req.Method)
	if method == "" || strings.TrimSpace(req.URL) == "" {
		return HTTPResponse{}, fmt.Errorf("%w: method and url are required", ErrInvalidRequest)
	}
	req.Method = method
	resp, err := c.callbacks.HTTPDo(ctx, req)
	if err != nil {
		return HTTPResponse{}, fmt.Errorf("host.http.do %s %s: %w", method, redactURL(req.URL), err)
	}
	return resp, nil
}

func stableName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalidRequest)
	}
	return trimmed, nil
}

var _ API = (*Client)(nil)
