package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func extractPluginConfigYAML(data string) string {
	if hasTopLevelPluginField(data) {
		return data
	}
	lines := strings.Split(data, "\n")
	hasHostPlugins := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "plugins:" && leadingSpaces(line) == 0 {
			hasHostPlugins = true
			break
		}
	}

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "antigravity-priority:" && trimmed != PluginID+":" {
			continue
		}
		if !isCPAPluginConfigPath(lines, index) {
			continue
		}
		indent := leadingSpaces(line)
		collected := collectIndentedBlock(lines[index+1:], indent)
		if len(collected) == 0 {
			return ""
		}
		return strings.Join(collected, "\n")
	}

	// If this is a full CPA config.yaml that doesn't define an antigravity-priority block,
	// return empty so Default() configuration is used instead of failing on host-level fields.
	if hasHostPlugins {
		return ""
	}
	return data
}

func hasTopLevelPluginField(data string) bool {
	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || leadingSpaces(line) != 0 {
			continue
		}
		key, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "enabled", "auto_apply", "interval", "antigravity_model_group", "max_concurrency", "min_change", "state_cache_path", "cache_path", "priority_rules":
			return true
		}
	}
	return false
}

func isCPAPluginConfigPath(lines []string, pluginIndex int) bool {
	pluginIndent := leadingSpaces(lines[pluginIndex])
	configsIndent := -1
	for index := pluginIndex - 1; index >= 0; index-- {
		trimmed := strings.TrimSpace(lines[index])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(lines[index])
		if indent >= pluginIndent {
			continue
		}
		if configsIndent < 0 {
			if trimmed != "configs:" {
				return false
			}
			configsIndent = indent
			continue
		}
		return trimmed == "plugins:" && indent < configsIndent
	}
	return false
}

func collectIndentedBlock(lines []string, parentIndent int) []string {
	collected := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := leadingSpaces(line)
		if indent <= parentIndent {
			break
		}
		collected = append(collected, line)
	}
	return normalizeIndentedBlock(collected)
}

func normalizeIndentedBlock(lines []string) []string {
	baseIndent := -1
	for _, line := range lines {
		indent := leadingSpaces(line)
		if baseIndent < 0 || indent < baseIndent {
			baseIndent = indent
		}
	}
	if baseIndent <= 0 {
		return lines
	}
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		if leadingSpaces(line) < baseIndent {
			normalized = append(normalized, strings.TrimLeft(line, " "))
			continue
		}
		normalized = append(normalized, line[baseIndent:])
	}
	return normalized
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func parseYAMLMap(data string) (map[string]any, error) {
	result := map[string]any{}
	priorityRules := map[string]any{}
	section := ""
	sectionIndent := -1

	for _, line := range strings.Split(data, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, invalid("config", trimmed, "must use key: value syntax")
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if indent == 0 {
			section = key
			sectionIndent = indent
			if key == "priority_rules" {
				result[key] = priorityRules
				continue
			}
			result[key] = yamlScalar(value)
			continue
		}
		if section == "priority_rules" {
			if indent <= sectionIndent {
				continue
			}
			priorityRules[key] = yamlScalar(value)
		}
	}
	normalizePriorityRulesKeys(result)
	return result, nil
}

func normalizePriorityRulesKeys(root map[string]any) {
	if root == nil {
		return
	}
	const prefix = "priority_rules."
	flatKeys := make([]string, 0)
	for key := range root {
		if strings.HasPrefix(key, prefix) {
			flatKeys = append(flatKeys, key)
		}
	}
	if len(flatKeys) == 0 {
		return
	}
	nested, _ := root["priority_rules"].(map[string]any)
	if nested == nil {
		nested = map[string]any{}
		root["priority_rules"] = nested
	}
	for _, key := range flatKeys {
		field := strings.TrimPrefix(key, prefix)
		if field == "" {
			delete(root, key)
			continue
		}
		nested[field] = root[key]
		delete(root, key)
	}
}

func parseDuration(field string, value string) (time.Duration, error) {
	text := yamlText(value)
	durationText := text
	if _, err := strconv.Atoi(text); err == nil {
		durationText = text + "m"
	}
	parsed, err := time.ParseDuration(durationText)
	if err != nil || parsed <= 0 {
		return 0, invalid(field, text, "must be a positive duration")
	}
	return parsed, nil
}

// parseDurationTolerant parses a duration string with case normalization.
// Returns (duration, normalizedForm, error). normalizedForm is non-empty when
// case normalization was applied (e.g. "30M" → "30m").
func parseDurationTolerant(field string, value string) (time.Duration, string, error) {
	text := yamlText(value)
	durationText := text
	if _, err := strconv.Atoi(text); err == nil {
		durationText = text + "m"
	}

	// Try parsing as-is first
	parsed, err := time.ParseDuration(durationText)
	if err == nil && parsed > 0 {
		return parsed, "", nil
	}

	// Try case-normalized version
	normalized := normalizeDurationCase(durationText)
	if normalized != durationText {
		parsed, err = time.ParseDuration(normalized)
		if err == nil && parsed > 0 {
			return parsed, normalized, nil
		}
	}

	return 0, "", invalid(field, text, "must be a positive duration")
}

// normalizeDurationCase converts uppercase duration units to lowercase.
// Supports: S→s, M→m (when not preceded by digit for ms), H→h, MS→ms.
func normalizeDurationCase(s string) string {
	return strings.ToLower(s)
}

func yamlScalar(value string) any {
	text := yamlText(value)
	if parsed, err := strconv.Atoi(text); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseBool(text); err == nil {
		return parsed
	}
	return text
}

func yamlText(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) > 1 && trimmed[0] == trimmed[len(trimmed)-1] && (trimmed[0] == '"' || trimmed[0] == '\'') {
		return trimmed[1 : len(trimmed)-1]
	}
	return trimmed
}

func invalid(field string, value string, reason string) error {
	return fmt.Errorf("%w: %s=%q %s", ErrInvalidConfig, field, value, reason)
}
