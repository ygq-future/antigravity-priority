package antigravity

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	"antigravity-priority/internal/core"
)

type availableModelsResponse struct {
	Models  map[string]availableModel `json:"models"`
	Buckets []quotaBucket             `json:"buckets"`
	Groups  []quotaGroup              `json:"groups"`
}

type quotaGroup struct {
	DisplayName string        `json:"displayName"`
	Description string        `json:"description"`
	Buckets     []quotaBucket `json:"buckets"`
}

type availableModel struct {
	ModelProvider string    `json:"modelProvider"`
	QuotaInfo     quotaInfo `json:"quotaInfo"`
}

type quotaInfo struct {
	RemainingFraction any           `json:"remainingFraction"`
	ResetTime         any           `json:"resetTime"`
	Windows           []quotaWindow `json:"windows"`
}

type quotaWindow struct {
	Name              string `json:"name"`
	RemainingFraction any    `json:"remainingFraction"`
	ResetTime         any    `json:"resetTime"`
}

type quotaBucket struct {
	ModelID           string `json:"modelId"`
	Window            string `json:"window"`
	RemainingFraction any    `json:"remainingFraction"`
	ResetTime         any    `json:"resetTime"`
}

type candidateWindow struct {
	resetAt   *time.Time
	remaining int64
	window    WindowType
}

// ParseAvailableModels parses the Antigravity quota summary JSON into target model group ProbeResult.
func ParseAvailableModels(raw []byte, observedAt time.Time, group ModelGroup) ProbeResult {
	var response availableModelsResponse
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&response); err != nil {
		return failedResult(observedAt, group, "parse antigravity quota failed")
	}

	windows := collectGroupWindows(response, group)
	selected, ok := pickEffectiveWindow(windows)
	if !ok {
		return failedResult(observedAt, group, "target model group quota unavailable")
	}

	result := ProbeResult{
		Provider:    core.ProviderAntigravity,
		ModelGroup:  group,
		ObservedAt:  observedAt.UTC(),
		ResetAt:     selected.resetAt,
		Remaining:   int64Ptr(selected.remaining),
		Window:      selected.window,
		Freshness:   core.FreshnessFresh,
		ProbeStatus: core.ProbeStatusReady,
		Status:      StatusReady,
		PlanType:    inferPlanType(windows),
	}

	if fiveHour, ok := firstWindow(windows, WindowFiveHour); ok {
		result.ShortWindowResetAt = fiveHour.resetAt
		result.ShortWindowRemaining = int64Ptr(fiveHour.remaining)
	}
	if weekly, ok := firstWindow(windows, WindowWeekly); ok {
		result.LongWindowResetAt = weekly.resetAt
		result.LongWindowRemaining = int64Ptr(weekly.remaining)
	}

	return result
}

func collectGroupWindows(response availableModelsResponse, group ModelGroup) []candidateWindow {
	windows := make([]candidateWindow, 0)
	for modelID, model := range response.Models {
		if !modelBelongsToGroup(modelID, model.ModelProvider, group) {
			continue
		}
		if len(model.QuotaInfo.Windows) == 0 {
			if window, ok := quotaFieldsToWindow(model.QuotaInfo.RemainingFraction, model.QuotaInfo.ResetTime, WindowUnknown); ok {
				windows = append(windows, window)
			}
			continue
		}
		for _, item := range model.QuotaInfo.Windows {
			if window, ok := quotaFieldsToWindow(item.RemainingFraction, item.ResetTime, classifyWindow(item.Name)); ok {
				windows = append(windows, window)
			}
		}
	}
	for _, bucket := range response.Buckets {
		if !modelBelongsToGroup(bucket.ModelID, "", group) {
			continue
		}
		windowType := classifyWindow(bucket.Window)
		if window, ok := quotaFieldsToWindow(bucket.RemainingFraction, bucket.ResetTime, windowType); ok {
			windows = append(windows, window)
		}
	}
	for _, quotaGroup := range response.Groups {
		if !quotaGroupBelongsToModelGroup(quotaGroup, group) {
			continue
		}
		for _, bucket := range quotaGroup.Buckets {
			if window, ok := quotaFieldsToWindow(bucket.RemainingFraction, bucket.ResetTime, classifyWindow(bucket.Window)); ok {
				windows = append(windows, window)
			}
		}
	}
	return windows
}

