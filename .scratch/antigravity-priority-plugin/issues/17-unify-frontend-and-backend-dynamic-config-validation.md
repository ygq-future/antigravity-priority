# 17 — Unify Frontend and Backend Dynamic Config Validation

**Status:** ready-for-agent

**What to fix:** Establish one canonical Dynamic Config contract and make backend validation, frontend controls, reset defaults, API responses, README, and README.en agree exactly.

## Canonical Bounds
- `max_concurrency`: integer `1..32`.
- `min_change`: integer `0..100`.
- `urgency_tolerance`: number `0..0.5`, inclusive; zero retains the exact-zero semantics defined in Ticket 15.
- `rate_limit_cooldown_minutes`: integer `1..1440`.
- `quota_sample_capacity`: integer `2..30`.
- `boost_start_priority` and `normal_start_priority`: integer `1..999`, with `normal_start_priority <= boost_start_priority`.
- `interval`: valid positive Go duration with a minimum of one minute.
- `antigravity_model_group`: `gemini` or `claude_gpt` only.
- Schedule window values follow the existing `HH:MM` and cross-midnight rules.

## Confirmed Requirements
- Backend validation is authoritative; bypassing the UI cannot persist an out-of-contract value.
- Frontend constraints and messages mirror the same contract for immediate feedback.
- A value accepted by the backend must be loadable and saveable again through the UI.
- Remove contradictory public documentation such as `1..32 (backend allows 64)`.
- Defaults remain sourced from `config.Default()` and agree with reset payloads and UI initial values.

## Acceptance Criteria
- [ ] Boundary tests cover minimum, maximum, just-below, and just-above values for every numeric field.
- [ ] Direct `POST /config` rejects `max_concurrency=33`, `min_change=101`, tolerance above 0.5, cooldown above 1440, and inverted priority starts.
- [ ] Frontend and backend accept all canonical boundary values, including tolerance 0.
- [ ] `README.md` and `README.en.md` contain the same non-contradictory ranges.
- [ ] Config GET -> unchanged POST round-trip succeeds for every accepted configuration.
