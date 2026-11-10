package management

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestManagementPageAssetContract(t *testing.T) {
	expectedNames := []string{"Overview", "History", "Diagnostics", "Config", "Help"}
	if len(managementPageFeatures) != len(expectedNames) {
		t.Fatalf("expected %d feature assets, got %d", len(expectedNames), len(managementPageFeatures))
	}

	for index, feature := range managementPageFeatures {
		if feature.name != expectedNames[index] {
			t.Errorf("feature %d is %q, want %q", index, feature.name, expectedNames[index])
		}
		if strings.TrimSpace(feature.markup) == "" {
			t.Errorf("%s feature has no markup asset", feature.name)
		}
		if strings.TrimSpace(feature.styles) == "" {
			t.Errorf("%s feature has no style asset", feature.name)
		}
		if len(feature.translationKeys) == 0 {
			t.Errorf("%s feature has no translation ownership declaration", feature.name)
		}
	}

	if got := strings.Count(StatusHTML, "<section id=\"panel"); got != len(expectedNames) {
		t.Fatalf("assembled page contains %d feature panels, want %d", got, len(expectedNames))
	}
	if StatusHTML != assembleManagementPage() {
		t.Fatal("StatusHTML is not the deterministic output of the page assembly seam")
	}
}

func TestOverviewUsesCPAEmailAsCredentialTitle(t *testing.T) {
	if !strings.Contains(StatusHTML, `const credDisplayName = item.email || "Credential";`) {
		t.Fatal("overview credential title must use the CPA Host email")
	}
	if strings.Contains(StatusHTML, `item.name || item.account || item.auth_index`) {
		t.Fatal("overview credential title still uses a non-email fallback chain")
	}
}

func TestOverview_SingleProbeButtonAndStatusDisabledBadge(t *testing.T) {
	if !strings.Contains(StatusHTML, `probeSingleCredential`) {
		t.Fatal("overview must include probeSingleCredential script function")
	}
	if !strings.Contains(StatusHTML, `onclick=\"probeSingleCredential('`) {
		t.Fatal("overview card must render single-credential probe button")
	}
	if !strings.Contains(StatusHTML, `t("statusDisabled")`) {
		t.Fatal("overview card must render statusDisabled badge for manually disabled accounts")
	}
	if !strings.Contains(StatusHTML, `item.target && item.target.disabled && !isManuallyDisabled`) {
		t.Fatal("overview depleted count must exclude manually disabled accounts")
	}
}

func TestStatusHTML_FeatureTranslationsExistInBothLanguages(t *testing.T) {
	zhKeys := languageKeys(t, "zh-CN")
	enKeys := languageKeys(t, "en-US")

	for _, feature := range managementPageFeatures {
		for _, key := range feature.translationKeys {
			if _, ok := zhKeys[key]; !ok {
				t.Errorf("%s feature translation key %q is missing from zh-CN", feature.name, key)
			}
			if _, ok := enKeys[key]; !ok {
				t.Errorf("%s feature translation key %q is missing from en-US", feature.name, key)
			}
		}
	}

	dataKeys := regexp.MustCompile(`data-i18n="([A-Za-z][A-Za-z0-9_]*)"`).FindAllStringSubmatch(StatusHTML, -1)
	for _, match := range dataKeys {
		key := match[1]
		if _, ok := zhKeys[key]; !ok {
			t.Errorf("assembled markup references missing zh-CN key %q", key)
		}
		if _, ok := enKeys[key]; !ok {
			t.Errorf("assembled markup references missing en-US key %q", key)
		}
	}

	callKeys := regexp.MustCompile(`\bt\("([A-Za-z][A-Za-z0-9_]*)"\)`).FindAllStringSubmatch(extractPageScript(t), -1)
	for _, match := range callKeys {
		key := match[1]
		if _, ok := zhKeys[key]; !ok {
			t.Errorf("assembled script references missing zh-CN key %q", key)
		}
		if _, ok := enKeys[key]; !ok {
			t.Errorf("assembled script references missing en-US key %q", key)
		}
	}
}

func TestStatusHTML_InlineHandlersResolve(t *testing.T) {
	script := extractPageScript(t)
	functionNames := map[string]struct{}{}
	functionRE := regexp.MustCompile(`\bfunction\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	for _, match := range functionRE.FindAllStringSubmatch(script, -1) {
		functionNames[match[1]] = struct{}{}
	}

	callRE := regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	handlerRE := regexp.MustCompile(`(?i)\bon[a-z]+\s*=\s*"([^"]+)"`)
	for _, match := range handlerRE.FindAllStringSubmatch(StatusHTML, -1) {
		for _, call := range callRE.FindAllStringSubmatch(match[1], -1) {
			if _, ok := functionNames[call[1]]; !ok {
				t.Errorf("inline handler %q calls unavailable function %q", match[1], call[1])
			}
		}
	}
}

