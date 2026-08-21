# 15 — Urgency Tolerance Zero Means Exact Clustering

**Status:** ready-for-agent

**What to fix:** Preserve `urgency_tolerance = 0` as a valid, explicit value meaning no non-zero score difference may share a priority bucket.

## Confirmed Requirements
- Valid range is inclusive: `0.0 <= urgency_tolerance <= 0.5`.
- `0` does not mean "use the default 0.05".
- With tolerance 0, only credentials with exactly equal clustering scores may receive the same priority tier; any positive difference creates a new tier.
- UI value, persisted Dynamic Config, Runtime Config, diagnostics, and Planner options must agree exactly.
- Default remains 0.05 only when no value has ever been configured or defaults are explicitly restored.

## Acceptance Criteria
- [ ] Saving 0 returns and persists 0 and Runtime uses 0.
- [ ] Planner test proves score deltas of 0 share a tier while any positive delta does not.
- [ ] Values below 0 or above 0.5 are rejected consistently by frontend and backend.
- [ ] Reload/reconfigure does not normalize an explicitly saved 0 to 0.05.
