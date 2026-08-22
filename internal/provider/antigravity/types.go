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

// RetrieveUserQuotaSummaryURL is the primary Antigravity quota endpoint.
const RetrieveUserQuotaSummaryURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

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
	Status               Status
	PlanType             core.PlanType
	Error                string
}
