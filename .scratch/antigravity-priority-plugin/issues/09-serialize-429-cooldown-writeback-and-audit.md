# 09 — Serialize 429 Cooldown Write-Back and Audit

**Status:** completed

**What to fix:** Preserve the immediate 429 Reactive Cooldown behavior while routing its host mutation through the Runtime single-flight boundary and a complete, redacted audit path.

## Confirmed Requirements
- A detected upstream 429 still demotes the affected credential immediately to `priority = -1` and keeps `disabled = false`.
- The 429 mutation must not race with `ManualApply`, `AutoApply`, `Probe`, `SyncHost`, or reset operations.
- The mutation result, including failure, must be observable in diagnostics/run history; host patch errors must not be silently discarded.
- Reactive cooldown is an explicit forced transition and must not be blocked by `min_change` or ordinary Fresh Evidence requirements.

## Acceptance Criteria
- [x] A deterministic concurrency test blocks a normal Apply write, injects a 429 event, and proves the final host state is deterministic.
- [x] All 429 host mutations execute under the Runtime single-flight policy.
- [x] Success and failure both create redacted audit/run-history records.
- [x] The Filter response does not claim an unqualified success when the cooldown state or host mutation fails.
