package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"slices"
	"strings"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

var warmupLanguages = []string{
	"Go", "TypeScript", "Python", "Rust", "Java", "C++", "Kotlin", "Swift", "PostgreSQL", "Docker",
}

var warmupConcepts = []string{
	"connection pooling", "idempotency keys", "LRU cache", "goroutine pools",
	"mutex lock contention", "database transactions", "JWT token verification",
	"rate limiting algorithms", "graceful process shutdown", "exponential backoff",
	"atomic compare-and-swap", "memory leak profiling", "HTTP/2 stream multiplexing",
	"structured concurrency", "zero-copy serialization", "distributed tracing spans",
	"circuit breaker pattern", "deadlock detection", "event loop lag",
}

var warmupAspects = []string{
	"throughput and latency", "concurrency safety", "memory overhead",
	"error propagation", "resource cleanup", "high availability",
	"preventing race conditions", "code maintainability", "scalability bottlenecks",
	"deterministic execution",
}

var warmupTasks = []string{
	"implement reliable health check endpoints",
	"safely drain worker queues on SIGTERM",
	"efficiently stream and parse large JSON payloads",
	"paginate large database query sets without offset drift",
	"handle transient upstream network timeouts",
	"verify HMAC signatures on incoming webhooks",
	"reconnect exponential backoff loops on TCP drop",
}

// GenerateNaturalWarmupPrompt creates a varied, realistic engineering query to trigger quota warmup.
func GenerateNaturalWarmupPrompt() string {
	lang := warmupLanguages[rand.IntN(len(warmupLanguages))]
	aspect := warmupAspects[rand.IntN(len(warmupAspects))]
	cIdx := rand.IntN(len(warmupConcepts))
	concept := warmupConcepts[cIdx]

	templateType := rand.IntN(6)
	switch templateType {
	case 0:
		return fmt.Sprintf("In %s, briefly explain the core mechanism of %s focusing on %s in 1-2 concise sentences.", lang, concept, aspect)
	case 1:
		return fmt.Sprintf("What is an idiomatic best practice for %s in %s? Summarize briefly in two sentences with respect to %s.", concept, lang, aspect)
	case 2:
		c2Idx := (cIdx + 1 + rand.IntN(len(warmupConcepts)-1)) % len(warmupConcepts)
		concept2 := warmupConcepts[c2Idx]
		return fmt.Sprintf("In %s, briefly compare %s with %s regarding %s in one short paragraph.", lang, concept, concept2, aspect)
	case 3:
		return fmt.Sprintf("Mention one subtle pitfall when dealing with %s in %s and how to prevent it, focusing on %s.", concept, lang, aspect)
	case 4:
		task := warmupTasks[rand.IntN(len(warmupTasks))]
		return fmt.Sprintf("Describe an idiomatic approach to %s in %s considering %s in 1-2 sentences.", task, lang, aspect)
	default:
		return fmt.Sprintf("How does %s optimize %s in high-load %s services? Give a concise one-sentence takeaway.", concept, aspect, lang)
	}
}

// RenderWarmupPrompt renders a natural warmup prompt.
func RenderWarmupPrompt(_ config.WarmupConfig, _ string) string {
	return GenerateNaturalWarmupPrompt()
}

// WarmupDebounceDuration is the minimum interval before a credential in the same pool can be warmed up again.
const WarmupDebounceDuration = 4*time.Hour + 30*time.Minute

// WarmupResetGraceBaseDelay is the buffer delay after upstream reset timestamp to accommodate ledger convergence.
const WarmupResetGraceBaseDelay = 35 * time.Second

// WarmupResetThresholdDuration is the threshold reset-countdown duration indicating a full un-started 5h window.
const WarmupResetThresholdDuration = 4 * time.Hour

var antigravityWarmupURLs = []string{
	"https://cloudcode-pa.googleapis.com/v1internal:generateContent",
	"https://daily-cloudcode-pa.googleapis.com/v1internal:generateContent",
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:generateContent",
}

type antigravityGenerateEnvelope struct {
	Project string                          `json:"project,omitempty"`
	Model   string                          `json:"model"`
	Request antigravityGenerateInnerRequest `json:"request"`
}

type antigravityGenerateInnerRequest struct {
	Contents         []antigravityContentPart   `json:"contents"`
	GenerationConfig antigravityGenerationParam `json:"generationConfig"`
}

type antigravityContentPart struct {
	Role  string                 `json:"role"`
	Parts []antigravityPartEntry `json:"parts"`
}

type antigravityPartEntry struct {
	Text string `json:"text"`
}

