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

- **Antigravity 专精双窗口调度**：专为 Google Antigravity 体系（支持 `gemini` 与 `claude_gpt` 配额模型组）深度定制，精准联动 5 小时短窗与 7 天周长窗配额。
- **同分平级分档与负载均衡**：相近紧迫度账号自动分配相同优先级整数，由 CPA 原生轮询分流，彻底杜绝单点打穿 429。
- **429 实时识别与熔断冷却**：自动捕获业务调用与探针 429 错误，立即将账号优先级降级至 `-1` 兜底队列，冷却期满后自动自愈恢复。
- **动态提前提权（Dynamic Boost Horizon）**：根据账号剩余周额度与物理消耗速率，自动在最佳时间窗口触发 `999, 998...` 梯次提权，彻底解决大额度账号到期撑死溢出的痛点。
- **周紧迫度平滑轮转（Weekly Urgency）**：实时量化单位时间消耗压力，使日常轮转中临近到期或高剩余的账号获得更优的基准优先级。
- **自适应动态学习率（Adaptive $C_{\text{cycle}}$）**：基于连续探测增量自动推算每个账号真实的周期消耗能力并平滑收敛，无需用户手动猜测和配置复杂的数学系数。
- **自愈式软降级与硬禁用**：5 小时短窗耗尽仅软降级优先级至 `-1`（重置后自动静默自愈），7 天周额度耗尽写入宿主硬禁用 `disabled = true`。
- **UI 动态配置中心（免重启热生效）**：CPA 宿主 YAML 仅需保留 `enabled: true`，所有业务与调度参数统一在 Web UI **`⚙️ 配置中心`** 可视化调节并即时热生效。
- **嵌入式双主题仪表盘**：零外部 CDN，严格 CSP 安全，完全自适应 CPA 宿主主题，提供配额监控、双组即时预测切换、变动 Diff 确认写回控制与全链路数据安全脱敏。

---

## 工作流程

```text
加载插件
  -> 读取 plugins.configs.antigravity-priority 配置
  -> 通过 host.auth.list 获取 CPA 凭证列表
  -> 过滤 Antigravity 凭证
       - 按所选模型组 (gemini 或 claude_gpt) 并发探测 5h 与 7d 双窗口剩余配额
       - 提取短窗剩余比例 R_5h、短窗重置倒计时 T_5h、周额度剩余比例 R_7d、周重置倒计时 T_7d
       - 结合自适应 C_cycle 学习率判定是否进入 Dynamic Boost 提权区间
  -> 依据 3 级比较器构建排序计划
       - Tier 1 (Boosted) : 提权区间 -> 分配 999, 998... (按周紧迫度降序)
       - Tier 2 (Regular) : 常规健康 -> 分配 100, 99... (按周紧迫度降序，短窗重置时间平局决胜)
       - Tier 3 (Depleted): 周耗尽 hard-disable > 短窗耗尽 soft-fallback (-1)
  -> 根据运行模式执行
       - apply：通过 host.auth.save 写回优先级与启用状态 (min_change 过滤微小变动)
       - probe / sync：仅更新内存状态、脱敏诊断与快照
  -> 在管理页面展示脱敏后的双窗口仪表、紧迫度评分、提权状态与审计摘要
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
| **自动定时调度 (`auto_apply`)** | `false` | 开 / 关 | 是否由后台定时器周期性自动执行探测、规划并写回宿主凭证优先级。 |
| **调度执行周期 (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, 自定义 | 自动探测与排序调度的运行周期。修改后立即重设定时器生效。 |
| **配额主控模型组 (`antigravity_model_group`)** | `gemini` | `gemini` / `claude_gpt` | 配额主控模型组，以此组配额为依据决定写回宿主的优先级。 |
| **生效时间区间 (`schedule_window`)** | `全天` | `HH:MM` 至 `HH:MM` | 每日生效时段（如 `09:00-23:00`，支持跨午夜如 `22:00-06:00`），非时段内自动休眠。 |
| **最大探测并发数 (`max_concurrency`)** | `6` | `1 ~ 32` | 向 Google 配额接口发起并发探测的最大协程数。 |
| **优先级变动写入阈值 (`min_change`)** | `1` | `0 ~ 100` | 优先级新旧变动绝对值达到该阈值才写入宿主，以减少磁盘 IO。 |
| **紧迫度分档容差 (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | 紧迫度差距在此容差内的账号分配相同优先级整数进行平级轮询。 |
| **自适应时序样本容量 (`quota_sample_capacity`)** | `6` | `2 ~ 30` | 滑动窗口保留的历史探测样本数，用于平滑估计燃尽率。 |
| **429 熔断冷却时长 (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` 分钟 | 遭遇 429 限流时临时降级至 `-1` 兜底队列的冷却期，到期自动自愈。 |
| **动态提权起始优先级 (`boost_start_priority`)** | `999` | `1 ~ 999` | 触发提权状态的第一梯队基准起始优先级。 |
| **常规健康起始优先级 (`normal_start_priority`)** | `100` | `1 ~ 999`，且不高于 Boost 起始值 | 常规可用健康梯队的基准起始优先级。 |

