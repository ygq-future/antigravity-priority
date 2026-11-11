package runtime

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"antigravity-priority/internal/core"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/provider/antigravity"
	"antigravity-priority/internal/state"
)

const (
	planTierRefreshInterval = 24 * time.Hour
	planTierRetryInterval   = time.Hour
	planTierRequestTimeout  = 5 * time.Second
	planTierMaxWorkers      = 4
)

const antigravityLoadCodeAssistBody = `{"metadata":{"ideType":"ANTIGRAVITY"}}`

type planTierJob struct {
	credential  core.Credential
	accessToken string
}

type planTierResult struct {
	credential core.Credential
	planType   core.PlanType
	err        error
}

// refreshPlanTiers discovers the real subscription tier for accounts whose
// cached tier is missing or stale. It is best-effort: individual account
// failures update retry metadata but never fail scheduling.
func refreshPlanTiers(
	ctx context.Context,
	doer quotaHTTPDoer,
	credentials []core.Credential,
	materials map[string]authMaterial,
	store *state.Store,
	now time.Time,
	targetedSingleAuth bool,
	maxConcurrency int,
) (bool, error) {
	if doer == nil || store == nil {
		return false, nil
	}

	jobs := make([]planTierJob, 0, len(credentials))
	for _, credential := range credentials {
		if credential.Provider != core.ProviderAntigravity {
			continue
		}
		if credential.Disabled && !targetedSingleAuth {
			continue
		}
		if record, ok := store.GetPlanTier(credential.AuthIndex); ok && !planTierDue(record, now) {
			continue
		}
		material, ok := materials[credential.AuthIndex]
		if !ok {
			continue
		}
		jobs = append(jobs, planTierJob{credential: credential, accessToken: material.accessToken})
	}
	if len(jobs) == 0 {
		return false, nil
	}

	results := fetchPlanTierResults(ctx, doer, jobs, maxConcurrency)
	if err := ctx.Err(); err != nil {
		return false, err
	}

	for _, result := range results {
		record, _ := store.GetPlanTier(result.credential.AuthIndex)
		record.AttemptedAt = now.UTC()
		if result.err == nil {
			record.PlanType = result.planType
			record.ObservedAt = now.UTC()
		}
		store.SetPlanTier(result.credential.AuthIndex, record)
	}
	return true, nil
}

func fetchPlanTierResults(ctx context.Context, doer quotaHTTPDoer, jobs []planTierJob, maxConcurrency int) []planTierResult {
	results := make([]planTierResult, len(jobs))
	workers := max(1, min(maxConcurrency, planTierMaxWorkers, len(jobs)))

	if workers == 1 {
		for index, job := range jobs {
			results[index] = runPlanTierJob(ctx, doer, job)
		}
		return results
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobsCh := make(chan int)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for index := range jobsCh {
				if runCtx.Err() != nil {
					return
				}
				results[index] = runPlanTierJob(runCtx, doer, jobs[index])
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

	wg.Wait()
	return results
}

func runPlanTierJob(ctx context.Context, doer quotaHTTPDoer, job planTierJob) planTierResult {
	planType, err := fetchPlanTier(ctx, doer, job.credential.AuthIndex, job.accessToken)
	return planTierResult{
		credential: job.credential,
		planType:   planType,
		err:        err,
	}
}

func fetchPlanTier(ctx context.Context, doer quotaHTTPDoer, authIndex, accessToken string) (core.PlanType, error) {
	requestCtx, cancel := context.WithTimeout(ctx, planTierRequestTimeout)
	defer cancel()

	response, err := doer.HTTPDoRaw(requestCtx, host.HTTPRequest{
		AuthIndex: authIndex,
		Method:    http.MethodPost,
		URL:       antigravity.LoadCodeAssistDailyURL,
		Headers:   antigravityLoadCodeAssistHeaders(accessToken),
		Body:      []byte(antigravityLoadCodeAssistBody),
	})
	if err != nil {
		return core.PlanTypeUnknown, err
	}
	if response.StatusCode != http.StatusOK {
		return core.PlanTypeUnknown, fmt.Errorf("loadCodeAssist status %d", response.StatusCode)
	}

	return antigravity.ParseLoadCodeAssist(response.Body)
}

func antigravityLoadCodeAssistHeaders(accessToken string) host.Header {
	token := "$TOKEN$"
	if trimmed := strings.TrimSpace(accessToken); trimmed != "" {
		token = trimmed
	}
	return host.Header{
		"Accept":        []string{"*/*"},
		"Authorization": []string{"Bearer " + token},
		"Content-Type":  []string{"application/json"},
		"User-Agent":    []string{antigravityHubUserAgent},
	}
}

// planTierDue reports whether a loadCodeAssist refresh is due. Successful
// refreshes are cached for a day; failed attempts retry hourly without
// discarding the last known tier.
func planTierDue(record state.PlanTierRecord, now time.Time) bool {
	if record.ObservedAt.IsZero() {
		return record.AttemptedAt.IsZero() || now.Sub(record.AttemptedAt) >= planTierRetryInterval
	}
	if now.Sub(record.ObservedAt) < planTierRefreshInterval {
		return false
	}
	return record.AttemptedAt.IsZero() || now.Sub(record.AttemptedAt) >= planTierRetryInterval
}
