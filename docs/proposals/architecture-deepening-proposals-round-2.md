# 架构深化方案设计：第二轮 (Architecture Deepening Proposals — Round 2)

本文档记录 `antigravity-priority` 第二轮架构审阅识别出的 **module 深化机遇 (Deepening Opportunities)**。

本轮审阅以近期高频变更区域为范围，重点检查：

- `internal/runtime`、`internal/state`、`internal/priority` 与 `internal/apply` 之间的调度执行链；
- `internal/management` 嵌入式管理页的最新资产拆分；
- Fresh Evidence、双 Model Group 投影、Host transition 与执行记录等跨 module 语义；
- 当前 interface 是否形成稳定测试表面，以及复杂度是否在 seam 两侧泄漏。

审阅使用 `module`、`interface`、`implementation`、`depth`、`deep`、`shallow`、`seam`、`adapter`、`leverage` 与 `locality` 作为统一架构词汇，并对候选执行 **deletion test**。

> **状态说明**：本文档中的四项均为 `待讨论 (Proposed)`。当前只记录问题与深化方向，不提前确定具体 interface，不代表已批准实施。

---

## 与第一轮提案的关系

第一轮提案记录在 [`architecture-deepening-proposals.md`](./architecture-deepening-proposals.md)，其中以下四项已经完成：

1. 将探测失败策略吸收进 Priority Planner；
2. 统一静态与动态配置至深度配置 module；
3. 拆分物理文档修改与 Host RPC adapter；
4. 将状态缓存深化为高语义状态引擎。

本轮候选不重新提出上述工作。部分问题与第一轮落地结果相邻，但关注的是落地后仍然存在的 interface 复杂度、语义泄漏和调用者编排知识。

---

## 目录

