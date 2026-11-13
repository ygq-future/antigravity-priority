package management

import (
	"os/exec"
	"strings"
	"testing"
)

func TestModelSelector_MarkupAndBuiltInCatalog(t *testing.T) {
	// 1. Verify custom-select wrapper and trigger exist
	if !strings.Contains(StatusHTML, `id="customModelSelect"`) {
		t.Fatal("StatusHTML must contain customModelSelect wrapper")
	}
	if !strings.Contains(StatusHTML, `id="customModelSelectTrigger"`) {
		t.Fatal("StatusHTML must contain customModelSelectTrigger button")
	}
	if !strings.Contains(StatusHTML, `id="customModelSelectBadge"`) {
		t.Fatal("StatusHTML must contain customModelSelectBadge")
	}
	if !strings.Contains(StatusHTML, `id="customModelSelectLabel"`) {
		t.Fatal("StatusHTML must contain customModelSelectLabel")
	}
	if !strings.Contains(StatusHTML, `id="customModelSelectMenu"`) {
		t.Fatal("StatusHTML must contain customModelSelectMenu")
	}

	// 2. Verify all three quota pool group headers are present
	if !strings.Contains(StatusHTML, `data-i18n="poolGemini"`) {
		t.Fatal("customModelSelectMenu must render poolGemini header")
	}
	if !strings.Contains(StatusHTML, `data-i18n="poolArgon"`) {
		t.Fatal("customModelSelectMenu must render poolArgon header")
	}
	if !strings.Contains(StatusHTML, `data-i18n="poolClaudeGPT"`) {
		t.Fatal("customModelSelectMenu must render poolClaudeGPT header")
	}

	// 3. Verify representative built-in models from all three pools are rendered with badges
	expectedModels := []string{
		"gemini-3.8-flash-high",
		"gemini-3-flash",
		"gemini-4-argon-high",
		"claude-sonnet-5-5-high",
		"claude-opus-5-5-high",
		"gpt-oss-120b-medium",
	}
	for _, m := range expectedModels {
		if !strings.Contains(StatusHTML, m) {
			t.Errorf("customModelSelectMenu missing model option %q", m)
		}
	}

	// 4. Verify no raw unstyled native select for model selection
	if strings.Contains(StatusHTML, `<select id="modelSelect">`) {
		t.Fatal("modelSelect must not be a raw unstyled <select>; must use custom styled dropdown primitive")
	}

	// 5. Verify obsolete config items are removed from Config Center UI
	if strings.Contains(configMarkup, `id="cfgModelGroup"`) {
		t.Fatal("cfgModelGroup form row must be removed from configMarkup")
	}
	if strings.Contains(configMarkup, `id="cfgMinChange"`) {
		t.Fatal("cfgMinChange form row must be removed from configMarkup")
	}

	// 6. Verify streamlined control bar buttons
	if !strings.Contains(StatusHTML, `id="btnRefresh"`) {
		t.Fatal("StatusHTML must contain btnRefresh")
	}
	if !strings.Contains(StatusHTML, `id="btnProbe"`) {
		t.Fatal("StatusHTML must contain btnProbe")
	}
	if !strings.Contains(StatusHTML, `id="btnReset"`) {
		t.Fatal("StatusHTML must contain btnReset")
	}
	if !strings.Contains(StatusHTML, `id="scheduleStatusBadge"`) {
		t.Fatal("StatusHTML must contain scheduleStatusBadge")
	}
	if !strings.Contains(StatusHTML, `class="model-exclude-btn"`) {
		t.Fatal("StatusHTML must render model-exclude-btn inside model dropdown")
	}
	if !strings.Contains(StatusHTML, `id="customModelExcludedPill"`) {
		t.Fatal("StatusHTML must include customModelExcludedPill trigger badge")
	}
}

