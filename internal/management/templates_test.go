package management_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"antigravity-priority/internal/management"
)

func TestHandler_Status_HTML_ServesEmbeddedTemplate(t *testing.T) {
	handler := management.NewHandler(&mockRunner{})

	// Test resource route
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set(management.RouteSourceHeader, "resource")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected content-type text/html; charset=utf-8, got %q", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Antigravity Priority") {
		t.Errorf("expected HTML body to contain 'Antigravity Priority'")
	}
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Errorf("expected HTML body to start with doctype")
	}

	// Test direct route without header
	reqDirect := httptest.NewRequest(http.MethodGet, "/status", nil)
	recDirect := httptest.NewRecorder()
	handler.ServeHTTP(recDirect, reqDirect)

	if recDirect.Code != http.StatusOK {
		t.Fatalf("expected direct /status to return 200, got %d", recDirect.Code)
	}
	if ct := recDirect.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("expected content-type text/html; charset=utf-8, got %q", ct)
	}
}

func TestStatusHTML_NoRemoteURLs(t *testing.T) {
	html := management.StatusHTML

	// Strict security check: no external URLs
	re := regexp.MustCompile(`https?://`)
	matches := re.FindAllString(html, -1)
	if len(matches) > 0 {
		t.Fatalf("security violation: StatusHTML contains %d remote URL reference(s)", len(matches))
	}
}

func TestStatusHTML_CSSTokenCompleteness(t *testing.T) {
	html := management.StatusHTML

	// Extract the style block
	styleStart := strings.Index(html, "<style>")
	styleEnd := strings.Index(html, "</style>")
	if styleStart == -1 || styleEnd == -1 {
		t.Fatalf("could not find <style> block in StatusHTML")
	}
	styleCSS := html[styleStart+7 : styleEnd]

	// Extract :root block tokens (light theme)
	rootTokens := extractCSSTokens(t, styleCSS, ":root {", "}")
	if len(rootTokens) == 0 {
		t.Fatalf("expected CSS variables in :root, found none")
	}

	// Extract dark media query tokens
	darkMediaTokens := extractCSSTokens(t, styleCSS, ":root:not([data-theme=\"light\"]) {", "}")
	if len(darkMediaTokens) == 0 {
		t.Fatalf("expected CSS variables in @media dark block, found none")
	}

	// Extract explicit [data-theme="dark"] tokens
	darkAttrTokens := extractCSSTokens(t, styleCSS, ":root[data-theme=\"dark\"] {", "}")
	if len(darkAttrTokens) == 0 {
		t.Fatalf("expected CSS variables in :root[data-theme=\"dark\"] block, found none")
	}

	// Ensure all required core tokens exist in root
	requiredTokens := []string{
		"--bg-primary",
		"--bg-surface",
		"--bg-card",
		"--bg-subtle",
		"--border-color",
		"--border-subtle",
		"--text-primary",
		"--text-secondary",
		"--text-muted",
		"--accent-blue",
		"--accent-green",
		"--accent-yellow",
		"--accent-red",
		"--accent-purple",
		"--meter-bg",
		"--meter-fill",
		"--meter-warn",
		"--meter-danger",
		"--badge-boost-bg",
		"--badge-boost-border",
		"--badge-boost-text",
		"--diff-from-bg",
		"--diff-from-text",
		"--diff-to-bg",
		"--diff-to-text",
	}

	for _, token := range requiredTokens {
		if _, ok := rootTokens[token]; !ok {
			t.Errorf("missing required token in :root: %s", token)
		}
		if _, ok := darkMediaTokens[token]; !ok {
			t.Errorf("missing required token in dark media query: %s", token)
		}
		if _, ok := darkAttrTokens[token]; !ok {
			t.Errorf("missing required token in data-theme='dark': %s", token)
		}
	}

	// Completeness verification: Every token defined in :root must have a dark counterpart
	for token := range rootTokens {
		if _, ok := darkMediaTokens[token]; !ok {
			t.Errorf("token %q defined in :root but missing in @media (prefers-color-scheme: dark)", token)
		}
		if _, ok := darkAttrTokens[token]; !ok {
			t.Errorf("token %q defined in :root but missing in :root[data-theme=\"dark\"]", token)
		}
	}
}

