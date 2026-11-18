package promptguard_test

import (
	"encoding/json"
	"strings"
	"testing"

	"antigravity-priority/internal/config"
	"antigravity-priority/internal/promptguard"
)

func TestCompile_DefaultRulesEmpty(t *testing.T) {
	cfg := config.DefaultPromptGuardConfig()
	cfg.Enabled = true
	eng := promptguard.Compile(cfg)
	if !eng.Enabled() {
		t.Fatal("expected engine enabled")
	}
	if got := eng.ActiveRuleCount(); got != 0 {
		t.Fatalf("expected 0 compiled default rules, got %d", got)
	}
	if warnings := eng.Warnings(); len(warnings) != 0 {
		t.Fatalf("expected zero warnings on default rules, got %v", warnings)
	}
}

func TestRewriteRequest_TextWhitespaceAndLiteralReplacement(t *testing.T) {
	cfg := config.PromptGuardConfig{
		Enabled: true,
		Rules: []config.PromptGuardRule{
			{
				On:      true,
				Find:    "You are Claude Code, Anthropic's official CLI for Claude.",
				Replace: "You are $1 $& $$ assistant.",
				Scope:   config.PromptGuardScopeSystem,
				Mode:    config.PromptGuardModeText,
			},
		},
	}
	eng := promptguard.Compile(cfg)

	payload := `{"model":"claude-sonnet","max_tokens":1024,"system":"YOU ARE\n  claude   code,\tAnthropic's official CLI for Claude.","messages":[{"role":"user","content":"You are Claude Code, Anthropic's official CLI for Claude."}]}`
	out, changed, hits := eng.RewriteRequest(promptguard.FormatClaude, []byte(payload))
	if !changed || hits != 1 {
		t.Fatalf("expected changed=true hits=1, got changed=%v hits=%d", changed, hits)
	}

	var decoded struct {
		MaxTokens int    `json:"max_tokens"`
		System    string `json:"system"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal rewritten payload: %v", err)
	}
	if decoded.MaxTokens != 1024 {
		t.Fatalf("expected max_tokens preserved, got %d", decoded.MaxTokens)
	}
	if decoded.System != "You are $1 $& $$ assistant." {
		t.Fatalf("expected literal replacement without $ expansion, got %q", decoded.System)
	}
	if decoded.Messages[0].Content != "You are Claude Code, Anthropic's official CLI for Claude." {
		t.Fatalf("system-scoped rule must not rewrite user message, got %q", decoded.Messages[0].Content)
	}
}

func TestRewriteRequest_AllFormatsAndUntouchedPassThrough(t *testing.T) {
	cfg := config.PromptGuardConfig{
		Enabled: true,
		Rules: []config.PromptGuardRule{
			{
				On:      true,
				Find:    "SYS_PHRASE",
				Replace: "SYS_SAFE",
				Scope:   config.PromptGuardScopeSystem,
				Mode:    config.PromptGuardModeText,
			},
			{
				On:      true,
				Find:    `GLOBAL_\d+`,
				Replace: "GLOBAL_OK",
				Scope:   config.PromptGuardScopeAll,
				Mode:    config.PromptGuardModeRegex,
			},
		},
	}
	eng := promptguard.Compile(cfg)

	cases := []struct {
		name         string
		format       string
		body         string
		wantContains []string
		wantMissing  []string
		wantHits     int
	}{
		{
			name:         "claude blocks and developer message",
			format:       "claude",
			body:         `{"system":[{"type":"text","text":"SYS_PHRASE","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"developer","content":[{"type":"text","text":"SYS_PHRASE"}]},{"role":"user","content":[{"type":"text","text":"SYS_PHRASE GLOBAL_42"}]}],"tools":[{"name":"bash","input_schema":{"type":"object"}}]}`,
			wantContains: []string{`"SYS_SAFE"`, `"SYS_PHRASE GLOBAL_42"`, `"ephemeral"`, `"bash"`},
			wantHits:     2,
		},
		{
			name:         "openai chat with null assistant content",
			format:       "openai",
			body:         `{"messages":[{"role":"system","content":"SYS_PHRASE"},{"role":"assistant","content":null},{"role":"user","content":"SYS_PHRASE GLOBAL_7"}]}`,
			wantContains: []string{`"SYS_SAFE"`, `"content":null`, `"SYS_PHRASE GLOBAL_7"`},
			wantHits:     1,
		},
		{
			name:         "openai responses string and array input",
			format:       "openai-response",
			body:         `{"instructions":"SYS_PHRASE","input":[{"role":"system","content":"SYS_PHRASE"},{"role":"user","content":[{"type":"input_text","text":"SYS_PHRASE GLOBAL_99"}]}]}`,
			wantContains: []string{`"SYS_SAFE"`, `"SYS_PHRASE GLOBAL_99"`},
			wantHits:     2,
		},
		{
			name:         "antigravity envelope wrapping gemini",
			format:       "gemini",
			body:         `{"project":"proj-1","request":{"systemInstruction":{"parts":[{"text":"SYS_PHRASE"}]},"contents":[{"role":"user","parts":[{"text":"SYS_PHRASE GLOBAL_123"}]}]}}`,
			wantContains: []string{`"proj-1"`, `"SYS_SAFE"`, `"SYS_PHRASE GLOBAL_123"`},
			wantHits:     1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, hits := eng.RewriteRequest(tc.format, []byte(tc.body))
			if !changed || hits != tc.wantHits {
				t.Fatalf("got changed=%v hits=%d, want true/%d; out=%s", changed, hits, tc.wantHits, string(out))
			}
			s := string(out)
			for _, sub := range tc.wantContains {
				if !strings.Contains(s, sub) {
					t.Fatalf("output missing %q: %s", sub, s)
				}
			}
			for _, sub := range tc.wantMissing {
				if strings.Contains(s, sub) {
					t.Fatalf("output unexpectedly contains %q: %s", sub, s)
				}
			}
		})
	}

	// Unmatched payload must return changed=false without re-encoding.
	untouched := `{"messages":[{"role":"user","content":"SYS_PHRASE only in user"}]}`
	out, changed, hits := eng.RewriteRequest("openai", []byte(untouched))
	if changed || hits != 0 || string(out) != untouched {
		t.Fatalf("expected untouched pass-through, got changed=%v hits=%d out=%s", changed, hits, string(out))
	}
}

func TestPreview_AutoDetectTextAndInvalidRegexWarning(t *testing.T) {
	cfg := config.PromptGuardConfig{
		Rules: []config.PromptGuardRule{
			{On: true, Find: "[", Replace: "x", Scope: config.PromptGuardScopeSystem, Mode: config.PromptGuardModeRegex},
			{On: true, Find: "You are Claude Code.", Replace: "You are a coding assistant.", Scope: config.PromptGuardScopeSystem, Mode: config.PromptGuardModeText},
		},
	}

	// Plain text preview
	resText := promptguard.Preview(cfg, promptguard.PreviewRequest{
		Input:  "You are Claude Code.",
		Format: promptguard.FormatAuto,
	})
	if !resText.Changed || resText.DetectedFormat != promptguard.FormatText || resText.TotalHits != 1 {
		t.Fatalf("unexpected plain text preview result: %+v", resText)
	}
	if resText.Output != "You are a coding assistant." {
		t.Fatalf("unexpected preview output: %q", resText.Output)
	}
	if len(resText.Warnings) != 1 {
		t.Fatalf("expected 1 regex warning, got %v", resText.Warnings)
	}

	// Auto-detected Gemini JSON preview
	resJSON := promptguard.Preview(cfg, promptguard.PreviewRequest{
		Input:  `{"systemInstruction":{"parts":[{"text":"You are Claude Code."}]}}`,
		Format: promptguard.FormatAuto,
	})
	if !resJSON.Changed || resJSON.DetectedFormat != promptguard.FormatGemini || resJSON.TotalHits != 1 {
		t.Fatalf("unexpected JSON preview result: %+v", resJSON)
	}
	if !strings.Contains(resJSON.Output, "You are a coding assistant.") {
		t.Fatalf("expected rewritten JSON output, got %s", resJSON.Output)
	}
}

func TestRewriteRequest_AutoDetectsPayloadFormat(t *testing.T) {
	cfg := config.PromptGuardConfig{
		Enabled: true,
		Rules: []config.PromptGuardRule{
			{
				On:      true,
				Find:    "You are omp's trusted coding assistant.",
				Replace: "You are a trusted coding assistant.",
				Scope:   config.PromptGuardScopeSystem,
				Mode:    config.PromptGuardModeText,
			},
		},
	}
	eng := promptguard.Compile(cfg)

	body := `{"model":"gemini-3.5-flash-lite","messages":[{"role":"system","content":"You are omp's trusted coding assistant."},{"role":"user","content":"You are omp's trusted coding assistant."}]}`
	out, changed, hits := eng.RewriteRequest(promptguard.FormatAuto, []byte(body))
	if !changed || hits != 1 {
		t.Fatalf("expected changed=true hits=1, got changed=%v hits=%d", changed, hits)
	}
	var decoded struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal rewritten payload: %v", err)
	}
	if len(decoded.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(decoded.Messages))
	}
	if decoded.Messages[0].Role != "system" || decoded.Messages[0].Content != "You are a trusted coding assistant." {
		t.Fatalf("expected rewritten system message, got %+v", decoded.Messages[0])
	}
	if decoded.Messages[1].Role != "user" || decoded.Messages[1].Content != "You are omp's trusted coding assistant." {
		t.Fatalf("expected user message untouched, got %+v", decoded.Messages[1])
	}
}