type antigravityGenerationParam struct {
	Temperature float64 `json:"temperature,omitempty"`
}
type warmupExecutionResult struct {
	StatusCode   int
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}

type antigravityGenerateResponse struct {
	Response struct {
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
			TotalTokenCount      int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	} `json:"response"`
}

func parseWarmupTokenUsage(body []byte) (int64, int64, int64) {
	var resp antigravityGenerateResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		in := int64(resp.Response.UsageMetadata.PromptTokenCount)
		out := int64(resp.Response.UsageMetadata.CandidatesTokenCount)
		total := int64(resp.Response.UsageMetadata.TotalTokenCount)
		if total == 0 && (in > 0 || out > 0) {
			total = in + out
		}
		if total > 0 {
			return in, out, total
		}
	}
	return 2, 8, 10
}

// isWarmupEligible evaluates whether a credential's evidence meets 5h full-quota and unstarted window criteria.
func isWarmupEligible(ev *priority.QuotaEvidence, now time.Time, lastWarmup time.Time) bool {
	if ev == nil {
		return false
	}

	// 1. Debounce protection: must have passed at least WarmupDebounceDuration since last warm-up
	if !lastWarmup.IsZero() && now.Sub(lastWarmup) < WarmupDebounceDuration {
		return false
	}

	// 2. Quota remaining must be at full capacity (100% or >= 99%)
	hasFullRemaining := false
	if ev.ShortWindowRemaining != nil && *ev.ShortWindowRemaining >= warmupFullRemainingPct {
		hasFullRemaining = true
	} else if ev.Remaining != nil && *ev.Remaining >= warmupFullRemainingPct && ev.ShortWindowRemaining == nil {
		hasFullRemaining = true
	}
	if !hasFullRemaining {
		return false
	}

	// 3. Reset countdown must indicate that the 5h window is un-started or fully elapsed
	resetAt := ev.ShortWindowResetAt
	if resetAt == nil {
		resetAt = ev.ResetAt
	}
	if resetAt == nil || resetAt.IsZero() {
		// If reset time is unspecified while remaining is 100%, upstream hasn't locked the window
		return true
	}

	// If reset time is in the past, previous window elapsed and new window is un-started
	if !resetAt.After(now) {
		return true
	}

	// If remaining duration is essentially the full 5 hours (>= 4h50m), window is idle
	remainingDuration := resetAt.Sub(now)
	return remainingDuration >= WarmupResetThresholdDuration
}

