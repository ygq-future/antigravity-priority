package runtime

import (
	"path/filepath"
	"testing"
	"time"

	"antigravity-priority/internal/config"
)

func TestGetOrRefreshPoolTablePrunesExpiredSessionAffinity(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	pool := config.AntigravityModelGroupGemini

	r := &Runtime{
		cfg: config.Config{StateCachePath: filepath.Join(t.TempDir(), "cache.json")},
		sessionAffinity: map[string]sessionBinding{
			"gemini::expired": {CandidateID: "account-a", ExpiresAt: now.Add(-time.Minute)},
			"gemini::active":  {CandidateID: "account-b", ExpiresAt: now.Add(time.Minute)},
		},
		plansByPool: map[config.AntigravityModelGroup]PoolRoutingTable{
			pool: {DecidedAt: now.Add(-2 * time.Minute)},
		},
	}

	r.getOrRefreshPoolTable(pool, now)

	if _, ok := r.sessionAffinity["gemini::expired"]; ok {
		t.Fatal("expired session binding was not pruned during routing refresh")
	}
	if _, ok := r.sessionAffinity["gemini::active"]; !ok {
		t.Fatal("active session binding was pruned")
	}
}
