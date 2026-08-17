# 0002: Adaptive Self-Learning Cycle Burn Rate ($C_{\text{cycle}}$)

## Context
Antigravity's throughput is physically bounded by its 5-hour short window. Calculating when to trigger dynamic boost ($T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5\text{h}$) requires knowing $C_{\text{cycle}}$ (the fraction of weekly quota burnable per 5 hours). Because Google only returns remaining percentages rather than absolute token numbers, and different account subscription tiers exhibit slight variations, requiring users to manually guess and configure $C_{\text{cycle}}$ is error-prone.

## Decision
Implement an adaptive online estimator for $C_{\text{cycle}}$ initialized with a safe baseline default of 0.15 (15%). On each probe cycle where observable consumption occurs ($\Delta R_{\text{5h}} \ge 5\%$), compute instantaneous burn ratio $C_{\text{obs}} = \Delta R_{\text{7d}} / \Delta R_{\text{5h}}$ and update a per-credential smoothed value via Exponential Moving Average ($\alpha = 0.3$) clamped to the safe physical interval $[0.08, 0.30]$. Persist this learned value in `refresh-cache.json`.

## Consequences
- Eliminates manual user configuration burden entirely.
- Dynamic Boost Horizon automatically tunes itself to each credential's real-world subscription capacity and usage patterns.
- Cold-start credentials use the robust 0.15 default until enough usage history is collected.
