package antigravity

import (
	"encoding/json"
	"testing"
	"time"

	"antigravity-priority/internal/core"
)

func TestParseAvailableModels_DualWindow(t *testing.T) {
	raw := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "5h",
							"remainingFraction": 0.85,
							"resetTime": "2026-08-17T15:00:00Z"
						},
						{
							"name": "weekly",
							"remainingFraction": 0.70,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			},
			"claude-3-5-sonnet": {
				"modelProvider": "anthropic",
				"quotaInfo": {
					"windows": [
						{
							"name": "5hr",
							"remainingFraction": 0.40,
							"resetTime": "2026-08-17T16:00:00Z"
						},
						{
							"name": "7d",
							"remainingFraction": 0.60,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			}
		}
	}`)

	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	// Test Gemini group
	geminiResult := ParseAvailableModels(raw, now, ModelGroupGemini)
	if geminiResult.Status != StatusReady {
		t.Fatalf("expected gemini StatusReady, got %v (err: %v)", geminiResult.Status, geminiResult.Error)
	}
	if geminiResult.PlanType != core.PlanTypePro {
		t.Errorf("expected PlanTypePro, got %v", geminiResult.PlanType)
	}
	if geminiResult.Window != WindowFiveHour {
		t.Errorf("expected effective WindowFiveHour, got %v", geminiResult.Window)
	}
	if geminiResult.Remaining == nil || *geminiResult.Remaining != 85 {
		t.Errorf("expected Remaining=85, got %v", geminiResult.Remaining)
	}
	if geminiResult.ShortWindowRemaining == nil || *geminiResult.ShortWindowRemaining != 85 {
		t.Errorf("expected ShortWindowRemaining=85, got %v", geminiResult.ShortWindowRemaining)
	}
	if geminiResult.ShortWindowResetAt == nil || geminiResult.ShortWindowResetAt.Format(time.RFC3339) != "2026-08-17T15:00:00Z" {
		t.Errorf("expected ShortWindowResetAt 2026-08-17T15:00:00Z, got %v", geminiResult.ShortWindowResetAt)
	}
	if geminiResult.LongWindowRemaining == nil || *geminiResult.LongWindowRemaining != 70 {
		t.Errorf("expected LongWindowRemaining=70, got %v", geminiResult.LongWindowRemaining)
	}
	if geminiResult.LongWindowResetAt == nil || geminiResult.LongWindowResetAt.Format(time.RFC3339) != "2026-08-24T00:00:00Z" {
		t.Errorf("expected LongWindowResetAt 2026-08-24T00:00:00Z, got %v", geminiResult.LongWindowResetAt)
	}

	// Test Claude GPT group
	claudeResult := ParseAvailableModels(raw, now, ModelGroupClaudeGPT)
	if claudeResult.Status != StatusReady {
		t.Fatalf("expected claude StatusReady, got %v (err: %v)", claudeResult.Status, claudeResult.Error)
	}
	if claudeResult.Remaining == nil || *claudeResult.Remaining != 40 {
		t.Errorf("expected Remaining=40, got %v", claudeResult.Remaining)
	}
	if claudeResult.ShortWindowRemaining == nil || *claudeResult.ShortWindowRemaining != 40 {
		t.Errorf("expected ShortWindowRemaining=40, got %v", claudeResult.ShortWindowRemaining)
	}
	if claudeResult.LongWindowRemaining == nil || *claudeResult.LongWindowRemaining != 60 {
		t.Errorf("expected LongWindowRemaining=60, got %v", claudeResult.LongWindowRemaining)
	}
}

func TestParseQuotaSummary_UsesOfficialGroupBuckets(t *testing.T) {
	raw := []byte(`{
		"groups": [
			{
				"displayName": "Gemini Models",
				"description": "Shared quota for Gemini models",
				"buckets": [
					{"bucketId":"gemini-5h","displayName":"5 hour","window":"5h","remainingFraction":0.914,"resetTime":"2026-08-24T15:00:00Z"},
					{"bucketId":"gemini-7d","displayName":"Weekly","window":"7d","remainingFraction":0.556,"resetTime":"2026-08-31T00:00:00Z"}
				]
			},
			{
				"displayName": "Claude and GPT Models",
				"description": "Shared quota for Claude and GPT models",
				"buckets": [
					{"bucketId":"claude-5h","window":"5h","remainingFraction":0.754,"resetTime":"2026-08-24T16:00:00Z"},
					{"bucketId":"claude-7d","window":"7d","remainingFraction":0.245,"resetTime":"2026-08-31T00:00:00Z"}
				]
			}
		]
	}`)

	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	gemini := ParseAvailableModels(raw, now, ModelGroupGemini)
	claude := ParseAvailableModels(raw, now, ModelGroupClaudeGPT)

	if gemini.ShortWindowRemaining == nil || *gemini.ShortWindowRemaining != 91 || gemini.LongWindowRemaining == nil || *gemini.LongWindowRemaining != 56 {
		t.Fatalf("Gemini group buckets = short %v long %v; want 91/56", gemini.ShortWindowRemaining, gemini.LongWindowRemaining)
	}
	if claude.ShortWindowRemaining == nil || *claude.ShortWindowRemaining != 75 || claude.LongWindowRemaining == nil || *claude.LongWindowRemaining != 25 {
		t.Fatalf("Claude/GPT group buckets = short %v long %v; want 75/25", claude.ShortWindowRemaining, claude.LongWindowRemaining)
	}
}

func TestParseQuotaSummary_PrefersCanonicalGroupBuckets(t *testing.T) {
	raw := []byte(`{
		"models": {
			"gemini-legacy": {"quotaInfo":{"windows":[
				{"name":"5h","remainingFraction":0.10,"resetTime":"2026-08-24T15:00:00Z"},
				{"name":"7d","remainingFraction":0.20,"resetTime":"2026-08-31T00:00:00Z"}
			]}}
		},
		"groups": [{
			"displayName":"Gemini Models",
			"buckets":[
				{"window":"5h","remainingFraction":0.91,"resetTime":"2026-08-24T15:00:00Z"},
				{"window":"7d","remainingFraction":0.56,"resetTime":"2026-08-31T00:00:00Z"}
			]
		}]
	}`)

	result := ParseAvailableModels(raw, time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC), ModelGroupGemini)
	if result.ShortWindowRemaining == nil || *result.ShortWindowRemaining != 91 || result.LongWindowRemaining == nil || *result.LongWindowRemaining != 56 {
		t.Fatalf("canonical group buckets were not authoritative: short %v long %v", result.ShortWindowRemaining, result.LongWindowRemaining)
	}
}

func TestParseAvailableModels_WeeklyOnlyFree(t *testing.T) {
	raw := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "weekly",
							"remainingFraction": 0.50,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			}
		}
	}`)

	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	result := ParseAvailableModels(raw, now, ModelGroupGemini)
	if result.Status != StatusReady {
		t.Fatalf("expected StatusReady, got %v", result.Status)
	}
	if result.PlanType != core.PlanTypeFree {
		t.Errorf("expected PlanTypeFree, got %v", result.PlanType)
	}
	if result.Window != WindowWeekly {
		t.Errorf("expected WindowWeekly, got %v", result.Window)
	}
	if result.Remaining == nil || *result.Remaining != 50 {
		t.Errorf("expected Remaining=50, got %v", result.Remaining)
	}
	if result.ShortWindowRemaining != nil {
		t.Errorf("expected nil ShortWindowRemaining, got %v", result.ShortWindowRemaining)
	}
	if result.LongWindowRemaining == nil || *result.LongWindowRemaining != 50 {
		t.Errorf("expected LongWindowRemaining=50, got %v", result.LongWindowRemaining)
	}
}

