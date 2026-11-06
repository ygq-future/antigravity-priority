package runtime

import (
	"context"
	"strings"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/state"
)

// appendRequestHistory prepends a new request history entry and trims to DefaultMaxRequestHistory.
func (r *Runtime) appendRequestHistory(entry state.RequestHistoryEntry) {
	r.mu.Lock()
	maxLen := state.DefaultMaxRequestHistory

	newHist := make([]state.RequestHistoryEntry, 0, len(r.requestHistory)+1)
	newHist = append(newHist, entry)
	for _, item := range r.requestHistory {
		if len(newHist) >= maxLen {
			break
		}
		newHist = append(newHist, item)
	}
	r.requestHistory = newHist

	cachePath := r.cfg.StateCachePath
	r.mu.Unlock()

	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(context.Background(), cachePath); err == nil {
		store.SetRequestHistory(newHist)
		_ = store.SaveAtomic(context.Background())
	}
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
	cachePath := r.cfg.StateCachePath
	r.mu.Unlock()

	if strings.TrimSpace(cachePath) == "" {
		cachePath = config.DefaultStateCachePath
	}
	if store, err := state.Load(ctx, cachePath); err == nil {
		store.SetRequestHistory(nil)
		_ = store.SaveAtomic(ctx)
	}
	return nil
}

// resolveAuthEmail attempts to resolve an auth index to an email address.
func (r *Runtime) resolveAuthEmail(authIndex string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.latestDualSnapshot != nil {
		for _, g := range r.latestDualSnapshot.Groups {
			for _, item := range g.Items {
				if item.AuthIndex == authIndex || item.Identity.AuthIndex == authIndex || item.Name == authIndex {
					if item.Email != "" {
						return item.Email
					}
					if item.Identity.Email != "" {
						return item.Identity.Email
					}
				}
			}
		}
	}
	return authIndex
}
