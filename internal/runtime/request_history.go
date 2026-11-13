package runtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"antigravity-priority/internal/host"
	"antigravity-priority/internal/state"
)

// requestHistoryErrorMessage returns the sanitized failure detail shown for a request history entry.
func requestHistoryErrorMessage(failed bool, statusCode int, body string) string {
	if !failed {
		return ""
	}
	if detail := strings.TrimSpace(host.RedactBytes([]byte(body))); detail != "" {
		return detail
	}
	if text := strings.TrimSpace(http.StatusText(statusCode)); text != "" {
		return fmt.Sprintf("HTTP %d %s", statusCode, text)
	}
	return fmt.Sprintf("HTTP %d", statusCode)
}

// appendRequestHistory prepends a new request history entry and trims to the configured retention limit.
func (r *Runtime) appendRequestHistory(entry state.RequestHistoryEntry) {
	r.mu.Lock()
	maxLen := r.requestHistoryLimitLocked()
	r.mu.Unlock()
	r.state.AppendRequestHistory(context.Background(), entry, maxLen)
}

// trimRequestHistory enforces the configured retention limit after a config change.
func (r *Runtime) trimRequestHistory(ctx context.Context) {
	r.mu.Lock()
	limit := r.requestHistoryLimitLocked()
	r.mu.Unlock()
	r.state.TrimRequestHistory(ctx, limit)
}

// requestHistoryLimitLocked returns the effective retention limit. The caller must hold r.mu.
func (r *Runtime) requestHistoryLimitLocked() int {
	if r.cfg.RequestHistoryLimit <= 0 {
		return state.DefaultMaxRequestHistory
	}
	return r.cfg.RequestHistoryLimit
}

// currentRequestHistory returns a copy of current request history entries.
func (r *Runtime) currentRequestHistory() []state.RequestHistoryEntry {
	return r.state.RequestHistory()
}

// GetRequestHistory returns current request history entries.
func (r *Runtime) GetRequestHistory(ctx context.Context) ([]state.RequestHistoryEntry, error) {
	return r.currentRequestHistory(), nil
}

// ClearRequestHistory clears all recorded request history entries.
func (r *Runtime) ClearRequestHistory(ctx context.Context) error {
	r.state.ClearRequestHistory(ctx)
	return nil
}

// resolveAuthEmail attempts to resolve an auth index to a masked email address (first 4 characters + ***).
func (r *Runtime) resolveAuthEmail(authIndex string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	raw := authIndex
	if r.latestSnapshotSet != nil {
		for _, g := range r.latestSnapshotSet.Groups {
			for _, item := range g.Items {
				if item.AuthIndex == authIndex || item.Identity.AuthIndex == authIndex || item.Name == authIndex {
					if item.Identity.Email != "" {
						return redactRuntimeIdentifier(item.Identity.Email)
					}
					if item.Email != "" {
						return redactRuntimeIdentifier(item.Email)
					}
				}
			}
		}
	}
	return redactRuntimeIdentifier(raw)
}
