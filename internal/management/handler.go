package management

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/apply"
	"antigravity-priority/internal/config"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/state"
)

// StatusInfo represents summary state for UI rendering and status inspection.
type StatusInfo struct {
	TotalCredentials int       `json:"total_credentials"`
	FreshCount       int       `json:"fresh_count"`
	UnknownCount     int       `json:"unknown_count"`
	FailedCount      int       `json:"failed_count"`
	NextProbeAt      time.Time `json:"next_probe_at"`
	LatestAudit      string    `json:"latest_audit"`
}

// Runner defines the application layer contract required by the management HTTP API.
type Runner interface {
	Run(ctx context.Context, request RunRequest) (apply.Result, error)
	Reset(ctx context.Context) (map[string]any, error)
	Status(ctx context.Context) (StatusInfo, error)
	LatestSnapshot(ctx context.Context) (apply.DualGroupSnapshot, error)
	SyncHost(ctx context.Context, modelGroup config.AntigravityModelGroup) (apply.DualGroupSnapshot, error)
	Diagnostics(ctx context.Context) (map[string]any, error)
	GetScheduleConfig(ctx context.Context) (config.ScheduleConfig, error)
	SetScheduleConfig(ctx context.Context, cfg config.ScheduleConfig) error
	GetDynamicConfig(ctx context.Context) (config.DynamicConfig, error)
	SetDynamicConfig(ctx context.Context, cfg config.DynamicConfig) error
	GetSamples(ctx context.Context, authIndex, modelGroup string) ([]state.QuotaSample, error)
}

// RunRequest encapsulates parameters for a manual scheduling run.
type RunRequest struct {
	Mode                  string
	AntigravityModelGroup config.AntigravityModelGroup
	AuthIndexes           []string
}

// Handler handles management HTTP API requests.
type Handler struct {
	runner Runner
	mu     sync.Mutex
	active bool
}

// NewHandler creates a new Handler with the given Runner.
func NewHandler(runner Runner) *Handler {
	return &Handler{
		runner: runner,
	}
}

const (
	// RouteSourceHeader marks internal routing origin (resource vs management).
	RouteSourceHeader = "X-Antigravity-Priority-Route-Source"

	// SourceResource marks requests originating from resource handler.
	SourceResource = "resource"
	// SourceManagement marks requests originating from management handler.
	SourceManagement = "management"

	// Management API Paths
	PathStatus         = "/status"
	PathRun            = "/run"
	PathSync           = "/sync"
	PathReset          = "/reset"
	PathDiagnostics    = "/diagnostics"
	PathSnapshotLatest = "/snapshot/latest"
	PathScheduleConfig = "/schedule/config"
	PathRuntimeConfig  = "/runtime-config"
	PathSamples        = "/samples"

	// URL Prefixes
	PrefixResourcePlugin   = "/v0/resource/plugins/" + config.PluginID
	PrefixManagementPlugin = "/v0/management/plugins/" + config.PluginID
	PrefixLegacyPlugin     = "/plugins/" + config.PluginID
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method
	source := r.Header.Get(RouteSourceHeader)

	if source == SourceResource {
		if path == PathStatus && method == http.MethodGet {
			h.handleStatus(w, r)
			return
		}
		h.writeJSONError(w, http.StatusNotFound, "resource route only serves static /status")
		return
	}

	switch {
	case (path == PathStatus || path == PrefixResourcePlugin+PathStatus || path == PrefixManagementPlugin+PathStatus) && method == http.MethodGet:
		h.handleStatus(w, r)
	case (path == PathRun || path == PrefixManagementPlugin+PathRun) && method == http.MethodPost:
		h.handleRun(w, r)
	case (path == PathSync || path == PrefixManagementPlugin+PathSync) && method == http.MethodPost:
		h.handleSync(w, r)
	case (path == PathReset || path == PrefixManagementPlugin+PathReset) && method == http.MethodPost:
		h.handleReset(w, r)
	case (path == PathDiagnostics || path == PrefixManagementPlugin+PathDiagnostics) && method == http.MethodGet:
		h.handleDiagnostics(w, r)
	case (path == PathSnapshotLatest || path == PrefixManagementPlugin+PathSnapshotLatest) && method == http.MethodGet:
		h.handleSnapshot(w, r)
	case (path == PathScheduleConfig || path == PrefixManagementPlugin+PathScheduleConfig) && method == http.MethodGet:
		h.handleGetScheduleConfig(w, r)
	case (path == PathScheduleConfig || path == PrefixManagementPlugin+PathScheduleConfig) && method == http.MethodPost:
		h.handleSetScheduleConfig(w, r)
	case (path == PathRuntimeConfig || path == PrefixManagementPlugin+PathRuntimeConfig) && method == http.MethodGet:
		h.handleGetConfig(w, r)
	case (path == PathRuntimeConfig || path == PrefixManagementPlugin+PathRuntimeConfig) && method == http.MethodPost:
		h.handleSetConfig(w, r)
	case (path == PathSamples || path == PrefixManagementPlugin+PathSamples) && method == http.MethodGet:
		h.handleGetSamples(w, r)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "route not found"})
	}
}

