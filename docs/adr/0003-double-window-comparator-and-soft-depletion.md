# 0003: Double-Window Comparator Hierarchy and Soft Depletion

## Context
Antigravity credentials have both a 5-hour short window and a 7-day weekly window. In previous implementations, exhaustion of either window resulted in identical treatment or a blunt 24-hour end-of-week boost that resulted in massive quota waste. Furthermore, hard-disabling credentials when short windows depleted required manual intervention or broke automatic rotation.

## Decision
Establish a strict 3-tier comparator hierarchy:
1. **Tier 1 (Boosted)**: Credentials with $T_{\text{7d}} \le T_{\text{required}}$ receive top priorities (`999, 998...`), ordered descending by Weekly Urgency ($\text{Urgency}_{\text{weekly}} = R_{\text{7d}} / \max(T_{\text{7d}}, 0.5)$).
2. **Tier 2 (Regular Active)**: Healthy credentials receive standard priorities (`100, 99...`), ordered descending by Weekly Urgency, tie-broken by shortest remaining 5h reset time ($T_{\text{5h}}$ ascending to exhaust near-refresh short windows), then AuthIndex.
3. **Tier 3 (Depleted)**:
   - Weekly quota depleted ($R_{\text{7d}} \le 0$): `priority = -1, disabled = true` (hard disable until next week).
   - 5h short window depleted ($R_{\text{5h}} \le 0$ but $R_{\text{7d}} > 0$): `priority = -1, disabled = false` (soft depletion, allowing silent automatic recovery on the next 5-hour reset).

## Consequences
- Maximizes quota utilization across the entire weekly cycle without end-of-week overflow.
- Short-window rate limits recover automatically without writing disruptive hard-disables into the host.
