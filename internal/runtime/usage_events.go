package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/priority"
	"antigravity-priority/internal/state"
)

type usageFailure struct {
	StatusCode int    `json:"StatusCode"`
	Body       string `json:"Body"`
}

type usageDetail struct {
	InputTokens         int64 `json:"InputTokens"`
	OutputTokens        int64 `json:"OutputTokens"`
	ReasoningTokens     int64 `json:"ReasoningTokens"`
	CachedTokens        int64 `json:"CachedTokens"`
	CacheReadTokens     int64 `json:"CacheReadTokens"`
	CacheCreationTokens int64 `json:"CacheCreationTokens"`
	TotalTokens         int64 `json:"TotalTokens"`
}

type usageEvent struct {
	Provider  string       `json:"Provider"`
	Model     string       `json:"Model"`
	AuthID    string       `json:"AuthID"`
	AuthIndex string       `json:"AuthIndex"`
	Failed    bool         `json:"Failed"`
	Failure   usageFailure `json:"Failure"`
	Detail    usageDetail  `json:"Detail"`
}

func (r *Runtime) handleUsageEvent(ctx context.Context, raw []byte) []byte {
	var payload usageEvent
	if err := json.Unmarshal(raw, &payload); err != nil {
		return failure(fmt.Errorf("decode usage event: %w", err))
	}
	if !isAntigravityUsageProvider(payload.Provider) {
		return mustMarshal(Envelope{OK: true})
	}
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthID)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}
	resolvedIndex := r.router.ResolveAuthIndex(authIndex, nil)
	email := r.resolveAuthEmail(resolvedIndex)
	pool := config.ClassifyModelQuotaPool(payload.Model)
	reasoningTokens := payload.Detail.ReasoningTokens
	cacheTokens := payload.Detail.CacheReadTokens
	if cacheTokens == 0 {
		cacheTokens = payload.Detail.CachedTokens
	}
	totalTokens := payload.Detail.TotalTokens
	if totalTokens == 0 {
		totalTokens = payload.Detail.InputTokens + payload.Detail.OutputTokens + reasoningTokens + cacheTokens
	} else if reasoningTokens == 0 && cacheTokens == 0 {
		diff := totalTokens - (payload.Detail.InputTokens + payload.Detail.OutputTokens)
		if diff > 0 {
			reasoningTokens = diff
		}
	}
	kind := state.FailureKind("")
	if payload.Failed {
		kind = failureKindFor(payload.Failure.StatusCode, payload.Failure.Body)
	}
	statusCode := 200
	if payload.Failed {
		switch {
		case payload.Failure.StatusCode != 0:
			statusCode = payload.Failure.StatusCode
		case kind == state.FailureUnauthorized:
			statusCode = http.StatusUnauthorized
		case kind == state.FailureForbidden:
			statusCode = http.StatusForbidden
		case kind == state.FailureRateLimit:
			statusCode = http.StatusTooManyRequests
		default:
			statusCode = http.StatusInternalServerError
		}
	}
	now := r.clock.Now().UTC()
	r.recordRequestActivity(now)
	r.appendRequestHistory(state.RequestHistoryEntry{
		ID:                  fmt.Sprintf("req-%d", now.UnixNano()),
		At:                  now,
		Model:               payload.Model,
		ModelPool:           string(pool),
		AuthIndex:           resolvedIndex,
		Email:               email,
		InputTokens:         payload.Detail.InputTokens,
		OutputTokens:        payload.Detail.OutputTokens,
		ReasoningTokens:     reasoningTokens,
		CachedTokens:        payload.Detail.CachedTokens,
		CacheReadTokens:     payload.Detail.CacheReadTokens,
		CacheCreationTokens: payload.Detail.CacheCreationTokens,
		TotalTokens:         totalTokens,
		Failed:              payload.Failed,
		StatusCode:          statusCode,
		ErrorMessage:        requestHistoryErrorMessage(payload.Failed, statusCode, payload.Failure.Body),
	})

	if !payload.Failed {
		if err := r.observeUsageSuccess(ctx, resolvedIndex, string(pool), payload.Model, payload.Detail); err != nil {
			return failure(err)
		}
		return mustMarshal(Envelope{OK: true})
	}

	switch kind {
	case state.FailureRateLimit:
		if err := r.observe429(ctx, resolvedIndex, string(pool), payload.Model, true); err != nil {
			return failure(err)
		}
	case state.FailureUnauthorized:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureUnauthorized,
			StatusCode:   http.StatusUnauthorized,
			Reason:       priority.ReasonUnauthorizedCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureForbidden:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureForbidden,
			StatusCode:   http.StatusForbidden,
			Reason:       priority.ReasonForbiddenCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureServerError:
		status := payload.Failure.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			ModelGroup:   string(pool),
			ProbeModel:   payload.Model,
			Kind:         state.FailureServerError,
			StatusCode:   status,
			Reason:       fmt.Sprintf("%d server error cooldown", status),
			Cooldown:     serverErrorBaseCooldown,
			ProbeEnabled: true,
		}, true); err != nil {
			return failure(err)
		}
	}
	return mustMarshal(Envelope{OK: true})
}

