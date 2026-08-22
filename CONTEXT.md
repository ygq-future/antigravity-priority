# Antigravity Priority Plugin

An intelligent quota pacing, adaptive burn-rate estimation, and priority scheduler designed exclusively for Google Antigravity credentials within CLIProxyAPI (CPA).

## Language

### Quota Windows & Capacity

**Short Window (5h)**:
The 5-hour rolling quota bucket allocated by Antigravity, representing short-term burst usage capacity.
_Avoid_: Rate limit, minute window, short cache

**Long Window (7d / Weekly)**:
The 7-day weekly total quota pool allocated by Antigravity for an account.
_Avoid_: Monthly quota, account balance, total tokens

**Model Group**:
The upstream independent quota计量 unit in Antigravity, either `gemini` (Gemini models) or `claude_gpt` (Claude and GPT models).
_Avoid_: Provider, engine, model name

### Scheduling & Pacing

**Cycle Burn Rate ($C_{\text{cycle}}$)**:
The fraction of the total weekly quota capacity that can be consumed within a single saturated 5-hour short window.
_Avoid_: Burn speed, consumption coefficient, usage weight

**Dynamic Boost Horizon ($T_{\text{required}}$)**:
The minimum physical time in hours required to consume all remaining weekly quota given the short-window bottleneck; triggers dynamic 999 priority boost when remaining time is less than or equal to this duration.
_Avoid_: Boost threshold, near-reset deadline, emergency window

**Weekly Urgency Index**:
The mathematical ratio of remaining weekly quota proportion to remaining hours until weekly reset ($\text{Urgency}_{\text{weekly}} = R_{\text{7d}} / \max(T_{\text{7d}}, 0.5)$), representing unit-time burn pressure.
_Avoid_: Priority score, account rank, sort index

**Equal Priority Clustering (Priority Bucketing)**:
Grouping credentials with near-identical Weekly Urgency metrics ($\Delta \text{Urgency} \le \text{UrgencyTolerance}$) into the same priority integer tier, enabling CPA to perform round-robin load balancing across healthy peers instead of single-point saturation.
_Avoid_: Random priority, flat priority, forced unique decrement

**Urgency Tolerance**:
The numerical delta threshold $\Delta \text{Urgency}$ (default 0.05) below which adjacent credentials are assigned identical priority scores.
_Avoid_: Margin of error, floating threshold, priority gap

**Fresh Evidence**:
Verified quota observation data obtained from a successful probe in the current scheduling round. A failed probe is unknown rather than quota depletion and cannot authorize a quota-driven Host Transition.
_Avoid_: Cached state, stale data, fallback record

### Quota Depletion & Cooldown States

**Host Transition**:
A deliberate change to one credential's persisted priority and/or disabled state in CPA Host, including scheduled Apply, 429 Reactive Cooldown, and priority reset. All target fields belong to one transition outcome, whose success is determined by the resulting credential state rather than request completion alone.
_Avoid_: Host write, patch operation, mutation

**429 Reactive Cooldown (Circuit Breaker)**:
Temporarily demoting an account's priority to `-1` upon encountering an upstream Google 429 Rate Limit error for a configurable duration (default 5 minutes), isolating the credential into the bottom fallback tier while preserving its enabled state.
_Avoid_: Ban, account punishment, hard disable

**Soft Depletion**:
Setting priority to `-1` while maintaining `disabled = false` when the 5-hour short window is exhausted ($R_{\text{5h}} \le 0$), enabling automatic self-healing upon the 5-hour reset.
_Avoid_: Temporary ban, mute, hard disable

**Hard Depletion**:
Setting priority to `-1` and marking `disabled = true` in host state when the 7-day weekly quota is completely exhausted ($R_{\text{7d}} \le 0$).
_Avoid_: Account ban, credential deletion, permanent lockout
