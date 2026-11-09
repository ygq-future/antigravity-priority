package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
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

// WarmupResetGraceMaxJitter is the random jitter added to prevent multiple credentials from synchronizing simultaneously.
const WarmupResetGraceMaxJitter = 25 * time.Second

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
	if ev.ShortWindowRemaining != nil && *ev.ShortWindowRemaining >= 99 {
		hasFullRemaining = true
	} else if ev.Remaining != nil && *ev.Remaining >= 99 && ev.ShortWindowRemaining == nil {
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
type warmupResetTarget struct {
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

func (r *Runtime) runWarmupResetWatcher() {
	for {
		nextDeadline := r.nextWarmupResetDeadline()
		var timer *time.Timer
		var timerC <-chan time.Time
		if !nextDeadline.IsZero() {
			delay := nextDeadline.Sub(r.clock.Now().UTC())
			if delay < 0 {
				delay = 0
			}
			timer = time.NewTimer(delay)
			timerC = timer.C
		}
		select {
		case <-r.rootCtx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-r.warmupResetWake:
			if timer != nil {
				timer.Stop()
			}
			r.processWarmupResetDeadlines()
		case <-timerC:
			r.processWarmupResetDeadlines()
		}
	}
}

func (r *Runtime) nextWarmupResetDeadline() time.Time {
	r.warmupResetMu.Lock()
	defer r.warmupResetMu.Unlock()
	var next time.Time
	for _, target := range r.warmupResetTargets {
		if !target.resetAt.IsZero() {
			next = earlierTime(next, target.resetAt)
		}
	}
	return next
}

func (r *Runtime) processWarmupResetDeadlines() {
	now := r.clock.Now().UTC()
	var dueTargets []warmupResetTarget

	r.warmupResetMu.Lock()
	for key, target := range r.warmupResetTargets {
		if !target.resetAt.IsZero() && !target.resetAt.After(now) {
			dueTargets = append(dueTargets, target)
			delete(r.warmupResetTargets, key)
		}
	}
	r.warmupResetMu.Unlock()

	for _, target := range dueTargets {
		tAuth := target.authIndex
		tGroup := target.group
		go func() {
			_ = r.Probe(r.rootCtx, tGroup, []string{tAuth})
		}()
	}
}


// triggerWarmupForEligibleCredentials checks eligible full-quota credentials in configured model pools
// and kicks off the 5h rolling window by issuing a minimal token generation request.
func (r *Runtime) triggerWarmupForEligibleCredentials(
	ctx context.Context,
	doer quotaHTTPDoer,
	cfg config.WarmupConfig,
	evidenceByGroup map[config.AntigravityModelGroup]evidence.Result,
	authMaterials map[string]authMaterial,
	now time.Time,
) {
	if !cfg.GeminiEnabled && !cfg.ClaudeEnabled {
		return
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

			key := authIdx + "::" + string(target.group)
			r.warmupMu.Lock()
			lastWarmup := r.lastWarmupByAuthAndGroup[key]
			r.warmupMu.Unlock()
			// If credential is in an active countdown, schedule precision wake-up at its reset time
			resetAt := ev.ShortWindowResetAt
			if resetAt == nil {
				resetAt = ev.ResetAt
			}
			if resetAt != nil && resetAt.After(now) && resetAt.Sub(now) < WarmupResetThresholdDuration {
				jitter := time.Duration(rand.IntN(int(WarmupResetGraceMaxJitter/time.Second))) * time.Second
				graceDelay := WarmupResetGraceBaseDelay + jitter
				r.warmupResetMu.Lock()
				r.warmupResetTargets[key] = warmupResetTarget{
					authIndex: authIdx,
					group:     target.group,
					resetAt:   resetAt.Add(graceDelay),
				}
				r.warmupResetMu.Unlock()
				r.signalWarmupResetWatcher()
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
				r.warmupMu.Lock()
				r.lastWarmupByAuthAndGroup[key] = now
				r.warmupMu.Unlock()
			}
		}
	}
}
