# AGENTS.md

`antigravity-priority` 是专为 CLIProxyAPI (CPA) 打造的高能效、专精型 **Google Antigravity 凭证配额调度与智能提权插件**。

### 核心设计原则
1. **单提供商专精化**：彻底剥离通用启发式与非 Antigravity 逻辑（如 Codex / xAI），只针对 Antigravity 的双时间窗口（5h 短窗 + 7d 周长窗）进行极致优化；
2. **自适应燃尽学习**：基于连续探测增量（$\Delta R_{\text{5h}} \ge 5\%, \Delta R_{\text{7d}} > 0$）自适应推算周期消耗能力 $C_{\text{cycle}}$ 并通过 EMA 平滑，杜绝用户手动配置负担；
3. **全局最优调度**：引入**配额燃尽紧迫度（Weekly Urgency）**与**动态提前提权（Dynamic Boost Horizon）**，彻底消除“最后 24 小时大额度账号撑死溢出”的痛点；
4. **自愈式软降级**：5 小时短窗耗尽仅软降级（`priority = -1, disabled = false`），无需人工干预即可在短窗刷新后自动恢复；周额度耗尽执行硬禁用（`priority = -1, disabled = true`）；
5. **CPA 深度融合**：嵌入式 Web 管理页纯原生自包含（零外部 CDN），深度适配 CPA 宿主暗色/明亮主题体系。

---

## 🔍 代码检索与索引契约 (Code Search Contract)

- **XERJ prefix**: `"agp"`

---

## 🚀 版本发布与提交前审阅规范 (Release & Pre-Commit Audit)

在每次进行版本迭代、功能改动完成、准备向用户提请提交或发布 Release 之前，**AI 智能体必须主动查阅并严格遵循 [`docs/release-checklist.md`](./docs/release-checklist.md)**。

必须严格对照该文档完成以下关联项自检：
1. **版本号 4 处一致性核对**：`registry.json`、`internal/runtime/runtime.go`、`internal/management/feature_shell_assets.go` 以及 `.github/release-notes/vX.Y.Z.md` 中的版本号必须 100% 同步一致；
2. **公开文档与双语规范**：`README.md` 与 `README.en.md` 保持中英双语对齐，聚焦核心价值，**严禁在面向用户的文档和 Release Notes 中出现 `(REQ-xx)` 内部研发标签**；每次起草 Release Notes 必须执行 `git log <last-tag>..HEAD --oneline` 完整回溯自上一次发布以来的全部提交，严禁遗漏任何中间 commit；
3. **架构与元数据契约**：确保 CPA 宿主 `config.yaml` 极简无扰、`buildMetadata()` 不暴露多余抽屉字段、`config.Default()` 作为基石兜底与 UI 配置中心保持一致；
4. **全套质量验证**：每完成一项功能或缺陷修复，必须先执行与 release workflow 一致的 `golangci-lint v2.12.2`，通过后再执行 `go test ./...`；提交或发布前必须再次执行 `golangci-lint run --timeout=5m`，并执行 `go build ./...`、`go vet ./...`、`go test ./...`、`go test -race ./...`，确保零告警与零数据竞争（默认不带 `-v`，避免冗余刷屏，失败时 Go 会自动打印失败详情）。

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
  - Web UI（`feature_shell_assets.go`）纯前端原生实现，无外部 CDN，样式完全遵从 CPA 主题。

---

## 📚 领域文档与配置索引 (Domain Docs & Configuration)

### Issue tracker

Issues and specs are tracked as local markdown files under `.scratch/<feature>/`. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical five-role triage labels (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context repository. See `docs/agents/domain.md`, `CONTEXT.md`, and `docs/adr/`.
