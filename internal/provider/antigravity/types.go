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
	// ModelGroupArgon represents the Gemini 4 Argon quota pool.
	ModelGroupArgon = config.AntigravityModelGroupArgon
	// ModelGroupClaudeGPT represents the Claude and GPT model group.
	ModelGroupClaudeGPT = config.AntigravityModelGroupClaudeGPT
)

// RetrieveUserQuotaSummaryDailyURL is the Antigravity consumer quota summary endpoint.
//
// Consumer Antigravity traffic runs against this host, and only this host reports the
// measured Gemini, Argon, and Claude/GPT buckets. The production host serves the
// separate Gemini CLI quota, so its buckets are not interchangeable with these.
const RetrieveUserQuotaSummaryDailyURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"

// GenerateContentDailyURL is the Antigravity consumer generation endpoint. Warm-up
// generations must use this surface: tokens consumed on another host never reach the
// measured consumer quota ledger.
const GenerateContentDailyURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:generateContent"

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
	// StatusUnavailable indicates the upstream did not expose this model group's
	// quota at all (for example, an Argon pool the account is not entitled to).
	// It is not a probe failure: the group is simply absent from the account.
	StatusUnavailable Status = "unavailable"
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