func TestIsModelEligible_RespectsBackendPoolEligibilityAndSupportedModels(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; Argon eligibility behavior is checked in the Node-enabled verification environment")
	}

	harness := `
` + templateScriptOverviewRender + `
function assert(cond, msg) {
    if (!cond) {
        console.error(msg);
        process.exit(1);
    }
}

const eligibleUltra = { plan_type: "ultra", routing_status: "active", pool_eligible: true, supported_models: ["gemini-4-argon-high"], quota_windows: { gemini_7d: { remaining: 8 }, argon_shared_with_gemini: true } };
assert(isModelEligible(eligibleUltra, "gemini-4-argon-high").eligible === true,
    "Ultra eligible through backend shared Gemini pool must stay eligible");

const unavailablePro = { plan_type: "pro", routing_status: "pool_unavailable", pool_eligible: false, quota_windows: { gemini_7d: { remaining: 20 } } };
assert(isModelEligible(unavailablePro, "gemini-4-argon-high").reason === "unsupported_model",
    "backend pool_unavailable / pool_eligible=false must be marked unsupported_model");

const unsupportedModel = { plan_type: "ultra", routing_status: "active", pool_eligible: true, supported_models: ["gemini-3.8-flash-high"] };
assert(isModelEligible(unsupportedModel, "gemini-4-argon-high").reason === "unsupported_model",
    "a credential whose supported_models omits the selected model must stay unsupported");

const disabledHost = { plan_type: "pro", routing_status: "disabled", pool_eligible: false, host: { disabled: true } };
assert(isModelEligible(disabledHost, "gemini-4-argon-high").reason === "disabled",
    "Host-disabled credential must return disabled");
`
	runNodeFixture(t, node, "argon-eligibility.js", harness)
}

