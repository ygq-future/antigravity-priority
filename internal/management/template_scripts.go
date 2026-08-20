package management

// TemplateScripts contains all client-side JavaScript for the dashboard.
const TemplateScripts = `
    <script>
        const BASE_PATH = "/v0/management/plugins/antigravity-priority";
        const SNAPSHOT_PATH = BASE_PATH + "/snapshot/latest";
        const DIAGNOSTICS_PATH = BASE_PATH + "/diagnostics";
        const RUN_PATH = BASE_PATH + "/run";
        const RESET_PATH = BASE_PATH + "/reset";
        const SCHEDULE_CONFIG_PATH = BASE_PATH + "/schedule/config";
        const CONFIG_PATH = BASE_PATH + "/config";

        const I18N = {
            "zh-CN": {
                title: "Antigravity 优先级管理",
                tabOverview: "概览与仪表盘",
                tabHistory: "执行历史",
                tabDiagnostics: "系统诊断",
                tabConfig: "⚙️ 配置中心",
                tabHelp: "使用帮助",
                kpiTotal: "总凭证数",
                kpiBoosted: "🚀 动态 Boost",
                kpiBoostedDesc: "高配额充裕候选",
                kpiDepleted: "耗尽 / 降级",
                kpiDepletedDesc: "等待窗口重置",
                kpiLastAudit: "最新调度审计",
                labelModelGroup: "模型组：",
                optGemini: "Gemini 模型",
                optClaudeGPT: "Claude 与 GPT 模型",
                btnRefresh: "刷新",
                btnKey: "密钥",
                btnDryRun: "试运行",
                btnApply: "立即写回",
                btnReset: "重置优先级",
                btnProbe: "刷新配额",
                confirmReset: "确定要将所有 Antigravity 凭证的优先级重置为默认未设置状态吗？",
                resetSuccess: "所有凭证优先级已重置为默认未设置状态",
                loading: "正在加载凭证与配额状态...",
                noCreds: "未发现 Antigravity 凭证",
                historyTitle: "最近执行记录 (最近 10 次)",
                noHistory: "暂无执行历史记录",
                diagnosticsTitle: "系统运行诊断与调度状态",
                helpTitle: "Antigravity Priority 原理与核心特色说明",
                helpP1: "本插件专为 CLIProxyAPI 的 Google Antigravity 设计，实现双窗口自适应优先级调度与智能写回：",
                help5hTitle: "⚡ 5小时短窗口 (5h) 自愈降级",
                help5hDesc: "实时监控高频请求配额与重置倒计时。短窗耗尽仅执行软降级（priority=-1, disabled=false），短窗刷新后自动恢复调度，有效防止触发 429 速率限制。",
                help7dTitle: "📅 7天长窗口 (7d) 全局把控",
                help7dDesc: "跟踪周配额剩余量及重置进度。周额度耗尽执行硬禁用（disabled=true），避免单张凭证在周期前半段过早耗尽。",
                helpBurnTitle: "📈 自适应燃尽学习 (C_cycle)",
                helpBurnDesc: "基于连续探测增量自动学习推算周期消耗能力并通过 EMA 平滑，杜绝人工手动配置负担，实时计算保底所需剩余额度。",
                helpUrgencyTitle: "⚖️ 配额燃尽紧迫度 (Weekly Urgency) 与平滑轮换",
                helpUrgencyDesc: "量化单周期内的额度使用压力，配合容差分档算法，将紧迫度相近的账号自动赋予相同优先级实现轮询均衡。",
                helpBoostTitle: "🚀 动态提前提权 (Dynamic Boost Horizon)",
                helpBoostDesc: "对双窗口余量充裕且周重置临近的凭证自动赋予第一梯队超高优先级（900-999），彻底消除大额度账号撑死溢出与浪费痛点。",
                help429Title: "⏳ 429 熔断冷却与自动自愈",
                help429Desc: "遭遇 Google 429 速率限制时自动临时降级为 -1，冷却期结束后在下次调度中自动探测自愈，无需人工介入。",
                btnClose: "关闭",
                btnApplyNow: "立即写回",
                previewTitle: "试运行计划预览",
                applyTitle: "写回执行结果",
                noChanges: "本次调度无优先级或禁用状态变化",
                statusActive: "正常活跃",
                statusBoosted: "🚀 动态 Boost",
                statusWeeklyDepleted: "周额度耗尽",
                statusShortDepleted: "5h短窗口耗尽",
                statusFailed: "探测失败",
                shortWindow: "5h 短窗口",
                longWindow: "7d 周窗口",
                resetIn: "重置倒计时",
                priority: "优先级",
                urgencyLabel: "紧迫度",
                burnLabel: "燃烧率",
                running: "执行中...",
                keyModalTitle: "CPA 管理密钥认证",
                keyModalDesc: "CPA 原生管理界面下请输入 config.yaml 中的 Management Key；若在 CPA-Plus 等增强面板中，请输入 CPA-Plus 登录密码（通常为 cpamp_... 格式）。",
                btnSaveKey: "保存并验证",
                btnCopy: "复制 JSON",
                copied: "✅ 已复制",
                scheduleActive: "自动调度运行中",
                schedulePaused: "自动调度已暂停",
                scheduleSleeping: "调度休眠中 (不在运行时间)",
                scheduleDisabled: "自动调度已关闭",
                confirmResetTitle: "⚠️ 确认重置凭证优先级",
                confirmResetMessage: "该操作将清除所有 Antigravity 凭证的自定义 priority 字段，恢复为宿主默认未分配状态。",
                confirmApplyTitle: "⚡ 确认立即写回优先级",
                confirmApplyMsg: "该操作将基于当前配额与健康度决策结果，立即更新 CPA 宿主中的凭证优先级与启用/禁用状态。是否继续？",
                predictedBadge: "🔮 预测",
                predictedNote: "当前查看预测视图，非主控组数据",
                probeSuccess: "配额探测完成",
                probeCooldown: "冷却中...",
                viewDetails: "🔍 查看明细",
                cfgCardScheduleTitle: "自动调度与时段配置",
                cfgAutoApply: "自动定时调度",
                cfgAutoApplyHint: "周期性自动探测并智能更新宿主凭证优先级",
                cfgInterval: "调度执行周期",
                cfgIntervalHint: "自动调度与探测的执行间隔（如 15m, 30m, 1h）",
                cfgModelGroup: "配额主控模型组",
                cfgModelGroupHint: "作为主要写回依据的模型组",
                cfgScheduleWindow: "生效时间区间",
                cfgScheduleWindowHint: "仅在每日指定时间段内执行调度（支持跨午夜，如 22:00-06:00）",
                cfgWindowEnabledLabel: "仅在指定时段运行 (非此时段休眠)",
                cfgCardPerfTitle: "探测与写回性能",
                cfgMaxConcurrency: "最大探测并发数",
                cfgMaxConcurrencyHint: "向 Google API 并发探测的最大协程数 (1-32)",
                cfgMinChange: "优先级变动写入阈值",
                cfgMinChangeHint: "优先级变化绝对值达到该阈值才写入宿主 (0-100)",
                cfgUrgencyTolerance: "紧迫度分档容差",
                cfgUrgencyToleranceHint: "紧迫度差距在此容差内分配相同优先级 (0.00-0.50)",
                cfgSampleCapacity: "自适应时序样本容量",
                cfgSampleCapacityHint: "滑动窗口保留的历史探测样本数，用于平滑估计燃尽率 (2-30)",
                cfgCooldownMinutes: "429 熔断冷却时长 (分钟)",
                cfgCooldownMinutesHint: "遭遇 429 限流时临时降级至 -1 的冷却期 (1-1440)",
                statusCooldown: "⏳ 429 冷却中",
                cfgCardRulesTitle: "优先级分值规则 (Priority Rules)",
                cfgRulesEnabled: "启用双窗口优先级规则",
                cfgRulesEnabledHint: "基于 5h 短窗口与 7d 长窗口配额综合决策",
                cfgBoostStart: "🚀 动态 Boost 起始优先级",
                cfgBoostStartHint: "充裕且即将重置的第一梯队凭证起始优先级 (1-999)",
                cfgNormalStart: "常规健康凭证起始优先级",
                cfgNormalStartHint: "常规可用梯队凭证的起始基准优先级 (1-999)",
                btnSaveConfig: "保存并立即生效",
                btnResetToDefaults: "恢复默认配置",
                configSaveSuccess: "配置已更新并立即生效",
                confirmResetConfigTitle: "⚠️ 确认恢复默认配置",
                confirmResetConfigMsg: "确定要将所有运行时配置恢复为官方推荐默认值吗？",
                optInterval5m: "5 分钟",
                optInterval15m: "15 分钟 (推荐)",
                optInterval30m: "30 分钟",
                optInterval1h: "1 小时",
                optIntervalCustom: "自定义",
                valErrInterval: "自定义调度周期格式不正确，需如 15m, 1h",
                valErrWindow: "生效时间格式不正确，需为 HH:mm 格式 (如 09:00, 23:00)",
                valErrConcurrency: "最大并发数超出范围，需在 1 到 32 之间",
                valErrMinChange: "变动写入阈值超出范围，需在 0 到 100 之间",
                valErrUrgencyTol: "紧迫度容差超出范围，需在 0.00 到 0.50 之间 (最多两位小数)",
                valErrSampleCapacity: "自适应样本容量超出范围，需在 2 到 30 之间",
                valErrCooldown: "429 冷却时长超出范围，需在 1 到 1440 分钟之间",
                valErrPriorityRange: "优先级分值超出范围，需在 1 到 999 之间",
                valErrPriorityOrder: "常规起始优先级不能大于 Boost 起始优先级"
            },
            "en-US": {
                title: "Antigravity Priority",
                tabOverview: "Overview & Meters",
                tabHistory: "Run History",
                tabDiagnostics: "Diagnostics",
                tabConfig: "⚙️ Config Center",
                tabHelp: "Help & Features",
                kpiTotal: "Total Credentials",
                kpiBoosted: "🚀 Boosted",
                kpiBoostedDesc: "High quota abundance",
                kpiDepleted: "Depleted / Down",
                kpiDepletedDesc: "Awaiting window reset",
                kpiLastAudit: "Latest Audit",
                labelModelGroup: "Model Group:",
                optGemini: "Gemini Models",
                optClaudeGPT: "Claude & GPT Models",
                btnRefresh: "Refresh",
                btnKey: "Key",
                btnDryRun: "Dry-Run",
                btnApply: "Apply",
                btnReset: "Reset Priority",
                btnProbe: "Fetch Quota",
                confirmReset: "Are you sure you want to reset all Antigravity credential priorities to default unset state?",
                resetSuccess: "All credential priorities have been reset to default unset state",
                loading: "Loading credentials & quota...",
                noCreds: "No Antigravity credentials found",
                historyTitle: "Execution History (Last 10)",
                noHistory: "No execution history yet",
                diagnosticsTitle: "System Diagnostics & Scheduler",
                helpTitle: "Antigravity Priority Mechanics & Features",
                helpP1: "Tailored for Google Antigravity in CLIProxyAPI with double-window adaptive scheduling and intelligent write-back:",
                help5hTitle: "⚡ 5-Hour Short Window (5h) Soft Demote",
                help5hDesc: "Monitors burst quota and reset countdowns. Short window depletion triggers soft demotion (priority=-1, disabled=false) with auto recovery on reset, avoiding 429 rate limits.",
                help7dTitle: "📅 7-Day Weekly Window (7d) Global Control",
                help7dDesc: "Tracks weekly remaining balance and reset progress. Weekly depletion triggers hard disable (disabled=true) to prevent early exhaustion.",
                helpBurnTitle: "📈 Adaptive Burn Rate Learning (C_cycle)",
                helpBurnDesc: "Incrementally learns actual consumption capability via probe deltas and EMA smoothing, computing required safe reserves automatically.",
                helpUrgencyTitle: "⚖️ Weekly Urgency & Load Balancing",
                helpUrgencyDesc: "Quantifies unit-time quota pressure and clusters accounts within tolerance into equal priority tiers for round-robin rotation.",
                helpBoostTitle: "🚀 Dynamic Boost Horizon",
                helpBoostDesc: "Elevates credentials with abundant balance nearing reset into top priority tier (900-999), preventing massive quota waste.",
                help429Title: "⏳ 429 Cooldown & Self-Healing",
                help429Desc: "Temporarily demotes credentials to -1 on 429 rate limit errors, automatically probing and restoring them after the cooldown expires.",
                btnClose: "Close",
                btnApplyNow: "Apply",
                previewTitle: "Dry-Run Plan Preview",
                applyTitle: "Apply Execution Result",
                noChanges: "No priority or status changes required",
                statusActive: "Active",
                statusBoosted: "🚀 Boosted",
                statusWeeklyDepleted: "Weekly Depleted",
                statusShortDepleted: "5h Depleted",
                statusFailed: "Probe Failed",
                shortWindow: "5h Window",
                longWindow: "7d Window",
                resetIn: "Resets in",
                priority: "Priority",
                urgencyLabel: "Urgency",
                burnLabel: "Burn Rate",
                running: "Running...",
                keyModalTitle: "CPA Management Key",
                keyModalDesc: "For native CPA Web UI, enter the Management Key from config.yaml; for CPA-Plus enhanced panels, enter your CPA-Plus login password (e.g. cpamp_... format).",
                btnSaveKey: "Save & Verify",
                btnCopy: "Copy JSON",
                copied: "✅ Copied",
                scheduleActive: "Auto Schedule: Running",
                schedulePaused: "Auto Schedule: Paused",
                scheduleSleeping: "Schedule: Sleeping (Outside Window)",
                scheduleDisabled: "Auto Schedule: Disabled",
                confirmResetTitle: "⚠️ Confirm Priority Reset",
                confirmResetMessage: "This will clear all Antigravity credential custom priority fields, restoring them to host default unset state.",
                confirmApplyTitle: "⚡ Confirm Immediate Apply",
                confirmApplyMsg: "This will immediately update credential priorities and active status in the CPA host based on current quota calculations. Proceed?",
                predictedBadge: "🔮 Predicted",
                predictedNote: "Viewing predicted priorities, not the active group",
                probeSuccess: "Quota probe complete",
                probeCooldown: "Cooling down...",
                viewDetails: "🔍 Details",
                cfgCardScheduleTitle: "Auto Scheduling & Time Window",
                cfgAutoApply: "Auto Periodic Scheduling",
                cfgAutoApplyHint: "Periodically probe and update host credential priorities",
                cfgInterval: "Scheduling Interval",
                cfgIntervalHint: "Execution interval for probing and scheduling (e.g. 15m, 30m, 1h)",
                cfgModelGroup: "Primary Model Group",
                cfgModelGroupHint: "Model group used as the primary write-back basis",
                cfgScheduleWindow: "Active Schedule Window",
                cfgScheduleWindowHint: "Only run scheduling within daily time window (supports cross-midnight, e.g. 22:00-06:00)",
                cfgWindowEnabledLabel: "Only run in specified window (sleep otherwise)",
                cfgCardPerfTitle: "Probing & Performance",
                cfgMaxConcurrency: "Max Probe Concurrency",
                cfgMaxConcurrencyHint: "Maximum concurrent goroutines for Google API probes (1-32)",
                cfgMinChange: "Priority Min Change Threshold",
                cfgMinChangeHint: "Minimum priority delta required to write back to host",
                cfgUrgencyTolerance: "Urgency Bucket Tolerance",
                cfgUrgencyToleranceHint: "Credentials within this tolerance share the same priority (0.00-0.50)",
                cfgSampleCapacity: "Adaptive Sample Capacity",
                cfgSampleCapacityHint: "Sliding window sample count for multi-sample burn rate estimation (2-30)",
                cfgCooldownMinutes: "429 Cooldown Duration (Min)",
                cfgCooldownMinutesHint: "Cooldown period demoting account to -1 on 429 errors (1-1440)",
                statusCooldown: "⏳ 429 Cooldown",
                cfgCardRulesTitle: "Priority Scoring Rules",
                cfgRulesEnabled: "Enable Double-Window Priority Rules",
                cfgRulesEnabledHint: "Decide priority based on 5h short and 7d long window quotas",
                cfgBoostStart: "🚀 Dynamic Boost Start Priority",
                cfgBoostStartHint: "Base priority for top-tier abundant credentials (1-999)",
                cfgNormalStart: "Normal Healthy Start Priority",
                cfgNormalStartHint: "Base priority for regular healthy credentials (1-999)",
                btnSaveConfig: "Save & Apply",
                btnResetToDefaults: "Reset to Defaults",
                configSaveSuccess: "Configuration updated and applied immediately",
                confirmResetConfigTitle: "⚠️ Confirm Reset to Defaults",
                confirmResetConfigMsg: "Are you sure you want to reset all runtime configurations to standard defaults?",
                optInterval5m: "5 Minutes",
                optInterval15m: "15 Minutes (Recommended)",
                optInterval30m: "30 Minutes",
                optInterval1h: "1 Hour",
                optIntervalCustom: "Custom",
                valErrInterval: "Invalid custom interval format (e.g. 15m, 1h)",
                valErrWindow: "Invalid active time format (must be HH:mm, e.g. 09:00, 23:00)",
                valErrConcurrency: "Max concurrency out of range (1 - 32)",
                valErrMinChange: "Min change threshold out of range (0 - 100)",
                valErrUrgencyTol: "Urgency tolerance out of range (0.00 - 0.50, max 2 decimals)",
                valErrSampleCapacity: "Sample capacity out of range (2 - 30)",
                valErrCooldown: "429 cooldown minutes out of range (1 - 1440)",
                valErrPriorityRange: "Priority out of range (1 - 999)",
                valErrPriorityOrder: "Normal start priority cannot be greater than Boost start priority"
            }
        };

        const LANG_STORAGE_KEY = "antigravity_priority_lang";
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
            return reason;
        }

        function formatHistoryKind(kind) {
            var k = (kind || "").toLowerCase();
            if (k === "apply" || k === "auto_apply" || k === "manual_apply") {
                return currentLang === "zh-CN" ? "立即写回" : "APPLY";
            }
            if (k === "dry_run" || k === "dry-run") {
                return currentLang === "zh-CN" ? "试运行" : "DRY-RUN";
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

        // Custom Select: Main Model Group
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
            if (event) event.stopPropagation();
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
                    label.textContent = key ? t(key) : selectedOpt.textContent;
                }
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
                    label.textContent = key ? t(key) : selectedOpt.textContent;
                }
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
            if (tabId === "history") fetchDiagnostics();
            if (tabId === "diagnostics") fetchDiagnostics();
            if (tabId === "config") fetchDynamicConfig();
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
            const resp = await fetch(path, {
                ...(options || {}),
                headers: { ...getAuthHeader(), ...((options && options.headers) || {}) }
            });
            if (resp.status === 401) {
                openKeyModal();
                throw new Error(currentLang === "zh-CN" ? "需要 CPA 管理密钥进行认证 (401 Unauthorized)" : "Management Key required (401 Unauthorized)");
            }
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

        async function fetchSnapshot() {
            try {
                const data = await apiFetch(SNAPSHOT_PATH);
                latestSnapshot = data;
                renderDashboard();
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        async function fetchDiagnostics() {
            try {
                const data = await apiFetch(DIAGNOSTICS_PATH);
                latestDiagnostics = data;
                renderDashboard();
                renderHistory();
                renderDiagnostics();
                renderScheduleStatus();
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        async function refreshDashboard() {
            const btn = document.getElementById("btnRefresh");
            if (btn) btn.disabled = true;
            try {
                await Promise.all([fetchSnapshot(), fetchDiagnostics()]);
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        async function triggerApplyWithConfirm() {
            var confirmed = await showThemedConfirm({
                title: t("confirmApplyTitle"),
                message: t("confirmApplyMsg"),
                confirmText: currentLang === "zh-CN" ? "确认写回" : "Apply",
                cancelText: currentLang === "zh-CN" ? "取消" : "Cancel",
                isDanger: false
            });
            if (!confirmed) return;
            triggerRun("apply");
        }

        async function triggerRun(mode) {
            if (mode === "apply") {
                const groupSelect = document.getElementById("modelGroupSelect");
                const selectedGroup = groupSelect ? groupSelect.value : "gemini";
                const activeGroup = (latestSnapshot && latestSnapshot.active_model_group) || "gemini";
                if (selectedGroup !== activeGroup) {
                    showToast(currentLang === "zh-CN" ? "仅主控组可执行写回，当前主控组为 " + activeGroup : "Only active group can apply. Active: " + activeGroup, "info");
                    return;
                }
                const groupData = (latestSnapshot && latestSnapshot.groups && latestSnapshot.groups[selectedGroup]) || {};
                const items = groupData.items || [];
                if (items.length === 0) {
                    showToast(currentLang === "zh-CN" ? "未发现有效凭证，无需执行写回" : "No credentials found to apply", "info");
                    return;
                }
                const changes = groupData.changes || [];
                if (changes.length === 0) {
                    showToast(currentLang === "zh-CN" ? "当前凭证状态与优先级已是最优，无需写回" : "All credentials in sync, no changes needed", "info");
                    return;
                }
            }

            const groupSelect = document.getElementById("modelGroupSelect");
            const group = groupSelect ? groupSelect.value : "gemini";
            const btnDry = document.getElementById("btnDryRun");
            const btnApp = document.getElementById("btnApply");

            if (btnDry) btnDry.disabled = true;
            if (btnApp) btnApp.disabled = true;

            try {
                const path = RUN_PATH + "?mode=" + encodeURIComponent(mode) + "&antigravity_model_group=" + encodeURIComponent(group);
                const result = await apiFetch(path, { method: "POST" });
                const items = (result && result.snapshot && result.snapshot.items) || (result && result.items) || [];
                if (items.length === 0) {
                    showToast(currentLang === "zh-CN" ? "未发现有效 Antigravity 凭证" : "No Antigravity credentials found", "info");
                } else {
                    showModal(mode, result, false);
                }
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btnDry) btnDry.disabled = false;
                if (btnApp) btnApp.disabled = false;
            }
        }

        // Show Modal: Distinguish Dry-Run (Quota & Metrics diff) vs Apply (Priority & Disabled diff)
        function showModal(mode, result, isFromHistory) {
            const modal = document.getElementById("diffModal");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

            title.textContent = mode === "dry-run" ? t("previewTitle") : t("applyTitle");
            const changes = (result && result.changes) || (result && result.snapshot && result.snapshot.changes) || [];
            const items = (result && result.snapshot && result.snapshot.items) || [];

            // Only live dry-run offers the "Apply" button; history view is strictly read-only
            btnApply.hidden = mode === "apply" || isFromHistory === true || changes.length === 0;

            summary.textContent = "Attempted: " + (result.attempted || changes.length) + ", Succeeded: " + (result.succeeded || 0) + ", Failed: " + (result.failed || 0) + ", Skipped: " + (result.skipped || 0);

            list.innerHTML = "";
            if (changes.length === 0 && items.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noChanges") + "</div>";
            } else if (mode === "dry-run") {
                // Dry-Run Mode: Show Detailed Metric Changes & Predictions
                var displayList = items.length > 0 ? items : changes;
                displayList.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";

                    const name = c.name || c.auth_index || "Credential";
                    const isBoost = Boolean(c.is_boosted || (c.reason && c.reason.indexOf("boost") >= 0));
                    const reasonText = formatReason(c.reason, isBoost, c.target && c.target.disabled);

                    let r5hText = c.r5h !== undefined ? Math.round(c.r5h * 100) + "%" : "-";
                    let r7dText = c.r7d !== undefined ? Math.round(c.r7d * 100) + "%" : "-";
                    let urgText = c.urgency !== undefined ? c.urgency.toFixed(2) : "-";
                    let burnText = c.cycle_burn_rate !== undefined ? c.cycle_burn_rate.toFixed(2) : "-";

                    let fromP = "-";
                    if (c.current && c.current.priority !== undefined) {
                        fromP = c.current.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.current.priority;
                    } else if (c.priority_from !== undefined) {
                        fromP = c.disabled_from ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing ? "-" : c.priority_from);
                    }

                    let toP = "-";
                    if (c.target && c.target.priority !== undefined) {
                        toP = c.target.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.target.priority;
                    } else if (c.priority_to !== undefined) {
                        toP = c.disabled_to ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority_to;
                    } else if (c.priority !== undefined) {
                        toP = c.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority;
                    }

                    row.innerHTML = "<div>" +
                        "<div style=\"font-weight:700; font-size:13px; display:flex; align-items:center; gap:6px;\">" +
                            "<span>" + escapeHTML(name) + "</span>" +
                            (isBoost ? "<span class=\"badge badge-boost\">🚀 Boost</span>" : "") +
                            "<span class=\"badge badge-subtle\">" + escapeHTML(reasonText) + "</span>" +
                        "</div>" +
                        "<div style=\"font-size:11px; color:var(--text-muted); margin-top:2px; display:flex; gap:8px; flex-wrap:wrap;\">" +
                            "<span>5h: <strong>" + r5hText + "</strong></span>" +
                            "<span>7d: <strong>" + r7dText + "</strong></span>" +
                            "<span>" + t("urgencyLabel") + ": <strong>" + urgText + "</strong></span>" +
                            "<span>" + t("burnLabel") + ": <strong>" + burnText + "</strong></span>" +
                        "</div>" +
                    "</div>" +
                    "<div class=\"diff-value-box\">" +
                        "<span class=\"diff-from\">" + fromP + "</span>" +
                        "<span>&rarr;</span>" +
                        "<span class=\"diff-to\">" + toP + "</span>" +
                    "</div>";
                    list.appendChild(row);
                });
            } else {
                // Apply Mode: Show Concrete Priority & Disabled Diff
                changes.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";
                    let fromP = "-";
                    if (c.current && c.current.priority !== undefined) {
                        fromP = c.current.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.current.priority;
                    } else if (c.priority_from !== undefined) {
                        fromP = c.disabled_from ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing ? "-" : c.priority_from);
                    }

                    let toP = "-";
                    if (c.target && c.target.priority !== undefined) {
                        toP = c.target.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.target.priority;
                    } else if (c.priority_to !== undefined) {
                        toP = c.disabled_to ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority_to;
                    } else if (c.priority !== undefined) {
                        toP = c.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority;
                    }

                    const name = c.name || c.auth_index || "Credential";
                    const isBoost = Boolean(c.is_boosted || (c.reason && c.reason.indexOf("boost") >= 0));
                    const reasonText = formatReason(c.reason, isBoost, c.disabled_to || (c.target && c.target.disabled));

                    row.innerHTML = "<div>" +
                        "<div style=\"font-weight:700; font-size:13px; display:flex; align-items:center; gap:6px;\">" +
                            "<span>" + escapeHTML(name) + "</span>" +
                            (isBoost ? "<span class=\"badge badge-boost\">🚀 Boost</span>" : "") +
                        "</div>" +
                        "<div style=\"font-size:12px; color:var(--text-muted);\">" + escapeHTML(reasonText) + "</div>" +
                    "</div>" +
                    "<div class=\"diff-value-box\">" +
                        "<span class=\"diff-from\">" + fromP + "</span>" +
                        "<span>&rarr;</span>" +
                        "<span class=\"diff-to\">" + toP + "</span>" +
                    "</div>";
                    list.appendChild(row);
                });
            }
            modal.hidden = false;
        }

        function closeModal() {
            document.getElementById("diffModal").hidden = true;
        }

        function applyFromModal() {
            closeModal();
            triggerApplyWithConfirm();
        }

        async function triggerReset() {
            var confirmed = await showThemedConfirm({
                title: t("confirmResetTitle"),
                message: t("confirmResetMessage"),
                confirmText: currentLang === "zh-CN" ? "确认重置" : "Reset",
                cancelText: currentLang === "zh-CN" ? "取消" : "Cancel",
                isDanger: true
            });
            if (!confirmed) return;

            const btn = document.getElementById("btnReset");
            if (btn) btn.disabled = true;
            try {
                const result = await apiFetch(RESET_PATH, { method: "POST" });
                showToast(result.message || t("resetSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        function formatCountdown(targetDateStr) {
            if (!targetDateStr) return "-";
            var target = new Date(targetDateStr).getTime();
            var now = Date.now();
            var diff = target - now;
            if (diff <= 0) return currentLang === "zh-CN" ? "就绪" : "Ready";

            var days = Math.floor(diff / (1000 * 60 * 60 * 24));
            var hours = Math.floor((diff % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
            var minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
            var seconds = Math.floor((diff % (1000 * 60)) / 1000);

            if (days > 0) {
                return days + "d " + pad(hours) + "h " + pad(minutes) + "m";
            }
            if (hours > 0) {
                return pad(hours) + "h " + pad(minutes) + "m " + pad(seconds) + "s";
            }
            if (minutes > 0) {
                return pad(minutes) + "m " + pad(seconds) + "s";
            }
            return pad(seconds) + "s";
        }

        function pad(n) { return n < 10 ? "0" + n : n; }

        function renderDashboard() {
            if (latestDiagnostics) {
                var auditEl = document.getElementById("valLastAudit");
                if (auditEl) auditEl.textContent = latestDiagnostics.latest_audit || "-";
                var nextRunAt = latestDiagnostics.scheduler && latestDiagnostics.scheduler.next_run_at;
                var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (latestDiagnostics.scheduler && latestDiagnostics.scheduler.next_wait || "-");
                var el = document.getElementById("valNextProbe");
                if (el) {
                    el.innerHTML = (currentLang === "zh-CN" ? "下次调度: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>";
                }
            }

            const container = document.getElementById("credentialsContainer");
            if (!container || !latestSnapshot) return;

            const groupSelect = document.getElementById("modelGroupSelect");
            const selectedGroup = groupSelect ? groupSelect.value : "gemini";
            const activeGroup = (latestSnapshot && latestSnapshot.active_model_group) || "gemini";
            const isPredictedView = selectedGroup !== activeGroup;
            const groupData = (latestSnapshot && latestSnapshot.groups && latestSnapshot.groups[selectedGroup]) || {};
            const items = groupData.items || [];
            let boostedCount = 0;
            let depletedCount = 0;
            let activeCount = 0;

            items.forEach(item => {
                if (item.is_boosted) boostedCount++;
                if (item.target && item.target.disabled) depletedCount++;
                if (item.target && !item.target.disabled) activeCount++;
            });

            document.getElementById("valTotalCreds").textContent = items.length;
            document.getElementById("valTotalDesc").textContent = activeCount + " " + (currentLang === "zh-CN" ? "活跃可用" : "active");
            document.getElementById("valBoosted").textContent = boostedCount;
            document.getElementById("valDepleted").textContent = depletedCount;

            container.innerHTML = "";
            if (items.length === 0) {
                container.innerHTML = "<div class=\"empty-state\">" + t("noCreds") + "</div>";
                return;
            }

            items.forEach(item => {
                const card = document.createElement("div");
                card.className = "cred-card";

                const isBoosted = item.is_boosted;
                const r5hPercent = Math.max(0, Math.min(100, Math.round((item.r5h || 0) * 100)));
                const r7dPercent = Math.max(0, Math.min(100, Math.round((item.r7d || 0) * 100)));

                let fill5hClass = "meter-fill-healthy";
                if (r5hPercent <= 10) fill5hClass = "meter-fill-danger";
                else if (r5hPercent <= 30) fill5hClass = "meter-fill-warning";

                let fill7dClass = "meter-fill-healthy";
                if (r7dPercent <= 10) fill7dClass = "meter-fill-danger";
                else if (r7dPercent <= 30) fill7dClass = "meter-fill-warning";

                const urgency = (item.urgency || 0).toFixed(2);
                const burnRate = (item.cycle_burn_rate || 0).toFixed(2);
                const currentP = item.current ? item.current.priority : (item.priority || "-");
                const targetP = item.target ? (item.target.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : item.target.priority) : currentP;

                let statusBadge = "<span class=\"badge badge-success\">" + t("statusActive") + "</span>";
                if (item.target && item.target.disabled) {
                    statusBadge = "<span class=\"badge badge-danger\">" + t("statusWeeklyDepleted") + "</span>";
                } else if (item.reason && item.reason.indexOf("429") >= 0) {
                    statusBadge = "<span class=\"badge badge-warning\">" + t("statusCooldown") + "</span>";
                } else if (isBoosted) {
                    statusBadge = "<span class=\"badge badge-boost\">" + t("statusBoosted") + "</span>";
                }

                const formattedReason = formatReason(item.reason, isBoosted, item.target && item.target.disabled);

                card.innerHTML =
                    "<div class=\"cred-info\">" +
                        "<div class=\"cred-name\">" +
                            "<span>" + escapeHTML(item.name || item.account || item.auth_index || "Credential") + "</span>" +
                        "</div>" +
                        "<div class=\"cred-meta\">ID: " + escapeHTML(item.auth_index || "-") + " · " + escapeHTML(item.plan_type || "Antigravity") + "</div>" +
                        "<div class=\"cred-badge-row\">" +
                            statusBadge +
                            "<div class=\"metric-pill metric-pill-urgency\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                            "<div class=\"metric-pill metric-pill-burn\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("shortWindow") + " (" + r5hPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.short_window_reset_at || "") + "\">" + formatCountdown(item.short_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill5hClass + "\" style=\"width: " + r5hPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"meter-container\">" +
                        "<div class=\"meter-label-row\">" +
                            "<span>" + t("longWindow") + " (" + r7dPercent + "%)</span>" +
                            "<span class=\"meter-countdown\" data-reset-time=\"" + (item.long_window_reset_at || "") + "\">" + formatCountdown(item.long_window_reset_at) + "</span>" +
                        "</div>" +
                        "<div class=\"meter-track\">" +
                            "<div class=\"meter-fill " + fill7dClass + "\" style=\"width: " + r7dPercent + "%\"></div>" +
                        "</div>" +
                    "</div>" +

                    "<div class=\"cred-priority\">" +
                        "<div class=\"priority-score-box\">" +
                            "<span style=\"font-size:12px; color:var(--text-muted); font-weight:650;\">" + t("priority") + ":</span>" +
                            "<span class=\"priority-score" + (isPredictedView ? " priority-predicted" : "") + "\">" + targetP + "</span>" +
                            (isPredictedView ? " <span class=\"badge badge-predicted\">" + t("predictedBadge") + "</span>" : "") +
                        "</div>" +
                        "<div style=\"font-size:11px; color:var(--text-secondary);\">" + escapeHTML(formattedReason) + "</div>" +
                    "</div>";

                container.appendChild(card);
            });
        }

        function renderHistory() {
            const list = document.getElementById("historyList");
            if (!list) return;

            const entries = (latestDiagnostics && latestDiagnostics.run_history) || [];
            list.innerHTML = "";
            if (entries.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noHistory") + "</div>";
                return;
            }

            entries.forEach(function(entry, idx) {
                const item = document.createElement("div");
                item.className = "history-item";
                const dateStr = entry.at ? new Date(entry.at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                const kindText = formatHistoryKind(entry.kind);
                const msg = entry.message || "";
                const hasSnapshot = entry.snapshot && (entry.snapshot.items || entry.snapshot.changes);

                const succText = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (entry.succeeded || 0);
                const failText = (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (entry.failed || 0);
                const skipText = (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (entry.skipped || 0);

                item.innerHTML =
                    "<div class=\"history-head\">" +
                        "<div style=\"display:flex; align-items:center; gap:8px;\">" +
                            "<span class=\"badge badge-subtle\">" + escapeHTML(kindText) + "</span>" +
                            "<span style=\"font-size:13px; font-weight:600;\">" + escapeHTML(dateStr) + "</span>" +
                        "</div>" +
                        "<div class=\"history-stats\">" +
                            "<span class=\"badge badge-success\">" + succText + "</span>" +
                            "<span class=\"badge badge-danger\">" + failText + "</span>" +
                            "<span class=\"badge badge-subtle\">" + skipText + "</span>" +
                            (hasSnapshot ? "<button type=\"button\" class=\"btn-secondary\" style=\"min-height:24px; height:24px; padding:0 8px; font-size:11px;\" onclick=\"showHistoryDetails(" + idx + ")\">" + t("viewDetails") + "</button>" : "") +
                        "</div>" +
                    "</div>" +
                    (msg ? "<div style=\"font-size:11.5px; color:var(--text-muted); font-family:monospace; line-height:1.3;\">" + escapeHTML(msg) + "</div>" : "");
                list.appendChild(item);
            });
        }

        function renderDiagnostics() {
            var raw = document.getElementById("rawDiagnostics");
            var sched = document.getElementById("schedulerInfo");
            if (raw && latestDiagnostics) {
                raw.textContent = JSON.stringify(latestDiagnostics, null, 2);
            }
            if (sched && latestDiagnostics && latestDiagnostics.scheduler) {
                var nextRunAt = latestDiagnostics.scheduler.next_run_at;
                var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (latestDiagnostics.scheduler.next_wait || "-");
                var intervalText = (currentLang === "zh-CN" ? "执行周期: " : "Interval: ") + (latestDiagnostics.scheduler.interval || "-");
                var activeText = (currentLang === "zh-CN" ? "运行状态: " : "Active: ") + (latestDiagnostics.scheduler.worker_active ? (currentLang === "zh-CN" ? "运行中" : "Yes") : (currentLang === "zh-CN" ? "已暂停" : "No"));
                var nextText = (currentLang === "zh-CN" ? "下次运行: " : "Next Run: ");

                sched.innerHTML =
                    "<div style=\"font-weight:700; color:var(--text-primary); display:flex; align-items:center; gap:6px;\">" +
                        "<span>⏰</span><span>" + (currentLang === "zh-CN" ? "调度器状态" : "Scheduler Status") + "</span>" +
                    "</div>" +
                    "<div style=\"display:flex; align-items:center; gap:8px; color:var(--text-secondary); font-size:12px; flex-wrap:wrap;\">" +
                        "<span>" + intervalText + "</span> · " +
                        "<span>" + activeText + "</span> · " +
                        "<span>" + nextText + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span></span>" +
                    "</div>";
            }
        }

        function updateAllCountdowns() {
            document.querySelectorAll(".meter-countdown[data-reset-time]").forEach(function(el) {
                var resetTime = el.getAttribute("data-reset-time");
                if (resetTime) {
                    el.textContent = formatCountdown(resetTime);
                }
            });
            document.querySelectorAll("[data-scheduler-countdown]").forEach(function(el) {
                var t = el.getAttribute("data-scheduler-countdown");
                if (t) el.textContent = formatCountdown(t);
            });
            // Periodically refresh scheduler status indicator to catch window transitions
            if (scheduleConfig) {
                renderScheduleStatus();
            }
        }

        function showToast(msg, type) {
            var root = document.getElementById("toastRoot");
            if (!root) return;
            var toast = document.createElement("div");
            toast.className = "toast " + (type === "error" ? "toast-error" : type === "info" ? "toast-info" : "toast-success");
            toast.textContent = msg;
            root.appendChild(toast);
            setTimeout(function() {
                toast.classList.add("toast-exit");
                setTimeout(function() { toast.remove(); }, 200);
            }, 2800);
        }

        function escapeHTML(str) {
            return String(str || "")
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/"/g, "&quot;")
                .replace(/'/g, "&#039;");
        }

        function showThemedConfirm(opts) {
            return new Promise(function(resolve) {
                var modal = document.getElementById("confirmModal");
                var titleEl = document.getElementById("confirmTitle");
                var msgEl = document.getElementById("confirmMessage");
                var okBtn = document.getElementById("confirmOkBtn");
                var cancelBtn = document.getElementById("confirmCancelBtn");

                titleEl.textContent = opts.title || "";
                msgEl.textContent = opts.message || "";
                okBtn.textContent = opts.confirmText || "OK";
                cancelBtn.textContent = opts.cancelText || "Cancel";

                if (opts.isDanger) {
                    okBtn.className = "btn-danger";
                } else {
                    okBtn.className = "btn-primary";
                }

                function cleanup(result) {
                    modal.hidden = true;
                    okBtn.removeEventListener("click", onOk);
                    cancelBtn.removeEventListener("click", onCancel);
                    document.removeEventListener("keydown", onKey);
                    resolve(result);
                }
                function onOk() { cleanup(true); }
                function onCancel() { cleanup(false); }
                function onKey(e) { if (e.key === "Escape") cleanup(false); }

                okBtn.addEventListener("click", onOk);
                cancelBtn.addEventListener("click", onCancel);
                document.addEventListener("keydown", onKey);
                modal.hidden = false;
            });
        }

        async function triggerProbe() {
            var btn = document.getElementById("btnProbe");
            if (!btn || btn.classList.contains("btn-cooldown")) return;

            btn.disabled = true;
            btn.classList.add("btn-cooldown");

            try {
                var groupSelect = document.getElementById("modelGroupSelect");
                var group = groupSelect ? groupSelect.value : "gemini";
                var path = RUN_PATH + "?mode=probe&antigravity_model_group=" + encodeURIComponent(group);
                await apiFetch(path, { method: "POST" });
                showToast(t("probeSuccess"), "success");
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            }

            var remaining = 10;
            var label = btn.querySelector("[data-i18n='btnProbe']") || btn.querySelector("span");
            var origText = label ? label.textContent : "";
            probeCooldownTimer = setInterval(function() {
                remaining--;
                if (label) label.textContent = t("probeCooldown") + " (" + remaining + "s)";
                if (remaining <= 0) {
                    clearInterval(probeCooldownTimer);
                    probeCooldownTimer = null;
                    btn.disabled = false;
                    btn.classList.remove("btn-cooldown");
                    if (label) label.textContent = origText;
                }
            }, 1000);
        }

        async function copyDiagnosticsJSON() {
            var raw = document.getElementById("rawDiagnostics");
            if (!raw || !raw.textContent) return;
            try {
                await navigator.clipboard.writeText(raw.textContent);
                showToast(t("copied"), "success");
            } catch (e) {
                var textarea = document.createElement("textarea");
                textarea.value = raw.textContent;
                textarea.style.position = "fixed";
                textarea.style.left = "-9999px";
                document.body.appendChild(textarea);
                textarea.select();
                document.execCommand("copy");
                document.body.removeChild(textarea);
                showToast(t("copied"), "success");
            }
        }

        async function fetchScheduleConfig() {
            try {
                scheduleConfig = await apiFetch(SCHEDULE_CONFIG_PATH);
                renderScheduleStatus();
            } catch (_) {}
        }

        function renderScheduleStatus() {
            var badge = document.getElementById("scheduleStatusBadge");
            var textEl = document.getElementById("scheduleStatusText");
            if (!badge || !textEl) return;

            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            if (!isAutoApplyEnabled) {
                badge.className = "schedule-status paused";
                badge.title = currentLang === "zh-CN" ? "自动定时调度已关闭，点击前往配置中心开启" : "Auto-scheduling disabled. Click to open Config Center.";
                badge.innerHTML = "⚪ <span id=\"scheduleStatusText\">" + t("scheduleDisabled") + "</span>";
                return;
            }

            if (!scheduleConfig) return;

            if (scheduleConfig.paused) {
                badge.className = "schedule-status paused";
                badge.title = currentLang === "zh-CN" ? "点击恢复自动调度" : "Click to resume";
                badge.innerHTML = "⏸ <span id=\"scheduleStatusText\">" + t("schedulePaused") + "</span>";
                return;
            }

            var windowActive = true;
            if (scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                windowActive = isCurrentTimeInScheduleWindow(scheduleConfig.window_start, scheduleConfig.window_end);
            }

            if (scheduleConfig.window_enabled && !windowActive) {
                badge.className = "schedule-status sleeping";
                badge.title = currentLang === "zh-CN" ? "当前时间不在生效时段内" : "Outside active schedule window";
                badge.innerHTML = "🌙 <span id=\"scheduleStatusText\">" + t("scheduleSleeping") + "</span>";
                return;
            }

            var windowStr = "";
            if (scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                windowStr = " (" + scheduleConfig.window_start + "-" + scheduleConfig.window_end + ")";
            }
            badge.className = "schedule-status active";
            badge.title = currentLang === "zh-CN" ? "点击暂停自动调度" : "Click to pause";
            badge.innerHTML = "🟢 <span id=\"scheduleStatusText\">" + t("scheduleActive") + windowStr + "</span>";
        }

        async function toggleSchedulePause() {
            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            if (!isAutoApplyEnabled) {
                switchTab("config");
                showToast(currentLang === "zh-CN" ? "请在配置中心开启“自动定时调度”开关" : "Please enable 'Auto Periodic Scheduling' in Config Center", "info");
                return;
            }

            if (!scheduleConfig) {
                await fetchScheduleConfig();
                if (!scheduleConfig) return;
            }

            var newConfig = {
                paused: !scheduleConfig.paused,
                window_enabled: scheduleConfig.window_enabled || false,
                window_start: scheduleConfig.window_start || "00:00",
                window_end: scheduleConfig.window_end || "23:59"
            };

            try {
                scheduleConfig = await apiFetch(SCHEDULE_CONFIG_PATH, {
                    method: "POST",
                    body: JSON.stringify(newConfig)
                });
                renderScheduleStatus();
                showToast(scheduleConfig.paused ? t("schedulePaused") : t("scheduleActive"), "info");
            } catch (err) {
                showToast(err.message, "error");
            }
        }

        function showHistoryDetails(index) {
            if (!latestDiagnostics || !latestDiagnostics.run_history) return;
            var entry = latestDiagnostics.run_history[index];
            if (!entry) return;

            var snap = entry.snapshot;
            if (!snap || (!snap.items && !snap.changes)) {
                showToast(currentLang === "zh-CN" ? "该记录无详细快照数据" : "No snapshot data for this entry", "info");
                return;
            }

            var result = {
                snapshot: snap,
                items: snap.items || [],
                changes: snap.changes || [],
                attempted: entry.attempted || 0,
                succeeded: entry.succeeded || 0,
                failed: entry.failed || 0,
                skipped: entry.skipped || 0
            };

            var mode = (entry.kind || "").toLowerCase().indexOf("apply") >= 0 ? "apply" : "dry-run";
            // Strictly read-only for historical detail modal
            showModal(mode, result, true);
        }

        async function fetchDynamicConfig() {
            try {
                dynamicConfig = await apiFetch(CONFIG_PATH);
                renderDynamicConfigForm(dynamicConfig);
                renderScheduleStatus();
            } catch (err) {
                showToast(err.message, "error");
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
            var modelGroup = (document.getElementById("cfgModelGroup") && document.getElementById("cfgModelGroup").value) || "gemini";
            var windowEnabled = Boolean(document.getElementById("cfgWindowEnabled") && document.getElementById("cfgWindowEnabled").checked);
            var windowStart = (document.getElementById("cfgWindowStart") && document.getElementById("cfgWindowStart").value.trim()) || "";
            var windowEnd = (document.getElementById("cfgWindowEnd") && document.getElementById("cfgWindowEnd").value.trim()) || "";
            var maxConcurrency = (document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "";
            var minChange = (document.getElementById("cfgMinChange") && document.getElementById("cfgMinChange").value) || "";
            var urgencyTol = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "";
            var sampleCapacity = (document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "";
            var cooldownMin = (document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "";
            var rulesEnabled = Boolean(document.getElementById("cfgRulesEnabled") && document.getElementById("cfgRulesEnabled").checked);
            var boostStart = (document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "";
            var normalStart = (document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "";

            return JSON.stringify({
                autoApply, interval, modelGroup, windowEnabled, windowStart, windowEnd,
                maxConcurrency, minChange, urgencyTol, sampleCapacity, cooldownMin, rulesEnabled, boostStart, normalStart
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

            var modelGroup = document.getElementById("cfgModelGroup");
            if (modelGroup) {
                modelGroup.value = cfg.antigravity_model_group || "gemini";
                updateCustomCfgModelDisplay();
            }

            var windowEnabled = document.getElementById("cfgWindowEnabled");
            var windowInputs = document.getElementById("cfgWindowInputs");
            var windowStart = document.getElementById("cfgWindowStart");
            var windowEnd = document.getElementById("cfgWindowEnd");
            var sched = cfg.schedule || {};
            if (windowEnabled) {
                windowEnabled.checked = Boolean(sched.window_enabled);
                if (windowInputs) windowInputs.style.opacity = sched.window_enabled ? "1" : "0.5";
            }
            if (windowStart) windowStart.value = sched.window_start || "09:00";
            if (windowEnd) windowEnd.value = sched.window_end || "23:00";

            var maxConcurrency = document.getElementById("cfgMaxConcurrency");
            if (maxConcurrency) maxConcurrency.value = cfg.max_concurrency || 6;

            var minChange = document.getElementById("cfgMinChange");
            if (minChange) minChange.value = cfg.min_change !== undefined ? cfg.min_change : 1;

            var urgencyTol = document.getElementById("cfgUrgencyTolerance");
            if (urgencyTol) urgencyTol.value = cfg.urgency_tolerance !== undefined ? cfg.urgency_tolerance : 0.05;

            var sampleCapacity = document.getElementById("cfgSampleCapacity");
            if (sampleCapacity) sampleCapacity.value = cfg.quota_sample_capacity !== undefined ? cfg.quota_sample_capacity : 6;

            var cooldownMin = document.getElementById("cfgCooldownMinutes");
            if (cooldownMin) cooldownMin.value = cfg.rate_limit_cooldown_minutes !== undefined ? cfg.rate_limit_cooldown_minutes : 5;

            var rules = cfg.priority_rules || {};
            var rulesEnabled = document.getElementById("cfgRulesEnabled");
            if (rulesEnabled) rulesEnabled.checked = rules.enabled !== false;

            var boostStart = document.getElementById("cfgBoostStartPriority");
            if (boostStart) boostStart.value = rules.boost_start_priority || 999;

            var normalStart = document.getElementById("cfgNormalStartPriority");
            if (normalStart) normalStart.value = rules.normal_start_priority || 100;

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

        function onWindowEnabledChange() {
            var chk = document.getElementById("cfgWindowEnabled");
            var inputs = document.getElementById("cfgWindowInputs");
            if (chk && inputs) {
                inputs.style.opacity = chk.checked ? "1" : "0.5";
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
                        if (!interval || !/^([1-9]\d*)(s|m|h)$/.test(interval)) {
                            showToast(t("valErrInterval"), "error");
                            if (intervalCustom) intervalCustom.focus();
                            updateSaveButtonState();
                            return;
                        }
                    } else {
                        interval = intervalSelect.value;
                    }
                }

                var modelGroup = (document.getElementById("cfgModelGroup") && document.getElementById("cfgModelGroup").value) || "gemini";
                var windowEnabled = Boolean(document.getElementById("cfgWindowEnabled") && document.getElementById("cfgWindowEnabled").checked);
                var windowStart = (document.getElementById("cfgWindowStart") && document.getElementById("cfgWindowStart").value.trim()) || "09:00";
                var windowEnd = (document.getElementById("cfgWindowEnd") && document.getElementById("cfgWindowEnd").value.trim()) || "23:00";

                if (windowEnabled) {
                    var timeRegex = /^([01]\d|2[0-3]):[0-5]\d$/;
                    if (!timeRegex.test(windowStart) || !timeRegex.test(windowEnd)) {
                        showToast(t("valErrWindow"), "error");
                        updateSaveButtonState();
                        return;
                    }
                }

                var maxConcurrency = parseInt((document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "6", 10);
                if (isNaN(maxConcurrency) || maxConcurrency < 1 || maxConcurrency > 32) {
                    showToast(t("valErrConcurrency"), "error");
                    updateSaveButtonState();
                    return;
                }

                var minChange = parseInt((document.getElementById("cfgMinChange") && document.getElementById("cfgMinChange").value) || "1", 10);
                if (isNaN(minChange) || minChange < 0 || minChange > 100) {
                    showToast(t("valErrMinChange"), "error");
                    updateSaveButtonState();
                    return;
                }

                var urgencyTolRaw = (document.getElementById("cfgUrgencyTolerance") && document.getElementById("cfgUrgencyTolerance").value) || "0.05";
                var urgencyTol = parseFloat(urgencyTolRaw);
                if (isNaN(urgencyTol) || urgencyTol < 0.0 || urgencyTol > 0.50 || !/^\d+(\.\d{1,2})?$/.test(urgencyTolRaw)) {
                    showToast(t("valErrUrgencyTol"), "error");
                    updateSaveButtonState();
                    return;
                }

                var sampleCapacity = parseInt((document.getElementById("cfgSampleCapacity") && document.getElementById("cfgSampleCapacity").value) || "6", 10);
                if (isNaN(sampleCapacity) || sampleCapacity < 2 || sampleCapacity > 30) {
                    showToast(t("valErrSampleCapacity"), "error");
                    updateSaveButtonState();
                    return;
                }

                var cooldownMin = parseInt((document.getElementById("cfgCooldownMinutes") && document.getElementById("cfgCooldownMinutes").value) || "5", 10);
                if (isNaN(cooldownMin) || cooldownMin < 1 || cooldownMin > 1440) {
                    showToast(t("valErrCooldown"), "error");
                    updateSaveButtonState();
                    return;
                }

                var rulesEnabled = Boolean(document.getElementById("cfgRulesEnabled") && document.getElementById("cfgRulesEnabled").checked);
                var boostStart = parseInt((document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "999", 10);
                var normalStart = parseInt((document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "100", 10);

                if (isNaN(boostStart) || boostStart < 1 || boostStart > 999 || isNaN(normalStart) || normalStart < 1 || normalStart > 999) {
                    showToast(t("valErrPriorityRange"), "error");
                    updateSaveButtonState();
                    return;
                }
                if (normalStart > boostStart) {
                    showToast(t("valErrPriorityOrder"), "error");
                    updateSaveButtonState();
                    return;
                }

                var reqBody = {
                    auto_apply: autoApply,
                    interval: interval,
                    antigravity_model_group: modelGroup,
                    max_concurrency: maxConcurrency,
                    min_change: minChange,
                    urgency_tolerance: urgencyTol,
                    rate_limit_cooldown_minutes: cooldownMin,
                    quota_sample_capacity: sampleCapacity,
                    priority_rules: {
                        enabled: rulesEnabled,
                        boost_start_priority: boostStart,
                        normal_start_priority: normalStart
                    },
                    schedule: {
                        paused: Boolean(scheduleConfig && scheduleConfig.paused),
                        window_enabled: windowEnabled,
                        window_start: windowStart,
                        window_end: windowEnd
                    }
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
                interval: "15m",
                antigravity_model_group: "gemini",
                max_concurrency: 6,
                min_change: 1,
                urgency_tolerance: 0.05,
                quota_sample_capacity: 6,
                rate_limit_cooldown_minutes: 5,
                priority_rules: {
                    enabled: true,
                    boost_start_priority: 999,
                    normal_start_priority: 100
                },
                schedule: {
                    paused: false,
                    window_enabled: false,
                    window_start: "00:00",
                    window_end: "23:59"
                }
            };
            renderDynamicConfigForm(defaultCfg);
            updateSaveButtonState();
            await saveDynamicConfig();
        }

        // Initialize application
        applyLanguage();
        refreshDashboard();
        fetchScheduleConfig();
        fetchDynamicConfig();
        countdownInterval = setInterval(updateAllCountdowns, 1000);
    </script>
`
