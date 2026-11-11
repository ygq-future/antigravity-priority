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
	client             *host.Client
	store              *state.Store
	credentials        []core.Credential
	authMaterials      map[string]authMaterial
	now                time.Time
	targetedSingleAuth bool
	maxConcurrency     int
	sampleCapacity     int
}

type collectedEvidence struct {
	RoundID      string
	ByGroup      map[config.AntigravityModelGroup]evidence.Result
	Observations []evidence.Observation
	Probed       int
	RateLimited  map[string]string
	Probes       []evidence.ProbeObservation
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
		if freshProbeNeeded(cred, input.targetedSingleAuth) {
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
		ByGroup:      make(map[config.AntigravityModelGroup]evidence.Result, len(config.AllAntigravityModelGroups)),
		Observations: make([]evidence.Observation, 0),
		Probed:       uniqueProbeCredentials(probes),
		RateLimited:  rateLimitedCredentials(probes),
	}
	for _, group := range config.AllAntigravityModelGroups {
		classified := evidence.Classify(evidence.Input{
			Round:       evidence.Round{ID: roundID, ModelGroup: group},
			Credentials: input.credentials,
			Probes:      probes,
			Historical:  historical,
		})
		classified = enrichFreshCycleBurnRate(input.store, classified)
		result.ByGroup[group] = classified
		result.Observations = append(result.Observations, classified.Observations...)
	}
	result.Probes = probes
	return result, nil
}

func rateLimitedCredentials(probes []evidence.ProbeObservation) map[string]string {
	result := make(map[string]string)
	for _, probe := range probes {
		if probe.Result.HTTPStatusCode == 429 {
			result[probe.Result.AuthIndex] = string(probe.Result.ModelGroup)
		}
	}
	return result
}

