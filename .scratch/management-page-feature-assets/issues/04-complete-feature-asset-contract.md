# 04: Complete the management page feature-asset contract

**What to build:** Complete feature ownership for History, Diagnostics, and Help, then deliver one verified management-page asset contract in which every user feature is assembled through `StatusHTML` and no caller depends on the former technical-layer organization.

**Blocked by:** 02 / Consolidate Overview workflows; 03 / Consolidate Config workflows.

**Status:** ready-for-agent

- [ ] History owns its markup, styles, behaviour, and translations while retaining existing refresh, detail, and presentation behaviour.
- [ ] Diagnostics owns its markup, styles, behaviour, and translations while retaining existing health, scheduler, cooldown, audit, and copy actions.
- [ ] Help owns its markup, styles, behaviour, and translations while retaining its existing content and navigation behaviour.
- [ ] All five functional areas are present, reachable, and assembled through the same deterministic `StatusHTML` seam.
- [ ] Shared Shell code contains only capabilities genuinely reused across features, with feature-specific state and actions remaining locally owned.
- [ ] The former technical-layer assembly fragments and their superseded implementation-coupled tests are fully contracted.
- [ ] Final-resource checks cover syntax, handler resolution, DOM references, bilingual i18n references, theme tokens, CSS variables, self-containment, and route delivery.
- [ ] Focused browser verification confirms navigation, theme, language, Overview operations, Config save/reset, History, Diagnostics, Help, and responsive presentation in the existing CPA-compatible environment.
- [ ] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.
- [ ] The delivered page requires no CPA runtime asset build or package installation and preserves existing Management contracts.
