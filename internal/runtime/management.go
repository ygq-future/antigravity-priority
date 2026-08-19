package runtime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/management"
	"antigravity-priority/internal/state"
)

// ManagementRequest represents the HTTP request envelope passed by CPA management.handle.
type ManagementRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Query   url.Values
	Body    []byte
}

// ManagementResponse represents the HTTP response envelope returned to CPA.
type ManagementResponse struct {
	StatusCode  int                 `json:"StatusCode"`
	ContentType string              `json:"content_type"`
	Headers     map[string][]string `json:"Headers"`
	Body        string              `json:"Body"`
}

type managementRoute struct {
	Method string `json:"Method"`
	Path   string `json:"Path"`
}

type managementResource struct {
	Path        string `json:"Path"`
	Menu        string `json:"Menu"`
	Description string `json:"Description"`
}

type managementRegistration struct {
	Routes    []managementRoute    `json:"routes"`
	Resources []managementResource `json:"resources"`
}

type managementRunner struct {
	runtime *Runtime
}

func (r *Runtime) registerManagement() []byte {
	result := managementRegistration{
		Routes: []managementRoute{
			{Method: http.MethodPost, Path: "/plugins/" + config.PluginID + "/run"},
			{Method: http.MethodPost, Path: "/plugins/" + config.PluginID + "/reset"},
			{Method: http.MethodGet, Path: "/plugins/" + config.PluginID + "/diagnostics"},
			{Method: http.MethodGet, Path: "/plugins/" + config.PluginID + "/snapshot/latest"},
			{Method: http.MethodGet, Path: "/plugins/" + config.PluginID + "/schedule/config"},
			{Method: http.MethodPost, Path: "/plugins/" + config.PluginID + "/schedule/config"},
			{Method: http.MethodGet, Path: "/plugins/" + config.PluginID + "/config"},
			{Method: http.MethodPost, Path: "/plugins/" + config.PluginID + "/config"},
		},
		Resources: []managementResource{
			{Path: "/status", Menu: "Antigravity Priority", Description: "Shows Antigravity priority status and audit summary."},
		},
	}
	return envelopeManagement(result, nil)
}

func (r *Runtime) handleManagement(ctx context.Context, raw []byte) []byte {
	request, err := decodeManagementRequest(raw)
	if err != nil {
		return failure(err)
	}
	httpRequest, err := request.toHTTPRequest(ctx)
	if err != nil {
		return failure(err)
	}
	recorder := httptest.NewRecorder()
	r.management.ServeHTTP(recorder, httpRequest)
	return envelopeManagement(newManagementResponse(recorder), nil)
}

func decodeManagementRequest(raw []byte) (ManagementRequest, error) {
	var request managementRequestWire
	if len(raw) == 0 {
		return ManagementRequest{}, fmt.Errorf("%w: management request is required", ErrInvalidRequest)
	}
	if err := json.Unmarshal(raw, &request); err != nil {
		return ManagementRequest{}, fmt.Errorf("%w: decode management request: %v", ErrInvalidRequest, err)
	}
	parsed, err := request.toManagementRequest()
	if err != nil {
		return ManagementRequest{}, err
	}
	if parsed.Method == "" || parsed.Path == "" {
		return ManagementRequest{}, fmt.Errorf("%w: management method and path are required", ErrInvalidRequest)
	}
	return parsed, nil
}

type managementRequestWire struct {
	Method      string          `json:"Method"`
	MethodLower string          `json:"method"`
	Path        string          `json:"Path"`
	PathLower   string          `json:"path"`
	Headers     http.Header     `json:"Headers"`
	QueryRaw    json.RawMessage `json:"Query"`
	QueryLower  string          `json:"query"`
	Body        string          `json:"Body"`
	BodyLower   string          `json:"body"`
}

func (w managementRequestWire) toManagementRequest() (ManagementRequest, error) {
	method := firstNonEmpty(w.Method, w.MethodLower)
	path := firstNonEmpty(w.Path, w.PathLower)
	body, err := decodeManagementBody(w.Body, w.BodyLower)
	if err != nil {
		return ManagementRequest{}, err
	}
	var query url.Values
	if len(w.QueryRaw) > 0 {
		var strQuery string
		if err := json.Unmarshal(w.QueryRaw, &strQuery); err == nil {
			values, err := url.ParseQuery(strings.TrimPrefix(strQuery, "?"))
			if err != nil {
				return ManagementRequest{}, fmt.Errorf("%w: decode management query: %v", ErrInvalidRequest, err)
			}
			query = values
		} else {
			var mapQuery map[string][]string
			if err := json.Unmarshal(w.QueryRaw, &mapQuery); err == nil {
				query = mapQuery
			}
		}
	}
	if query == nil && strings.TrimSpace(w.QueryLower) != "" {
		values, err := url.ParseQuery(strings.TrimPrefix(w.QueryLower, "?"))
		if err != nil {
			return ManagementRequest{}, fmt.Errorf("%w: decode management query: %v", ErrInvalidRequest, err)
		}
		query = values
	}
	return ManagementRequest{Method: method, Path: path, Headers: w.Headers, Query: query, Body: body}, nil
}