func TestRoutingBoard_DynamicCallOrderAndFiveBars_NodeHarness(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; routing board behavior is checked in the Node-enabled verification environment")
	}

	harness := `
const toastCalls = [];
function element(initial) {
    var el = Object.assign({
        hidden: false,
        _innerHTML: "",
        textContent: "",
        children: [],
        attributes: {},
        style: {},
        classList: {
            classes: [],
            add: function(c) { this.classes.push(c); },
            remove: function(c) { this.classes = this.classes.filter(x => x !== c); },
            contains: function(c) { return this.classes.includes(c); },
            toggle: function(c, force) {
                if (force === undefined) force = !this.contains(c);
                if (force) this.add(c); else this.remove(c);
            }
        },
        appendChild: function(child) {
            this.children.push(child);
            if (child && child.innerHTML) this._innerHTML += child.innerHTML;
        },
        setAttribute: function(k, v) { this.attributes[k] = v; },
        getAttribute: function(k) { return this.attributes[k] || null; },
        querySelectorAll: function() { return []; }
    }, initial || {});
    Object.defineProperty(el, "innerHTML", {
        get: function() { return this._innerHTML; },
        set: function(val) {
            this._innerHTML = val;
            if (val === "") { this.children = []; }
        }
    });
    return el;
}

const elements = {
    toastRoot: element(),
    modelSelect: element({value: "gemini-3.8-flash-high"}),
    customModelSelect: element(),
    customModelSelectMenu: element(),
    customModelSelectBadge: element(),
    customModelSelectLabel: element(),
    valLastAudit: element(),
    valNextProbe: element(),
    valTotalCreds: element(),
    valTotalDesc: element(),
    valBoosted: element(),
    valDepleted: element(),
    credentialsContainer: element(),
    btnProbe: element(),
    btnRefresh: element(),
    btnReset: element(),
    samplesModal: element({hidden: true}),
    samplesModalTitle: element(),
    samplesModalSubtitle: element(),
    samplesModalBody: element(),
    btnSamplePoolGemini: element(),
    btnSamplePoolArgon: element(),
    btnSamplePoolClaude: element()
};

const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function(tag) { return element({tagName: tag}); },
    querySelectorAll: function() { return []; },
    addEventListener: function() {}
};
const window = {
    addEventListener: function() {},
    innerHeight: 800
};

function showToast(msg, type) {
    toastCalls.push({msg: msg, type: type});
}
` + templateScriptAccountDisplay + templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptOverviewReset + templateScriptControls + templateScriptProbeSchedule + templateScriptOverviewSamples + `
const ZH = {
    statusActive: "正常活跃",
    statusFailed: "探测失败",
    statusDisabled: "已禁用",
    statusWeeklyDepleted: "周额度耗尽",
    statusCooldown: "⏳ 429 冷却中",
    statusBoosted: "🚀 动态 Boost",
    statusUnsupported: "⛔ 不支持该模型",
    shortWindow: "5h 短窗口",
    longWindow: "7d 周窗口",
    argonWindow: "Argon 专属周窗口",
    urgencyLabel: "紧迫度",
    burnLabel: "燃烧率",
    actualPriority: "实际",
    targetPriority: "目标",
    unsetPriority: "[未设置]",
    badgeGemini: "Gemini",
    badgeArgon: "Argon",
    badgeClaudeGPT: "Claude/GPT"
};

function t(key) { return ZH[key] || key; }
function escapeHTML(s) { return String(s || ""); }
function formatCountdown(d) { return "02h 15m"; }

currentLang = "zh-CN";
dynamicConfig = { auto_apply: true };
latestDiagnostics = { latest_audit: "scheduled probe completed: 4 observations" };
scheduleConfig = null;
isAuthBlocked = false;

// Mock 4 accounts across gemini, argon, and claude_gpt
const sampleSnapshot = {
    models: [
        { id: "gemini-3.8-flash-high", display_name: "Gemini 3.8 Flash High", quota_pool: "gemini" },
        { id: "gemini-4-argon-high", display_name: "Gemini 4 Argon High", quota_pool: "argon" },
        { id: "claude-sonnet-5-5-high", display_name: "Claude Sonnet 5.5 High", quota_pool: "claude_gpt" }
    ],
    groups: {
        gemini: {
            items: [
                {
                    auth_index: "auth-a",
                    email: "pro-a@example.com",
                    plan_type: "pro",
                    is_boosted: true,
                    urgency: 0.85,
                    cycle_burn_rate: 0.15,
                    target: { priority: 999, disabled: false },
                    host: { priority: 999, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 90, reset_at: "2026-11-05T20:00:00Z" },
                        gemini_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 70, reset_at: "2026-11-05T20:00:00Z" },
                        claude_7d: { remaining: 60, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 80, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-b",
                    email: "pro-b@example.com",
                    plan_type: "pro",
                    is_boosted: false,
                    urgency: 0.95,
                    cycle_burn_rate: 0.18,
                    target: { priority: 100, disabled: false },
                    host: { priority: 100, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 50, reset_at: "2026-11-05T21:00:00Z" },
                        gemini_7d: { remaining: 40, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 80, reset_at: "2026-11-05T21:00:00Z" },
                        claude_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 30, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-c",
                    email: "free-c@example.com",
                    plan_type: "free",
                    is_boosted: false,
                    urgency: 1.05,
                    cycle_burn_rate: 0.15,
                    target: { priority: 10, disabled: false },
                    host: { priority: 10, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 40, reset_at: "2026-11-05T21:00:00Z" },
                        gemini_7d: { remaining: 30, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: null },
                        claude_7d: { remaining: null },
                        argon_7d:  { remaining: 30, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-d",
                    email: "cooling-d@example.com",
                    plan_type: "pro",
                    is_boosted: false,
                    reason: "health probe HTTP 404",
                    cooldown_until: "2026-11-05T18:05:00Z",
                    target: { priority: -1, disabled: false },
                    host: { priority: -1, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 0, reset_at: "2026-11-05T18:00:00Z" },
                        gemini_7d: { remaining: 50, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 60, reset_at: "2026-11-05T18:00:00Z" },
                        claude_7d: { remaining: 70, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 50, reset_at: "2026-11-12T00:00:00Z" }
                    }
                }
            ]
        },
        argon: {
            items: [
                {
                    auth_index: "auth-a",
                    email: "pro-a@example.com",
                    plan_type: "pro",
                    is_boosted: true,
                    target: { priority: 999, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 90, reset_at: "2026-11-05T20:00:00Z" },
                        gemini_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 70, reset_at: "2026-11-05T20:00:00Z" },
                        claude_7d: { remaining: 60, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 80, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-b",
                    email: "pro-b@example.com",
                    plan_type: "pro",
                    target: { priority: 100, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 50, reset_at: "2026-11-05T21:00:00Z" },
                        gemini_7d: { remaining: 40, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 80, reset_at: "2026-11-05T21:00:00Z" },
                        claude_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 30, reset_at: "2026-11-12T00:00:00Z" }
                    }
                }
            ]
        },
        claude_gpt: {
            items: [
                {
                    auth_index: "auth-b",
                    email: "pro-b@example.com",
                    plan_type: "pro",
                    is_boosted: true,
                    target: { priority: 999, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 50, reset_at: "2026-11-05T21:00:00Z" },
                        gemini_7d: { remaining: 40, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 80, reset_at: "2026-11-05T21:00:00Z" },
                        claude_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 30, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-a",
                    email: "pro-a@example.com",
                    plan_type: "pro",
                    is_boosted: false,
                    target: { priority: 100, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 90, reset_at: "2026-11-05T20:00:00Z" },
                        gemini_7d: { remaining: 85, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: 70, reset_at: "2026-11-05T20:00:00Z" },
                        claude_7d: { remaining: 60, reset_at: "2026-11-12T00:00:00Z" },
                        argon_7d:  { remaining: 80, reset_at: "2026-11-12T00:00:00Z" }
                    }
                },
                {
                    auth_index: "auth-c",
                    email: "free-c@example.com",
                    plan_type: "free",
                    routing_status: "pool_unavailable",
                    pool_eligible: false,
                    target: { priority: -1, disabled: false },
                    quota_windows: {
                        gemini_5h: { remaining: 40, reset_at: "2026-11-05T21:00:00Z" },
                        gemini_7d: { remaining: 30, reset_at: "2026-11-12T00:00:00Z" },
                        claude_5h: { remaining: null },
                        claude_7d: { remaining: null },
                        argon_7d:  { remaining: 30, reset_at: "2026-11-12T00:00:00Z" }
                    }
                }
            ]
        }
    }
};

latestSnapshot = sampleSnapshot;

(function() {
    // === SCENARIO 1: Select gemini-3.8-flash-high ===
    selectModel("gemini-3.8-flash-high");
    if (elements.modelSelect.value !== "gemini-3.8-flash-high") {
        throw new Error("selectModel failed to set modelSelect value");
    }

    // Verify KPI counters for Gemini
    if (String(elements.valTotalCreds.textContent) !== "4") {
        throw new Error("expected 4 total credentials, got " + elements.valTotalCreds.textContent);
    }
    if (String(elements.valBoosted.textContent) !== "1") {
        throw new Error("expected 1 boosted credential, got " + elements.valBoosted.textContent);
    }
    if (String(elements.valDepleted.textContent) !== "1") {
        throw new Error("expected 1 depleted/cooling credential, got " + elements.valDepleted.textContent);
    }

    const cardsG = elements.credentialsContainer.children;
    if (cardsG.length !== 4) {
        throw new Error("expected 4 rendered cards, got " + cardsG.length);
    }

    // Call order: Card 0 should be pro-a (Tier #1 · Boost), Card 1 should be pro-b (Tier #2), Card 2 should be free-c (Tier #3), Card 3 should be cooling-d
    if (cardsG[0].innerHTML.indexOf("pro-a@example.com") < 0 || cardsG[0].innerHTML.indexOf("Tier #1") < 0 || cardsG[0].innerHTML.indexOf("Boost") < 0) {
        throw new Error("card 0 in Gemini must be Tier #1 Boost pro-a: " + cardsG[0].innerHTML);
    }
    if (cardsG[1].innerHTML.indexOf("pro-b@example.com") < 0 || cardsG[1].innerHTML.indexOf("Tier #2") < 0) {
        throw new Error("card 1 in Gemini must be Tier #2 pro-b: " + cardsG[1].innerHTML);
    }
    if (cardsG[2].innerHTML.indexOf("free-c@example.com") < 0 || cardsG[2].innerHTML.indexOf("Tier #3") < 0) {
        throw new Error("card 2 in Gemini must be Tier #3 free-c: " + cardsG[2].innerHTML);
    }
    if (cardsG[3].innerHTML.indexOf("cooling-d@example.com") < 0 || cardsG[3].innerHTML.indexOf("冷却避让") < 0) {
        throw new Error("card 3 in Gemini must be bypassed cooling: " + cardsG[3].innerHTML);
    }

    // Verify all 5 quota bars rendered on every card
    const card0 = cardsG[0].innerHTML;
    const requiredBars = ["Gemini 5h", "Gemini 7d", "Claude 5h", "Claude 7d", "Argon 7d"];
    for (const bar of requiredBars) {
        if (card0.indexOf(bar) < 0) {
            throw new Error("card 0 missing quota bar: " + bar);
        }
    }

    // In Gemini route, Gemini 5h and Gemini 7d must be active-pool-meter
    if (card0.indexOf("active-pool-meter") < 0) {
        throw new Error("card 0 must contain active-pool-meter classes for governing bars");
    }

    // === SCENARIO 2: Select claude-sonnet-5-5-high ===
    selectModel("claude-sonnet-5-5-high");

    const cardsC = elements.credentialsContainer.children;
    // In Claude route: pro-b is top (priority 999 -> Tier #1 · Boost), pro-a is Tier #2, free-c is unsupported
    if (cardsC[0].innerHTML.indexOf("pro-b@example.com") < 0 || cardsC[0].innerHTML.indexOf("Tier #1") < 0) {
        throw new Error("card 0 in Claude route must be Tier #1 pro-b: " + cardsC[0].innerHTML);
    }
    // free-c should be marked unsupported model
    const lastCard = cardsC[cardsC.length - 1].innerHTML;
    if (lastCard.indexOf("free-c@example.com") < 0 || lastCard.indexOf("不支持该模型") < 0) {
        throw new Error("free plan without Claude quota must be marked unsupported in Claude route: " + lastCard);
    }

    // === SCENARIO 3: Select gemini-4-argon-high ===
    selectModel("gemini-4-argon-high");
    const cardsA = elements.credentialsContainer.children;
    if (cardsA.length !== 2) {
        throw new Error("expected 2 cards in Argon pool, got " + cardsA.length);
    }

    // === SCENARIO 4: Samples modal pool switcher ===
    cachedSamplesData = {
        groups: {
            gemini: { name: "Gemini", samples: [{ observed_at: "2026-11-05T12:00:00Z", short_window_rem: 90, long_window_rem: 85 }] },
            argon: { name: "Argon", samples: [{ observed_at: "2026-11-05T12:00:00Z", short_window_rem: 90, long_window_rem: 80 }] },
            claude_gpt: { name: "Claude", samples: [{ observed_at: "2026-11-05T12:00:00Z", short_window_rem: 70, long_window_rem: 60 }] }
        }
    };
    renderSamplesModalGroup("argon", "test@example.com");
    if (elements.samplesModalBody.innerHTML.indexOf("80%") < 0) {
        throw new Error("samples modal failed to render Argon sample data: " + elements.samplesModalBody.innerHTML);
    }
    renderSamplesModalGroup("claude_gpt", "test@example.com");
    if (elements.samplesModalBody.innerHTML.indexOf("60%") < 0) {
        throw new Error("samples modal failed to switch to Claude sample data: " + elements.samplesModalBody.innerHTML);
    }
})();
`

	runNodeFixture(t, node, "routing-board.js", harness)
}