// enrichFreshCycleBurnRate makes the in-memory result use the same learned
// rate that buildProjectionEvidence will use after the state write. Without
// this boundary, a fresh probe can be planned with the cold-start default and
// the next dashboard sync can plan the identical quota with the learned rate.
func enrichFreshCycleBurnRate(store *state.Store, result evidence.Result) evidence.Result {
	if store == nil {
		return result
	}
	for index := range result.Eligible {
		rate := store.GetCycleBurnRate(result.Eligible[index].AuthIndex, string(result.Eligible[index].ModelGroup))
		result.Eligible[index].CycleBurnRate = rate
	}
	for index := range result.Observations {
		observation := &result.Observations[index]
		if observation.Kind != evidence.ObservationFresh || observation.Evidence == nil {
			continue
		}
		observation.Evidence.CycleBurnRate = store.GetCycleBurnRate(observation.AuthIndex, string(observation.ModelGroup))
	}
	return result
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

func freshProbeNeeded(cred core.Credential, targetedSingleAuth bool) bool {
	if !cred.Disabled {
		return true
	}
	// Bypassed: disabled credentials on the host must never be probed during
	// batch operations (auto-scheduling or "Probe All").
	// Targeted single-credential probe allows viewing quota on demand.
	return targetedSingleAuth
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
			items, err := probeAndRecord(ctx, input.client, input.store, job, input.now, input.sampleCapacity, roundID)
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
	for range workers {
		wg.Go(func() {
			for index := range jobsCh {
				if runCtx.Err() != nil {
					return
				}
				items, err := probeAndRecord(runCtx, input.client, input.store, jobs[index], input.now, input.sampleCapacity, roundID)
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
		})
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

func probeAndRecord(ctx context.Context, client *host.Client, store *state.Store, job probeJob, now time.Time, sampleCapacity int, roundID string) ([]antigravity.ProbeResult, error) {
	results := executeAntigravityQuotaRequest(ctx, client, antigravityQuotaRequest{
		AuthIndex:   job.credential.AuthIndex,
		AccessToken: job.authMaterial.accessToken,
		ProjectID:   job.authMaterial.projectID,
		ObservedAt:  now,
	})
	// The planner prefers ProbeResult.PlanType over credential.PlanType, so
	// propagate the authoritative account tier into every group result before
	// it becomes quota evidence. The per-group cache remains a fallback for
	// accounts whose tier has not been discovered yet.
	if store != nil {
		if tier, ok := store.GetPlanTier(job.credential.AuthIndex); ok &&
			tier.PlanType != "" && tier.PlanType != core.PlanTypeUnknown {
			for group, result := range results {
				result.PlanType = tier.PlanType
				results[group] = result
			}
		}
	}
	applySharedArgonFallback(results)
	items := make([]antigravity.ProbeResult, 0, len(config.AllAntigravityModelGroups))
	for _, group := range config.AllAntigravityModelGroups {
		result, ok := results[antigravity.ModelGroup(group)]
		if !ok {
			continue
		}
		if err := recordAntigravityProbeResult(ctx, store, result, now, sampleCapacity, roundID); err != nil {
			return nil, err
		}
		items = append(items, result)
	}
	return items, nil
}

// applySharedArgonFallback reuses the measured Gemini pool for Ultra and
// Ultra-lite accounts. Those plans run Gemini 4 Argon against the shared Gemini
// 5h/weekly buckets, so upstream omits an independent Argon bucket. Treating
// that absence as a failed or unavailable pool would soft-downgrade Argon
// routing even though the account can serve it. PlanType is supplied by the
// loadCodeAssist overlay above; while it is unknown, the pool stays unavailable
// and the next probe retries rather than guessing entitlement.
func applySharedArgonFallback(results map[antigravity.ModelGroup]antigravity.ProbeResult) {
	argon, hasArgon := results[antigravity.ModelGroupArgon]
	gemini, hasGemini := results[antigravity.ModelGroupGemini]
	if !hasArgon || argon.Status == antigravity.StatusReady || !hasGemini || gemini.Status != antigravity.StatusReady {
		return
	}
	if gemini.PlanType != core.PlanTypeUltra && gemini.PlanType != core.PlanTypeUltraLite {
		return
	}

	shared := gemini
	shared.ModelGroup = antigravity.ModelGroupArgon
	shared.ArgonWindowResetAt = nil
	shared.ArgonWindowRemaining = nil
	shared.Error = ""
	results[antigravity.ModelGroupArgon] = shared
}

func recordAntigravityProbeResult(ctx context.Context, store *state.Store, result antigravity.ProbeResult, now time.Time, sampleCapacity int, roundID string) error {
	switch {
	case result.Status == antigravity.StatusUnavailable:
		return store.MarkProbeUnavailable(ctx, state.ProbeUnavailable{
			AuthIndex:  result.AuthIndex,
			Provider:   core.ProviderAntigravity,
			ModelGroup: string(result.ModelGroup),
			ObservedAt: now,
			Reason:     result.Error,
		})
	case result.Status != antigravity.StatusReady || result.ResetAt == nil || result.Remaining == nil:
		return store.MarkProbeFailure(ctx, state.ProbeFailure{
			AuthIndex:  result.AuthIndex,
			Provider:   core.ProviderAntigravity,
			ModelGroup: string(result.ModelGroup),
			ObservedAt: now,
			Err:        errors.New(result.Error),
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
		ArgonWindowResetAt:   timeOrZero(result.ArgonWindowResetAt),
		ArgonWindowRemaining: result.ArgonWindowRemaining,
		PlanType:             result.PlanType,
		SampleCapacity:       sampleCapacity,
		ProbeRoundID:         roundID,
	})
}

func historicalObservations(store *state.Store, credentials []core.Credential) []evidence.HistoricalObservation {
	if store == nil {
		return nil
	}
	result := make([]evidence.HistoricalObservation, 0, len(credentials)*2)
	for _, credential := range credentials {
		for _, group := range config.AllAntigravityModelGroups {
			entry, ok := store.GetEntry(credential.AuthIndex, string(group))
			if !ok {
				continue
			}
			historical := evidence.HistoricalObservation{
				AuthIndex:         credential.AuthIndex,
				ModelGroup:        group,
				LastError:         entry.LastError,
				FailureAt:         entry.LastFailureAt,
				Unavailable:       entry.Unavailable,
				UnavailableAt:     entry.UnavailableAt,
				UnavailableReason: entry.UnavailableReason,
			}
			quota, ok := store.GetHistoricalEvidence(credential.AuthIndex, string(group))
			if ok {
				historical.Evidence = quota
			}
			if !ok && historical.LastError == "" && !historical.Unavailable {
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
