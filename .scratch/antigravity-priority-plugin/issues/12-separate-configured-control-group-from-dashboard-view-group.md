# 12 — Separate Configured Control Group from Dashboard View Group

**Status:** completed

**What to fix:** Treat the configured Antigravity model group as the sole write-back/control authority and the dashboard selector as a read-only choice of which already-calculated group to display.

## Confirmed Requirements
- `antigravity_model_group` from Dynamic Config remains the control group until the user saves a new configuration value.
- Switching the overview selector between `gemini` and `claude_gpt` must not change `active_model_group`, write-back authority, or Target/Predicted semantics.
- Clicking Refresh while viewing the alternate group must synchronize Host data and recompute both groups while preserving the configured control group.
- DevServer must not relabel a fixed Gemini primary snapshot as Claude/GPT or swap group contents.

## Acceptance Criteria
- [x] With configured control group `gemini`, selecting Claude/GPT and refreshing still returns `active_model_group = gemini`.
- [x] Gemini cards remain Target and Claude/GPT cards remain Predicted until configuration is explicitly changed.
- [x] Group payloads retain their correct PlanType, target priorities, and evidence after refresh.
- [x] Browser/UI and Runtime tests cover both control-group directions.
