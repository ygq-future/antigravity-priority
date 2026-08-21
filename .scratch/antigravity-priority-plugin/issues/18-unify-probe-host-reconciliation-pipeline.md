# 18 — Unify Probe and Host Reconciliation Pipeline

**Status:** ready-for-agent

**What to fix:** Give automatic scheduling and manual quota refresh one Runtime-owned pipeline that preserves the required pre-probe Host discovery and performs a second authoritative Host reconciliation before planning, rendering, or writing.

## Canonical Pipeline
1. Pre-probe CPA Host synchronization: enumerate the current Antigravity credentials and obtain the authentication material required for Google requests. Disabled credentials remain included.
2. Force Google quota probing for the selected/all required credentials and persist both model groups plus learned history.
3. Post-probe CPA Host synchronization: re-read the current inventory and mutable Host fields to detect additions, deletions, priority changes, and disabled-state changes made during probing.
4. Combine post-probe Host state with current-round evidence and historical learned state; calculate both model-group Plans.
5. For Auto/Manual Apply, write the configured control group's eligible changes. For quota refresh, return the recomputed dual-group snapshot without host mutation.

## Reconciliation Rules
- Credentials deleted during probing are removed from the final Plan and must not be written.
- Credentials added during probing are shown after reconciliation but cannot be promoted/written without current-round Fresh Evidence.
- Priority/disabled changes made during probing become the `current` state used by Planner and Apply.
- Manual quota refresh and automatic scheduling must not maintain divergent copies of this ordering logic.

## Acceptance Criteria
- [ ] Ordered mocks prove both Host synchronizations occur around Google probing.
- [ ] A host credential added during probe appears in the returned snapshot without unsafe promotion.
- [ ] A host credential deleted during probe receives no write attempt.
- [ ] A manual disabled/priority change during probe is preserved as final current state.
- [ ] Quota Refresh executes no Host patch; Auto Apply patches only after reconciliation and planning.
