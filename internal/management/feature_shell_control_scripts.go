package management

const templateScriptControls = `        // Custom Select: shared open/close + viewport-aware placement.
        // A menu that would overflow the bottom of the viewport flips above its
        // trigger and is clamped to the space actually available.
        function toggleCustomSelectMenu(wrapperId, menuId) {
            const wrapper = document.getElementById(wrapperId);
            const menu = document.getElementById(menuId);
            if (!wrapper || !menu) return;
            const isOpen = !menu.hidden;
            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
            if (!isOpen) positionCustomSelectMenu(wrapper, menu);
        }

        function positionCustomSelectMenu(wrapper, menu) {
            wrapper.classList.remove("drop-up");
            menu.style.maxHeight = "";
            const trigger = wrapper.querySelector(".custom-select-trigger");
            if (!trigger) return;
            const rect = trigger.getBoundingClientRect();
            const boundary = customSelectBoundary(trigger);
            const edgeGap = 8;
            const spaceBelow = boundary.bottom - rect.bottom - edgeGap;
            const spaceAbove = rect.top - boundary.top - edgeGap;
            const naturalHeight = menu.scrollHeight;
            const dropUp = naturalHeight > spaceBelow && spaceAbove > spaceBelow;
            if (dropUp) wrapper.classList.add("drop-up");
            const available = Math.max(dropUp ? spaceAbove : spaceBelow, 48);
            if (naturalHeight > available) menu.style.maxHeight = available + "px";
        }

        // Nearest clipping rect: viewport intersected with every scroll container
        // above the trigger, so flipped menus never hide behind a panel edge.
        function customSelectBoundary(trigger) {
            const viewportHeight = window.innerHeight || document.documentElement.clientHeight;
            const boundary = { top: 0, bottom: viewportHeight };
            let node = trigger.parentElement;
            while (node && node !== document.body && node !== document.documentElement) {
                const overflowY = window.getComputedStyle(node).overflowY;
                if (overflowY !== "visible" && node.clientHeight > 0) {
                    const box = node.getBoundingClientRect();
                    boundary.top = Math.max(boundary.top, box.top);
                    boundary.bottom = Math.min(boundary.bottom, box.bottom);
                }
                node = node.parentElement;
            }
            return boundary;
        }

        function repositionOpenCustomSelects() {
            document.querySelectorAll(".custom-select-wrapper.open").forEach(wrapper => {
                const menu = wrapper.querySelector(".custom-select-options");
                if (menu && !menu.hidden) positionCustomSelectMenu(wrapper, menu);
            });
        }

        function handleCustomSelectViewportChange(event) {
            const target = event && event.target;
            if (target && target.nodeType === 1 && target.closest(".custom-select-options")) return;
            repositionOpenCustomSelects();
        }

        // Custom Select: Model Selector Dropdown
        function toggleCustomModelSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customModelSelect");
            toggleCustomSelectMenu("customModelSelect", "customModelSelectMenu");
        }

        function selectModel(modelId, event) {
            if (event) {
                event.stopPropagation();
            }
            const modelInput = document.getElementById("modelSelect");
            if (modelInput) modelInput.value = modelId;

            const menu = document.getElementById("customModelSelectMenu");
            if (menu && typeof menu.querySelectorAll === "function") {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === modelId;
                    opt.classList.toggle("selected", isSelected);
                    opt.setAttribute("aria-selected", isSelected ? "true" : "false");
                });
            }

            updateCustomModelSelectDisplay();
            closeAllCustomSelects();
            renderDashboard();
        }

        function updateCustomModelSelectDisplay() {
            const modelInput = document.getElementById("modelSelect");
            const modelId = (modelInput && modelInput.value) || "gemini-3.8-flash-high";
            const pool = resolveModelPool(modelId);
            const badge = document.getElementById("customModelSelectBadge");
            const label = document.getElementById("customModelSelectLabel");

            if (badge) {
                badge.className = "model-pool-badge " + (pool === "argon" ? "badge-argon" : (pool === "claude_gpt" ? "badge-claude" : "badge-gemini"));
                badge.textContent = pool === "argon" ? (t("badgeArgon") || "Argon") : (pool === "claude_gpt" ? (t("badgeClaudeGPT") || "Claude/GPT") : (t("badgeGemini") || "Gemini"));
            }
            if (label) {
                label.textContent = modelId;
            }
        }

        function updateModelDropdownFromSnapshot(models) {
            if (!Array.isArray(models) || models.length === 0) return;
            window.cpaAvailableModels = models;
            populateWarmupModelDropdowns(models);
            const menu = document.getElementById("customModelSelectMenu");
            if (!menu) return;

            const geminiModels = [];
            const argonModels = [];
            const claudeModels = [];
            models.forEach(m => {
                const p = m.quota_pool || resolveModelPool(m.id);
                if (p === "argon") argonModels.push(m);
                else if (p === "claude_gpt") claudeModels.push(m);
                else geminiModels.push(m);
            });

            const currentModel = (document.getElementById("modelSelect") && document.getElementById("modelSelect").value) || "gemini-3.8-flash-high";
            let html = "";

            function renderGroup(titleKey, fallbackTitle, groupModels, poolClass, poolBadgeText) {
                if (groupModels.length === 0) return;
                html += "<div class=\"model-select-group-header\" data-i18n=\"" + titleKey + "\">" + escapeHTML(t(titleKey) || fallbackTitle) + "</div>";
                groupModels.forEach(m => {
                    const isSelected = m.id === currentModel;
                    html += "<div class=\"custom-select-option" + (isSelected ? " selected" : "") + "\" data-value=\"" + escapeHTML(m.id) + "\" data-pool=\"" + escapeHTML(m.quota_pool || "") + "\" onclick=\"selectModel('" + escapeHTML(m.id) + "', event)\" role=\"option\" aria-selected=\"" + isSelected + "\">" +
                        "<span class=\"model-pool-badge " + poolClass + "\">" + escapeHTML(poolBadgeText) + "</span>" +
                        "<span class=\"model-option-name\">" + escapeHTML(m.display_name || m.id) + "</span>" +
                        "<span class=\"custom-select-check\"><svg width=\"14\" height=\"14\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.5\"><polyline points=\"20 6 9 17 4 12\"></polyline></svg></span>" +
                    "</div>";
                });
            }

            renderGroup("poolGemini", "Gemini 额度池", geminiModels, "badge-gemini", t("badgeGemini") || "Gemini");
            renderGroup("poolArgon", "Argon 额度池", argonModels, "badge-argon", t("badgeArgon") || "Argon");
            renderGroup("poolClaudeGPT", "Claude & GPT 额度池", claudeModels, "badge-claude", t("badgeClaudeGPT") || "Claude/GPT");

            menu.innerHTML = html;
        }
        function populateWarmupModelDropdowns(models) {
            if (!Array.isArray(models) || models.length === 0) return;
            window.cpaAvailableModels = models;

            const geminiModels = [];
            const claudeModels = [];
            models.forEach(m => {
                const p = m.quota_pool || resolveModelPool(m.id);
                if (p === "gemini") {
                    geminiModels.push(m);
                } else if (p === "claude_gpt") {
                    claudeModels.push(m);
                }
            });

            // 1. Gemini Warmup Select
            const gSelect = document.getElementById("cfgWarmupGeminiModelSelect");
            const gMenu = document.getElementById("customWarmupGeminiMenu");
            if (gSelect && gMenu && geminiModels.length > 0) {
                const candidateVal = (dynamicConfig && dynamicConfig.warmup && dynamicConfig.warmup.gemini_model) || gSelect.value || geminiModels[0].id;
                const hasVal = geminiModels.some(m => m.id === candidateVal);
                const currentVal = hasVal ? candidateVal : geminiModels[0].id;
                let selHtml = "";
                let menuHtml = "";
                geminiModels.forEach(m => {
                    const isSel = m.id === currentVal;
                    selHtml += "<option value=\"" + escapeHTML(m.id) + "\"" + (isSel ? " selected" : "") + ">" + escapeHTML(m.display_name || m.id) + "</option>";
                    menuHtml += "<div class=\"custom-select-option" + (isSel ? " selected" : "") + "\" data-value=\"" + escapeHTML(m.id) + "\" onclick=\"selectWarmupGeminiOption('" + escapeHTML(m.id) + "', event)\" role=\"option\" aria-selected=\"" + isSel + "\">" +
                        "<span>" + escapeHTML(m.display_name || m.id) + "</span>" +
                        "<span class=\"custom-select-check\"><svg width=\"14\" height=\"14\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.5\"><polyline points=\"20 6 9 17 4 12\"></polyline></svg></span>" +
                        "</div>";
                });
                gSelect.innerHTML = selHtml;
                gSelect.value = currentVal;
                gMenu.innerHTML = menuHtml;
                updateCustomWarmupGeminiDisplay();
            }

            // 2. Claude Warmup Select
            const cSelect = document.getElementById("cfgWarmupClaudeModelSelect");
            const cMenu = document.getElementById("customWarmupClaudeMenu");
            if (cSelect && cMenu && claudeModels.length > 0) {
                const candidateVal = (dynamicConfig && dynamicConfig.warmup && dynamicConfig.warmup.claude_model) || cSelect.value || claudeModels[0].id;
                const hasVal = claudeModels.some(m => m.id === candidateVal);
                const currentVal = hasVal ? candidateVal : claudeModels[0].id;
                let selHtml = "";
                let menuHtml = "";
                claudeModels.forEach(m => {
                    const isSel = m.id === currentVal;
                    selHtml += "<option value=\"" + escapeHTML(m.id) + "\"" + (isSel ? " selected" : "") + ">" + escapeHTML(m.display_name || m.id) + "</option>";
                    menuHtml += "<div class=\"custom-select-option" + (isSel ? " selected" : "") + "\" data-value=\"" + escapeHTML(m.id) + "\" onclick=\"selectWarmupClaudeOption('" + escapeHTML(m.id) + "', event)\" role=\"option\" aria-selected=\"" + isSel + "\">" +
                        "<span>" + escapeHTML(m.display_name || m.id) + "</span>" +
                        "<span class=\"custom-select-check\"><svg width=\"14\" height=\"14\" viewBox=\"0 0 24 24\" fill=\"none\" stroke=\"currentColor\" stroke-width=\"2.5\"><polyline points=\"20 6 9 17 4 12\"></polyline></svg></span>" +
                        "</div>";
                });
                cSelect.innerHTML = selHtml;
                cSelect.value = currentVal;
                cMenu.innerHTML = menuHtml;
                updateCustomWarmupClaudeDisplay();
            }
        }


        // Compatibility wrappers
        function toggleCustomSelect(event) {
            toggleCustomModelSelect(event);
        }
        function selectModelGroup(value, event) {
            let model = "gemini-3.8-flash-high";
            if (value === "argon") model = "gemini-4-argon-high";
            else if (value === "claude_gpt") model = "claude-sonnet-5-5-high";
            selectModel(model, event);
        }
        function updateCustomSelectDisplay() {
            updateCustomModelSelectDisplay();
        }
        // Custom Select: Config Interval
        function toggleCustomIntervalSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customIntervalSelect");
            toggleCustomSelectMenu("customIntervalSelect", "customIntervalMenu");
        }

        function selectIntervalOption(value, event) {
            if (event) event.stopPropagation();
            const select = document.getElementById("cfgIntervalSelect");
            if (select) select.value = value;

            const menu = document.getElementById("customIntervalMenu");
            if (menu) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === value;
                    opt.classList.toggle("selected", isSelected);
                });
            }

            updateCustomIntervalDisplay();
            onIntervalSelectChange();
            closeAllCustomSelects();
            updateSaveButtonState();
        }

        function updateCustomIntervalDisplay() {
            const select = document.getElementById("cfgIntervalSelect");
            const label = document.getElementById("customIntervalLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) {
                    const key = selectedOpt.getAttribute("data-i18n");
                    label.textContent = key ? t(key) : (selectedOpt.text || selectedOpt.value);
                }
            }
            const menu = document.getElementById("customIntervalMenu");
            if (menu && select) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === select.value;
                    opt.classList.toggle("selected", isSelected);
                });
            }
        }
        // Custom Select: Warmup Gemini Model
        function toggleCustomWarmupGeminiSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customWarmupGeminiModelSelect");
            toggleCustomSelectMenu("customWarmupGeminiModelSelect", "customWarmupGeminiMenu");
        }

        function selectWarmupGeminiOption(value, event) {
            if (event) event.stopPropagation();
            const select = document.getElementById("cfgWarmupGeminiModelSelect");
            if (select) select.value = value;
            updateCustomWarmupGeminiDisplay();
            closeAllCustomSelects();
            updateSaveButtonState();
        }

        function updateCustomWarmupGeminiDisplay() {
            const select = document.getElementById("cfgWarmupGeminiModelSelect");
            const label = document.getElementById("customWarmupGeminiLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) label.textContent = selectedOpt.text || selectedOpt.value;
            }
            const menu = document.getElementById("customWarmupGeminiMenu");
            if (menu && select) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === select.value;
                    opt.classList.toggle("selected", isSelected);
                });
            }
        }

        // Custom Select: Warmup Claude Model
        function toggleCustomWarmupClaudeSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customWarmupClaudeModelSelect");
            toggleCustomSelectMenu("customWarmupClaudeModelSelect", "customWarmupClaudeMenu");
        }

        function selectWarmupClaudeOption(value, event) {
            if (event) event.stopPropagation();
            const select = document.getElementById("cfgWarmupClaudeModelSelect");
            if (select) select.value = value;
            updateCustomWarmupClaudeDisplay();
            closeAllCustomSelects();
            updateSaveButtonState();
        }

        function updateCustomWarmupClaudeDisplay() {
            const select = document.getElementById("cfgWarmupClaudeModelSelect");
            const label = document.getElementById("customWarmupClaudeLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) label.textContent = selectedOpt.text || selectedOpt.value;
            }
            const menu = document.getElementById("customWarmupClaudeMenu");
            if (menu && select) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === select.value;
                    opt.classList.toggle("selected", isSelected);
                });
            }
        }


        function updateAllCustomSelectDisplays() {
            updateCustomSelectDisplay();
            updateCustomIntervalDisplay();
            updateCustomWarmupGeminiDisplay();
            updateCustomWarmupClaudeDisplay();
        }

        function closeAllCustomSelects(exceptId) {
            const list = [
                { wrap: "customModelSelect", menu: "customModelSelectMenu" },
                { wrap: "customIntervalSelect", menu: "customIntervalMenu" },
                { wrap: "customWarmupGeminiModelSelect", menu: "customWarmupGeminiMenu" },
                { wrap: "customWarmupClaudeModelSelect", menu: "customWarmupClaudeMenu" },
            ];
            list.forEach(item => {
                if (item.wrap !== exceptId) {
                    const w = document.getElementById(item.wrap);
                    const m = document.getElementById(item.menu);
                    if (w && m) {
                        m.hidden = true;
                        w.classList.remove("open");
                    }
                }
            });
        }

        document.addEventListener("click", () => {
            closeAllCustomSelects();
        });
        window.addEventListener("resize", handleCustomSelectViewportChange);
        document.addEventListener("scroll", handleCustomSelectViewportChange, true);

        function switchTab(tabId) {
            activeTab = tabId;
            document.querySelectorAll(".tab").forEach(tab => {
                tab.classList.toggle("active", tab.dataset.tab === tabId);
            });
            document.getElementById("panelOverview").hidden = tabId !== "overview";
            document.getElementById("panelHistory").hidden = tabId !== "history";
            document.getElementById("panelDiagnostics").hidden = tabId !== "diagnostics";
            document.getElementById("panelConfig").hidden = tabId !== "config";
            document.getElementById("panelHelp").hidden = tabId !== "help";
            if (isAuthBlocked) return;
            if (tabId === "history") fetchDiagnostics();
            if (tabId === "diagnostics") fetchDiagnostics();
            if (tabId === "config") fetchDynamicConfig();
            if (tabId === "overview") refreshDashboard({ withSync: true, silent: true });
        }

        function getAuthHeader() {
            const headers = { "Content-Type": "application/json" };
            const key = getManagementKey();
            if (key) {
                headers["Authorization"] = "Bearer " + key;
                headers["X-Management-Key"] = key;
            }
            return headers;
        }

        async function apiFetch(path, options) {
            if (isAuthBlocked) {
                openKeyModal();
                throw new Error(currentLang === "zh-CN" ? "需要 CPA 管理密钥进行认证 (401 Unauthorized)" : "Management Key required (401 Unauthorized)");
            }
            const resp = await fetch(path, {
                ...(options || {}),
                headers: { ...getAuthHeader(), ...((options && options.headers) || {}) }
            });
            if (resp.status === 401) {
                isAuthBlocked = true;
                if (dashboardRefreshInterval) {
                    clearInterval(dashboardRefreshInterval);
                    dashboardRefreshInterval = null;
                }
                openKeyModal();
                throw new Error(currentLang === "zh-CN" ? "需要 CPA 管理密钥进行认证 (401 Unauthorized)" : "Management Key required (401 Unauthorized)");
            }
            if (resp.status === 403) {
                const text = await resp.text();
                if (text.indexOf("banned") >= 0 || text.indexOf("IP") >= 0) {
                    if (dashboardRefreshInterval) {
                        clearInterval(dashboardRefreshInterval);
                        dashboardRefreshInterval = null;
                    }
                }
                throw new Error(text || resp.statusText);
            }
            isAuthBlocked = false;
            const text = await resp.text();
            if (!resp.ok) {
                let errMessage = text || resp.statusText;
                try {
                    const parsed = JSON.parse(text);
                    if (parsed.error) errMessage = parsed.error;
                } catch (_) {}
                throw new Error(errMessage);
            }
            return text ? JSON.parse(text) : {};
        }

`
