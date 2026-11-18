package runtime

import (
	"context"
	"log"
	"sync"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/state"
)

// One worker drains the FIFO outside runMu. Keys stay reserved through completion,
// so another quota probe can finish without duplicating a slow generation.
type warmupQueue struct {
	mu      sync.Mutex
	keys    map[string]bool
	pending []queuedWarmup
	done    chan struct{}
	paused  bool
}

type queuedWarmup struct {
	warmupJob
	store *state.Store
}

func (r *Runtime) enqueueEligibleWarmups(client *host.Client, cfg config.WarmupConfig, group config.AntigravityModelGroup, byGroup map[config.AntigravityModelGroup]evidence.Result, materials map[string]authMaterial, disabled map[string]struct{}, store *state.Store, now time.Time) {
	q := &r.warmups
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.paused || r.rootCtx.Err() != nil {
		return
	}
	if q.keys == nil {
		q.keys = make(map[string]bool)
	}
	visitEligibleWarmups(cfg, group, byGroup, materials, disabled, store, now, func(job warmupJob) {
		key := warmupCandidateKey(job.authIndex, string(job.group))
		if q.keys[key] {
			return
		}
		q.keys[key] = true
		q.pending = append(q.pending, queuedWarmup{job, store})
	})
	if len(q.pending) > 0 && q.done == nil {
		q.done = make(chan struct{})
		go r.drainWarmups(client)
	}
}

func (r *Runtime) drainWarmups(client *host.Client) {
	q := &r.warmups
	for {
		q.mu.Lock()
		if len(q.pending) == 0 || q.paused || r.rootCtx.Err() != nil {
			q.pending = nil
			clear(q.keys)
			close(q.done)
			q.done = nil
			q.mu.Unlock()
			return
		}
		j := q.pending[0]
		q.pending[0] = queuedWarmup{}
		q.pending = q.pending[1:]
		q.mu.Unlock()
		r.runQueuedWarmup(client, j)
		q.mu.Lock()
		delete(q.keys, warmupCandidateKey(j.authIndex, string(j.group)))
		q.mu.Unlock()
	}
}

func (r *Runtime) runQueuedWarmup(client *host.Client, j queuedWarmup) {
	ctx := r.rootCtx
	cfg, err := r.Config()
	if err != nil || !cfg.Enabled || j.store != r.state.StoreOrNil() {
		return
	}
	enabled := warmupEffectiveEnabled(cfg.Warmup, j.authIndex, j.group)
	if j.group == config.AntigravityModelGroupArgon {
		enabled = cfg.Warmup.ArgonEnabled
	}
	if !enabled {
		return
	}
	// A credential may have been disabled or removed while an earlier job waited upstream.
	files, err := client.ListAuthFiles(ctx)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.AuthIndex != j.authIndex {
			continue
		}
		if f.Disabled {
			return
		}
		raw, err := readCredentialAuthJSON(ctx, client, j.authIndex)
		if err != nil {
			return
		}
		if disabled, _ := disabledFromJSON(raw); disabled {
			return
		}
		j.material.accessToken = accessTokenFromJSON(raw)
		j.material.projectID = projectIDFromJSON(raw)
		now := r.clock.Now().UTC()
		if _, blocked := j.store.GetActiveCooldownEntriesForGroup(now, string(j.group))[j.authIndex]; blocked {
			return
		}
		if j.group == config.AntigravityModelGroupArgon {
			// Another probe can replace the quota or tier while this job waits.
			ev, ok := j.store.GetHistoricalEvidence(j.authIndex, string(j.group))
			if !ok || !argonWarmupEligible(ev, now, j.store.GetLastWarmupAt(j.authIndex, string(j.group))) {
				return
			}
		}
		if ctx.Err() == nil && r.executeWarmupJob(ctx, client, j.warmupJob, j.store, now) {
			// A successful upstream generation counts even if shutdown canceled
			// the request context while the host callback was returning.
			saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := j.store.SaveStateAtomic(saveCtx); err != nil {
				log.Printf("[antigravity-priority] persist warm-up completion: %v", err)
			}
		}
		return
	}
}

// Drop pending jobs, but allow the in-flight generation to finish and persist.
func (q *warmupQueue) pause() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paused = true
	for _, j := range q.pending {
		delete(q.keys, warmupCandidateKey(j.authIndex, string(j.group)))
	}
	q.pending = nil
}

func (q *warmupQueue) resume() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paused = false
}

func (q *warmupQueue) contains(key string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.keys[key]
}

func (q *warmupQueue) isPaused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

func (q *warmupQueue) wait(ctx context.Context) error {
	q.mu.Lock()
	done := q.done
	q.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
