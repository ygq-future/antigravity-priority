package config

import "strings"

// AntigravityModelGroup represents the quota model group in Antigravity.
type AntigravityModelGroup string

const (
	// AntigravityModelGroupGemini represents Gemini models group.
	AntigravityModelGroupGemini AntigravityModelGroup = "gemini"
	// AntigravityModelGroupArgon represents the Gemini 4 Argon quota pool.
	AntigravityModelGroupArgon AntigravityModelGroup = "argon"
	// AntigravityModelGroupClaudeGPT represents Claude and GPT models group.
	AntigravityModelGroupClaudeGPT AntigravityModelGroup = "claude_gpt"
)

// AllAntigravityModelGroups enumerates all active quota pool authorities.
var AllAntigravityModelGroups = []AntigravityModelGroup{
	AntigravityModelGroupGemini,
	AntigravityModelGroupArgon,
	AntigravityModelGroupClaudeGPT,
}

// ParseAntigravityModelGroup parses a string representation of a model group into an AntigravityModelGroup enum.
func ParseAntigravityModelGroup(value string) (AntigravityModelGroup, error) {
	switch normalizeAntigravityModelGroup(value) {
	case "", string(AntigravityModelGroupGemini):
		return AntigravityModelGroupGemini, nil
	case string(AntigravityModelGroupArgon):
		return AntigravityModelGroupArgon, nil
	case string(AntigravityModelGroupClaudeGPT), "claudegpt", "claude-gpt", "claude gpt", "claude_and_gpt", "claude-and-gpt", "claude and gpt":
		return AntigravityModelGroupClaudeGPT, nil
	default:
		return "", invalid("antigravity_model_group", value, "must be gemini, argon, or claude_gpt")
	}
}

func normalizeAntigravityModelGroup(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(yamlText(value)))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return trimmed
}

// ClassifyModelQuotaPool classifies a model identifier into its corresponding Antigravity quota pool.
// It strips optional prefix (e.g. "teamA/gemini-4-argon-high" -> "gemini-4-argon-high") and
// thinking suffix (e.g. "claude-sonnet-5-5-high(16384)" -> "claude-sonnet-5-5-high").
func ClassifyModelQuotaPool(model string) AntigravityModelGroup {
	trimmed := strings.TrimSpace(model)
	if idx := strings.LastIndex(trimmed, "/"); idx != -1 {
		trimmed = trimmed[idx+1:]
	}
	if idx := strings.Index(trimmed, "("); idx != -1 {
		trimmed = trimmed[:idx]
	}
	lower := strings.ToLower(strings.TrimSpace(trimmed))
	if strings.Contains(lower, "argon") {
		return AntigravityModelGroupArgon
	}
	for _, kw := range []string{"claude", "opus", "sonnet", "haiku", "fable", "gpt", "o1", "o3", "o4", "openai"} {
		if strings.Contains(lower, kw) {
			return AntigravityModelGroupClaudeGPT
		}
	}
	return AntigravityModelGroupGemini
}
