# 19 — Remove Redundant Priority Rules Enabled Switch

**Status:** completed

**What to fix:** Remove `priority_rules.enabled`; the scheduling algorithm always requires numeric Boost and Regular starting priorities, using configured values with standard defaults of 999 and 100.

## Confirmed Semantics
- The double-window scheduling algorithm remains active whenever scheduling/planning is invoked.
- `boost_start_priority` always has an effective value, default 999.
- `normal_start_priority` always has an effective value, default 100.
- There is no mode where disabling this switch removes target priority calculation.
- Remove the misleading UI toggle and public documentation that describes it as enabling/disabling the double-window algorithm.

## Compatibility
- Existing configurations/cache documents containing `priority_rules.enabled` must continue to load without startup failure.
- Define and test a migration that preserves the effective behavior of legacy `enabled=false` configurations rather than unexpectedly activating previously ignored custom values.
- Newly persisted Dynamic Config and API responses omit the redundant field.

## Acceptance Criteria
- [x] Dynamic Config, Runtime Config, UI form, default/reset payloads, README files, and tests no longer expose an active `priority_rules.enabled` setting.
- [x] Planner always receives explicit effective Boost and Regular starting priorities.
- [x] Missing start values resolve to 999/100 through the canonical default path.
- [x] Legacy `enabled=true` and `enabled=false` documents have deterministic, tested migration behavior.