func TestStatusHTML_StaticDOMReferencesResolve(t *testing.T) {
	idNames := map[string]struct{}{}
	idRE := regexp.MustCompile(`\bid="([A-Za-z][A-Za-z0-9_-]*)"`)
	for _, match := range idRE.FindAllStringSubmatch(StatusHTML, -1) {
		idNames[match[1]] = struct{}{}
	}

	script := extractPageScript(t)
	getElementRE := regexp.MustCompile(`document\.getElementById\(\s*["']([A-Za-z][A-Za-z0-9_-]*)["']\s*\)`)
	selectorRE := regexp.MustCompile(`document\.querySelector(?:All)?\(\s*["']#([A-Za-z][A-Za-z0-9_-]*)["']\s*\)`)
	for _, expression := range []*regexp.Regexp{getElementRE, selectorRE} {
		for _, match := range expression.FindAllStringSubmatch(script, -1) {
			if _, ok := idNames[match[1]]; !ok {
				t.Errorf("script references missing assembled DOM id %q", match[1])
			}
		}
	}
}

func TestStatusHTML_OperationalWorkflowContract(t *testing.T) {
	workflowSnippets := []string{
		"function triggerProbe()",
		"RUN_PATH + \"?mode=probe",
		"function triggerReset()",
		"RESET_PATH",
		"function fetchDynamicConfig()",
		"function saveDynamicConfig()",
		"CONFIG_PATH",
		"function resetDynamicConfigToDefaults()",
		"confirmResetConfigTitle",
	}
	for _, snippet := range workflowSnippets {
		if !strings.Contains(StatusHTML, snippet) {
			t.Errorf("assembled page is missing workflow contract %q", snippet)
		}
	}
}

func TestStatusHTML_JavaScriptSyntax(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; JavaScript syntax is checked in the Node-enabled verification environment")
	}

	scriptPath := filepath.Join(t.TempDir(), "management-page.js")
	if err := os.WriteFile(scriptPath, []byte(extractPageScript(t)), 0o600); err != nil {
		t.Fatalf("write JavaScript fixture: %v", err)
	}
	cmd := exec.Command(node, "--check", scriptPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("assembled management-page script failed node --check: %v\n%s", err, output)
	}
}

