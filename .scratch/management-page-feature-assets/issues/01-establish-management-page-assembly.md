# 01: Establish the management page assembly seam

**What to build:** Deliver the existing CPA management page through one deterministic `StatusHTML` assembly seam. Consolidate the Shell capabilities used across features—document frame, navigation, theme, language, notifications, shared requests, controls, and modal foundations—and verify the final delivered resource as one internally complete page.

**Blocked by:** None (can start immediately).

**Status:** completed

- [x] `/status` continues to serve the same self-contained management page contract and content type.
- [x] The assembled resource remains native, inline, CSP-compatible, and free of remote URLs or runtime asset loading.
- [x] Shell navigation, theme preference, language switching, notifications, shared controls, and modal foundations retain their existing user-visible behaviour.
- [x] JavaScript dependency order and CSS cascade order are owned by the assembly implementation rather than repeated by callers.
- [x] The final script is syntax-checked as part of the test suite.
- [x] Automated checks verify that inline handlers resolve, statically determinable DOM references are present, and referenced i18n keys exist in both languages.
- [x] Theme token completeness and CSS variable-definition coverage remain enforced through the final resource.
- [x] Tests cross the assembled `StatusHTML` seam and avoid coupling to private fragment names or exact source spelling.

## Comments

- Completed the deterministic `StatusHTML` seam and moved Shell document assets behind the shared assembly contract.
- Final verification includes `go test ./internal/management`, full repository gates, and focused browser smoke coverage.
