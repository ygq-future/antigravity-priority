package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"antigravity-priority/internal/host"
	"antigravity-priority/internal/provider/antigravity"
)

var antigravityQuotaURLs = []string{
	antigravity.RetrieveUserQuotaSummaryURL,
	antigravity.RetrieveUserQuotaSummaryDailyURL,
	antigravity.RetrieveUserQuotaSummarySandboxURL,
}

var antigravityAvailableModelsURLs = []string{
	antigravity.FetchAvailableModelsURL,
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:fetchAvailableModels",
	"https://cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels",
}

type antigravityQuotaRequest struct {
	AuthIndex   string
	AccessToken string
	ProjectID   string
	ObservedAt  time.Time
}

type quotaHTTPDoer interface {
	HTTPDoRaw(context.Context, host.HTTPRequest) (host.HTTPResponse, error)
}

func executeAntigravityQuotaRequest(ctx context.Context, doer quotaHTTPDoer, request antigravityQuotaRequest) map[antigravity.ModelGroup]antigravity.ProbeResult {
	// The quota summary is the only endpoint that returns canonical named buckets
	// (5h / weekly) for every model group, which is the evidence shape that yields
	// reset countdowns and an inferable plan tier. Available models only carries
	// per-model quotas without window names, so it may only supplement a group the
	// summary omits.
	results, lastStatus, lastErr := queryQuotaURLs(ctx, doer, antigravityQuotaURLs, request)

	if !hasCompleteQuota(results) {
		fallbackResults, fbStatus, fbErr := queryQuotaURLs(ctx, doer, antigravityAvailableModelsURLs, request)
		if fbStatus != 0 {
			lastStatus = fbStatus
		}
		if fbErr != nil && lastErr == nil {
			lastErr = fbErr
		}
		results = mergeQuotaResults(results, fallbackResults)
	}

	if results != nil {
		return results
	}
	message := fmt.Sprintf("retrieve quota summary status %d", lastStatus)
	if lastErr != nil && lastStatus == 0 {
		message = "host http do failed"
	}
	return failedAntigravityQuotaResults(request, message, lastStatus)
}

// hasCompleteQuota reports whether a probe response covers the pools that are
// universally present for Antigravity accounts. Gemini and Claude/GPT are always
// required. Argon is entitlement-gated: it can be absent even on a Pro plan, so
// its presence is captured opportunistically but presence is never required to
// finalize the probe. Treating plan type as proof of Argon entitlement would
// make every probe fan out across all fallback hosts for such accounts.
func hasCompleteQuota(results map[antigravity.ModelGroup]antigravity.ProbeResult) bool {
	if results == nil {
		return false
	}
	gemini, geminiOK := results[antigravity.ModelGroupGemini]
	claude, claudeOK := results[antigravity.ModelGroupClaudeGPT]
	if !geminiOK || gemini.Status != antigravity.StatusReady {
		return false
	}
	if !claudeOK || claude.Status != antigravity.StatusReady {
		return false
	}
	return true
}

func queryQuotaURLs(ctx context.Context, doer quotaHTTPDoer, urls []string, request antigravityQuotaRequest) (map[antigravity.ModelGroup]antigravity.ProbeResult, int, error) {
	var lastStatus int
	var lastErr error
	var merged map[antigravity.ModelGroup]antigravity.ProbeResult

	for _, url := range urls {
		response, err := postAntigravityQuota(ctx, doer, url, request)
		if err != nil {
			lastErr = err
			continue
		}
		lastStatus = response.StatusCode
		if response.StatusCode != http.StatusOK {
			continue
		}
		results := antigravity.ParseAllModelGroups(response.Body, request.ObservedAt.UTC())
		for group, result := range results {
			result.AuthIndex = request.AuthIndex
			result.HTTPStatusCode = response.StatusCode
			results[group] = result
		}
		merged = mergeQuotaResults(merged, results)
		// A 200 is not proof of completeness: hosts have answered with only a
		// subset of the model groups. Keep asking until every expected pool is
		// covered so a partial answer cannot silently hide a group's quota.
		if hasCompleteQuota(merged) {
			return merged, response.StatusCode, nil
		}
	}

	return merged, lastStatus, lastErr
}

