# 02: Use Shared Projection for Quota Runs

**What to build:** Route Production, Manual Apply, Auto Apply, and Probe through the shared dual-group projection after post-probe Host reconciliation, so one quota request updates both group views and only the Control Model Group Plan can reach Host write-back.

**Blocked by:** 01 — Project Control and Predicted Model Groups; Fresh Evidence Authority 02 — Migrate quota runs to the Fresh Evidence authority.

**Status:** completed

- [x] Production creates one complete dual-group Snapshot from the post-probe Host inventory and the two independently classified Model Groups.
- [x] Manual Apply and Auto Apply send only the returned Control Model Group Plan to the Host Transition flow.
- [x] Predicted items and changes remain visible for comparison and are unavailable to Host write-back.
- [x] Probe updates both group projections from the single Antigravity response and performs no Host mutation.
- [x] A failure or missing observation in one group remains local to that group and does not borrow data from the other group.
- [x] Runtime behavioural tests cover both configured control directions, complete two-group results, post-probe reconciliation, Apply authority, and non-writing Probe behaviour.

## Completion

Production and Probe now create one shared projection after post-probe Host reconciliation. Only `ControlPlan` is handed to Host Transition; the predicted plan is intentionally internal to the projection result.
