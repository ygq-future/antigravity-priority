# 0002: Adaptive Self-Learning Cycle Burn Rate ($C_{\text{cycle}}$)

## Context
Antigravity's throughput is physically bounded by its 5-hour short window. Calculating when to trigger dynamic boost ($T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5\text{h}$) requires knowing $C_{\text{cycle}}$ (the fraction of weekly quota burnable per 5 hours). Because Google only returns remaining percentages rather than absolute token numbers, and different account subscription tiers exhibit slight variations, requiring users to manually guess and configure $C_{\text{cycle}}$ is error-prone.

## Decision
Implement an adaptive online estimator for $C_{\text{cycle}}$ initialized with a safe baseline default of 0.15 (15%).

To prevent incremental small consumption steps (e.g. $\Delta R_{\text{5h}} < 5\%$ per individual probe interval) from being dropped, maintain a sliding window queue of point-in-time quota samples (`Samples []QuotaSample`, configurable capacity $N \in [2, 30]$, default 6) per credential and model group:
1. **Window Reset Eviction**: When a 5h rolling window reset occurs (reset time changes or 5h quota replenishes), clear the sample queue and establish a fresh baseline.
2. **Zero-Consumption Deduplication**: Consecutive probes with unchanged quota refresh the latest sample's timestamp without growing the queue or evicting older baseline observations.
3. **Multi-Sample Span Delta**: When cumulative 5h consumption across the sliding window reaches $\Delta R_{\text{5h}} = (S_0.R_{\text{5h}} - S_{\text{curr}}.R_{\text{5h}}) \ge 5\%$ and $\Delta R_{\text{7d}} > 0$, compute $C_{\text{obs}} = \Delta R_{\text{7d}} / \Delta R_{\text{5h}}$, clamp to the physical interval $[0.08, 0.30]$, and update smoothed $C_{\text{cycle}}$ via Exponential Moving Average ($\alpha = 0.3$). Advance the baseline to $S_{\text{curr}}$ to prevent double-counting.

Persist this learned state and sample queue in `data/antigravity-priority-cache.json`.

## Consequences
- Eliminates manual user configuration burden entirely.
- Eliminates estimation stalls caused by slow or light consumption spanning across probe intervals.
- Dynamic Boost Horizon automatically tunes itself to each credential's real-world subscription capacity and usage patterns.
- Cold-start credentials use the robust 0.15 default until enough usage history is collected.
