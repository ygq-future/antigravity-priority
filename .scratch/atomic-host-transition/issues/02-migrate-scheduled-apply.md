# 02: Migrate Scheduled Apply to Host Transitions

**What to build:** Make Manual Apply and Auto Apply execute Planner decisions through the atomic Host Transition lifecycle, so that quota-based scheduling keeps all existing safety gates while producing truthful per-credential Host outcomes.

**Blocked by:** 01 — Atomic Host Transition Module.

**Status:** ready-for-agent

- [ ] Immutable Planner changes are converted into Host Transition intents without moving quota-based calculations out of Planner.
- [ ] Fresh Evidence, ForceWrite, minimum-change, disabled-state, and cooldown gates retain their existing behaviour.
- [ ] Manual Apply and Auto Apply continue to execute under Runtime's single-flight policy.
- [ ] Priority and disabled changes for one credential no longer use separate physical write calls.
- [ ] A failed, conflicting, or uncertain credential does not prevent later credentials from being processed.
- [ ] The current-round Host reconciliation before planning and write-back remains intact.
- [ ] Probe and read-only synchronization remain non-writing operations.
- [ ] High-seam Runtime tests prove both Manual Apply and Auto Apply route through the new transition lifecycle without inspecting its implementation.
