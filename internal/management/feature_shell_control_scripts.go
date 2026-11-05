package management

const templateScriptControls = `        let userSelectedModel = false;


        // Custom Select: Model Selector Dropdown
        function toggleCustomModelSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customModelSelect");
            const wrapper = document.getElementById("customModelSelect");
            const menu = document.getElementById("customModelSelectMenu");
            if (!wrapper || !menu) return;
            const isOpen = !menu.hidden;
            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
        }

        function selectModel(modelId, event) {
            if (event) {
                event.stopPropagation();
                userSelectedModel = true;
                userSelectedModelGroup = true;
            }
            const modelInput = document.getElementById("modelSelect");
            if (modelInput) modelInput.value = modelId;

            const pool = resolveModelPool(modelId);
            const groupSelect = document.getElementById("modelGroupSelect");
            if (groupSelect) groupSelect.value = pool;

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
            const wrapper = document.getElementById("customIntervalSelect");
            const menu = document.getElementById("customIntervalMenu");
            if (!wrapper || !menu) return;
            const isOpen = !menu.hidden;
            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
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

        function updateAllCustomSelectDisplays() {
            updateCustomSelectDisplay();
            updateCustomIntervalDisplay();
        }

        function closeAllCustomSelects(exceptId) {
            const list = [
                { wrap: "customModelSelect", menu: "customModelSelectMenu" },
                { wrap: "customModelGroupSelect", menu: "customSelectMenu" },
                { wrap: "customIntervalSelect", menu: "customIntervalMenu" },
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