1. [统一 Host transition 与执行记录](#1-统一-host-transition-与执行记录)
2. [收拢 Fresh Evidence 权威归属](#2-收拢-fresh-evidence-权威归属)
3. [深化双 Model Group 规划投影](#3-深化双-model-group-规划投影)
4. [按用户功能收拢管理页资产](#4-按用户功能收拢管理页资产)
5. [推荐顺序与组合关系](#5-推荐顺序与组合关系)

---

## 1. 统一 Host transition 与执行记录

- **状态**：`待讨论 (Proposed)`
- **推荐强度**：`Strong`
- **依赖类别**：`local-substitutable`
- **建议优先级**：`P1 / Top recommendation`
- **涉及文件**：
  - `internal/apply/apply.go`
  - `internal/apply/audit.go`
  - `internal/runtime/production_runner.go`
  - `internal/runtime/runtime.go`
  - `internal/state/store.go`

### 目前的痛点

正常 Apply、429 Reactive Cooldown 与 ResetAllPriorities 是三种 Host 状态变更，但目前使用三套不完全一致的执行路径。

正常 Apply 的主要调用链为：

```text
priority.Plan
  → apply.Apply
  → Host PatchPriority / PatchDisabled
  → Runtime.snapshotRunEntry
  → json.Marshal
  → Store.SetRuntimeSnapshot
  → Store.SaveAtomic
```

429 Reactive Cooldown 由 `Runtime.triggerCooldown` 手工构造 `priority.Plan` 和 `priority.Change`，调用 `apply.Apply` 后再自行生成历史记录并持久化。

ResetAllPriorities 则绕开 `apply.Apply`，直接循环调用 `client.ResetPriority`，随后自行构造 `apply.Result`、快照、历史记录和持久化数据。

这些分叉导致 Host transition 的顺序、错误、结果、脱敏和持久化语义缺乏单一权威。

### 精确证据

#### 1.1 `Auditor` seam 的生产 adapter 是空 implementation

`apply.Request` 强制要求 `Auditor`：

```go
type Auditor interface {
    SaveSnapshot(ctx context.Context, snapshot PlanSnapshot) error
    RecordEvent(ctx context.Context, event AuditEvent) error
}
```

`apply.Apply` 在 Host 写入前调用 `SaveSnapshot` 与 `RecordEvent`。但是 Runtime 的生产 implementation 只返回 `ctx.Err()`，没有使用或保存传入参数：

```go
func (r *Runtime) SaveSnapshot(ctx context.Context, snapshot apply.PlanSnapshot) error {
    return ctx.Err()
}

func (r *Runtime) RecordEvent(ctx context.Context, event apply.AuditEvent) error {
    return ctx.Err()
}
```

真实记录发生在 Host 写入后，并分散在 Runtime 各执行分支中。因此 `Auditor` interface 表达的写前保证与生产行为不一致。

目前只有一个生产 adapter，且该 adapter 没有实际 implementation。按照“一种 adapter 代表 hypothetical seam，两种 adapter 才证明真实变化”的原则，该 seam 尚未证明自身价值。

#### 1.2 执行记录持久化在多个分支重复

以下步骤在 Probe、零变更 Apply、普通 Apply、Reset 与 429 Cooldown 中以不同形式重复：

```text
snapshotRunEntry
→ json.Marshal(result)
→ json.Marshal(history)
→ store.SetRuntimeSnapshot
→ store.SaveAtomic
```

部分路径检查 `SaveAtomic` 返回值，部分路径使用 `_ = store.SaveAtomic(ctx)` 忽略错误。执行记录是否可靠落盘取决于具体触发路径。

#### 1.3 部分 Host transition 缺乏明确结果语义

`applyChange` 可能按以下顺序执行：

```text
PatchPriority 成功
→ PatchDisabled 失败
```

此时 Host 已经发生部分变更，但最终结果主要标记为 `failed`。当前结果没有明确表达哪些字段已经成功写入，以及 Host 可能处于何种部分状态。

#### 1.4 Reset 的尝试与失败统计被压缩

`ResetAllPriorities` 只在 `ResetPriority` 成功时增加 `resetCount`，最终却同时使用该值作为：

```go
Attempted: resetCount,
Succeeded: resetCount,
```

如果尝试 10 个凭证而成功 7 个，记录可能只显示尝试 7 个、成功 7 个，失败的 3 次 transition 没有进入可信结果模型。

### 为什么需要深化

Host 状态变更是 Apply Layer 的核心所有权。调用者和测试需要通过一个稳定 interface 回答：

- 计划改变什么；
- 实际尝试了什么；
- 哪些字段成功写入；
- 哪些字段失败；
- 是否出现部分成功；
- 脱敏执行记录是否成功持久化；
- Host 当前可能处于什么状态。

当前这些知识分散在 `apply`、`runtime` 和 `state`，导致一次 transition 的完整行为无法通过单一 seam 验证。

### Deletion test

如果直接删除当前 `Auditor` interface 和 Runtime 的空 implementation，生产行为几乎不会改变，说明该 module 是 shallow 的。

相反，如果形成一个真正负责 Host transition 与执行记录生命周期的 deep module，再删除它，门禁、写入顺序、部分成功、结果统计、脱敏和持久化复杂度会重新散落到普通 Apply、Reset 与 429 Cooldown 三个调用者中。这表明该方向能够产生真实 depth 与 leverage。

### 深化方向

深化现有 Apply module，使一次 Host transition 的以下行为保持 locality：

```text
验证写入门禁
→ 捕获写前状态
→ 执行 Host transition
→ 表达完整成功、部分成功、失败或跳过
→ 生成脱敏执行记录
→ 持久化执行结果
```

普通 Apply、Reset 与 429 Reactive Cooldown 应复用同一 transition 语义。Runtime 仍然拥有 ManualApply、AutoApply、Reset 和 Cooldown 等用例执行权；本提案不把应用用例所有权移动到 Apply module。

具体 interface 必须在后续设计阶段确定，当前不预设新的 port 或 adapter。

### 优化目的

- 让 Host transition 的事实结果成为唯一权威；
- 消除名义审计与真实持久化之间的契约差异；
- 统一三种写入路径的门禁、错误和统计语义；
- 让执行记录与实际 Host 变化保持一致；
- 建立能够覆盖部分成功的稳定测试表面。

### 预期结果

- `Attempted`、`Succeeded`、`Failed`、`Skipped` 与部分成功统计可信；
- Reset、429 Cooldown 与普通 Apply 共享一致结果模型；
- 执行记录持久化错误不再被静默忽略；
- 测试可直接覆盖“priority 成功、disabled 失败”等 transition；
- Runtime 中重复的历史记录、序列化与持久化编排明显减少；
- 诊断页展示的最近写入健康状态能够对应真实 Host transition；
- Apply Layer 更符合 AGENTS.md 中“写回门禁与全脱敏审计落盘”的所有权规范。

### 风险与约束

- 不得把优先级计算移入 Apply Layer；Planner 仍必须是无副作用纯函数；
- 必须明确定义部分成功后的重试行为，避免重复写入造成新的状态漂移；
- 不应为了测试而暴露内部 seam；
- 如果最终仍只有一种生产 adapter，不应保留没有行为差异的抽象；
- 必须继续保证所有审计信息完全脱敏。

### ADR 关系

本提案不与现有 ADR 冲突。设计审问形成的决策已记录为 [`ADR-0005`](../adr/0005-atomic-host-transition-and-truthful-outcomes.md)，并强化：

- ADR-0003 中 Soft Depletion 与 Hard Depletion 的实际写入语义；
- ADR-0004 中 429 Reactive Cooldown 的非破坏性 Host transition；
- AGENTS.md 中 Apply Layer 对门禁、Host Patch 与脱敏审计的所有权。

### 设计审问已确认决策（2026-08-22）

以下结论已经通过 `/grill-with-docs` 与用户确认，后续设计和实施不得静默改变：

1. **同一凭证的目标字段属于一个 Host Transition**：priority 与 disabled 位于同一凭证文件时，通过一次文件替换共同提交，不保留两个独立业务结果；
2. **结果以 Host 最终状态为准**：请求是否返回错误不能单独决定 transition 是否成功；
3. **三类操作共享 transition 语义**：普通 Apply、429 Reactive Cooldown 与 Reset Priority 使用一致的成功、失败、验证和执行记录语义；
4. **credential 之间独立执行**：单个 credential 失败不阻止其他 credential；
5. **Host 当前状态具有并发权威**：检测到读取后的外部 decision-state 修改时，放弃旧 Plan，不覆盖，由下一轮 Fresh Evidence 和 Planner 重新决策；
6. **文件替换成功是 commit point**：commit point 前取消可安全返回失败；commit point 后的 context 取消不能否定已经发生的 Host Transition；
7. **提交后重读验证**：验证目标字段和 JSON 有效性，不要求整个文件字节完全相同；无法确认最终状态时报告 `uncertain`；
8. **Host outcome 与 Record outcome 分离**：Host 状态事实和执行记录可靠性是两个正交结果；
9. **并发比较只覆盖决策相关状态**：比较 credential identity、priority 与 disabled；无关字段使用最新 Host 文档并保持不变；
10. **字段目标使用显式操作语义**：每个目标字段表达 `unchanged`、`set` 或 `unset`，支持三类用例共享 transition implementation；
11. **不执行同轮自动重试**：失败后直接记录，由后续定时轮次基于新的 Fresh Evidence 补偿；
12. **429 Cooldown 事实独立持久化**：Host demotion 失败不撤销已经发生的 cooldown；
13. **Host outcome 使用五个终态**：`no_change`、`committed`、`failed`、`conflict` 与 `uncertain`，具体原因通过 reason 表达；
14. **整轮统计从 transition 明细推导**：汇总不得由各执行路径手工维护；
15. **三类用例统一产生 transition intent**：普通 Apply 从 Planner changes 转换；429 使用 `set priority=-1 / unchanged disabled`；Reset 使用 `unset priority / unchanged disabled`。Planner 继续独占调度计算。

#### 明确拒绝的推测性复杂度

以下设计在本轮被明确排除，除非未来出现可复现故障或新增真实使用场景：

- write-ahead sidecar journal；
- 精确的进程崩溃恢复与 pending 协调协议；
- journal schema、fingerprint identity、幂等清理与独立持久化路径；
- 同轮自动重试、退避或 replay；
- 为尚未出现的多 writer 场景引入额外 port 或 adapter。

---

## 2. 收拢 Fresh Evidence 权威归属

- **状态**：`待讨论 (Proposed)`
- **推荐强度**：`Strong`
- **依赖类别**：`in-process`
- **建议优先级**：`P1`
- **涉及文件**：
  - `internal/priority/planner.go`
  - `internal/state/store.go`
  - `internal/runtime/production_runner.go`
  - `internal/runtime/runtime.go`

### 目前的痛点

`CONTEXT.md` 将 Fresh Evidence 定义为当前调度轮次成功探测得到的、经过验证的配额观察数据。

但代码中，证据是否属于 Fresh Evidence 需要由 Store、Runtime 和 Planner 共同推断。`priority.ProbeEvidence` 同时暴露：

```text
ObservedAt
Freshness
ProbeStatus
Status
EvidenceFresh
```

成功证据、失败证据、缓存证据、本轮证据和历史预测证据没有通过单一 interface 表达，而是依赖多个字段组合和调用顺序。

### 精确证据

#### 2.1 Planner 对成功与失败证据使用不同的新鲜度条件

Planner 判断成功证据时检查：

```text
EvidenceFresh == true
Freshness == Fresh
ProbeStatus == Ready
Status == Ready
```

但是 `freshEvidenceByAuthIndex` 也会接纳 `Status == ProbeFailed` 的证据，而没有对失败证据执行同样完整的 current-round 检查。随后 `initialItems` 会将该失败证据标记为 `EvidenceFresh = true` 并生成禁用决策。

#### 2.2 Store 把缓存状态转换成看似新鲜的证据

`state.GetCachedEvidence` 从成功缓存构造 `priority.ProbeEvidence` 时，会设置 `FreshnessFresh`、`ProbeStatusReady` 与 `EvidenceFresh = true`。

但缓存存在并不自动等于“当前调度轮次的 Fresh Evidence”。缓存是否可用于只读展示、预测或 Host 写回，需要调用者进一步判断。

#### 2.3 Runtime 私有过滤函数承担隐藏门禁

生产调度通过 `currentRoundEvidence` 检查：

```go
item.ObservedAt.Equal(observedAt)
```

以筛选本轮证据。该过滤保护了生产 Apply，但它是 Runtime 私有知识，没有体现在 Planner 或 Store 的 interface 中。

#### 2.4 不同 Planner 调用者没有统一执行过滤

`runProductionTask` 在实际规划前调用 `currentRoundEvidence`。

`SyncHost` 和 `ResetAllPriorities` 在生成快照时，则直接执行：

```text
store.BuildGroupEvidence
→ priority.PlanFreshOnly
```

这些路径目前主要生成只读快照，没有直接应用该 Plan，但旧失败缓存可能被展示为当前失败或目标禁用。更重要的是，未来新增调用者必须知道并复制这条隐藏门禁，否则可能把历史缓存升级为可写 Fresh Evidence。

### 为什么需要深化

Fresh Evidence 决定 Plan 是否能够产生 Host 写回，是调度系统的安全门禁。调用者不应通过拼接五个状态字段和比较时间戳来恢复这一领域概念。

如果每个调用者必须知道缓存来源、观察时间、探测结果和预测用途，当前 evidence interface 就过于复杂，且容易误用。

### Deletion test

如果删除 `currentRoundEvidence`，复杂度不会消失，而会立刻散回所有 Planner 调用者：每个调用者都需要重新实现观察时间、成功状态和失败状态过滤。

这说明“本轮可写证据”和“历史只读证据”的选择值得集中到现有 evidence module 中。该方向不需要增加 adapter，应通过深化 in-process implementation 获得 depth。

### 深化方向

让 evidence 生成与选择 module 成为 Fresh Evidence 的唯一权威，在内部吸收：

- `ObservedAt` 与当前调度轮次的关系；
- 成功和失败探测的 provenance；
- 缓存观察与 Fresh Evidence 的差异；
- 可用于 Host 写回的证据；
- 只用于历史预测或诊断展示的证据。

调用者只表达用途，不再自行解释多个 freshness 字段。具体 interface 在后续设计阶段确定。

### 优化目的

- 让 Fresh Evidence 定义只有一个 implementation；
- 防止缓存证据被意外升级为可写证据；
- 消除 Runtime 调用者对时间戳和状态组合的重复判断；
- 让 Planner 接收语义清晰且来源明确的输入；
- 让证据规则通过一个稳定 interface 完整测试。

### 预期结果

- Apply、Sync、Reset 和双 Model Group 预测共享一致的 evidence 语义；
- 旧 `ProbeFailed` 不会被误认为当前轮次失败；
- 新增 Planner 调用者不需要复制 `currentRoundEvidence`；
- Planner 不再依赖调用者正确拼装五个 freshness 字段；
- 测试可直接覆盖当前轮成功、当前轮失败、过期成功、过期失败和历史预测；
- `CONTEXT.md` 中 Fresh Evidence 的领域定义在代码中拥有单一权威。

### 风险与约束

- 不得把缓存预测与 Fresh Evidence 混为同一写入资格；
- 需要保留 alternate Model Group 的只读预测能力；
- 必须明确失败证据的 current-round 判定，不得让历史失败触发新的 Host 禁用；
- 不应新增只有一个 adapter 的 port；
- 不得破坏 Planner 纯函数属性。

### ADR 关系

本提案不与现有 ADR 冲突，并强化：

- `CONTEXT.md` 中 Fresh Evidence 的定义；
- ADR-0003 对 Soft Depletion、Hard Depletion 和写回条件的约束；
- 第一轮状态引擎深化提案，但不重复其缓存与采样职责。

---

## 3. 深化双 Model Group 规划投影

- **状态**：`待讨论 (Proposed)`
- **推荐强度**：`Strong`
- **依赖类别**：`in-process`
- **建议优先级**：`P2`
- **涉及文件**：
  - `internal/runtime/production_runner.go`
  - `internal/runtime/runtime.go`
  - `internal/apply/audit.go`
  - `internal/priority/planner.go`

### 目前的痛点

Antigravity 的 `gemini` 与 `claude_gpt` 是两个独立 Model Group。系统使用配置指定的主控 Model Group 产生实际写回，同时为 alternate Model Group 生成只读预测。

这一完整投影流程目前在多个 Runtime 用例中重复：

```text
读取主控组 evidence
→ PlanFreshOnly
→ Snapshot

计算 alternate Model Group
→ 读取 alternate evidence
→ PlanFreshOnly
→ SnapshotPredicted

→ NewDualGroupSnapshot
```

### 精确证据

#### 3.1 生产调度重复装配双组投影

`runProductionTask` 分别处理主控组与 alternate 组的 evidence、Plan 和 Snapshot，再调用 `NewDualGroupSnapshot`。

#### 3.2 SyncHost 重复相同知识

`SyncHost` 再次执行主控组和 alternate 组的：

```text
BuildGroupEvidence
→ PlanFreshOnly
→ Snapshot / SnapshotPredicted
→ NewDualGroupSnapshot
```

#### 3.3 ResetAllPriorities 第三次重复

Reset 完成 Host 状态变更后，为了更新管理页快照，再次执行同样的双组投影。

#### 3.4 LatestSnapshot fallback 维护另一套结果构造规则

没有最新双组快照时，`LatestSnapshot` 自行创建一个包含 legacy 单组结果和空 predicted Snapshot 的 fallback。

因此每个调用者都必须知道：

- 主控 Model Group 的来源；
- alternate Model Group 的计算；
- 哪个 Plan 可写；
- 哪个 Snapshot 必须标记为 predicted；
- cooldown options 与 observation time 的传递；
- fallback 的结果形状。

### 为什么需要深化

这不是单纯的代码重复，而是一组必须跨所有调用者保持一致的领域规则。

任何预测语义、cooldown 传播或 Snapshot 结构调整，都需要同时修改 Production、Sync、Reset 和 fallback。遗漏任一路径就可能让管理页在不同操作后展示不同语义。

当前测试只能通过大型 Runtime 测试准备 Host inventory、Store、Clock 与两组 evidence，纯投影逻辑没有独立测试表面。

### Deletion test

删除任意一个重复投影块不会消除复杂度，只会迫使该调用者重新实现主控组、alternate 组、Planner 和 Snapshot 规则。

如果将双 Model Group 投影深化为 Runtime 内部 module，再删除它，四条调用路径都必须重建完整算法。这证明该 module 能产生真实 leverage。

### 深化方向

将以下知识集中到 Runtime 内部的 deep module：

- 主控与 alternate Model Group 的选择；
- 两组 evidence 到两组 Plan 的映射；
- 主控结果与 predicted 结果的角色；
- cooldown options 与 observation time；
- `DualGroupSnapshot` 的统一构造与空状态。

该 module 只负责 in-process 投影，不执行 Host 写回或外部网络请求。Planner 继续保持无副作用纯函数，Runtime 继续拥有调度用例所有权。

具体 interface 在后续设计阶段确定。

### 优化目的

- 让双 Model Group 的投影语义只有一个 implementation；
- 防止 Production、Sync、Reset 和 fallback 发生行为漂移；
- 把纯计算从大型 Runtime 用例中提炼为稳定测试表面；
- 降低未来修改 predicted 规则的影响范围；
- 保持主控组可写、alternate 组只读的明确契约。

### 预期结果

- 所有 Runtime 用例产生一致的 `DualGroupSnapshot`；
- 调用者不再自行计算 alternate Model Group；
- 不会出现某一路径遗漏 `SnapshotPredicted` 标记；
- 可以通过纯输入测试主控组与预测组，不需要完整 Host 和磁盘设置；
- Runtime 用例更专注于执行顺序，而不是重复数据投影；
- 如果未来增加新的只读展示规则，修改位置保持集中。

### 风险与约束

- 不得让 predicted Plan 进入 Host 写回；
- 不得把网络探测或 Store 持久化吸收到纯投影 implementation；
- 不得移动 Runtime 对 ManualApply、AutoApply、Probe、SyncHost 与 Reset 的用例所有权；
- 不得让 Planner 获得副作用；
- 必须和 Fresh Evidence 深化方向协调，避免新 module 再次解释 freshness 字段。

### ADR 关系

本提案不与现有 ADR 冲突，并符合：

- ADR-0001 的 Antigravity 单提供商专精化；
- `CONTEXT.md` 中 `gemini` 与 `claude_gpt` Model Group 的领域定义；
- AGENTS.md 中 Runtime 对调度用例的唯一所有权。

---

## 4. 按用户功能收拢管理页资产

- **状态**：`待讨论 (Proposed)`
- **推荐强度**：`Worth exploring`
- **依赖类别**：`in-process`
- **建议优先级**：`P3`
- **涉及文件**：
  - `internal/management/templates.go`
  - `internal/management/template_scripts.go`
  - `internal/management/template_styles.go`
  - `internal/management/template_*.go`
  - `internal/management/templates_test.go`

### 目前的痛点

近期提交将原本数千行的嵌入式 CSS 和 JavaScript 大字符串拆分到约 27 个文件。这解决了单文件长度问题，但拆分主要沿技术类型展开：HTML、CSS、JavaScript 和 i18n 分别存放。

因此，一个用户功能仍然横跨多个 module。例如配置中心相关知识分布在：

```text
TemplateConfig
templateScriptConfig
templateStyleConfig
templateScriptI18N
公共 controls
公共 modal
boot 初始化
```

文件变短了，但完成一次功能修改仍需跨多个位置恢复完整上下文。

### 精确证据

#### 4.1 JavaScript 片段依赖手工连接顺序

`template_scripts.go` 明确按 dependency order 连接多个私有片段。片段之间通过全局函数、全局变量和 DOM ID 形成隐式 interface。

HTML 中的内联调用，例如：

```html
onclick="openKeyModal()"
onclick="toggleLanguage()"
onclick="toggleTheme()"
onclick="switchTab('overview')"
```

要求相关函数必须存在于最终全局作用域，且初始化顺序正确。该 interface 没有被类型或装配结构显式表达。

#### 4.2 CSS 片段依赖手工 cascade order

`template_styles.go` 按固定顺序连接 token、shell、controls、overview、history、diagnostics、config、overlays 与 responsive 片段。

改变连接顺序可能改变选择器覆盖结果，因此文件顺序本身成为调用者必须维护的隐式知识。

#### 4.3 一个用户功能跨越多种技术文件

配置、诊断、概览等功能分别横跨 markup、style、behaviour 与语言资源。维护者或 AI 为理解一个按钮，需要打开多个文件并重建全局依赖。

#### 4.4 测试主要检查源码字符串

`templates_test.go` 当前主要使用：

- `strings.Contains` 检查元素或源码片段；
- 正则检查外部 URL；
- 正则检查 CSS token；
- 特定 JavaScript 源码 substring 检查行为要求。

这些测试能够发现资源遗漏，但不能可靠验证：

- JavaScript 是否可以解析和执行；
- 全局函数是否在调用时存在；
- DOM ID 与选择器是否匹配；
- i18n key 是否完整；
- 用户操作是否产生预期 DOM 行为；
- 片段顺序变化是否破坏运行时行为。

测试越过最终资源 interface，直接依赖 implementation 文本。行为不变但写法变化时测试可能失败；源码仍在但实际行为失效时测试可能继续通过。

### 为什么需要深化

管理页的实际维护单位是用户功能，而不是“所有 CSS”或“所有 JavaScript”。

理想 locality 应让一个功能的 markup、style、behaviour 与语言资源彼此相邻，同时把装配顺序和共享运行时细节隐藏在 implementation 内。

当前拆分改善了文件长度，但 27 个片段本身大多没有独立行为或稳定 interface。它们依赖最终字符串连接和全局命名才能工作，属于 shallow module。

### Deletion test

删除任意单个片段文件并把原字符串搬回聚合文件，行为复杂度不会集中或消失，说明单个片段没有形成显著 depth。

如果按用户功能形成 deep module，再删除其中一个功能 module，该功能的 markup、style、behaviour、文案、装配和验证知识会重新散回多个位置。这说明按用户功能集中可以获得更强 locality。

### 深化方向

保留最终 `StatusHTML` 的小 interface，但按用户功能集中管理页 implementation，例如从概念上形成：

```text
Overview 功能
History 功能
Diagnostics 功能
Config 功能
Modal 功能
```

每个功能内部收拢相关 markup、style、behaviour 和 i18n。最终装配仍生成单个原生、自包含的 `StatusHTML`。

测试表面应逐步从源码 substring 转向最终资源的可执行行为，包括脚本解析、DOM 依赖、i18n 完整性和核心用户操作。

具体文件组织和 interface 在后续设计阶段确定。本提案不主张把所有资产重新合并成一个巨型文件。

### 优化目的

- 让一次用户功能修改保持 locality；
- 隐藏全局函数、DOM ID、脚本顺序和 CSS 顺序等装配知识；
- 减少维护者与 AI 理解一个功能时需要跨越的文件数量；
- 保持 `StatusHTML` 的小 interface 和自包含交付方式；
- 让最终浏览器行为成为主要测试表面。

### 预期结果

- 修改配置、诊断或概览功能时，相关知识集中在同一 module；
- JavaScript 全局命名和装配顺序的泄漏减少；
- CSS cascade order 更集中、更容易验证；
- 删除一个功能时可同时删除其 markup、style、behaviour 与文案；
- 测试能够发现脚本解析、DOM 依赖和真实交互故障；
- `StatusHTML` 继续作为管理页唯一外部 interface；
- 管理页继续满足 CPA Host 的嵌入式加载要求。

### 风险与约束

- 必须保持纯原生、自包含、运行时零外部 CDN；
- 必须继续兼容 CPA Host 暗色和明亮主题；
- 不得引入前端构建链作为 CPA 运行时依赖；
- 不应退回单个数千行巨型字符串；
- 功能 module 之间的共享代码必须通过 deletion test，避免制造新的 shallow module；
- 浏览器行为测试应控制复杂度，避免让测试工具链超过管理页 implementation 本身。

### ADR 关系

本提案不与现有 ADR 冲突。实施时必须严格保持 AGENTS.md 中 Web UI 的约束：

- 纯前端原生实现；
- 自包含；
- 零外部 CDN；
- 深度适配 CPA Host 暗色与明亮主题。

---

## 5. 推荐顺序与组合关系

### 推荐实施顺序

| 顺序 | 提案 | 强度 | 主要原因 |
|---|---|---|---|
| 1 | 统一 Host transition 与执行记录 | Strong | 直接影响 Host 状态、失败表达和审计可信度 |
| 2 | 收拢 Fresh Evidence 权威归属 | Strong | 决定证据是否具备 Host 写回资格，是领域安全门禁 |
| 3 | 深化双 Model Group 规划投影 | Strong | 消除重复领域编排并建立纯计算测试表面 |
| 4 | 按用户功能收拢管理页资产 | Worth exploring | 主要改善维护 locality 与行为测试，当前生产风险较低 |

### 三个调度候选的组合关系

前三项可以形成一条职责清晰的执行链：

```text
Evidence module
  │  只输出用途明确的当前轮或历史证据
  ▼
Dual Model Group projection module
  │  统一主控组与 predicted 组投影
  ▼
priority.Planner
  │  无副作用纯函数，只输出不可变 Plan
  ▼
Host transition / execution record module
  │  执行门禁、Host transition 与脱敏记录
  ▼
CPA Host + persisted execution history
```

该组合保持现有架构所有权：

- Domain Core 负责证据语义和纯规划；
- Apply Layer 负责 Host transition 与脱敏执行记录；
- Runtime 负责调度用例、Ticker 与 `runMu` 单飞；
- Management 与 CGO adapter 保持薄透传。

### 进入实施前需要确认的问题

在任何候选进入具体设计或实现前，需要分别确认：

1. 哪些行为属于目标 module 的外部 interface，哪些只应成为内部 seam；
2. 是否存在至少两种真实 adapter，还是只需要 in-process implementation；
3. 哪些现有测试应迁移到新的 interface 测试表面，哪些 shallow module 测试应删除；
4. 部分 Host transition、失败证据与 predicted Plan 的精确语义；
5. 是否需要新增或更新 `CONTEXT.md` 领域术语；
6. 是否有 load-bearing 决策需要记录为新的 ADR。

在上述问题完成讨论前，本文档不定义具体 interface，也不授权实现代码变更。
