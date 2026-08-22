# 02: Consolidate Overview workflows

**What to build:** Give Overview complete ownership of its markup, styles, behaviour, and translations while preserving the operational workflows CPA administrators use to inspect Model Groups and safely run Probe, Apply, Reset, refresh, and Diff confirmation.

**Blocked by:** 01 / Establish the management page assembly seam.

**Status:** ready-for-agent

- [ ] Overview owns its user-interface assets as one feature slice behind the shared assembly seam.
- [ ] Control Model Group and Predicted Model Group presentation remains unchanged, including view switching and read-only Predicted behaviour.
- [ ] Probe, refresh, Apply, and Reset continue to call the existing Management routes with the existing request semantics.
- [ ] Apply reaches its Management action only after the existing two-stage Diff confirmation succeeds.
- [ ] Cancelled or dismissed confirmation leaves Host state unchanged and sends no Apply action.
- [ ] Overview refresh preserves the user's selected display group while configuration remains the Control Model Group authority.
- [ ] Overview remains usable under both supported languages, CPA theme synchronization, explicit theme preference, and responsive layouts.
- [ ] Behaviour tests assert observable Overview outcomes through the assembled page rather than private helper names or source substrings.
- [ ] Superseded Overview-specific technical-layer fragments and equivalent implementation-coupled tests are contracted after coverage moves to the feature seam.
