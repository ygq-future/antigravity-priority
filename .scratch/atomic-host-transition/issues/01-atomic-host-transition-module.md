# 01: Atomic Host Transition Module

**What to build:** Give CPA administrators one trustworthy Host Transition for each credential: all requested priority and disabled changes commit together, preserve unrelated credential data, respect newer Host state, and return an outcome that describes the state CPA can actually read.

**Blocked by:** None (can start immediately).

**Status:** completed

- [x] A transition expresses priority and disabled independently as `unchanged`, `set`, or `unset`; zero values never imply an operation.
- [x] All target fields for one credential are applied through one document replacement, with unknown and unrelated fields preserved.
- [x] An already-satisfied target returns `no_change` without an unnecessary document replacement.
- [x] A decision-relevant Host change after planning returns `conflict` and is not overwritten, while unrelated metadata changes are preserved.
- [x] A committed document is reread and verified; an unprovable final state returns `uncertain` rather than a false success.
- [x] Cancellation before the commit point returns `failed`; cancellation after the commit point does not negate a committed Host state.
- [x] One credential failure, conflict, or uncertainty does not prevent later credentials in the same round from executing.
- [x] Host outcomes are limited to `no_change`, `committed`, `failed`, `conflict`, and `uncertain`, with stable reasons carrying the detailed cause.
- [x] Round statistics are derived from credential outcomes rather than independently incremented counters.
- [x] Tests use real temporary credential documents through the Host Transition module interface and do not assert on private helper calls.

## Comments

- Completed in commit `1fe981c`; atomic document replacement, explicit field operations, conflict handling, verification, cancellation semantics, credential independence, and derived outcomes are covered by the Host Transition tests.
