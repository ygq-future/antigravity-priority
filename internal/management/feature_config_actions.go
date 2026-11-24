package management

import (
	"encoding/json"
	"strings"

	"antigravity-priority/internal/config"
)

func buildDefaultPromptGuardRulesJSON() string {
	encoded, err := json.Marshal(config.DefaultPromptGuardRules())
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

var templateScriptConfig = strings.Replace(templateScriptConfigRaw, "__DEFAULT_PROMPT_GUARD_RULES_JSON__", buildDefaultPromptGuardRulesJSON(), 1)

const templateScriptConfigRaw = `        let originalConfigState = null;
        const DEFAULT_PROMPT_GUARD_RULES = __DEFAULT_PROMPT_GUARD_RULES_JSON__;
        const MAX_PROMPT_GUARD_RULES = 200;
        let promptGuardRules = clonePromptGuardRules(DEFAULT_PROMPT_GUARD_RULES);

        function clonePromptGuardRules(rules) {
            if (!Array.isArray(rules)) return [];
            return rules.map(function(r) {
                return {
                    on: r && r.on !== false,
                    find: r && typeof r.find === "string" ? r.find : "",
                    replace: r && typeof r.replace === "string" ? r.replace : "",
                    scope: r && r.scope === "all" ? "all" : "system",
                    mode: r && r.mode === "regex" ? "regex" : "text",
                    flags: r && typeof r.flags === "string" ? r.flags : "",
                    custom: Boolean(r && r.custom)
                };
            });
        }

        async function fetchDynamicConfig() {
            if (isAuthBlocked) return;
            try {
                dynamicConfig = await apiFetch(CONFIG_PATH);
                renderDynamicConfigForm(dynamicConfig);
                renderScheduleStatus();
                if (typeof syncModelExclusionDropdownUI === "function") syncModelExclusionDropdownUI();
                if (typeof renderDashboard === "function") renderDashboard();
            } catch (err) {
                if (!isAuthBlocked) showToast(err.message, "error");
            }
        }

        function getDynamicConfigFormData() {
            var autoApply = Boolean(document.getElementById("cfgAutoApply") && document.getElementById("cfgAutoApply").checked);
            var intervalSelect = document.getElementById("cfgIntervalSelect");
            var intervalCustom = document.getElementById("cfgIntervalCustom");
            var interval = "15m";
            if (intervalSelect) {
                interval = intervalSelect.value === "custom" ? ((intervalCustom && intervalCustom.value.trim()) || "") : intervalSelect.value;
            }
            var maxConcurrency = (document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "";
            var urgencyTol = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "";
            var sampleCapacity = (document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "";
            var cooldownMin = (document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "";
            var warmupGeminiEnabled = Boolean(document.getElementById("cfgWarmupGeminiEnabled") && document.getElementById("cfgWarmupGeminiEnabled").checked);
            var warmupGeminiModel = (document.getElementById("cfgWarmupGeminiModelSelect") && document.getElementById("cfgWarmupGeminiModelSelect").value) || "gemini-3-flash";
            var warmupClaudeEnabled = Boolean(document.getElementById("cfgWarmupClaudeEnabled") && document.getElementById("cfgWarmupClaudeEnabled").checked);
            var warmupClaudeModel = (document.getElementById("cfgWarmupClaudeModelSelect") && document.getElementById("cfgWarmupClaudeModelSelect").value) || "claude-sonnet-5-5-high";
            var pgEnabled = Boolean(document.getElementById("cfgPromptGuardEnabled") && document.getElementById("cfgPromptGuardEnabled").checked);

            return JSON.stringify({
                autoApply, interval,
                maxConcurrency, urgencyTol, sampleCapacity, cooldownMin,
                warmupGeminiEnabled, warmupGeminiModel, warmupClaudeEnabled, warmupClaudeModel,
                pgEnabled, promptGuardRules
            });
        }

        function updateSaveButtonState() {
            var btn = document.getElementById("btnSaveConfig");
            if (!btn) return;
            if (!originalConfigState) {
                btn.disabled = true;
                return;
            }
            var current = getDynamicConfigFormData();
            btn.disabled = current === originalConfigState;
        }

        function updatePromptGuardToggleText(enabled) {
            var pgText = document.getElementById("cfgPromptGuardStatusText");
            if (pgText) {
                pgText.textContent = enabled ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
            }
        }

        function updatePromptGuardUILabels() {
            var pgCheckbox = document.getElementById("cfgPromptGuardEnabled");
            if (pgCheckbox) updatePromptGuardToggleText(Boolean(pgCheckbox.checked));
            var autoApply = document.getElementById("cfgAutoApply");
            var autoText = document.getElementById("cfgAutoApplyStatusText");
            if (autoApply && autoText) {
                autoText.textContent = autoApply.checked ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
            }
            renderPromptGuardRules();
        }

        function renderPromptGuardRules(focusIndex) {
            var listEl = document.getElementById("cfgPgRulesList");
            var countEl = document.getElementById("cfgPgRuleCount");
            if (countEl) {
                countEl.textContent = promptGuardRules.length + " / " + MAX_PROMPT_GUARD_RULES;
            }
            var addBtn = document.getElementById("btnPgAddRule");
            if (addBtn) {
                addBtn.disabled = promptGuardRules.length >= MAX_PROMPT_GUARD_RULES;
            }
            if (!listEl) return;

            var searchInput = document.getElementById("cfgPgSearch");
            var query = searchInput ? searchInput.value.trim().toLowerCase() : "";

            if (promptGuardRules.length === 0) {
                listEl.innerHTML = '<div class="pg-empty-box">' + escapeHTML(t("cfgPgEmptyRules")) + "</div>";
                return;
            }

            var htmlParts = [];
            var renderedCount = 0;
            for (var i = 0; i < promptGuardRules.length; i++) {
                var rule = promptGuardRules[i];
                if (query) {
                    var hay = (rule.find + "\n" + rule.replace).toLowerCase();
                    if (hay.indexOf(query) < 0) continue;
                }
                renderedCount++;
                var badgeLabel = rule.custom ? t("cfgPgBadgeCustom") : t("cfgPgBadgeRemote");
                var badgeCls = rule.custom ? "pg-source-badge pg-source-custom" : "pg-source-badge pg-source-remote";
                var badgeTitle = rule.custom ? t("cfgPgBadgeCustomTitle") : t("cfgPgBadgeRemoteTitle");
                var modeLabel = rule.mode === "regex" ? t("cfgPgModeRegex") : t("cfgPgModeText");
                var modeCls = rule.mode === "regex" ? "pg-pill-btn active-regex" : "pg-pill-btn";
                var rowCls = rule.on ? "pg-rule-row" : "pg-rule-row pg-rule-disabled";
                htmlParts.push(
                    '<div class="' + rowCls + '" data-pg-index="' + i + '">' +
                        '<label class="pg-mini-toggle">' +
                            '<input type="checkbox" ' + (rule.on ? "checked " : "") + 'onchange="togglePromptGuardRule(' + i + ', this.checked)">' +
                            '<span class="pg-mini-slider"></span>' +
                        "</label>" +
                        '<span class="' + badgeCls + '" title="' + escapeHTML(badgeTitle) + '">' + escapeHTML(badgeLabel) + "</span>" +
                        '<input type="text" class="pg-rule-input pg-find-input" data-pg-field="find" autocomplete="off" value="' + escapeHTML(rule.find) + '" placeholder="' + escapeHTML(t("cfgPgFindPlaceholder")) + '" oninput="handlePromptGuardRuleInput(this)">' +
                        '<span class="pg-arrow">→</span>' +
                        '<input type="text" class="pg-rule-input pg-replace-input" data-pg-field="replace" autocomplete="off" value="' + escapeHTML(rule.replace) + '" placeholder="' + escapeHTML(t("cfgPgReplacePlaceholder")) + '" oninput="handlePromptGuardRuleInput(this)">' +
                        '<button type="button" class="' + modeCls + '" onclick="cyclePromptGuardRuleMode(' + i + ')">' + escapeHTML(modeLabel) + "</button>" +
                        '<button type="button" class="pg-delete-btn" onclick="deletePromptGuardRule(' + i + ')" title="Delete">✕</button>' +
                    "</div>"
                );
            }
            if (renderedCount === 0) {
                listEl.innerHTML = '<div class="pg-empty-box">' + escapeHTML(t("cfgPgFilteredEmpty")) + "</div>";
                return;
            }
            listEl.innerHTML = htmlParts.join("");
            if (typeof focusIndex === "number" && focusIndex >= 0) {
                var target = listEl.querySelector('.pg-rule-row[data-pg-index="' + focusIndex + '"] .pg-find-input');
                if (target && typeof target.focus === "function") target.focus();
            }
        }

        function handlePromptGuardRuleInput(el) {
            if (!el) return;
            var row = el.closest ? el.closest("[data-pg-index]") : null;
            if (!row) return;
            var index = Number(row.getAttribute("data-pg-index"));
            var field = el.getAttribute("data-pg-field");
            if (!isNaN(index) && (field === "find" || field === "replace")) {
                updatePromptGuardRuleField(index, field, el.value);
            }
        }

        function togglePromptGuardRule(index, checked) {
            if (!promptGuardRules[index]) return;
            promptGuardRules[index].on = Boolean(checked);
            renderPromptGuardRules();
            updateSaveButtonState();
        }

        function updatePromptGuardRuleField(index, field, value) {
            if (!promptGuardRules[index]) return;
            promptGuardRules[index][field] = String(value || "");
            updateSaveButtonState();
        }

        function cyclePromptGuardRuleScope(index) {
            if (!promptGuardRules[index]) return;
            promptGuardRules[index].scope = promptGuardRules[index].scope === "all" ? "system" : "all";
            renderPromptGuardRules();
            updateSaveButtonState();
        }

        function cyclePromptGuardRuleMode(index) {
            if (!promptGuardRules[index]) return;
            promptGuardRules[index].mode = promptGuardRules[index].mode === "regex" ? "text" : "regex";
            renderPromptGuardRules();
            updateSaveButtonState();
        }

        function deletePromptGuardRule(index) {
            if (index < 0 || index >= promptGuardRules.length) return;
            promptGuardRules.splice(index, 1);
            renderPromptGuardRules();
            updateSaveButtonState();
        }

        function addPromptGuardRule() {
            if (promptGuardRules.length >= MAX_PROMPT_GUARD_RULES) {
                showToast(t("valErrPgMaxRules"), "error");
                return;
            }
            var searchInput = document.getElementById("cfgPgSearch");
            if (searchInput && searchInput.value) searchInput.value = "";
            promptGuardRules.unshift({
                on: true,
                find: "",
                replace: "",
                scope: "system",
                mode: "text",
                flags: "",
                custom: true
            });
            renderPromptGuardRules(0);
            updateSaveButtonState();
        }

        async function syncPromptGuardRemoteRules() {
            var btn = document.getElementById("btnPgSyncRemote");
            if (btn) btn.disabled = true;
            try {
                var res = await apiFetch(PROMPT_GUARD_SYNC_PATH, {
                    method: "POST",
                    body: JSON.stringify({})
                });
                if (res && Array.isArray(res.rules)) {
                    promptGuardRules = clonePromptGuardRules(res.rules);
                    renderPromptGuardRules();
                    updateSaveButtonState();
                    var summary = res.summary || {};
                    var msg = currentLang === "zh-CN"
                        ? ("词库同步成功：新增 " + (summary.added || 0) + " 条，更新 " + (summary.updated || 0) + " 条，删除 " + (summary.deleted || 0) + " 条，保留自定义 " + (summary.preserved_custom || 0) + " 条")
                        : ("Sync complete: " + (summary.added || 0) + " added, " + (summary.updated || 0) + " updated, " + (summary.deleted || 0) + " deleted, " + (summary.preserved_custom || 0) + " custom preserved");
                    showToast(msg, "success");
                }
            } catch (err) {
                showToast(err.message || t("cfgPgSyncError"), "error");
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        function buildPromptGuardBisectPrompt() {
            if (currentLang === "zh-CN") {
                return [
                    "我的请求通过 Antigravity 上游触发了通用 429 (\x60RESOURCE_EXHAUSTED: Resource has been exhausted (e.g. check quota).\x60 且无 details)，这通常是因为系统提示词中的某个关键词/短语命中了上游风控。",
                    "",
                    "请你使用**二分法（Binary Search）**帮我定位具体是系统提示词中的哪一个关键词触发了 429：",
                    "1. 构造最小复现：尽量使用快速的 flash-lite 小模型（如 \x60gemini-3.5-flash-lite\x60）进行验证；保留触发 429 的原始系统提示词（将 \x60max_tokens\x60 设为 \x6016\x60 等最小值，用户消息设为简单内容如 \x60hi\x60），确认稳定返回该通用 429。",
                    "请求超时设为 **10 秒**：flash-lite 正常 3 秒左右回复；命中风控时上游会反复重试、30 秒以上才吐出 429，所以**超过 10 秒没有任何响应就直接判定为 429，不要等它返回**。",
                    "2. 刷新 429 冷却状态（关键）：**每次遇到 429 后**，都必须先发送一次小的、干净的、不带任何系统提示词的简短提问（如 \x60hi\x60）来刷新冷却状态，确认返回 **200** 后再进行下一轮 429 排查；如果干净请求同样返回 429，说明前面尝试过多已影响干净请求，需**停下来冷却 2 分钟**，再继续用干净请求验证，直到回复 200 才能继续尝试碰撞 429。",
                    "3. 二分系统提示词：将 \x60system\x60 / \x60instructions\x60 / \x60systemInstruction\x60 文本对半拆分，分别测试保留前半段和保留后半段，哪一半继续触发 429 就继续对该半对半拆分测试。",
                    "4. 缩窄至最短短语：逐层从段落 -> 句子 -> 词组继续二分，直到找出删除其中任意关键单词就不再触发 429 的**最短连续触发短语**。",
                    "5. 输出结果：告诉我定位到的「触发原文」以及建议的「无害替换文本」，我将直接添加到 Prompt Guard 规则中（仅作用于系统提示词）。"
                ].join("\n");
            }
            return [
                "My request hit a generic upstream 429 (\x60RESOURCE_EXHAUSTED: Resource has been exhausted (e.g. check quota).\x60 with no details), which is usually caused by a specific keyword or phrase in the system prompt.",
                "",
                "Please use **binary search (bisecting)** to isolate the exact trigger phrase in the system prompt:",
                "1. Minimal repro: Prefer a fast flash-lite model (e.g. \x60gemini-3.5-flash-lite\x60) for verification; keep the failing system prompt with minimal \x60max_tokens\x60 (e.g. \x6016\x60) and a simple user message like \x60hi\x60, and confirm it reliably returns the generic 429.",
                "Set the request timeout to **10 seconds**: flash-lite normally replies in ~3s, while a throttled upstream retries for 30s+ before emitting 429—so **treat 10 seconds with no response as a 429 and do not wait for it**.",
                "2. Refresh 429 cooldown (important): **After every 429**, send a small, clean, short request without any system prompt (e.g. \x60hi\x60) to clear the cooldown and wait for a **200** before the next round. If the clean request also returns 429, prior attempts have spilled over—**pause for 2 minutes** to cool down, then retry the clean request until it returns 200 before resuming 429 probing.",
                "3. Bisect system instructions: Split \x60system\x60 / \x60instructions\x60 / \x60systemInstruction\x60 in half and test each half separately, recursing into whichever half still triggers 429.",
                "4. Narrow down: Shrink from paragraph -> sentence -> phrase until you find the **shortest contiguous trigger phrase** whose removal stops the 429.",
                "5. Report back: Provide the exact trigger phrase and a harmless replacement text so I can add it as a Prompt Guard rule (system prompt only)."
            ].join("\n");
        }

        function copyPromptGuardText(text, successKey) {
            if (!text) return;
            if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(function() {
                    showToast(t(successKey), "success");
                }).catch(function() {
                    showToast(t("cfgPgCopyBisectError"), "error");
                });
                return;
            }
            try {
                var ta = document.createElement("textarea");
                ta.value = text;
                ta.style.position = "fixed";
                ta.style.opacity = "0";
                document.body.appendChild(ta);
                ta.select();
                var ok = document.execCommand("copy");
                document.body.removeChild(ta);
                showToast(ok ? t(successKey) : t("cfgPgCopyBisectError"), ok ? "success" : "error");
            } catch (e) {
                showToast(t("cfgPgCopyBisectError"), "error");
            }
        }

        function copyPromptGuardBisectPrompt() {
            copyPromptGuardText(buildPromptGuardBisectPrompt(), "cfgPgCopyBisectSuccess");
        }

        function copyPromptGuardPreviewOutput() {
            var outEl = document.getElementById("cfgPgPreviewOutput");
            if (!outEl) return;
            var text = outEl.value !== undefined ? outEl.value : (outEl.textContent || "");
            copyPromptGuardText(text, "cfgPgCopyOutputSuccess");
        }

        function toggleCustomPgFormatSelect(event) {
            if (event) event.stopPropagation();
            if (typeof closeAllCustomSelects === "function") closeAllCustomSelects("customPgFormatSelect");
            if (typeof toggleCustomSelectMenu === "function") toggleCustomSelectMenu("customPgFormatSelect", "customPgFormatMenu");
        }

        function selectPgFormatOption(value, event) {
            if (event) event.stopPropagation();
            var select = document.getElementById("cfgPgPreviewFormat");
            if (select) select.value = value;
            updateCustomPgFormatDisplay();
            if (typeof closeAllCustomSelects === "function") closeAllCustomSelects();
        }

        function updateCustomPgFormatDisplay() {
            var select = document.getElementById("cfgPgPreviewFormat");
            var label = document.getElementById("customPgFormatLabel");
            if (select && label && select.options && select.selectedIndex >= 0) {
                var selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) label.textContent = selectedOpt.text || selectedOpt.value;
            }
            var menu = document.getElementById("customPgFormatMenu");
            if (menu && select) {
                menu.querySelectorAll(".custom-select-option").forEach(function(opt) {
                    opt.classList.toggle("selected", opt.getAttribute("data-value") === select.value);
                });
            }
        }

        async function runPromptGuardPreview() {
            var inputEl = document.getElementById("cfgPgPreviewInput");
            var formatEl = document.getElementById("cfgPgPreviewFormat");
            var resultEl = document.getElementById("cfgPgPreviewResult");
            if (!inputEl || !resultEl) return;

            var rawFormat = (formatEl && formatEl.value) || "auto";
            var format = rawFormat;
            var scope = "system";
            if (rawFormat === "text:system") {
                format = "text";
                scope = "system";
            } else if (rawFormat === "text:all") {
                format = "text";
                scope = "all";
            }

            try {
                var res = await apiFetch(PROMPT_GUARD_PREVIEW_PATH, {
                    method: "POST",
                    body: JSON.stringify({
                        input: inputEl.value || "",
                        format: format,
                        scope: scope,
                        rules: promptGuardRules
                    })
                });
                var parts = [];
                var summaryText = res.total_hits > 0
                    ? (t("cfgPgPreviewHitsPrefix") + String(res.total_hits) + " (" + String(res.detected_format || format) + ")")
                    : (t("cfgPgPreviewNoHits") + " (" + String(res.detected_format || format) + ")");
                parts.push('<div class="pg-preview-summary"><span>' + escapeHTML(summaryText) + '</span><button type="button" class="btn-secondary pg-preview-copy-btn" onclick="copyPromptGuardPreviewOutput()">' + escapeHTML(t("cfgPgBtnCopyOutput")) + '</button></div>');
                if (Array.isArray(res.warnings) && res.warnings.length > 0) {
                    parts.push('<div class="pg-preview-warnings">' + res.warnings.map(function(w) {
                        return '<span class="pg-warn-chip">' + escapeHTML(w) + "</span>";
                    }).join("") + "</div>");
                }
                if (Array.isArray(res.matches) && res.matches.length > 0) {
                    parts.push('<div class="pg-preview-matches">' + res.matches.map(function(m) {
                        var replaceHtml = m.replace ? escapeHTML(m.replace) : '<span class="pg-match-empty">(' + escapeHTML(t("cfgPgMatchEmpty")) + ')</span>';
                        return '<div class="pg-match-card">' +
                            '<div class="pg-match-diff">' +
                                '<div class="pg-match-find">' + escapeHTML(m.find) + '</div>' +
                                '<div class="pg-match-replace">' + replaceHtml + '</div>' +
                            '</div>' +
                            '<span class="pg-match-count">×' + Number(m.hits) + '</span>' +
                        '</div>';
                    }).join("") + "</div>");
                }
                parts.push('<textarea id="cfgPgPreviewOutput" class="pg-preview-output" readonly rows="8">' + escapeHTML(res.output || "") + "</textarea>");
                resultEl.innerHTML = parts.join("");
                resultEl.hidden = false;
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        function renderDynamicConfigForm(cfg) {
            if (!cfg) return;

            var autoApply = document.getElementById("cfgAutoApply");
            var autoText = document.getElementById("cfgAutoApplyStatusText");
            if (autoApply) {
                autoApply.checked = Boolean(cfg.auto_apply);
                if (autoText) autoText.textContent = cfg.auto_apply ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
            }

            var intervalSelect = document.getElementById("cfgIntervalSelect");
            var intervalCustom = document.getElementById("cfgIntervalCustom");
            var intervalVal = cfg.interval || "15m";
            if (intervalSelect) {
                if (["5m", "15m", "30m", "1h"].indexOf(intervalVal) >= 0) {
                    intervalSelect.value = intervalVal;
                    if (intervalCustom) intervalCustom.hidden = true;
                } else {
                    intervalSelect.value = "custom";
                    if (intervalCustom) {
                        intervalCustom.value = intervalVal;
                        intervalCustom.hidden = false;
                    }
                }
                updateCustomIntervalDisplay();
            }

            var maxConcurrency = document.getElementById("cfgMaxConcurrency");
            if (maxConcurrency) maxConcurrency.value = cfg.max_concurrency || 6;

            var urgencyTol = document.getElementById("cfgUrgencyTolerance");
            if (urgencyTol) urgencyTol.value = cfg.urgency_tolerance !== undefined ? cfg.urgency_tolerance : 0.05;

            var sampleCapacity = document.getElementById("cfgSampleCapacity");
            if (sampleCapacity) sampleCapacity.value = cfg.quota_sample_capacity !== undefined ? cfg.quota_sample_capacity : 6;

            var cooldownMin = document.getElementById("cfgCooldownMinutes");
            if (cooldownMin) cooldownMin.value = cfg.rate_limit_cooldown_minutes !== undefined ? cfg.rate_limit_cooldown_minutes : 5;

            var warmup = cfg.warmup || {};
            var wGemini = document.getElementById("cfgWarmupGeminiEnabled");
            if (wGemini) wGemini.checked = Boolean(warmup.gemini_enabled);
            var wGeminiModel = document.getElementById("cfgWarmupGeminiModelSelect");
            if (wGeminiModel) wGeminiModel.value = warmup.gemini_model || "gemini-3-flash";
            if (typeof updateCustomWarmupGeminiDisplay === "function") updateCustomWarmupGeminiDisplay();

            var wClaude = document.getElementById("cfgWarmupClaudeEnabled");
            if (wClaude) wClaude.checked = Boolean(warmup.claude_enabled);
            var wClaudeModel = document.getElementById("cfgWarmupClaudeModelSelect");
            if (wClaudeModel) wClaudeModel.value = warmup.claude_model || "claude-sonnet-5-5-high";
            if (window.cpaAvailableModels && typeof populateWarmupModelDropdowns === "function") {
                populateWarmupModelDropdowns(window.cpaAvailableModels);
            } else if (typeof updateCustomWarmupGeminiDisplay === "function") {
                updateCustomWarmupGeminiDisplay();
                updateCustomWarmupClaudeDisplay();
            }

            var pg = cfg.prompt_guard || {};
            var pgEnabled = document.getElementById("cfgPromptGuardEnabled");
            if (pgEnabled) {
                pgEnabled.checked = Boolean(pg.enabled);
                updatePromptGuardToggleText(Boolean(pg.enabled));
            }
            promptGuardRules = Array.isArray(pg.rules) ? clonePromptGuardRules(pg.rules) : clonePromptGuardRules(DEFAULT_PROMPT_GUARD_RULES);
            renderPromptGuardRules();
            updateCustomPgFormatDisplay();

            // Sync original snapshot and disable save button
            originalConfigState = getDynamicConfigFormData();
            updateSaveButtonState();
        }

        function onIntervalSelectChange() {
            var select = document.getElementById("cfgIntervalSelect");
            var custom = document.getElementById("cfgIntervalCustom");
            if (!select || !custom) return;
            custom.hidden = select.value !== "custom";
            if (select.value === "custom" && !custom.value) {
                custom.value = "20m";
            }
            updateSaveButtonState();
        }

        document.addEventListener("change", function(e) {
            if (e.target && e.target.id === "cfgAutoApply") {
                var autoText = document.getElementById("cfgAutoApplyStatusText");
                if (autoText) {
                    autoText.textContent = e.target.checked ? (currentLang === "zh-CN" ? "已开启" : "Enabled") : (currentLang === "zh-CN" ? "已关闭" : "Disabled");
                }
            }
            if (e.target && e.target.id === "cfgPromptGuardEnabled") {
                updatePromptGuardToggleText(Boolean(e.target.checked));
            }
            if (e.target && e.target.closest("#panelConfig")) {
                updateSaveButtonState();
            }
        });

        document.addEventListener("input", function(e) {
            if (e.target && e.target.closest("#panelConfig")) {
                updateSaveButtonState();
            }
        });

        // Comprehensive Frontend Value Validation
        function parseGoDurationMillis(value) {
            var text = String(value || "").trim();
            if (!text) return NaN;
            if (text.charAt(0) === "+") text = text.slice(1);
            var units = { ns: 0.000001, us: 0.001, "µs": 0.001, "μs": 0.001, ms: 1, s: 1000, m: 60000, h: 3600000 };
            var token = /([0-9]+(?:\.[0-9]*)?|\.[0-9]+)(ns|us|µs|μs|ms|s|m|h)/g;
            var total = 0;
            var consumed = "";
            var match;
            while ((match = token.exec(text)) !== null) {
                consumed += match[0];
                total += Number(match[1]) * units[match[2]];
            }
            return consumed === text ? total : NaN;
        }

        function parseStrictInteger(value) {
            var text = String(value || "").trim();
            return /^-?\d+$/.test(text) ? Number(text) : NaN;
        }

        async function saveDynamicConfig() {
            var btn = document.getElementById("btnSaveConfig");
            if (btn) btn.disabled = true;

            try {
                var autoApply = Boolean(document.getElementById("cfgAutoApply") && document.getElementById("cfgAutoApply").checked);
                var intervalSelect = document.getElementById("cfgIntervalSelect");
                var intervalCustom = document.getElementById("cfgIntervalCustom");
                var interval = "15m";
                if (intervalSelect) {
                    if (intervalSelect.value === "custom") {
                        interval = (intervalCustom && intervalCustom.value.trim()) || "";
                        if (!interval || parseGoDurationMillis(interval) < 60000) {
                            showToast(t("valErrInterval"), "error");
                            if (intervalCustom) intervalCustom.focus();
                            updateSaveButtonState();
                            return;
                        }
                    } else {
                        interval = intervalSelect.value;
                    }
                }

                var maxConcurrency = parseStrictInteger((document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "6");
                if (isNaN(maxConcurrency) || maxConcurrency < 1 || maxConcurrency > 32) {
                    showToast(t("valErrConcurrency"), "error");
                    updateSaveButtonState();
                    return;
                }

                var urgencyTolRaw = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "0.05";
                var urgencyTol = Number(urgencyTolRaw);
                if (urgencyTolRaw.trim() === "" || isNaN(urgencyTol) || urgencyTol < 0.0 || urgencyTol > 0.50) {
                    showToast(t("valErrUrgencyTol"), "error");
                    updateSaveButtonState();
                    return;
                }

                var sampleCapacity = parseStrictInteger((document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "6");
                if (isNaN(sampleCapacity) || sampleCapacity < 2 || sampleCapacity > 30) {
                    showToast(t("valErrSampleCapacity"), "error");
                    updateSaveButtonState();
                    return;
                }

                var cooldownMin = parseStrictInteger((document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "5");
                if (isNaN(cooldownMin) || cooldownMin < 1 || cooldownMin > 1440) {
                    showToast(t("valErrCooldown"), "error");
                    updateSaveButtonState();
                    return;
                }

                if (promptGuardRules.length > MAX_PROMPT_GUARD_RULES) {
                    showToast(t("valErrPgMaxRules"), "error");
                    updateSaveButtonState();
                    return;
                }

                var reqBody = {
                    auto_apply: autoApply,
                    interval: interval,
                    max_concurrency: maxConcurrency,
                    urgency_tolerance: urgencyTol,
                    rate_limit_cooldown_minutes: cooldownMin,
                    quota_sample_capacity: sampleCapacity,
                    request_history_limit: (dynamicConfig && dynamicConfig.request_history_limit !== undefined) ? dynamicConfig.request_history_limit : 100,
                    schedule: {
                        paused: Boolean(
                            (typeof scheduleConfig !== "undefined" && scheduleConfig && scheduleConfig.paused) ||
                            (typeof latestDiagnostics !== "undefined" && latestDiagnostics && latestDiagnostics.scheduler && latestDiagnostics.scheduler.paused) ||
                            (dynamicConfig && dynamicConfig.schedule && dynamicConfig.schedule.paused)
                        )
                    },
                    custom_tiers: (dynamicConfig && dynamicConfig.custom_tiers) ? dynamicConfig.custom_tiers : undefined,
                    excluded_models: (dynamicConfig && Array.isArray(dynamicConfig.excluded_models)) ? dynamicConfig.excluded_models : undefined,
                    warmup: {
                        gemini_enabled: Boolean(document.getElementById("cfgWarmupGeminiEnabled") && document.getElementById("cfgWarmupGeminiEnabled").checked),
                        gemini_model: (document.getElementById("cfgWarmupGeminiModelSelect") && document.getElementById("cfgWarmupGeminiModelSelect").value) || "gemini-3-flash",
                        claude_enabled: Boolean(document.getElementById("cfgWarmupClaudeEnabled") && document.getElementById("cfgWarmupClaudeEnabled").checked),
                        claude_model: (document.getElementById("cfgWarmupClaudeModelSelect") && document.getElementById("cfgWarmupClaudeModelSelect").value) || "claude-sonnet-5-5-high",
                        overrides: (dynamicConfig && dynamicConfig.warmup && dynamicConfig.warmup.overrides && typeof dynamicConfig.warmup.overrides === "object")
                            ? dynamicConfig.warmup.overrides
                            : undefined
                    },
                    prompt_guard: {
                        enabled: Boolean(document.getElementById("cfgPromptGuardEnabled") && document.getElementById("cfgPromptGuardEnabled").checked),
                        rules: clonePromptGuardRules(promptGuardRules)
                    },
                };

                dynamicConfig = await apiFetch(CONFIG_PATH, {
                    method: "POST",
                    body: JSON.stringify(reqBody)
                });
                renderDynamicConfigForm(dynamicConfig);
                await fetchScheduleConfig();
                await refreshDashboard();
                showToast(t("configSaveSuccess"), "success");
            } catch (err) {
                showToast(err.message, "error");
                updateSaveButtonState();
            }
        }

        async function resetDynamicConfigToDefaults() {
            var confirmed = await showThemedConfirm({
                title: t("confirmResetConfigTitle"),
                message: t("confirmResetConfigMsg"),
                confirmText: currentLang === "zh-CN" ? "确认恢复默认" : "Reset to Defaults",
                cancelText: currentLang === "zh-CN" ? "取消" : "Cancel",
                isDanger: false
            });
            if (!confirmed) return;

            var defaultCfg = {
                auto_apply: false,
                max_concurrency: 6,
                urgency_tolerance: 0.05,
                quota_sample_capacity: 6,
                rate_limit_cooldown_minutes: 5,
                schedule: {
                    paused: false
                },
                warmup: {
                    gemini_enabled: false,
                    gemini_model: "gemini-3-flash",
                    claude_enabled: false,
                    claude_model: "claude-sonnet-5-5-high"
                },
                prompt_guard: {
                    enabled: false,
                    rules: clonePromptGuardRules(DEFAULT_PROMPT_GUARD_RULES)
                },
            };
            renderDynamicConfigForm(defaultCfg);
            updateSaveButtonState();
            await saveDynamicConfig();
        }

`
