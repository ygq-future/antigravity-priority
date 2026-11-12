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
	// The quota summary is the single consumer endpoint that returns canonical named
	// buckets (5h / weekly), which is the evidence shape that yields reset countdowns
	// and an inferable plan tier. Groups the response omits stay unobserved: they are
	// reported as failures instead of being filled from a payload that has no window
	// names and cannot be verified against the same quota.
	response, err := postAntigravityQuota(ctx, doer, antigravity.RetrieveUserQuotaSummaryDailyURL, request)
	if err != nil {
		return failedAntigravityQuotaResults(request, "host http do failed", 0)
	}
	if response.StatusCode != http.StatusOK {
		return failedAntigravityQuotaResults(request, fmt.Sprintf("retrieve quota summary status %d", response.StatusCode), response.StatusCode)
	}
	results := antigravity.ParseAllModelGroups(response.Body, request.ObservedAt.UTC())
	for group, result := range results {
		result.AuthIndex = request.AuthIndex
		result.HTTPStatusCode = response.StatusCode
		results[group] = result
	}
	return results
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

// antigravityHubUserAgent identifies the desktop (hub) client. Google gates both
// the measured Gemini windows and the separate Gemini 4 Argon weekly bucket on the
// client identity: hub 1.0.0 receives only Claude/GPT on the daily host, while the
// current 2.24.0 identity receives the measured Gemini, Argon, and Claude/GPT groups.
const antigravityHubUserAgent = "antigravity/hub/2.24.0 darwin/arm64"

// antigravityWarmupUserAgent mirrors the identity the host sends for real consumer
// generation traffic, including the metadata fields the desktop client reports.
// Warm-up generations must look like that traffic to be accounted against the same
// measured quota.
const antigravityWarmupUserAgent = "antigravity/hub/2.29.0 (aidev_client; os_type=darwin; arch=arm64; cl=1013060670)"

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
