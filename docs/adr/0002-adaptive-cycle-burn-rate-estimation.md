# 0002: Adaptive Self-Learning Cycle Burn Rate ($C_{\text{cycle}}$)

## Context
Antigravity's throughput is physically bounded by its 5-hour short window. Calculating when to trigger dynamic boost ($T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5\text{h}$) requires knowing $C_{\text{cycle}}$ (the fraction of weekly quota burnable per 5 hours). Because Google only returns remaining percentages rather than absolute token numbers, and different account subscription tiers exhibit slight variations, requiring users to manually guess and configure $C_{\text{cycle}}$ is error-prone.

## Decision
Implement an adaptive online estimator for $C_{\text{cycle}}$ initialized with a safe baseline default of 0.15 (15%).

To prevent incremental small consumption steps (e.g. $\Delta R_{\text{5h}} < 5\%$ per individual probe interval) from being dropped, maintain one bounded FIFO history of point-in-time quota samples (`Samples []QuotaSample`, configurable capacity $N \in [2, 30]$, default 6) per credential and model group. Each distinct observation receives a monotonically increasing sequence number, while the entry stores `LearningBaselineSequence` as the estimator cursor:
1. **Bounded Historical Record**: Samples remain available for management history and adaptive learning. Reaching capacity is the only reason an old sample is removed.
2. **Zero-Consumption Deduplication**: Consecutive probes with unchanged 5h quota, 7d quota, and short-window reset refresh the latest sample's timestamp without growing the history.
3. **Window Boundary Rebase**: When a 5h rolling window reset occurs (reset time changes or 5h quota replenishes), preserve history and move the learning baseline to the current sample so estimation never spans different short windows.
4. **Multi-Sample Span Delta**: Resolve the baseline by sequence, falling back to the oldest retained sample if FIFO rotation has removed it. When cumulative consumption reaches $\Delta R_{\text{5h}} = (S_{\text{base}}.R_{\text{5h}} - S_{\text{curr}}.R_{\text{5h}}) \ge 5\%$ and $\Delta R_{\text{7d}} > 0$, compute $C_{\text{obs}} = \Delta R_{\text{7d}} / \Delta R_{\text{5h}}$, clamp to the physical interval $[0.08, 0.30]$, and update smoothed $C_{\text{cycle}}$ via Exponential Moving Average ($\alpha = 0.3$). Advance only the baseline cursor to $S_{\text{curr}}$ to prevent double-counting.

Persist this learned state and sample queue in `data/antigravity-priority-cache.json`.

## Consequences
- Eliminates manual user configuration burden entirely.
- Eliminates estimation stalls caused by slow or light consumption spanning across probe intervals.
- Preserves the bounded observation history for quota-trend inspection independently of estimator progress.
- Dynamic Boost Horizon automatically tunes itself to each credential's real-world subscription capacity and usage patterns.
- Cold-start credentials use the robust 0.15 default until enough usage history is collected.
