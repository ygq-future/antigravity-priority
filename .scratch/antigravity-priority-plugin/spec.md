# Spec: Antigravity Smart Priority Plugin

Status: `ready-for-agent`

## Problem Statement

Users managing multiple Google Antigravity accounts within CLIProxyAPI (CPA) face significant quota inefficiencies when using generic multi-provider schedulers:
1. **End-of-Cycle Quota Waste**: Antigravity enforces both a 5-hour burst window and a 7-day weekly total quota. A generic scheduler that only boosts priority during the final 24 hours cannot consume large remaining balances due to the physical 5-hour throughput limit, leaving valuable weekly quota to expire unused.
2. **Suboptimal Daily Rotation**: Accounts with tight expiration deadlines are not prioritized smoothly over accounts with ample remaining time throughout the week.
3. **Disruptive Rate Limit Handling**: When an account exhausts its 5-hour short window, existing plugins hard-disable the credential, requiring manual recovery or failing to leverage automatic 5-hour replenishment.
4. **Code and Configuration Bloat**: Generic plugins contain unnecessary logic for other providers (Codex, xAI) and force users to manually configure complex usage rates.

## Solution

Build `antigravity-priority`, a high-performance, single-provider priority scheduler and quota manager designed exclusively for Google Antigravity in CPA:
1. **Dynamic Boost Horizon**: Calculates the exact physical time required to burn remaining weekly quota ($T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5\text{h}$) and dynamically triggers 999 priority boost as early as necessary (e.g., 27+ hours ahead for an 80% balance).
2. **Weekly Urgency Index**: Orders healthy credentials by unit-time quota consumption pressure ($\text{Urgency}_{\text{weekly}} = R_{\text{7d}} / \max(T_{\text{7d}}, 0.5)$) with 5-hour reset tie-breaking, ensuring smooth, optimal rotation across the entire 7-day cycle.
3. **Adaptive Self-Learning Burn Rate**: Automatically measures actual consumption ratios ($\Delta R_{\text{7d}} / \Delta R_{\text{5h}}$) from successive probe observations, updating a smoothed $C_{\text{cycle}}$ value without user configuration burden.
4. **Soft Depletion for 5-Hour Windows**: Sets exhausted short windows to priority `-1` while keeping `disabled = false`, allowing silent automatic rotation back into service when the 5-hour window resets.
5. **Native CPA Dark/Light Theme Management UI**: Provides an embedded dashboard with real-time double-window progress bars, urgency scores, model group switching (`gemini` vs `claude_gpt`), and dry-run/apply controls that seamlessly match CPA's theme.

## User Stories

1. As a CPA user with multiple Antigravity accounts, I want accounts with large remaining weekly quota to automatically boost to top priority early enough before weekly reset, so that none of my weekly quota expires unused.
2. As a CPA user with high-load workflows, I want the scheduler to continuously calculate Weekly Urgency, so that accounts closer to their reset deadlines are prioritized over accounts with longer horizons.
3. As a developer using Antigravity Pro, I want credentials that exhaust their 5-hour short window to soft-deplete without being hard-disabled in CPA, so that they automatically resume receiving traffic once their 5-hour window resets.
4. As an Antigravity user, I want credentials whose 7-day weekly quota is completely depleted to be hard-disabled in CPA, so that no requests fail due to out-of-quota errors.
5. As a CPA administrator, I want the plugin to automatically learn each account's cycle burn rate from actual usage deltas, so that I do not need to guess or manually tune complex mathematical coefficients.
6. As a user working with Claude/GPT models on Antigravity, I want to configure `antigravity_model_group: "claude_gpt"` in configuration or switch it on the management UI, so that priority scheduling optimizes for Claude/GPT quota instead of Gemini quota when needed.
7. As a plugin operator, I want an embedded Web Management UI that displays 5-hour and 7-day progress bars, reset countdowns, urgency scores, and boost badges for every credential, so that I have clear observability into my quota fleet.
8. As a user operating CPA in dark mode, I want the management UI to automatically adapt to CPA's dark theme, so that I have a consistent visual experience without harsh white backgrounds.
9. As an automated deployment pipeline, I want full cross-platform dynamic libraries (Linux, macOS, Windows, FreeBSD on x86/ARM) built with checksums, so that the plugin can be installed on any CPA host environment.
10. As an open-source contributor, I want strict linting, zero warnings, and 100% core test coverage, so that any future changes can be verified safely against regressions.
11. As a CPA host, I want CGO ABI calls (`cliproxy_plugin_init`, `antigravityPriorityPluginCall`, `antigravityPriorityPluginFreeBuffer`, `antigravityPriorityPluginShutdown`) to execute with single-flight concurrency safety, so that host callbacks never experience race conditions or memory leaks.
12. As a CPA administrator, I want to perform a manual "Dry-Run" from the management UI or API, so that I can inspect the planned priority changes before committing them to host state.
13. As a CPA administrator, I want to trigger an immediate "Apply" from the management UI, so that updated priorities are written back to CPA credentials on demand.
14. As a security-conscious user, I want all sensitive credentials, authorization headers, and access tokens to be strictly redacted from diagnostic endpoints and logs, so that sensitive materials are never exposed.

## Implementation Decisions

1. **Single-Provider Repository Structure**:
   - The root repository is initialized as a dedicated Go module `antigravity-priority`.
   - Legacy multi-provider implementations (`codex`, `xai`) are completely excluded.
   - Reference source (`example-src/`) is preserved locally and ignored via `.gitignore`.