func TestParseAvailableModels_BucketsAndGroups(t *testing.T) {
	raw := []byte(`{
		"buckets": [
			{
				"modelId": "gemini-1.5-pro",
				"window": "5 hours",
				"remainingFraction": 90,
				"resetTime": 1723906800
			}
		],
		"groups": [
			{
				"displayName": "Claude and GPT Models",
				"description": "Shared quota for Claude and GPT",
				"buckets": [
					{
						"modelId": "claude-group-bucket",
						"window": "weekly",
						"remainingFraction": "0.45",
						"resetTime": "2026-08-24 00:00:00"
					}
				]
			}
		]
	}`)

	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	// Gemini from top-level buckets
	geminiResult := ParseAvailableModels(raw, now, ModelGroupGemini)
	if geminiResult.Status != StatusReady {
		t.Fatalf("expected gemini StatusReady, got %v", geminiResult.Status)
	}
	if geminiResult.Remaining == nil || *geminiResult.Remaining != 90 {
		t.Errorf("expected Remaining=90, got %v", geminiResult.Remaining)
	}
	if geminiResult.Window != WindowFiveHour {
		t.Errorf("expected WindowFiveHour, got %v", geminiResult.Window)
	}

	// Claude from groups
	claudeResult := ParseAvailableModels(raw, now, ModelGroupClaudeGPT)
	if claudeResult.Status != StatusReady {
		t.Fatalf("expected claude StatusReady, got %v", claudeResult.Status)
	}
	if claudeResult.Remaining == nil || *claudeResult.Remaining != 45 {
		t.Errorf("expected Remaining=45, got %v", claudeResult.Remaining)
	}
	if claudeResult.Window != WindowWeekly {
		t.Errorf("expected WindowWeekly, got %v", claudeResult.Window)
	}
}