// executeAntigravityWarmup sends an unconstrained natural content generation request to kick off the 5h window.
func executeAntigravityWarmup(ctx context.Context, doer quotaHTTPDoer, authIndex, accessToken, projectID, model, prompt string) (warmupExecutionResult, error) {
	reqBody := antigravityGenerateEnvelope{
		Project: strings.TrimSpace(projectID),
		Model:   model,
		Request: antigravityGenerateInnerRequest{
			Contents: []antigravityContentPart{
				{
					Role: "user",
					Parts: []antigravityPartEntry{
						{Text: prompt},
					},
				},
			},
			GenerationConfig: antigravityGenerationParam{
				Temperature: 0.7,
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return warmupExecutionResult{}, fmt.Errorf("marshal warmup request: %w", err)
	}

	var lastStatus int
	var lastErr error

	for _, url := range antigravityWarmupURLs {
		resp, err := doer.HTTPDoRaw(ctx, host.HTTPRequest{
			AuthIndex: authIndex,
			Method:    http.MethodPost,
			URL:       url,
			Headers:   antigravityQuotaHeaders(accessToken),
			Body:      bodyBytes,
		})
		if err != nil {
			lastErr = err
			continue
		}
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			in, out, total := parseWarmupTokenUsage(resp.Body)
			return warmupExecutionResult{
				StatusCode:   resp.StatusCode,
				InputTokens:  in,
				OutputTokens: out,
				TotalTokens:  total,
			}, nil
		}

		if resp.StatusCode == http.StatusForbidden {
			// Upstream client identity fallback retry
			legacyResp, legacyErr := doer.HTTPDoRaw(ctx, host.HTTPRequest{
				AuthIndex: authIndex,
				Method:    http.MethodPost,
				URL:       url,
				Headers:   antigravityQuotaHeadersForUserAgent(accessToken, antigravityLegacyHubUserAgent),
				Body:      bodyBytes,
			})
			if legacyErr == nil && legacyResp.StatusCode == http.StatusOK {
				in, out, total := parseWarmupTokenUsage(legacyResp.Body)
				return warmupExecutionResult{
					StatusCode:   legacyResp.StatusCode,
					InputTokens:  in,
					OutputTokens: out,
					TotalTokens:  total,
				}, nil
			}
		}
	}

	return warmupExecutionResult{StatusCode: lastStatus}, lastErr
}

// WarmupSweepInterval is how often the warm-up watcher re-evaluates persisted
// quota evidence for 5h windows that are due to be warmed up.
const WarmupSweepInterval = time.Minute

// warmupIdleSweepInterval is the re-evaluation cadence while warm-up is disabled.
const warmupIdleSweepInterval = 5 * time.Minute

// warmupResetMaxAttempts bounds how many probes one 5h reset may trigger when
// the upstream window has not converged yet.
const warmupResetMaxAttempts = 5

// warmupFullRemainingPct is the remaining percentage that counts as a full,
// warmable 5h window.
const warmupFullRemainingPct = 99

// warmupAttempt tracks retry state for one credential pool window so a
// non-converging upstream cannot turn the watcher into a probe loop.
type warmupAttempt struct {
	resetAt   time.Time
	attempts  int
	nextAt    time.Time
	exhausted bool
}

// warmupCandidate is a credential pool whose 5h window is due for a warm-up probe.
type warmupCandidate struct {
	key        string
	authIndex  string
	group      config.AntigravityModelGroup
	resetAt    time.Time
	lastWarmup time.Time
}

func (r *Runtime) signalWarmupResetWatcher() {
	select {
	case r.warmupResetWake <- struct{}{}:
	default:
	}
}

// runWarmupResetWatcher re-derives warm-up deadlines from persisted quota
// evidence instead of relying on an in-flight scheduling run, so a paused auto
// schedule (or a restart) cannot suppress a 5h window warm-up.
func (r *Runtime) runWarmupResetWatcher() {
	for {
		wait := r.sweepWarmupResets(r.rootCtx)
		timer := time.NewTimer(wait)
		select {
		case <-r.rootCtx.Done():
			timer.Stop()
			return
		case <-r.warmupResetWake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
	}
}

// enabledWarmupGroups returns the quota pools the operator enabled warm-up for.
func enabledWarmupGroups(warmup config.WarmupConfig) []config.AntigravityModelGroup {
	groups := make([]config.AntigravityModelGroup, 0, 2)
	if warmup.GeminiEnabled {
		groups = append(groups, config.AntigravityModelGroupGemini)
	}
	if warmup.ClaudeEnabled {
		groups = append(groups, config.AntigravityModelGroupClaudeGPT)
	}
	return groups
}

// sweepWarmupResets probes every credential pool whose persisted 5h window is
// due, then returns how long the watcher should wait before sweeping again.
func (r *Runtime) sweepWarmupResets(ctx context.Context) time.Duration {
	if ctx.Err() != nil {
		return warmupIdleSweepInterval
	}
	cfg, err := r.Config()
	if err != nil {
		return warmupIdleSweepInterval
	}
	groups := enabledWarmupGroups(cfg.Warmup)
	if len(groups) == 0 {
		r.clearWarmupAttempts()
		return warmupIdleSweepInterval
	}
	if !cfg.Enabled {
		return WarmupSweepInterval
	}
	store, err := state.Load(ctx, warmupStatePath(cfg))
	if err != nil {
		return WarmupSweepInterval
	}
	now := r.clock.Now().UTC()
	candidates := r.dueWarmupCandidates(store, groups, now)
	if len(candidates) == 0 {
		return WarmupSweepInterval
	}
	disabled := r.disabledWarmupAuthIndexes(ctx)
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return WarmupSweepInterval
		}
		if _, isDisabled := disabled[candidate.authIndex]; isDisabled {
			continue
		}
		r.recordWarmupAttempt(candidate, now)
		if err := r.probeWarmupCandidate(ctx, cfg, candidate); errors.Is(err, ErrRunInProgress) {
			// Another run owns the lock; do not spend a retry budget on it.
			r.clearWarmupAttempt(candidate.key)
		}
	}
	return WarmupSweepInterval
}

