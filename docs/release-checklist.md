# Antigravity Priority 发布审阅与版本迭代检查规范 (Release & Audit Checklist)

本文档定义了 `antigravity-priority` 插件在每次版本迭代、代码提交与正式发布前的**标准化审阅流程与变更检查矩阵**。每次完成功能开发或缺陷修复后，在提交代码与发布 Release 前，**必须严格依照本清单逐项自检**，确保所有关联文件与元数据变更到位。

---

## 目录
1. [版本号同步一致性矩阵](#1-版本号同步一致性矩阵)
2. [文档与双语同步审阅清单](#2-文档与双语同步审阅清单)
3. [CPA 宿主与插件架构契约审阅](#3-cpa-宿主与插件架构契约审阅)
4. [代码质量与测试验证矩阵](#4-代码质量与测试验证矩阵)
5. [标准化发布工作流步骤](#5-标准化发布工作流步骤)

---

## 1. 版本号同步一致性矩阵

当进行版本升级（如 `v1.0.1` $\to$ `v1.1.0`）时，**必须确保以下 4 个位置的版本号完全一致**：

| 检查项 | 目标文件 | 关键位置 / 字段 | 审阅标准 |
| :--- | :--- | :--- | :--- |
| **① 插件注册表** | `registry.json` | `plugins[0].version` | 必须为纯三段式版本号，如 `"1.1.0"`（无 `v` 前缀） |
| **② 运行时元数据** | `internal/runtime/runtime.go` | `buildMetadata().Version` | 必须与 `registry.json` 完全一致，如 `"1.1.0"` |
| **③ 嵌入式 Web 仪表盘** | `internal/management/feature_shell_assets.go` | `<span class="version-badge">v1.1.0</span>` | 必须带有 `v` 前缀，如 `v1.1.0` |
| **④ 专属 Release Note** | `.github/release-notes/vX.Y.Z.md` | 文件名与一级标题 `# antigravity-priority vX.Y.Z` | 文件名必须为 `vX.Y.Z.md`，内容双语齐备 |

---

## 2. 文档与双语同步审阅清单

用户面向的公开文档应保持专业清晰、**聚焦核心功能价值**，遵循以下规范：

1. **核心功能聚焦原则**：
   - 概览仅保留高阶核心能力（如 Antigravity 双窗口调度、Dynamic Boost 提权、Weekly Urgency 轮转、自适应学习率、自愈式软降级、UI 动态配置中心、双主题仪表盘）；
   - **严禁过度展开技术细节**（无需单独罗列如“复用 CPA 宿主链路”、“新鲜证据门禁”、“两阶段纯内存计算”等开发层面的细节优化）。
2. **纯净化原则（无内部研发代号）**：
   - **严禁在 `README.md`、`README.en.md` 和 Release Notes 中暴露 `(REQ-xx)` 内部研发标签**。

| 文档 | 审阅重点 | 检查要点 |
| :--- | :--- | :--- |
| **`README.md`** (中文) | 功能概览、极简配置、管理 API | - 聚焦核心功能，无细碎实现堆砌；<br>- **严禁带有 `(REQ-xx)` 内部标签**；<br>- 配置说明准确体现“极简 YAML + Web 配置中心”模式；<br>- 接口列表与路由变更保持同步。 |
| **`README.en.md`** (英文) | 英文对照完整性 | - 中英文段落结构 1:1 对齐；<br>- 英文表达地道准确；<br>- **严禁带有 `(REQ-xx)` 内部标签**。 |
| **`.github/release-notes/vX.Y.Z.md`** | 双语 Release Note | - 包含 `### 中文` 与 `### English` 两大章节；<br>- 重点分模块梳理核心功能升级、性能优化、体验改善；<br>- **严禁带有 `(REQ-xx)` 内部标签**。 |
| **`docs/requirements/`** | 需求与技术规格路线图 | - 状态从 `待实施 (Ready)` 更新为 `已完成 (Completed)`；<br>- 涵盖需求规格、领域模型与架构设计（内部研发文档可保留 REQ 编号）。 |

---

## 3. CPA 宿主与插件架构契约审阅

| 审阅维度 | 架构原理与检查标准 | 达标标准 |
| :--- | :--- | :--- |
| **宿主配置极简** | CPA 宿主 `config.yaml` 仅作为基础加载开关，不承载日常业务参数。 | 用户 YAML 仅需 `enabled: true`，物理路径 `state_cache_path` 默认缺省为 `data/antigravity-priority-cache.json`。 |
| **元数据干净度 (`buildMetadata`)** | CPA 官方管理端根据 `Metadata.ConfigFields` 渲染抽屉表单，用户在抽屉保存会写回 `config.yaml` 导致 CPA 强制重载。为了让配置中心独占管理业务参数，**`ConfigFields` 保持为 `nil`**（方案 A 极简无扰）。 | `buildMetadata()` 中不包含 `ConfigFields`，避免产生表单冲突与意外重载。 |
| **分层配置底座 (`config.Default()`)** | `config.Default()` 是系统冷启动、无缓存文件时的基准兜底以及 UI 恢复默认值的**单一真实来源 (Single Source of Truth)**。 | `Default()` 返回的默认结构（15m, gemini, 6, 1, 999, 100）必须与推荐配置完全一致。 |
| **动态配置持久化与热重载** | 业务与调度参数保存至 `data/antigravity-priority-cache.json` 的 `app_config` 节点，修改后立即热重设 Ticker Worker。 | 动态配置持久化存盘 (`SaveAtomic`)，启动与 Reconfigure 时自动合并持久化配置，0 秒免重启热生效。 |

---

## 4. 代码质量与测试验证矩阵

提交前必须在本地终端执行以下全套验证命令，确保 **100% 通过且零告警**：

```bash
# 1. 编译自检（确保所有包与子模块无语法/类型错误）
go build ./...

# 2. 静态代码分析（确保无遗漏变量、无废弃调用）
go vet ./...

# 3. 完整单元测试（包括文档完整性、注册表合规性、模板合规性测试）
go test -v ./...

# 4. 数据竞争检测（确保并发调度、探针协程池与缓存读写零 race condition）
go test -race ./...
```

### 关键测试项说明
- **`TestRegistryJSON_SchemaValidation`**：校验 `registry.json` 符合 CPA 插件商店标准。
- **`TestDocumentation_BilingualCompleteness`**：校验 `README.md` 与 `README.en.md` 存在且双语完整。
- **`TestStatusHTML_NoRemoteURLs`**：确保 `StatusHTML` 零外部 CDN、零远程字体/脚本，100% 离线自包含，符合严格 CSP。
- **`TestStatusHTML_CSSTokenCompleteness`**：确保明亮/暗色/羊皮纸 3 种主题变量完整闭环。

---

## 5. 标准化发布工作流步骤

```text
[功能开发 / 需求迭代]
         │
         ▼
[1. 关联代码与文档更新]
   ├─ 更新业务代码与单元测试 (TDD)
   ├─ 检查版本号一致性 (registry.json, runtime.go, feature_shell_assets.go)
   ├─ 编写 .github/release-notes/vX.Y.Z.md (双语、无 REQ 标签、核心功能)
   └─ 同步更新 README.md 与 README.en.md (高度凝练、无技术细节堆砌)
         │
         ▼
[2. 执行发布前质量自检]
   ├─ go build ./...
   ├─ go vet ./...
   ├─ go test -v ./...
   └─ go test -race ./...
         │
         ▼
[3. 用户确认 (安全底线)]
   └─ "Never commit without explicit user confirmation during the conversation"
         │
         ▼
[4. Git Commit 提交与打标]
   ├─ Commit 消息遵循 Conventional Commits 规范 (如 chore: bump version to v1.1.0)
   └─ 创建 Git Tag: git tag vX.Y.Z
         │
         ▼
[5. GitHub Release 自动化发布]
   └─ CI 自动交叉编译生成各平台动态库产物 (.so / .dylib / .dll)
```
