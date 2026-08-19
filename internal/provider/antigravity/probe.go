package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"antigravity-priority/internal/host"
)

// Prober executes Antigravity quota summary probes via host HTTP callbacks.
type Prober struct {
	host  httpDoer
	clock clock
}

// NewProber creates a Prober with the provided host HTTP doer and clock.
func NewProber(hostAPI httpDoer, clockSource clock) Prober {
	if clockSource == nil {
		clockSource = realClock{}
	}
	return Prober{host: hostAPI, clock: clockSource}
}

// Probe queries the Antigravity quota endpoint and returns normalized quota evidence.
func (p Prober) Probe(ctx context.Context, request ProbeRequest) ProbeResult {
	observedAt := p.clock.Now().UTC()
	if p.host == nil {
		return failedProbe(request, observedAt, "host http doer unavailable")
	}

	var lastStatus int
	var lastErr error
	for _, url := range retrieveUserQuotaSummaryURLs {
		response, err := p.host.HTTPDo(ctx, host.HTTPRequest{
			AuthIndex: request.AuthIndex,
			Method:    http.MethodPost,
			URL:       url,
			Headers:   probeHeaders(request),
			Body:      probeBody(request),
		})
		if err != nil {
			lastErr = err
			continue
		}
		lastStatus = response.StatusCode
		if response.StatusCode != http.StatusOK {
			continue
		}
		result := ParseAvailableModels(response.Body, observedAt, request.ModelGroup)
		if result.Status == StatusReady {
			result.AuthIndex = request.AuthIndex
			return result
		}
	}

	if lastErr != nil && lastStatus == 0 {
		return failedProbe(request, observedAt, "host http do failed")
	}
	return failedProbe(request, observedAt, fmt.Sprintf("retrieve quota summary status %d", lastStatus))
}

func probeBody(request ProbeRequest) []byte {
	projectID := strings.TrimSpace(request.ProjectID)
	if projectID == "" {
		return nil
	}
	body, err := json.Marshal(struct {
		Project string `json:"project"`
	}{Project: projectID})
	if err != nil {
		return nil
	}
	return body
}

func probeHeaders(request ProbeRequest) host.Header {
	headers := host.Header{
		"Accept":        []string{"application/json"},
		"Authorization": []string{"Bearer $TOKEN$"},
		"Content-Type":  []string{"application/json"},
		"User-Agent":    []string{"antigravity/cli/1.0.8 darwin/arm64"},
	}
	if token := strings.TrimSpace(request.AccessToken); token != "" {
		headers["Authorization"] = []string{"Bearer " + token}
	}
	return headers
}

// ProbeAll queries the Antigravity quota endpoint and returns normalized quota evidence
// for all model groups. A single HTTP request is made; the response is parsed for both
// gemini and claude_gpt groups simultaneously.
func (p Prober) ProbeAll(ctx context.Context, request ProbeRequest) map[ModelGroup]ProbeResult {
	observedAt := p.clock.Now().UTC()
	if p.host == nil {
		return allGroupsFailedProbe(request, observedAt, "host http doer unavailable")
	}

	var lastStatus int
	var lastErr error
	for _, url := range retrieveUserQuotaSummaryURLs {
		response, err := p.host.HTTPDo(ctx, host.HTTPRequest{
			AuthIndex: request.AuthIndex,
			Method:    http.MethodPost,
			URL:       url,
			Headers:   probeHeaders(request),
			Body:      probeBody(request),
		})
		if err != nil {
			lastErr = err
			continue
		}
		lastStatus = response.StatusCode
		if response.StatusCode != http.StatusOK {
			continue
		}
		results := ParseAllModelGroups(response.Body, observedAt)
		for group, result := range results {
			result.AuthIndex = request.AuthIndex
			results[group] = result
		}
		return results
	}

	msg := fmt.Sprintf("retrieve quota summary status %d", lastStatus)
	if lastErr != nil && lastStatus == 0 {
		msg = "host http do failed"
	}
	return allGroupsFailedProbe(request, observedAt, msg)
}

func allGroupsFailedProbe(request ProbeRequest, observedAt time.Time, message string) map[ModelGroup]ProbeResult {
	return map[ModelGroup]ProbeResult{
		ModelGroupGemini:    failedProbeForGroup(request, observedAt, message, ModelGroupGemini),
		ModelGroupClaudeGPT: failedProbeForGroup(request, observedAt, message, ModelGroupClaudeGPT),
	}
}

func failedProbeForGroup(request ProbeRequest, observedAt time.Time, message string, group ModelGroup) ProbeResult {
	result := failedResult(observedAt, group, safeError(message))
	result.AuthIndex = request.AuthIndex
	return result
}

func failedProbe(request ProbeRequest, observedAt time.Time, message string) ProbeResult {
	result := failedResult(observedAt, request.ModelGroup, safeError(message))
	result.AuthIndex = request.AuthIndex
	return result
}

func safeError(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return "probe failed"
	}
	return trimmed
}
