# antigravity-priority {version}

## 更新内容

### 中文

- 用简洁、面向使用者的条目说明「改了什么、对我有什么影响」。
- 避免内部函数名、结构体字段名、源码路径等技术细节。
- 配置相关变更写清：字段名、默认行为、是否需要改配置。
- 行为变更写清：谁受益、谁不受影响。
- 不写「升级说明」小节；安装/替换动态库并重启宿主的通用步骤由商店或 README 覆盖即可。

**本次更新 ({version}) 包含：**
- 专精于 Google Antigravity 配额体系，支持 5 小时短窗与 7 天周长窗双窗口调度。
- 引入 Weekly Urgency（周配额燃尽紧迫度）与 Dynamic Boost Horizon（动态提前提权视界）算法，彻底杜绝额度过期浪费。
- 自适应周期消耗速率估算（Adaptive $C_{\text{cycle}}$ Estimator），自动学习真实消耗速率。
- 5 小时短窗口耗尽软降级（Soft Depletion），7 天周长窗耗尽硬禁用。
- 深度适配 CPA 明暗双主题的原生嵌入式管理面板（`/status`）。

### English

- Write short, user-facing bullets: what changed and how it affects day-to-day use.
- Avoid internal symbol names, struct fields, or source paths.
- For config changes: name the setting, default behavior, and whether users must edit config.
- For behavior changes: who is affected and who is not.
- Do not add a separate "Upgrade notes" section; replace the plugin binary and restart the host as usual.

**This update ({version}) includes:**
- Exclusively tailored for Google Antigravity quota management with dual-window (5h burst + 7d weekly) scheduling.
- Implements Weekly Urgency Index and Dynamic Boost Horizon algorithms to eliminate end-of-cycle quota waste.
- Adaptive cycle burn-rate estimation ($C_{\text{cycle}}$) learns actual consumption patterns without manual tuning.
- Self-healing soft depletion for 5h window exhaustion and hard disabling for 7d weekly exhaustion.
- Native CPA light/dark theme embedded web management dashboard (`/status`).
