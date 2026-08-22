# 03: Consolidate Config workflows

**What to build:** Give Config complete ownership of its markup, styles, behaviour, and translations while preserving the full administrator workflow for loading, editing, validating, saving, and resetting Dynamic Config and schedule settings.

**Blocked by:** 01 / Establish the management page assembly seam.

**Status:** ready-for-agent

- [ ] Config owns its user-interface assets as one feature slice behind the shared assembly seam.
- [ ] Existing Dynamic Config and schedule values load into the same controls with the same defaults and display semantics.
- [ ] Editing and language switching preserve the administrator's current form values.
- [ ] Validation retains the existing accepted values, rejected values, and user feedback before any save request is sent.
- [ ] Saving sends the existing Management requests and reflects success or failure through the existing page feedback.
- [ ] Reset restores the existing configured defaults only after the current confirmation flow succeeds.
- [ ] Config remains usable under both supported languages, CPA theme synchronization, explicit theme preference, and responsive layouts.
- [ ] Behaviour tests cover load, edit, validation, save, and reset through observable form and request outcomes.
- [ ] Superseded Config-specific technical-layer fragments and equivalent implementation-coupled tests are contracted after coverage moves to the feature seam.
