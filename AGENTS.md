# AGENTS.md

> **说明**：本文档与 [CLAUDE.md](./CLAUDE.md) 保持内容完全一致，共同定义了本项目的核心设计原则、严格交互确认协议、Git 提交验收门禁与发布审阅规范。

---

`antigravity-priority` 是专为 CLIProxyAPI (CPA) 打造的高能效、专精型 **Google Antigravity 凭证配额调度与智能提权插件**。

### 核心设计原则
1. **单提供商专精化**：彻底剥离通用启发式与非 Antigravity 逻辑（如 Codex / xAI），只针对 Antigravity 的双时间窗口（5h 短窗 + 7d 周长窗）进行极致优化；
2. **自适应燃尽学习**：基于连续探测增量（$\Delta R_{\text{5h}} \ge 5\%, \Delta R_{\text{7d}} > 0$）自适应推算周期消耗能力 $C_{\text{cycle}}$ 并通过 EMA 平滑，杜绝用户手动配置负担；
3. **全局最优调度**：引入**配额燃尽紧迫度（Weekly Urgency）**与**动态提前提权（Dynamic Boost Horizon）**，彻底消除“最后 24 小时大额度账号撑死溢出”的痛点；
4. **自愈式软降级**：5 小时短窗耗尽仅软降级（`priority = -1, disabled = false`），无需人工干预即可在短窗刷新后自动恢复；周额度耗尽执行硬禁用（`priority = -1, disabled = true`）；
5. **CPA 深度融合**：嵌入式 Web 管理页纯原生自包含（零外部 CDN），深度适配 CPA 宿主暗色/明亮主题体系。

---

## 🛑 严格交互与确认约束 (Strict Interaction Protocol)

除了用户明确输入了 `/<command>` 之类的 skill 或指令（如 `/impl` 等显式自动化指令）以外，当用户对某些功能抱有疑问、要求排查问题、分析缺陷、或要求做某些具体事情时，**严禁未经确认直接修改代码或盲目执行操作**。代理必须严格遵守以下确认流程：

1. **先输出思考与理解**：清晰告知用户你是如何理解用户所说的话和其核心诉求的；
2. **说明思路与解决路径**：说明你分析问题的思路、可能的根因排查方向或功能实现策略；
3. **列出具体行动计划**：明确说明你可能会怎么做（将要修改哪些文件、执行哪些命令、调整哪些逻辑）；
4. **说明预期结果与影响**：清晰阐述完成操作之后的预期结果、系统行为变化及是否有潜在副作用；
5. **等待用户明确确认**：将上述信息完整反馈给用户，**只有在得到用户的明确回复与确认后，方可开始具体修改或执行操作**。

---

## 🔒 Git 提交汇报与验收门禁 (Strict Git Commit Protocol)

**每次写完功能或修复后，严禁代理自主执行 Git commit 提交**。即使全量质量门禁完全通过，也必须严格遵守以下汇报与确认流程：

1. **汇报工作情况**：向用户详细汇报本次完成了哪些具体工作、修改了哪些文件、质量门禁的执行结果；
2. **说明验收需求**：明确说明本次修改是否有需要用户在真机/目标环境下验收的部分（如 Web UI 暗色主题表现、进度条刷新、CPA 宿主加载、接口交互等）；
3. **提供清晰验收步骤**：给出具体、可执行的验收操作步骤及对应的预期表现；
4. **等待用户确认**：只有在用户亲自确认验收结果，并给出明确指令（如“可以提交”、“commit”等）后，方可创建 Git commit。**严禁在未经用户明确授权的情况下自行提交**。

---

## 🚀 版本发布与提交前审阅规范 (Release & Pre-Commit Audit)

在每次进行版本迭代、功能改动完成、准备向用户提请提交或发布 Release 之前，**AI 智能体必须主动查阅并严格遵循 [`docs/release-checklist.md`](./docs/release-checklist.md)**。

必须严格对照该文档完成以下关联项自检：
1. **版本号 4 处一致性核对**：`registry.json`、`internal/runtime/runtime.go`、`internal/management/templates.go` 以及 `.github/release-notes/vX.Y.Z.md` 中的版本号必须 100% 同步一致；
2. **公开文档与双语规范**：`README.md` 与 `README.en.md` 保持中英双语对齐，聚焦核心价值，**严禁在面向用户的文档和 Release Notes 中出现 `(REQ-xx)` 内部研发标签**；
3. **架构与元数据契约**：确保 CPA 宿主 `config.yaml` 极简无扰、`buildMetadata()` 不暴露多余抽屉字段、`config.Default()` 作为基石兜底与 UI 配置中心保持一致；
4. **全套质量验证**：执行 `go build ./...`、`go vet ./...`、`go test -v ./...`、`go test -race ./...` 确保零告警与零数据竞争。

---

## 🏗️ 架构分层与所有权规范 (Architecture & Ownership)

- **Domain Core (`internal/core`, `internal/priority`, `internal/state`, `internal/provider/antigravity`)**：
  - `priority.Planner` 必须为**无副作用纯函数**，只输出不可变 `Plan` 对象；
  - 严禁在领域核心层调用 CPA 宿主写回或外部网络请求。
- **Apply Layer (`internal/apply`, `internal/host`)**：
  - 仅负责根据 `Plan` 门禁（Fresh Evidence、`ForceWrite` 与 `min_change`）执行宿主 `PatchPriority` / `PatchDisabled` 及全脱敏审计落盘。
- **Application Layer (`internal/runtime`)**：
  - 唯一拥有调度用例执行所有权（`ManualApply` / `AutoApply` / `Probe` / `SyncHost` / `ResetAllPriorities`）；
  - 统一协调 Ticker 定时器与单飞互斥锁（`runMu`）。
- **Adapters (`main.go`, `internal/management`)**：
  - CGO ABI 导出与 HTTP Management API 仅作为薄适配器层透传调用 Runtime 用例，**严禁自行计算或改写优先级逻辑**；
  - Web UI（`templates.go`）纯前端原生实现，无外部 CDN，样式完全遵从 CPA 主题。

---

## 📚 领域文档与配置索引 (Domain Docs & Configuration)

## Agent skills

### Issue tracker

Issues and specs are tracked as local markdown files under `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical five-role triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context repository. See `docs/agents/domain.md`, `CONTEXT.md`, and `docs/adr/`.
