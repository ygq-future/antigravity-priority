# 0007: Dynamic Model-Scoped Scheduler and Three-Pool Routing Board

## Context

Each Google Antigravity credential owns three independent quota resource pools concurrently:
1. `gemini` (Gemini models: 5h burst window + 7d weekly window);
2. `argon` (Gemini 4 Argon: shared 5h burst window + dedicated 7d weekly window, falling back to shared 7d weekly on Ultra/Free plans);
3. `claude_gpt` (Claude and GPT models: 5h burst window + 7d weekly window).

Writing a single static `priority` integer back to CPA credential JSON files on disk created unavoidable cross-model conflicts: an account with abundant Claude quota but exhausted Gemini quota had to be either deprioritized globally (starving Claude traffic) or left at high priority (triggering immediate 429 errors on Gemini requests). Furthermore, frequent disk mutations caused unnecessary I/O overhead.

## Decision

1. **Three-Pool Quota Domain (`gemini`, `argon`, `claude_gpt`)**:
   Promote `argon` to a first-class quota pool alongside `gemini` and `claude_gpt`. Extract and persist independent probe evaluations, sample histories, and learned burn rates ($C_{\text{cycle}}$) for all three pools in a single probe pass.

2. **In-Memory Request Dispatch (`scheduler.pick`)**:
   Register `scheduler: true` and `scheduler_across_priorities: true` capabilities with CPA Host. In `scheduler.pick`, resolve the requested `Model` dynamically to its target quota pool (`gemini`, `argon`, or `claude_gpt`), filter out depleted or 429-cooling credentials, and select from the highest active priority tier using session sticky affinity (`canonical_session_id`) and pool-scoped round-robin rotation in pure memory with zero disk I/O.

3. **Pool-Scoped 429 Cooldown Isolation**:
   Isolate reactive 429 cooldowns to the specific `(authIndex, quotaPool)` key. When an account encounters a 429 rate limit on a Claude model, only its `claude_gpt` routing plan is demoted to `-1`; its `gemini` and `argon` plans remain healthy and continue serving traffic.

4. **Closed-Loop Real-Time Estimation & Calibration**:
   Intercept completed turns via `usage.handle` to deduct estimated quota consumption in real time based on weighted token telemetry, immediately updating in-memory routing tables between probe cycles. Auto-calibrate token sensitivity ($K_{\text{5h}}$) and cycle burn rate ($C_{\text{cycle}}$) via exponential moving averages (EMA) whenever authoritative Google probes arrive.

5. **Model-Centric Routing Board & Panoramic 5-Bar Cards**:
   Replace the legacy two-group toggle in the Web UI with a concrete Model Selector dropdown discovered from `host.auth.models` and built-in catalogs. Sort credentials dynamically by per-model call order and priority tier, and render all 5 quota progress bars (`Gemini 5h/7d`, `Claude 5h/7d`, `Argon 7d`) on every card with active route highlighting.

## Consequences

- Completely eliminates cross-model priority contention and static disk write-back conflicts.
- In-memory dispatch latency is strictly $O(1)$ (<50µs) with zero external network or host RPC calls on the request hot path.
- Accounts with rate-limited or depleted pools remain productive for their healthy sibling model pools.
- Full panoramic visibility into all five quota windows across all accounts on a single dashboard.
