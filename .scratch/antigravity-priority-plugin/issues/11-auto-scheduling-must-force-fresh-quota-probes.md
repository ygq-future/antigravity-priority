# 11 — Auto Scheduling Must Force Fresh Quota Probes

**Status:** completed

**What to fix:** Every automatic scheduling round must obtain current Google quota evidence before calculating or writing target priorities; a successful cached observation from an earlier round is not sufficient.

## Confirmed Requirements
- Every round begins by synchronizing the CPA Host credential inventory and reading the authentication material required for probing; this pre-probe Host synchronization must not be removed.
- `TriggerAutoApply` forces a Google quota probe for every eligible Antigravity credential in the round.
- The algorithm may use historical samples and learned Cycle Burn Rate, but the current-round 5h/7d remaining values must come from the fresh probe.
- Disabled Host credentials remain probe-eligible; disabled state affects planning/write-back, not quota observation.
- After probing, the Runtime synchronizes the CPA Host again before planning and write-back so additions, deletions, priority changes, and disabled-state changes made during the probe are observed.
- A credential added after the pre-probe synchronization has no current-round Fresh Evidence and must not be promoted or written from stale/absent evidence; a credential deleted during probing must not be written.
- Probe failure must follow the defined safe-degradation policy and must never promote from stale evidence.

## Acceptance Criteria
- [x] A Runtime test seeds fresh cached evidence, runs `AutoApply`, and proves Google HTTP is still called.
- [x] A 5-minute auto interval produces a fresh `ObservedAt` every successful round rather than reusing the 15-minute cache TTL.
- [x] Disabled credentials are included in automatic quota probing.
- [x] Tests prove the order `Host sync -> Google probe -> Host sync -> Plan -> Apply`.
- [x] Add/delete/priority/disabled mutations injected during probing are reflected safely in the final Plan and Apply set.
- [x] Planning and host write-back occur only after the current round's probe collection and post-probe Host synchronization complete.
