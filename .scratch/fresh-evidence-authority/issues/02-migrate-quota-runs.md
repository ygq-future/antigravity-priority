# 02: Migrate quota runs to the Fresh Evidence authority

**What to build:** Route Manual Apply, Auto Apply, and Probe through the Evidence module so current successful probes drive quota planning, failed credentials remain unchanged and visible as failures, successful peers continue, and Probe remains strictly non-writing.

**Blocked by:** 01 — Establish the Fresh Evidence authority.

**Status:** completed

- [x] Manual Apply uses a current probe and leaves a failed credential's Host priority and disabled state unchanged.
- [x] Auto Apply preserves its existing probe-attempt behaviour and leaves a credential unchanged after the final failed attempt.
- [x] One failed credential does not block successfully probed credentials from being planned and applied in the same round.
- [x] Probe records current failure without applying a Host Transition or presenting failure as a target-disabled decision.
- [x] Post-probe Host reconciliation still respects credentials added, removed, disabled, or reprioritized while probing was in flight.
- [x] Runtime-level behavioural tests cover success, failure, mixed results, retries, and the non-writing Probe path through public use cases.
