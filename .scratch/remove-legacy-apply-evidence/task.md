# 任务：清理 legacy 宿主写回与 EvidenceFresh 语义

> 完成后删除本文件。

## 背景

当前调度主链路是 `scheduler.pick`：

- 路由表由 `state.Store.BuildHistoricalEvidence` + `priority.PlanWithEvidence` 生成（`internal/runtime/scheduler.go`）；
- 始终使用持久化中的最新配额，不存在“新鲜 / 陈旧”状态；
- `d1c247e` 已删除概览的历史数据标签。

`EvidenceFresh` 目前只剩 legacy `mode=apply` 宿主写回路径的内部门禁：

- `priority.PlanItem.EvidenceFresh`
- `priority.markHistoricalReasons`（生成 `historical: ` 前缀）
- `priority.shouldChange`（`!EvidenceFresh` 时不写回）
- `apply.SnapshotItem.EvidenceFresh` / `apply.Change.EvidenceFresh`
- management `/run?mode=apply` 分支

UI 已不暴露“立即写回”；`auto_apply` 只做后台配额校准。诊断页已移除新鲜度 KPI、证据列和 `historical:` 前缀。

## 任务清单

- [ ] 确认 `mode=apply` 是否仍有外部消费者；若无，删除 management 的 apply 分支与相关测试。
- [ ] 删除 `priority.PlanItem.EvidenceFresh`、`markHistoricalReasons`、`shouldChange` 的 `!EvidenceFresh` 门禁。
- [ ] 删除 `apply.Change.EvidenceFresh`、`SnapshotItem.EvidenceFresh` 及 JSON `evidence_fresh` 字段。
- [ ] 若 `ResetAllPriorities` 仍保留宿主写回，明确其与调度的关系；否则一并清理。
- [ ] 更新 `README.md` / `README.en.md` / `docs/release-checklist.md` / 测试夹具。
- [ ] 运行 `golangci-lint run --timeout=5m`、`go build ./...`、`go vet ./...`、`go test ./...`、`go test -race ./...`。

## 验收

- 全仓库无 `EvidenceFresh` / `evidence_fresh` / `historical:` / `markHistoricalReasons` / `shouldChange` 的残留（历史文档除外）。
- `scheduler.pick` 行为不变：仍使用最新持久化配额。
- 诊断页不再出现任何“新鲜 / 陈旧 / 历史”状态。

## 备注

本次只记录问题，不实施。完成后删除 `.scratch/remove-legacy-apply-evidence/` 目录。
