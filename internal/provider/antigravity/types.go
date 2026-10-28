package antigravity

import (
	"time"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/core"
)

// ModelGroup represents the upstream quota model group in Antigravity.
type ModelGroup = config.AntigravityModelGroup

const (
	// ModelGroupGemini represents the Gemini model group.
	ModelGroupGemini = config.AntigravityModelGroupGemini
	// ModelGroupClaudeGPT represents the Claude and GPT model group.
	ModelGroupClaudeGPT = config.AntigravityModelGroupClaudeGPT
)

// RetrieveUserQuotaSummaryURL is the primary Antigravity quota summary endpoint.
//
// Production must be asked first: the daily host has been observed answering 200
// with only the Claude/GPT group, which silently hides the shared Gemini windows
// and the separate Gemini 4 Argon weekly bucket.
const RetrieveUserQuotaSummaryURL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

// RetrieveUserQuotaSummaryDailyURL is the daily Antigravity quota summary endpoint,
// kept as a fallback for when production is unreachable.
const RetrieveUserQuotaSummaryDailyURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

// RetrieveUserQuotaSummarySandboxURL is the sandbox Antigravity quota summary endpoint.
const RetrieveUserQuotaSummarySandboxURL = "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:retrieveUserQuotaSummary"

// FetchAvailableModelsURL is the fallback Antigravity available models endpoint providing per-model quotas.
const FetchAvailableModelsURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels"

// WindowType indicates the quota window type classified from upstream responses.
type WindowType string

const (
	// WindowUnknown indicates an unspecified or unrecognized window type.
	WindowUnknown WindowType = "unknown"
	// WindowFiveHour indicates a 5-hour short rolling quota window.
	WindowFiveHour WindowType = "5h"
	// WindowWeekly indicates a 7-day weekly quota window.
	WindowWeekly WindowType = "weekly"
)

// Status indicates the outcome of an Antigravity probe.
type Status string

const (
	// StatusReady indicates the probe produced actionable quota evidence.
	StatusReady Status = "ready"
	// StatusProbeFailed indicates the probe failed to produce valid quota evidence.
	StatusProbeFailed Status = "probe_failed"
)

// ProbeResult is the normalized quota evidence output from probing Antigravity.
type ProbeResult struct {
	Provider             core.Provider
	AuthIndex            string
	ModelGroup           ModelGroup
	ObservedAt           time.Time
	ResetAt              *time.Time
	Remaining            *int64
	Window               WindowType
	ShortWindowResetAt   *time.Time
	ShortWindowRemaining *int64
	LongWindowResetAt    *time.Time
	LongWindowRemaining  *int64
	ArgonWindowResetAt   *time.Time
	ArgonWindowRemaining *int64
	Status               Status
	PlanType             core.PlanType
	Error                string
	HTTPStatusCode       int
}
