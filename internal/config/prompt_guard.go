package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxPromptGuardRules caps the number of configured Prompt Guard rewrite rules.
	MaxPromptGuardRules = 200

	// PromptGuardScopeSystem applies a rule only to system / developer instructions.
	PromptGuardScopeSystem = "system"
	// PromptGuardScopeAll applies a rule across system instructions and conversation messages.
	PromptGuardScopeAll = "all"

	// PromptGuardModeText treats Find as a case-insensitive, whitespace-flexible phrase.
	PromptGuardModeText = "text"
	// PromptGuardModeRegex treats Find as a RE2 regular expression.
	PromptGuardModeRegex = "regex"
)

// PromptGuardRule defines a single phrase/regex rewrite rule for Antigravity requests.
type PromptGuardRule struct {
	On      bool   `json:"on"`
	Find    string `json:"find"`
	Replace string `json:"replace"`
	Scope   string `json:"scope"`
	Mode    string `json:"mode"`
	Flags   string `json:"flags,omitempty"`
	Custom  bool   `json:"custom,omitempty"`
}

// PromptGuardConfig controls request-body keyword filtering and replacement for Antigravity.
//
// Rules == nil or empty means "no rules active"; rules are synchronized from the remote phrases.json repository.
type PromptGuardConfig struct {
	Enabled bool              `json:"enabled"`
	SyncURL string            `json:"sync_url,omitempty"`
	Rules   []PromptGuardRule `json:"rules"`
}

// DefaultPromptGuardConfig returns the default Prompt Guard configuration (disabled
// until enabled by the user, with empty rules until synchronized from phrases.json).
func DefaultPromptGuardConfig() PromptGuardConfig {
	return PromptGuardConfig{
		Enabled: false,
		SyncURL: "",
		Rules:   DefaultPromptGuardRules(),
	}
}

// DefaultPromptGuardRules returns an empty slice as there are no built-in default rules;
// rules are synchronized from the remote phrases.json repository or added by the user.
func DefaultPromptGuardRules() []PromptGuardRule {
	return []PromptGuardRule{}
}

// UnmarshalJSON distinguishes an omitted/null rules field from an explicit empty array (`"rules": []`).
func (p *PromptGuardConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled *bool            `json:"enabled"`
		SyncURL *string          `json:"sync_url"`
		Rules   *json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Enabled != nil {
		p.Enabled = *raw.Enabled
	}
	if raw.SyncURL != nil {
		p.SyncURL = strings.TrimSpace(*raw.SyncURL)
	}
	if raw.Rules == nil || string(*raw.Rules) == "null" {
		p.Rules = []PromptGuardRule{}
		return nil
	}
	var rules []PromptGuardRule
	if err := json.Unmarshal(*raw.Rules, &rules); err != nil {
		return err
	}
	if rules == nil {
		rules = []PromptGuardRule{}
	}
	p.Rules = rules
	p.Normalize()
	return nil
}

// Clone returns a deep copy of PromptGuardConfig preserving nil vs empty-slice distinction.
func (p PromptGuardConfig) Clone() PromptGuardConfig {
	out := PromptGuardConfig{
		Enabled: p.Enabled,
		SyncURL: p.SyncURL,
	}
	if p.Rules == nil {
		out.Rules = []PromptGuardRule{}
		return out
	}
	out.Rules = make([]PromptGuardRule, len(p.Rules))
	copy(out.Rules, p.Rules)
	return out
}

// Normalize canonicalizes rule scopes, modes, and flags while preserving an
// explicit empty slice when the user cleared all rules.
func (p *PromptGuardConfig) Normalize() {
	if p.Rules == nil {
		p.Rules = []PromptGuardRule{}
		return
	}
	for i := range p.Rules {
		p.Rules[i].Normalize()
	}
}

