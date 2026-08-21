# 13 — Overview Entry Syncs Host and Replans with Latest Config

**Status:** completed

**What to fix:** Merge the stale-after-config-save and stale-on-return behaviors into one overview refresh contract: entering the dashboard behaves like Refresh by synchronizing current Host state and rebuilding target snapshots with the latest active configuration.

## Confirmed Requirements
- Initial page entry, every return to the Overview tab, and the explicit Refresh button share the same application use case.
- That use case reads the latest CPA Host credential state, combines it with cached quota/history, and runs Planner for both model groups.
- It does not call Google solely for an overview refresh.
- Configuration values that affect planning include at least the control model group, `min_change`, `urgency_tolerance`, and priority start values.
- Saving configuration followed by returning to Overview must display targets computed from the saved values; a stale `/snapshot/latest` read is insufficient.

## Acceptance Criteria
- [x] Change `normal_start_priority` from 100 to 50, save, return to Overview, and observe regular targets recalculated from 50.
- [x] Changing `urgency_tolerance`, `min_change`, or the configured control group is reflected on the next Overview entry without a quota probe.
- [x] Credentials added, removed, disabled, or reprioritized in CPA are reflected on Overview entry.
- [x] Overview entry and explicit Refresh use one Runtime-owned synchronization/planning path.
- [x] Tests prove Overview refresh performs no Google HTTP request.
