package state

import (
	"strings"
	"time"
)

const (
	// DefaultMaxRequestHistory is the default maximum number of recent requests retained in history.
	DefaultMaxRequestHistory = 100

	// RequestHistorySourceWarmup marks internal 5h window warm-up generations.
	RequestHistorySourceWarmup = "warmup"
	// RequestHistorySourceHealth marks internal cooldown recovery health probes.
	RequestHistorySourceHealth = "health"
)

// RequestHistoryEntry records a request served by or probed against an Antigravity credential.
type RequestHistoryEntry struct {
	ID                  string    `json:"id"`
	Source              string    `json:"source,omitempty"`
	At                  time.Time `json:"at"`
	Model               string    `json:"model"`
	ModelPool           string    `json:"model_pool"` // "gemini", "argon", "claude_gpt"
	AuthIndex           string    `json:"auth_index"`
	Email               string    `json:"email"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	ReasoningTokens     int64     `json:"reasoning_tokens,omitempty"`
	CachedTokens        int64     `json:"cached_tokens,omitempty"`
	CacheReadTokens     int64     `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens int64     `json:"cache_creation_tokens,omitempty"`
	TotalTokens         int64     `json:"total_tokens"`
	Failed              bool      `json:"failed"`
	StatusCode          int       `json:"status_code"`
	ErrorMessage        string    `json:"error_message,omitempty"`
}

// IsBackgroundSynthetic reports whether the entry was generated internally by
// the plugin (such as a 5h warm-up or cooldown health probe) rather than an
// inbound client request.
func (e RequestHistoryEntry) IsBackgroundSynthetic() bool {
	switch strings.ToLower(strings.TrimSpace(e.Source)) {
	case RequestHistorySourceWarmup, RequestHistorySourceHealth:
		return true
	}
	return strings.HasPrefix(e.ID, "warmup-") || strings.HasPrefix(e.ID, "health-")
}