2. **Core Scheduling Architecture & Double-Window Comparator**:
   - Extraction of 4 core metrics per probe: $R_{\text{5h}}$ (0.0~1.0), $T_{\text{5h}}$ (hours), $R_{\text{7d}}$ (0.0~1.0), $T_{\text{7d}}$ (hours).
   - $\text{Urgency}_{\text{weekly}} = R_{\text{7d}} / \max(T_{\text{7d}}, 0.5)$.
   - $T_{\text{required}} = (R_{\text{7d}} / C_{\text{cycle}}) \times 5.0$.
   - $\text{IsBoosted} = (T_{\text{7d}} \le T_{\text{required}}) \land (R_{\text{7d}} > 0)$.
   - Comparator hierarchy:
     1. Boosted items: `priority = 999, 998...` ordered descending by Weekly Urgency.
     2. Regular items: `priority = 100, 99...` ordered descending by Weekly Urgency, tie-broken by $T_{\text{5h}}$ ascending (earliest reset first), then AuthIndex.
     3. Depleted items: Weekly $R_{\text{7d}} \le 0 \implies$ `priority = -1, disabled = true`; 5h $R_{\text{5h}} \le 0 \implies$ `priority = -1, disabled = false`.

3. **Adaptive Burn Rate Estimator**:
   - Initial cold-start default: $C_{\text{cycle}} = 0.15$.
   - On observable consumption in the same 5h window ($\Delta R_{\text{5h}} \ge 0.05$ and $\Delta R_{\text{7d}} > 0$):
     $$C_{\text{obs}} = \frac{\Delta R_{\text{7d}}}{\Delta R_{\text{5h}}}$$
     $$C_{\text{cycle}}^{\text{new}} = 0.3 \times \text{clamp}(C_{\text{obs}}, 0.08, 0.30) + 0.7 \times C_{\text{cycle}}^{\text{old}}$$
   - Persisted in local state store (`refresh-cache.json`).

4. **Configuration Schema**:
   ```yaml
   enabled: true
   auto_apply: false
   interval: 15m
   antigravity_model_group: "gemini" # gemini | claude_gpt
   max_concurrency: 6
   min_change: 1
   priority_rules:
     enabled: true
     boost_start_priority: 999
     normal_start_priority: 100
   ```

5. **CGO ABI Export Layer**:
   - `cliproxy_plugin_init`
   - `antigravityPriorityPluginCall`
   - `antigravityPriorityPluginFreeBuffer`
   - `antigravityPriorityPluginShutdown`
   - Dynamic library base: `antigravity-priority`.

6. **Web Management UI & CPA Theme System**:
   - Self-contained, zero-external-CDN HTML/CSS/JS template embedded in binary.
   - Dual-theme support adhering to CPA host: CSS variables defined for `:root`, `@media (prefers-color-scheme: dark)`, and `[data-theme="dark"]`.
   - Renders 5h/7d double window meters, countdowns, adaptive $C_{\text{cycle}}$, urgency score, boost badges, model group switcher, and dry-run/apply triggers.

7. **Quality Gates & CI/CD**:
   - `.golangci.yml` configuring strict linters (`govet`, `staticcheck`, `errcheck`, `gofumpt`, `revive`, `misspell`, `gocritic`).
   - Formatting standard: `gofumpt` + `goimports`.
   - GitHub Actions: `ci.yml` (lint + race tests + cross-compilation check) and `release.yml` (7-platform matrix build + sha256 checksums).
   - Standardized `registry.json` conforming to `CLIProxyAPI-Plugins-Store` requirements.

## Testing Decisions

1. **High-Seam Behavioral Testing**:
   - Unit and integration tests verify external behavior, invariant fulfillment, and mathematical precision without asserting on volatile internal state.
   - Test suites use mock clocks (`Clock`) and mock host HTTP callers (`HTTPDoer`) to eliminate non-determinism.

2. **Core Modules Under Test**:
   - `internal/priority`: 100% branch and table-driven coverage testing Weekly Urgency calculations, Dynamic Boost Horizon boundaries, 3-tier comparator sorting, soft/hard depletion, and uniqueness.
   - `internal/state`: Tests state loading, atomic save, and adaptive $C_{\text{cycle}}$ estimator transitions under various delta scenarios.
   - `internal/provider/antigravity`: Tests parsing of Google `retrieveUserQuotaSummary` JSON structures, window classification (`5h` vs `weekly`), multi-window selection, and error handling.
   - `internal/runtime`: Tests single-flight execution, dry-run vs apply write-backs, auto-apply interval cooldowns, and management HTTP request dispatch.

3. **Quality & Coverage Invariants**:
   - Core scheduling and parsing packages must maintain $\ge 90\%$ test coverage.
   - All tests must pass with `-race` enabled on all supported platforms.

## Out of Scope

- Support for non-Antigravity providers (OpenAI Codex, xAI, Anthropic direct, etc.).
- Direct token-based billing or external payment gateway integration.
- Custom external database persistence (state is stored locally in `refresh-cache.json`).

## Further Notes

- The plugin is fully compliant with `CLIProxyAPI-Plugins-Store` submission standards.
- Domain terms in this document match the canonical vocabulary defined in `CONTEXT.md`.
- Architecture decisions are recorded in `docs/adr/0001-antigravity-exclusive-architecture.md`, `docs/adr/0002-adaptive-cycle-burn-rate-estimation.md`, and `docs/adr/0003-double-window-comparator-and-soft-depletion.md`.