func decodeManagementBody(official string, legacy string) ([]byte, error) {
	if official != "" {
		decoded, err := base64.StdEncoding.DecodeString(official)
		if err != nil {
			return nil, fmt.Errorf("%w: decode management body: %v", ErrInvalidRequest, err)
		}
		return decoded, nil
	}
	return []byte(legacy), nil
}

func (r ManagementRequest) toHTTPRequest(ctx context.Context) (*http.Request, error) {
	normalized, source := normalizeManagementPath(r.Path)
	if !strings.HasPrefix(normalized, "/") {
		return nil, fmt.Errorf("%w: management path must start with /", ErrInvalidRequest)
	}
	path := normalized
	if r.Query != nil {
		encoded := r.Query.Encode()
		if encoded != "" {
			path += "?" + encoded
		}
	}
	request, err := http.NewRequestWithContext(ctx, r.Method, path, bytes.NewBuffer(r.Body))
	if err != nil {
		return nil, fmt.Errorf("%w: build management request: %v", ErrInvalidRequest, err)
	}
	if r.Headers != nil {
		request.Header = r.Headers.Clone()
	} else {
		request.Header = make(http.Header)
	}
	request.Header.Set(management.RouteSourceHeader, source)
	return request, nil
}

func normalizeManagementPath(path string) (normalized string, source string) {
	resourcePrefix := "/v0/resource/plugins/" + config.PluginID
	managementPrefix := "/v0/management/plugins/" + config.PluginID
	legacyRoutePrefix := "/plugins/" + config.PluginID

	switch {
	case path == resourcePrefix:
		return "/", "resource"
	case strings.HasPrefix(path, resourcePrefix+"/"):
		return strings.TrimPrefix(path, resourcePrefix), "resource"
	case path == managementPrefix:
		return "/", "management"
	case strings.HasPrefix(path, managementPrefix+"/"):
		return strings.TrimPrefix(path, managementPrefix), "management"
	case path == legacyRoutePrefix:
		return "/", "management"
	case strings.HasPrefix(path, legacyRoutePrefix+"/"):
		return strings.TrimPrefix(path, legacyRoutePrefix), "management"
	default:
		return path, "management"
	}
}

func newManagementResponse(recorder *httptest.ResponseRecorder) ManagementResponse {
	result := recorder.Result()
	return ManagementResponse{
		StatusCode:  result.StatusCode,
		ContentType: result.Header.Get("Content-Type"),
		Headers:     result.Header,
		Body:        base64.StdEncoding.EncodeToString(recorder.Body.Bytes()),
	}
}

func envelopeManagement(result any, err error) []byte {
	if err != nil {
		return failure(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return failure(fmt.Errorf("encode management result: %w", err))
	}
	return mustMarshal(Envelope{OK: true, Result: encoded})
}

func (r managementRunner) Run(ctx context.Context, request management.RunRequest) (apply.Result, error) {
	if request.Mode == "apply" {
		if err := r.runtime.ManualApply(ctx, request.AntigravityModelGroup, request.AuthIndexes); err != nil {
			return apply.Result{}, err
		}
		result, _ := r.runtime.currentRunSnapshot()
		return result, nil
	}
	if request.Mode == "probe" {
		if err := r.runtime.Probe(ctx, request.AntigravityModelGroup, request.AuthIndexes); err != nil {
			return apply.Result{}, err
		}
		result, _ := r.runtime.currentRunSnapshot()
		return result, nil
	}
	if err := r.runtime.DryRun(ctx, request.AntigravityModelGroup, request.AuthIndexes); err != nil {
		return apply.Result{}, err
	}
	result, _ := r.runtime.currentRunSnapshot()
	return result, nil
}

func (r managementRunner) Reset(ctx context.Context) (map[string]any, error) {
	return r.runtime.ResetAllPriorities(ctx)
}

func (r managementRunner) Status(ctx context.Context) (management.StatusInfo, error) {
	return r.runtime.Status(ctx)
}

func (r managementRunner) LatestSnapshot(ctx context.Context) (apply.DualGroupSnapshot, error) {
	return r.runtime.LatestSnapshot(ctx)
}

func (r managementRunner) Diagnostics(ctx context.Context) (map[string]any, error) {
	return r.runtime.Diagnostics(ctx)
}

func (r managementRunner) GetScheduleConfig(ctx context.Context) (state.ScheduleConfig, error) {
	return r.runtime.GetScheduleConfig(ctx)
}

func (r managementRunner) SetScheduleConfig(ctx context.Context, cfg state.ScheduleConfig) error {
	return r.runtime.SetScheduleConfig(ctx, cfg)
}

func (r managementRunner) GetDynamicConfig(ctx context.Context) (state.DynamicConfig, error) {
	return r.runtime.GetDynamicConfig(ctx)
}

func (r managementRunner) SetDynamicConfig(ctx context.Context, cfg state.DynamicConfig) error {
	return r.runtime.SetDynamicConfig(ctx, cfg)
}

var _ management.Runner = managementRunner{}
