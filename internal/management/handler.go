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
	Status(ctx context.Context) (StatusInfo, error)
	LatestSnapshot(ctx context.Context) (apply.PlanSnapshot, error)
	Diagnostics(ctx context.Context) (map[string]any, error)
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

// RouteSourceHeader marks internal routing origin (resource vs management).
const RouteSourceHeader = "X-Antigravity-Priority-Route-Source"

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method
	source := r.Header.Get(RouteSourceHeader)

	if source == "resource" {
		if path == "/status" && method == http.MethodGet {
			h.handleStatus(w, r)
			return
		}
		h.writeJSONError(w, http.StatusNotFound, "resource route only serves static /status")
		return
	}

	switch {
	case (path == "/status" || path == "/v0/resource/plugins/"+config.PluginID+"/status" || path == "/v0/management/plugins/"+config.PluginID+"/status") && method == http.MethodGet:
		h.handleStatus(w, r)
	case (path == "/run" || path == "/v0/management/plugins/"+config.PluginID+"/run") && method == http.MethodPost:
		h.handleRun(w, r)
	case (path == "/diagnostics" || path == "/v0/management/plugins/"+config.PluginID+"/diagnostics") && method == http.MethodGet:
		h.handleDiagnostics(w, r)
	case (path == "/snapshot/latest" || path == "/v0/management/plugins/"+config.PluginID+"/snapshot/latest") && method == http.MethodGet:
		h.handleSnapshot(w, r)
	default:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "route not found"})
	}
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(StatusHTML))
}

func (h *Handler) handleRun(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("mode")
	if mode != "dry-run" && mode != "apply" {
		h.writeJSONError(w, http.StatusBadRequest, "invalid mode: must be 'dry-run' or 'apply'")
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
