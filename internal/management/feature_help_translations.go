package management

const featureHelpTranslations = `        const HELP_ZH = {
            "zh-CN": {
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
            }
        };
        const HELP_EN = {
            "en-US": {
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
            }
        };
`
