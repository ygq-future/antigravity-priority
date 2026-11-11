# 本地 Runtime Dev Server

`cmd/devserver` 启动一个不访问真实 Antigravity 网络的本地 CPA 仿真环境。它使用生产 Runtime、Planner、Host Transition、State Cache 和管理 API，只把 CPA Host 与 Antigravity 配额接口替换为本地实现。

## 启动

```bash
go run ./cmd/devserver
```

默认地址是 `http://localhost:8080/status`，默认生成 10 个 auth 文件：

```text
data/devserver/auth-files/
data/devserver/quota-state.json
data/devserver/refresh-cache.json
data/devserver/refresh-cache.runtime.json
data/devserver/refresh-cache.runs.json
data/devserver/refresh-cache.requests.json
```

已有 `.json` auth 文件会被读取并保留；启动参数中的账号数表示目录中至少生成多少个账号，不会覆盖手动修改的文件。

## 参数

```text
-addr       HTTP 监听地址，默认 127.0.0.1:8080
-accounts   最少生成的账号数量，默认 10
-auth-dir   CPA auth JSON 目录
-quota-state 模拟配额状态文件
-state-cache Runtime 状态缓存文件
-seed       随机种子；0 表示使用时间种子
-auto-apply 启动时打开 Runtime 自动调度
-delay       模拟网络延迟时间，默认 800ms (支持 500ms, 1s 等格式；设为 0 禁用延迟)
```

也可以使用对应的 `ANTIGRAVITY_DEVSERVER_*` 环境变量。

## 仿真规则

- auth 文件按照 CPA Antigravity 凭证格式生成，Runtime 会通过 `host.auth.list/get/get_runtime/save` 读取和写回。
- `host.http.do` 在本地直接返回 `retrieveUserQuotaSummary` 兼容 JSON，不发起真实网络请求。
- 一次凭证探测返回线上同构的配额摘要：Gemini 共享池（5h、7d）、独立的 `Gemini 4 Argon` 周窗口组（`gemini-argon-wtu-weekly`）以及 Claude/GPT 组（5h、7d）。
- 默认账号中 `dev-auth-001` ~ `dev-auth-007` 模拟为 Pro 账号（包含 5h 短窗与 7d 长窗）；`dev-auth-008` ~ `dev-auth-010` 模拟为 Free 账号（无 5h 短窗，仅有 7d 周窗口），用于本地仿真验证 Tier 3（10 ~ 1 分值区间）兜底调度与禁止提权逻辑。
- 首次探测返回满额；后续探测随机消耗两个模型组的额度。
- 任一模型组的任一窗口耗尽后，下一次该凭证探测恢复两个模型组的满额。
- 配额状态、样本历史、冷却与重置审计和调度诊断分别由模拟 Host 与生产 Runtime 持久化。

## 请求历史仿真与错误验收

Dev Server 首次启动时会写入成功与失败混合的请求历史样本（含 429、500、403），便于直接验收错误详情展示。历史保留条数可在页面「请求历史」右上角或「⚙️ 配置中心」中调整（1-1000，默认 100），修改后立即裁剪并持久化。

注入任意失败请求并写入请求历史：

```bash
# 默认注入 500 错误，错误详情会显示在请求历史状态后的圆形信息图标中：
curl -X POST "http://localhost:8080/dev/simulate-error?auth_index=dev-auth-001"

# 自定义状态码、错误详情与模型：
curl -X POST "http://localhost:8080/dev/simulate-error?auth_index=dev-auth-001&status=503&message=upstream%20overloaded&model=claude-sonnet-5-5-high"
```

## 429 熔断与恢复仿真 (Manual Testing)

Dev Server 提供了快速注入 429 异常和成功请求的辅助接口，方便在本地人工验收 429 熔断与自愈机制：

```bash
# 模拟向某个凭证注入 429 报错（连续两次触发降权至 -1 与 429 冷却）：
curl -X POST "http://localhost:8080/dev/simulate-429?auth_index=auth-001"

# 模拟某凭证请求成功（清零连续 429 计数，或触发冷却即时自愈恢复）：
curl -X POST "http://localhost:8080/dev/simulate-success?auth_index=auth-001"
```

若不传 `auth_index` 参数，默认选择第一个可用的凭证账号。
