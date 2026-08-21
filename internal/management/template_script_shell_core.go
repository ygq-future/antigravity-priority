package management

const templateScriptShellCore = `        const LANG_STORAGE_KEY = "antigravity_priority_lang";
        let currentLang = "zh-CN";
        try {
            const savedLang = localStorage.getItem(LANG_STORAGE_KEY);
            if (savedLang === "zh-CN" || savedLang === "en-US") {
                currentLang = savedLang;
            }
        } catch (_) {}
        let latestSnapshot = null;
        let latestDiagnostics = null;
        let activeTab = "overview";
        let countdownInterval = null;
        let probeCooldownTimer = null;
        let scheduleConfig = null;
        let dynamicConfig = null;
        let originalConfigState = null;
        let userSelectedModelGroup = false;

        function getManagementKey() {
            try {
                const keys = ['management_key', 'management-key', 'managementKey', 'cpa_management_key', 'admin_key', 'key'];
                for (const k of keys) {
                    const v = localStorage.getItem(k) || sessionStorage.getItem(k);
                    if (v && v.trim()) return v.trim();
                }
            } catch (_) {}

            try {
                if (window.parent && window.parent !== window) {
                    const keys = ['management_key', 'management-key', 'managementKey', 'cpa_management_key', 'admin_key', 'key'];
                    for (const k of keys) {
                        const v = window.parent.localStorage.getItem(k) || window.parent.sessionStorage.getItem(k);
                        if (v && v.trim()) return v.trim();
                    }
                    const parentParams = new URLSearchParams(window.parent.location.search);
                    const pKey = parentParams.get('key') || parentParams.get('management_key') || parentParams.get('management-key');
                    if (pKey && pKey.trim()) return pKey.trim();
                }
            } catch (_) {}

            try {
                const params = new URLSearchParams(window.location.search);
                const qKey = params.get('key') || params.get('management_key') || params.get('management-key');
                if (qKey && qKey.trim()) return qKey.trim();
            } catch (_) {}

            return "";
        }

        function setSavedKey(key) {
            try {
                if (key) {
                    localStorage.setItem('management_key', key.trim());
                    sessionStorage.setItem('management_key', key.trim());
                } else {
                    localStorage.removeItem('management_key');
                    sessionStorage.removeItem('management_key');
                }
            } catch (_) {}
        }

        function openKeyModal() {
            const input = document.getElementById("manualKeyInput");
            if (input) input.value = getManagementKey();
            const modal = document.getElementById("keyModal");
            if (modal) modal.hidden = false;
        }

        function closeKeyModal() {
            const modal = document.getElementById("keyModal");
            if (modal) modal.hidden = true;
        }

        function saveKeyAndRefresh() {
            const input = document.getElementById("manualKeyInput");
            if (input) {
                setSavedKey(input.value);
            }
            closeKeyModal();
            refreshDashboard();
        }

        const THEME_STORAGE_KEY = "antigravity_priority_theme";

        function updateThemeIcon(theme) {
            const icon = document.getElementById("themeIcon");
            if (!icon) return;
            if (theme === "dark") {
                icon.textContent = "🌙";
            } else if (theme === "light") {
                icon.textContent = "☀️";
            } else {
                icon.textContent = "🌓";
            }
        }

        function toggleTheme() {
            const currentTheme = document.documentElement.getAttribute("data-theme");
            let nextTheme = "light";
            if (!currentTheme) {
                const isDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
                nextTheme = isDark ? "light" : "dark";
            } else {
                nextTheme = currentTheme === "dark" ? "light" : "dark";
            }
            document.documentElement.setAttribute("data-theme", nextTheme);
            try {
                localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
            } catch (_) {}
            updateThemeIcon(nextTheme);
        }

        function syncThemeFromParent() {
            try {
                if (window.parent && window.parent !== window && window.parent.document && window.parent.document.documentElement) {
                    const pDoc = window.parent.document.documentElement;
                    const pBody = window.parent.document.body;

                    const pTheme = pDoc.getAttribute("data-theme") || (pBody && pBody.getAttribute("data-theme"));
                    if (pTheme) {
                        document.documentElement.setAttribute("data-theme", pTheme);
                        updateThemeIcon(pTheme);
                    } else {
                        document.documentElement.removeAttribute("data-theme");
                        updateThemeIcon("system");
                    }

                    const isDark = pDoc.classList.contains("dark") || (pBody && pBody.classList.contains("dark")) || pTheme === "dark";
                    if (isDark) {
                        document.documentElement.setAttribute("data-theme", "dark");
                        updateThemeIcon("dark");
                    }

                    const parentStyle = window.parent.getComputedStyle(pDoc);
                    const cpaVarNames = [
                        '--bg-primary', '--bg-secondary', '--bg-tertiary', '--bg-quinary', '--bg-hover',
                        '--text-primary', '--text-secondary', '--text-tertiary', '--text-muted',
                        '--border-color', '--border-primary', '--border-secondary', '--border-hover',
                        '--primary-color', '--primary-hover', '--primary-active', '--primary-contrast',
                        '--success-color', '--warning-color', '--error-color', '--danger-color',
                        '--amber-color', '--quota-medium-color', '--floating-surface'
                    ];

                    for (const name of cpaVarNames) {
                        const val = parentStyle.getPropertyValue(name);
                        if (val && val.trim()) {
                            document.documentElement.style.setProperty(name, val.trim());
                        }
                    }

                    const sec = parentStyle.getPropertyValue('--bg-secondary') || parentStyle.getPropertyValue('--bg-primary');
                    const tert = parentStyle.getPropertyValue('--bg-tertiary');
                    if (sec && sec.trim()) {
                        document.documentElement.style.setProperty('--bg-surface', sec.trim());
                        document.documentElement.style.setProperty('--bg-card', sec.trim());
                    }
                    if (tert && tert.trim()) {
                        document.documentElement.style.setProperty('--bg-subtle', tert.trim());
                        document.documentElement.style.setProperty('--meter-bg', tert.trim());
                    }
                    return;
                }
            } catch (_) {}

            // Standalone or DevServer mode: restore saved theme
            try {
                const savedTheme = localStorage.getItem(THEME_STORAGE_KEY);
                if (savedTheme === "dark" || savedTheme === "light") {
                    document.documentElement.setAttribute("data-theme", savedTheme);
                    updateThemeIcon(savedTheme);
                } else {
                    const isDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
                    updateThemeIcon(isDark ? "dark" : "light");
                }
            } catch (_) {}
        }

        syncThemeFromParent();
        try {
            if (window.parent && window.parent !== window && window.parent.document) {
                const observer = new MutationObserver(function() {
                    syncThemeFromParent();
                });
                observer.observe(window.parent.document.documentElement, {
                    attributes: true,
                    attributeFilter: ["data-theme", "class", "style"]
                });
                if (window.parent.document.body) {
                    observer.observe(window.parent.document.body, {
                        attributes: true,
                        attributeFilter: ["data-theme", "class", "style"]
                    });
                }
            }
        } catch (_) {}

        function t(key) {
            return (I18N[currentLang] && I18N[currentLang][key]) || I18N["zh-CN"][key] || key;
        }

        function formatReason(reason, isBoosted, isDisabled) {
            if (!reason) {
                if (isDisabled) return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
                if (isBoosted) return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
                return currentLang === "zh-CN" ? "正常活跃" : "Active";
            }
            var lower = reason.toLowerCase();
            if (lower.indexOf("boost") >= 0) {
                return currentLang === "zh-CN" ? "🚀 优先提权" : "🚀 Boosted";
            }
            if (lower.indexOf("remaining positive") >= 0) {
                return currentLang === "zh-CN" ? "余量充足" : "Positive Balance";
            }
            if (lower.indexOf("weekly depleted") >= 0) {
                return currentLang === "zh-CN" ? "周额度耗尽" : "Weekly Depleted";
            }
            if (lower.indexOf("short") >= 0 && lower.indexOf("depleted") >= 0) {
                return currentLang === "zh-CN" ? "5h短窗口耗尽" : "5h Depleted";
            }
            if (lower.indexOf("429") >= 0 || lower.indexOf("cooldown") >= 0) {
                return currentLang === "zh-CN" ? "⏳ 429 冷却中" : "⏳ 429 Cooldown";
            }
            if (lower.indexOf("predicted") >= 0) {
                return currentLang === "zh-CN" ? "🔮 预测优先级" : "🔮 Predicted";
            }
            if (lower.indexOf("in sync") >= 0 || lower.indexOf("optimal") >= 0) {
                return currentLang === "zh-CN" ? "状态最优" : "In Sync";
            }
            if (lower.indexOf("disabled on host") >= 0) {
                return currentLang === "zh-CN" ? "已禁用" : "Disabled";
            }
            return reason;
        }

        function formatHistoryKind(kind) {
            var k = (kind || "").toLowerCase();
            if (k === "apply" || k === "auto_apply" || k === "manual_apply") {
                return currentLang === "zh-CN" ? "立即写回" : "APPLY";
            }
            if (k === "probe") {
                return currentLang === "zh-CN" ? "配额探测" : "PROBE";
            }
            if (k === "reset") {
                return currentLang === "zh-CN" ? "重置优先级" : "RESET";
            }
            return (kind || "RUN").toUpperCase();
        }

        function isCurrentTimeInScheduleWindow(startStr, endStr) {
            if (!startStr || !endStr) return true;
            var startParts = startStr.trim().split(":");
            var endParts = endStr.trim().split(":");
            if (startParts.length !== 2 || endParts.length !== 2) return true;
            var startMin = parseInt(startParts[0], 10) * 60 + parseInt(startParts[1], 10);
            var endMin = parseInt(endParts[0], 10) * 60 + parseInt(endParts[1], 10);
            if (isNaN(startMin) || isNaN(endMin) || startMin === endMin) return true;
            if (startMin === 0 && endMin === 24 * 60) return true;

            var now = new Date();
            var nowMin = now.getHours() * 60 + now.getMinutes();

            if (startMin < endMin) {
                return nowMin >= startMin && nowMin < endMin;
            }
            // Cross midnight (e.g. 22:00 to 06:00)
            return nowMin >= startMin || nowMin < endMin;
        }

        function toggleLanguage() {
            currentLang = currentLang === "zh-CN" ? "en-US" : "zh-CN";
            try {
                localStorage.setItem(LANG_STORAGE_KEY, currentLang);
            } catch (_) {}
            applyLanguage();
            renderDashboard();
            renderHistory();
            renderDiagnostics();
            renderScheduleStatus();
            if (dynamicConfig) renderDynamicConfigForm(dynamicConfig);
        }

        function applyLanguage() {
            document.documentElement.lang = currentLang;
            document.querySelectorAll("[data-i18n]").forEach(el => {
                const key = el.getAttribute("data-i18n");
                if (key && I18N[currentLang] && I18N[currentLang][key]) {
                    el.innerHTML = I18N[currentLang][key];
                }
            });
            const langLabel = document.getElementById("langLabel");
            if (langLabel) langLabel.textContent = currentLang === "zh-CN" ? "EN / 中文" : "中文 / EN";
            updateAllCustomSelectDisplays();
        }

`
