# 0004: Equal Priority Clustering and 429 Reactive Cooldown

## Context
When multiple healthy credentials possess nearly identical quota balances and Weekly Urgency scores, strictly decrementing priorities (e.g. `100, 99, 98...`) causes CLIProxyAPI (CPA) to route 100% of incoming traffic to the single highest-priority account (`100`). Under high concurrency, this single account rapidly exhausts its upstream Google RPM/TPM rate limits and returns HTTP 429, while the remaining healthy accounts (`99, 98...`) sit idle. Furthermore, when CPA experiences a 429 error on an account, repeatedly hitting it without a cooling-off period causes compounding retries and service latency.

## Decision
1. **Equal Priority Clustering (Priority Bucketing)**:
   Group credentials whose Weekly Urgency metrics differ by less than a configurable tolerance ($\Delta \text{Urgency} \le \text{UrgencyTolerance}$, default `0.05`) into the same priority integer tier (e.g. all assigned `100`). This allows CPA to perform native round-robin load balancing across all credentials within the same urgency tier, distributing burst concurrency evenly. Priority decrements only occur when the metric delta exceeds the tolerance threshold.

2. **429 Reactive Cooldown Circuit Breaker**:
   Upon detecting an upstream 429 rate limit error (via CPA filter event hooks or probe responses), immediately demote the affected credential's priority to `-1` (soft depletion, keeping `disabled = false`) for a configurable duration (`RateLimitCooldownDuration`, default 5 minutes). This moves the credential to the bottom fallback tier behind all active (`100`) and unconfigured (`0`) accounts, preventing repeated retries while preserving emergency fallback availability and eliminating destructive hard-disabling conflicts.

## Consequences
- Burst concurrency is evenly distributed across all healthy peer credentials rather than saturating a single account.
- 429 errors trigger immediate automatic traffic isolation without human intervention.
- The 429 cooldown is completely non-destructive: credentials automatically recover their calculated priority once the cooldown timer expires.
- Both `UrgencyTolerance` and `RateLimitCooldownDuration` are dynamically configurable via the Web UI Config Center with zero restart required.
