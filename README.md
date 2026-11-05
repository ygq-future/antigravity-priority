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
- **池级隔离 429 熔断与自愈**：429 限流精准隔离至受限配额池（例如 Claude 池遭遇 429 不影响 Gemini 与 Argon 调用），瞬时多账号异常由风暴保护吸收，冷却期满或同池请求成功后立即恢复。
- **闭环实时用量推算与探针自动校准**：通过 `usage.handle` 实时估算各凭证 token 消耗带来的额度变化，并在周期性后台探测时平滑自校准 EMA 灵敏度 $K_{\text{5h}}$ 与周期燃尽率 $C_{\text{cycle}}$。
- **动态提前提权（Dynamic Boost Horizon）**：根据账号剩余周额度与物理消耗速率，自动在最佳时间窗口触发 `999, 998...` 梯次提权，彻底解决大额度账号到期撑死溢出的痛点。
- **周紧迫度平滑轮转（Weekly Urgency）**：实时量化单位时间消耗压力，使日常轮转中临近到期或高剩余的账号获得更优的基准优先级。
- **自适应动态学习率（Adaptive $C_{\text{cycle}}$）**：基于连续探测增量自动推算每个账号真实的周期消耗能力并平滑收敛，无需用户手动猜测和配置复杂的数学系数。
- **自愈式软降级与硬禁用**：5 小时短窗耗尽仅软降级优先级至 `-1`（重置后自动静默自愈），7 天周额度耗尽执行硬禁用 `disabled = true`。
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
      # state_cache_path: "data/antigravity-priority-cache.json" # 可选，自定义持久化缓存路径
```

| 宿主字段 | 默认值 | 必填 | 说明 |
| :--- | :--- | :--- | :--- |
| **`enabled`** | `true` | 是 | 插件全局启用开关。设为 `false` 时彻底停止调度与后台任务。 |
| **`state_cache_path`** | `data/antigravity-priority-cache.json` | 否 | 插件状态快照、动态配置与时序自适应学习率的持久化存储文件路径。 |

> **⚠️ 路径迁移提示**：插件不支持跨路径自动热迁移旧数据。若在运行中途更改了 `state_cache_path`，系统在新路径未找到文件时将默认作为全新冷启动。如需保留原有的动态配置、历史记录与自适应学习率，请在修改 YAML 前停止 CPA / 插件，手动将原缓存文件移动或复制至新路径。

### 2. UI 动态配置中心 (免重启热生效)

在管理面板的 **`⚙️ 配置中心`** 标签页中，可随时在线配置并立即热生效以下选项：

| 配置项 | 默认值 | 范围/选项 | 说明 |
| :--- | :--- | :--- | :--- |
| **后台定时配额校准 (`auto_apply`)** | `true` | 开 / 关 | 是否由后台定时器周期性向 Google 发起权威配额探测，自动校准实时消耗估算器。 |
| **配额校准执行周期 (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, 自定义 | 自动探测与校准的运行周期。修改后立即重设定时器生效。 |
| **生效时间区间 (`schedule_window`)** | `全天` | `HH:MM` 至 `HH:MM` | 每日生效时段（如 `09:00-23:00`，支持跨午夜如 `22:00-06:00`），非时段内自动休眠。 |
| **最大探测并发数 (`max_concurrency`)** | `6` | `1 ~ 32` | 向 Google 配额接口发起并发探测的最大协程数。 |
| **紧迫度分档容差 (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | 紧迫度差距在此容差内的账号分配相同优先级整数进行平级轮询。 |
| **自适应时序样本容量 (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO 保留的历史探测样本数，同时用于消耗趋势查看与燃尽率学习。 |
| **429 熔断冷却时长 (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` 分钟 | 遭遇 429 限流时临时降级至 `-1` 兜底队列的冷却期，到期自动自愈。 |
| **动态提权起始优先级 (`boost_start_priority`)** | `999` | `1 ~ 999` | 触发提权状态的第一梯队基准起始优先级。 |
| **常规健康起始优先级 (`normal_start_priority`)** | `100` | `1 ~ 999`，且不高于 Boost 起始值 | 常规可用健康梯队的基准起始优先级。 |
> **持久化保障**：所有通过 UI 配置中心修改的选项会自动原子保存至当前持久化缓存文件（默认 `data/antigravity-priority-cache.json`），重启 CPA 容器数据不丢失，且优先级高于 YAML 初始值。最近一次自动调度的完成时间同样持久化，重启后按剩余周期继续计时，不会从零重新开始调度。

---

## 管理页面与接口

插件通过 `management.register` 分别向 CPA 宿主注册 **resources**（静态管理仪表盘）与 **routes**（动态管理 API）。

### 资源页面（静态 Web UI）

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **访问方式**：在 CPA 管理后台侧边栏点击 **"Antigravity Priority"** 菜单，或在浏览器中直接访问 `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status`。
  - **核心功能**：
    - **模型驱动路由看板**：支持下拉选择具体模型（Gemini 池、Argon 池、Claude & GPT 池），即时展示对应模型的完整凭证调用次序（Tier #1 · 999 提权、Tier #2 · 100 平级轮询等）与生效状态。
    - **全景五进度条卡片**：每个凭证卡片同时渲染全部 5 根配额进度条（Gemini 5h/7d、Claude 5h/7d、Argon 7d），并高亮当前选中模型的生效配额；秒级自适应倒计时与自适应消耗速率 $C_{\text{cycle}}$。
    - **两阶段控制**：提供 `🔄 刷新同步 (Sync Host)`、`📡 刷新配额 (10s冷却)`、`🧹 清除静态优先级`、`🔄 重置默认`。
    - **执行历史**：最近 10 次执行记录，支持点击 `🔍 查看明细` 弹窗查看 Apply 实际写回或 Probe 探测快照明细。
    - **采样历史**：支持自由切换 Gemini、Argon 与 Claude & GPT 三大配额池时序样本。
    - **系统诊断**：调度引擎生命周期、时段状态、429 熔断与冷却监控看板、完整 email 身份、最近写入健康度与脱敏审计流，支持一键复制诊断 JSON。
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
  - 导出调度器运行诊断数据、429 活跃熔断记录、后台 Ticker 状态、最近写入体征与近期执行记录；账号身份使用完整 email，令牌、密钥及持久化审计字段保持脱敏。
- `POST /v0/management/plugins/antigravity-priority/sync`
  - 主动从 CPA 宿主同步最新凭证文件列表并即时重新生成双组快照。
- `GET /v0/management/plugins/antigravity-priority/samples?auth_index=xxx`
  - 获取指定凭证在各模型组下的历史滑动窗口时序采样数据。
- `GET /v0/management/plugins/antigravity-priority/samples?probe_round_id=xxx&model_group=gemini|argon|claude_gpt`
  - 获取指定探测轮次和模型组内实际新增的额度样本；额度未变化的凭证不会出现在结果中。
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - 获取最近一次双模型组决策规划快照（`DualGroupSnapshot`）；响应保留完整 CPA email 和供 API 关联使用的技术 `auth_index`，管理页仅以 email 作为账号名称，敏感令牌与审计字段保持脱敏。

---

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
