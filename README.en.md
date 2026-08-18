<div align="center">

# Antigravity Smart Priority (`antigravity-priority`)

[中文](./README.md) | [English](./README.en.md)

</div>

A high-performance, single-provider priority scheduler and quota management plugin designed exclusively for **Google Antigravity** credentials in [CLIProxyAPI (CPA)](https://github.com/router-for-me/CLIProxyAPI). The plugin ID, dynamic library basename, and CPA configuration key are all `antigravity-priority`.

---

## Navigation

- [Overview](#overview)
- [Workflow](#workflow)
- [Build and Installation](#build-and-installation)
- [Plugin Store Source](#plugin-store-source)
- [Configuration](#configuration)
- [Management Page and API](#management-page-and-api)
- [License](#license)

---

## Overview

- **Tailored for Antigravity**: Dedicated exclusively to Google Antigravity (supporting both `gemini` and `claude_gpt` quota model groups) with deep awareness of 5-hour burst and 7-day weekly quota windows.
- **Dynamic Boost Horizon**: Computes the physical time required to burn remaining weekly balance, proactively triggering `999, 998...` priority boost well in advance (e.g. 27–40 hours ahead) to completely eliminate end-of-cycle quota waste.
- **Weekly Urgency Index**: Smoothly balances credentials throughout the entire 7-day cycle by quantifying unit-time consumption pressure.
- **Adaptive Cycle Burn Rate ($C_{\text{cycle}}$)**: Automatically measures actual consumption ratios from consecutive probe observations and smooths via EMA without requiring manual coefficient tuning.
- **Self-Healing Soft Depletion & Hard Disabling**:
  - **5-Hour Window Depleted**: Demotes priority to `-1` while keeping `disabled = false`, allowing silent auto-recovery when the 5-hour window resets.
  - **7-Day Window Depleted**: Demotes priority to `-1` and applies `disabled = true` to prevent out-of-quota host failures (weekly hard-disable takes precedence over short-window soft depletion).
- **CPA Host Integration**: Reuses CPA credential discovery, proxying, and persistence flows via `host.auth.list`, `host.auth.get`, `host.auth.get_runtime`, and `host.auth.save`.
- **Fresh Evidence Gating**: Plans priority updates only from fresh and ready probe evidence collected in the current run.
- **Strict End-to-End Redaction**: All sensitive credentials, authorization headers, tokens, and cookies are automatically masked as `[REDACTED]` across UI, diagnostics, snapshots, and logs.
- **Native Dual-Theme Management UI**: Embedded zero-external-CDN dashboard adhering seamlessly to CPA's dark and light modes.

---

## Workflow

```text
Load plugin
  -> Read plugins.configs.antigravity-priority configuration
  -> Fetch CPA credential list through host.auth.list
  -> Filter Antigravity credentials
       - Concurrently probe 5h and 7d quota windows for selected model group (gemini or claude_gpt)
       - Extract short window remaining ratio R_5h, reset countdown T_5h, weekly remaining ratio R_7d, reset countdown T_7d
       - Evaluate Dynamic Boost Horizon eligibility using learned C_cycle burn rate
  -> Construct sorting plan via 3-Tier Comparator Hierarchy
       - Tier 1 (Boosted) : Dynamic boost zone -> Priorities 999, 998... (Weekly Urgency descending)
       - Tier 2 (Regular) : Healthy accounts  -> Priorities 100, 99... (Weekly Urgency descending, 5h reset tie-break)
       - Tier 3 (Depleted): Weekly hard-disable > 5h soft-fallback (-1)
  -> Decide write-back by execution mode
       - apply: write priority and enabled state via host.auth.save (min_change filter)
       - dry-run / preview: update in-memory state, redacted diagnostics, and snapshot only
  -> Display double-window meters, urgency scores, boost badges, and audit summary on management page
```

---

## Build and Installation

The plugin runs as a CGO dynamic library. CPA derives the plugin ID from the dynamic library filename, so the filename must stay `antigravity-priority.<ext>`.

### Local Compilation
```bash
# Linux / macOS
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.so .

# Windows (MSYS2 / MinGW)
go build -buildmode=c-shared -trimpath -ldflags="-s -w" -o antigravity-priority.dll .
```

### Installation in CPA
Place the compiled binary into one of the CPA plugin discovery directories:
- `plugins/<GOOS>/<GOARCH>/antigravity-priority.<ext>`
- `plugins/<GOOS>/<GOARCH>-<variant>/antigravity-priority.<ext>`
- `plugins/antigravity-priority.<ext>`

Extensions: `.so` on Linux and FreeBSD, `.dylib` on macOS, and `.dll` on Windows.

---

## Plugin Store Source

To install this plugin through the CPA plugin store, add the raw registry URL to your CPA `config.yaml`:

```yaml
plugins:
  enabled: true
  store-sources:
    - "https://raw.githubusercontent.com/ygq-future/antigravity-priority/main/registry.json"
```

> **Note**: Do not use GitHub web URLs containing `/blob/`. After saving `store-sources`, restart CPA or reload plugins through the management UI, then refresh the plugin store list to install with one click.

---

## Configuration

Enable the plugin system in CPA's `config.yaml` and configure plugin settings under `plugins.configs.antigravity-priority`:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    antigravity-priority:
      enabled: true
      auto_apply: false                 # Enable periodic background write-backs (default false)
      interval: 15m                     # Probing and scheduling interval
      antigravity_model_group: "gemini" # Primary quota model group: gemini or claude_gpt
      max_concurrency: 6                # Max concurrent HTTP probe workers
      min_change: 1                     # Priority delta threshold for write-back
      priority_rules:
        enabled: true
        boost_start_priority: 999       # Starting priority for boosted credentials
        normal_start_priority: 100      # Starting priority for regular credentials
```

### Configuration Fields

| Field | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `enabled` | boolean | `true` | Plugin switch. Requires global `plugins.enabled: true` and successful dynamic library registration. |
| `auto_apply` | boolean | `false` | When true, scheduled background runs automatically write priorities back to CPA host. When false, run manually via management UI. |
| `interval` | duration | `15m` | Background probing and scheduling period (e.g. `15m`, `30m`, `1h`). |
| `antigravity_model_group` | string | `gemini` | Primary quota model group: `gemini` (Gemini 2.5/Flash) or `claude_gpt` (Claude 3.5/GPT-4o). |
| `max_concurrency` | integer | `6` | Maximum number of concurrent probe HTTP workers. |
| `min_change` | integer | `1` | Priority change threshold below which write-back is skipped to reduce disk/database IO. |
| `priority_rules.enabled` | boolean | `true` | Enables multi-tier priority rules. |
| `priority_rules.boost_start_priority` | integer | `999` | Starting base priority for boosted credentials. |
| `priority_rules.normal_start_priority` | integer | `100` | Starting base priority for regular healthy credentials. |

> **Tip**: All settings can also be visually adjusted directly in the CPA Plugin Manager interface via `ConfigFields`.

---

## Management Page and API

The plugin registers **resources** (static management dashboard) and **routes** (dynamic management APIs) with the CPA host via `management.register`.

### Product Boundary

| Capability | Entry Path | Notes |
| :--- | :--- | :--- |
| Automated Priority & Rules | CPA Plugin Manager visual fields or `config.yaml` | Edit `auto_apply`, `interval`, `antigravity_model_group`, etc. |
| Resource Dashboard Page | `/v0/resource/plugins/antigravity-priority/status` | Static HTML dashboard: Key verify + double-window meters + Dry-Run/Apply |
| Manual Run / Apply | `/v0/management/plugins/antigravity-priority/run` | Dynamic Management API (requires Management Key) |
| Redacted Diagnostics & Snapshot | `/v0/management/plugins/antigravity-priority/diagnostics` | Inspect probe status, adaptive rates, and recent run history |
| Read-Only Config View | Host `GET /v0/management/plugins/antigravity-priority/config` | Read-only configuration provided by CPA host |

### Resource Page (Static Web UI)

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **Access**: Click **"Antigravity Priority"** in the CPA management sidebar, or open `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status` in your browser.
  - **Features**: Embedded dual-theme dashboard (strictly CSP-compliant). Authenticate using your Management Key to inspect real-time 5h burst and 7d weekly progress bars, countdown timers, adaptive burn rate $C_{\text{cycle}}$, urgency scores, and 🚀 boost badges; switch model groups dynamically, and execute **Dry-Run (preview)** or **Apply (write-back)**.

### Management API (Dynamic, Key Required)

- `POST /v0/management/plugins/antigravity-priority/run?mode=dry-run`
  - Triggers a probe and priority planning cycle (simulation), updating the latest snapshot and UI preview **without modifying host credentials**.
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply`
  - Triggers a probe and planning cycle, and **commits updated priorities and disabled states to the CPA host**.
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply&antigravity_model_group=claude_gpt`
  - Executes probe and write-back specifically for the Claude/GPT model group.
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - Exports redacted scheduler diagnostic metrics, background ticker status, and recent execution history.
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - Returns the latest redacted decision planning snapshot.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
