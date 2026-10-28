# 03 — State Store Persistence Consolidation and Temp Safety

Status: resolved

## What to fix
1. **Redundant Disk I/O Amplification in Production Runner**:
   During a single execution of `runProductionTask` (`internal/runtime/production_runner.go`), `store.SaveAtomic(ctx)` is invoked multiple times consecutively:
   - Line 105: Before 429 rate limit observation (if rate limits occurred).
   - `runtime.go:1182`: Inside `triggerCooldown` when recording 429 cooldown state.
   - Line 119: Unconditionally to persist freshly probed evidence observations.
   - Line 205: After plan execution to record `autoDisabled` state changes.
   - `recording.go:76`: In `projectRun` to record latest audit, result, and run history.
   Each `SaveAtomic` marshals the complete JSON document and performs `os.WriteFile` and `os.Rename`, causing repeated write amplification on disk for a single scheduling round.

2. **Static Temporary File Path Collision Risk**:
   In `internal/state/store.go:264`, the atomic save uses a fixed temporary filename:
   ```go
   tmpPath := filepath.Join(dir, filepath.Base(path)+".tmp")
   ```
   On Windows, if a previous crash or concurrent process left a locked handle or file lock on that exact `.tmp` name, subsequent writes or renames fail.

## Confirmed Semantics
1. Atomic file writes should generate unique temporary paths using `os.CreateTemp(dir, filepath.Base(path)+".*.tmp")`, ensuring collision-free writes and clean cleanup on failure.
2. In `runProductionTask`, consolidate in-memory state mutations across probing, rate limiting, and plan execution, reducing disk writes to the minimal necessary persistence points (e.g. post-probe and post-apply).

## Acceptance Criteria
- [x] `SaveAtomic` uses a dynamic, collision-safe temporary file pattern (`os.CreateTemp`) instead of a static `.tmp` filename.
- [x] Multiple consecutive `SaveAtomic` calls within `runProductionTask` are consolidated without losing persistence durability or crash consistency.
- [x] `store_test.go` and `runtime_test.go` verify atomic persistence, concurrency safety, and recovery across reloads.
