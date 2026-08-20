# 架构深化方案设计 (Architecture Deepening Proposals)

本文档系统性记录 `antigravity-priority` 代码库在架构审查中识别出的 **模块深化机遇 (Deepening Opportunities)**。基于 `/codebase-design` 原则，目标在于增强模块的 **深度 (Depth)**、收敛维护者的 **内聚性 (Locality)**、提升调用者的 **杠杆效应 (Leverage)**，并确保 **接口即测试表面 (The interface is the test surface)**。

---

## 目录

1. [提案 1: 将探测失败策略吸收进 Priority Planner (Absorb Probe Failure Policy) [已完成]](#1-将探测失败策略吸收进-priority-planner-已完成)
2. [提案 2: 统一静态与动态配置至深度配置模块 (Unified Configuration Module) [已完成]](#2-统一静态与动态配置至深度配置模块-已完成)
3. [提案 3: 拆分物理文档修改与主机 RPC 客户端 (Extract Document Patcher Adapter) [已完成]](#3-拆分物理文档修改与主机-rpc-客户端-已完成)
4. [提案 4: 将状态缓存深化为高语义状态引擎 (Consolidate State Engine) [已完成]](#4-将状态缓存深化为高语义状态引擎-已完成)

---

## 1. 将探测失败策略吸收进 Priority Planner [已完成]

- **状态**：`已完成 (Completed)`
- **落地提交**：`a89510d refactor(priority): absorb probe failure policy into planner and archive deepening proposals`
- **推荐评级**：`Strong` (强烈推荐)
- **依赖类别**：`in-process`
- **涉及文件**：
  - `internal/priority/planner.go`
  - `internal/runtime/production_runner.go`

### 现状与缺陷 (Problem)
`priority.Planner` 虽设计为纯函数，但在 `PlanFreshOnly` 中仅处理了 `EvidenceStatusReady` 的有效配额证据。当上游探测失败（`EvidenceStatusProbeFailed`）时，规划器未在核心层对其进行领域状态分类，而是由运行时编排层 `production_runner.go` 中的 `withProbeFailureTemporaryDisables` 采用命令式循环遍历 `plan.Items` 和 `plan.Changes` 并后置修改字段（`Disabled = true, Reason = "failedQuotaFetch"`）。这造成领域禁用策略外溢到了运行时编排层，且破坏了规划输出的不可变契约。

### 深化方案 (Solution)
深化 `priority.PlanFreshOnly`，使其能够直接消费包含 `EvidenceStatusProbeFailed` 在内的全量新鲜证据。当账号处于非禁用态且探测失败时，规划器内部直接生成 `Disabled = true`、`Reason = "failedQuotaFetch"` 的 `PlanItem`，并由 `changes()` 机制自动产出写入变更。从 `production_runner.go` 中彻底移除 `withProbeFailureTemporaryDisables`。

### 收益 (Benefits)
- **Locality**：100% 的账号提权、正常排序、配额耗尽与故障禁用决策完全收敛于 `internal/priority`；
- **Interface & Testability**：`priority.PlanFreshOnly` 成为唯一的测试表面，单测无需启动或模拟运行时编排即可全量覆盖故障禁用场景；
- **Deletion Test**：删除 50+ 行脆弱的后置切片遍历与修改逻辑。

---

## 2. 统一静态与动态配置至深度配置模块 [已完成]

- **状态**：`已完成 (Completed)`
- **落地提交**：`33cb31b refactor(config): unify dynamic and static configuration into deep config module`
- **推荐评级**：`Strong` (强烈推荐)
- **依赖类别**：`in-process`
- **涉及文件**：
  - `internal/config/config.go`
  - `internal/config/schedule.go`
  - `internal/state/store.go`
  - `internal/state/schedule.go`
  - `internal/runtime/runtime.go`
  - `internal/runtime/production_runner.go`
  - `internal/management/handler.go`

### 现状与缺陷 (Problem)
系统配置当前被割裂在三个不同的浅层数据结构中：
1. `config.Config`：处理启动时的静态 YAML/JSON 解析；
2. `state.DynamicConfig`：处理 Web UI 配置中心的动态热更新；
3. `state.ScheduleConfig`：处理定时间隔与静默窗口。

这导致配置校验、默认值补齐、边界约束（如 `quota_sample_capacity`、`urgency_tolerance`、`min_change`）和热重载合并逻辑分散在 `config`、`runtime` 和 `management` 等多个包中，`runtime.go` 内部充斥着 4 个浅层转换函数。

### 深化方案 (Solution)
将 `internal/config` 深化为单一的配置权威中心（Configuration Authority）：
- 由 `config.Config` 统一封装静态加载、动态热更结构定义、字段边界校验、默认值继承与双向序列化；
- `state.Store` 仅负责存储和读取持久化配置快照（Raw Snapshot），不再侵入具体的配置结构体与业务校验；
- `Runtime` 与 `Management` 仅与深化的 `config` 模块交互，消除手动的字段拷贝与重复校验。

### 收益 (Benefits)
- **Locality**：所有的配置定义、校验约束和序列化规范完全内聚在 `internal/config`；
- **Leverage**：调用方使用单一统一接口完成配置加载、热更与校验，消除重复的手写映射；
- **Deletion Test**：移除 `runtime.go` 中多处冗余的 `mergePersistedDynamicConfig`、`applyDynamicConfigToRuntime` 等浅层胶水代码。

---

## 3. 拆分物理文档修改与主机 RPC 客户端 [已完成]

- **状态**：`已完成 (Completed)`
- **推荐评级**：`Worth exploring` (值得探索)
- **依赖类别**：`ports & adapters`
- **涉及文件**：
  - `internal/host/client.go`
  - `internal/host/patcher.go`
  - `internal/host/redact.go`
  - `internal/host/types.go`

### 现状与缺陷 (Problem)
`host.Client` 原先承担了两个物理职责截然不同的适配器角色：
1. **CGO Host RPC 客户端**：通过 CGO 桥接调用 CPA 宿主的 `ListAuthFiles`、`GetAuth`、`GetRuntime`、`HTTPDo`；
2. **物理文件 AST 修改器**：由于 CPA 未提供原生优先级修改接口，`Client` 直接通过 `os.ReadFile`、`os.CreateTemp` 和原子重命名对本地凭据 JSON 文件进行 AST 字节级原地修改（`PatchPriority`、`ResetPriority`、`PatchDisabled`）。

这导致 `host.Client` 既是一个网络/RPC 适配器，又是一个磁盘 I/O 文件存储引擎。单元测试中测试 RPC 逻辑时，往往不得不依赖或产生真实的磁盘临时文件。

### 深化方案 (Solution)
在主机通信与本地文件修改之间建立清晰的 **接缝 (Seam)**：
- 拆分出专门的 `DocumentPatcher` 适配器，专门封装原子物理文件写入、AST 字段定位与回填；
- `host.Client` 纯粹作为 CGO/RPC 适配器，只负责与 CPA 宿主上下文交互；
- 将通用的脱敏工具（`RedactBytes` 等）提炼为独立的脱敏模块 `redact.go`。

### 收益 (Benefits)
- **Locality**：磁盘物理文件操作与网络/RPC 回调解耦，各自职责清晰；
- **Testability**：Host RPC 测试可做到 100% 纯内存 Mock，零磁盘副作用；
- **Seam Discipline**：两种具体的 Adapter 分别支撑不同的外部交互环境。

---

## 4. 将状态缓存深化为高语义状态引擎 [已完成]

- **状态**：`已完成 (Completed)`
- **推荐评级**：`Worth exploring` (值得探索)
- **依赖类别**：`in-process`
- **涉及文件**：
  - `internal/state/store.go`
  - `internal/state/estimator.go`
  - `internal/state/schedule.go`
  - `internal/runtime/production_runner.go`
  - `internal/runtime/probe_evidence.go`

### 现状与缺陷 (Problem)
`state.Store` 原先是一个被动的线程安全 JSON 字典容器，向外暴露了 15 个细粒度的 Getter 和 Setter。
调用方（如 `production_runner.go`）必须显式编排复杂的内部状态变迁逻辑：
- 手动调用 `store.ClearExpiredCooldowns(now)`；
- 手动调用 `store.NeedsProbe(...)`；
- 手动将 `state.Entry` 转换为 `priority.ProbeEvidence`；
- 手动拼装备选模型组预测证据。

此外，`estimator.go` 中依然残留着未被生产逻辑引用的旧版 `CalculateCycleBurnRate` 估算函数。

### 深化方案 (Solution)
将 `state.Store` 深化为具备高杠杆的 **执行状态引擎 (Execution State Engine)**：
- 提供语义化的高阶接口 `GetCachedEvidence` 与 `BuildGroupEvidence`，将采样队列滑动窗口演进、EMA 平滑、持久化落盘与证据生成封装在内部；
- 提供 `GetActiveCooldowns(now)` 内部自动收敛过期冷却清理；
- 彻底删除 `estimator.go` 中遗留的废弃单跨度估算死代码。

### 收益 (Benefits)
- **Leverage**：调用方仅需 1 个语义化调用即可完成原本需要多步编排的状态变迁；
- **Locality**：配额样本历史、燃尽率推算与熔断器状态转换完全封装在状态引擎内部；
- **Deletion Test**：删除未使用的冗余估算函数与运行时 80+ 行拼装代码。