func quotaGroupBelongsToModelGroup(quotaGroup quotaGroup, group ModelGroup) bool {
	text := strings.ToLower(strings.TrimSpace(quotaGroup.DisplayName + " " + quotaGroup.Description))
	if group == ModelGroupClaudeGPT {
		return strings.Contains(text, "claude") || strings.Contains(text, "gpt")
	}
	return strings.Contains(text, "gemini") && !strings.Contains(text, "claude") && !strings.Contains(text, "gpt")
}

func pickEffectiveWindow(windows []candidateWindow) (candidateWindow, bool) {
	fiveHour, hasFiveHour := firstWindow(windows, WindowFiveHour)
	weekly, hasWeekly := firstWindow(windows, WindowWeekly)
	if hasFiveHour && hasWeekly {
		if weekly.remaining <= 0 {
			return weekly, true
		}
		if fiveHour.remaining <= 0 {
			return fiveHour, true
		}
		return fiveHour, true
	}
	if hasWeekly {
		return weekly, true
	}
	if hasFiveHour {
		return fiveHour, true
	}
	for _, window := range windows {
		return window, true
	}
	return candidateWindow{}, false
}

func firstWindow(windows []candidateWindow, windowType WindowType) (candidateWindow, bool) {
	for _, window := range windows {
		if window.window == windowType {
			return window, true
		}
	}
	return candidateWindow{}, false
}

func quotaFieldsToWindow(rawRemaining any, rawReset any, windowType WindowType) (candidateWindow, bool) {
	resetAt, ok := parseAnyTime(rawReset)
	if !ok {
		return candidateWindow{}, false
	}
	remaining, ok := remainingPercent(rawRemaining)
	if !ok {
		return candidateWindow{}, false
	}
	return candidateWindow{resetAt: resetAt, remaining: remaining, window: windowType}, true
}

func modelBelongsToGroup(modelID string, provider string, group ModelGroup) bool {
	text := strings.ToLower(strings.TrimSpace(modelID + " " + provider))
	if group == ModelGroupClaudeGPT {
		return strings.Contains(text, "claude") || strings.Contains(text, "gpt") || strings.Contains(text, "openai")
	}
	return strings.Contains(text, "gemini") && !strings.Contains(text, "claude") && !strings.Contains(text, "gpt")
}

// ClassifyWindow identifies the window type from a window name or descriptor.
func ClassifyWindow(name string) WindowType {
	return classifyWindow(name)
}

func classifyWindow(name string) WindowType {
	text := strings.ToLower(strings.TrimSpace(name))
	if text == "" {
		return WindowUnknown
	}
	// Long window takes priority to prevent strings like "5d 15h" from false-matching short window
	if strings.Contains(text, "week") || strings.Contains(text, "7d") || hasDayToken(text) {
		return WindowWeekly
	}
	// Match standalone digit 5 + hour unit, rejecting 15h, 25hr, etc.
	if hasFiveHourToken(text) {
		return WindowFiveHour
	}
	return WindowUnknown
}

// hasDayToken detects Nd day unit (e.g. 5d, 7 days), used for weekly/multiday quota names.
func hasDayToken(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			continue
		}
		j := i + 1
		for j < len(text) && text[j] >= '0' && text[j] <= '9' {
			j++
		}
		k := j
		for k < len(text) && text[k] == ' ' {
			k++
		}
		if isDayUnitAt(text, k) {
			return true
		}
		i = j
	}
	return false
}