func TestStatusHTML_AllReferencedVarsDefined(t *testing.T) {
	html := management.StatusHTML
	styleStart := strings.Index(html, "<style>")
	styleEnd := strings.Index(html, "</style>")
	if styleStart == -1 || styleEnd == -1 {
		t.Fatalf("could not find <style> block in StatusHTML")
	}
	styleCSS := html[styleStart+7 : styleEnd]

	rootTokens := extractCSSTokens(t, styleCSS, ":root {", "}")

	// Find all var(--xyz) occurrences
	reVar := regexp.MustCompile(`var\((--[a-zA-Z0-9_-]+)\)`)
	matches := reVar.FindAllStringSubmatch(styleCSS, -1)

	for _, match := range matches {
		varName := match[1]
		if _, ok := rootTokens[varName]; !ok {
			t.Errorf("CSS uses var(%s) but %s is not defined in :root", varName, varName)
		}
	}
}

func TestStatusHTML_ContainsRequiredUIElements(t *testing.T) {
	html := management.StatusHTML

	checks := []struct {
		name     string
		contains string
	}{
		{"5h Short Window meter", "shortWindow"},
		{"7d Weekly Window meter", "longWindow"},
		{"Countdown clocks", "meter-countdown"},
		{"Adaptive C_cycle burn rate", "burnLabel"},
		{"Weekly urgency index", "urgencyLabel"},
		{"Boost badge styling", "badge-boost"},
		{"Model Group dropdown", "modelGroupSelect"},
		{"Gemini model group option", "gemini"},
		{"Claude GPT model group option", "claude_gpt"},
		{"Probe action button", "btnProbe"},
		{"Apply action button", "btnApply"},
		{"Diff modal container", "diffModal"},
		{"System font stack", "-apple-system"},
		{"Confirm modal container", "confirmModal"},
		{"Config center panel", "panelConfig"},
		{"Auto apply toggle", "cfgAutoApply"},
		{"Interval select", "cfgIntervalSelect"},
		{"Save config button", "btnSaveConfig"},
		{"Reset config button", "btnResetConfig"},
		{"Diagnostics panel", "panelDiagnostics"},
		{"Diagnostics KPI grid", "diag-kpi-grid"},
		{"Diagnostics copy button", "btnCopyDiagnostics"},
		{"Diagnostics scheduler section", "diagSectionScheduler"},
		{"Diagnostics cooldown section", "diagSectionCooldown"},
		{"Diagnostics audit section", "diagSectionAudit"},
	}

	for _, c := range checks {
		if !strings.Contains(html, c.contains) {
			t.Errorf("StatusHTML missing %s (search string: %q)", c.name, c.contains)
		}
	}

	// Verify raw JSON pre block is completely removed
	if strings.Contains(html, "id=\"rawDiagnostics\"") {
		t.Errorf("StatusHTML should not contain rawDiagnostics pre block")
	}

	// Verify old inaccurate "宿主已手动禁用" is removed
	if strings.Contains(html, "宿主已手动禁用") {
		t.Errorf("StatusHTML should not contain '宿主已手动禁用'")
	}
	// Verify "priority <= 0" is removed
	if strings.Contains(html, "priority <= 0") {
		t.Errorf("StatusHTML should not contain 'priority <= 0'")
	}
	if strings.Contains(html, "cfgRulesEnabled") || strings.Contains(html, "enabled: rulesEnabled") {
		t.Error("StatusHTML must not expose the removed priority_rules.enabled switch")
	}
	if !strings.Contains(html, `if (tabId === "overview") refreshDashboard(true)`) {
		t.Error("returning to Overview must use the same synchronized refresh path")
	}
	if !strings.Contains(html, "latestDiagnostics.latest_apply") || !strings.Contains(html, "No Apply write recorded yet") {
		t.Error("diagnostics write-health UI must use latest_apply and expose the no-Apply state")
	}
	if strings.Contains(html, `SYNC_PATH + "?antigravity_model_group="`) {
		t.Error("dashboard view selector must not be sent as write-back/control authority")
	}
	// Verify no double "} else {" syntax error
	if strings.Contains(html, "} else {\n                summary.textContent") || strings.Contains(html, "} else {\r\n                summary.textContent") {
		t.Errorf("StatusHTML should not contain duplicate else block")
	}
}

func extractCSSTokens(t *testing.T, css string, blockStart string, blockEnd string) map[string]string {
	t.Helper()
	tokens := make(map[string]string)

	idx := strings.Index(css, blockStart)
	if idx == -1 {
		return tokens
	}
	content := css[idx+len(blockStart):]
	endIdx := strings.Index(content, blockEnd)
	if endIdx == -1 {
		return tokens
	}
	blockBody := content[:endIdx]

	reToken := regexp.MustCompile(`(--[a-zA-Z0-9_-]+)\s*:\s*([^;]+);`)
	matches := reToken.FindAllStringSubmatch(blockBody, -1)

	for _, m := range matches {
		tokens[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
	return tokens
}