// dueWarmupCandidates lists the pools whose 5h window is due for a warm-up probe.
func (r *Runtime) dueWarmupCandidates(store *state.Store, groups []config.AntigravityModelGroup, now time.Time) []warmupCandidate {
	if store == nil {
		return nil
	}
	enabled := make(map[config.AntigravityModelGroup]struct{}, len(groups))
	cooldowns := make(map[config.AntigravityModelGroup]map[string]state.CooldownEntry, len(groups))
	for _, group := range groups {
		enabled[group] = struct{}{}
		cooldowns[group] = store.GetActiveCooldownEntriesForGroup(now, string(group))
	}
	candidates := make([]warmupCandidate, 0)
	for _, entry := range store.Entries() {
		group := config.AntigravityModelGroup(entry.ModelGroup)
		if _, ok := enabled[group]; !ok {
			continue
		}
		if strings.TrimSpace(entry.AuthIndex) == "" {
			continue
		}
		resetAt := entry.ShortWindowResetAt
		if resetAt.IsZero() {
			resetAt = entry.ResetAt
		}
		if resetAt.IsZero() {
			continue
		}
		key := warmupAttemptKey(entry.AuthIndex, string(group))
		lastWarmup := store.GetLastWarmupAt(entry.AuthIndex, entry.ModelGroup)
		if !lastWarmup.IsZero() && now.Sub(lastWarmup) < WarmupDebounceDuration {
			continue
		}
		if !warmupWindowDue(entry, resetAt, now) {
			continue
		}
		if _, inCooldown := cooldowns[group][entry.AuthIndex]; inCooldown {
			continue
		}
		if r.warmupAttemptBlocked(key, resetAt, now) {
			continue
		}
		candidates = append(candidates, warmupCandidate{
			key:        key,
			authIndex:  entry.AuthIndex,
			group:      group,
			resetAt:    resetAt,
			lastWarmup: lastWarmup,
		})
	}
	slices.SortFunc(candidates, func(left, right warmupCandidate) int {
		return strings.Compare(left.key, right.key)
	})
	return candidates
}

// warmupWindowDue reports whether the stored 5h window either already reset or
// is a full, un-started window that should be kicked off now.
func warmupWindowDue(entry state.Entry, resetAt, now time.Time) bool {
	if !now.Before(resetAt.Add(WarmupResetGraceBaseDelay)) {
		return true
	}
	if resetAt.Sub(now) < WarmupResetThresholdDuration {
		return false
	}
	remaining := entry.Remaining
	if entry.ShortWindowRemaining != nil {
		remaining = *entry.ShortWindowRemaining
	}
	return remaining >= warmupFullRemainingPct
}

func (r *Runtime) warmupAttemptBlocked(key string, resetAt, now time.Time) bool {
	r.warmupResetMu.Lock()
	defer r.warmupResetMu.Unlock()
	attempt, ok := r.warmupAttempts[key]
	if !ok || !attempt.resetAt.Equal(resetAt) {
		return false
	}
	return attempt.exhausted || now.Before(attempt.nextAt)
}

func (r *Runtime) recordWarmupAttempt(candidate warmupCandidate, now time.Time) {
	r.warmupResetMu.Lock()
	defer r.warmupResetMu.Unlock()
	attempt := r.warmupAttempts[candidate.key]
	if !attempt.resetAt.Equal(candidate.resetAt) {
		attempt = warmupAttempt{resetAt: candidate.resetAt}
	}
	attempt.attempts++
	if attempt.attempts >= warmupResetMaxAttempts {
		attempt.exhausted = true
	} else {
		attempt.nextAt = now.Add(warmupRetryDelay(attempt.attempts))
	}
	r.warmupAttempts[candidate.key] = attempt
}

func (r *Runtime) clearWarmupAttempt(key string) {
	r.warmupResetMu.Lock()
	defer r.warmupResetMu.Unlock()
	delete(r.warmupAttempts, key)
}

func (r *Runtime) clearWarmupAttempts() {
	r.warmupResetMu.Lock()
	defer r.warmupResetMu.Unlock()
	clear(r.warmupAttempts)
}

// warmupRetryDelay backs off retries after a reset whose warm-up did not land.
func warmupRetryDelay(attempts int) time.Duration {
	delay := WarmupResetGraceBaseDelay
	for i := 1; i < attempts; i++ {
		delay *= 2
		if delay >= 10*time.Minute {
			return 10 * time.Minute
		}
	}
	return delay
}

// probeWarmupCandidate runs the targeted probe that refreshes evidence and
// performs the warm-up, then clears retry state once the warm-up landed.
func (r *Runtime) probeWarmupCandidate(ctx context.Context, cfg config.Config, candidate warmupCandidate) error {
	if err := r.Probe(ctx, []string{candidate.authIndex}); err != nil {
		return err
	}
	store, err := state.Load(ctx, warmupStatePath(cfg))
	if err != nil {
		return err
	}
	if store.GetLastWarmupAt(candidate.authIndex, string(candidate.group)).After(candidate.lastWarmup) {
		r.clearWarmupAttempt(candidate.key)
	}
	return nil
}

// warmupAttemptKey identifies retry state for one credential pool.
func warmupAttemptKey(authIndex, modelGroup string) string {
	return strings.TrimSpace(authIndex) + "::" + strings.TrimSpace(modelGroup)
}

