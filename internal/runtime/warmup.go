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
	"strconv"
	"strings"
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
	"antigravity-priority/internal/evidence"
	"antigravity-priority/internal/host"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/provider/antigravity"
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

// geminiWarmupMinOutputTokens is the completion length requested from Gemini warm-ups.
// Gemini short-window accounting only registers measurable usage once the completion is
// substantial; the concise prompts that are enough for Claude/GPT stay too small here.
const geminiWarmupMinOutputTokens = 600

// GenerateDetailedGeminiWarmupPrompt creates a long-answer engineering query for Gemini pools.
func GenerateDetailedGeminiWarmupPrompt() string {
	lang := warmupLanguages[rand.IntN(len(warmupLanguages))]
	aspect := warmupAspects[rand.IntN(len(warmupAspects))]
	concept := warmupConcepts[rand.IntN(len(warmupConcepts))]
	return fmt.Sprintf(
		"In %s, explain in depth how %s works with respect to %s, with concrete examples. "+
			"Write at least %d tokens across several paragraphs. Do not summarize, do not shorten, and do not stop early.",
		lang, concept, aspect, geminiWarmupMinOutputTokens,
	)
}

// RenderWarmupPrompt renders the warm-up prompt for one quota pool.
func RenderWarmupPrompt(group config.AntigravityModelGroup) string {
	switch group {
	case config.AntigravityModelGroupGemini, config.AntigravityModelGroupArgon:
		return GenerateDetailedGeminiWarmupPrompt()
	default:
		return GenerateNaturalWarmupPrompt()
	}
}

// WarmupDebounceDuration is the minimum interval before a credential in the same pool can be warmed up again.
const WarmupDebounceDuration = 4*time.Hour + 30*time.Minute

// Reset warm-ups are serialized through one global lane so the 5h refresh
// points of different credentials never coincide. The gap depends on whether
// the next pool belongs to the same credential: pools of one account may warm
// 30-60 minutes apart, while switching to another credential waits 60-120.
const (
	WarmupLaneSameCredentialMinGap  = 30 * time.Minute
	WarmupLaneSameCredentialMaxGap  = 60 * time.Minute
	WarmupLaneCrossCredentialMinGap = 60 * time.Minute
	WarmupLaneCrossCredentialMaxGap = 120 * time.Minute
)

// warmupLaneGap returns the randomized delay before the next reset warm-up
// slot opens.
func warmupLaneGap(sameCredential bool) time.Duration {
	if sameCredential {
		return WarmupLaneSameCredentialMinGap + rand.N(WarmupLaneSameCredentialMaxGap-WarmupLaneSameCredentialMinGap)
	}
	return WarmupLaneCrossCredentialMinGap + rand.N(WarmupLaneCrossCredentialMaxGap-WarmupLaneCrossCredentialMinGap)
}

// WarmupResetGraceBaseDelay is the buffer delay after upstream reset timestamp to accommodate ledger convergence.
const WarmupResetGraceBaseDelay = 35 * time.Second

// WarmupResetThresholdDuration is the threshold reset-countdown duration indicating a full un-started 5h window.
const WarmupResetThresholdDuration = 4 * time.Hour

type antigravityGenerateEnvelope struct {
	Project     string                          `json:"project,omitempty"`
	Model       string                          `json:"model"`
	UserAgent   string                          `json:"userAgent,omitempty"`
	RequestType string                          `json:"requestType,omitempty"`
	RequestID   string                          `json:"requestId,omitempty"`
	Request     antigravityGenerateInnerRequest `json:"request"`
}