func TestCredentialCardReorderAndSingleProbeLoading_NodeHarness(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; card animation behavior is checked in the Node-enabled verification environment")
	}

	harness := templateScriptAccountDisplay + templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptOverviewReset + templateScriptProbeSchedule + `
const toastCalls = [];
var activeRects = {};
var renderStartRects = {};
var renderEndRects = {};
function element(initial) {
    var el = Object.assign({
        hidden: false,
        textContent: "",
        _innerHTML: "",
        children: [],
        attributes: {},
        style: {},
        classList: {
            classes: [],
            add: function(c) { if (this.classes.indexOf(c) < 0) this.classes.push(c); },
            remove: function(c) { this.classes = this.classes.filter(function(x) { return x !== c; }); },
            contains: function(c) { return this.classes.indexOf(c) >= 0; }
        },
        appendChild: function(child) {
            this.children.push(child);
            if (child && child.innerHTML) this._innerHTML += child.innerHTML;
        },
        setAttribute: function(k, v) { this.attributes[k] = v; },
        getAttribute: function(k) {
            return Object.prototype.hasOwnProperty.call(this.attributes, k) ? this.attributes[k] : null;
        },
        getBoundingClientRect: function() {
            var key = this.attributes ? this.attributes["data-auth-index"] : "";
            var rect = activeRects[key] || { left: 0, top: 0 };
            return { left: rect.left, top: rect.top };
        },
        querySelector: function() { return null; },
        querySelectorAll: function() { return []; }
    }, initial || {});
    Object.defineProperty(el, "innerHTML", {
        get: function() { return this._innerHTML; },
        set: function(val) {
            this._innerHTML = val;
            if (val === "") { this.children = []; }
        }
    });
    return el;
}

const elements = {
    toastRoot: element(),
    modelSelect: element({ value: "gemini-3.8-flash-high" }),
    valLastAudit: element(),
    valNextProbe: element(),
    valTotalCreds: element(),
    valTotalDesc: element(),
    valBoosted: element(),
    valDepleted: element(),
    credentialsContainer: element()
};
elements.credentialsContainer.querySelectorAll = function(sel) {
    return sel === ".cred-card" ? this.children : [];
};
elements.credentialsContainer.appendChild = function(child) {
    activeRects = renderEndRects;
    this.children.push(child);
    if (child && child.innerHTML) this._innerHTML += child.innerHTML;
};

const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function(tag) { return element({ tagName: tag }); }
};

function showToast(msg, type) { toastCalls.push({ msg: msg, type: type }); }
function t(key) { return key === "probing" ? "探测中..." : key; }
function formatCountdown() { return "00h 00m"; }
function escapeHTML(s) { return String(s || ""); }

currentLang = "zh-CN";
dynamicConfig = null;
latestDiagnostics = null;
scheduleConfig = null;
isAuthBlocked = false;
const RUN_PATH = "/run";

function snapshotWithPriorities(priorityA, priorityB) {
    return {
        groups: {
            gemini: {
                items: [
                    { auth_index: "auth-a", email: "a@example.com", target: { priority: priorityA, disabled: false }, host: { priority: priorityA, disabled: false } },
                    { auth_index: "auth-b", email: "b@example.com", target: { priority: priorityB, disabled: false }, host: { priority: priorityB, disabled: false } }
                ]
            }
        }
    };
}

(async function() {
    // 1. Reordering must animate every displaced card from its previous slot.
    latestSnapshot = snapshotWithPriorities(900, 100);
    renderStartRects = { "auth-a": { left: 0, top: 0 }, "auth-b": { left: 0, top: 120 } };
    renderEndRects = { "auth-a": { left: 0, top: 0 }, "auth-b": { left: 0, top: 120 } };
    activeRects = renderStartRects;
    renderDashboard();

    const firstCards = elements.credentialsContainer.children;
    if (firstCards.length !== 2) {
        throw new Error("expected 2 cards, got " + firstCards.length);
    }
    if (firstCards[0].getAttribute("data-auth-index") !== "auth-a") {
        throw new Error("auth-a must start first");
    }

    latestSnapshot = snapshotWithPriorities(100, 900);
    renderStartRects = { "auth-a": { left: 0, top: 0 }, "auth-b": { left: 0, top: 120 } };
    renderEndRects = { "auth-a": { left: 0, top: 120 }, "auth-b": { left: 0, top: 0 } };
    activeRects = renderStartRects;
    renderDashboard();

    const movedCards = elements.credentialsContainer.children;
    if (movedCards.length !== 2) {
        throw new Error("expected 2 cards after reorder, got " + movedCards.length);
    }
    const movedB = movedCards[0];
    const movedA = movedCards[1];
    if (movedB.getAttribute("data-auth-index") !== "auth-b") {
        throw new Error("auth-b must render first after the swap");
    }
    if (String(movedB.style.transform) !== "translate(0px, 0px)") {
        throw new Error("reordered card must animate from its previous slot, got transform=" + movedB.style.transform);
    }
    if (String(movedB.style.transition).indexOf("transform 0.48s") < 0) {
        throw new Error("reordered card must carry a transform transition, got " + movedB.style.transition);
    }
    if (String(movedA.style.transform) !== "translate(0px, 0px)") {
        throw new Error("both displaced cards must animate, got transform=" + movedA.style.transform);
    }

    // 2. A stable layout must not animate at all.
    renderStartRects = { "auth-a": { left: 0, top: 120 }, "auth-b": { left: 0, top: 0 } };
    renderEndRects = renderStartRects;
    activeRects = renderStartRects;
    renderDashboard();
    const stableCards = elements.credentialsContainer.children;
    if (stableCards[0].style.transform || stableCards[0].style.transition) {
        throw new Error("unchanged layout must not apply inline transform/transition");
    }

    // 3. A single-credential probe must expose a card-level loading animation.
    latestSnapshot = snapshotWithPriorities(900, 100);
    renderDashboard();
    const probeCard = elements.credentialsContainer.children[0];
    if (probeCard.getAttribute("data-auth-index") !== "auth-a") {
        throw new Error("probe target must be the first card");
    }
    const probeBtn = element({ textContent: "⚡ 探测" });
    probeCard.querySelector = function(sel) {
        return sel === "[data-probe-action]" ? probeBtn : null;
    };

    let probingClassSeen = false;
    let spinnerSeen = false;
    apiFetch = async function() {
        probingClassSeen = probeCard.classList.contains("cred-card-probing");
        spinnerSeen = String(probeBtn.innerHTML).indexOf("probe-spinner") >= 0;
        return { probe_outcomes: [{ auth_index: "auth-a", model_group: "gemini", status: "success" }] };
    };
    fetchSnapshot = async function() {};

    await probeSingleCredential("auth-a", probeBtn);

    if (!probingClassSeen) {
        throw new Error("card must expose cred-card-probing while the probe is running");
    }
    if (!spinnerSeen) {
        throw new Error("probe button must render the spinner while the probe is running");
    }
    if (probeCard.classList.contains("cred-card-probing")) {
        throw new Error("probing class must be cleared after the probe completes");
    }
    if (String(probeBtn.innerHTML).indexOf("probe-spinner") >= 0) {
        throw new Error("probe button spinner must be cleared after the probe completes");
    }

    // 4. A re-render during the probe must preserve the loading affordance.
    probingAuthIndexes.add("auth-a");
    renderDashboard();
    const rerendered = elements.credentialsContainer.children[0];
    if (!rerendered.classList.contains("cred-card-probing")) {
        throw new Error("re-render during a probe must keep cred-card-probing");
    }
    if (String(rerendered.innerHTML).indexOf("probe-spinner") < 0) {
        throw new Error("re-render during a probe must keep the probe spinner");
    }
    probingAuthIndexes.delete("auth-a");
    renderDashboard();
    const settled = elements.credentialsContainer.children[0];
    if (settled.classList.contains("cred-card-probing")) {
        throw new Error("cleared probe state must not leak into a re-render");
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`

	runNodeFixture(t, node, "card-animation.js", harness)
}

