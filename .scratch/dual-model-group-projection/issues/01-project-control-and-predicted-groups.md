# 01: Project Control and Predicted Model Groups

**What to build:** Provide one pure projection interface that turns a configured Control Model Group, reconciled Host inventory, classified evidence for both Model Groups, shared planning inputs, and projection time into the only control Plan available to callers plus one complete dual-group Snapshot.

**Blocked by:** Fresh Evidence Authority 01 — Establish the Fresh Evidence authority.

**Status:** completed

- [x] Both control directions produce the configured Control Model Group as Target and the other Model Group as Predicted.
- [x] The returned control Plan belongs to the configured Control Model Group; the Predicted Plan is exposed only through its Snapshot.
- [x] Both groups use the same Host inventory, planning configuration, cooldowns, and projection time while retaining their own evidence and calculated values.
- [x] Missing or failed data remains unknown in its own group, and every result contains both canonical group keys.
- [x] Source observation times remain unchanged while the Snapshot uses one explicit projection-generation time.
- [x] A table-driven contract suite verifies role assignment, group independence, stable empty state, immutability, and both control directions through the projection interface.

## Completion

The pure `runtime.ProjectDualModelGroups` seam now owns control/alternate selection, role marking, complete response assembly, explicit projection time, and startup fallback handling. Runtime projection tests cover both control directions, independent group evidence, missing-group stability, and immutability.
