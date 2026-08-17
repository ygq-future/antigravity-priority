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

**Fresh Evidence**:
Verified quota observation data obtained from a successful probe in the current scheduling round.
_Avoid_: Cached state, stale data, fallback record

### Quota Depletion States

**Soft Depletion**:
Setting priority to `-1` while maintaining `disabled = false` when the 5-hour short window is exhausted ($R_{\text{5h}} \le 0$), enabling automatic self-healing upon the 5-hour reset.
_Avoid_: Temporary ban, mute, hard disable

**Hard Depletion**:
Setting priority to `-1` and marking `disabled = true` in host state when the 7-day weekly quota is completely exhausted ($R_{\text{7d}} \le 0$).
_Avoid_: Account ban, credential deletion, permanent lockout