// disabledWarmupAuthIndexes returns host-disabled credentials, which background
// warm-up must never probe.
func (r *Runtime) disabledWarmupAuthIndexes(ctx context.Context) map[string]struct{} {
	if r.hostCallbacks == nil {
		return nil
	}
	files, err := host.NewClient(r.hostCallbacks).ListAuthFiles(ctx)
	if err != nil {
		return nil
	}
	disabled := make(map[string]struct{})
	for _, file := range files {
		if file.Disabled && strings.TrimSpace(file.AuthIndex) != "" {
			disabled[file.AuthIndex] = struct{}{}
		}
	}
	return disabled
}

func warmupStatePath(cfg config.Config) string {
	if strings.TrimSpace(cfg.StateCachePath) == "" {
		return config.DefaultStateCachePath
	}
	return cfg.StateCachePath
}

// triggerWarmupForEligibleCredentials checks eligible full-quota credentials in configured model pools
// and kicks off the 5h rolling window by issuing a minimal token generation request.
// It returns the number of successful warm-ups so the caller can persist them.
func (r *Runtime) triggerWarmupForEligibleCredentials(
	ctx context.Context,
	doer quotaHTTPDoer,
	cfg config.WarmupConfig,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
	authMaterials map[string]authMaterial,
	disabledAuthIndexes map[string]struct{},
	store *state.Store,
	now time.Time,
) int {
	if !cfg.GeminiEnabled && !cfg.ClaudeEnabled {
		return 0
	}
	cfg.Normalize()

	type targetTask struct {
		group config.AntigravityModelGroup
		model string
	}
	var targets []targetTask
	if cfg.GeminiEnabled {
		targets = append(targets, targetTask{
			group: config.AntigravityModelGroupGemini,
			model: cfg.GeminiModel,
		})
	}
	if cfg.ClaudeEnabled {
		targets = append(targets, targetTask{
			group: config.AntigravityModelGroupClaudeGPT,
			model: cfg.ClaudeModel,
		})
	}

	warmed := 0
	for _, target := range targets {
		groupRes, ok := evidenceByGroup[target.group]
		if !ok {
			continue
		}
		for _, ev := range groupRes.Eligible {
			authIdx := ev.AuthIndex
			if authIdx == "" {
				continue
			}
			if len(disabledAuthIndexes) > 0 {
				if _, isDisabled := disabledAuthIndexes[authIdx]; isDisabled {
					continue
				}
			}

			key := warmupAttemptKey(authIdx, string(target.group))
			var lastWarmup time.Time
			if store != nil {
				lastWarmup = store.GetLastWarmupAt(authIdx, string(target.group))
			}
			if !isWarmupEligible(&ev, now, lastWarmup) {
				continue
			}

			material, hasMaterial := authMaterials[authIdx]
			if !hasMaterial {
				continue
			}

			prompt := RenderWarmupPrompt(cfg, "")
			res, err := executeAntigravityWarmup(ctx, doer, authIdx, material.accessToken, material.projectID, target.model, prompt)

			errMsg := ""
			if err != nil {
				errMsg = fmt.Sprintf("5h warmup error: %v", err)
			} else if res.StatusCode != http.StatusOK {
				errMsg = fmt.Sprintf("5h warmup HTTP %d", res.StatusCode)
			}

			maskedEmail := redactRuntimeIdentifier(material.email)
			if maskedEmail == "" {
				maskedEmail = redactRuntimeIdentifier(authIdx)
			}

			r.appendRequestHistory(state.RequestHistoryEntry{
				ID:           fmt.Sprintf("warmup-%s-%d", authIdx, now.UnixNano()),
				At:           now,
				Model:        target.model,
				ModelPool:    string(target.group),
				AuthIndex:    authIdx,
				Email:        maskedEmail,
				InputTokens:  res.InputTokens,
				OutputTokens: res.OutputTokens,
				TotalTokens:  res.TotalTokens,
				Failed:       res.StatusCode != http.StatusOK || err != nil,
				StatusCode:   res.StatusCode,
				ErrorMessage: errMsg,
			})

			log.Printf("[antigravity-priority] 5h warmup: auth=%s (%s) pool=%s model=%s status=%d err=%v in=%d out=%d total=%d",
				authIdx, maskedEmail, target.group, target.model, res.StatusCode, err, res.InputTokens, res.OutputTokens, res.TotalTokens)

			if err == nil && res.StatusCode == http.StatusOK {
				warmed++
				if store != nil {
					store.SetWarmupCompleted(authIdx, string(target.group), now)
				}
				r.clearWarmupAttempt(key)
			}
		}
	}
	return warmed
}
