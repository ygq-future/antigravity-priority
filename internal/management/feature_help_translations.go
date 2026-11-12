package management

const featureHelpTranslations = `        const HELP_ZH = {
            "zh-CN": {
                helpTitle: "Antigravity Priority 原理与核心特色说明",
                helpGithub: "GitHub 主页",
                helpP1: "本插件专为 CLIProxyAPI 的 Google Antigravity 设计，围绕 Gemini、Argon 与 Claude & GPT 三大独立配额池执行自适应优先级调度与纯内存动态分流：",
                help5hTitle: "⚡ 5 小时短窗 (5h) 自愈软降级",
                help5hDesc: "实时跟踪 Gemini 与 Claude & GPT 的高频配额与重置倒计时。短窗耗尽时仅在内存中将优先级降至 -1（保持 disabled=false），窗口刷新后自动恢复调度，绝不改写宿主凭证的禁用状态。",
                help7dTitle: "📅 7 天周长窗 (7d) 全局把控",
                help7dDesc: "同时跟踪 Gemini、Argon 与 Claude & GPT 的周配额剩余量与重置进度。周额度耗尽同样执行纯内存软降级（priority=-1, disabled=false），避免单张凭证在周期前半段过早透支。",
                helpBurnTitle: "📈 自适应燃尽学习 (C_cycle / K_5h)",
                helpBurnDesc: "结合后台探测增量与实时请求用量，自动学习每个账号的周期燃尽率 C_cycle 与 5h 消耗灵敏度 K_5h，并通过 EMA 平滑收敛，无需手动配置数学系数。",
                helpUrgencyTitle: "⚖️ 配额燃尽紧迫度 (Weekly Urgency) 与平滑轮换",
                helpUrgencyDesc: "量化单位时间内的周额度消耗压力，配合容差分档将紧迫度相近的账号聚为同一优先级，并结合会话粘性与轮询指针实现均衡分流。",
                helpBoostTitle: "🚀 动态提前提权 (Dynamic Boost Horizon)",
                helpBoostDesc: "当剩余周额度无法在重置前自然耗尽时（周重置临近），自动进入 Boost 第一梯队，从默认 999 起按紧迫度梯次递减；Free 账号不参与提权，确保 Pro 优先。",
                helpWarmupTitle: "♨️ 5 小时短窗满额自动预热",
                helpWarmupDesc: "Gemini 或 Claude &amp; GPT 短窗 100% 满额、7d 仍有余量且重置倒计时未激活时，自动发送一条轻量自然请求提前开启 5 小时周期；重置到点后依据持久化配额证据补发，凭证之间随机间隔 60–120 分钟、同凭证不同分组 30–60 分钟，避免 5h 刷新时间点重合；构建防抖、随机抖动与卡片顶部图标可局部覆盖全局状态（点击结果与全局一致时自动恢复跟随），FREE 账号不参与预热。",
                helpTierTitle: "🎯 梯队分层与自定义覆盖",
                helpTierDesc: "调用次序为 自定义梯队 > Boost > 常规 Pro > Free 兜底；同梯队凭证共享相同优先级并轮询分流，支持在概览卡片上为单个账号指定自定义梯队。",
                help429Title: "🛡️ 健康故障转移与池级自愈",
                help429Desc: "401/403 视为账号级异常直接隔离并软降级；429 与 5xx 精准隔离至受影响配额池，并进入带抖动的指数退避探测队列，探测成功后自动恢复原梯队，同账号其他配额池不受影响。",
                helpUITitle: "🖥️ 可视化调度与观测",
                helpUIDesc: "概览看板展示模型路由与 5 条配额进度条，请求历史记录模型、Token 消耗与状态，系统诊断透出配额池矩阵、决策解释、健康隔离、运行时间线与四类缓存文件状态；配置中心修改后即时热生效。",
            }
        };
        const HELP_EN = {
            "en-US": {
                helpTitle: "Antigravity Priority Mechanics & Features",
                helpGithub: "GitHub Homepage",
                helpP1: "Tailored for Google Antigravity in CLIProxyAPI, scheduling the Gemini, Argon, and Claude & GPT quota pools with adaptive priorities and in-memory dynamic dispatch:",
                help5hTitle: "⚡ 5-Hour Short Window (5h) Self-Healing Soft Demotion",
                help5hDesc: "Tracks burst quota and reset countdowns for Gemini and Claude & GPT. Exhaustion only demotes priority to -1 in memory (disabled=false) and recovers automatically after reset; the host credential's disabled state is never mutated.",
                help7dTitle: "📅 7-Day Weekly Window (7d) Global Control",
                help7dDesc: "Tracks weekly remaining quota and reset progress across Gemini, Argon, and Claude & GPT. Weekly exhaustion also performs in-memory soft demotion (priority=-1, disabled=false) to prevent early overdraw.",
                helpBurnTitle: "📈 Adaptive Burn Learning (C_cycle / K_5h)",
                helpBurnDesc: "Learns each account's cycle burn rate C_cycle and 5h token sensitivity K_5h from probe deltas and live request usage, smoothed via EMA with no manual coefficient tuning.",
                helpUrgencyTitle: "⚖️ Weekly Urgency & Smooth Rotation",
                helpUrgencyDesc: "Quantifies per-hour weekly quota pressure and clusters accounts within tolerance into equal priority tiers, combining session affinity and round-robin for balanced dispatch.",
                helpBoostTitle: "🚀 Dynamic Boost Horizon",
                helpBoostDesc: "When remaining weekly quota cannot be naturally burned before reset, credentials enter the Boost tier starting at the configured 999 and descending by urgency; Free accounts never boost, keeping Pro first.",
                helpWarmupTitle: "♨️ 5-Hour Full-Quota Auto Warm-Up",
                helpWarmupDesc: "When a Gemini or Claude &amp; GPT 5h window is 100% full, the 7d pool still has quota, and no countdown is active, a lightweight natural request starts the 5-hour cycle early; reset-aligned persisted evidence triggers a follow-up, credentials are staggered 60–120 minutes apart and pools of one credential 30–60 minutes apart so refresh points never coincide; the card icon overrides the global state locally (a click that matches the global state restores follow mode) and Free plans never warm up.",
                helpTierTitle: "🎯 Tier Hierarchy & Custom Overrides",
                helpTierDesc: "Dispatch order is Custom Tier > Boost > Regular Pro > Free fallback; peers in the same tier share an identical priority and rotate round-robin, and any credential can be pinned to a custom tier from its overview card.",
                help429Title: "🛡️ Scoped Health Failover & Pool Self-Healing",
                help429Desc: "401/403 are treated as account-level failures and quarantined with soft demotion; 429 and 5xx are isolated to the affected pool and enter a jittered exponential-backoff probe queue, restoring the original tier after one success without impacting sibling pools.",
                helpUITitle: "🖥️ Visual Scheduling & Observability",
                helpUIDesc: "The overview board renders model routing and five quota bars, request history records models, token usage and status, diagnostics expose the pool matrix, decision explanations, health isolation, run timeline, and the four cache files, and the Config Center applies changes hot.",
            }
        };
`
