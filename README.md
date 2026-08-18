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

- **Antigravity 专精优化**：专为 Google Antigravity 体系（支持 `gemini` 与 `claude_gpt` 配额模型组）深度定制，精准提取 5 小时短窗与 7 天周长窗配额数据。
- **动态提前提权（Dynamic Boost Horizon）**：根据账号剩余周额度与物理消耗速率，动态计算完全燃尽所需的最少时间，自动在最佳时间窗口（如提前 27~40 小时）触发 `999, 998...` 梯次提权，彻底解决“死等最后 24 小时导致大额度账号撑死溢出”的痛点。
- **周紧迫度平滑轮转（Weekly Urgency）**：实时量化单位时间消耗压力，使日常轮转中临近到期或高剩余的账号获得更优的基准优先级。
- **自适应动态学习率（Adaptive $C_{\text{cycle}}$）**：基于连续探测增量自动推算每个账号真实的周期消耗能力并平滑收敛，无需用户手动猜测和配置复杂的数学系数。
- **自愈式软降级与硬禁用**：
  - **5 小时短窗耗尽**：仅软降级优先级至 `-1`，保持 `disabled = false`，5 小时后窗口自动重置即可在下一轮探测静默自愈；
  - **7 天周额度耗尽**：标记优先级 `-1` 并写入宿主硬禁用 `disabled = true`（周硬禁用优先级高于短窗软降级）。
- **复用 CPA 宿主链路**：通过宿主回调 `host.auth.list`、`host.auth.get`、`host.auth.get_runtime`、`host.auth.save` 复用 CPA 的凭证、代理和写入链路。
- **新鲜证据门禁**：只对本轮最新且可用的探测证据生成排序变更，配额探测失败或不可用凭证保留当前状态。
- **全链路安全脱敏**：所有敏感凭证信息、Authorization Header、Token 及 Cookie 均在审计快照、诊断接口与日志中自动遮蔽为 `[REDACTED]`。
- **CPA 原生双主题管理面板**：内嵌单文件原生 Web 仪表盘（零外部 CDN，严格 CSP 安全），完全自适应 CPA 宿主的明亮与暗色主题。

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
  -> 根据运行模式决定是否写回
       - apply：通过 host.auth.save 写回优先级与启用状态 (min_change 过滤微小变动)
       - dry-run / preview：仅更新内存状态、脱敏诊断与快照
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

在 CPA 的 `config.yaml` 中启用插件系统，并在 `plugins.configs.antigravity-priority` 下保留插件自有配置：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    antigravity-priority:
      enabled: true
      auto_apply: false                 # 是否由定时器自动执行并写回排序结果 (默认 false)
      interval: 15m                     # 自动探测与排序调度周期
      antigravity_model_group: "gemini" # 配额主控模型组: gemini 或 claude_gpt
      max_concurrency: 6                # 探针并发 HTTP Worker 数量
      min_change: 1                     # 优先级写回变动阈值
      priority_rules:
        enabled: true
        boost_start_priority: 999       # 动态提权起始优先级
        normal_start_priority: 100      # 常规轮转起始优先级
```

### 字段说明

| 字段 | 类型 | 默认值 | 说明 |
| :--- | :--- | :--- | :--- |
| `enabled` | boolean | `true` | 单插件开关；还需要全局 `plugins.enabled: true` 且动态库注册成功。 |
| `auto_apply` | boolean | `false` | 是否由后台定时器自动执行并写回排序结果。若为 `false`，后台仅更新内存与快照，可通过管理页手动执行写回。 |
| `interval` | duration | `15m` | 自动排序与探测的时间周期（例如 `15m`, `30m`, `1h`）。 |
| `antigravity_model_group` | string | `gemini` | 配额主控模型组，支持 `gemini`（Gemini 2.5/Flash）与 `claude_gpt`（Claude 3.5/GPT-4o）。 |
| `max_concurrency` | integer | `6` | 探测并发 HTTP Worker 数量上限。 |
| `min_change` | integer | `1` | 优先级最小变动阈值，当新旧优先级差异小于该值时跳过写回以减少存储 IO。 |
| `priority_rules.enabled` | boolean | `true` | 是否启用多级优先级规则；关闭时使用内置默认排序。 |
| `priority_rules.boost_start_priority` | integer | `999` | 触发提权状态的起始基准优先级。 |
| `priority_rules.normal_start_priority` | integer | `100` | 常规健康账号的起始基准优先级。 |

> **提示**：可通过 CPA 插件管理的可视化配置字段（`ConfigFields`）直接编辑上述选项，也可在 `config.yaml` 中手动修改。

---

## 管理页面与接口

插件通过 `management.register` 分别向 CPA 宿主注册 **resources**（静态管理仪表盘）与 **routes**（动态管理 API）。

### 产品边界

| 能力 | 访问入口 | 说明 |
| :--- | :--- | :--- |
| 自动优先级与规则配置 | CPA 插件管理可视化字段 或 `config.yaml` | 修改 `auto_apply`、`interval`、`antigravity_model_group` 等配置 |
| 插件资源管理页面 | `/v0/resource/plugins/antigravity-priority/status` | 静态 HTML 仪表盘：Key 验证 + 5h/7d 配额进度 + 模型组切换 + Dry-Run/Apply |
| 手动排序 / 执行写入 | `/v0/management/plugins/antigravity-priority/run` | 动态 Management API（需 Management Key） |
| 脱敏诊断与快照 | `/v0/management/plugins/antigravity-priority/diagnostics` | 查看探针状态、自适应速率与执行记录 |
| 只读配置查看 | 宿主 `GET /v0/management/plugins/antigravity-priority/config` | CPA 宿主提供的插件配置只读查看接口 |

### 资源页面（静态 Web UI）

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **访问方式**：在 CPA 管理后台侧边栏点击 **"Antigravity Priority"** 菜单，或在浏览器中直接访问 `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status`。
  - **功能**：返回内嵌的原生双主题 Web 仪表盘。输入 Management Key 登录后，可实时查看所有凭证的 5 小时短窗与 7 天周长窗配额进度条、重置倒计时、自适应消耗速率 $C_{\text{cycle}}$、紧迫度得分与 🚀 提权状态；支持在线切换模型组，并提供 **Dry-Run（试运行预览）** 与 **Apply（立即写回）** 交互按钮。

### 管理 API（动态接口，需 Management Key 鉴权）

- `POST /v0/management/plugins/antigravity-priority/run?mode=dry-run`
  - 触发一次探测与优先级计算（试运行），更新最新内存快照与界面预览，**不修改宿主凭证**。
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply`
  - 触发探测与计算，并将最新计算得到的优先级与启用状态**写回 CPA 宿主**。
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply&antigravity_model_group=claude_gpt`
  - 临时切换为主控 Claude/GPT 模型组并执行探测与写回。
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - 导出全脱敏的调度器运行诊断数据、后台 Ticker 状态与近期执行记录。
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - 获取最近一次运行的脱敏决策规划快照。

---

## 许可证

本项目使用 MIT License，详见 [LICENSE](./LICENSE)。
