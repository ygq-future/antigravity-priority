package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/provider/antigravity"
	"antigravity-priority/internal/state"
)

type collectInput struct {
	client         *host.Client
	store          *state.Store
	credentials    []core.Credential
	authMaterials  map[string]authMaterial
	now            time.Time
	cacheTTL       time.Duration
	forceProbe     bool
	maxConcurrency int
	modelGroup     config.AntigravityModelGroup
	sampleCapacity int
}

type probeJob struct {
	credential   core.Credential
	authMaterial authMaterial
}

func collectFreshEvidence(ctx context.Context, input collectInput) ([]priority.ProbeEvidence, error) {
	prober := antigravity.NewProber(input.client, fixedClock{now: input.now})
	jobs := make([]probeJob, 0, len(input.credentials))
	cachedEvidence := make([]priority.ProbeEvidence, 0)

	for _, cred := range input.credentials {
		needsProbe, err := freshProbeNeeded(ctx, input, cred.AuthIndex, string(input.modelGroup))
		if err != nil {
			return nil, err
		}
		if needsProbe {
			jobs = append(jobs, probeJob{
				credential:   cred,
				authMaterial: input.authMaterials[cred.AuthIndex],
			})
			continue
		}

		if entry, ok := input.store.GetEntry(cred.AuthIndex, string(input.modelGroup)); ok && entry.SchemaVersion == state.SchemaVersion {
			cachedEvidence = append(cachedEvidence, cachedEvidenceFromEntry(entry))
		}
	}

	if len(jobs) == 0 {
		return cachedEvidence, nil
	}

	probedEvidence, err := runProbeJobs(ctx, prober, input, jobs)
	if err != nil {
		return nil, err
	}

	return append(cachedEvidence, probedEvidence...), nil
}

func freshProbeNeeded(ctx context.Context, input collectInput, authIndex string, modelGroup string) (bool, error) {
	if input.forceProbe {
		return true, nil
	}
	return input.store.NeedsProbe(ctx, state.ProbeCheck{
		AuthIndex:  authIndex,
		Provider:   core.ProviderAntigravity,
		ModelGroup: modelGroup,
		Now:        input.now,
		Policy:     probePolicy(input.cacheTTL),
	})
}

func probePolicy(cacheTTL time.Duration) state.ProbePolicy {
	if cacheTTL <= 0 {
		cacheTTL = 15 * time.Minute
	}
	return state.ProbePolicy{TTL: cacheTTL, ResetStaleAfter: time.Hour}
}

