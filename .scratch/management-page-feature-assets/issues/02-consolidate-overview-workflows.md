# 02: Consolidate Overview workflows

**What to build:** Give Overview complete ownership of its markup, styles, behaviour, and translations while preserving the operational workflows CPA administrators use to inspect Model Groups and safely run Probe, Apply, Reset, refresh, and Diff confirmation.

**Blocked by:** 01 / Establish the management page assembly seam.

**Status:** completed

- [x] Overview owns its user-interface assets as one feature slice behind the shared assembly seam.
- [x] Control Model Group and Predicted Model Group presentation remains unchanged, including view switching and read-only Predicted behaviour.
- [x] Probe, refresh, Apply, and Reset continue to call the existing Management routes with the existing request semantics.
- [x] Apply reaches its Management action only after the existing two-stage Diff confirmation succeeds.
- [x] Cancelled or dismissed confirmation leaves Host state unchanged and sends no Apply action.
- [x] Overview refresh preserves the user's selected display group while configuration remains the Control Model Group authority.
- [x] Overview remains usable under both supported languages, CPA theme synchronization, explicit theme preference, and responsive layouts.
- [x] Behaviour tests assert observable Overview outcomes through the assembled page rather than private helper names or source substrings.
- [x] Superseded Overview-specific technical-layer fragments and equivalent implementation-coupled tests are contracted after coverage moves to the feature seam.

## Comments

- Completed Overview asset ownership and retained the Model Group, Probe, Apply/Diff, Reset, refresh, and display-state workflows.
- Browser smoke verified the Overview interaction paths in the integrated `/status` page.
