# 03: Migrate Cooldown and Reset to Host Transitions

**What to build:** Give 429 Reactive Cooldown and priority reset the same atomic, truthful Host Transition behaviour as scheduled Apply while preserving their distinct user-facing semantics.

**Blocked by:** 01 — Atomic Host Transition Module.

**Status:** completed

- [x] A 429 observation is persisted through the existing cooldown mechanism before its Host Transition is attempted.
- [x] 429 Reactive Cooldown sets priority to `-1` and leaves disabled unchanged.
- [x] A failed cooldown Host Transition does not erase or invalidate the persisted cooldown fact.
- [x] Priority reset removes the priority field and leaves disabled unchanged for every eligible credential.
- [x] Reset reports every attempted credential, including failures, instead of treating the success count as the attempt count.
- [x] Cooldown and Reset execute under Runtime's single-flight policy through the common transition lifecycle.
- [x] One credential failure does not stop later credentials, and neither operation performs a same-round automatic retry.
- [x] Deterministic Runtime tests cover cooldown success and failure, reset success and partial failure, and preservation of disabled state.

## Comments

- Completed in commit `1fe981c`; cooldown facts are persisted before Host mutation, while cooldown and reset use explicit operational intents with independent credential outcomes.
