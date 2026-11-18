// Package promptguard implements request-body keyword filtering and replacement
// across client request formats before Antigravity upstream translation.
package promptguard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"antigravity-priority/internal/config"
)

const (
	FormatAuto      = "auto"
	FormatText      = "text"
	FormatClaude    = "claude"
	FormatOpenAI    = "openai"
	FormatResponses = "openai-response"
	FormatGemini    = "gemini"
	FormatInteractions = "interactions"
)

type compiledRule struct {
	index   int
	rule    config.PromptGuardRule
	re      *regexp.Regexp
	replace string
}

// Engine holds pre-compiled Prompt Guard rules ready for concurrent read-only matching.
type Engine struct {
	enabled     bool
	systemRules []*compiledRule
	warnings    []string
}

// Compile compiles a validated or raw PromptGuardConfig into an immutable Engine.
// Invalid regex rules are skipped and recorded as warnings so live traffic is never blocked.
func Compile(cfg config.PromptGuardConfig) *Engine {
	rules := cfg.Rules
	if rules == nil {
		rules = config.DefaultPromptGuardRules()
	}

	eng := &Engine{
		enabled: cfg.Enabled,
	}
	if len(rules) == 0 {
		return eng
	}

	limit := min(len(rules), config.MaxPromptGuardRules)
	eng.systemRules = make([]*compiledRule, 0, limit)
	for i := range limit {
		rule := rules[i]
		rule.Normalize()
		if !rule.On || strings.TrimSpace(rule.Find) == "" {
			continue
		}
		re, err := config.CompilePromptGuardRuleRegex(rule)
		if err != nil {
			eng.warnings = append(eng.warnings, fmt.Sprintf("rule #%d (%q): %v", i+1, rule.Find, err))
			continue
		}
		cr := &compiledRule{
			index:   i,
			rule:    rule,
			re:      re,
			replace: rule.Replace,
		}
		eng.systemRules = append(eng.systemRules, cr)
	}
	return eng
}

// Enabled reports whether Prompt Guard rewriting is active.
func (e *Engine) Enabled() bool {
	return e != nil && e.enabled
}

// ActiveRuleCount returns the number of compiled enabled rules.
func (e *Engine) ActiveRuleCount() int {
	if e == nil {
		return 0
	}
	return len(e.systemRules)
}

// Warnings returns any non-fatal rule compilation warnings collected at compile time.
func (e *Engine) Warnings() []string {
	if e == nil || len(e.warnings) == 0 {
		return nil
	}
	out := make([]string, len(e.warnings))
	copy(out, e.warnings)
	return out
}

type hitCounter struct {
	total  int
	byRule map[int]int
}

func (h *hitCounter) add(ruleIndex int, count int) {
	if h == nil || count <= 0 {
		return
	}
	h.total += count
	if h.byRule == nil {
		h.byRule = make(map[int]int)
	}
	h.byRule[ruleIndex] += count
}

func rewriteText(text string, rules []*compiledRule, hits *hitCounter) (string, bool) {
	if text == "" || len(rules) == 0 {
		return text, false
	}
	out := text
	for _, cr := range rules {
		matched := 0
		out = cr.re.ReplaceAllStringFunc(out, func(string) string {
			matched++
			return cr.replace
		})
		if matched > 0 {
			hits.add(cr.index, matched)
		}
	}
	return out, out != text
}

// RewriteRequest applies compiled Prompt Guard rules to a JSON request body according
// to sourceFormat. When no text actually changes, it returns (body, false, 0) without
// re-serializing the payload.
func (e *Engine) RewriteRequest(sourceFormat string, body []byte) ([]byte, bool, int) {
	if e == nil || !e.enabled || len(e.systemRules) == 0 || len(bytes.TrimSpace(body)) == 0 {
		return body, false, 0
	}
	format := normalizeRequestFormat(sourceFormat)
	if format == "" || format == FormatAuto {
		format = DetectFormat(body)
	}
	if format == "" {
		return body, false, 0
	}
	hits := &hitCounter{}
	rewritten, changed := rewriteJSONPayload(format, body, e.systemRules, nil, hits)
	if !changed {
		return body, false, 0
	}
	return rewritten, true, hits.total
}

// PreviewRequest describes a dry-run preview request from the management UI.
type PreviewRequest struct {
	Input  string                   `json:"input"`
	Format string                   `json:"format,omitempty"`
	Scope  string                   `json:"scope,omitempty"`
	Rules  []config.PromptGuardRule `json:"rules,omitempty"`
}

// RuleMatch summarizes matches produced by one rule during dry-run preview.
type RuleMatch struct {
	Index   int    `json:"index"`
	Find    string `json:"find"`
	Replace string `json:"replace"`
	Scope   string `json:"scope"`
	Mode    string `json:"mode"`
	Hits    int    `json:"hits"`
}

// PreviewResult returns dry-run transformation details without forwarding any upstream request.
type PreviewResult struct {
	Changed        bool        `json:"changed"`
	DetectedFormat string      `json:"detected_format"`
	TotalHits      int         `json:"total_hits"`
	Output         string      `json:"output"`
	Matches        []RuleMatch `json:"matches"`
	Warnings       []string    `json:"warnings,omitempty"`
}

// Preview runs compiled rules against an arbitrary JSON payload or plain prompt text.
// Unlike RewriteRequest, Preview executes rules regardless of cfg.Enabled so users can
// test draft rules before enabling Prompt Guard globally.
func Preview(cfg config.PromptGuardConfig, req PreviewRequest) PreviewResult {
	cfg.Enabled = true
	eng := Compile(cfg)
	res := PreviewResult{
		Output:   req.Input,
		Matches:  []RuleMatch{},
		Warnings: eng.Warnings(),
	}

	raw := []byte(req.Input)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		res.DetectedFormat = FormatText
		return res
	}

	format := normalizeRequestFormat(req.Format)
	if format == "" || format == FormatAuto {
		detected := ""
		if trimmed[0] == '{' && json.Valid(trimmed) {
			detected = DetectFormat(trimmed)
		}
		if detected != "" {
			format = detected
		} else {
			format = FormatText
		}
	}
	res.DetectedFormat = format

	hits := &hitCounter{}
	if format == FormatText {
		out, changed := rewriteText(req.Input, eng.systemRules, hits)
		res.Output = out
		res.Changed = changed
	} else {
		rewritten, changed := rewriteJSONPayload(format, raw, eng.systemRules, nil, hits)
		res.Changed = changed
		if changed {
			var indented bytes.Buffer
			if err := json.Indent(&indented, rewritten, "", "  "); err == nil {
				res.Output = indented.String()
			} else {
				res.Output = string(rewritten)
			}
		}
	}

	res.TotalHits = hits.total
	for _, cr := range eng.systemRules {
		count := hits.byRule[cr.index]
		if count <= 0 {
			continue
		}
		res.Matches = append(res.Matches, RuleMatch{
			Index:   cr.index,
			Find:    cr.rule.Find,
			Replace: cr.rule.Replace,
			Scope:   cr.rule.Scope,
			Mode:    cr.rule.Mode,
			Hits:    count,
		})
	}
	return res
}