func TestSamplesModalUsesSelectedModelGroupOnly(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; samples modal behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptOverviewActionsCore + templateScriptOverviewSamples + `
function element(initial) {
    return Object.assign({hidden:true,textContent:"",innerHTML:""}, initial || {});
}
const elements = {
    samplesModal: element(),
    samplesModalTitle: element(),
    samplesModalSubtitle: element(),
    samplesModalBody: element(),
    modelGroupSelect: element({value:"claude_gpt"})
};
const document = {
    getElementById: function(id) { return elements[id] || null; }
};
const SAMPLES_PATH = "/samples";
const currentLang = "zh-CN";
function t(key) { return key; }
function escapeHTML(value) { return String(value); }
async function apiFetch() {
    return {
        groups: {
            gemini: {name:"Gemini 模型", samples:[{observed_at:"2026-08-23T02:00:00Z",short_window_rem:100,long_window_rem:100}]},
            claude_gpt: {name:"Claude & GPT 模型", samples:[
                {observed_at:"2026-08-23T02:00:00Z",short_window_rem:0,long_window_rem:45},
                {observed_at:"2026-08-23T02:15:00Z",short_window_rem:100,long_window_rem:45}
            ]}
        }
    };
}
(async function() {
    await openSamplesModal("auth-test", "Test Account");
    const html = elements.samplesModalBody.innerHTML;
    if (html.indexOf("Gemini") >= 0) throw new Error("selected Claude/GPT view rendered Gemini data");
    if ((html.match(/class="sample-group"/g) || []).length !== 2) throw new Error("selected group sample count mismatch");
    if (html.indexOf("0%") < 0) throw new Error("zero quota was not rendered as 0%");
    if (html.indexOf("100%") > html.indexOf("0%")) throw new Error("samples are not sorted newest first");
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
	runNodeFixture(t, node, "samples-modal.js", harness)
}

func TestProbeHistoryDetailsRenderBothModelGroups(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; probe history behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptHistoryDetails + templateScriptModals + `
const calls = [];
function element(initial) {
    return Object.assign({hidden:true,textContent:"",innerHTML:"",children:[],appendChild:function(child){this.children.push(child);}}, initial || {});
}
const elements = {
    diffModal: element(),
    modalTitle: element(),
    modalSummary: element(),
    modalDiffList: element(),
    btnModalApply: element()
};
const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function() { return element(); }
};
const SAMPLES_PATH = "/samples";
const currentLang = "zh-CN";
const latestDiagnostics = {run_history:[{kind:"probe",probe_round_id:"round-1"}]};
function showToast(message, kind) { throw new Error(message + " (" + kind + ")"); }
function escapeHTML(value) { return String(value); }
function apiFetch(path) {
    calls.push(path);
    if (path.indexOf("model_group=gemini") >= 0) {
        return Promise.resolve({records:[{email:"gemini@example.com",sample:{observed_at:"2026-08-23T02:00:00Z",short_window_rem:0,long_window_rem:25}}]});
    }
    if (path.indexOf("model_group=argon") >= 0) {
        return Promise.resolve({records:[{email:"argon@example.com",sample:{observed_at:"2026-08-23T02:00:30Z",long_window_rem:80}}]});
    }
    return Promise.resolve({records:[{email:"claude@example.com",sample:{observed_at:"2026-08-23T02:01:00Z",short_window_rem:75,long_window_rem:0}}]});
}
function t(key) { return key; }
(async function() {
    showHistoryDetails(0);
    await new Promise(function(resolve) { setTimeout(resolve, 0); });
    if (calls.length !== 3) throw new Error("probe history did not request all three model groups, got " + calls.length);
    if (!calls.some(function(path) { return path.indexOf("model_group=gemini") >= 0; })) throw new Error("Gemini probe samples were not requested");
    if (!calls.some(function(path) { return path.indexOf("model_group=argon") >= 0; })) throw new Error("Argon probe samples were not requested");
    if (!calls.some(function(path) { return path.indexOf("model_group=claude_gpt") >= 0; })) throw new Error("Claude/GPT probe samples were not requested");
    if (elements.modalDiffList.innerHTML.indexOf("history-comparison-table") < 0) throw new Error("probe history comparison table is missing");
    if (elements.modalDiffList.innerHTML.indexOf("history-comparison-quota-only") < 0) throw new Error("probe history must use the quota-only table layout");
    if (elements.modalDiffList.innerHTML.indexOf("本次探测 Gemini 额度") < 0 || elements.modalDiffList.innerHTML.indexOf("本次探测 Argon 额度") < 0 || elements.modalDiffList.innerHTML.indexOf("本次探测 Claude/GPT 额度") < 0) throw new Error("probe history model-group columns are missing");
    if (elements.modalDiffList.innerHTML.indexOf("优先级变化") >= 0) throw new Error("probe history must not render a priority column");
    if (elements.modalDiffList.innerHTML.indexOf("gemini@example.com") < 0 || elements.modalDiffList.innerHTML.indexOf("argon@example.com") < 0 || elements.modalDiffList.innerHTML.indexOf("claude@example.com") < 0) throw new Error("probe history accounts are missing");
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
	runNodeFixture(t, node, "probe-history.js", harness)
}

func TestAutoScheduleHistoryCombinesProbeAndHostDetails(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; automatic schedule history is checked in the Node-enabled verification environment")
	}

	harness := templateScriptHistoryDetails + templateScriptModals + `
function element(initial) {
    return Object.assign({hidden:true,textContent:"",innerHTML:"",children:[],appendChild:function(child){this.children.push(child);}}, initial || {});
}
const elements = {
    diffModal: element(),
    modalTitle: element(),
    modalSummary: element(),
    modalDiffList: element(),
    btnModalApply: element()
};
const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function() { return element(); }
};
const SAMPLES_PATH = "/samples";
const currentLang = "zh-CN";
const latestDiagnostics = {run_history:[{
    kind:"auto_apply",
    trigger:"auto_apply",
    probe_round_id:"round-auto",
    snapshot:{changes:[{email:"account@example.com",current:{priority:98,disabled:false},target:{priority:99,disabled:false},reason:"weekly urgency"}]},
    attempted:1,
    succeeded:1
}]};
function showToast(message, kind) { throw new Error(message + " (" + kind + ")"); }
function escapeHTML(value) { return String(value); }
function apiFetch(path) {
    return Promise.resolve({records:[{email:"account@example.com",sample:{observed_at:"2026-08-24T02:00:00Z",short_window_rem:90,long_window_rem:55}}]});
}
function t(key) { return key; }
function extractChanges(result) { return result && Array.isArray(result.changes) ? result.changes : []; }
function formatReason(reason) { return reason || ""; }
(async function() {
    showHistoryDetails(0);
    await new Promise(function(resolve) { setTimeout(resolve, 0); });
    if (elements.modalTitle.textContent !== "自动调度明细") throw new Error("automatic schedule title is missing");
    if (elements.modalDiffList.innerHTML.indexOf("history-comparison-table") < 0) throw new Error("automatic schedule comparison table is missing");
    if (elements.modalDiffList.innerHTML.indexOf("优先级变化") < 0) throw new Error("automatic schedule priority column is missing");
    if (elements.modalDiffList.innerHTML.indexOf("account@example.com") < 0) throw new Error("automatic schedule account is missing");
    if ((elements.modalDiffList.innerHTML.match(/history-comparison-row/g) || []).length !== 2) throw new Error("Gemini, Claude/GPT, and priority data were not merged into one account row");
    if (elements.modalDiffList.innerHTML.indexOf("weekly urgency") < 0) throw new Error("automatic schedule priority details are missing");
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
	runNodeFixture(t, node, "auto-schedule-history.js", harness)
}

func TestStatusHTML_AuthCircuitBreaker(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; auth circuit-breaker behavior is checked in the Node-enabled verification environment")
	}

	harness := `
function element(initial) {
    return Object.assign({hidden:true,textContent:"",innerHTML:"",value:"",disabled:false,classList:{add:function(){},remove:function(){},toggle:function(){}}}, initial || {});
}
const elements = {
    keyModal: element(),
    manualKeyInput: element(),
    panelOverview: element(),
    panelHistory: element(),
    panelDiagnostics: element(),
    panelConfig: element(),
    panelHelp: element(),
    toastRoot: element()
};
const document = {
    getElementById: function(id) { return elements[id] || null; },
    querySelectorAll: function() { return []; },
    addEventListener: function() {},
    createElement: function() { return element(); },
    documentElement: { getAttribute: function() { return "dark"; } }
};
const window = {
    matchMedia: function() { return { matches: true }; },
    location: { search: "" }
};
const localStorage = {
    getItem: function() { return null; },
    setItem: function() {},
    removeItem: function() {}
};
const sessionStorage = {
    getItem: function() { return null; },
    setItem: function() {},
    removeItem: function() {}
};

let fetchCount = 0;
global.fetch = async function(path) {
    fetchCount++;
    return {
        status: 401,
        ok: false,
        statusText: "Unauthorized",
        text: async function() { return "missing management key"; }
    };
};
` + extractPageScript(t) + `
(async function() {
    // 1. Initial boot: top-level initializeApp() gets 401 -> sets isAuthBlocked -> stops further initialization
    await new Promise(function(resolve) { setTimeout(resolve, 50); });
    if (fetchCount !== 1) {
        throw new Error("expected exactly 1 fetch before auth circuit-breaker tripped, got " + fetchCount);
    }
    if (!isAuthBlocked) {
        throw new Error("expected isAuthBlocked to be true after 401");
    }
    if (elements.keyModal.hidden !== false) {
        throw new Error("expected keyModal to be visible after 401");
    }

    // 2. Further calls while auth is blocked must NOT issue network requests
    try {
        await apiFetch("/test");
    } catch (_) {}
    if (fetchCount !== 1) {
        throw new Error("expected apiFetch to block without network call when isAuthBlocked=true");
    }

    // 3. Tab switches while blocked must NOT issue network requests
    switchTab("diagnostics");
    if (fetchCount !== 1) {
        throw new Error("expected switchTab to not fetch when isAuthBlocked=true");
    }

    // 4. Entering valid key and saving resets auth block and allows successful fetch
    global.fetch = async function(path) {
        fetchCount++;
        return {
            status: 200,
            ok: true,
            statusText: "OK",
            text: async function() { return JSON.stringify({ ok: true, active_model_group: "gemini" }); }
        };
    };
    elements.manualKeyInput.value = "secret_key";
    saveKeyAndRefresh();
    await new Promise(function(resolve) { setTimeout(resolve, 50); });
    if (isAuthBlocked) {
        throw new Error("expected isAuthBlocked to be false after saveKeyAndRefresh with valid key");
    }
    if (fetchCount <= 1) {
        throw new Error("expected initializeApp to retry requests after key save");
    }
    process.exit(0);
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
	runNodeFixture(t, node, "auth-circuit-breaker.js", harness)
}
func runNodeFixture(t *testing.T, node, filename, script string) {
	t.Helper()
	scriptPath := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		t.Fatalf("write JavaScript fixture: %v", err)
	}
	cmd := exec.Command(node, scriptPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("JavaScript fixture failed: %v\n%s", err, output)
	}
}

func extractPageScript(t *testing.T) string {
	t.Helper()
	start := strings.Index(StatusHTML, "<script>")
	end := strings.Index(StatusHTML, "</script>")
	if start == -1 || end == -1 || end <= start+len("<script>") {
		t.Fatal("assembled page does not contain one complete inline script")
	}
	return StatusHTML[start+len("<script>") : end]
}

func languageKeys(t *testing.T, language string) map[string]struct{} {
	t.Helper()
	script := extractPageScript(t)
	languageBlockRE := regexp.MustCompile(`(?ms)^\s*"` + regexp.QuoteMeta(language) + `": \{\s*(.*?)^\s*\}`)
	matches := languageBlockRE.FindAllStringSubmatch(script, -1)
	if len(matches) == 0 {
		t.Fatalf("assembled script does not contain %s translations", language)
	}
	keys := map[string]struct{}{}
	keyRE := regexp.MustCompile(`(?m)^\s*([A-Za-z][A-Za-z0-9_]*)\s*:`)
	for _, languageMatch := range matches {
		for _, keyMatch := range keyRE.FindAllStringSubmatch(languageMatch[1], -1) {
			keys[keyMatch[1]] = struct{}{}
		}
	}
	return keys
}

func TestStatusHTML_CPAThemeAlignmentAndDevServerHelper(t *testing.T) {
	if strings.Contains(StatusHTML, "id=\"btnThemeToggle\"") {
		t.Fatal("assembled markup must not expose standalone theme toggle button")
	}
	if strings.Contains(StatusHTML, "toggleTheme()") {
		t.Fatal("assembled markup must not expose toggleTheme inline handler")
	}

	script := extractPageScript(t)
	if !strings.Contains(script, "window.__setTheme = window.setTheme = function") {
		t.Fatal("assembled script must provide window.__setTheme / window.setTheme helper for console debugging")
	}
	if !strings.Contains(script, `cleanInlineThemeStyles()`) {
		t.Fatal("assembled script must clean inline styles when synchronizing theme")
	}
	if !strings.Contains(script, `const resolvedTheme = isDark ? "dark" : "light";`) {
		t.Fatal("syncThemeFromParent must explicitly resolve to dark or light")
	}
}
func TestProbeOutcomeToastAndRenderFailureTruth(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; probe outcome behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptProbeSchedule + `
const toastCalls = [];
function element(initial) {
    return Object.assign({
        hidden: false,
        textContent: "",
        innerHTML: "",
        children: [],
        attributes: {},
        style: {},
        classList: {
            classes: [],
            add: function(c) { this.classes.push(c); },
            remove: function(c) { this.classes = this.classes.filter(x => x !== c); },
            contains: function(c) { return this.classes.includes(c); }
        },
        appendChild: function(child) { this.children.push(child); if (child && child.innerHTML) this.innerHTML += child.innerHTML; },
        setAttribute: function(k, v) { this.attributes[k] = v; },
        getAttribute: function(k) { return this.attributes[k] || null; }
    }, initial || {});
}

const elements = {
    toastRoot: element(),
    modelGroupSelect: element({value: "gemini"}),
    valLastAudit: element(),
    valNextProbe: element(),
    valTotalCreds: element(),
    valTotalDesc: element(),
    valBoosted: element(),
    valDepleted: element(),
    credentialsContainer: element(),
    credentialsLoadingOverlay: element({hidden: true}),
    btnProbe: element()
};

const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function(tag) { return element({tagName: tag}); },
    querySelectorAll: function() { return []; }
};

function showToast(msg, type) {
    toastCalls.push({msg: msg, type: type});
}

const ZH = {
    statusActive: "正常活跃",
    statusFailed: "探测失败",
    statusDisabled: "已禁用",
    statusWeeklyDepleted: "周额度耗尽",
    statusCooldown: "⏳ 429 冷却中",
    statusBoosted: "🚀 动态 Boost",
    probeSingleSuccess: "已更新该凭证配额",
    probeSingleFailed: "探测失败",
    probeSingleNoResult: "未返回该凭证的探测结果",
    probeBatchFailed: "配额探测完成，存在失败凭证",
    shortWindow: "5h 短窗口",
    longWindow: "7d 周窗口",
    urgencyLabel: "紧迫度",
    burnLabel: "燃烧率",
    actualPriority: "实际",
    targetPriority: "目标",
    predictedPriority: "预测",
    unsetPriority: "[未设置]"
};

function t(key) { return ZH[key] || key; }
function escapeHTML(s) { return String(s || ""); }
function formatCountdown(d) { return "01h 00m"; }

currentLang = "zh-CN";
dynamicConfig = { auto_apply: false, antigravity_model_group: "gemini" };
latestDiagnostics = { latest_audit: "test audit" };
scheduleConfig = null;
isAuthBlocked = false;
userSelectedModelGroup = false;
const RUN_PATH = "/run";
const SNAPSHOT_PATH = "/snapshot/latest";
const DIAGNOSTICS_PATH = "/diagnostics";

async function apiFetch(path, options) {
    if (path.indexOf("mode=probe") >= 0) {
        return {
            attempted: 0,
            succeeded: 0,
            probe_outcomes: [
                {
                    auth_index: "fail-403",
                    model_group: "gemini",
                    status: "failed",
                    error: "retrieve quota summary status 403",
                    http_status_code: 403
                }
            ]
        };
    }
    return {};
}

async function fetchSnapshot() {}
async function fetchDiagnostics() {}

(async function() {
    // 1. Test single probe failure toast
    const probeBtn = element({textContent: "⚡ 探测"});
    await probeSingleCredential("fail-403", probeBtn);
    if (toastCalls.length !== 1) {
        throw new Error("expected exactly 1 toast, got " + toastCalls.length);
    }
    if (toastCalls[0].type !== "error") {
        throw new Error("expected error toast for failed probe, got: " + toastCalls[0].type);
    }
    if (toastCalls[0].msg.indexOf("403") < 0 || toastCalls[0].msg.indexOf("探测失败") < 0) {
        throw new Error("expected toast to mention 探测失败 and 403, got: " + toastCalls[0].msg);
    }

    // 2. Test card render: 403 failed card
    latestSnapshot = {
        active_model_group: "gemini",
        groups: {
            gemini: {
                items: [
                    {
                        auth_index: "fail-403",
                        email: "test-403@example.com",
                        reason: "probe failed: retrieve quota summary status 403; historical: fresh remaining positive",
                        evidence_fresh: false,
                        current: { priority: 99, disabled: false },
                        target: { priority: 99, disabled: false },
                        r5h: 1.0,
                        r7d: 1.0,
                        urgency: 0.98,
                        cycle_burn_rate: 0.16
                    },
                    {
                        auth_index: "cooldown-429",
                        email: "test-429@example.com",
                        reason: "probe failed: retrieve quota summary status 429; historical: 429 rate limit cooldown",
                        evidence_fresh: false,
                        current: { priority: -1, disabled: false },
                        target: { priority: -1, disabled: false },
                        r5h: 0.8,
                        r7d: 0.9,
                        urgency: 1.1,
                        cycle_burn_rate: 0.15
                    },
                    {
                        auth_index: "healthy-cached",
                        email: "cached@example.com",
                        reason: "historical: fresh remaining positive",
                        evidence_fresh: false,
                        short_window_reset_at: "2026-10-16T08:00:00Z",
                        long_window_reset_at: "2026-10-23T08:00:00Z",
                        current: { priority: 98, disabled: false },
                        target: { priority: 98, disabled: false },
                        r5h: 0.95,
                        r7d: 0.99,
                        urgency: 1.02,
                        cycle_burn_rate: 0.15
                    }
                ]
            }
        }
    };

    renderDashboard();

    const renderedCards = elements.credentialsContainer.children;
    if (renderedCards.length !== 3) {
        throw new Error("expected 3 rendered cards, got " + renderedCards.length);
    }

    // Card 0: 403 failure
    const card0HTML = renderedCards[0].innerHTML;
    if (card0HTML.indexOf("cred-reason-danger") < 0 || card0HTML.indexOf("探测失败") < 0) {
        throw new Error("card 0 should have cred-reason-danger with '探测失败': " + card0HTML);
    }
    if (card0HTML.indexOf("余量充足") >= 0) {
        throw new Error("card 0 must NOT show '余量充足' when probe failed");
    }
    if (card0HTML.indexOf("探测失败: retrieve quota summary status 403") < 0) {
        throw new Error("card 0 should show specific probe failure detail: " + card0HTML);
    }
    if (card0HTML.indexOf("meter-container-stale") >= 0 || card0HTML.indexOf("历史数据") >= 0) {
        throw new Error("card 0 must NOT show the removed stale '历史数据' tag: " + card0HTML);
    }

    // Card 1: health cooldown badge must win over the probe-failed state
    const card1HTML = renderedCards[1].innerHTML;
    if (card1HTML.indexOf("badge-warning") < 0) {
        throw new Error("card 1 should prioritize the cooldown badge over probe failed: " + card1HTML);
    }
    if (card1HTML.indexOf("探测失败: retrieve quota summary status 429") < 0) {
        throw new Error("card 1 reason line should display failure cause: " + card1HTML);
    }

    // Card 2: cached credential with valid latest probe observation (when another credential was probed)
    const card2HTML = renderedCards[2].innerHTML;
    if (card2HTML.indexOf("meter-container-stale") >= 0 || card2HTML.indexOf("历史数据") >= 0) {
        throw new Error("card 2 with valid latest probe observation must NOT show stale '历史数据' tag: " + card2HTML);
    }
    // 3. Test triggerProbe with batch failure
    elements.btnProbe.querySelector = function() { return element(); };
    elements.btnProbe.classList = { contains: function() { return false; }, add: function() {}, remove: function() {} };
    await triggerProbe();
    clearInterval(probeCooldownTimer);
    const lastToast = toastCalls[toastCalls.length - 1];
    if (lastToast.type !== "error" || lastToast.msg.indexOf("配额探测完成，存在失败凭证 (1)") < 0) {
        throw new Error("expected batch probe failure toast, got: " + JSON.stringify(lastToast));
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`

	runNodeFixture(t, node, "probe-outcome-truth.js", harness)
}

func TestTriggerProbe_ListLoadingState(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; probe outcome behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptProbeSchedule + `
function element(initial) {
    return Object.assign({
        hidden: true,
        textContent: "",
        innerHTML: "",
        children: [],
        classList: {
            classes: [],
            add: function(c) { this.classes.push(c); },
            remove: function(c) { this.classes = this.classes.filter(x => x !== c); },
            contains: function(c) { return this.classes.includes(c); }
        },
        querySelector: function() { return element({textContent: "刷新配额"}); }
    }, initial || {});
}

const elements = {
    credentialsContainer: element({hidden: false}),
    credentialsLoadingOverlay: element({hidden: true}),
    btnProbe: element({hidden: false}),
    modelGroupSelect: element({value: "gemini"})
};

const document = {
    getElementById: function(id) { return elements[id] || null; }
};

const RUN_PATH = "/run";

function showToast() {}
function t(key) { return key; }
function refreshDashboard() { return Promise.resolve(); }

var loadingDuringProbe = null;
var gridClassDuringProbe = null;

apiFetch = async function(path, opts) {
    loadingDuringProbe = elements.credentialsLoadingOverlay.hidden;
    gridClassDuringProbe = elements.credentialsContainer.classList.contains("credentials-probing");
    return { probe_outcomes: [{ auth_index: "acc-1", status: "success" }] };
};

(async function() {
    await triggerProbe();
    clearInterval(probeCooldownTimer);

    if (loadingDuringProbe !== false) {
        throw new Error("expected credentialsLoadingOverlay.hidden to be false during probe, got: " + loadingDuringProbe);
    }
    if (gridClassDuringProbe !== true) {
        throw new Error("expected credentialsContainer to have credentials-probing class during probe");
    }
    if (elements.credentialsLoadingOverlay.hidden !== true) {
        throw new Error("expected credentialsLoadingOverlay.hidden to be restored to true after probe, got: " + elements.credentialsLoadingOverlay.hidden);
    }
    if (elements.credentialsContainer.classList.contains("credentials-probing")) {
        throw new Error("expected credentialsContainer to have removed credentials-probing class after probe");
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`

	runNodeFixture(t, node, "probe-loading-state.js", harness)
}

func TestRefreshDashboard_ManualSyncLoadingState(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; refresh dashboard behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptProbeSchedule + `
function element(initial) {
    return Object.assign({
        hidden: true,
        textContent: "",
        innerHTML: "",
        children: [],
        classList: {
            classes: [],
            add: function(c) { this.classes.push(c); },
            remove: function(c) { this.classes = this.classes.filter(x => x !== c); },
            contains: function(c) { return this.classes.includes(c); }
        },
        querySelector: function() { return element(); }
    }, initial || {});
}

const elements = {
    credentialsContainer: element({hidden: false}),
    credentialsLoadingOverlay: element({hidden: true}),
    btnRefresh: element({hidden: false})
};

const document = {
    getElementById: function(id) { return elements[id] || null; }
};

const SYNC_PATH = "/sync";
const DIAGNOSTICS_PATH = "/diagnostics";
var isAuthBlocked = false;

function showToast() {}
function t(key) { return key; }
function renderDashboard() {}
function renderHistory() {}
function renderDiagnostics() {}
function renderScheduleStatus() {}
function updateModelDropdownFromSnapshot() {}

var loadingDuringSync = null;
var gridClassDuringSync = null;
var btnRefreshingDuringSync = null;

apiFetch = async function(path, opts) {
    loadingDuringSync = elements.credentialsLoadingOverlay.hidden;
    gridClassDuringSync = elements.credentialsContainer.classList.contains("credentials-probing");
    btnRefreshingDuringSync = elements.btnRefresh.classList.contains("btn-refreshing");
    return { models: [] };
};

(async function() {
    latestSnapshot = { models: [] };
    await refreshDashboard({ withSync: true, notifySync: true });

    if (loadingDuringSync !== false) {
        throw new Error("expected credentialsLoadingOverlay.hidden to be false during manual sync, got: " + loadingDuringSync);
    }
    if (gridClassDuringSync !== true) {
        throw new Error("expected credentialsContainer to have credentials-probing class during manual sync");
    }
    if (btnRefreshingDuringSync !== true) {
        throw new Error("expected btnRefresh to have btn-refreshing class during manual sync");
    }
    if (elements.credentialsLoadingOverlay.hidden !== true) {
        throw new Error("expected credentialsLoadingOverlay.hidden to be restored to true after sync, got: " + elements.credentialsLoadingOverlay.hidden);
    }
    if (elements.credentialsContainer.classList.contains("credentials-probing")) {
        throw new Error("expected credentialsContainer to have removed credentials-probing class after sync");
    }
    if (elements.btnRefresh.classList.contains("btn-refreshing")) {
        throw new Error("expected btnRefresh to have removed btn-refreshing class after sync");
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`

	runNodeFixture(t, node, "refresh-loading-state.js", harness)
}

func TestRenderScheduleStatus_DiagnosticsFallbackAndSync(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; skipping UI JS test")
	}

	harness := templateScriptOverviewActionsCore + templateScriptProbeSchedule + `
function element(initial) {
    return Object.assign({
        hidden: true,
        textContent: "",
        innerHTML: "",
        className: "",
        title: ""
    }, initial || {});
}

const elements = {
    scheduleStatusBadge: element({hidden: false, className: "schedule-status active"}),
    scheduleStatusText: element({textContent: "init"})
};

const document = {
    getElementById: function(id) { return elements[id] || null; }
};

var dynamicConfig = { auto_apply: true };
var scheduleConfig = null;
var latestDiagnostics = {
    scheduler: {
        paused: true,
        window_enabled: false,
        window_start: "00:00",
        window_end: "23:59"
    }
};
var isAuthBlocked = false;
var currentLang = "zh-CN";
const DIAGNOSTICS_PATH = "/diagnostics";

function t(key) { return key; }
function renderDashboard() {}
function renderHistory() {}
function renderDiagnostics() {}
function showToast() {}

apiFetch = async function(path) {
    if (path === DIAGNOSTICS_PATH) {
        return {
            scheduler: {
                paused: true,
                window_enabled: true,
                window_start: "09:00",
                window_end: "21:00"
            }
        };
    }
    return {};
};

(async function() {
    // 1. renderScheduleStatus should fallback to latestDiagnostics.scheduler if scheduleConfig is null
    renderScheduleStatus();
    if (!elements.scheduleStatusBadge.className.includes("paused")) {
        throw new Error("expected badge to be paused via diagnostics fallback, got: " + elements.scheduleStatusBadge.className);
    }
    if (!elements.scheduleStatusBadge.innerHTML.includes("schedulePaused")) {
        throw new Error("expected badge HTML to contain schedulePaused, got: " + elements.scheduleStatusBadge.innerHTML);
    }

    // 2. fetchDiagnostics should synchronize scheduleConfig
    await fetchDiagnostics();
    if (!scheduleConfig) {
        throw new Error("expected scheduleConfig to be initialized by fetchDiagnostics");
    }
    if (scheduleConfig.paused !== true) {
        throw new Error("expected scheduleConfig.paused to be true, got: " + scheduleConfig.paused);
    }
    if (scheduleConfig.window_start !== "09:00" || scheduleConfig.window_end !== "21:00") {
        throw new Error("expected scheduleConfig window to be synced to 09:00-21:00, got: " + scheduleConfig.window_start + "-" + scheduleConfig.window_end);
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`

	runNodeFixture(t, node, "render-schedule-diagnostics-fallback.js", harness)
}
