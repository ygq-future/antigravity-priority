# 14 — Diagnostics Latest Write Health Must Use Latest Apply

**Status:** ready-for-agent

**What to fix:** Ensure the diagnostics "latest write health" panel is sourced from the most recent host-writing Apply execution, not from a later Probe, Sync, or other read-only operation.

## Confirmed Requirements
- A quota refresh must not replace the displayed success/failure/skip counts of the latest Apply.
- Probe, Sync, Reset, and Apply results retain distinct operation kinds and timestamps.
- The panel's counts, status badge, timestamp, and audit message must all come from the same Apply record.
- When no Apply has occurred, show an explicit no-write-history state rather than borrowing another operation's timestamp.

## Acceptance Criteria
- [ ] Perform a successful Apply, then Probe; diagnostics continues to show the Apply counts and Apply timestamp.
- [ ] An Apply failure remains visible after subsequent read-only operations.
- [ ] With Probe-only history, the latest-write panel shows "no Apply yet".
- [ ] Runtime and template tests cover mixed operation histories.
