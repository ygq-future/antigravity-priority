# Spec: Management Page Feature Assets

Status: `ready-for-agent`

## Problem Statement

CPA administrators need the embedded management page to remain reliable while its Overview, History, Diagnostics, Config, and Help capabilities continue to evolve. Today, one user-facing change commonly requires edits across separate markup, style, JavaScript, and i18n files, plus knowledge of global functions, DOM identifiers, script concatenation order, and CSS cascade order.

The current split reduces individual file size but leaves each user feature scattered across shallow modules. Tests primarily inspect source substrings, so they can fail after a harmless rewrite or pass while a handler, selector, translation key, or runtime dependency is broken. Maintainers need each feature's knowledge to be local without changing the page users already depend on.

## Solution

Deepen the management-page implementation around user features while preserving `StatusHTML` as its single small external interface. Overview, History, Diagnostics, Config, and Help each own their markup, style, behaviour, and translations. A small Shell owns only genuine cross-feature capabilities such as navigation, theme and language handling, notifications, shared controls, and modal foundations.

One assembly implementation hides JavaScript dependency order, CSS cascade order, shared globals, and final document construction. Tests cross the highest useful seam—the fully assembled `StatusHTML`—to verify that the delivered resource is self-contained, syntactically valid, internally closed, and capable of its most important user flows. The change is behaviour-equivalent and does not introduce a frontend framework or runtime build chain.

## User Stories

1. As a CPA administrator, I want the management page to keep the same five functional areas, so that the architecture change does not alter how I operate the plugin.
2. As a CPA administrator, I want Overview to retain both Model Group views, so that I can inspect Control and Predicted results as before.
3. As a CPA administrator, I want Probe, Apply, Reset, and Diff confirmation to behave unchanged, so that existing operational workflows remain dependable.
4. As a CPA administrator, I want History and Diagnostics to retain their existing information and actions, so that troubleshooting capability is not reduced.
5. As a CPA administrator, I want Config values and save/reset behaviour preserved, so that internal asset movement cannot change scheduling configuration.
6. As a CPA administrator, I want Chinese and English switching to preserve current values and page state, so that localization does not disrupt my work.
7. As a CPA administrator, I want CPA Host theme synchronization and the existing light/dark preference to remain intact, so that the embedded page stays visually consistent.
8. As a CPA administrator, I want the page to remain responsive in CPA and a direct browser, so that the existing access modes continue working.
9. As a CPA administrator, I want the page to remain self-contained and free of remote dependencies, so that it works under CPA's strict security policy.
10. As an integration client, I want `/status` and Management routes to keep their existing contracts, so that no client migration is required.
11. As a maintainer, I want a user feature's markup, style, behaviour, and translations owned together, so that one change can be understood and verified locally.
12. As a maintainer, I want cross-feature Shell capabilities kept behind a small interface, so that feature modules do not depend on an undocumented web of globals.
13. As a maintainer, I want JavaScript dependency and CSS cascade order hidden by assembly, so that callers do not maintain ordering knowledge.
14. As a maintainer, I want broken handlers, selectors, and translation references detected from the final resource, so that assembly defects fail before release.
15. As a maintainer, I want core behaviours tested through user-observable outcomes, so that internal rewrites do not require unrelated test changes.
16. As a maintainer, I want obsolete substring tests replaced after equivalent contract coverage exists, so that the suite remains focused rather than duplicated.
17. As a maintainer, I want shared code extracted only when multiple features actually use it, so that the reorganization does not create new shallow modules.
18. As a maintainer, I want the page to build and run without a frontend application toolchain, so that CPA deployment remains a single Go-delivered resource.

## Implementation Decisions

1. **Primary module and seam**
   - Keep the fully assembled `StatusHTML` as the management page's only external interface and primary test seam.
   - Deepen one in-process assembly module that owns document composition, style ordering, script ordering, and inclusion of all feature assets.
   - Do not add a port or adapter because the assembly has no external dependency or alternative implementation.

2. **Feature ownership**
   - Organize the implementation around Overview, History, Diagnostics, Config, and Help.
   - Each feature owns its markup, styles, behaviour, and Chinese/English strings.
   - A feature's private DOM identifiers, selectors, render helpers, and event logic remain inside that feature wherever the single-document runtime permits.
   - Migration may be incremental internally, but the repository must not retain parallel technical-layer and feature-layer ownership after a feature is moved.

3. **Shell ownership**
   - Shell owns the document frame, tab navigation, theme and language state, notifications, shared request primitive, common controls, and modal foundations that are genuinely used across features.
   - Keep the Shell interface small; feature-specific selectors, rendering, actions, and translations must not accumulate there.
   - Require deletion-test leverage before extracting additional shared modules.

4. **Assembly contract**
   - Generate one deterministic HTML document containing all CSS and JavaScript inline.
   - Hide required JavaScript dependency order and CSS cascade order inside assembly rather than making callers concatenate fragments.
   - Preserve strict CSP compatibility and prohibit remote URLs, CDN assets, runtime module loading, and external fonts or scripts.
   - Preserve the existing HTTP content type and Management route behaviour.

