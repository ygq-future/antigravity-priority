# 03 — Adaptive Burn Rate Estimator & State Store

**What to build:** Maintain per-credential state and historical observations in an atomic local cache (`refresh-cache.json`). Adaptively estimate the cycle burn rate ($C_{\text{cycle}}$) from in-window usage deltas ($\Delta R_{\text{5h}} \ge 0.05$ and $\Delta R_{\text{7d}} > 0$), apply EMA smoothing ($\alpha = 0.3$) and safe bounding $[0.08, 0.30]$, preserve previously learned $C_{\text{cycle}}$ when no valid delta occurs (never resetting to 0.15), and fall back to default 0.15 only during cold-start.

**Blocked by:** 01 (Repository Foundation & Minimal CGO Tracer Bullet)

**Status:** completed

## Scope
- Implement state persistence and cache store in `internal/state/store.go`.
- Implement adaptive $C_{\text{cycle}}$ estimator:
  - Cold-start baseline: $C_{\text{cycle}} = 0.15$.
  - Delta observation trigger: in the same 5h window with $\Delta R_{\text{5h}} \ge 0.05$ and $\Delta R_{\text{7d}} > 0$.
  - Observed ratio: $C_{\text{obs}} = \Delta R_{\text{7d}} / \Delta R_{\text{5h}}$.
  - Clamping & smoothing: $C_{\text{cycle}}^{\text{new}} = 0.3 \times \text{clamp}(C_{\text{obs}}, 0.08, 0.30) + 0.7 \times C_{\text{cycle}}^{\text{old}}$.
  - Persistence: update per-credential entry and save atomically to `refresh-cache.json`.
- State re-loading and continuity.

## Explicit Non-Goals
- No priority planning calculations (handled in Ticket 04).
- No host write-back operations (handled in Ticket 05).

## Acceptance Criteria
- [x] Cold-start credentials with no cache history return default $C_{\text{cycle}} = 0.15$.
- [x] On valid in-window consumption ($\Delta R_{\text{5h}} \ge 0.05, \Delta R_{\text{7d}} > 0$), $C_{\text{cycle}}$ updates correctly via EMA and remains clamped within $[0.08, 0.30]$.
- [x] When $\Delta R_{\text{5h}} < 0.05$, $\Delta R_{\text{7d}} \le 0$, or across 5h window resets, the learned $C_{\text{cycle}}$ is strictly preserved and does NOT reset to 0.15.
- [x] State cache writes atomically via temp-file + rename, preventing cache corruption on crash.

## Required Tests
- [x] Unit tests for cold-start initialization.
- [x] Unit tests for zero consumption (preserving learned value).
- [x] Unit tests for consumption under 5% threshold (preserving learned value).
- [x] Unit tests for 5h window reset boundary (preserving learned value).
- [x] Unit tests for valid observation update and multi-step EMA convergence.
- [x] Unit tests for upper clamp (0.30) and lower clamp (0.08).
- [x] Persistence test: SaveAtomic to disk, reload from disk, verify state integrity.
- [x] Automated test coverage $\ge 90\%$ for `internal/state` (achieved 93.1%).