func mergeQuotaResults(merged map[antigravity.ModelGroup]antigravity.ProbeResult, incoming map[antigravity.ModelGroup]antigravity.ProbeResult) map[antigravity.ModelGroup]antigravity.ProbeResult {
	if incoming == nil {
		return merged
	}
	if merged == nil {
		merged = make(map[antigravity.ModelGroup]antigravity.ProbeResult, len(incoming))
	}
	for group, result := range incoming {
		if existing, exists := merged[group]; !exists || existing.Status != antigravity.StatusReady {
			merged[group] = result
		}
	}
	return merged
}

func failedAntigravityQuotaResults(request antigravityQuotaRequest, message string, statusCode int) map[antigravity.ModelGroup]antigravity.ProbeResult {
	// Only the universal pools are synthesized here. Argon is entitlement-gated:
	// a total transport failure does not prove the account lacks Argon, so it must
	// not fabricate an Argon failure record. Distinguishing "no Argon entitlement"
	// from "Argon probe failed" is a separate status/semantic change.
	results := make(map[antigravity.ModelGroup]antigravity.ProbeResult, 2)
	for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
		result := antigravity.ParseAvailableModels(nil, request.ObservedAt.UTC(), group)
		result.AuthIndex = request.AuthIndex
		result.Error = message
		result.HTTPStatusCode = statusCode
		results[group] = result
	}
	return results
}

func antigravityQuotaBody(projectID string) []byte {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil
	}
	body, _ := json.Marshal(struct {
		Project string `json:"project"`
	}{Project: projectID})
	return body
}

// antigravityHubUserAgent identifies the desktop (hub) client. Google gates the
// separate Gemini 4 Argon weekly bucket on the client identity: the IDE family and
// older hub versions receive a summary without it, so the dashboard can never show
// Argon quota while the probe advertises a legacy identity.
const antigravityHubUserAgent = "antigravity/hub/2.23.0 darwin/arm64"

// antigravityLegacyHubUserAgent is the previously used identity. It is retried only
// after the current identity is rejected, so a hard upstream rejection cannot erase
// the shared Gemini/Claude windows that the legacy identity still serves.
const antigravityLegacyHubUserAgent = "antigravity/hub/2.9.1 darwin/arm64"

func postAntigravityQuota(ctx context.Context, doer quotaHTTPDoer, url string, request antigravityQuotaRequest) (host.HTTPResponse, error) {
	response, err := doer.HTTPDoRaw(ctx, host.HTTPRequest{
		AuthIndex: request.AuthIndex,
		Method:    http.MethodPost,
		URL:       url,
		Headers:   antigravityQuotaHeaders(request.AccessToken),
		Body:      antigravityQuotaBody(request.ProjectID),
	})
	if err != nil || response.StatusCode != http.StatusForbidden {
		return response, err
	}

	legacy, legacyErr := doer.HTTPDoRaw(ctx, host.HTTPRequest{
		AuthIndex: request.AuthIndex,
		Method:    http.MethodPost,
		URL:       url,
		Headers:   antigravityQuotaHeadersForUserAgent(request.AccessToken, antigravityLegacyHubUserAgent),
		Body:      antigravityQuotaBody(request.ProjectID),
	})
	if legacyErr != nil {
		// Keep the rejection status: the fallback must not mask it with a transport error.
		return response, nil
	}
	return legacy, nil
}

func antigravityQuotaHeaders(accessToken string) host.Header {
	return antigravityQuotaHeadersForUserAgent(accessToken, antigravityHubUserAgent)
}

func antigravityQuotaHeadersForUserAgent(accessToken string, userAgent string) host.Header {
	token := "$TOKEN$"
	if trimmed := strings.TrimSpace(accessToken); trimmed != "" {
		token = trimmed
	}
	return host.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer " + token},
		"Content-Type":  []string{"application/json"},
		"User-Agent":    []string{userAgent},
	}
}
