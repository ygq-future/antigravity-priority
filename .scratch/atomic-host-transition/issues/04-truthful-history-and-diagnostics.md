# 04: Truthful Execution History and Diagnostics

**What to build:** Show CPA administrators one consistent, redacted account of what ordinary Apply, 429 Reactive Cooldown, and priority reset actually did, with Host truth kept separate from execution-record persistence health.

**Blocked by:** 02 — Migrate Scheduled Apply to Host Transitions; 03 — Migrate Cooldown and Reset to Host Transitions.

**Status:** ready-for-agent

- [ ] All three Host-changing paths project the same credential-level outcome vocabulary into execution history and diagnostics.
- [ ] Host outcome and record-persistence outcome remain independently observable; a persistence failure does not rewrite an already committed Host outcome as failed.
- [ ] Attempted, committed, no-change, failed, conflicting, and uncertain totals are derived from transition details.
- [ ] The existing bounded display-history behaviour is preserved without introducing a new persistence path or recovery journal.
- [ ] Duplicate post-write serialization and state-cache persistence branches are replaced by one result-projection path.
- [ ] Management responses and stored history redact authentication material and sensitive identifiers consistently.
- [ ] Persistence failures and uncertain Host outcomes are visible in diagnostics rather than silently ignored.
- [ ] Behavioural tests cover all three paths, derived totals, redaction, and record-persistence failure after a committed Host Transition.
