# 03: Consolidate Config workflows

**What to build:** Give Config complete ownership of its markup, styles, behaviour, and translations while preserving the full administrator workflow for loading, editing, validating, saving, and resetting Dynamic Config and schedule settings.

**Blocked by:** 01 / Establish the management page assembly seam.

**Status:** completed

- [x] Config owns its user-interface assets as one feature slice behind the shared assembly seam.
- [x] Existing Dynamic Config and schedule values load into the same controls with the same defaults and display semantics.
- [x] Editing and language switching preserve the administrator's current form values.
- [x] Validation retains the existing accepted values, rejected values, and user feedback before any save request is sent.
- [x] Saving sends the existing Management requests and reflects success or failure through the existing page feedback.
- [x] Reset restores the existing configured defaults only after the current confirmation flow succeeds.
- [x] Config remains usable under both supported languages, CPA theme synchronization, explicit theme preference, and responsive layouts.
- [x] Behaviour tests cover load, edit, validation, save, and reset through observable form and request outcomes.
- [x] Superseded Config-specific technical-layer fragments and equivalent implementation-coupled tests are contracted after coverage moves to the feature seam.

## Comments

- Completed Config asset ownership, including Dynamic Config load/edit/validation/save/reset flow preservation.
- Browser smoke verified range validation, save, and confirmed reset feedback in the integrated page.
