package config

import "strings"

// AntigravityModelGroup represents the quota model group in Antigravity.
type AntigravityModelGroup string

const (
	// AntigravityModelGroupGemini represents Gemini models group.
	AntigravityModelGroupGemini AntigravityModelGroup = "gemini"
	// AntigravityModelGroupClaudeGPT represents Claude and GPT models group.
	AntigravityModelGroupClaudeGPT AntigravityModelGroup = "claude_gpt"
)

// ParseAntigravityModelGroup parses a string representation of a model group into an AntigravityModelGroup enum.
func ParseAntigravityModelGroup(value string) (AntigravityModelGroup, error) {
	switch normalizeAntigravityModelGroup(value) {
	case "", string(AntigravityModelGroupGemini):
		return AntigravityModelGroupGemini, nil
	case string(AntigravityModelGroupClaudeGPT), "claudegpt", "claude-gpt", "claude gpt", "claude_and_gpt", "claude-and-gpt", "claude and gpt":
		return AntigravityModelGroupClaudeGPT, nil
	default:
		return "", invalid("antigravity_model_group", value, "must be gemini or claude_gpt")
	}
}

func normalizeAntigravityModelGroup(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(yamlText(value)))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return trimmed
}
