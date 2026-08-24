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

- **Tailored Antigravity Dual-Window Scheduling**: Dedicated exclusively to Google Antigravity (supporting both `gemini` and `claude_gpt` model groups) with deep awareness of 5-hour burst and 7-day weekly quota windows.
- **Equal Priority Clustering & Load Balancing**: Accounts with close urgency metrics automatically share the same priority integer, enabling CPA native round-robin distribution and eliminating single-account 429 rate limit saturation.
- **429 Reactive Cooldown Circuit Breaker**: Automatically intercepts upstream 429 errors and demotes affected accounts to `-1` fallback tier, auto-recovering after a configurable cooldown.
- **Dynamic Boost Horizon**: Intelligently computes the required burn horizon to proactively elevate abundant accounts to top priority (`999, 998...`), eliminating end-of-cycle quota waste.
- **Weekly Urgency Balancing**: Quantifies unit-time consumption pressure to smoothly rotate accounts throughout the 7-day cycle.
- **Adaptive Online Learning ($C_{\text{cycle}}$)**: Automatically measures and smooths real consumption velocity without requiring manual coefficient tuning.
- **Self-Healing Soft Fallback & Hard Disabling**: Soft-depletes 5-hour burst exhaustion for automatic recovery upon reset, and hard-disables exhausted 7-day weekly accounts.
- **Web UI Dynamic Config Center**: CPA host YAML only requires `enabled: true`. All scheduling intervals, concurrency, model groups, and scoring rules are visually managed with instant zero-restart hot-reloads.
- **Embedded Dual-Theme Dashboard**: Zero external CDN and strictly CSP-compliant, providing real-time quota meters, instant prediction switching, complete email identity, and write-back diff confirmation controls while keeping tokens, keys, and persisted audits safely redacted.

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
  -> Execute according to run mode
       - apply: replace each credential document once through the unified Host Transition and verify the resulting state (min_change filter)
       - probe / sync: update in-memory state, diagnostics, and management snapshots only
  -> Display full CPA email, double-window meters, urgency scores, boost badges, and a redacted audit summary on the authenticated management page; authIndex remains an internal technical correlation key
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

### 1. Minimalist CPA Host Configuration (`config.yaml`)

Starting from v1.1.0, it is recommended to keep only the basic plugin enablement switch in CPA's `config.yaml`. All business and scheduling parameters can be visually managed via the Web dashboard's **`⚙️ Config Center`**:

```yaml
plugins:
  enabled: true
  dir: "plugins"
  configs:
    antigravity-priority:
      enabled: true
      # state_cache_path: "data/antigravity-priority-cache.json" # Optional custom cache path
```

| Host Parameter | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| **`enabled`** | `true` | Yes | Global plugin switch. When `false`, all scheduling and background workers are halted. |
| **`state_cache_path`** | `data/antigravity-priority-cache.json` | No | File path for persisting state snapshots, dynamic configuration, and adaptive burn rates. |

> **⚠️ Path Migration Note**: The plugin does not perform automatic hot migration across storage paths. If `state_cache_path` is changed mid-operation, the plugin treats missing new paths as clean cold starts. To retain existing dynamic configuration, run history, and learned burn rates, stop CPA/plugin and manually move or copy the existing cache file to the new path before updating the configuration.

### 2. UI Dynamic Config Center (Zero-Restart Hot Reload)

In the **`⚙️ Config Center`** tab of the management dashboard, the following options can be visually adjusted and applied instantly:

