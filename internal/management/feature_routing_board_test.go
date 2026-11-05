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
    modelGroupSelect: element({value: "gemini"}),
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

function showToast(msg, type) {
    toastCalls.push({msg: msg, type: type});
}
` + templateScriptOverviewActionsCore + templateScriptOverviewRender + templateScriptControls + templateScriptProbeSchedule + templateScriptOverviewSamples + `
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
    predictedPriority: "预测",
    unsetPriority: "[未设置]",
    badgeGemini: "Gemini",
    badgeArgon: "Argon",
    badgeClaudeGPT: "Claude/GPT"
};

function t(key) { return ZH[key] || key; }
function escapeHTML(s) { return String(s || ""); }
function formatCountdown(d) { return "02h 15m"; }

currentLang = "zh-CN";
dynamicConfig = { auto_apply: true, antigravity_model_group: "gemini" };
latestDiagnostics = { latest_audit: "scheduled probe completed: 4 observations" };
scheduleConfig = null;
isAuthBlocked = false;
userSelectedModelGroup = false;

// Mock 4 accounts across gemini, argon, and claude_gpt
const sampleSnapshot = {
    active_model_group: "gemini",
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
                    current: { priority: 999, disabled: false },
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
                    current: { priority: 100, disabled: false },
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
                    current: { priority: 10, disabled: false },
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
                    reason: "429 rate limit cooldown",
                    target: { priority: -1, disabled: false },
                    current: { priority: -1, disabled: false },
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
                    target: { priority: 10, disabled: false },
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
    if (elements.modelGroupSelect.value !== "gemini") {
        throw new Error("selectModel failed to sync modelGroupSelect to gemini");
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
    if (elements.modelGroupSelect.value !== "claude_gpt") {
        throw new Error("selectModel failed to sync to claude_gpt");
    }

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
    if (elements.modelGroupSelect.value !== "argon") {
        throw new Error("selectModel failed to sync to argon");
    }
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
