package management

import (
	"fmt"
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
		"function triggerApplyWithConfirm()",
		"showModal(\"apply-confirm\"",
		"function executeDirectApply()",
		"RUN_PATH + \"?mode=apply",
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

	applyConfirm := strings.Index(StatusHTML, `showModal("apply-confirm"`)
	directApply := strings.Index(StatusHTML, `RUN_PATH + "?mode=apply`)
	if applyConfirm == -1 || directApply == -1 || applyConfirm >= directApply {
		t.Fatal("assembled apply workflow does not expose confirmation before direct apply")
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

func TestApplyPreviewWorkflow(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; apply preview behavior is checked in the Node-enabled verification environment")
	}

	testCases := []struct {
		name       string
		changes    string
		items      string
		wantModal  bool
		wantMode   string
		wantCount  int
		wantToasts int
	}{
		{
			name:      "projected target change is previewed",
			changes:   `[]`,
			items:     `[{auth_index:"credential-1",name:"Credential 1",current:{priority:99,priority_missing:false,disabled:false},target:{priority:98,priority_missing:false,disabled:false}}]`,
			wantModal: true,
			wantMode:  "projected",
			wantCount: 1,
		},
		{
			name:       "synchronized state reports optimal",
			changes:    `[]`,
			items:      `[{auth_index:"credential-1",name:"Credential 1",current:{priority:99,priority_missing:false,disabled:false},target:{priority:99,priority_missing:false,disabled:false}}]`,
			wantToasts: 1,
		},
		{
			name:      "write qualified changes retain pending preview",
			changes:   `[{auth_index:"credential-1",name:"Credential 1",current:{priority:99,priority_missing:false,disabled:false},target:{priority:98,priority_missing:false,disabled:false}}]`,
			items:     `[{auth_index:"credential-1",name:"Credential 1",current:{priority:99,priority_missing:false,disabled:false},target:{priority:98,priority_missing:false,disabled:false}}]`,
			wantModal: true,
			wantMode:  "pending",
			wantCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			harness := templateScriptOverviewActionsCore + templateScriptModals + `
const toastCalls = [];
function element(initial) {
    return Object.assign({hidden:false,textContent:"",innerHTML:"",children:[],appendChild:function(child){this.children.push(child);}}, initial || {});
}
const elements = {
    modelGroupSelect: element({value:"gemini"}),
    diffModal: element({hidden:true}),
    modalTitle: element(),
    modalSummary: element(),
    modalDiffList: element(),
    btnModalApply: element()
};
const document = {
    getElementById: function(id) {
        return elements[id] || null;
    },
    createElement: function() { return element(); }
};
function showToast(message, kind) { toastCalls.push({message, kind}); }
function t(key) { return key; }
function escapeHTML(value) { return String(value); }
let currentLang = "zh-CN";
dynamicConfig = { antigravity_model_group: "gemini" };
latestSnapshot = { active_model_group: "gemini", groups: { gemini: { changes: ` + tc.changes + `, items: ` + tc.items + ` } } };

(async function() {
    await triggerApplyWithConfirm();
    const expected = ` + applyPreviewExpectationJSON(tc.wantModal, tc.wantMode, tc.wantCount, tc.wantToasts) + `;
    if (elements.diffModal.hidden === expected.modal) {
        throw new Error("unexpected modal visibility: " + elements.diffModal.hidden);
    }
    if (toastCalls.length !== expected.toasts) {
        throw new Error("unexpected toast count: " + toastCalls.length);
    }
    if (expected.modal) {
        const expectedSummaryKey = expected.mode === "projected" ? "projectedApplyPreview" : "pendingApplyPreview";
        if (elements.modalSummary.textContent.indexOf(expectedSummaryKey) !== 0) {
            throw new Error("unexpected preview summary: " + elements.modalSummary.textContent);
        }
        if (elements.modalDiffList.children.length !== expected.count) {
            throw new Error("unexpected rendered preview count: " + elements.modalDiffList.children.length);
        }
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
			runNodeFixture(t, node, "apply-preview.js", harness)
		})
	}
}

func TestSamplesModalUsesSelectedModelGroupOnly(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; samples modal behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptOverviewSamples + `
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
    return Promise.resolve({records:[{email:"claude@example.com",sample:{observed_at:"2026-08-23T02:01:00Z",short_window_rem:75,long_window_rem:0}}]});
}
function t(key) { return key; }
(async function() {
    showHistoryDetails(0);
    await new Promise(function(resolve) { setTimeout(resolve, 0); });
    if (calls.length !== 2) throw new Error("probe history did not request both model groups");
    if (!calls.some(function(path) { return path.indexOf("model_group=gemini") >= 0; })) throw new Error("Gemini probe samples were not requested");
    if (!calls.some(function(path) { return path.indexOf("model_group=claude_gpt") >= 0; })) throw new Error("Claude/GPT probe samples were not requested");
    if (elements.modalDiffList.innerHTML.indexOf("history-comparison-table") < 0) throw new Error("probe history comparison table is missing");
    if (elements.modalDiffList.innerHTML.indexOf("history-comparison-quota-only") < 0) throw new Error("probe history must use the quota-only table layout");
    if (elements.modalDiffList.innerHTML.indexOf("本次探测 Gemini 额度") < 0 || elements.modalDiffList.innerHTML.indexOf("本次探测 Claude/GPT 额度") < 0) throw new Error("probe history model-group columns are missing");
    if (elements.modalDiffList.innerHTML.indexOf("优先级变化") >= 0) throw new Error("probe history must not render a priority column");
    if (elements.modalDiffList.innerHTML.indexOf("gemini@example.com") < 0 || elements.modalDiffList.innerHTML.indexOf("claude@example.com") < 0) throw new Error("probe history accounts are missing");
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

func applyPreviewExpectationJSON(wantModal bool, wantMode string, wantCount, wantToasts int) string {
	return fmt.Sprintf(`{modal:%t,mode:%q,count:%d,toasts:%d}`, wantModal, wantMode, wantCount, wantToasts)
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