// isDayUnitAt matches day units starting at offset: days/day/d.
func isDayUnitAt(text string, offset int) bool {
	if offset >= len(text) {
		return false
	}
	for _, unit := range []string{"days", "day", "d"} {
		end := offset + len(unit)
		if end > len(text) || text[offset:end] != unit {
			continue
		}
		if end == len(text) || text[end] < 'a' || text[end] > 'z' {
			return true
		}
	}
	return false
}

// hasFiveHourToken detects standalone 5-hour tokens: 5h / 5hr / 5 hour(s).
// Rejects 15h, 25hr, etc.
func hasFiveHourToken(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			continue
		}
		j := i + 1
		for j < len(text) && text[j] >= '0' && text[j] <= '9' {
			j++
		}
		// Must be exactly one digit and equal to '5'
		if j-i != 1 || text[i] != '5' {
			i = j
			continue
		}
		// Also verify preceding character is not alphanumeric
		if i > 0 && ((text[i-1] >= 'a' && text[i-1] <= 'z') || (text[i-1] >= 'A' && text[i-1] <= 'Z')) {
			i = j
			continue
		}
		k := j
		for k < len(text) && text[k] == ' ' {
			k++
		}
		if isHourUnitAt(text, k) {
			return true
		}
		i = j
	}
	return false
}

// isHourUnitAt matches hour units starting at offset: hours/hour/hrs/hr/h.
func isHourUnitAt(text string, offset int) bool {
	if offset >= len(text) {
		return false
	}
	for _, unit := range []string{"hours", "hour", "hrs", "hr", "h"} {
		end := offset + len(unit)
		if end > len(text) || text[offset:end] != unit {
			continue
		}
		if end == len(text) || text[end] < 'a' || text[end] > 'z' {
			return true
		}
	}
	return false
}

func inferPlanType(windows []candidateWindow) core.PlanType {
	if _, ok := firstWindow(windows, WindowFiveHour); ok {
		return core.PlanTypePro
	}
	if _, ok := firstWindow(windows, WindowWeekly); ok {
		return core.PlanTypeFree
	}
	return core.PlanTypeUnknown
}

func parseAnyTime(raw any) (*time.Time, bool) {
	switch value := raw.(type) {
	case nil:
		return nil, false
	case string:
		return parseTimeString(value)
	case float64:
		return parseUnix(int64(value))
	case json.Number:
		integer, err := value.Int64()
		if err != nil {
			return nil, false
		}
		return parseUnix(integer)
	default:
		return nil, false
	}
}

func parseTimeString(value string) (*time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	if integer, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		return parseUnix(integer)
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			utc := parsed.UTC()
			return &utc, true
		}
	}
	return nil, false
}

func parseUnix(value int64) (*time.Time, bool) {
	if value <= 0 {
		return nil, false
	}
	if value > 1_000_000_000_000 {
		parsed := time.UnixMilli(value).UTC()
		return &parsed, true
	}
	parsed := time.Unix(value, 0).UTC()
	return &parsed, true
}

func remainingPercent(raw any) (int64, bool) {
	value, ok := toFloat64(raw)
	if !ok {
		return 0, false
	}
	if value <= 1 {
		value *= 100
	}
	if value <= 0 {
		return 0, true
	}
	return int64(math.Ceil(value)), true
}

func toFloat64(raw any) (float64, bool) {
	switch value := raw.(type) {
	case float64:
		return value, true
	case json.Number:
		parsed, err := value.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func failedResult(observedAt time.Time, group ModelGroup, message string) ProbeResult {
	return ProbeResult{
		Provider:    core.ProviderAntigravity,
		ModelGroup:  group,
		ObservedAt:  observedAt.UTC(),
		Window:      WindowUnknown,
		Freshness:   core.FreshnessUnknown,
		ProbeStatus: core.ProbeStatusUnknown,
		Status:      StatusProbeFailed,
		PlanType:    core.PlanTypeUnknown,
		Error:       message,
	}
}

func int64Ptr(value int64) *int64 {
	return &value
}