5. **Behaviour compatibility**
   - Preserve the five tabs, all current actions and confirmation flows, Model Group presentation, configuration semantics, diagnostics, history, help content, translations, theme behaviour, and responsive presentation.
   - Preserve CPA embedding and direct-browser operation using the existing native Web platform baseline.
   - Treat this as an architecture-only change; no user-facing redesign or product copy reinterpretation is included.

6. **Runtime and tooling**
   - Keep CPA runtime delivery independent of a frontend build chain and package installation.
   - Test tooling may execute or inspect the assembled resource during repository verification, but generated bundles and package-manager infrastructure must not become production inputs.
   - Prefer standard, already-available tooling for syntax and closure checks; add a dependency only if a high-value behaviour cannot be tested reliably without it.

7. **Migration and contraction**
   - Move one complete feature slice at a time so each intermediate change can prove behavioural equivalence through the final resource.
   - Remove superseded technical-layer fragments and their direct substring tests once the corresponding feature contract is covered.
   - Keep transport tests thin and retain only source-level checks that express stable security or presentation contracts.

8. **Documentation impact**
   - No new domain glossary entry is required because feature-asset ownership is an implementation concern rather than an Antigravity domain concept.
   - No ADR is required because the work consolidates existing commitments without introducing a new hard-to-reverse product or system trade-off.

## Testing Decisions

1. **Primary test seam**
   - Test the fully assembled `StatusHTML`, not private feature fragments or concatenation helpers.
   - Assert user-observable document contracts and behaviours rather than implementation source spelling or file order.

2. **Resource security and delivery**
   - Verify `/status` serves the complete resource with the existing status and content type.
   - Verify the final resource has no remote URLs and remains self-contained under the existing CSP assumptions.
   - Retain CSS token completeness and defined-variable checks because they express stable theme contracts.

3. **Executable resource integrity**
   - Extract and parse the final JavaScript so syntax errors fail the suite.
   - Verify every inline event handler references an available callable function.
   - Verify referenced DOM identifiers and selectors resolve to elements supplied by the assembled document where statically determinable.
   - Verify every referenced i18n key is present for both supported languages.

4. **High-value behaviour coverage**
   - Add a small number of executable tests for Config load/edit/save and validation outcomes.
   - Verify Apply's two-stage Diff confirmation reaches the existing Management action only after confirmation.
   - Verify feature navigation and language/theme changes preserve the existing page state where those behaviours share the same Shell primitives.
   - Keep the behaviour set intentionally small and prioritize flows whose failure could change Host or scheduling configuration.

5. **Feature contract coverage**
   - Verify all five functional areas are present and reachable.
   - Verify Overview retains Model Group selection, Probe, Apply, Reset, refresh, and target/predicted presentation controls.
   - Verify History, Diagnostics, Config, and Help retain their existing essential controls and information regions.
   - Verify responsive and theme token contracts without introducing a browser-version matrix.

6. **Replace, do not layer**
   - Replace source-substring tests when the same requirement is covered through final-resource structure or behaviour.
   - Retain narrow substring or regex checks only for stable security invariants that are clearer at the delivered resource level.
   - Do not preserve tests for private fragment names, concatenation sequence, helper names, or exact JavaScript source text.

7. **Regression and quality gates**
   - Keep HTTP handler tests focused on transport and the assembly suite focused on page contracts.
   - Require repository formatting and static analysis.
   - Require `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` to pass.
   - Perform focused browser verification in the existing CPA-compatible environment for the final integrated change.

## Out of Scope

- A visual redesign, navigation redesign, new tab, new user action, or revised product copy meaning.
- Changes to Management routes, authentication, request payloads, response schemas, or Runtime behaviour.
- Changes to Control Model Group, Predicted Model Group, Fresh Evidence, Planner, Host Transition, cooldown, or reset semantics.
- A frontend framework, component framework, general feature registry, plugin system, dependency-injection framework, or micro-frontend architecture.
- A bundler, transpiler, npm application, generated production bundle, CSS-in-JS system, or runtime asset loader.
- Remote scripts, styles, fonts, images, analytics, telemetry, or CDN dependencies.
- A new browser-version support matrix or a third user-selectable theme.
- Broad end-to-end browser automation for every visual detail or control.
- Rewriting every existing management-page test when it already expresses a stable final-resource contract.
- Reorganizing Runtime, Planner, State Store, Apply, or Host modules as part of this work.
- Implementing the other second-round architecture proposals.
- Committing, releasing, or changing version numbers as part of this specification.

## Further Notes

- This specification implements the accepted fourth proposal and deliberately keeps one primary seam: the delivered `StatusHTML` resource.
- Feature assets are implementation modules, not new domain concepts or independently loaded runtime packages.
- The goal is locality and reliable assembly, not a larger frontend architecture.
- Core behavioural tests should concentrate on operations that can change Host state or scheduling configuration; structural closure checks cover the wider static surface economically.
- The work should be split into a small number of blocker-aware tracer-bullet tickets before implementation.
