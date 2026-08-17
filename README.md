# Antigravity Smart Priority (`antigravity-priority`)

专为 [CLIProxyAPI (CPA)](https://github.com/router-for-me/CLIProxyAPI) 打造的高能效、专精型 **Google Antigravity 凭证智能配额调度与自适应提权插件**。

---

## 🌟 核心特性 (Features)

* 🚀 **动态提前提权视界 (Dynamic Boost Horizon)**：根据账号剩余周额度与物理消耗速度上限，动态计算理论完全燃尽所需的最少时间 $T_{\text{required}}$，自动在最佳时间窗口（如提前 27~40 小时）触发 `999, 998...` 梯次提权，彻底消灭“死等最后 24 小时导致大额度账号溢出浪费”的顽疾。
* 📊 **燃尽紧迫度指数 (Weekly Urgency Index)**：以 $\text{Urgency}_{\text{weekly}} = \frac{R_{\text{7d}}}{\max(T_{\text{7d}}, 0.5)}$ 实时量化单位时间消耗压力，使日常轮转中临近到期或高剩余的账号获得更优的基准优先级。
* 🧠 **自适应动态学习率 (Adaptive $C_{\text{cycle}}$ Estimator)**：基于连续探测增量（$\Delta R_{\text{5h}} \ge 5\%, \Delta R_{\text{7d}} > 0$）自动推算每个账号真实的周期消耗能力，经 EMA 指数滑动平均与安全限幅平滑收敛，无需用户手动猜测和微调数学系数。
* 🔄 **自愈式短窗软降级 (Self-Healing Soft Depletion)**：
  * **5 小时短窗耗尽**：仅软降级优先级至 `-1`，保持 `disabled = false`，5 小时后窗口自动重置即可在下一轮探测静默自愈；
  * **7 天周额度耗尽**：标记优先级 `-1` 并写入宿主硬禁用 `disabled = true`（周硬禁用优先级高于短窗软降级）。
* 🎨 **深度适配 CPA 主题的嵌入式 Web 仪表盘**：单文件纯原生内嵌（零外部 CDN，严格 CSP 安全），完全响应 CPA 宿主的明亮与暗色主题；直观展示 5h/7d 双进度条、倒计时、紧迫度得分、🚀 提权徽章与 Dry-Run 试运行预览。
* 🔒 **全链路安全脱敏 (Strict Redaction)**：所有敏感凭证信息、Authorization Header、Token 及 Cookie 均在审计快照、诊断接口与日志中自动遮蔽为 `[REDACTED]`。
* 📦 **全平台标准分发**：支持 Linux (x86/ARM)、macOS (Intel/Apple Silicon)、Windows (x86/ARM) 及 FreeBSD 全平台自动化编译构建，完全符合官方插件商店 `CLIProxyAPI-Plugins-Store` 收录规范。

---

## 📐 核心算法数学模型 (Mathematical Model)

每次探测获取到凭证的配额信息后，系统提取 4 个核心维度：
* $R_{\text{5h}}$：5 小时短窗剩余配额比例（$0.0 \sim 1.0$）
* $T_{\text{5h}}$：5 小时短窗距离重置的剩余小时数
* $R_{\text{7d}}$：7 天周长窗剩余配额比例（$0.0 \sim 1.0$）
* $T_{\text{7d}}$：7 天周长窗距离重置的剩余小时数

```text
                                 [ Antigravity Quota Probe ]
                                              │
                      ┌───────────────────────┴───────────────────────┐
                      ▼                                               ▼
         [ 5h Short Window: R_5h, T_5h ]                 [ 7d Long Window: R_7d, T_7d ]
                      │                                               │
                      │ ── ΔR_5h >= 0.05 & ΔR_7d > 0 ───────────────► │
                      │                                               │
                      │                                    [ Adaptive C_cycle Estimator ]
                      │                                               │
                      │                                               ▼
                      │                                  [ Dynamic Boost Horizon Check ]
                      │                                    T_required = (R_7d/C_cycle)*5
                      │                                    IsBoosted = T_7d <= T_required
                      │                                               │
                      ▼                                               ▼
     ┌─────────────────────────────────────────────────────────────────────────────────┐
     │                             3-Tier Comparator Hierarchy                         │
     │ 1. Tier 1 (Boosted) : IsBoosted=true -> Priorities 999, 998... (Urgency Desc)   │
     │ 2. Tier 2 (Regular) : Healthy -> Priorities 100, 99... (Urgency Desc -> T_5h)   │
     │ 3. Tier 3 (Depleted): R_7d<=0 (Hard Disable) > R_5h<=0 (Soft Fallback -1)       │
     └─────────────────────────────────────────────────────────────────────────────────┘
```

### 1. 周额度燃尽紧迫度指数（Weekly Urgency）
$$\text{Urgency}_{\text{weekly}} = \frac{R_{\text{7d}}}{\max(T_{\text{7d}}, 0.5)}$$

### 2. 自适应周期消耗速率估算（Adaptive $C_{\text{cycle}}$）
* **初始状态（Cold-Start）**：$C_{\text{cycle}} = 0.15$（假设 5 小时满载最高消耗周额度的 15%）；
* **增量学习**：在同一 5h 窗口内，当 $\Delta R_{\text{5h}} \ge 0.05$ 且 $\Delta R_{\text{7d}} > 0$ 时：
  $$C_{\text{obs}} = \frac{\Delta R_{\text{7d}}}{\Delta R_{\text{5h}}}$$
  $$C_{\text{cycle}}^{\text{new}} = 0.3 \times \text{clamp}(C_{\text{obs}}, 0.08, 0.30) + 0.7 \times C_{\text{cycle}}^{\text{old}}$$
* **状态保持**：无有效消耗增量或跨窗口时，严格保持已学习的值不变，不回退 0.15。

### 3. 动态提前提权视界（Dynamic Boost Horizon）
$$T_{\text{required}} = \left( \frac{R_{\text{7d}}}{C_{\text{cycle}}} \right) \times 5\text{ 小时}$$
$$\text{IsBoosted} = (T_{\text{7d}} \le T_{\text{required}}) \land (R_{\text{7d}} > 0)$$

---

## ⚙️ 配置说明 (Configuration)

在 CPA 宿主的 `config.yaml` 中配置启用该插件：

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    antigravity-priority:
      enabled: true
      auto_apply: false                 # 是否开启后台定时自动写回 (默认 false)
      interval: 15m                     # 自动探测与写回周期
      antigravity_model_group: "gemini" # 主控模型组: gemini 或 claude_gpt
      max_concurrency: 6                # 探针并发 HTTP Worker 数量
      min_change: 1                     # 优先级写回变动阈值
      priority_rules:
        enabled: true
        boost_start_priority: 999       # 动态提权起始优先级
        normal_start_priority: 100      # 常规轮转起始优先级
```

---

## 🔨 构建与安装 (Build & Installation)

插件编译为标准 CGO 动态链接库（`c-shared`）：

### 本地编译
```bash
# Linux / macOS
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.so .

# Windows (MSYS2 / MinGW)
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.dll .
```

### 部署到 CPA
将编译生成的产物放入 CPA 插件目录：
* `plugins/antigravity-priority.so` (Linux/FreeBSD)
* `plugins/antigravity-priority.dylib` (macOS)
* `plugins/antigravity-priority.dll` (Windows)

---

## 🌐 管理页面与接口 (Management API & UI)

* **状态仪表盘**：`GET /status`（嵌入式双主题 HTML 页面）
* **手动排序 / 试运行**：
  * `POST /run?mode=dry-run`：仅生成规划快照与优先级变动预览，不修改宿主；
  * `POST /run?mode=apply`：执行规划并写回 CPA 宿主。
* **脱敏诊断**：`GET /diagnostics`
* **最新快照**：`GET /snapshot/latest`

---

## 📄 许可证 (License)

本项目采用 [MIT License](./LICENSE) 开源许可证。
