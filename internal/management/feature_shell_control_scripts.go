package management

const templateScriptControls = `        // Custom Select: Main Model Group
        function toggleCustomSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customModelGroupSelect");
            const wrapper = document.getElementById("customModelGroupSelect");
            const menu = document.getElementById("customSelectMenu");
            if (!wrapper || !menu) return;
            const isOpen = !menu.hidden;
            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
        }

        function selectModelGroup(value, event) {
            if (event) {
                event.stopPropagation();
                userSelectedModelGroup = true;
            }
            const select = document.getElementById("modelGroupSelect");
            if (select) select.value = value;

            const menu = document.getElementById("customSelectMenu");
            if (menu) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === value;
                    opt.classList.toggle("selected", isSelected);
                });
            }

            updateCustomSelectDisplay();
            closeAllCustomSelects();
            renderDashboard();
        }

        function updateCustomSelectDisplay() {
            const select = document.getElementById("modelGroupSelect");
            const label = document.getElementById("customSelectLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) {
                    const key = selectedOpt.getAttribute("data-i18n");
                    label.textContent = key ? t(key) : selectedOpt.textContent;
                }
            }
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

        // Custom Select: Config Model Group
        function toggleCustomCfgModelSelect(event) {
            if (event) event.stopPropagation();
            closeAllCustomSelects("customCfgModelGroupSelect");
            const wrapper = document.getElementById("customCfgModelGroupSelect");
            const menu = document.getElementById("customCfgModelMenu");
            if (!wrapper || !menu) return;
            const isOpen = !menu.hidden;
            menu.hidden = isOpen;
            wrapper.classList.toggle("open", !isOpen);
        }

        function selectCfgModelOption(value, event) {
            if (event) event.stopPropagation();
            const select = document.getElementById("cfgModelGroup");
            if (select) select.value = value;

            const menu = document.getElementById("customCfgModelMenu");
            if (menu) {
                menu.querySelectorAll(".custom-select-option").forEach(opt => {
                    const isSelected = opt.getAttribute("data-value") === value;
                    opt.classList.toggle("selected", isSelected);
                });
            }

            updateCustomCfgModelDisplay();
            closeAllCustomSelects();
            updateSaveButtonState();
        }

        function updateCustomCfgModelDisplay() {
            const select = document.getElementById("cfgModelGroup");
            const label = document.getElementById("customCfgModelLabel");
            if (select && label) {
                const selectedOpt = select.options[select.selectedIndex];
                if (selectedOpt) {
                    const key = selectedOpt.getAttribute("data-i18n");
                    label.textContent = key ? t(key) : (selectedOpt.text || selectedOpt.value);
                }
            }
            const menu = document.getElementById("customCfgModelMenu");
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
            updateCustomCfgModelDisplay();
        }

        function closeAllCustomSelects(exceptId) {
            const list = [
                { wrap: "customModelGroupSelect", menu: "customSelectMenu" },
                { wrap: "customIntervalSelect", menu: "customIntervalMenu" },
                { wrap: "customCfgModelGroupSelect", menu: "customCfgModelMenu" }
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