func TestParseAvailableModels_SingleWindowDirectQuotaInfo(t *testing.T) {
	raw := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"remainingFraction": 0.65,
					"resetTime": "2026-08-17T18:00:00Z"
				}
			}
		}
	}`)

	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	result := ParseAvailableModels(raw, now, ModelGroupGemini)
	if result.Status != StatusReady {
		t.Fatalf("expected StatusReady, got %v", result.Status)
	}
	if result.Window != WindowUnknown {
		t.Errorf("expected WindowUnknown, got %v", result.Window)
	}
	if result.Remaining == nil || *result.Remaining != 65 {
		t.Errorf("expected Remaining=65, got %v", result.Remaining)
	}
}

func TestParseAvailableModels_DepletionPriority(t *testing.T) {
	// When weekly is 0 and 5h is 80, effective window should pick weekly
	raw := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "5h",
							"remainingFraction": 0.80,
							"resetTime": "2026-08-17T15:00:00Z"
						},
						{
							"name": "weekly",
							"remainingFraction": 0.0,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			}
		}
	}`)

	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	result := ParseAvailableModels(raw, now, ModelGroupGemini)
	if result.Status != StatusReady {
		t.Fatalf("expected StatusReady, got %v", result.Status)
	}
	if result.Window != WindowWeekly {
		t.Errorf("expected WindowWeekly to take precedence on depletion, got %v", result.Window)
	}
	if result.Remaining == nil || *result.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %v", result.Remaining)
	}

	// When 5h is 0 and weekly is 50, effective window should pick 5h
	raw5hDepleted := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"windows": [
						{
							"name": "5h",
							"remainingFraction": 0.0,
							"resetTime": "2026-08-17T15:00:00Z"
						},
						{
							"name": "weekly",
							"remainingFraction": 0.50,
							"resetTime": "2026-08-24T00:00:00Z"
						}
					]
				}
			}
		}
	}`)

	result5h := ParseAvailableModels(raw5hDepleted, now, ModelGroupGemini)
	if result5h.Status != StatusReady {
		t.Fatalf("expected StatusReady, got %v", result5h.Status)
	}
	if result5h.Window != WindowFiveHour {
		t.Errorf("expected WindowFiveHour on short window depletion, got %v", result5h.Window)
	}
	if result5h.Remaining == nil || *result5h.Remaining != 0 {
		t.Errorf("expected Remaining=0, got %v", result5h.Remaining)
	}
}

func TestParseAvailableModels_ErrorsAndEdgeCases(t *testing.T) {
	now := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)

	// Malformed JSON
	badJSON := ParseAvailableModels([]byte(`invalid-json`), now, ModelGroupGemini)
	if badJSON.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on bad JSON, got %v", badJSON.Status)
	}

	// Missing target model group
	geminiOnly := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"remainingFraction": 0.5,
					"resetTime": "2026-08-17T15:00:00Z"
				}
			}
		}
	}`)
	claudeMissing := ParseAvailableModels(geminiOnly, now, ModelGroupClaudeGPT)
	if claudeMissing.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed when group missing, got %v", claudeMissing.Status)
	}

	// Invalid time format
	invalidTime := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"remainingFraction": 0.5,
					"resetTime": "not-a-valid-time"
				}
			}
		}
	}`)
	timeErr := ParseAvailableModels(invalidTime, now, ModelGroupGemini)
	if timeErr.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on invalid time, got %v", timeErr.Status)
	}

	// Invalid remaining format
	invalidRemaining := []byte(`{
		"models": {
			"gemini-2.0-flash": {
				"modelProvider": "google",
				"quotaInfo": {
					"remainingFraction": "not-a-number",
					"resetTime": "2026-08-17T15:00:00Z"
				}
			}
		}
	}`)
	remErr := ParseAvailableModels(invalidRemaining, now, ModelGroupGemini)
	if remErr.Status != StatusProbeFailed {
		t.Errorf("expected StatusProbeFailed on invalid remaining, got %v", remErr.Status)
	}
}

func TestClassifyWindow(t *testing.T) {
	tests := []struct {
		input    string
		expected WindowType
	}{
		// 5-hour short window variants
		{"5h", WindowFiveHour},
		{"5H", WindowFiveHour},
		{"5hr", WindowFiveHour},
		{"5HR", WindowFiveHour},
		{"5hrs", WindowFiveHour},
		{"5 hour", WindowFiveHour},
		{"5 hours", WindowFiveHour},
		{"5  hours", WindowFiveHour},
		{"rolling 5h quota", WindowFiveHour},
		{"burst 5hr limit", WindowFiveHour},

		// Rejections: false positive checks
		{"15h", WindowUnknown},
		{"25h", WindowUnknown},
		{"15hr", WindowUnknown},
		{"25hrs", WindowUnknown},
		{"35 hours", WindowUnknown},
		{"50h", WindowUnknown},
		{"50hr", WindowUnknown},
		{"h5", WindowUnknown},

		// Weekly / Long window variants
		{"weekly", WindowWeekly},
		{"WEEKLY", WindowWeekly},
		{"week", WindowWeekly},
		{"7d", WindowWeekly},
		{"7D", WindowWeekly},
		{"7 days", WindowWeekly},
		{"7day", WindowWeekly},
		{"5d 15h", WindowWeekly},
		{"5d", WindowWeekly},
		{"6d", WindowWeekly},

		// Unknown / empty
		{"", WindowUnknown},
		{"   ", WindowUnknown},
		{"daily", WindowUnknown},
		{"monthly", WindowUnknown},
	}

	for _, tt := range tests {
		got := ClassifyWindow(tt.input)
		if got != tt.expected {
			t.Errorf("ClassifyWindow(%q) = %v; want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseAnyTime(t *testing.T) {
	// RFC3339
	tm, ok := parseAnyTime("2026-08-17T12:34:56Z")
	if !ok || tm == nil || tm.Year() != 2026 {
		t.Errorf("failed to parse RFC3339: %v", tm)
	}

	// Unix timestamp seconds as number
	tm2, ok := parseAnyTime(float64(1723906800))
	if !ok || tm2 == nil {
		t.Errorf("failed to parse unix seconds float: %v", tm2)
	}

	// Unix timestamp millis as json.Number
	tm3, ok := parseAnyTime(json.Number("1723906800000"))
	if !ok || tm3 == nil {
		t.Errorf("failed to parse unix millis json.Number: %v", tm3)
	}

	// String unix
	tm4, ok := parseAnyTime("1723906800")
	if !ok || tm4 == nil {
		t.Errorf("failed to parse unix seconds string: %v", tm4)
	}

	// Nil and invalid
	if _, ok := parseAnyTime(nil); ok {
		t.Errorf("expected false on nil")
	}
	if _, ok := parseAnyTime(true); ok {
		t.Errorf("expected false on bool")
	}
}

func TestRemainingPercent(t *testing.T) {
	tests := []struct {
		raw      any
		expected int64
		ok       bool
	}{
		{0.85, 85, true},
		{1.0, 100, true},
		{0.0, 0, true},
		{-0.5, 0, true},
		{50.0, 50, true},
		{json.Number("0.75"), 75, true},
		{"0.92", 92, true},
		{"100", 100, true},
		{nil, 0, false},
		{true, 0, false},
	}

	for _, tt := range tests {
		got, ok := remainingPercent(tt.raw)
		if ok != tt.ok || (ok && got != tt.expected) {
			t.Errorf("remainingPercent(%v) = (%v, %v); want (%v, %v)", tt.raw, got, ok, tt.expected, tt.ok)
		}
	}
}
