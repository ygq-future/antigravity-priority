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

	newHist := make([]state.RequestHistoryEntry, 0, len(r.requestHistory)+1)
	newHist = append(newHist, entry)
	for _, item := range r.requestHistory {
		if len(newHist) >= maxLen {
			break
		}
		newHist = append(newHist, item)
	}
	r.requestHistory = newHist
	r.mu.Unlock()

	r.persistRequestHistory(context.Background())
}

// trimRequestHistory enforces the configured retention limit after a config change.
func (r *Runtime) trimRequestHistory(ctx context.Context) {
	r.mu.Lock()
	limit := r.requestHistoryLimitLocked()
	if len(r.requestHistory) <= limit {
		r.mu.Unlock()
		return
	}
	r.requestHistory = append([]state.RequestHistoryEntry(nil), r.requestHistory[:limit]...)
	r.mu.Unlock()

	r.persistRequestHistory(ctx)
}

// requestHistoryLimitLocked returns the effective retention limit. The caller must hold r.mu.
func (r *Runtime) requestHistoryLimitLocked() int {
	if r.cfg.RequestHistoryLimit <= 0 {
		return state.DefaultMaxRequestHistory
	}
	return r.cfg.RequestHistoryLimit
}

// persistRequestHistory writes only the request-history sidecar. It never
// reads or rewrites quota evidence, so high request volume cannot disturb
// unrelated cache documents.
// The persist mutex keeps concurrent writers from replacing a newer snapshot with an older one.
func (r *Runtime) persistRequestHistory(ctx context.Context) {
	r.requestHistoryPersistMu.Lock()
	defer r.requestHistoryPersistMu.Unlock()

	r.mu.Lock()
	entries := append([]state.RequestHistoryEntry(nil), r.requestHistory...)
	cachePath := r.cfg.StateCachePath
	r.mu.Unlock()

	_ = state.WriteRequestHistoryAtomic(ctx, cachePath, entries)
}

// currentRequestHistory returns a copy of current request history entries.
func (r *Runtime) currentRequestHistory() []state.RequestHistoryEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]state.RequestHistoryEntry, len(r.requestHistory))
	copy(out, r.requestHistory)
	return out
}

// GetRequestHistory returns current request history entries.
func (r *Runtime) GetRequestHistory(ctx context.Context) ([]state.RequestHistoryEntry, error) {
	return r.currentRequestHistory(), nil
}

// ClearRequestHistory clears all recorded request history entries.
func (r *Runtime) ClearRequestHistory(ctx context.Context) error {
	r.mu.Lock()
	r.requestHistory = nil
	r.mu.Unlock()

	r.persistRequestHistory(ctx)
	return nil
}

// resolveAuthEmail attempts to resolve an auth index to a masked email address (first 4 characters + ***).
func (r *Runtime) resolveAuthEmail(authIndex string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	raw := authIndex
	if r.latestDualSnapshot != nil {
		for _, g := range r.latestDualSnapshot.Groups {
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