func runProbeJobs(ctx context.Context, prober antigravity.Prober, input collectInput, jobs []probeJob) ([]priority.ProbeEvidence, error) {
	workers := input.maxConcurrency
	if workers < 1 {
		workers = 2
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}

	if workers == 1 {
		evidence := make([]priority.ProbeEvidence, 0, len(jobs))
		for _, job := range jobs {
			item, err := probeAndRecord(ctx, prober, input.store, job, input.now, input.modelGroup, input.sampleCapacity)
			if err != nil {
				return nil, err
			}
			if item.Status != priority.EvidenceStatusUnknown {
				evidence = append(evidence, item)
			}
		}
		return evidence, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		index int
		item  priority.ProbeEvidence
		err   error
	}

	jobsCh := make(chan int)
	resultsCh := make(chan result, len(jobs))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobsCh {
				if runCtx.Err() != nil {
					return
				}
				job := jobs[index]
				item, err := probeAndRecord(runCtx, prober, input.store, job, input.now, input.modelGroup, input.sampleCapacity)
				select {
				case resultsCh <- result{index: index, item: item, err: err}:
				case <-runCtx.Done():
					return
				}
				if err != nil {
					cancel()
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobsCh)
		for index := range jobs {
			select {
			case jobsCh <- index:
			case <-runCtx.Done():
				return
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	ordered := make([]priority.ProbeEvidence, len(jobs))
	present := make([]bool, len(jobs))
	var firstErr error
	for res := range resultsCh {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			cancel()
			continue
		}
		if res.item.Status != priority.EvidenceStatusUnknown {
			ordered[res.index] = res.item
			present[res.index] = true
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	evidence := make([]priority.ProbeEvidence, 0, len(jobs))
	for index, ok := range present {
		if ok {
			evidence = append(evidence, ordered[index])
		}
	}
	return evidence, nil
}

func probeAndRecord(ctx context.Context, prober antigravity.Prober, store *state.Store, job probeJob, now time.Time, modelGroup config.AntigravityModelGroup, sampleCapacity int) (priority.ProbeEvidence, error) {
	results := prober.ProbeAll(ctx, antigravity.ProbeRequest{
		AuthIndex:   job.credential.AuthIndex,
		AccessToken: job.authMaterial.accessToken,
		ProjectID:   job.authMaterial.projectID,
		ModelGroup:  modelGroup,
	})

	// Record all model groups from the single probe response (REQ-05: dual-group persistence).
	var primaryEvidence priority.ProbeEvidence
	var primaryErr error
	for group, result := range results {
		evidence, err := recordAntigravityProbeResult(ctx, store, result, now, sampleCapacity)
		if group == modelGroup {
			primaryEvidence = evidence
			primaryErr = err
		}
	}

	return primaryEvidence, primaryErr
}

func recordAntigravityProbeResult(ctx context.Context, store *state.Store, result antigravity.ProbeResult, now time.Time, sampleCapacity int) (priority.ProbeEvidence, error) {
	if result.Status != antigravity.StatusReady || result.ResetAt == nil || result.Remaining == nil {
		err := store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:   result.AuthIndex,
			Provider:    core.ProviderAntigravity,
			ModelGroup:  string(result.ModelGroup),
			ObservedAt:  now,
			Err:         errors.New(result.Error),
			NextProbeAt: now.Add(time.Hour),
		})
		return priority.ProbeEvidence{
			Provider:    core.ProviderAntigravity,
			AuthIndex:   result.AuthIndex,
			Freshness:   result.Freshness,
			ProbeStatus: result.ProbeStatus,
			Status:      priority.EvidenceStatusProbeFailed,
		}, err
	}

	err := store.MarkProbeSuccess(ctx, state.ProbeSuccess{
		AuthIndex:            result.AuthIndex,
		Provider:             core.ProviderAntigravity,
		ModelGroup:           string(result.ModelGroup),
		ObservedAt:           result.ObservedAt,
		ResetAt:              *result.ResetAt,
		Remaining:            *result.Remaining,
		ShortWindowResetAt:   timeOrZero(result.ShortWindowResetAt),
		ShortWindowRemaining: result.ShortWindowRemaining,
		LongWindowResetAt:    timeOrZero(result.LongWindowResetAt),
		LongWindowRemaining:  result.LongWindowRemaining,
		Source:               state.SourceFreshProbe,
		NextProbeAt:          result.ObservedAt.Add(time.Hour),
		SampleCapacity:       sampleCapacity,
	})

	cycleBurnRate := store.GetCycleBurnRate(result.AuthIndex, string(result.ModelGroup))

	return priority.ProbeEvidence{
		Provider:             core.ProviderAntigravity,
		AuthIndex:            result.AuthIndex,
		ObservedAt:           result.ObservedAt,
		ResetAt:              result.ResetAt,
		Remaining:            result.Remaining,
		ShortWindowResetAt:   result.ShortWindowResetAt,
		ShortWindowRemaining: result.ShortWindowRemaining,
		LongWindowResetAt:    result.LongWindowResetAt,
		LongWindowRemaining:  result.LongWindowRemaining,
		Freshness:            result.Freshness,
		ProbeStatus:          result.ProbeStatus,
		Status:               priority.EvidenceStatusReady,
		PlanType:             result.PlanType,
		EvidenceFresh:        true,
		CycleBurnRate:        cycleBurnRate,
	}, err
}

func cachedEvidenceFromEntry(entry state.Entry) priority.ProbeEvidence {
	// A cached entry with LastError set is a recorded probe failure — return it
	// as EvidenceStatusProbeFailed so withProbeFailureTemporaryDisables can handle it.
	if entry.LastError != "" {
		return priority.ProbeEvidence{
			Provider:    core.ProviderAntigravity,
			AuthIndex:   entry.AuthIndex,
			ObservedAt:  entry.ObservedAt,
			Freshness:   core.FreshnessUnknown,
			ProbeStatus: core.ProbeStatusUnknown,
			Status:      priority.EvidenceStatusProbeFailed,
		}
	}

	var resetAt *time.Time
	if !entry.ResetAt.IsZero() {
		r := entry.ResetAt
		resetAt = &r
	}
	var shortResetAt *time.Time
	if !entry.ShortWindowResetAt.IsZero() {
		r := entry.ShortWindowResetAt
		shortResetAt = &r
	}
	var longResetAt *time.Time
	if !entry.LongWindowResetAt.IsZero() {
		r := entry.LongWindowResetAt
		longResetAt = &r
	}

	remaining := entry.Remaining
	cycleBurnRate := entry.CycleBurnRate
	if cycleBurnRate <= 0 {
		cycleBurnRate = state.DefaultCycleBurnRate
	}

	return priority.ProbeEvidence{
		Provider:             core.ProviderAntigravity,
		AuthIndex:            entry.AuthIndex,
		ObservedAt:           entry.ObservedAt,
		ResetAt:              resetAt,
		Remaining:            &remaining,
		ShortWindowResetAt:   shortResetAt,
		ShortWindowRemaining: entry.ShortWindowRemaining,
		LongWindowResetAt:    longResetAt,
		LongWindowRemaining:  entry.LongWindowRemaining,
		Freshness:            core.FreshnessFresh,
		ProbeStatus:          core.ProbeStatusReady,
		Status:               priority.EvidenceStatusReady,
		PlanType:             entry.PlanType,
		EvidenceFresh:        true,
		CycleBurnRate:        cycleBurnRate,
	}
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

// alternateModelGroup returns the other model group.
func alternateModelGroup(group config.AntigravityModelGroup) config.AntigravityModelGroup {
	if group == config.AntigravityModelGroupClaudeGPT {
		return config.AntigravityModelGroupGemini
	}
	return config.AntigravityModelGroupClaudeGPT
}

// buildCachedEvidenceForGroup constructs ProbeEvidence for a model group from the state store cache.
// Used to build predicted priority snapshots for the alternate (non-primary) group.
func buildCachedEvidenceForGroup(store *state.Store, credentials []core.Credential, modelGroup string) []priority.ProbeEvidence {
	evidence := make([]priority.ProbeEvidence, 0, len(credentials))
	for _, cred := range credentials {
		entry, ok := store.GetEntry(cred.AuthIndex, modelGroup)
		if !ok || entry.SchemaVersion != state.SchemaVersion {
			continue
		}
		evidence = append(evidence, cachedEvidenceFromEntry(entry))
	}
	return evidence
}
