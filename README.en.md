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

- **Tailored Antigravity Three-Pool Scheduling**: Dedicated exclusively to Google Antigravity (supporting `gemini`, `argon`, and `claude_gpt` independent quota pools) with deep awareness of 5-hour burst and 7-day weekly quota windows.
- **Native CPA `scheduler.pick` Pure Memory Dispatch**: Hooks into CPA via `scheduler: true` and `scheduler_across_priorities: true` to resolve inbound model requests dynamically and route traffic to optimal credentials in memory with **zero disk writes and zero static priority conflicts**.
- **Model-Centric Routing Board & 5-Bar Quota Cards**: Replaces legacy two-group toggles with an intuitive **Model Routing Board**; each credential card displays all five quota progress bars (`Gemini 5h/7d`, `Claude 5h/7d`, `Argon 7d`) with active route highlighting and call-order tier badges.
- **Session-Sticky Affinity & Equal-Priority Clustering**: Accounts with close urgency metrics share the same priority integer tier, enabling CPA native round-robin distribution with `canonical_session_id` session affinity within the top cluster to eliminate 429 single-point saturation.
- **Custom Tier Overrides & Round-Robin Load Balancing**: Allows freely assigning explicit target tiers (Tier #1, Tier #2...) on overview cards; custom tiers take absolute precedence, and candidates sharing the same custom tier receive identical priority scores for round-robin load distribution.
- **Real-Time End-to-End Request History**: Intercepts and logs every inbound request with its model, target quota pool, token breakdown (input, output, reasoning, cache), and status code in a high-density table with one-click clear.
- **Pool-Scoped 429 Cooldown Circuit Breaker**: Isolates 429 rate limit demotions to the affected quota pool without disabling sibling pools on the same account (e.g. 429 on Claude does not affect Gemini or Argon traffic), absorbing transient fleet-wide bursts with storm protection; cooldown expiry enters the probe queue and priority restores after a successful probe or same-pool request.
- **Health Failover & Credential Quarantine**: Quarantines 401 and 403 at account scope as credential failures with immediate soft demotion without active retry probes; demotes 5xx at pool scope; 429/5xx enter an exponential-backoff probe queue that reuses warmup generation requests and restores the original tier after one successful probe.
- **Closed-Loop Real-Time Estimation & Probe Auto-Calibration**: Predicts token consumption per turn via `usage.handle` to update routing tiers in real-time, smoothing EMA token sensitivity $K_{\text{5h}}$ and burn rate $C_{\text{cycle}}$ on periodic Google probes.
- **Dynamic Boost Horizon**: Intelligently computes the required burn horizon to proactively elevate abundant accounts to top priority (`999, 998...`), eliminating end-of-cycle quota waste.
- **Weekly Urgency Balancing**: Quantifies unit-time consumption pressure to smoothly rotate accounts throughout the 7-day cycle.
- **Adaptive Online Learning ($C_{\text{cycle}}$)**: Automatically measures and smooths real consumption velocity without requiring manual coefficient tuning.
- **Self-Healing Dual-Window Soft Fallback**: Both 5-hour burst exhaustion and 7-day weekly exhaustion trigger pure in-memory soft demotion (priority `-1`, keeping `disabled = false`). Credential `disabled` status remains strictly owned by the user/CPA host, and accounts automatically recover once quota resets.
- **5-Hour Short-Window Quota Warm-Up**: Automatically dispatches lightweight natural engineering prompts when Gemini or Claude & GPT 5h windows are 100% full with an inactive countdown, starting the 5-hour reset cycle proactively to prevent idle quota waste. Once a window's reset time elapses, warm-up is driven by persisted quota evidence rather than the scheduling cycle, so a paused schedule or a plugin restart never skips it.
- **Web UI Dynamic Config Center**: CPA host YAML only requires `enabled: true`. All scheduling intervals, concurrency, and scoring rules are visually managed with instant zero-restart hot-reloads.
- **Real Subscription Tier Detection (Ultra / Ultra-lite / Pro / Free)**: Discovers the effective plan from `loadCodeAssist` `paidTier` / `currentTier` with a daily state cache and `paidTier` precedence, rendering accurate `ULTRA` / `ULTRA-LITE` / `PRO` / `FREE` badges; failures fall back to window inference without affecting scheduling.
- **Independent Pro & Free Tier Physical Separation**: Accounts without a 5-hour burst window (Free tier) are automatically isolated into a dedicated fallback tier (default priority 10 downwards) and barred from Boost elevation, guaranteeing Pro accounts strictly take precedence while allowing customizable baseline priority in the Web UI Config Center.
- **Embedded Dual-Theme Dashboard**: Zero external CDN and strictly CSP-compliant, providing real-time quota meters, model routing board, complete email identity, and redacted audit summaries, plus a persistent overview account-masking switch that keeps both ends visible and hides only the middle characters.
---

## Workflow

```text
Load plugin
  -> Read plugins.configs.antigravity-priority configuration
  -> Register scheduler.pick and management capabilities (scheduler_across_priorities: true)
  -> Fetch CPA credential list and model registry through host.auth.list and host.auth.models
  -> Periodic probing & precomputation
       - Concurrently extract quota windows across all three pools (gemini, argon, claude_gpt)
       - Evaluate Dynamic Boost Horizon eligibility using learned C_cycle burn rate
       - Precompute high-speed in-memory routing tables (O(1) lookup)
  -> Handle inbound requests (scheduler.pick)
       - Resolve requested model (e.g. gemini-3.8-flash-high, gemini-4-argon-high, claude-sonnet-5-5-high)
       - Map to target quota pool (gemini / argon / claude_gpt)
       - Filter depleted and 429-cooling credentials, select highest-priority tier (999 Boost > 100 Normal > 10 Free)
       - Dispatch within the top tier using canonical_session_id affinity and round-robin cursors
       - Return target auth_id in pure memory with zero disk I/O
  -> Closed-loop real-time tracking (usage.handle)
       - Deduct estimated quota per turn from reported token usage, updating routing tiers
       - Auto-calibrate against authoritative Google probes to eliminate drift
  -> Display model-centric routing board, call order tiers, 5-bar cards, and redacted audit summaries
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
      # state_cache_path: "data/antigravity-priority/state.json" # Optional custom cache path
```

| Host Parameter | Default | Required | Description |
| :--- | :--- | :--- | :--- |
| **`enabled`** | `true` | Yes | Global plugin switch. When `false`, all scheduling and background workers are halted. |
| **`state_cache_path`** | `data/antigravity-priority/state.json` | No | Main state file path, stored by default under `data/antigravity-priority/`. The same directory derives `runtime.json`, `runs.json`, and `requests.json` for runtime config, run audit, and request history so high-frequency writes never touch unrelated state. |

> **⚠️ Path Migration Note**: The legacy default `data/antigravity-priority-cache.json` is migrated automatically into `data/antigravity-priority/` on startup, and old files are removed only after all four new documents are written. Changing `state_cache_path` to a different path does not hot-migrate; stop CPA/plugin and move or copy the whole cache directory before updating the configuration.

### 2. UI Dynamic Config Center (Zero-Restart Hot Reload)

In the **`⚙️ Config Center`** tab of the management dashboard, the following options can be visually adjusted and applied instantly:

| Setting | Default | Range/Options | Description |
| :--- | :--- | :--- | :--- |
| **Background Quota Calibration (`auto_apply`)** | `false` | On / Off | When true, scheduled background runs periodically probe Google API to calibrate real-time usage estimators. |
| **Calibration Interval (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, custom | Background quota probe and calibration interval. Changes take effect immediately without restarting. |
| **Active Schedule Window (`schedule_window`)** | `All day` | `HH:MM` to `HH:MM` | Daily active time window (e.g. `09:00-23:00`, supports cross-midnight like `22:00-06:00`). Sleeps outside window. |
| **Max Probe Concurrency (`max_concurrency`)** | `6` | `1 ~ 32` | Maximum concurrent goroutines for quota probe requests to Google API. |
| **Urgency Bucket Tolerance (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | Accounts within this tolerance share the same priority for round-robin balancing. |
| **Adaptive Sample Capacity (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO history retained for both consumption-trend inspection and burn-rate learning. |
| **429 Cooldown Duration (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` min | Cooldown duration isolating rate-limited quota pool into fallback tier on 429 errors. |
| **Boost Start Priority (`boost_start_priority`)** | `999` | `1 ~ 999` | Base priority for tier-1 boosted credentials. |
| **Normal Healthy Start Priority (`normal_start_priority`)** | `100` | `1 ~ 999`, not above Boost start | Base priority for tier-2 regular healthy credentials. |
| **Free Tier Fallback Start Priority (`free_start_priority`)** | `10` | `1 ~ 999`, not above Normal start | Base priority for Free accounts without 5h window, serving as fallback after Pro. |
| **Gemini 5h Auto Warm-Up (`warmup.gemini_enabled`)** | `false` | On / Off | Automatically sends a lightweight prompt when Gemini 5h quota is 100% full and inactive to trigger the reset countdown. |
| **Gemini Warm-Up Model (`warmup.gemini_model`)** | `gemini-3-flash` | Selectable | Model used to trigger the Gemini 5h reset countdown. |
| **Claude 5h Auto Warm-Up (`warmup.claude_enabled`)** | `false` | On / Off | Automatically sends a lightweight prompt when Claude 5h quota is 100% full and inactive to trigger the reset countdown. |
| **Claude Warm-Up Model (`warmup.claude_model`)** | `claude-sonnet-5-5-high` | Selectable | Model used to trigger the Claude 5h reset countdown. |

---

## Management Page and API

The plugin registers **resources** (static management dashboard) and **routes** (dynamic management APIs) with the CPA host via `management.register`.

### Resource Page (Static Web UI)

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **Access**: Click **"Antigravity Priority"** in the CPA management sidebar, or open `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status` in your browser.
  - **Key Features**:
    - **Model-Centric Routing Board**: Dropdown to select concrete models (Gemini pool, Argon pool, Claude & GPT pool), instantly rendering call order tiers (Tier #1 · 999 Boost, Tier #2 · 100 Clustered Round-Robin, etc.) and active eligibility.
    - **Panoramic 5-Bar Quota Cards**: Every card displays all 5 quota bars (Gemini 5h/7d, Claude 5h/7d, Argon 7d), highlighting the active route bars for the selected model with adaptive countdown timers and learned $C_{\text{cycle}}$.
    - **Scheduling & Synchronization Control**: Provides `🔄 Sync Host`, `📡 Fetch Quota (10s cooldown)`, and `🔄 Reset Static Priorities`.
    - **Real-Time Request History**: Retains recent request logs (configurable retention, default 100, max 1000) with model, target quota pool, token breakdown, and response status with one-click clear.
    - **Samples Modal**: Seamlessly toggle between Gemini, Argon, and Claude & GPT sample histories.
    - **System Diagnostics**: Health banner, three KPIs (scheduler, isolation, last run), a three-pool quota matrix, per-credential decision explanations (priority, urgency, T7d/T_req, C_cycle, reason), 429 storm and cooldown isolation, run timeline, and the path, size, and presence of the four cache files, with one-click JSON Copy.
    - **⚙️ Config Center**: Online management of all scheduling and algorithm parameters with instant hot reload.

### Management API (Dynamic, Key Required)

- `POST /v0/management/plugins/antigravity-priority/run?mode=probe`
  - Triggers a fresh network quota probe against Google API and updates local cache and snapshot, calibrating in-memory routing tables only, **with no CPA Host write-back**.
- `GET /v0/management/plugins/antigravity-priority/runtime-config`
  - Retrieves current full runtime configuration.
- `POST /v0/management/plugins/antigravity-priority/runtime-config`
  - Submits updated runtime configuration and hot-applies immediately.
- `GET /v0/management/plugins/antigravity-priority/schedule/config`
  - Retrieves active schedule time window and pause state.
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - Updates active schedule time window or toggles pause/resume.
- `GET /v0/management/plugins/antigravity-priority/diagnostics`
  - Exports scheduler state, plan snapshot, cooldowns and 429 storm, run history, active policy, and the status of the four cache files; account identity uses full email while tokens, keys, and persisted audit fields remain redacted.
- `POST /v0/management/plugins/antigravity-priority/sync`
  - Actively re-syncs latest credential files from CPA host and regenerates three-pool snapshots immediately.
- `GET /v0/management/plugins/antigravity-priority/samples?auth_index=xxx`
  - Retrieves multi-sample sliding window time-series quota observations for a specific credential.
- `GET /v0/management/plugins/antigravity-priority/samples?probe_round_id=xxx&model_group=gemini|argon|claude_gpt`
  - Retrieves quota samples actually appended for the specified probe round and model group; credentials with unchanged quota are absent from the result.
- `GET /v0/management/plugins/antigravity-priority/snapshot/latest`
  - Returns the latest three-pool decision planning snapshot (`DualGroupSnapshot`) with full CPA email and technical `auth_index` for API correlation; the management page uses email as the account name while sensitive tokens and audit fields remain redacted.
- `GET /v0/management/plugins/antigravity-priority/request-history`
  - Retrieves up to 100 recent real-time inbound request execution logs.
- `POST /v0/management/plugins/antigravity-priority/request-history/clear`
  - Clears all recorded inbound request history entries.
- `GET /v0/management/plugins/antigravity-priority/custom-tier`
  - Retrieves the dictionary of configured credential custom tier overrides.
- `POST /v0/management/plugins/antigravity-priority/custom-tier`
  - Configures or clears custom priority tier overrides for a specific credential (e.g. Tier #1, Tier #2...; pass 0 to restore automatic tiering).

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