| Setting | Default | Range/Options | Description |
| :--- | :--- | :--- | :--- |
| **Auto Periodic Scheduling (`auto_apply`)** | `false` | On / Off | When true, scheduled background runs automatically probe and commit priority updates. |
| **Scheduling Interval (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, custom | Background execution interval. Changes take effect immediately without restarting. |
| **Primary Model Group (`antigravity_model_group`)** | `gemini` | `gemini` / `claude_gpt` | Primary model group used as the basis for host priority write-backs. |
| **Active Schedule Window (`schedule_window`)** | `All day` | `HH:MM` to `HH:MM` | Daily active time window (e.g. `09:00-23:00`, supports cross-midnight like `22:00-06:00`). Sleeps outside window. |
| **Max Probe Concurrency (`max_concurrency`)** | `6` | `1 ~ 32` | Maximum concurrent goroutines for quota probe requests to Google API. |
| **Priority Min Change Threshold (`min_change`)** | `1` | `0 ~ 100` | Minimum priority delta required to trigger a write-back to host. |
| **Urgency Bucket Tolerance (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | Accounts within this tolerance share the same priority for round-robin balancing. |
| **Adaptive Sample Capacity (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO history retained for both consumption-trend inspection and burn-rate learning. |
| **429 Cooldown Duration (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` min | Cooldown duration demoting account to `-1` fallback tier on 429 errors. |
| **Boost Start Priority (`boost_start_priority`)** | `999` | `1 ~ 999` | Base priority for tier-1 boosted credentials. |
| **Normal Healthy Start Priority (`normal_start_priority`)** | `100` | `1 ~ 999`, not above Boost start | Base priority for tier-2 regular healthy credentials. |

> **Persistence Guarantee**: All settings changed in the UI Config Center are atomically saved to the active persistence cache file (defaults to `data/antigravity-priority-cache.json`), surviving CPA container restarts and taking precedence over initial YAML values.

---

## Management Page and API

The plugin registers **resources** (static management dashboard) and **routes** (dynamic management APIs) with the CPA host via `management.register`.

### Resource Page (Static Web UI)

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **Access**: Click **"Antigravity Priority"** in the CPA management sidebar, or open `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status` in your browser.
  - **Key Features**:
    - **Overview & Meters**: Real-time 5h/7d quota meters, adaptive countdown timers, learned $C_{\text{cycle}}$, urgency scores, and 🚀 boost badges; list/dual-column view toggle and scroll containment.
    - **Instant Model Group Switching**: Toggle between Gemini and Claude/GPT views with smart `🔮 Predicted Priority` badges.
    - **Two-Stage Control**: `📡 Fetch Quota (10s cooldown)`, `⚡ Apply Now (with Diff confirmation)`, `🔄 Reset to Default`.
    - **Execution History**: Last 10 runs with `🔍 View Details` modal to inspect Apply write-back or Probe snapshots.
    - **System Diagnostics**: Scheduler engine lifecycle, active window states, 429 rate limit circuit breaking monitor, full email identity, and last apply health metrics with one-click JSON Copy.
    - **⚙️ Config Center**: Online management of all scheduling and algorithm parameters with instant hot reload.

### Management API (Dynamic, Key Required)

- `POST /v0/management/plugins/antigravity-priority/run?mode=probe`
  - Triggers a fresh network quota probe against Google API and updates local cache and snapshot, **no priority write-back**.
- `POST /v0/management/plugins/antigravity-priority/run?mode=apply`
  - Runs fresh probe/planning, **commits each credential's target through the unified Host Transition, verifies CPA host state, and returns redacted outcomes**.
- `GET /v0/management/plugins/antigravity-priority/runtime-config`
  - Retrieves current full runtime configuration.
- `POST /v0/management/plugins/antigravity-priority/runtime-config`
  - Submits updated runtime configuration and hot-applies immediately.
- `GET /v0/management/plugins/antigravity-priority/schedule/config`
  - Retrieves active schedule time window and pause state.
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - Updates active schedule time window or toggles pause/resume.
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - Exports scheduler diagnostic metrics, active 429 cooldowns, background worker state, and run history; account identity uses full email while tokens, keys, and persisted audit fields remain redacted.
- `POST /v0/management/plugins/antigravity-priority/sync`
  - Actively re-syncs latest credential files from CPA host and regenerates dual-group snapshots immediately.
- `GET /v0/management/plugins/antigravity-priority/samples?auth_index=xxx`
  - Retrieves multi-sample sliding window time-series quota observations for a specific credential.
- `GET /v0/management/plugins/antigravity-priority/samples?probe_round_id=xxx&model_group=gemini|claude_gpt`
  - Retrieves quota samples actually appended for the specified probe round and model group; credentials with unchanged quota are absent from the result.
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - Returns the latest dual-group decision planning snapshot (`DualGroupSnapshot`) with full CPA email and technical `auth_index` for API correlation; the management page uses email as the account name while sensitive tokens and audit fields remain redacted.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
