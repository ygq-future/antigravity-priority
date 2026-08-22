# 04: Complete the management page feature-asset contract

**What to build:** Complete feature ownership for History, Diagnostics, and Help, then deliver one verified management-page asset contract in which every user feature is assembled through `StatusHTML` and no caller depends on the former technical-layer organization.

**Blocked by:** 02 / Consolidate Overview workflows; 03 / Consolidate Config workflows.

**Status:** completed

- [x] History owns its markup, styles, behaviour, and translations while retaining existing refresh, detail, and presentation behaviour.
- [x] Diagnostics owns its markup, styles, behaviour, and translations while retaining existing health, scheduler, cooldown, audit, and copy actions.
- [x] Help owns its markup, styles, behaviour, and translations while retaining its existing content and navigation behaviour.
- [x] All five functional areas are present, reachable, and assembled through the same deterministic `StatusHTML` seam.
- [x] Shared Shell code contains only capabilities genuinely reused across features, with feature-specific state and actions remaining locally owned.
- [x] The former technical-layer assembly fragments and their superseded implementation-coupled tests are fully contracted.
- [x] Final-resource checks cover syntax, handler resolution, DOM references, bilingual i18n references, theme tokens, CSS variables, self-containment, and route delivery.
- [x] Focused browser verification confirms navigation, theme, language, Overview operations, Config save/reset, History, Diagnostics, Help, and responsive breakpoints in the existing CPA-compatible environment.
- [x] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.
- [x] The delivered page requires no CPA runtime asset build or package installation and preserves existing Management contracts.

## Comments

- Completed the feature-asset contract: Overview, History, Diagnostics, Config, and Help now declare complete assets locally, while Shell retains only shared document capabilities.
- Added final-resource workflow coverage for Probe, Apply/Diff confirmation, Reset, Config load/save/reset, handlers, DOM references, bilingual translations, and JavaScript syntax.
- Browser smoke used the integrated local CPA-compatible dev server and exercised both the default layout and compact responsive breakpoints.
