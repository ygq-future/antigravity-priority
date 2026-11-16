package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxPromptGuardRules caps the number of configured Prompt Guard rewrite rules.
	MaxPromptGuardRules = 50

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
}

// PromptGuardConfig controls request-body keyword filtering and replacement for Antigravity.
//
// Rules == nil means "not customized, seed from DefaultPromptGuardRules()".
// Rules != nil with len == 0 means "explicitly cleared by the user; keep empty".
type PromptGuardConfig struct {
	Enabled bool              `json:"enabled"`
	Rules   []PromptGuardRule `json:"rules"`
}

// DefaultPromptGuardConfig returns the default Prompt Guard configuration (disabled
// until enabled by the user, pre-populated with the built-in Antigravity rules).
func DefaultPromptGuardConfig() PromptGuardConfig {
	return PromptGuardConfig{
		Enabled: false,
		Rules:   DefaultPromptGuardRules(),
	}
}

// DefaultPromptGuardRules returns a fresh copy of the 14 built-in rules ported from
// magpie-prompt-guard.
func DefaultPromptGuardRules() []PromptGuardRule {
	return []PromptGuardRule{
		{
			On:      true,
			Find:    "You are a Claude agent, built on Anthropic's Claude Agent SDK.",
			Replace: "You are a coding assistant.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are a Claude agent, built on the Claude Agent SDK.",
			Replace: "You are a coding assistant.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Claude Code, Anthropic's official CLI for Claude.",
			Replace: "You are a coding assistant.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Claude Code, Anthropic's CLI for Claude.",
			Replace: "You are a coding assistant.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Claude Code.",
			Replace: "You are a coding assistant.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "built on Anthropic's Claude Agent SDK",
			Replace: "built on the Claude Agent SDK by Anthropic",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Codex, a coding agent built on GPT-5",
			Replace: "You are Codex, a coding agent built on GPT-5",
			Scope:   PromptGuardScopeAll,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Codex, an agent built on GPT-5",
			Replace: "You are Codex, an agent built on GPT-5",
			Scope:   PromptGuardScopeAll,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You're a coding agent running in the Codex CLI",
			Replace: "You're a coding agent running in the Codex CLI",
			Scope:   PromptGuardScopeAll,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "RFC 2119 keywords: MUST, REQUIRED, SHOULD, RECOMMENDED, MAY, OPTIONAL.",
			Replace: "RFC 2119 key words: MUST, REQUIRED, SHOULD, RECOMMENDED, MAY, OPTIONAL.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are Hermes Agent",
			Replace: "You're Hermes Agent",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "You are OpenCode",
			Replace: "You're OpenCode",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "a coding assistant running inside pi",
			Replace: "a coding assistant running within pi",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
		{
			On:      true,
			Find:    "Read xd://<tool> for full docs + JSON schema before first use.",
			Replace: "Before first use, read xd://<tool> for full docs and the JSON schema.",
			Scope:   PromptGuardScopeSystem,
			Mode:    PromptGuardModeText,
		},
	}
}

// UnmarshalJSON distinguishes an omitted/null rules field (falls back to built-in
// defaults) from an explicit empty array (`"rules": []`, preserved as empty).
func (p *PromptGuardConfig) UnmarshalJSON(data []byte) error {
	var raw struct {
		Enabled *bool            `json:"enabled"`
		Rules   *json.RawMessage `json:"rules"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Enabled != nil {
		p.Enabled = *raw.Enabled
	}
	if raw.Rules == nil || string(*raw.Rules) == "null" {
		p.Rules = DefaultPromptGuardRules()
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
	}
	if p.Rules == nil {
		out.Rules = DefaultPromptGuardRules()
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
		p.Rules = DefaultPromptGuardRules()
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
