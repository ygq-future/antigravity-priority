package management

// TemplateScripts contains all client-side JavaScript for the dashboard.
const TemplateScripts = `
    <script>
        const BASE_PATH = "/v0/management/plugins/antigravity-priority";
        const SNAPSHOT_PATH = BASE_PATH + "/snapshot/latest";
        const DIAGNOSTICS_PATH = BASE_PATH + "/diagnostics";
        const RUN_PATH = BASE_PATH + "/run";
        const SYNC_PATH = BASE_PATH + "/sync";
        const RESET_PATH = BASE_PATH + "/reset";
        const SCHEDULE_CONFIG_PATH = BASE_PATH + "/schedule/config";
        const CONFIG_PATH = BASE_PATH + "/config";
        const SAMPLES_PATH = BASE_PATH + "/samples";

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
                btnApply: "立即写回",
                btnReset: "重置优先级",
                btnProbe: "刷新配额",
                btnSamples: "采样",
                samplesModalTitle: "自适应时序采样明细",
                colObservedAt: "采样时间",
                colShortRem: "5h 余量",
                colLongRem: "7d 余量",
                noSamples: "暂无采样历史记录 (需等待连续探测)",
                actualPriority: "实际",
                targetPriority: "目标",
                predictedPriority: "预测",
                pendingApply: "待写回",
                unsetPriority: "[未设置]",
                syncSuccess: "已从 CPA 宿主同步最新凭证文件",
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
                btnConfirmApply: "确认写回",
                noChanges: "本次调度无优先级或禁用状态变化",
                noChangesToApply: "所有凭证已处于最优状态，无待写回变更",
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
                valErrPriorityOrder: "常规起始优先级不能大于 Boost 起始优先级",
                btnCopyDiagnostics: "复制诊断 JSON",
                diagKpiScheduler: "⏰ 调度引擎",
                diagKpiCooldown: "🛡️ 429 熔断监控",
                diagCooldownSubText: "自动降权 -1 机制就绪",
                diagKpiLastApply: "📊 最近写入体征",
                diagSectionScheduler: "调度运行详情与时间窗口",
                diagSchedIntervalLabel: "基础执行周期",
                diagSchedWorkerLabel: "后台 Worker 协程",
                diagSchedLastRunLabel: "上次自动调度",
                diagSchedNextRunLabel: "下次调度预计时间",
                diagWindowPolicyLabel: "调度时段策略:",
                diagSectionCooldown: "429 熔断与冷却凭证监控",
                diagCooldownEmptyText: "当前无处于 429 冷却中的凭证，所有账号运行正常",
                diagSectionAudit: "最近写入执行与脱敏审计",
                diagAuditSummaryLabel: "审计摘要流水",
                diagWindowContinuous: "全天 24 小时持续调度",
                diagWindowActive: "处于活跃调度窗口",
                diagWindowSleeping: "处于静默休眠窗口 (跳过调度)",
                diagNoLastRun: "暂无执行记录",
                diagStatusRunning: "运行中",
                diagStatusPaused: "已暂停",
                diagStatusSleeping: "休眠中",
                diagStatusDisabled: "已关闭",
                diagWorkerActive: "活跃运行",
                diagWorkerInactive: "未启动",
                diagHealthy: "正常",
                diagTripped: "熔断冷却中",
                diagCoolingDownCount: "个凭证冷却中",
                diagAllPassed: "全部成功",
                diagHasFailed: "存在失败",
                diagNoChanges: "无变更",
                diagSuccessPill: "成功",
                diagFailedPill: "失败",
                diagSkippedPill: "跳过",
                diagAttemptedPill: "尝试",
                diagRemainingSec: "恢复倒计时: ",
                copiedSuccess: "诊断数据已成功复制到剪贴板"
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
                btnApply: "Apply",
                btnReset: "Reset Priority",
                btnProbe: "Fetch Quota",
                btnSamples: "Samples",
                samplesModalTitle: "Adaptive Quota Samples",
                colObservedAt: "Observed At",
                colShortRem: "5h Quota",
                colLongRem: "7d Quota",
                noSamples: "No quota samples recorded yet",
                actualPriority: "Actual",
                targetPriority: "Target",
                predictedPriority: "Predicted",
                pendingApply: "Pending",
                unsetPriority: "[Unset]",
                syncSuccess: "Synchronized with CPA host auth files",
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
                btnConfirmApply: "Confirm Apply",
                noChanges: "No priority or status changes required",
                noChangesToApply: "All credentials are in optimal state, no changes to apply",
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
                valErrPriorityOrder: "Normal start priority cannot be greater than Boost start priority",
                btnCopyDiagnostics: "Copy Diagnostics JSON",
                diagKpiScheduler: "⏰ Scheduler Engine",
                diagKpiCooldown: "🛡️ 429 Circuit Breakers",
                diagCooldownSubText: "Auto demotion to -1 ready",
                diagKpiLastApply: "📊 Last Apply Health",
                diagSectionScheduler: "Scheduling Details & Time Window",
                diagSchedIntervalLabel: "Base Interval",
                diagSchedWorkerLabel: "Background Worker",
                diagSchedLastRunLabel: "Last Auto Run",
                diagSchedNextRunLabel: "Next Run Estimated",
                diagWindowPolicyLabel: "Window Policy:",
                diagSectionCooldown: "429 Rate Limit Cooldowns",
                diagCooldownEmptyText: "No credentials in 429 cooldown, all accounts operating normally",
                diagSectionAudit: "Latest Execution & Desensitized Audit",
                diagAuditSummaryLabel: "Audit Stream",
                diagWindowContinuous: "24/7 continuous scheduling",
                diagWindowActive: "In active schedule window",
                diagWindowSleeping: "In sleeping window (skipping schedule)",
                diagNoLastRun: "No execution records yet",
                diagStatusRunning: "Running",
                diagStatusPaused: "Paused",
                diagStatusSleeping: "Sleeping",
                diagStatusDisabled: "Disabled",
                diagWorkerActive: "Active",
                diagWorkerInactive: "Inactive",
                diagHealthy: "Healthy",
                diagTripped: "Cooling Down",
                diagCoolingDownCount: "creds cooling down",
                diagAllPassed: "All Succeeded",
                diagHasFailed: "Has Failures",
                diagNoChanges: "No Changes",
                diagSuccessPill: "Succeeded",
                diagFailedPill: "Failed",
                diagSkippedPill: "Skipped",
                diagAttemptedPill: "Attempted",
                diagRemainingSec: "Recover in: ",
                copiedSuccess: "Diagnostics JSON copied to clipboard"
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
            if (tabId === "history") fetchDiagnostics();
            if (tabId === "diagnostics") fetchDiagnostics();
            if (tabId === "config") fetchDynamicConfig();
			if (tabId === "overview") refreshDashboard(true);
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
                if (!userSelectedModelGroup && data && data.active_model_group) {
                    const select = document.getElementById("modelGroupSelect");
                    if (select && select.value !== data.active_model_group) {
                        select.value = data.active_model_group;
                        updateCustomSelectDisplay();
                        const menu = document.getElementById("customSelectMenu");
                        if (menu) {
                            menu.querySelectorAll(".custom-select-option").forEach(opt => {
                                const isSelected = opt.getAttribute("data-value") === data.active_model_group;
                                opt.classList.toggle("selected", isSelected);
                            });
                        }
                    }
                }
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

        async function syncHost() {
            try {
				const data = await apiFetch(SYNC_PATH, { method: "POST" });
                latestSnapshot = data;
                renderDashboard();
                showToast(t("syncSuccess"), "success");
            } catch (err) {
                await fetchSnapshot();
            }
        }

        async function refreshDashboard(withSync) {
            const btn = document.getElementById("btnRefresh");
            if (btn) btn.disabled = true;
            try {
                if (withSync) {
                    await syncHost();
                } else {
                    await fetchSnapshot();
                }
                await fetchDiagnostics();
            } finally {
                if (btn) btn.disabled = false;
            }
        }

        function extractChanges(result) {
            if (!result) return [];
            if (Array.isArray(result.changes) && result.changes.length > 0) {
                return result.changes;
            }
            if (result.snapshot && Array.isArray(result.snapshot.changes) && result.snapshot.changes.length > 0) {
                return result.snapshot.changes;
            }
            if (result.snapshot && Array.isArray(result.snapshot.items) && result.snapshot.items.length > 0) {
                return result.snapshot.items;
            }
            if (Array.isArray(result.items) && result.items.length > 0) {
                return result.items;
            }
            return [];
        }

        async function triggerApplyWithConfirm() {
            const groupSelect = document.getElementById("modelGroupSelect");
            const selectedGroup = groupSelect ? groupSelect.value : "gemini";
            const activeGroup = (dynamicConfig && dynamicConfig.antigravity_model_group) || (latestSnapshot && latestSnapshot.active_model_group) || "gemini";
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
                showToast(t("noChangesToApply"), "info");
                return;
            }

            // Show changes preview modal with direct confirm button
            showModal("apply-confirm", {
                snapshot: groupData,
                changes: changes,
                items: items
            }, false);
        }

        async function executeDirectApply() {
            closeModal();
            const groupSelect = document.getElementById("modelGroupSelect");
            const group = groupSelect ? groupSelect.value : "gemini";
            const btnApp = document.getElementById("btnApply");
            if (btnApp) btnApp.disabled = true;

            try {
                const path = RUN_PATH + "?mode=apply&antigravity_model_group=" + encodeURIComponent(group);
                const result = await apiFetch(path, { method: "POST" });
                const succeeded = (result && result.succeeded !== undefined) ? result.succeeded : 0;
                const attempted = (result && result.attempted !== undefined) ? result.attempted : 0;
                if (succeeded > 0 || attempted > 0) {
                    showToast(currentLang === "zh-CN" ? "已成功写回 CPA 宿主凭证优先级" : "Applied credential priorities to CPA host successfully", "success");
                } else {
                    showToast(t("noChangesToApply"), "info");
                }
                await refreshDashboard();
            } catch (err) {
                showToast(err.message, "error");
            } finally {
                if (btnApp) btnApp.disabled = false;
            }
        }

        // Show Modal: Apply Confirm vs History Details Snapshot
        function showModal(mode, result, isFromHistory) {
            const modal = document.getElementById("diffModal");
            const title = document.getElementById("modalTitle");
            const summary = document.getElementById("modalSummary");
            const list = document.getElementById("modalDiffList");
            const btnApply = document.getElementById("btnModalApply");

            const isApplyConfirm = (mode === "apply-confirm");
            const changes = extractChanges(result);

            if (isApplyConfirm) {
                title.textContent = t("confirmApplyTitle");
                btnApply.hidden = false;
                btnApply.textContent = t("btnConfirmApply");
                btnApply.onclick = executeDirectApply;
                summary.textContent = (currentLang === "zh-CN" ? "待写回凭证数量: " : "Credentials to update: ") + changes.length;
            } else {
                title.textContent = currentLang === "zh-CN" ? "执行明细快照" : "Execution Details Snapshot";
                btnApply.hidden = true;
                summary.textContent = (currentLang === "zh-CN" ? "成功: " : "Succeeded: ") + (result.succeeded || 0) + ", " +
                    (currentLang === "zh-CN" ? "失败: " : "Failed: ") + (result.failed || 0) + ", " +
                    (currentLang === "zh-CN" ? "跳过: " : "Skipped: ") + (result.skipped || 0);
            }

            list.innerHTML = "";
            if (changes.length === 0) {
                list.innerHTML = "<div class=\"empty-state\">" + t("noChanges") + "</div>";
            } else {
                changes.forEach(c => {
                    const row = document.createElement("div");
                    row.className = "diff-card";
                    let fromP = "-";
                    if (c.current && c.current.priority !== undefined && c.current.priority !== null) {
                        fromP = c.current.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.current.priority_missing ? t("unsetPriority") : c.current.priority);
                    } else if (c.priority_from !== undefined && c.priority_from !== null) {
                        fromP = c.disabled_from ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing ? t("unsetPriority") : c.priority_from);
                    }

                    let toP = "-";
                    if (c.target && c.target.priority !== undefined && c.target.priority !== null) {
                        toP = c.target.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.target.priority_missing ? t("unsetPriority") : c.target.priority);
                    } else if (c.priority_to !== undefined && c.priority_to !== null) {
                        toP = c.disabled_to ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : (c.priority_missing_to ? t("unsetPriority") : c.priority_to);
                    } else if (c.priority !== undefined && c.priority !== null) {
                        toP = (c.disabled || (c.target && c.target.disabled)) ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : c.priority;
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

        async function openSamplesModal(authIndex, name) {
            const modal = document.getElementById("samplesModal");
            const title = document.getElementById("samplesModalTitle");
            const sub = document.getElementById("samplesModalSubtitle");
            const body = document.getElementById("samplesModalBody");
            if (!modal || !body) return;

            title.textContent = (name || authIndex) + " - " + t("samplesModalTitle");
            sub.textContent = "ID: " + authIndex;
            body.innerHTML = "<div class=\"empty-state\">" + t("loading") + "</div>";
            modal.hidden = false;

            try {
                const path = SAMPLES_PATH + "?auth_index=" + encodeURIComponent(authIndex);
                const data = await apiFetch(path);
                const groups = (data && data.groups) || {};
                const geminiData = (groups.gemini && groups.gemini.samples) || [];
                const claudeData = (groups.claude_gpt && groups.claude_gpt.samples) || [];

                const maxLen = Math.max(geminiData.length, claudeData.length);
                if (maxLen === 0) {
                    body.innerHTML = "<div class=\"empty-state\">" + t("noSamples") + "</div>";
                    return;
                }

                function renderMiniMeter(val) {
                    if (val === undefined || val === null || val === "-") return "<span style=\"color:var(--text-muted);\">-</span>";
                    const num = Math.max(0, Math.min(100, parseInt(val, 10) || 0));
                    let fillClass = "meter-fill-healthy";
                    if (num <= 10) fillClass = "meter-fill-danger";
                    else if (num <= 30) fillClass = "meter-fill-warning";

                    return "<div style=\"display:flex; align-items:center; gap:8px; width:100%; min-width:85px; max-width:140px;\">" +
                        "<div style=\"flex:1; height:5px; background:var(--meter-bg); border-radius:999px; overflow:hidden;\">" +
                            "<div class=\"meter-fill " + fillClass + "\" style=\"width:" + num + "%; height:100%; border-radius:999px;\"></div>" +
                        "</div>" +
                        "<strong style=\"font-size:11px; font-family:monospace; min-width:32px; text-align:right;\">" + num + "%</strong>" +
                    "</div>";
                }

                let html = "<table class=\"sample-table\">" +
                    "<thead><tr>" +
                        "<th style=\"width:36px; text-align:center;\">#</th>" +
                        "<th style=\"width:150px;\">" + t("colObservedAt") + "</th>" +
                        "<th style=\"width:110px;\">" + (currentLang === "zh-CN" ? "模型组" : "Group") + "</th>" +
                        "<th>" + t("shortWindow") + "</th>" +
                        "<th>" + t("longWindow") + "</th>" +
                    "</tr></thead>";

                for (let i = 0; i < maxLen; i++) {
                    const g = geminiData[i];
                    const c = claudeData[i];
                    const timeRaw = (g && g.observed_at) || (c && c.observed_at);
                    const timeStr = timeRaw ? new Date(timeRaw).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";

                    const g5hMeter = g ? renderMiniMeter(g.short_window_rem) : "-";
                    const g7dMeter = g ? renderMiniMeter(g.long_window_rem) : "-";
                    const c5hMeter = c ? renderMiniMeter(c.short_window_rem) : "-";
                    const c7dMeter = c ? renderMiniMeter(c.long_window_rem) : "-";

                    html += "<tbody class=\"sample-group\">" +
                        "<tr>" +
                            "<td rowspan=\"2\" class=\"sample-group-bottom\" style=\"font-weight:700; text-align:center; vertical-align:middle; border-bottom:1px solid var(--border-color);\">" + (i + 1) + "</td>" +
                            "<td rowspan=\"2\" class=\"sample-group-bottom\" style=\"vertical-align:middle; border-bottom:1px solid var(--border-color); font-size:12px; color:var(--text-secondary); font-family:monospace;\">" + escapeHTML(timeStr) + "</td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\"><span class=\"badge badge-subtle\" style=\"font-size:10px;\">🔵 Gemini</span></td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\">" + g5hMeter + "</td>" +
                            "<td style=\"border-bottom:1px dashed var(--border-subtle); padding:6px 10px;\">" + g7dMeter + "</td>" +
                        "</tr>" +
                        "<tr>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\"><span class=\"badge badge-predicted\" style=\"font-size:10px;\">🟣 Claude/GPT</span></td>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + c5hMeter + "</td>" +
                            "<td class=\"sample-group-bottom\" style=\"border-bottom:1px solid var(--border-color); padding:6px 10px;\">" + c7dMeter + "</td>" +
                        "</tr>" +
                    "</tbody>";
                }

                html += "</table>";
                body.innerHTML = html;
            } catch (err) {
                body.innerHTML = "<div class=\"empty-state\" style=\"color:var(--accent-red-text);\">" + escapeHTML(err.message) + "</div>";
            }
        }

        function closeSamplesModal() {
            const modal = document.getElementById("samplesModal");
            if (modal) modal.hidden = true;
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

                var sched = latestDiagnostics.scheduler || {};
                var isAutoApplyEnabled = false;
                if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                    isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
                } else if (latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                    isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
                }

                var isPaused = scheduleConfig ? Boolean(scheduleConfig.paused) : Boolean(sched.paused);
                var isSleeping = false;
                if (scheduleConfig && scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                    isSleeping = !isCurrentTimeInScheduleWindow(scheduleConfig.window_start, scheduleConfig.window_end);
                } else if (sched.window_enabled && sched.window_start && sched.window_end) {
                    isSleeping = !isCurrentTimeInScheduleWindow(sched.window_start, sched.window_end);
                }

                var el = document.getElementById("valNextProbe");
                if (el) {
                    if (!isAutoApplyEnabled) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleDisabled") + "</span>";
                    } else if (isPaused) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-yellow-text); font-weight:600;\">" + t("schedulePaused") + "</span>";
                    } else if (isSleeping) {
                        el.innerHTML = (currentLang === "zh-CN" ? "调度状态: " : "Schedule: ") + "<span style=\"color:var(--accent-purple-text); font-weight:600;\">" + t("scheduleSleeping") + "</span>";
                    } else {
                        var nextRunAt = sched.next_run_at;
                        var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (sched.next_wait || "-");
                        el.innerHTML = (currentLang === "zh-CN" ? "下次调度: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>";
                    }
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

                let actualP = "-";
                if (item.current) {
                    if (item.current.disabled) actualP = currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]";
                    else if (item.current.priority_missing || item.current.priority === undefined || item.current.priority === null) actualP = t("unsetPriority");
                    else actualP = item.current.priority;
                } else if (item.priority_missing || item.priority === undefined || item.priority === null) {
                    actualP = t("unsetPriority");
                } else if (item.priority !== undefined && item.priority !== null) {
                    actualP = item.disabled ? (currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]") : item.priority;
                }

                let targetP = "-";
                if (item.target) {
                    if (item.target.disabled) targetP = currentLang === "zh-CN" ? "[已禁用]" : "[Disabled]";
                    else if (item.target.priority_missing || item.target.priority === undefined || item.target.priority === null) targetP = t("unsetPriority");
                    else targetP = item.target.priority;
                } else {
                    targetP = actualP;
                }

                let tagBadge = "";
                if (isPredictedView) {
                    tagBadge = "<span class=\"badge badge-predicted\">" + t("predictedBadge") + "</span>";
                } else if (String(actualP) !== String(targetP)) {
                    tagBadge = "<span class=\"badge badge-pending\">" + t("pendingApply") + "</span>";
                }

                let statusBadge = "<span class=\"badge badge-success\">" + t("statusActive") + "</span>";
                if (item.target && item.target.disabled) {
                    statusBadge = "<span class=\"badge badge-danger\">" + t("statusWeeklyDepleted") + "</span>";
                } else if (item.reason && item.reason.indexOf("429") >= 0) {
                    statusBadge = "<span class=\"badge badge-warning\">" + t("statusCooldown") + "</span>";
                } else if (isBoosted) {
                    statusBadge = "<span class=\"badge badge-boost\">" + t("statusBoosted") + "</span>";
                }

                const formattedReason = formatReason(item.reason, isBoosted, item.target && item.target.disabled);
                const authIdx = item.auth_index || "";
                const credDisplayName = item.name || item.account || item.auth_index || "Credential";

                card.innerHTML =
                    "<div class=\"cred-info\">" +
                        "<div class=\"cred-name\">" +
                            "<span>" + escapeHTML(credDisplayName) + "</span>" +
                        "</div>" +
                        "<div class=\"cred-meta\">ID: " + escapeHTML(item.auth_index || "-") + " · " + escapeHTML(item.plan_type || "Antigravity") + "</div>" +
                        "<div class=\"cred-badge-row\">" +
                            statusBadge +
                            "<div class=\"metric-pill metric-pill-urgency\">" + t("urgencyLabel") + "<strong>" + urgency + "</strong></div>" +
                            "<div class=\"metric-pill metric-pill-burn\">" + t("burnLabel") + "<strong>" + burnRate + "</strong></div>" +
                            "<button type=\"button\" class=\"btn-secondary\" style=\"min-height:20px; height:20px; padding:0 6px; font-size:11px; margin-left:auto; border-radius:4px;\" onclick=\"openSamplesModal('" + escapeHTML(authIdx) + "', '" + escapeHTML(credDisplayName) + "')\">📊 " + t("btnSamples") + "</button>" +
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
                        "<div class=\"priority-score-box\" style=\"flex-wrap:wrap;\">" +
                            "<span style=\"font-size:11.5px; color:var(--text-muted); font-weight:600;\">" + t("actualPriority") + ":</span>" +
                            "<span style=\"font-size:12.5px; font-weight:700; color:var(--text-secondary); font-family:SFMono-Regular, Consolas, Menlo, monospace;\">" + actualP + "</span>" +
                            "<span style=\"color:var(--border-color); margin:0 1px;\">|</span>" +
                            "<span style=\"font-size:11.5px; color:var(--text-muted); font-weight:600;\">" + (isPredictedView ? t("predictedPriority") : t("targetPriority")) + ":</span>" +
                            "<span class=\"priority-score" + (isPredictedView ? " priority-predicted" : "") + "\">" + targetP + "</span>" +
                            tagBadge +
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

        function copyDiagnosticsJSON() {
            if (!latestDiagnostics) {
                showToast(currentLang === "zh-CN" ? "暂无诊断数据可复制" : "No diagnostics data to copy", "info");
                return;
            }
            var text = JSON.stringify(latestDiagnostics, null, 2);
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(text).then(function() {
                    showToast(t("copiedSuccess") || (currentLang === "zh-CN" ? "诊断数据已成功复制到剪贴板" : "Diagnostics JSON copied to clipboard"), "success");
                }).catch(function() {
                    fallbackCopyText(text);
                });
            } else {
                fallbackCopyText(text);
            }
        }

        function fallbackCopyText(text) {
            var textarea = document.createElement("textarea");
            textarea.value = text;
            textarea.style.position = "fixed";
            textarea.style.left = "-9999px";
            textarea.style.top = "0";
            document.body.appendChild(textarea);
            textarea.focus();
            textarea.select();
            try {
                var successful = document.execCommand("copy");
                if (successful) {
                    showToast(t("copiedSuccess") || (currentLang === "zh-CN" ? "诊断数据已成功复制到剪贴板" : "Diagnostics JSON copied to clipboard"), "success");
                } else {
                    showToast("Copy failed", "error");
                }
            } catch (_) {
                showToast("Copy failed", "error");
            }
            document.body.removeChild(textarea);
        }

        function renderDiagnostics() {
            if (!latestDiagnostics) return;

            var sched = latestDiagnostics.scheduler || {};
            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }

            var isPaused = scheduleConfig ? Boolean(scheduleConfig.paused) : Boolean(sched.paused);
            var isSleeping = false;
            var windowStart = scheduleConfig && scheduleConfig.window_start ? scheduleConfig.window_start : sched.window_start;
            var windowEnd = scheduleConfig && scheduleConfig.window_end ? scheduleConfig.window_end : sched.window_end;
            var windowEnabled = scheduleConfig && scheduleConfig.window_enabled !== undefined ? scheduleConfig.window_enabled : sched.window_enabled;

            if (windowEnabled && windowStart && windowEnd) {
                isSleeping = !isCurrentTimeInScheduleWindow(windowStart, windowEnd);
            }

            // --- 1. KPI Card 1: Scheduler Engine ---
            var schedBadge = document.getElementById("diagSchedBadge");
            var schedInterval = document.getElementById("diagSchedInterval");
            var schedCountdown = document.getElementById("diagSchedCountdown");
            if (schedBadge && schedInterval && schedCountdown) {
                var nextRunAt = sched.next_run_at;
                var nextStr = nextRunAt ? formatCountdown(nextRunAt) : (sched.next_wait || "-");

                if (!isAutoApplyEnabled) {
                    schedBadge.className = "badge badge-subtle";
                    schedBadge.textContent = t("diagStatusDisabled");
                    schedCountdown.innerHTML = "<span style=\"color:var(--text-muted); font-weight:600;\">" + t("scheduleDisabled") + "</span>";
                } else if (isPaused) {
                    schedBadge.className = "badge badge-warning";
                    schedBadge.textContent = t("diagStatusPaused");
                    schedCountdown.innerHTML = "<span style=\"color:var(--accent-yellow-text); font-weight:600;\">" + t("schedulePaused") + "</span>";
                } else if (isSleeping) {
                    schedBadge.className = "badge badge-predicted";
                    schedBadge.textContent = t("diagStatusSleeping");
                    schedCountdown.innerHTML = "<span style=\"color:var(--accent-purple-text); font-weight:600;\">" + t("scheduleSleeping") + "</span>";
                } else {
                    schedBadge.className = "badge badge-success";
                    schedBadge.textContent = t("diagStatusRunning");
                    schedCountdown.innerHTML = (currentLang === "zh-CN" ? "下次运行: " : "Next run: ") + "<span class=\"meter-countdown\" data-scheduler-countdown=\"" + (nextRunAt || "") + "\">" + nextStr + "</span>";
                }
                schedInterval.textContent = sched.interval || "-";
            }

            // --- 2. KPI Card 2: 429 Cooldowns ---
            var cooldowns = (latestDiagnostics.active_cooldowns || []).slice();
            if (cooldowns.length === 0 && latestSnapshot && latestSnapshot.groups) {
                // Fallback scan across snapshot items in case active_cooldowns was not populated
                var groupKey = (document.getElementById("modelGroupSelect") && document.getElementById("modelGroupSelect").value) || (latestSnapshot.active_model_group || "gemini");
                var gData = latestSnapshot.groups[groupKey] || {};
                (gData.items || []).forEach(function(it) {
                    var r = (it.reason || "").toLowerCase();
                    if (r.indexOf("429") >= 0 || r.indexOf("cooldown") >= 0) {
                        cooldowns.push({
                            auth_index: it.name || it.auth_index,
                            model_group: groupKey,
                            reason: it.reason || "429 rate limit cooldown",
                            cooldown_until: (it.short_window_reset_at || it.reset_at || null)
                        });
                    }
                });
            }
            var cdBadge = document.getElementById("diagCooldownBadge");
            var cdCount = document.getElementById("diagCooldownCount");
            var cdSub = document.getElementById("diagCooldownSub");
            if (cdBadge && cdCount && cdSub) {
                if (cooldowns.length === 0) {
                    cdBadge.className = "badge badge-success";
                    cdBadge.textContent = t("diagHealthy");
                    cdCount.textContent = "0";
                    cdCount.style.color = "var(--text-primary)";
                    cdSub.textContent = t("diagCooldownSubText");
                } else {
                    cdBadge.className = "badge badge-danger";
                    cdBadge.textContent = t("diagTripped");
                    cdCount.textContent = cooldowns.length + " " + t("diagCoolingDownCount");
                    cdCount.style.color = "var(--accent-red-text)";
                    cdSub.textContent = (currentLang === "zh-CN" ? "已自动降权至 -1 保护中" : "Demoted to -1 for protection");
                }
            }

            // --- 3. KPI Card 3: Latest Apply Health ---
			var latestApply = latestDiagnostics.latest_apply || null;
			var succ = latestApply ? (latestApply.succeeded || 0) : 0;
			var fail = latestApply ? (latestApply.failed || 0) : 0;
			var skip = latestApply ? (latestApply.skipped || 0) : 0;
			var attempted = latestApply ? (latestApply.attempted || 0) : 0;
            var lastRunTime = (latestApply && latestApply.at) || null;

            var applyBadge = document.getElementById("diagApplyBadge");
            var applyStats = document.getElementById("diagApplyStats");
            var applyTime = document.getElementById("diagApplyTime");
            if (applyBadge && applyStats && applyTime) {
                if (!lastRunTime && (attempted === 0 && succ === 0 && fail === 0 && skip === 0)) {
                    applyBadge.className = "badge badge-subtle";
                    applyBadge.textContent = t("diagNoLastRun");
                    applyStats.textContent = "-";
                    applyTime.textContent = t("diagNoLastRun");
                } else if (fail > 0) {
                    applyBadge.className = "badge badge-danger";
                    applyBadge.textContent = t("diagHasFailed");
                    applyStats.textContent = succ + " " + t("diagSuccessPill") + " · " + fail + " " + t("diagFailedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                } else if (succ > 0) {
                    applyBadge.className = "badge badge-success";
                    applyBadge.textContent = t("diagAllPassed");
                    applyStats.textContent = succ + " " + t("diagSuccessPill") + " · " + skip + " " + t("diagSkippedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                } else {
                    applyBadge.className = "badge badge-subtle";
                    applyBadge.textContent = t("diagNoChanges");
                    applyStats.textContent = skip + " " + t("diagSkippedPill");
                    applyTime.textContent = lastRunTime ? new Date(lastRunTime).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
                }
            }

            // --- 4. Section 1: Scheduling Details & Window Panel ---
            var detInterval = document.getElementById("diagSchedDetailInterval");
            var detWorker = document.getElementById("diagSchedDetailWorker");
            var detLastRun = document.getElementById("diagSchedDetailLastRun");
            var detNextRun = document.getElementById("diagSchedDetailNextRun");
            var detWinText = document.getElementById("diagSchedDetailWindowText");
            var detWinBadge = document.getElementById("diagSchedDetailWindowBadge");

            if (detInterval) detInterval.textContent = sched.interval || "-";
            if (detWorker) {
                var workerActive = Boolean(sched.worker_active);
                detWorker.innerHTML = workerActive
                    ? "<span class=\"badge badge-success\" style=\"font-size:11px;\">" + t("diagWorkerActive") + "</span>"
                    : "<span class=\"badge badge-subtle\" style=\"font-size:11px;\">" + t("diagWorkerInactive") + "</span>";
            }
            if (detLastRun) {
                detLastRun.textContent = sched.last_auto_apply_at ? new Date(sched.last_auto_apply_at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : t("diagNoLastRun");
            }
            if (detNextRun) {
                detNextRun.textContent = sched.next_run_at ? new Date(sched.next_run_at).toLocaleString(currentLang === "zh-CN" ? "zh-CN" : "en-US") : "-";
            }
            if (detWinText && detWinBadge) {
                if (!windowEnabled || !windowStart || !windowEnd) {
                    detWinText.textContent = t("diagWindowContinuous");
                    detWinBadge.className = "badge badge-success";
                    detWinBadge.textContent = "24/7";
                } else {
                    detWinText.textContent = windowStart + " - " + windowEnd;
                    if (isSleeping) {
                        detWinBadge.className = "badge badge-predicted";
                        detWinBadge.textContent = t("diagWindowSleeping");
                    } else {
                        detWinBadge.className = "badge badge-success";
                        detWinBadge.textContent = t("diagWindowActive");
                    }
                }
            }

            // --- 5. Section 2: 429 Cooldown Content ---
            var cdContent = document.getElementById("diagCooldownContent");
            if (cdContent) {
                if (cooldowns.length === 0) {
                    cdContent.innerHTML = "<div class=\"diag-cooldown-empty\">" +
                        "<span>✅</span>" +
                        "<span>" + t("diagCooldownEmptyText") + "</span>" +
                    "</div>";
                } else {
                    var html = "<div class=\"diag-cooldown-list\">";
                    cooldowns.forEach(function(c) {
                        var cdUntil = c.cooldown_until;
                        var cdCountdownStr = cdUntil ? formatCountdown(cdUntil) : "-";
                        var groupTag = c.model_group ? "<span class=\"badge badge-subtle\" style=\"font-size:10px;\">" + escapeHTML(c.model_group) + "</span>" : "";
                        html += "<div class=\"diag-cooldown-item\">" +
                            "<div style=\"display:flex; flex-direction:column; gap:4px; min-width:0;\">" +
                                "<div style=\"display:flex; align-items:center; gap:6px;\">" +
                                    "<strong style=\"font-size:13px; color:var(--text-primary);\">" + escapeHTML(c.auth_index || "Credential") + "</strong>" +
                                    groupTag +
                                    "<span class=\"badge badge-danger\" style=\"font-size:10px;\">429 Cooldown</span>" +
                                "</div>" +
                                "<div style=\"font-size:11.5px; color:var(--text-muted); font-family:monospace;\">" + escapeHTML(c.reason || "429 rate limit") + "</div>" +
                            "</div>" +
                            "<div style=\"text-align:right; font-size:12px;\">" +
                                "<div style=\"color:var(--text-muted); font-size:11px;\">" + t("diagRemainingSec") + "</div>" +
                                "<strong class=\"meter-countdown\" data-cooldown-countdown=\"" + (cdUntil || "") + "\" style=\"font-family:monospace; color:var(--accent-yellow-text); font-size:13px;\">" + cdCountdownStr + "</strong>" +
                            "</div>" +
                        "</div>";
                    });
                    html += "</div>";
                    cdContent.innerHTML = html;
                }
            }

            // --- 6. Section 3: Latest Audit & Metrics ---
            var auditText = document.getElementById("diagAuditText");
            var auditPills = document.getElementById("diagAuditPills");
            if (auditText) {
				auditText.textContent = latestApply ? (latestApply.message || "-") : (currentLang === "zh-CN" ? "尚无 Apply 写入记录" : "No Apply write recorded yet");
            }
            if (auditPills) {
                var succCount = succ;
                var failCount = fail;
                var skipCount = skip;
                var attCount = attempted;

                auditPills.innerHTML =
                    "<span class=\"badge badge-success\">" + succCount + " " + t("diagSuccessPill") + "</span>" +
                    "<span class=\"badge badge-danger\">" + failCount + " " + t("diagFailedPill") + "</span>" +
                    "<span class=\"badge badge-subtle\">" + skipCount + " " + t("diagSkippedPill") + "</span>" +
                    "<span class=\"badge badge-subtle\" style=\"font-weight:700;\">" + attCount + " " + t("diagAttemptedPill") + "</span>";
            }
        }

        function updateAllCountdowns() {
            document.querySelectorAll(".meter-countdown[data-reset-time]").forEach(function(el) {
                var resetTime = el.getAttribute("data-reset-time");
                if (resetTime) {
                    el.textContent = formatCountdown(resetTime);
                }
            });

            document.querySelectorAll(".meter-countdown[data-cooldown-countdown]").forEach(function(el) {
                var cdTime = el.getAttribute("data-cooldown-countdown");
                if (cdTime) {
                    el.textContent = formatCountdown(cdTime);
                }
            });

            var isAutoApplyEnabled = false;
            if (dynamicConfig && dynamicConfig.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(dynamicConfig.auto_apply);
            } else if (latestDiagnostics && latestDiagnostics.management_api && latestDiagnostics.management_api.auto_apply !== undefined) {
                isAutoApplyEnabled = Boolean(latestDiagnostics.management_api.auto_apply);
            }
            var isPaused = scheduleConfig ? Boolean(scheduleConfig.paused) : (latestDiagnostics && latestDiagnostics.scheduler && Boolean(latestDiagnostics.scheduler.paused));
            var isSleeping = false;
            if (scheduleConfig && scheduleConfig.window_enabled && scheduleConfig.window_start && scheduleConfig.window_end) {
                isSleeping = !isCurrentTimeInScheduleWindow(scheduleConfig.window_start, scheduleConfig.window_end);
            }

            if (isAutoApplyEnabled && !isPaused && !isSleeping) {
                document.querySelectorAll("[data-scheduler-countdown]").forEach(function(el) {
                    var t = el.getAttribute("data-scheduler-countdown");
                    if (t) el.textContent = formatCountdown(t);
                });
            }
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
                renderDashboard();
                renderDiagnostics();
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

            // Strictly read-only for historical detail modal
            showModal("history", result, true);
        }

        async function fetchDynamicConfig() {
            try {
                dynamicConfig = await apiFetch(CONFIG_PATH);
                if (!userSelectedModelGroup && dynamicConfig && dynamicConfig.antigravity_model_group) {
                    const select = document.getElementById("modelGroupSelect");
                    if (select && select.value !== dynamicConfig.antigravity_model_group) {
                        select.value = dynamicConfig.antigravity_model_group;
                        updateCustomSelectDisplay();
                        const menu = document.getElementById("customSelectMenu");
                        if (menu) {
                            menu.querySelectorAll(".custom-select-option").forEach(opt => {
                                const isSelected = opt.getAttribute("data-value") === dynamicConfig.antigravity_model_group;
                                opt.classList.toggle("selected", isSelected);
                            });
                        }
                    }
                }
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
            var boostStart = (document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "";
            var normalStart = (document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "";

            return JSON.stringify({
                autoApply, interval, modelGroup, windowEnabled, windowStart, windowEnd,
                maxConcurrency, minChange, urgencyTol, sampleCapacity, cooldownMin, boostStart, normalStart
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

				var maxConcurrency = parseStrictInteger((document.getElementById("cfgMaxConcurrency") && document.getElementById("cfgMaxConcurrency").value) || "6");
                if (isNaN(maxConcurrency) || maxConcurrency < 1 || maxConcurrency > 32) {
                    showToast(t("valErrConcurrency"), "error");
                    updateSaveButtonState();
                    return;
                }

				var minChange = parseStrictInteger((document.getElementById("cfgMinChange") && document.getElementById("cfgMinChange").value) || "1");
                if (isNaN(minChange) || minChange < 0 || minChange > 100) {
                    showToast(t("valErrMinChange"), "error");
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

				var boostStart = parseStrictInteger((document.getElementById("cfgBoostStartPriority") && document.getElementById("cfgBoostStartPriority").value) || "999");
				var normalStart = parseStrictInteger((document.getElementById("cfgNormalStartPriority") && document.getElementById("cfgNormalStartPriority").value) || "100");

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

        // Initialize application with deterministic dependency ordering
        async function initializeApp() {
            applyLanguage();
            await fetchDynamicConfig();
            await fetchScheduleConfig();
            await refreshDashboard(true);
            countdownInterval = setInterval(updateAllCountdowns, 1000);
        }
        initializeApp();
    </script>
`
