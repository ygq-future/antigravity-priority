# 04 — Weekly Urgency Planner & Pure Dry-Run

**What to build:** Implement the pure-function priority planner that takes credentials, fresh probe evidence, and learned state, and generates an immutable `Plan` containing target priorities, disabled states, and change records. Delivers the complete business logic for **Dry-Run** without invoking any host write-back operations.

**Blocked by:** 02 (Antigravity Multi-Window Prober & Quota Evidence), 03 (Adaptive Burn Rate Estimator & State Store)

**Status:** completed

## Scope
- Implement priority planning in `internal/priority/` (`planner.go`, `boost.go`, `urgency.go`, `comparator.go`).
- Core mathematical calculations:
  - $\text{Urgency}_{\text{weekly}} = R_{\text{7d}} / \max(T_{\text{7d}}, 0.5)$.
  - $T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5.0$.
  - $\text{IsBoosted} = (T_{\text{7d}} \le T_{\text{required}}) \land (R_{\text{7d}} > 0)$.
- 3-tier comparator hierarchy and assignment:
  - **Tier 1 (Boosted)**: starts at `boost_start_priority` (default 999) and decrements (`999, 998, 997...`), ordered descending by $\text{Urgency}_{\text{weekly}}$.
  - **Tier 2 (Regular)**: starts at `normal_start_priority` (default 100) and decrements (`100, 99, 98...`), ordered descending by $\text{Urgency}_{\text{weekly}} \to T_{\text{5h}}$ ascending $\to$ AuthIndex.
  - **Tier 3 (Depletion & Precedence)**:
    1. If $R_{\text{7d}} \le 0$: `priority = -1, disabled = true` (**weekly hard depletion has highest precedence**).
    2. Else if $R_{\text{5h}} \le 0$: `priority = -1, disabled = false` (**short-window soft depletion**).
- Priority uniqueness enforcement: all enabled credentials in the active model group receive distinct positive priorities, tagging shifted unprobed peers with `ForceWrite = true`.

## Explicit Non-Goals
- No host write-back modification (handled in Ticket 05).
- No HTTP API handling (handled in Ticket 06).

## Acceptance Criteria
- [x] Produces an immutable `Plan` data structure with zero side-effects on host state.
- [x] Boosted tier decrements from 999; no two boosted credentials receive the same priority.
- [x] When both $R_{\text{7d}} \le 0$ and $R_{\text{5h}} \le 0$ occur, weekly hard depletion (`disabled = true`) takes strict precedence over soft depletion.
- [x] Healthy credentials sort deterministically by Weekly Urgency, tie-breaking by earliest 5h reset ($T_{\text{5h}}$ asc), then AuthIndex.
- [x] Positive priorities within the active model group are guaranteed unique.

## Required Tests
- [x] Table-driven test matrix covering all decision branches:
  - Dynamic boost threshold activation boundary.
  - Decrementing boost priority assignment (999, 998...).
  - Weekly urgency ordering.
  - 5h reset countdown tie-breaking.
  - AuthIndex deterministic resolution.
  - Hard vs soft depletion precedence.
  - Priority uniqueness and unprobed peer `ForceWrite` marking.
- [x] Automated test coverage $\ge 90\%$ for `internal/priority` (achieved 97.4%).