func TestRewriteRequest_AllCPAEntryFormatsAndShapes(t *testing.T) {
	cfg := config.PromptGuardConfig{
		Enabled: true,
		Rules: []config.PromptGuardRule{
			{
				On:      true,
				Find:    "You are omp's trusted coding assistant.",
				Replace: "You are a trusted coding assistant.",
				Scope:   config.PromptGuardScopeSystem,
				Mode:    config.PromptGuardModeText,
			},
		},
	}
	eng := promptguard.Compile(cfg)

	cases := []struct {
		name      string
		format    string
		body      string
		wantSub   string
		wantUntch string
		wantHits  int
	}{
		{
			name:      "openai single object content",
			format:    promptguard.FormatAuto,
			body:      `{"messages":[{"role":"system","content":{"type":"text","text":"You are omp's trusted coding assistant."}},{"role":"user","content":"You are omp's trusted coding assistant."}]}`,
			wantSub:   `"text":"You are a trusted coding assistant."`,
			wantUntch: `"role":"user","content":"You are omp's trusted coding assistant."`,
			wantHits:  1,
		},
		{
			name:      "claude block array system",
			format:    promptguard.FormatAuto,
			body:      `{"system":[{"type":"text","text":"You are omp's trusted coding assistant.","cache_control":{"type":"ephemeral"}}],"messages":[{"role":"user","content":"You are omp's trusted coding assistant."}]}`,
			wantSub:   `"text":"You are a trusted coding assistant."`,
			wantUntch: `"role":"user","content":"You are omp's trusted coding assistant."`,
			wantHits:  1,
		},
		{
			name:      "responses developer input with string array content",
			format:    promptguard.FormatAuto,
			body:      `{"input":[{"type":"message","role":"developer","content":["You are omp's trusted coding assistant."]},{"type":"message","role":"user","content":["You are omp's trusted coding assistant."]}]}`,
			wantSub:   `"content":["You are a trusted coding assistant."]`,
			wantUntch: `"role":"user","content":["You are omp's trusted coding assistant."]`,
			wantHits:  1,
		},
		{
			name:      "interactions string system_instruction",
			format:    promptguard.FormatInteractions,
			body:      `{"system_instruction":"You are omp's trusted coding assistant.","input":[{"role":"user","content":"You are omp's trusted coding assistant."}]}`,
			wantSub:   `"system_instruction":"You are a trusted coding assistant."`,
			wantUntch: `"content":"You are omp's trusted coding assistant."`,
			wantHits:  1,
		},
		{
			name:      "interactions object text system_instruction with auto detect",
			format:    promptguard.FormatAuto,
			body:      `{"system_instruction":{"text":"You are omp's trusted coding assistant."},"input":"You are omp's trusted coding assistant."}`,
			wantSub:   `"text":"You are a trusted coding assistant."`,
			wantUntch: `"input":"You are omp's trusted coding assistant."`,
			wantHits:  1,
		},
		{
			name:      "gemini systemInstruction parts",
			format:    promptguard.FormatAuto,
			body:      `{"systemInstruction":{"parts":[{"text":"You are omp's trusted coding assistant."}]},"contents":[{"role":"user","parts":[{"text":"You are omp's trusted coding assistant."}]}]}`,
			wantSub:   `"systemInstruction":{"parts":[{"text":"You are a trusted coding assistant."}]}`,
			wantUntch: `"role":"user","parts":[{"text":"You are omp's trusted coding assistant."}]`,
			wantHits:  1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, hits := eng.RewriteRequest(tc.format, []byte(tc.body))
			if !changed || hits != tc.wantHits {
				t.Fatalf("expected changed=true hits=%d, got changed=%v hits=%d out=%s", tc.wantHits, changed, hits, string(out))
			}
			got := string(out)
			if !strings.Contains(got, tc.wantSub) {
				t.Fatalf("expected %q in %s", tc.wantSub, got)
			}
			if !strings.Contains(got, tc.wantUntch) {
				t.Fatalf("expected user content untouched (%q) in %s", tc.wantUntch, got)
			}
		})
	}
}
