<div align="center">

# Antigravity Smart Priority (`antigravity-priority`)

[中文](./README.md) | [English](./README.en.md)

</div>

CLIProxyAPI (CPA) 专精型 **Google Antigravity 凭证智能配额调度与自适应提权插件**。插件 ID、动态库基础名与 CPA 配置键均为 `antigravity-priority`。

---

## 导航

- [功能概览](#功能概览)
- [工作流程](#工作流程)
- [构建与安装](#构建与安装)
- [插件商店来源](#插件商店来源)
- [配置说明](#配置说明)
- [管理页面与接口](#管理页面与接口)
- [许可证](#许可证)

---

## 功能概览

- **Antigravity 三大配额池专精调度**：针对 `gemini`、`argon` 与 `claude_gpt` 三大配额池深度定制，联动 5 小时短窗与 7 天周长窗；自动识别 `ULTRA` / `ULTRA-LITE` / `PRO` / `FREE` 订阅档位，并将无短窗的 Free 凭证隔离至独立兜底梯队。
- **CPA 原生 `scheduler.pick` 纯内存动态调度**：通过 `scheduler: true` 与 `scheduler_across_priorities: true` 毫秒级解析模型并路由至最优凭证，实现零磁盘写入、无静态优先级冲突的纯内存调度。
- **动态提前提权与周紧迫度平衡**：基于 `usage.handle` 实时估算 Token 消耗与自适应周期燃尽率 $C_{\text{cycle}}$，在最佳窗口触发 `999, 998...` 梯次提权并结合周紧迫度平滑轮转，避免大额度账号到期浪费。
- **会话粘性与自定义梯队**：同梯队凭证支持 `canonical_session_id` 会话粘性与轮询分流；支持在卡片上覆盖自定义梯队（Tier #1、Tier #2 等）实现精确的优先次序与负载均衡。
- **自愈式软降级与健康故障转移**：短窗或周额度耗尽时仅执行纯内存软降级（优先级 `-1`，不修改宿主机 `disabled` 状态），配额重置后自动恢复；对 401/403 凭证异常与 429/5xx 限流故障执行隔离与退避探测恢复。
- **智能配额预热（5h 短窗 & Pro Argon 周额度）**：支持为满额且未激活倒计时的 Gemini / Claude 5h 短窗以及 Pro Argon 周额度自动发送轻量预热请求，配合全局错峰车道打散刷新时间点，并支持单卡局部覆盖。
- **自适应探测校准**：根据近期请求频率与突发流量动态调整后台探测节奏，空闲跳过、活跃校准，在减少无效探测的同时保持配额与灵敏度模型准确。
- **模型路由看板与全局排除**：提供具体模型路由看板与五进度条凭证卡片（`Gemini 5h/7d`、`Claude 5h/7d`、`Argon 7d`），支持一键全局排除指定模型并即时热生效。
- **系统提示词 Prompt Guard**：支持从远程词库同步规则并本地自定义关键词/正则改写（仅作用于系统与开发者指令），可在配置中心预览与管理（默认关闭）。
- **零重启配置中心与双主题仪表盘**：CPA 宿主 YAML 仅需启用插件，所有调度与业务参数均可在嵌入式 CSP 安全仪表盘的 **`⚙️ 配置中心`** 实时热生效，同时提供实时请求历史、完整身份展示与一键脱敏视图。
---

## 工作流程

```text
加载插件
  -> 读取 plugins.configs.antigravity-priority 配置
  -> 注册 scheduler.pick 与 management 能力 (scheduler_across_priorities: true)
  -> 通过 host.auth.list 与 host.auth.models 获取 CPA 凭证与模型列表
  -> 定期探测与初始化
       - 单次探测并发提取三大配额池 (gemini, argon, claude_gpt) 的 5h 与 7d 双窗口数据
       - 结合自适应 C_cycle 学习率判定各配额池是否进入 Dynamic Boost 提权区间
       - 预先计算三大配额池的高速内存路由表 (O(1) 查找)
  -> 接收业务请求 (scheduler.pick)
       - 解析请求模型 (如 gemini-3.8-flash-high, gemini-4-argon-high, claude-sonnet-5-5-high)
       - 映射到目标配额池 (gemini / argon / claude_gpt)
       - 过滤耗尽与 429 冷却凭证，选取最优优先级梯队 (999 Boost > 100 Normal > 10 Free)
       - 在最优平级梯队内结合 canonical_session_id 会话粘性与轮询指针分流
       - 纯内存极速返回目标 auth_id，不产生任何磁盘文件 IO
  -> 闭环实时追踪 (usage.handle)
       - 捕获各轮交互消耗 token，实时推算额度扣减，更新路由分档
       - 下一次后台探测时平滑自校准，消除误差
  -> 在认证管理页面展示具体模型路由看板、调用顺序梯队、5 进度条卡片与脱敏审计摘要
```

---

## 构建与安装

插件以 CGO 动态库形式运行，宿主会从动态库文件名去掉扩展名得到插件 ID，因此文件名必须保持为 `antigravity-priority.<ext>`。

### 本地编译
```bash
# Linux / macOS
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.so .

# Windows (MSYS2 / MinGW)
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.dll .
```

### 部署到 CPA
把产物放入 CPA 插件发现目录之一：
- `plugins/<GOOS>/<GOARCH>/antigravity-priority.<ext>`
- `plugins/<GOOS>/<GOARCH>-<variant>/antigravity-priority.<ext>`
- `plugins/antigravity-priority.<ext>`

扩展名：Linux/FreeBSD 为 `.so`，macOS 为 `.dylib`，Windows 为 `.dll`。

---

## 插件商店来源

如需通过 CPA 插件商店安装本插件，第三方来源必须指向 `registry.json` 的原始 JSON 文本：

```yaml
plugins:
  enabled: true
  store-sources:
    - "https://raw.githubusercontent.com/ygq-future/antigravity-priority/main/registry.json"
```

> **注意**：不要使用包含 `/blob/` 的 GitHub 网页地址。修改 `store-sources` 后，重启 CPA 或通过管理端重新加载配置，再刷新插件商店列表即可一键安装。

---

## 配置说明

### 1. CPA 宿主极简配置 (`config.yaml`)

从 v1.1.0 起，推荐在 CPA `config.yaml` 中仅保留最干净的插件启用开关，所有业务参数均可在 Web 仪表盘的 **`⚙️ 配置中心`** 可视化调节：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    antigravity-priority:
      enabled: true
      # state_cache_path: "data/antigravity-priority/state.json" # 可选，自定义持久化缓存路径
```

| 宿主字段 | 默认值 | 必填 | 说明 |
| :--- | :--- | :--- | :--- |
| **`enabled`** | `true` | 是 | 插件全局启用开关。设为 `false` 时彻底停止调度与后台任务。 |
| **`state_cache_path`** | `data/antigravity-priority/state.json` | 否 | 主状态文件路径，默认收纳于 `data/antigravity-priority/` 目录。同目录自动派生 `runtime.json`、`runs.json`、`requests.json`，分别承载运行配置、运行审计与请求历史，避免高频写入互相影响。 |

> **⚠️ 路径迁移提示**：旧版默认路径 `data/antigravity-priority-cache.json` 会在启动时自动迁移到 `data/antigravity-priority/` 目录，并在四份新文件全部落盘后删除旧文件，不留残留。若手动把 `state_cache_path` 改到全新路径，插件不会跨路径热迁移；请停止 CPA / 插件后将整个缓存目录移动或复制到新位置。

### 2. UI 动态配置中心 (免重启热生效)

在管理面板的 **`⚙️ 配置中心`** 标签页中，可随时在线配置并立即热生效以下选项：

| 配置项 | 默认值 | 范围/选项 | 说明 |
| :--- | :--- | :--- | :--- |
| **自适应配额校准 (`auto_apply`)** | `false` | 开 / 关 | 是否由后台按请求频率自适应向 Google 发起权威配额探测：空闲跳过、低频自动延后、高频在周期上限内完成校准。 |
| **高频校准周期上限 (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, 自定义 | 高频使用时的最大校准间隔（含随机抖动）；低频自动拉长至 2–6 小时，空闲不触发。修改后立即热生效。 |
| **最大探测并发数 (`max_concurrency`)** | `6` | `1 ~ 32` | 向 Google 配额接口发起并发探测的最大协程数。 |
| **紧迫度分档容差 (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | 紧迫度差距在此容差内的账号分配相同优先级整数进行平级轮询。 |
| **自适应时序样本容量 (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO 保留的历史探测样本数，同时用于消耗趋势查看与燃尽率学习。 |
| **429 熔断冷却时长 (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` 分钟 | 遭遇 429 限流时将受限配额池临时置入冷却队列避让，到期自动自愈。 |
| **Prompt Guard (`prompt_guard`)** | 关闭 | 配置中心 | 为路由至 Antigravity 的请求管理系统/开发者指令改写规则，规则来自远程词库差分同步，支持自定义短语或正则规则（上限 200 条）与预览；默认关闭。 |
| **Gemini 5h 满额自动预热 (`warmup.gemini_enabled`)** | `false` | 开 / 关 | 当 Gemini 5h 短窗 100% 满额、7d 仍有余量且倒计时未激活时，自动发送一次长回答请求（末尾追加不少于 600 token 的约束）提前触发 5h 倒计时；凭证之间随机间隔 60–120 分钟、同凭证不同分组 30–60 分钟。 |
| **Gemini 预热目标模型 (`warmup.gemini_model`)** | `gemini-3-flash` | 可选 / 自定义 | 用于触发 Gemini 5h 重置倒计时的预热模型。 |
| **Claude 5h 满额自动预热 (`warmup.claude_enabled`)** | `false` | 开 / 关 | 当 Claude 5h 短窗 100% 满额、7d 仍有余量且倒计时未激活时，自动发送一次短请求提前触发 5h 倒计时；凭证之间随机间隔 60–120 分钟、同凭证不同分组 30–60 分钟。 |
| **Claude 预热目标模型 (`warmup.claude_model`)** | `claude-sonnet-5-5-high` | 可选 / 自定义 | 用于触发 Claude 5h 重置倒计时的预热模型。 |
| **Argon 预热目标模型 (`warmup.argon_model`)** | `gemini-4-argon-high` | 仅此选项 | 用于 Pro 凭证独立周额度预热。 |
| **单凭证预热覆盖 (`warmup.overrides`)** | 空 | 概览卡片顶部图标 | 点击图标写入 `auth_index -> true/false` 的局部状态；点击结果与全局一致时自动清除并恢复跟随；局部开启会清除该凭证 4h30m 防抖。 |

---

## 管理页面与接口

插件通过 `management.register` 分别向 CPA 宿主注册 **resources**（静态管理仪表盘）与 **routes**（动态管理 API）。

### 资源页面（静态 Web UI）

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **访问方式**：在 CPA 管理后台侧边栏点击 **"Antigravity Priority"** 菜单，或在浏览器中直接访问 `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status`。
  - **核心功能**：
    - **模型驱动路由看板**：支持下拉选择具体模型（Gemini 池、Argon 池、Claude & GPT 池），即时展示对应模型的完整凭证调用次序（Tier #1 · 999 提权、Tier #2 · 100 平级轮询等）与生效状态；模型行可直接排除/恢复该模型，选择结果自动记忆。
    - **全景五进度条卡片**：每个凭证卡片同时渲染全部 5 根配额进度条（Gemini 5h/7d、Claude 5h/7d、Argon 7d），并高亮当前选中模型的生效配额；秒级自适应倒计时与自适应消耗速率 $C_{\text{cycle}}$。
    - **调度与同步控制**：提供 `🔄 刷新同步 (Sync Host)`、`📡 刷新配额 (10s冷却)`、`🔄 重置静态优先级 (Reset Priorities)`。
    - **实时请求历史**：实时追踪捕获最近请求记录（保留条数可配置，默认 100，最高 1000），记录请求 Model、目标配额池、Token 消耗细分与状态码，支持一键清空。
    - **采样历史**：支持自由切换 Gemini、Argon 与 Claude & GPT 三大配额池时序样本。
    - **系统诊断**：运行健康横幅、调度/隔离冷却/最近运行三类 KPI、三配额池矩阵、逐凭证调度决策解释（优先级、Urgency、T7d/T_req、C_cycle 与原因）、429 风暴与冷却隔离、运行时间线，以及四类缓存文件的路径、体积与缺失状态，支持一键复制诊断 JSON。
    - **⚙️ 配置中心**：在线修改所有调度与算法参数，0 秒热生效与一键恢复默认。

### 管理 API（动态接口，需 Management Key 鉴权）

- `POST /v0/management/plugins/antigravity-priority/run?mode=probe`
  - 触发一次向 Google API 的全量配额探测并更新本地缓存与快照，仅校准内存路由表，**不写回 CPA 宿主**。
- `GET /v0/management/plugins/antigravity-priority/runtime-config`
  - 获取当前完整运行时配置。
- `POST /v0/management/plugins/antigravity-priority/runtime-config`
  - 提交更新运行时配置并热生效。
- `GET /v0/management/plugins/antigravity-priority/schedule/config`
  - 获取自动调度暂停状态。
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - 动态切换自动调度暂停/恢复状态。
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - 导出调度器状态、计划快照、冷却与 429 风暴、运行历史、当前策略与四类缓存文件状态；账号身份使用完整 email，令牌、密钥及持久化审计字段保持脱敏。
- `POST /v0/management/plugins/antigravity-priority/sync`
  - 主动从 CPA 宿主同步最新凭证文件列表并即时重新生成三大配额池快照。
- `GET /v0/management/plugins/antigravity-priority/samples?auth_index=xxx`
  - 获取指定凭证在各模型组下的历史滑动窗口时序采样数据。
- `GET /v0/management/plugins/antigravity-priority/samples?probe_round_id=xxx&model_group=gemini|argon|claude_gpt`
  - 获取指定探测轮次和模型组内实际新增的额度样本；额度未变化的凭证不会出现在结果中。
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - 获取最近一次三大配额池决策规划快照（`DualGroupSnapshot`）；响应保留完整 CPA email 和供 API 关联使用的技术 `auth_index`，管理页仅以 email 作为账号名称，敏感令牌与审计字段保持脱敏。
- `GET /v0/management/plugins/antigravity-priority/request-history`
  - 获取最近 100 条实时请求调用历史列表。
- `POST /v0/management/plugins/antigravity-priority/request-history/clear`
  - 清空当前保存的请求调用历史。
- `GET /v0/management/plugins/antigravity-priority/custom-tier`
  - 获取已配置的凭证自定义梯队覆盖字典。
- `POST /v0/management/plugins/antigravity-priority/custom-tier`
  - 配置或清除指定凭证的自定义梯队（Tier #1, Tier #2...，设置为 0 表示恢复自动规划）。
- `GET /v0/management/plugins/antigravity-priority/excluded-models`
  - 获取当前全局排除的模型列表。
- `POST /v0/management/plugins/antigravity-priority/excluded-models`
  - 添加或移除全局排除模型（请求体 `{"model": "模型名", "excluded": true|false}`）；被排除模型跳过全部 Antigravity 凭证，改由 CPA 其他渠道承接。
- `POST /v0/management/plugins/antigravity-priority/prompt-guard/preview`
  - 预览 Prompt Guard 规则对给定系统指令的改写结果。
- `POST /v0/management/plugins/antigravity-priority/prompt-guard/sync`
  - 从远程词库差分同步 Prompt Guard 规则（保留自定义规则、新增/更新/删除远程条目），可选 `{"sync_url": "..."}` 指定词库地址，返回同步后规则与差分统计。

---

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