func isAntigravityUsageProvider(provider string) bool {
	p := strings.ToLower(strings.TrimSpace(provider))
	return p == "antigravity" || p == "google" || p == "gemini" || p == "google-antigravity" || strings.Contains(p, "antigravity")
}

func failureKindFor(statusCode int, body string) state.FailureKind {
	switch statusCode {
	case http.StatusUnauthorized:
		return state.FailureUnauthorized
	case http.StatusForbidden:
		return state.FailureForbidden
	case http.StatusTooManyRequests:
		return state.FailureRateLimit
	}
	if statusCode >= 500 && statusCode <= 599 {
		return state.FailureServerError
	}
	if statusCode != 0 {
		return ""
	}
	upper := strings.ToUpper(body)
	switch {
	case strings.Contains(upper, "UNAUTHENTICATED") || strings.Contains(upper, "401"):
		return state.FailureUnauthorized
	case strings.Contains(upper, "PERMISSION_DENIED") || strings.Contains(upper, "403"):
		return state.FailureForbidden
	case strings.Contains(upper, "RESOURCE_EXHAUSTED") || strings.Contains(upper, "RATE_LIMIT") || strings.Contains(upper, "429"):
		return state.FailureRateLimit
	case strings.Contains(upper, "UNAVAILABLE") || strings.Contains(upper, "500") || strings.Contains(upper, "502") ||
		strings.Contains(upper, "503") || strings.Contains(upper, "504"):
		return state.FailureServerError
	default:
		return ""
	}
}

func (r *Runtime) handleFilterEvent(ctx context.Context, raw []byte) []byte {
	var payload struct {
		AuthIndex  string `json:"auth_index"`
		AuthName   string `json:"auth_name"`
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
		Model      string `json:"model"`
		ModelGroup string `json:"model_group"`
	}
	_ = json.Unmarshal(raw, &payload)
	authIndex := firstNonEmpty(payload.AuthIndex, payload.AuthName)
	if authIndex == "" {
		return mustMarshal(Envelope{OK: true})
	}
	resolvedIndex := r.router.ResolveAuthIndex(authIndex, nil)
	model := strings.TrimSpace(payload.Model)
	modelGroup := strings.TrimSpace(payload.ModelGroup)
	if modelGroup == "" && model != "" {
		modelGroup = string(config.ClassifyModelQuotaPool(model))
	}
	kind := failureKindFor(payload.StatusCode, payload.Error)
	// Without a pool or model the failure cannot be isolated safely; only 401 and 403
	// are unambiguously account-wide.
	if modelGroup == "" && kind != state.FailureUnauthorized && kind != state.FailureForbidden {
		return mustMarshal(Envelope{OK: true})
	}

	switch kind {
	case state.FailureRateLimit:
		if err := r.observe429(ctx, resolvedIndex, modelGroup, model, true); err != nil {
			return failure(err)
		}
	case state.FailureUnauthorized:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureUnauthorized,
			StatusCode:   http.StatusUnauthorized,
			Reason:       priority.ReasonUnauthorizedCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureForbidden:
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			Kind:         state.FailureForbidden,
			StatusCode:   http.StatusForbidden,
			Reason:       priority.ReasonForbiddenCooldown,
			Cooldown:     unauthorizedQuarantine,
			ProbeEnabled: false,
		}, true); err != nil {
			return failure(err)
		}
	case state.FailureServerError:
		status := payload.StatusCode
		if status == 0 {
			status = http.StatusInternalServerError
		}
		if err := r.observeFailure(ctx, failureInput{
			AuthIndex:    resolvedIndex,
			ModelGroup:   modelGroup,
			ProbeModel:   model,
			Kind:         state.FailureServerError,
			StatusCode:   status,
			Reason:       fmt.Sprintf("%d server error cooldown", status),
			Cooldown:     serverErrorBaseCooldown,
			ProbeEnabled: true,
		}, true); err != nil {
			return failure(err)
		}
	}

	return mustMarshal(Envelope{OK: true})
}
