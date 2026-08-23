package runtime

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
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

type collectedEvidence struct {
	RoundID      string
	ByGroup      map[config.AntigravityModelGroup]evidence.Result
	Observations []evidence.Observation
	Probed       int
}

type probeJob struct {
	credential   core.Credential
	authMaterial authMaterial
}

var probeRoundSequence atomic.Uint64

func collectFreshEvidence(ctx context.Context, input collectInput) (collectedEvidence, error) {
	probes := make([]evidence.ProbeObservation, 0)
	jobs := make([]probeJob, 0, len(input.credentials))
	for _, cred := range input.credentials {
		needsProbe, err := freshProbeNeeded(ctx, input, cred.AuthIndex, string(input.modelGroup))
		if err != nil {
			return collectedEvidence{}, err
		}
		if needsProbe {
			jobs = append(jobs, probeJob{
				credential:   cred,
				authMaterial: input.authMaterials[cred.AuthIndex],
			})
		}
	}

	roundID := newProbeRoundID(input.now)
	if len(jobs) > 0 {
		probed, err := runProbeJobs(ctx, input, jobs, roundID)
		if err != nil {
			return collectedEvidence{}, err
		}
		probes = append(probes, probed...)
	}

	historical := historicalObservations(input.store, input.credentials)
	result := collectedEvidence{
		RoundID:      roundID,
		ByGroup:      make(map[config.AntigravityModelGroup]evidence.Result, 2),
		Observations: make([]evidence.Observation, 0),
		Probed:       uniqueProbeCredentials(probes),
	}
	for _, group := range []config.AntigravityModelGroup{
		config.AntigravityModelGroupGemini,
		config.AntigravityModelGroupClaudeGPT,
	} {
		classified := evidence.Classify(evidence.Input{
			Round:       evidence.Round{ID: roundID, ModelGroup: group},
			Credentials: input.credentials,
			Probes:      probes,
			Historical:  historical,
		})
		result.ByGroup[group] = classified
		result.Observations = append(result.Observations, classified.Observations...)
	}
	return result, nil
}

func newProbeRoundID(now time.Time) string {
	sequence := probeRoundSequence.Add(1)
	return now.UTC().Format(time.RFC3339Nano) + "-" + strconv.FormatUint(sequence, 10)
}

func uniqueProbeCredentials(probes []evidence.ProbeObservation) int {
	authIndexes := make(map[string]struct{}, len(probes))
	for _, probe := range probes {
		if probe.Result.AuthIndex != "" {
			authIndexes[probe.Result.AuthIndex] = struct{}{}
		}
	}
	return len(authIndexes)
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

func runProbeJobs(ctx context.Context, input collectInput, jobs []probeJob, roundID string) ([]evidence.ProbeObservation, error) {
	workers := input.maxConcurrency
	if workers < 1 {
		workers = 2
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}

	if workers == 1 {
		observations := make([]evidence.ProbeObservation, 0, len(jobs)*2)
		for _, job := range jobs {
			items, err := probeAndRecord(ctx, input.client, input.store, job, input.now, input.sampleCapacity)
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				observations = append(observations, evidence.ProbeObservation{RoundID: roundID, Result: item})
			}
		}
		return observations, nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		index int
		items []antigravity.ProbeResult
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
				items, err := probeAndRecord(runCtx, input.client, input.store, jobs[index], input.now, input.sampleCapacity)
				select {
				case resultsCh <- result{index: index, items: items, err: err}:
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

	ordered := make([][]antigravity.ProbeResult, len(jobs))
	var firstErr error
	for item := range resultsCh {
		if item.err != nil {
			if firstErr == nil {
				firstErr = item.err
			}
			cancel()
			continue
		}
		ordered[item.index] = item.items
	}
	if firstErr != nil {
		return nil, firstErr
	}

	observations := make([]evidence.ProbeObservation, 0, len(jobs)*2)
	for _, items := range ordered {
		for _, item := range items {
			observations = append(observations, evidence.ProbeObservation{RoundID: roundID, Result: item})
		}
	}
	return observations, nil
}

func probeAndRecord(ctx context.Context, client *host.Client, store *state.Store, job probeJob, now time.Time, sampleCapacity int) ([]antigravity.ProbeResult, error) {
	results := executeAntigravityQuotaRequest(ctx, client, antigravityQuotaRequest{
		AuthIndex:   job.credential.AuthIndex,
		AccessToken: job.authMaterial.accessToken,
		ProjectID:   job.authMaterial.projectID,
		ObservedAt:  now,
	})

	items := make([]antigravity.ProbeResult, 0, 2)
	for _, group := range []antigravity.ModelGroup{
		antigravity.ModelGroupGemini,
		antigravity.ModelGroupClaudeGPT,
	} {
		result, ok := results[group]
		if !ok {
			continue
		}
		if err := recordAntigravityProbeResult(ctx, store, result, now, sampleCapacity); err != nil {
			return nil, err
		}
		items = append(items, result)
	}
	return items, nil
}

func recordAntigravityProbeResult(ctx context.Context, store *state.Store, result antigravity.ProbeResult, now time.Time, sampleCapacity int) error {
	if result.Status != antigravity.StatusReady || result.ResetAt == nil || result.Remaining == nil {
		return store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:   result.AuthIndex,
			Provider:    core.ProviderAntigravity,
			ModelGroup:  string(result.ModelGroup),
			ObservedAt:  now,
			Err:         errors.New(result.Error),
			NextProbeAt: now.Add(time.Hour),
		})
	}

	return store.MarkProbeSuccess(ctx, state.ProbeSuccess{
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
}

func historicalObservations(store *state.Store, credentials []core.Credential) []evidence.HistoricalObservation {
	if store == nil {
		return nil
	}
	result := make([]evidence.HistoricalObservation, 0, len(credentials)*2)
	for _, credential := range credentials {
		for _, group := range []config.AntigravityModelGroup{
			config.AntigravityModelGroupGemini,
			config.AntigravityModelGroupClaudeGPT,
		} {
			entry, ok := store.GetEntry(credential.AuthIndex, string(group))
			if !ok {
				continue
			}
			historical := evidence.HistoricalObservation{
				AuthIndex:  credential.AuthIndex,
				ModelGroup: group,
				LastError:  entry.LastError,
				FailureAt:  entry.LastFailureAt,
			}
			quota, ok := store.GetHistoricalEvidence(credential.AuthIndex, string(group))
			if ok {
				historical.Evidence = quota
			}
			if !ok && historical.LastError == "" {
				continue
			}
			result = append(result, historical)
		}
	}
	return result
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
