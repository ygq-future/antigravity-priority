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
```

已有 `.json` auth 文件会被读取并保留；启动参数中的账号数表示目录中至少生成多少个账号，不会覆盖手动修改的文件。

## 参数

```text
-addr       HTTP 监听地址，默认 :8080
-accounts   最少生成的账号数量，默认 10
-auth-dir   CPA auth JSON 目录
-quota-state 模拟配额状态文件
-state-cache Runtime 状态缓存文件
-seed       随机种子；0 表示使用时间种子
-auto-apply 启动时打开 Runtime 自动调度
```

也可以使用对应的 `ANTIGRAVITY_DEVSERVER_*` 环境变量。

## 仿真规则

- auth 文件按照 CPA Antigravity 凭证格式生成，Runtime 会通过 `host.auth.list/get/get_runtime/save` 读取和写回。
- `host.http.do` 在本地直接返回 `retrieveUserQuotaSummary` 兼容 JSON，不发起真实网络请求。
- 一次凭证探测同时返回 Gemini 和 Claude/GPT 两个模型组的 5h、7d 窗口。
- 首次探测返回满额；后续探测随机消耗两个模型组的额度。
- 任一模型组的任一窗口耗尽后，下一次该凭证探测恢复两个模型组的满额。
- 配额状态、样本历史、优先级写回和调度诊断分别由模拟 Host 与生产 Runtime 持久化。
