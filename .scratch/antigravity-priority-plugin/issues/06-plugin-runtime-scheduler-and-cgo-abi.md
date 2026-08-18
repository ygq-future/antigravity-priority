# 06 — Plugin Runtime, Scheduler & CGO ABI

**What to build:** Implement Application-layer runtime orchestration, background Ticker scheduling with interval cooldowns, and the core use cases (`DryRun`, `Apply`, `Diagnostics`, `Status`). Expose these use cases through both HTTP Management API endpoints and CGO ABI entry points, verifying binary compilation and multi-threaded single-flight safety.

**Blocked by:** 05 (Host Apply Engine & Audit Snapshot)

**Status:** completed

## Scope
- Implement runtime coordination in `internal/runtime/` (`runtime.go`, `ticker.go`, `production_runner.go`, `probe_evidence.go`).
- Single-flight concurrency locking via `runMu` (returns 409 Conflict / ErrRunInProgress on concurrent runs).
- Background Ticker worker managing periodic `AutoApply` respecting `interval` minimum wait.
- Implement Application use cases (`DryRun`, `Apply`, `Diagnostics`, `Status`) as the sole business execution path.
- Implement HTTP Management API in `internal/management/handler.go` (`POST /run`, `GET /diagnostics`, `GET /snapshot/latest`).
- Bind CGO ABI exports in `main.go` (`cliproxy_plugin_init`, `antigravityPriorityPluginCall`, `antigravityPriorityPluginFreeBuffer`, `antigravityPriorityPluginShutdown`).

## Explicit Non-Goals
- No HTML Web UI presentation (handled in Ticket 07).
- No multi-platform cross-compilation packaging (handled in Ticket 08).

## Acceptance Criteria
- [x] `DryRun` and `Apply` runtime use cases own all planning and execution logic; CGO and HTTP API act purely as thin adapters.
- [x] Single-flight concurrency control prevents simultaneous execution runs.
- [x] Ticker worker executes automated periodic scheduling without interval drift or premature triggers.
- [x] `go build -buildmode=c-shared` succeeds, and exported symbols are verified.
- [x] CGO memory contract (allocate buffer $\to$ copy response $\to$ host free) executes without leak or double-free across repeated invocations.

## Required Tests
- [x] Runtime lifecycle tests (register, reconfigure, shutdown).
- [x] Single-flight conflict test (concurrent `/run` returns 409 Conflict).
- [x] Auto-apply interval cooldown and ticker worker execution tests.
- [x] Management HTTP handler tests (`POST /run?mode=dry-run`, `POST /run?mode=apply`, `GET /diagnostics`, `GET /snapshot/latest`).
- [x] CGO ABI integration smoke test for buffer allocation and memory lifecycle.
