# 02: Migrate Scheduled Apply to Host Transitions

**What to build:** Make Manual Apply and Auto Apply execute Planner decisions through the atomic Host Transition lifecycle, so that quota-based scheduling keeps all existing safety gates while producing truthful per-credential Host outcomes.

**Blocked by:** 01 — Atomic Host Transition Module.

**Status:** completed

- [x] Immutable Planner changes are converted into Host Transition intents without moving quota-based calculations out of Planner.
- [x] Fresh Evidence, ForceWrite, minimum-change, disabled-state, and cooldown gates retain their existing behaviour.
- [x] Manual Apply and Auto Apply continue to execute under Runtime's single-flight policy.
- [x] Priority and disabled changes for one credential use one complete-document replacement.
- [x] A failed, conflicting, or uncertain credential does not prevent later credentials from being processed.
- [x] The current-round Host reconciliation before planning and write-back remains intact.
- [x] Probe and read-only synchronization remain non-writing operations.
- [x] High-seam Runtime tests prove both Manual Apply and Auto Apply route through the transition lifecycle without inspecting its implementation.

## Comments

- Completed in commit `1fe981c`; Manual Apply and Auto Apply now project Planner changes through the common Host Transition lifecycle while preserving Runtime ownership and scheduling gates.
