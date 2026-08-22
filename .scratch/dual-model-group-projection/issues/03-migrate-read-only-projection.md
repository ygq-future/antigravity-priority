# 03: Use Shared Projection for Read-Only Views

**What to build:** Route SyncHost, post-reset projection, LatestSnapshot, and Control Model Group configuration changes through the shared projection semantics, giving every read-only path the same complete two-group shape and truthful Target/Predicted roles.

**Blocked by:** 01 — Project Control and Predicted Model Groups; Fresh Evidence Authority 03 — Migrate read-only and operational paths.

**Status:** ready-for-agent

- [ ] SyncHost recomputes both groups from current Host state and classified read-only evidence without Google requests or Host writes.
- [ ] Reset rebuilds both projections from post-reset Host state while keeping the reset operation independent from quota authority.
- [ ] LatestSnapshot returns the most recent shared projection or a stable two-group empty state with the configured control identifier.
- [ ] Repeated reads retain the original projection-generation time.
- [ ] Saving a new Control Model Group performs no Host write, and the next synchronized projection assigns Target and Predicted roles to the correct stored group contents.
- [ ] Management behaviour preserves the user's selected display group while Refresh recomputes both groups under the configured control authority.
- [ ] Runtime and Management tests cover both control directions, startup empty state, historical observations, reset state, configuration changes, and view-only switching.