func (h *Handler) handleReset(w http.ResponseWriter, r *http.Request) {
	if !h.tryAcquire() {
		h.writeJSONError(w, http.StatusConflict, "concurrency conflict: runner is already active")
		return
	}
	defer h.release()

	result, err := h.runner.Reset(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(StatusHTML))
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode != "apply" && mode != "probe" {
		h.writeJSONError(w, http.StatusBadRequest, "invalid mode: must be 'apply' or 'probe'")
		return
	}

	if !h.tryAcquire() {
		h.writeJSONError(w, http.StatusConflict, "concurrency conflict: runner is already active")
		return
	}
	defer h.release()

	var modelGroup config.AntigravityModelGroup
	if mgStr := r.URL.Query().Get("antigravity_model_group"); mgStr != "" {
		mg, err := config.ParseAntigravityModelGroup(mgStr)
		if err != nil {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		modelGroup = mg
	}

	authIndexes := r.URL.Query()["auth_index"]

	result, err := h.runner.Run(r.Context(), RunRequest{
		Mode:                  mode,
		AntigravityModelGroup: modelGroup,
		AuthIndexes:           authIndexes,
	})
	if err != nil {
		if strings.Contains(err.Error(), "run already in progress") {
			h.writeJSONError(w, http.StatusConflict, err.Error())
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	diag, err := h.runner.Diagnostics(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	redactedDiag := redactMap(diag)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(redactedDiag)
}

func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := h.runner.LatestSnapshot(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	snapBytes, err := json.Marshal(snap)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "marshal snapshot failed: "+err.Error())
		return
	}

	var snapMap map[string]any
	if err := json.Unmarshal(snapBytes, &snapMap); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "unmarshal snapshot failed: "+err.Error())
		return
	}

	redactedSnap := redactMap(snapMap)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(redactedSnap)
}

func (h *Handler) handleGetScheduleConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.runner.GetScheduleConfig(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) handleSetScheduleConfig(w http.ResponseWriter, r *http.Request) {
	var req scheduleConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid schedule config JSON: "+err.Error())
		return
	}

	if err := config.ValidateScheduleWindow(req.WindowStart, req.WindowEnd); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg := config.ScheduleConfig{
		Paused:        req.Paused,
		WindowEnabled: req.WindowEnabled,
		WindowStart:   req.WindowStart,
		WindowEnd:     req.WindowEnd,
	}

	if err := h.runner.SetScheduleConfig(r.Context(), cfg); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cfg)
}

type scheduleConfigRequest struct {
	Paused        bool   `json:"paused"`
	WindowEnabled bool   `json:"window_enabled"`
	WindowStart   string `json:"window_start"`
	WindowEnd     string `json:"window_end"`
}

func (h *Handler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.runner.GetDynamicConfig(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var req config.DynamicConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid dynamic config JSON: "+err.Error())
		return
	}

	if err := h.runner.SetDynamicConfig(r.Context(), req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg, err := h.runner.GetDynamicConfig(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if !h.tryAcquire() {
		h.writeJSONError(w, http.StatusConflict, "concurrency conflict: runner is already active")
		return
	}
	defer h.release()

	var modelGroup config.AntigravityModelGroup
	if mgStr := r.URL.Query().Get("antigravity_model_group"); mgStr != "" {
		mg, err := config.ParseAntigravityModelGroup(mgStr)
		if err != nil {
			h.writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		modelGroup = mg
	}

	snap, err := h.runner.SyncHost(r.Context(), modelGroup)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	snapBytes, err := json.Marshal(snap)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "marshal snapshot failed: "+err.Error())
		return
	}

	var snapMap map[string]any
	if err := json.Unmarshal(snapBytes, &snapMap); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "unmarshal snapshot failed: "+err.Error())
		return
	}

	redactedSnap := redactMap(snapMap)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(redactedSnap)
}

func (h *Handler) handleGetSamples(w http.ResponseWriter, r *http.Request) {
	authIndex := r.URL.Query().Get("auth_index")
	if authIndex == "" {
		h.writeJSONError(w, http.StatusBadRequest, "auth_index is required")
		return
	}

	geminiSamples, _ := h.runner.GetSamples(r.Context(), authIndex, "gemini")
	claudeSamples, _ := h.runner.GetSamples(r.Context(), authIndex, "claude_gpt")

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"auth_index": authIndex,
		"groups": map[string]any{
			"gemini": map[string]any{
				"name":    "Gemini 模型",
				"samples": geminiSamples,
			},
			"claude_gpt": map[string]any{
				"name":    "Claude & GPT 模型",
				"samples": claudeSamples,
			},
		},
	})
}

func (h *Handler) tryAcquire() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.active {
		return false
	}
	h.active = true
	return true
}

func (h *Handler) release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
}

func (h *Handler) writeJSONError(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": redactText(errMsg)})
}

func redactMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		if isSensitiveKey(k) {
			dst[k] = "[REDACTED]"
			continue
		}
		switch val := v.(type) {
		case string:
			dst[k] = redactText(val)
		case map[string]any:
			dst[k] = redactMap(val)
		case []any:
			dst[k] = redactSlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

func redactSlice(src []any) []any {
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case string:
			dst[i] = redactText(val)
		case map[string]any:
			dst[i] = redactMap(val)
		case []any:
			dst[i] = redactSlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
}

func redactText(val string) string {
	return host.RedactBytes([]byte(val))
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	return k == "authorization" ||
		k == "cookie" ||
		k == "set_cookie" ||
		strings.Contains(k, "token") ||
		strings.Contains(k, "api_key") ||
		strings.Contains(k, "apikey") ||
		strings.Contains(k, "secret")
}