> **持久化保障**：所有通过 UI 配置中心修改的选项会自动原子保存至当前持久化缓存文件（默认 `data/antigravity-priority-cache.json`），重启 CPA 容器数据不丢失，且优先级高于 YAML 初始值。

---

## 管理页面与接口

插件通过 `management.register` 分别向 CPA 宿主注册 **resources**（静态管理仪表盘）与 **routes**（动态管理 API）。

### 资源页面（静态 Web UI）

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **访问方式**：在 CPA 管理后台侧边栏点击 **"Antigravity Priority"** 菜单，或在浏览器中直接访问 `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status`。
  - **核心功能**：
    - **概览与仪表盘**：5h/7d 双窗口配额进度条、自适应秒级倒计时、自适应消耗速率 $C_{\text{cycle}}$、周紧迫度得分与 🚀 提权状态；支持单行/双列网格切换与容器内独立滚动。
    - **双模型组即时切换**：随时切换 Gemini 或 Claude/GPT 视图，非主控组智能标注 `🔮 预测优先级`。
    - **两阶段控制**：提供 `📡 刷新配额 (10s冷却)`、`⚡ 立即写回 (带Diff确认)`、`🔄 重置默认`。
    - **执行历史**：最近 10 次执行记录，支持点击 `🔍 查看明细` 弹窗查看 Apply 实际写回或 Probe 探测快照明细。
    - **系统诊断**：调度引擎生命周期、时段状态、429 熔断与冷却监控看板、最近写入健康度与脱敏审计流，支持一键复制完整诊断 JSON。
    - **⚙️ 配置中心**：在线修改所有调度与算法参数，0 秒热生效与一键恢复默认。

### 管理 API（动态接口，需 Management Key 鉴权）

- `POST /v0/management/plugins/antigravity-priority/run?mode=probe`
  - 触发一次向 Google API 的全量配额探测并更新本地缓存与快照，**不执行写回**。
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply`
  - 触发全量探测计算并将最新得到的优先级与启用状态**写回 CPA 宿主**。
- `GET /v0/management/plugins/antigravity-priority/config`
  - 获取当前完整运行时配置。
- `POST /v0/management/plugins/antigravity-priority/config`
  - 提交更新运行时配置并热生效。
- `GET /v0/management/plugins/antigravity-priority/schedule/config`
  - 获取自动调度时间区间与暂停状态。
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - 动态切换自动调度暂停/恢复或更新生效时间区间。
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - 导出全脱敏的调度器运行诊断数据、429 活跃熔断记录、后台 Ticker 状态、最近写入体征与近期执行记录。
- `POST /v0/management/plugins/antigravity-priority/sync`
  - 主动从 CPA 宿主同步最新凭证文件列表并即时重新生成双组快照。
- `GET /v0/management/plugins/antigravity-priority/samples?auth_index=xxx`
  - 获取指定凭证在各模型组下的历史滑动窗口时序采样数据。
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - 获取最近一次双模型组脱敏决策规划快照（`DualGroupSnapshot`）。

---

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
