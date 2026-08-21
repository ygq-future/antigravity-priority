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
	var lastStatus int
	var lastErr error
	for _, url := range antigravityQuotaURLs {
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
			results[group] = result
		}
		return results
	}
	message := fmt.Sprintf("retrieve quota summary status %d", lastStatus)
	if lastErr != nil && lastStatus == 0 {
		message = "host http do failed"
	}
	return failedAntigravityQuotaResults(request, message)
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
		"User-Agent":    []string{"antigravity/cli/1.0.8 darwin/arm64"},
	}
}

func failedAntigravityQuotaResults(request antigravityQuotaRequest, message string) map[antigravity.ModelGroup]antigravity.ProbeResult {
	results := make(map[antigravity.ModelGroup]antigravity.ProbeResult, 2)
	for _, group := range []antigravity.ModelGroup{antigravity.ModelGroupGemini, antigravity.ModelGroupClaudeGPT} {
		result := antigravity.ParseAvailableModels(nil, request.ObservedAt.UTC(), group)
		result.AuthIndex = request.AuthIndex
		result.Error = message
		results[group] = result
	}
	return results
}