func TestModelSelector_ExcludeToggleAndDashboardSync_NodeHarness(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not available; model exclusion dropdown behavior is checked in the Node-enabled verification environment")
	}

	harness := `
const toastCalls = [];
const apiCalls = [];
let currentLang = "zh-CN";
let isAuthBlocked = false;
let dynamicConfig = { excluded_models: [] };
let latestDiagnostics = null;
let scheduleConfig = null;
const EXCLUDED_MODELS_PATH = "/v0/management/plugins/antigravity-priority/excluded-models";

function makeClassList() {
    return {
        classes: [],
        add: function(c) { if (this.classes.indexOf(c) < 0) this.classes.push(c); },
        remove: function(c) { this.classes = this.classes.filter(function(x) { return x !== c; }); },
        contains: function(c) { return this.classes.indexOf(c) >= 0; },
        toggle: function(c, force) {
            if (force === undefined) force = !this.contains(c);
            if (force) this.add(c); else this.remove(c);
        }
    };
}

function element(initial) {
    var el = Object.assign({
        hidden: false,
        disabled: false,
        textContent: "",
        _innerHTML: "",
        children: [],
        attributes: {},
        style: {},
        classList: makeClassList(),
        appendChild: function(child) {
            this.children.push(child);
            if (child && child.innerHTML) this._innerHTML += child.innerHTML;
        },
        setAttribute: function(k, v) { this.attributes[k] = String(v); },
        getAttribute: function(k) {
            return Object.prototype.hasOwnProperty.call(this.attributes, k) ? this.attributes[k] : null;
        },
        querySelector: function() { return null; },
        querySelectorAll: function() { return []; }
    }, initial || {});
    Object.defineProperty(el, "innerHTML", {
        get: function() { return this._innerHTML; },
        set: function(val) {
            this._innerHTML = val;
            if (val === "") { this.children = []; }
        }
    });
    return el;
}

// Minimal option/button DOM nodes tracked after updateModelDropdownFromSnapshot
let optionNodes = [];
const elements = {
    toastRoot: element(),
    modelSelect: element({ value: "gemini-3-flash" }),
    customModelSelect: element(),
    customModelSelectMenu: element({ hidden: false }),
    customModelSelectBadge: element(),
    customModelSelectLabel: element(),
    customModelExcludedPill: element({ hidden: true }),
    valLastAudit: element(),
    valNextProbe: element(),
    valTotalCreds: element(),
    valTotalDesc: element(),
    valBoosted: element(),
    valDepleted: element(),
    credentialsContainer: element()
};
elements.customModelSelectMenu.querySelectorAll = function(sel) {
    if (sel === ".custom-select-option") return optionNodes;
    return [];
};
elements.credentialsContainer.querySelectorAll = function(sel) {
    return sel === ".cred-card" ? this.children : [];
};

const document = {
    getElementById: function(id) { return elements[id] || null; },
    createElement: function(tag) { return element({ tagName: tag }); },
    querySelectorAll: function() { return []; },
    addEventListener: function() {}
};
const window = {
    addEventListener: function() {},
    innerHeight: 800
};
function showToast(msg, type) { toastCalls.push({ msg: msg, type: type }); }
function escapeHTML(str) { return String(str || "").replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;"); }
` + featureOverviewTranslations + `
function t(key) {
    return (OVERVIEW_ZH["zh-CN"] && OVERVIEW_ZH["zh-CN"][key]) || key;
}
` + templateScriptAccountDisplay + templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptOverviewReset + templateScriptControls + templateScriptProbeSchedule + `
latestSnapshot = {
    models: [
        { id: "gemini-3-flash", display_name: "gemini-3-flash", quota_pool: "gemini" },
        { id: "gemini-3.8-flash-high", display_name: "gemini-3.8-flash-high", quota_pool: "gemini" }
    ],
    groups: {
        gemini: {
            items: [
                {
                    auth_index: "auth-a",
                    email: "alpha@example.com",
                    routing_status: "active",
                    pool_eligible: true,
                    supported_models: ["gemini-3-flash", "gemini-3.8-flash-high"],
                    target: { priority: 100, disabled: false },
                    host: { priority: 100, disabled: false }
                }
            ]
        }
    }
};

function rebuildOptionNodesFromMenuHTML() {
    const html = elements.customModelSelectMenu.innerHTML;
    optionNodes = [];
    const re = /data-value="([^"]+)"/g;
    let m;
    while ((m = re.exec(html)) !== null) {
        const modelId = m[1];
        const btnSpan = element({ textContent: isModelExcluded(modelId) ? t("btnExcludedModel") : t("btnExcludeModel") });
        const btn = element({
            attributes: { "data-model": modelId, "aria-pressed": isModelExcluded(modelId) ? "true" : "false" },
            classList: makeClassList(),
            querySelector: function(s) { return s === "span" ? btnSpan : null; }
        });
        if (isModelExcluded(modelId)) btn.classList.add("is-excluded");
        const opt = element({
            attributes: { "data-value": modelId },
            classList: makeClassList(),
            querySelector: function(s) { return s === ".model-exclude-btn" ? btn : null; }
        });
        if (isModelExcluded(modelId)) opt.classList.add("is-excluded");
        optionNodes.push(opt);
    }
}

apiFetch = async function(path, opts) {
    apiCalls.push({ path: path, body: JSON.parse(opts.body) });
    const body = JSON.parse(opts.body);
    if (body.excluded) {
        return { ok: true, model: body.model, excluded: true, excluded_models: [body.model] };
    }
    return { ok: true, model: body.model, excluded: false, excluded_models: [] };
};

(async function() {
    updateModelDropdownFromSnapshot(latestSnapshot.models);
    rebuildOptionNodesFromMenuHTML();
    if (elements.customModelSelectMenu.innerHTML.indexOf("model-exclude-btn") < 0) {
        throw new Error("dropdown menu must render model-exclude-btn");
    }
    renderDashboard();
    if (elements.valTotalDesc.textContent.indexOf("1") < 0) {
        throw new Error("expected 1 active credential before exclusion, got " + elements.valTotalDesc.textContent);
    }

    let stopped = false;
    const btn = optionNodes[0].querySelector(".model-exclude-btn");
    await toggleModelExcluded("gemini-3-flash", btn, { stopPropagation: function() { stopped = true; } });
    if (!stopped) {
        throw new Error("toggleModelExcluded must stop click propagation");
    }
    if (apiCalls.length !== 1 || apiCalls[0].path !== EXCLUDED_MODELS_PATH || apiCalls[0].body.excluded !== true) {
        throw new Error("unexpected API call on exclude: " + JSON.stringify(apiCalls));
    }
    if (elements.customModelSelectMenu.hidden !== false) {
        throw new Error("toggling exclude must keep dropdown open");
    }
    if (elements.customModelExcludedPill.hidden !== false) {
        throw new Error("active excluded model must show customModelExcludedPill");
    }
    if (!optionNodes[0].classList.contains("is-excluded") || !btn.classList.contains("is-excluded")) {
        throw new Error("excluded model option and button must receive is-excluded class");
    }
    if (elements.valTotalDesc.textContent.indexOf("0") < 0 || elements.credentialsContainer.innerHTML.indexOf("已从 AG 排除") < 0 || elements.credentialsContainer.innerHTML.indexOf("title=\"") < 0) {
        throw new Error("excluded active model must bypass cards with excluded badge and tooltip, got: " + elements.credentialsContainer.innerHTML);
    }

    // Restore
    await toggleModelExcluded("gemini-3-flash", btn, { stopPropagation: function() {} });
    if (elements.customModelExcludedPill.hidden !== true) {
        throw new Error("restoring active model must hide customModelExcludedPill");
    }
    if (elements.valTotalDesc.textContent.indexOf("1") < 0) {
        throw new Error("restored model must return credential to active, got " + elements.valTotalDesc.textContent);
    }
})().catch(function(err) {
    console.error(err.stack || err.message);
    process.exit(1);
});
`
	runNodeFixture(t, node, "model-exclude-toggle.js", harness)
}