// Normalize canonicalizes scope, mode, and flags on a single rule.
func (r *PromptGuardRule) Normalize() {
	scope := strings.ToLower(strings.TrimSpace(r.Scope))
	switch scope {
	case PromptGuardScopeAll:
		r.Scope = PromptGuardScopeAll
	case "", PromptGuardScopeSystem:
		r.Scope = PromptGuardScopeSystem
	default:
		r.Scope = scope
	}

	mode := strings.ToLower(strings.TrimSpace(r.Mode))
	switch mode {
	case PromptGuardModeRegex:
		r.Mode = PromptGuardModeRegex
	case "", PromptGuardModeText:
		r.Mode = PromptGuardModeText
	default:
		r.Mode = mode
	}

	r.Flags = strings.TrimSpace(r.Flags)
}

// Validate checks bounds and regex compilability of PromptGuardConfig.
func (p PromptGuardConfig) Validate() error {
	if len(p.Rules) > MaxPromptGuardRules {
		return fmt.Errorf("prompt_guard.rules exceeds maximum of %d rules (got %d)", MaxPromptGuardRules, len(p.Rules))
	}
	for i, rule := range p.Rules {
		scope := strings.ToLower(strings.TrimSpace(rule.Scope))
		if scope != "" && scope != PromptGuardScopeSystem && scope != PromptGuardScopeAll {
			return fmt.Errorf("prompt_guard.rules[%d].scope must be %q or %q, got %q", i, PromptGuardScopeSystem, PromptGuardScopeAll, rule.Scope)
		}
		mode := strings.ToLower(strings.TrimSpace(rule.Mode))
		if mode != "" && mode != PromptGuardModeText && mode != PromptGuardModeRegex {
			return fmt.Errorf("prompt_guard.rules[%d].mode must be %q or %q, got %q", i, PromptGuardModeText, PromptGuardModeRegex, rule.Mode)
		}
		if mode == PromptGuardModeRegex && rule.On && strings.TrimSpace(rule.Find) != "" {
			if _, err := CompilePromptGuardRuleRegex(rule); err != nil {
				return fmt.Errorf("prompt_guard.rules[%d]: %w", i, err)
			}
		}
	}
	return nil
}

// CompilePromptGuardRuleRegex compiles a single PromptGuardRule into a RE2 *regexp.Regexp.
// Text-mode rules use case-insensitive whitespace-flexible matching; regex-mode rules
// map supported flags (g, i, m, s) into Go RE2 inline flags (defaulting to case-insensitive).
func CompilePromptGuardRuleRegex(rule PromptGuardRule) (*regexp.Regexp, error) {
	if strings.TrimSpace(rule.Find) == "" {
		return nil, fmt.Errorf("empty find pattern")
	}
	mode := strings.ToLower(strings.TrimSpace(rule.Mode))
	if mode == PromptGuardModeRegex {
		inlineFlags, err := parsePromptGuardRegexFlags(rule.Flags)
		if err != nil {
			return nil, err
		}
		pattern := rule.Find
		if inlineFlags != "" {
			pattern = "(?" + inlineFlags + ")(?:" + rule.Find + ")"
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %w", rule.Find, err)
		}
		return re, nil
	}

	fields := strings.Fields(strings.TrimSpace(rule.Find))
	if len(fields) == 0 {
		return nil, fmt.Errorf("empty find pattern")
	}
	escaped := make([]string, len(fields))
	for i, f := range fields {
		escaped[i] = regexp.QuoteMeta(f)
	}
	pattern := "(?i)" + strings.Join(escaped, `\s+`)
	return regexp.Compile(pattern)
}

func parsePromptGuardRegexFlags(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return "i", nil
	}
	var hasI, hasM, hasS bool
	for _, ch := range trimmed {
		switch ch {
		case 'g':
			// Go regexp ReplaceAll* is always global.
		case 'i':
			hasI = true
		case 'm':
			hasM = true
		case 's':
			hasS = true
		default:
			return "", fmt.Errorf("unsupported regex flag %q (supported: g, i, m, s)", string(ch))
		}
	}
	var b strings.Builder
	if hasI {
		b.WriteByte('i')
	}
	if hasM {
		b.WriteByte('m')
	}
	if hasS {
		b.WriteByte('s')
	}
	return b.String(), nil
}
