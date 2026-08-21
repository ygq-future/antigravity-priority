# 16 — Planner Requires Explicit Decision Time

**Status:** ready-for-agent

**What to fix:** Remove the Planner's hidden wall-clock fallback so a Plan is a deterministic function of credentials, quota evidence, learned state, configuration, and an explicit decision time.

## Confirmed Requirements
- `priority.PlanFreshOnly` and its normalization path must not call `time.Now()`.
- Every production caller supplies one UTC `Now` value for the complete planning round.
- Missing/zero decision time is rejected or made impossible by the interface; it must not be silently replaced.
- The explicit decision time used for planning is available to snapshots/audit so a decision can be replayed.

## Acceptance Criteria
- [ ] Static/source test proves `internal/priority` contains no wall-clock read in the planning path.
- [ ] Calling Planner twice with identical inputs and explicit `Now` produces deeply equal Plans.
- [ ] Boundary tests for window reset and Dynamic Boost Horizon use pinned time and are deterministic.
- [ ] All Runtime planning use cases pass the same round timestamp into both model-group calculations.
