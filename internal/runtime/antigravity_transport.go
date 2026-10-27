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
	"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:retrieveUserQuotaSummary",
	"https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary",
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
	TargetGroup antigravity.ModelGroup
}

type quotaHTTPDoer interface {
	HTTPDoRaw(context.Context, host.HTTPRequest) (host.HTTPResponse, error)
}

func executeAntigravityQuotaRequest(ctx context.Context, doer quotaHTTPDoer, request antigravityQuotaRequest) map[antigravity.ModelGroup]antigravity.ProbeResult {
	primaryURLs, fallbackURLs := selectQuotaEndpointURLs(request.TargetGroup)

	results, lastStatus, lastErr := queryQuotaURLs(ctx, doer, primaryURLs, request)

	if !hasRequestedQuota(results, request.TargetGroup) {
		fallbackResults, fbStatus, fbErr := queryQuotaURLs(ctx, doer, fallbackURLs, request)
		if fbStatus != 0 {
			lastStatus = fbStatus
		}
		if fbErr != nil && lastErr == nil {
			lastErr = fbErr
		}
		if fallbackResults != nil {
			if results == nil {
				results = make(map[antigravity.ModelGroup]antigravity.ProbeResult, len(fallbackResults))
			}
			for group, fResult := range fallbackResults {
				if existing, exists := results[group]; !exists || existing.Status != antigravity.StatusReady {
					results[group] = fResult
				}
			}
		}
	}

	if results != nil {
		return pruneUnrequestedFailedGroups(results, request.TargetGroup)
	}
	message := fmt.Sprintf("retrieve quota summary status %d", lastStatus)
	if lastErr != nil && lastStatus == 0 {
		message = "host http do failed"
	}
	return failedAntigravityQuotaResults(request, message, lastStatus)
}

func selectQuotaEndpointURLs(target antigravity.ModelGroup) ([]string, []string) {
	if target == antigravity.ModelGroupGemini {
		return antigravityAvailableModelsURLs, antigravityQuotaURLs
	}
	return antigravityQuotaURLs, antigravityAvailableModelsURLs
}

func hasRequestedQuota(results map[antigravity.ModelGroup]antigravity.ProbeResult, target antigravity.ModelGroup) bool {
	if results == nil {
		return false
	}
	switch target {
	case antigravity.ModelGroupGemini:
		return results[antigravity.ModelGroupGemini].Status == antigravity.StatusReady
	case antigravity.ModelGroupClaudeGPT:
		return results[antigravity.ModelGroupClaudeGPT].Status == antigravity.StatusReady
	default:
		return results[antigravity.ModelGroupGemini].Status == antigravity.StatusReady &&
			results[antigravity.ModelGroupClaudeGPT].Status == antigravity.StatusReady
	}
}

func queryQuotaURLs(ctx context.Context, doer quotaHTTPDoer, urls []string, request antigravityQuotaRequest) (map[antigravity.ModelGroup]antigravity.ProbeResult, int, error) {
	var lastStatus int
	var lastErr error

	for _, url := range urls {
		response, err := doer.HTTPDoRaw(ctx, host.HTTPRequest{
			AuthIndex: request.AuthIndex,
			Method:    http.MethodPost,
			URL:       url,
			Headers:   antigravityQuotaHeaders(request.AccessToken),
			Body:      antigravityQuotaBody(request.ProjectID),
		})
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
		return results, response.StatusCode, nil
	}

	return nil, lastStatus, lastErr
}

func pruneUnrequestedFailedGroups(results map[antigravity.ModelGroup]antigravity.ProbeResult, target antigravity.ModelGroup) map[antigravity.ModelGroup]antigravity.ProbeResult {
	if results == nil || target == "" {
		return results
	}
	for group, res := range results {
		if group != target && res.Status != antigravity.StatusReady {
			delete(results, group)
		}
	}
	return results
}

func failedAntigravityQuotaResults(request antigravityQuotaRequest, message string, statusCode int) map[antigravity.ModelGroup]antigravity.ProbeResult {
	results := make(map[antigravity.ModelGroup]antigravity.ProbeResult, 2)
	for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
		if request.TargetGroup != "" && group != request.TargetGroup {
			continue
		}
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

func antigravityQuotaHeaders(accessToken string) host.Header {
	token := "$TOKEN$"
	if trimmed := strings.TrimSpace(accessToken); trimmed != "" {
		token = trimmed
	}
	return host.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer " + token},
		"Content-Type":  []string{"application/json"},
		"User-Agent":    []string{"antigravity/hub/2.9.1 darwin/arm64"},
	}
}

