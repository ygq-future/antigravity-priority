package state

import "time"

const (
	// DefaultMaxRequestHistory is the default maximum number of recent requests retained in history.
	DefaultMaxRequestHistory = 100
)

// RequestHistoryEntry records an inbound request served by an Antigravity credential.
type RequestHistoryEntry struct {
	ID                  string    `json:"id"`
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
