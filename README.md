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

- **Antigravity 专精三大配额池调度**：专为 Google Antigravity 体系（`gemini`、`argon` 与 `claude_gpt` 三大独立配额池）深度定制，精准联动 5 小时短窗与 7 天周长窗配额。
- **CPA 原生 `scheduler.pick` 纯内存动态调度**：通过注册 `scheduler: true` 与 `scheduler_across_priorities: true`，在收到请求时毫秒级解析目标模型并调度至最优凭证，实现**零磁盘写入、纯内存无冲突调度**。
- **模型驱动路由看板与五进度条卡片**：彻底取代旧版双组下拉框，提供直观的**具体模型路由看板**；每个凭证卡片同时渲染全部 5 根配额进度条（Gemini 5h/7d、Claude 5h/7d、Argon 7d），并高亮当前选中模型的生效配额与调用次序分档。
- **会话粘性与同分平级分档**：相近紧迫度账号自动分配相同优先级整数，顶层平级集群内支持 `canonical_session_id` 会话粘性绑定与平滑轮询分流，杜绝单点打穿 429。
- **自定义梯队覆盖与轮询负载均衡 (Custom Tier Overrides)**：支持在概览卡片上为凭证自由设定目标梯队（Tier #1、Tier #2 等），自定义梯队具备最高优先级，同梯队凭证自动获得相同优先级分值并通过 Round-Robin 算法均衡分流。
- **全链路实时请求调用历史 (Real-Time Request History)**：实时监控记录每次请求的模型、匹配配额池、Token 消耗（输入、输出、思考、缓存）以及返回状态，提供高密度仪表盘与一键清空。
- **池级隔离 429 熔断与自愈**：429 限流精准隔离至受限配额池（例如 Claude 池遭遇 429 不影响 Gemini 与 Argon 调用），瞬时多账号异常由风暴保护吸收，冷却期满后进入探测队列，探测成功或同池请求成功后恢复。
- **健康故障转移与凭证异常隔离 (Health Failover)**：401 与 403 均视为凭证异常直接执行账号级隔离并软降级（不进行重试探测与刷新），5xx 按池级短暂降级；429/5xx 进入指数退避探测队列并复用预热请求验证，一次成功即恢复原梯队。
- **闭环实时用量推算与探针自动校准**：通过 `usage.handle` 实时估算各凭证 token 消耗带来的额度变化，并在周期性后台探测时平滑自校准 EMA 灵敏度 $K_{\text{5h}}$ 与周期燃尽率 $C_{\text{cycle}}$。
- **动态提前提权（Dynamic Boost Horizon）**：根据账号剩余周额度与物理消耗速率，自动在最佳时间窗口触发 `999, 998...` 梯次提权，彻底解决大额度账号到期撑死溢出的痛点。
- **周紧迫度平滑轮转（Weekly Urgency）**：实时量化单位时间消耗压力，使日常轮转中临近到期或高剩余的账号获得更优的基准优先级。
- **自适应动态学习率（Adaptive $C_{\text{cycle}}$）**：基于连续探测增量自动推算每个账号真实的周期消耗能力并平滑收敛，无需用户手动猜测和配置复杂的数学系数。
- **自愈式双窗软降级与池级隔离**：5 小时短窗与 7 天周长窗额度耗尽均执行纯内存软降级（优先级降至 `-1`，保持 `disabled = false`），宿主机账号 `disabled` 状态 100% 归属用户；配额重置刷新后自动静默恢复。
- **5 小时短窗重置自动预热 (Short-Window Quota Warmup)**：当 Gemini 或 Claude & GPT 5h 短窗处于 100% 满额且重置倒计时尚未激活时，自动发送微小自然工程请求提前开启 5 小时重置倒计时，避免空窗期浪费；窗口重置到点后由持久化配额证据驱动补一次预热，自动调度暂停或插件重启都不会漏掉，支持独立开关与预热模型热配置。
- **UI 动态配置中心（免重启热生效）**：CPA 宿主 YAML 仅需保留 `enabled: true`，所有业务与调度参数统一在 Web UI **`⚙️ 配置中心`** 可视化调节并即时热生效。
- **Pro 与 Free 凭证独立分层**：无 5 小时短窗的 Free 凭证自动隔离至独立兜底底仓（默认 10 往下分配），严禁进入 Boost 提权抢占流量，确保 Pro 凭证绝对优先调用，且支持在 Web 配置中心按需调整基准分值。
- **嵌入式双主题仪表盘**：零外部 CDN，严格 CSP 安全，完全自适应 CPA 宿主主题，提供全景配额监控、模型路由看板、完整 email 身份与脱敏诊断审计流。
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
| **后台定时配额校准 (`auto_apply`)** | `false` | 开 / 关 | 是否由后台定时器周期性向 Google 发起权威配额探测，自动校准实时消耗估算器。 |
| **配额校准执行周期 (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, 自定义 | 自动探测与校准的运行周期。修改后立即重设定时器生效。 |
| **生效时间区间 (`schedule_window`)** | `全天` | `HH:MM` 至 `HH:MM` | 每日生效时段（如 `09:00-23:00`，支持跨午夜如 `22:00-06:00`），非时段内自动休眠。 |
| **最大探测并发数 (`max_concurrency`)** | `6` | `1 ~ 32` | 向 Google 配额接口发起并发探测的最大协程数。 |
| **紧迫度分档容差 (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | 紧迫度差距在此容差内的账号分配相同优先级整数进行平级轮询。 |
| **自适应时序样本容量 (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO 保留的历史探测样本数，同时用于消耗趋势查看与燃尽率学习。 |
| **429 熔断冷却时长 (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` 分钟 | 遭遇 429 限流时将受限配额池临时置入冷却队列避让，到期自动自愈。 |
| **动态提权起始优先级 (`boost_start_priority`)** | `999` | `1 ~ 999` | 触发提权状态的第一梯队基准起始优先级。 |
| **常规健康起始优先级 (`normal_start_priority`)** | `100` | `1 ~ 999`，且不高于 Boost 起始值 | 常规可用健康梯队的基准起始优先级。 |
| **Free 账号兜底起始优先级 (`free_start_priority`)** | `10` | `1 ~ 999`，且不高于常规起始值 | 仅有周额度、无 5h 短窗的 Free 账号基准优先级，排在 Pro 之后兜底。 |
| **Gemini 5h 满额自动预热 (`warmup.gemini_enabled`)** | `false` | 开 / 关 | 当 Gemini 5h 短窗 100% 满额且倒计时未激活时，自动发送微小请求提前触发 5h 倒计时。 |
| **Gemini 预热目标模型 (`warmup.gemini_model`)** | `gemini-3-flash` | 可选 / 自定义 | 用于触发 Gemini 5h 重置倒计时的预热模型。 |
| **Claude 5h 满额自动预热 (`warmup.claude_enabled`)** | `false` | 开 / 关 | 当 Claude 5h 短窗 100% 满额且倒计时未激活时，自动发送微小请求提前触发 5h 倒计时。 |
| **Claude 预热目标模型 (`warmup.claude_model`)** | `claude-sonnet-5-5-high` | 可选 / 自定义 | 用于触发 Claude 5h 重置倒计时的预热模型。 |

---

## 管理页面与接口

插件通过 `management.register` 分别向 CPA 宿主注册 **resources**（静态管理仪表盘）与 **routes**（动态管理 API）。

### 资源页面（静态 Web UI）

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **访问方式**：在 CPA 管理后台侧边栏点击 **"Antigravity Priority"** 菜单，或在浏览器中直接访问 `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status`。
  - **核心功能**：
    - **模型驱动路由看板**：支持下拉选择具体模型（Gemini 池、Argon 池、Claude & GPT 池），即时展示对应模型的完整凭证调用次序（Tier #1 · 999 提权、Tier #2 · 100 平级轮询等）与生效状态。
    - **全景五进度条卡片**：每个凭证卡片同时渲染全部 5 根配额进度条（Gemini 5h/7d、Claude 5h/7d、Argon 7d），并高亮当前选中模型的生效配额；秒级自适应倒计时与自适应消耗速率 $C_{\text{cycle}}$。
    - **调度与同步控制**：提供 `🔄 刷新同步 (Sync Host)`、`📡 刷新配额 (10s冷却)`、`🔄 重置静态优先级 (Reset Priorities)`。
    - **实时请求历史**：实时追踪捕获最近请求记录（保留条数可配置，默认 100，最高 1000），记录请求 Model、目标配额池、Token 消耗细分与状态码，支持一键清空。
    - **采样历史**：支持自由切换 Gemini、Argon 与 Claude & GPT 三大配额池时序样本。
    - **系统诊断**：运行健康横幅、调度/隔离冷却/最近运行三类 KPI、三配额池矩阵、逐凭证调度决策解释（优先级、Urgency、T7d/T_req、C_cycle 与原因）、429 风暴与冷却隔离、运行时间线，以及四类缓存文件的路径、体积与缺失状态，支持一键复制诊断 JSON。
    - **⚙️ 配置中心**：在线修改所有调度与算法参数，0 秒热生效与一键恢复默认。

### 管理 API（动态接口，需 Management Key 鉴权）

- `POST /v0/management/plugins/antigravity-priority/run?mode=probe`
  - 触发一次向 Google API 的全量配额探测并更新本地缓存与快照，**不执行写回**。
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply`
  - 触发全量探测计算，通过统一 Host Transition 将最新目标**一次性写回并校验 CPA 宿主**，返回脱敏执行结果。
- `GET /v0/management/plugins/antigravity-priority/runtime-config`
  - 获取当前完整运行时配置。
- `POST /v0/management/plugins/antigravity-priority/runtime-config`
  - 提交更新运行时配置并热生效。
- `GET /v0/management/plugins/antigravity-priority/schedule/config`
  - 获取自动调度时间区间与暂停状态。
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - 动态切换自动调度暂停/恢复或更新生效时间区间。
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

---

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