type antigravityGenerateInnerRequest struct {
	Contents         []antigravityContentPart   `json:"contents"`
	GenerationConfig antigravityGenerationParam `json:"generationConfig"`
	SessionID        string                     `json:"sessionId,omitempty"`
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

// warmupEnvelopeUserAgent is the top-level envelope field the desktop client sends for
// every generation request; it is separate from the HTTP User-Agent header.
const warmupEnvelopeUserAgent = "antigravity"

type warmupExecutionResult struct {
	StatusCode      int
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

type antigravityGenerateResponse struct {
	Response struct {
		UsageMetadata struct {
			PromptTokenCount        int `json:"promptTokenCount"`
			CandidatesTokenCount    int `json:"candidatesTokenCount"`
			ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
			CachedContentTokenCount int `json:"cachedContentTokenCount"`
			TotalTokenCount         int `json:"totalTokenCount"`
		} `json:"usageMetadata"`
	} `json:"response"`
}

// parseWarmupTokenUsage reports the consumption of one warm-up generation.
// Thinking models bill their reasoning as output tokens, so history must carry
// that bucket or the row shows 0 output next to a total that looks unexplained.
func parseWarmupTokenUsage(body []byte) warmupExecutionResult {
	var resp antigravityGenerateResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		usage := resp.Response.UsageMetadata
		result := warmupExecutionResult{
			StatusCode:      http.StatusOK,
			InputTokens:     int64(usage.PromptTokenCount),
			OutputTokens:    int64(usage.CandidatesTokenCount),
			ReasoningTokens: int64(usage.ThoughtsTokenCount),
			CachedTokens:    int64(usage.CachedContentTokenCount),
			TotalTokens:     int64(usage.TotalTokenCount),
		}
		if result.ReasoningTokens == 0 {
			if diff := result.TotalTokens - result.InputTokens - result.OutputTokens; diff > 0 {
				result.ReasoningTokens = diff
			}
		}
		if result.TotalTokens == 0 {
			result.TotalTokens = result.InputTokens + result.OutputTokens + result.ReasoningTokens
		}
		if result.TotalTokens > 0 {
			return result
		}
	}
	return warmupExecutionResult{
		StatusCode:   http.StatusOK,
		InputTokens:  2,
		OutputTokens: 8,
		TotalTokens:  10,
	}
}

// warmupWeeklyExhausted reports whether an exhausted 7d pool blocks a warm-up.
// Both the watcher candidate pass and the execution-time eligibility check use
// the same rule; a nil reading means "unknown" and does not block.
func warmupWeeklyExhausted(remaining *int64) bool {
	return remaining != nil && *remaining <= 0
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

	// 2. Weekly pool must still have quota: one warm-up consumes both pools, so
	// an exhausted 7d window makes the request pointless.
	if warmupWeeklyExhausted(ev.LongWindowRemaining) {
		return false
	}

	// 3. Quota remaining must be at full capacity (100% or >= 99%)
	hasFullRemaining := false
	if ev.ShortWindowRemaining != nil && *ev.ShortWindowRemaining >= warmupFullRemainingPct {
		hasFullRemaining = true
	} else if ev.Remaining != nil && *ev.Remaining >= warmupFullRemainingPct && ev.ShortWindowRemaining == nil {
		hasFullRemaining = true
	}
	if !hasFullRemaining {
		return false
	}

	// 4. Reset countdown must indicate that the 5h window is un-started or fully elapsed
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

// executeAntigravityWarmup sends a consumer-shaped generation request that kicks off the 5h window.
func executeAntigravityWarmup(ctx context.Context, doer quotaHTTPDoer, authIndex, accessToken, projectID, model, prompt string) (warmupExecutionResult, error) {
	reqBody := antigravityGenerateEnvelope{
		Project:     strings.TrimSpace(projectID),
		Model:       model,
		UserAgent:   warmupEnvelopeUserAgent,
		RequestType: "agent",
		RequestID:   newWarmupRequestID(),
		Request: antigravityGenerateInnerRequest{
			SessionID: newWarmupSessionID(),
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

	resp, err := doer.HTTPDoRaw(ctx, host.HTTPRequest{
		AuthIndex: authIndex,
		Method:    http.MethodPost,
		URL:       antigravity.GenerateContentDailyURL,
		Headers:   antigravityQuotaHeadersForUserAgent(accessToken, antigravityWarmupUserAgent),
		Body:      bodyBytes,
	})
	if err != nil {
		return warmupExecutionResult{}, err
	}
	if resp.StatusCode == http.StatusOK {
		return warmupResultFromBody(resp.Body), nil
	}
	if resp.StatusCode == http.StatusForbidden {
		// The legacy identity is retried only after the consumer identity is rejected.
		legacyResp, legacyErr := doer.HTTPDoRaw(ctx, host.HTTPRequest{
			AuthIndex: authIndex,
			Method:    http.MethodPost,
			URL:       antigravity.GenerateContentDailyURL,
			Headers:   antigravityQuotaHeadersForUserAgent(accessToken, antigravityLegacyHubUserAgent),
			Body:      bodyBytes,
		})
		if legacyErr == nil && legacyResp.StatusCode == http.StatusOK {
			return warmupResultFromBody(legacyResp.Body), nil
		}
	}
	return warmupExecutionResult{StatusCode: resp.StatusCode}, nil
}

// warmupResultFromBody reports the token usage of a successful warm-up generation.
func warmupResultFromBody(body []byte) warmupExecutionResult {
	return parseWarmupTokenUsage(body)
}

// newWarmupRequestID builds the per-request identifier the consumer client sends.
func newWarmupRequestID() string {
	return fmt.Sprintf("agent-%016x%016x", rand.Uint64(), rand.Uint64())
}

// newWarmupSessionID builds the conversation identifier the consumer client sends.
func newWarmupSessionID() string {
	return "-" + strconv.FormatUint(rand.Uint64()&0x7FFFFFFFFFFFFFFF, 10)
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

// probeAttempt tracks retry state for the reset-boundary evidence refresh of one
// credential pool, so a non-converging upstream cannot turn the watcher into a
// probe loop. Warm-up dispatch itself keeps no failure state.
type probeAttempt struct {
	resetAt   time.Time
	attempts  int
	nextAt    time.Time
	exhausted bool
}

// warmupCandidate is a credential pool whose 5h window is due for a warm-up probe.
type warmupCandidate struct {
	key       string
	authIndex string
	group     config.AntigravityModelGroup
	resetAt   time.Time
}

func (r *Runtime) signalWarmupResetWatcher() {
	select {
	case r.warmupResetWake <- struct{}{}:
	default:
	}
}

// runWarmupResetWatcher re-derives warm-up deadlines from persisted quota
// evidence instead of relying on an in-flight scheduling run, so a paused auto
// schedule (or a restart) cannot suppress a 5h window warm-up. The global lane
// stored alongside the runtime document spreads those warm-ups apart.
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

// sweepWarmupResets refreshes every credential once when a 5h reset boundary
// passes, then handles warm-up-only candidates. It returns how long the
// watcher should wait before sweeping again.
func (r *Runtime) sweepWarmupResets(ctx context.Context) time.Duration {
	if ctx.Err() != nil {
		return WarmupSweepInterval
	}
	cfg, err := r.Config()
	if err != nil {
		return WarmupSweepInterval
	}
	if !cfg.Enabled {
		return WarmupSweepInterval
	}
	store, err := r.state.Store(ctx)
	if err != nil {
		return WarmupSweepInterval
	}
	now := r.clock.Now().UTC()

	groups := warmupCandidateGroups(cfg.Warmup)
	maxWait := WarmupSweepInterval
	if len(groups) == 0 {
		maxWait = warmupIdleSweepInterval
	}

	refresh := r.dueResetRefreshCandidates(store, now)
	warm := r.dueWarmupCandidates(store, cfg.Warmup, now)
	if len(refresh) == 0 && len(warm) == 0 {
		return nextResetBoundaryWait(store, now, maxWait)
	}
	disabled := r.disabledAuthIndexes(ctx)
	if len(disabled) > 0 {
		warm = slices.DeleteFunc(warm, func(candidate warmupCandidate) bool {
			_, isDisabled := disabled[candidate.authIndex]
			return isDisabled
		})
	}

	// At most one reset warm-up may leave per lane slot, and the slot is
	// reserved before the probe so no other pool can warm in the same window.
	target, hasTarget := r.allocateWarmupSlot(ctx, store, warm, now)
	candidates := refresh
	if hasTarget {
		candidates = mergeCandidates(refresh, []warmupCandidate{target})
	}
	if len(candidates) == 0 {
		return nextResetBoundaryWait(store, now, maxWait)
	}

	// One targeted probe refreshes every pool owned by a credential, so an
	// auth index is probed once even when several of its pools are due.
	byAuth := make(map[string][]warmupCandidate)
	authOrder := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if _, seen := byAuth[candidate.authIndex]; !seen {
			authOrder = append(authOrder, candidate.authIndex)
		}
		byAuth[candidate.authIndex] = append(byAuth[candidate.authIndex], candidate)
	}
	for _, authIndex := range authOrder {
		if ctx.Err() != nil {
			return WarmupSweepInterval
		}
		if _, isDisabled := disabled[authIndex]; isDisabled {
			continue
		}
		due := byAuth[authIndex]
		probeCandidate := due[0]
		scope := WarmupScope{Suppress: true}
		isTarget := hasTarget && authIndex == target.authIndex
		if isTarget {
			probeCandidate = target
			scope = WarmupScope{Group: target.group}
		}
		for _, candidate := range due {
			r.recordProbeAttempt(candidate, now)
		}
		if err := r.probeResetCandidate(ctx, probeCandidate, scope); errors.Is(err, ErrRunInProgress) {
			// Another run owns the lock; do not spend a retry budget on it.
			for _, candidate := range due {
				r.clearProbeAttempt(candidate.key)
			}
		}
	}
	return nextResetBoundaryWait(store, now, WarmupSweepInterval)
}

// allocateWarmupSlot decides whether the global lane admits one reset warm-up.
// The gap is drawn once per pending pool and persisted, so per-minute sweeps
// cannot re-roll a shorter wait. The slot is reserved before the probe is
// issued, so a restart or a slow upstream can never let two pools warm inside
// the same window.
func (r *Runtime) allocateWarmupSlot(ctx context.Context, store *state.Store, candidates []warmupCandidate, now time.Time) (warmupCandidate, bool) {
	if len(candidates) == 0 {
		return warmupCandidate{}, false
	}
	target := candidates[0]
	lane := store.GetWarmupLane()

	gate := lane.NextAt
	if lane.NextAuth != target.authIndex || gate.IsZero() {
		gate = lane.LastAt.Add(warmupLaneGap(lane.LastAuth == target.authIndex))
	}

	// A deadline farther out than one maximum gap can only come from a backward
	// clock correction; treat it as stale instead of starving warm-ups.
	if !lane.LastAt.IsZero() && now.Before(gate) && gate.Sub(now) <= WarmupLaneCrossCredentialMaxGap {
		if lane.NextAuth != target.authIndex || lane.NextAt.IsZero() {
			lane.NextAt = gate
			lane.NextAuth = target.authIndex
			if err := r.state.SetWarmupLane(ctx, lane); err != nil {
				log.Printf("[antigravity-priority] 5h warmup lane reservation failed: %v", err)
				return warmupCandidate{}, false
			}
		}
		return warmupCandidate{}, false
	}

	if err := r.state.SetWarmupLane(ctx, state.WarmupLane{LastAt: now, LastAuth: target.authIndex}); err != nil {
		log.Printf("[antigravity-priority] 5h warmup lane reservation failed: %v", err)
		return warmupCandidate{}, false
	}
	return target, true
}
// dueResetRefreshCandidates lists every 5h pool whose reset grace deadline has
// passed. Argon shares Gemini's physical 5h window, so scanning it would count
// one credential boundary twice. The refresh probe itself still covers every
// credential and every pool.
func (r *Runtime) dueResetRefreshCandidates(store *state.Store, now time.Time) []warmupCandidate {
	return r.collectCandidates(store, now, candidateFilter{
		groups: []config.AntigravityModelGroup{
			config.AntigravityModelGroupGemini,
			config.AntigravityModelGroupClaudeGPT,
		},
		due: func(_ state.Entry, resetAt, now time.Time) bool {
			return resetBoundaryDue(resetAt, now)
		},
		attemptBlocked: r.probeAttemptBlocked,
	})
}

// dueWarmupCandidates lists the pools whose 5h window is due for a warm-up probe.
func (r *Runtime) dueWarmupCandidates(store *state.Store, warmup config.WarmupConfig, now time.Time) []warmupCandidate {
	return r.collectCandidates(store, now, candidateFilter{
		groups:          warmupCandidateGroups(warmup),
		respectDebounce: true,
		excludeFreePlan: true,
		due:             warmupWindowDue,
		allow: func(authIndex string, group config.AntigravityModelGroup) bool {
			return warmupEffectiveEnabled(warmup, authIndex, group)
		},
	})
}

// warmupCandidateGroups lists the pools the watcher may probe: globally enabled
// pools plus any pool that a card-level override may force on.
func warmupCandidateGroups(warmup config.WarmupConfig) []config.AntigravityModelGroup {
	overrideOn := warmup.HasEnabledOverride()
	groups := make([]config.AntigravityModelGroup, 0, 2)
	if warmup.GeminiEnabled || overrideOn {
		groups = append(groups, config.AntigravityModelGroupGemini)
	}
	if warmup.ClaudeEnabled || overrideOn {
		groups = append(groups, config.AntigravityModelGroupClaudeGPT)
	}
	return groups
}

// candidateFilter selects persisted credential pools for one watcher pass.
type candidateFilter struct {
	groups          []config.AntigravityModelGroup
	respectDebounce bool
	excludeFreePlan bool
	due             func(entry state.Entry, resetAt, now time.Time) bool
	allow           func(authIndex string, group config.AntigravityModelGroup) bool
	attemptBlocked  func(key string, resetAt, now time.Time) bool
}

// collectCandidates owns cooldown, debounce, and retry suppression so reset
// refreshes and warm-ups share one selection path.
func (r *Runtime) collectCandidates(store *state.Store, now time.Time, filter candidateFilter) []warmupCandidate {
	if store == nil {
		return nil
	}
	enabled := make(map[config.AntigravityModelGroup]struct{}, len(filter.groups))
	cooldowns := make(map[config.AntigravityModelGroup]map[string]state.CooldownEntry, len(filter.groups))
	for _, group := range filter.groups {
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
		if filter.allow != nil && !filter.allow(entry.AuthIndex, group) {
			continue
		}
		if filter.excludeFreePlan && entry.PlanType == core.PlanTypeFree {
			continue
		}
		resetAt := entry.ShortWindowResetAt
		if resetAt.IsZero() {
			resetAt = entry.ResetAt
		}
		if resetAt.IsZero() {
			continue
		}
		key := warmupCandidateKey(entry.AuthIndex, string(group))
		lastWarmup := store.GetLastWarmupAt(entry.AuthIndex, entry.ModelGroup)
		if filter.respectDebounce && !lastWarmup.IsZero() && now.Sub(lastWarmup) < WarmupDebounceDuration {
			continue
		}
		if !filter.due(entry, resetAt, now) {
			continue
		}
		if _, inCooldown := cooldowns[group][entry.AuthIndex]; inCooldown {
			continue
		}
		if filter.attemptBlocked != nil && filter.attemptBlocked(key, resetAt, now) {
			continue
		}
		candidates = append(candidates, warmupCandidate{
			key:       key,
			authIndex: entry.AuthIndex,
			group:     group,
			resetAt:   resetAt,
		})
	}
	slices.SortFunc(candidates, func(left, right warmupCandidate) int {
		return strings.Compare(left.key, right.key)
	})
	return candidates
}

// mergeCandidates deduplicates candidate lists by credential pool and keeps a
// deterministic probe order.
func mergeCandidates(lists ...[]warmupCandidate) []warmupCandidate {
	merged := make([]warmupCandidate, 0)
	seen := make(map[string]struct{})
	for _, list := range lists {
		for _, candidate := range list {
			if _, ok := seen[candidate.key]; ok {
				continue
			}
			seen[candidate.key] = struct{}{}
			merged = append(merged, candidate)
		}
	}
	slices.SortFunc(merged, func(left, right warmupCandidate) int {
		return strings.Compare(left.key, right.key)
	})
	return merged
}

// nextResetBoundaryWait returns the delay until the earliest pending 5h reset
// grace deadline, capped at maxWait. Waking at the deadline keeps the refresh
// probe inside the grace window instead of waiting for the next full sweep.
func nextResetBoundaryWait(store *state.Store, now time.Time, maxWait time.Duration) time.Duration {
	if store == nil {
		return maxWait
	}
	wait := maxWait
	for _, entry := range store.Entries() {
		resetAt := entry.ShortWindowResetAt
		if resetAt.IsZero() {
			resetAt = entry.ResetAt
		}
		if resetAt.IsZero() {
			continue
		}
		until := resetAt.Add(WarmupResetGraceBaseDelay).Sub(now)
		if until > 0 && until < wait {
			wait = until
		}
	}
	if wait < time.Second {
		return time.Second
	}
	return wait
}

// resetBoundaryDue reports whether a 5h reset has passed the ledger
// convergence grace, which is when a refresh probe must observe the new
// window. A full window that never started also lands here once its stale
// reset timestamp elapses.
func resetBoundaryDue(resetAt, now time.Time) bool {
	return !now.Before(resetAt.Add(WarmupResetGraceBaseDelay))
}

// warmupWindowDue reports whether the stored 5h window either already reset or
// is a full, un-started window that should be kicked off now.
func warmupWindowDue(entry state.Entry, resetAt, now time.Time) bool {
	// A warm-up consumes the weekly pool too; never probe a 5h window when the
	// 7d quota is already exhausted.
	if warmupWeeklyExhausted(entry.LongWindowRemaining) {
		return false
	}
	if resetBoundaryDue(resetAt, now) {
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

// probeAttemptBlocked bounds the evidence-refresh loop for a reset boundary the
// upstream never converges. It gates probes only; warm-up dispatch keeps no
// attempt state.
func (r *Runtime) probeAttemptBlocked(key string, resetAt, now time.Time) bool {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	attempt, ok := r.probeAttempts[key]
	if !ok || !attempt.resetAt.Equal(resetAt) {
		return false
	}
	return attempt.exhausted || now.Before(attempt.nextAt)
}

func (r *Runtime) recordProbeAttempt(candidate warmupCandidate, now time.Time) {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	attempt := r.probeAttempts[candidate.key]
	if !attempt.resetAt.Equal(candidate.resetAt) {
		attempt = probeAttempt{resetAt: candidate.resetAt}
	}
	attempt.attempts++
	if attempt.attempts >= warmupResetMaxAttempts {
		attempt.exhausted = true
	} else {
		attempt.nextAt = now.Add(warmupRetryDelay(attempt.attempts))
	}
	r.probeAttempts[candidate.key] = attempt
}

func (r *Runtime) clearProbeAttempt(key string) {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	delete(r.probeAttempts, key)
}

// warmupRetryDelay backs off repeated evidence-refresh probes at one reset boundary.
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

// probeResetCandidate runs the targeted probe that refreshes one credential's
// quota evidence. The scope decides whether that probe may warm one specific
// pool, while lane-spacing probes only refresh evidence. Warm-up failures keep
// no state: the next natural candidate pass decides what happens next.
func (r *Runtime) probeResetCandidate(ctx context.Context, candidate warmupCandidate, scope WarmupScope) error {
	return r.runTaskRequest(ctx, TaskRequest{
		Trigger:     TriggerProbe,
		AuthIndexes: []string{candidate.authIndex},
		Warmup:      scope,
	})
}

// warmupCandidateKey identifies retry state for one credential pool.
func warmupCandidateKey(authIndex, modelGroup string) string {
	return strings.TrimSpace(authIndex) + "::" + strings.TrimSpace(modelGroup)
}

// disabledAuthIndexes returns host-disabled credentials, which background
// refresh and warm-up must never probe.
func (r *Runtime) disabledAuthIndexes(ctx context.Context) map[string]struct{} {
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


// triggerWarmupForEligibleCredentials checks eligible full-quota credentials in configured model pools
// and kicks off the 5h rolling window by issuing a minimal token generation request.
// It returns the number of successful warm-ups so the caller can persist them.
func (r *Runtime) triggerWarmupForEligibleCredentials(
	ctx context.Context,
	doer quotaHTTPDoer,
	cfg config.WarmupConfig,
	groupFilter config.AntigravityModelGroup,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
	authMaterials map[string]authMaterial,
	disabledAuthIndexes map[string]struct{},
	store *state.Store,
	now time.Time,
) int {
	if !cfg.GeminiEnabled && !cfg.ClaudeEnabled && !cfg.HasEnabledOverride() {
		return 0
	}
	cfg.Normalize()

	type targetTask struct {
		group config.AntigravityModelGroup
		model string
	}
	// Both pools are considered; the per-credential effective state below
	// decides whether a globally disabled pool still warms a local override.
	targets := []targetTask{
		{group: config.AntigravityModelGroupGemini, model: cfg.GeminiModel},
		{group: config.AntigravityModelGroupClaudeGPT, model: cfg.ClaudeModel},
	}
	if groupFilter != "" {
		targets = slices.DeleteFunc(targets, func(target targetTask) bool {
			return target.group != groupFilter
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
			if ev.PlanType == core.PlanTypeFree {
				continue
			}
			if !warmupEffectiveEnabled(cfg, authIdx, target.group) {
				continue
			}
			if len(disabledAuthIndexes) > 0 {
				if _, isDisabled := disabledAuthIndexes[authIdx]; isDisabled {
					continue
				}
			}

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

			prompt := RenderWarmupPrompt(target.group)
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
				ID:              fmt.Sprintf("warmup-%s-%d", authIdx, now.UnixNano()),
				Source:          state.RequestHistorySourceWarmup,
				At:              now,
				Model:           target.model,
				ModelPool:       string(target.group),
				AuthIndex:       authIdx,
				Email:           maskedEmail,
				InputTokens:     res.InputTokens,
				OutputTokens:    res.OutputTokens,
				ReasoningTokens: res.ReasoningTokens,
				CachedTokens:    res.CachedTokens,
				TotalTokens:     res.TotalTokens,
				Failed:          res.StatusCode != http.StatusOK || err != nil,
				StatusCode:      res.StatusCode,
				ErrorMessage:    errMsg,
			})

			log.Printf("[antigravity-priority] 5h warmup: auth=%s (%s) pool=%s model=%s status=%d err=%v in=%d out=%d think=%d total=%d",
				authIdx, maskedEmail, target.group, target.model, res.StatusCode, err, res.InputTokens, res.OutputTokens, res.ReasoningTokens, res.TotalTokens)

			if err == nil && res.StatusCode == http.StatusOK {
				warmed++
				if store != nil {
					store.SetWarmupCompleted(authIdx, string(target.group), now)
				}
			}
		}
	}
	return warmed
}
