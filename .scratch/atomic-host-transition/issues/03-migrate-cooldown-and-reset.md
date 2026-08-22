# 03: Migrate Cooldown and Reset to Host Transitions

**What to build:** Give 429 Reactive Cooldown and priority reset the same atomic, truthful Host Transition behaviour as scheduled Apply while preserving their distinct user-facing semantics.

**Blocked by:** 01 — Atomic Host Transition Module.

**Status:** ready-for-agent

- [ ] A 429 observation is persisted through the existing cooldown mechanism before its Host Transition is attempted.
- [ ] 429 Reactive Cooldown sets priority to `-1` and leaves disabled unchanged.
- [ ] A failed cooldown Host Transition does not erase or invalidate the persisted cooldown fact.
- [ ] Priority reset removes the priority field and leaves disabled unchanged for every eligible credential.
- [ ] Reset reports every attempted credential, including failures, instead of treating the success count as the attempt count.
- [ ] Cooldown and Reset execute under Runtime's single-flight policy and do not use direct legacy field-patching paths.
- [ ] One credential failure does not stop later credentials, and neither operation performs a same-round automatic retry.
- [ ] Deterministic Runtime tests cover cooldown success and failure, reset success and partial failure, and preservation of disabled state.
