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

- **Tailored Antigravity Three-Pool Scheduling**: Dedicated exclusively to Google Antigravity (supporting `gemini`, `argon`, and `claude_gpt` quota pools) with deep awareness of 5-hour burst and 7-day weekly quota windows; on Ultra / Ultra-lite, Argon reuses measured Gemini dual-window evidence for scheduling and dashboard display instead of fabricating an independent Argon bucket.
- **Native CPA `scheduler.pick` Pure Memory Dispatch**: Hooks into CPA via `scheduler: true` and `scheduler_across_priorities: true` to resolve inbound model requests dynamically and route traffic to optimal credentials in memory with **zero disk writes and zero static priority conflicts**.
- **Model-Centric Routing Board & 5-Bar Quota Cards**: Replaces legacy two-group toggles with an intuitive **Model Routing Board**; each credential card displays all five quota progress bars (`Gemini 5h/7d`, `Claude 5h/7d`, `Argon 7d`) with active route highlighting and call-order tier badges, and the last selected model is remembered across reloads.
- **Global Model Exclusion (Excluded Models)**: Any model can be excluded with one click from the routing-board dropdown. Excluded models skip every Antigravity credential and go to the highest-priority candidate among other CPA providers; the exclusion set persists, applies without restart, and the trigger and cards show exclusion badges.
- **Session-Sticky Affinity & Equal-Priority Clustering**: Accounts with close urgency metrics share the same priority integer tier, enabling CPA native round-robin distribution with `canonical_session_id` session affinity within the top cluster to eliminate 429 single-point saturation.
- **Custom Tier Overrides & Round-Robin Load Balancing**: Allows freely assigning explicit target tiers (Tier #1, Tier #2...) on overview cards; custom tiers take absolute precedence, and candidates sharing the same custom tier receive identical priority scores for round-robin load distribution.
- **Real-Time End-to-End Request History**: Intercepts and logs every inbound request with its model, target quota pool, token breakdown (input, output, reasoning, cache), and status code in a high-density table with one-click clear.
- **Pool-Scoped 429 Cooldown Circuit Breaker**: Isolates 429 rate limit demotions to the affected quota pool without disabling sibling pools on the same account (e.g. 429 on Claude does not affect Gemini or Argon traffic), absorbing transient fleet-wide bursts with storm protection; cooldown expiry enters the probe queue and priority restores after a successful probe or same-pool request.
- **Health Failover & Credential Quarantine**: Quarantines 401 and 403 at account scope as credential failures with immediate soft demotion without active retry probes; demotes 5xx at pool scope; 429/5xx enter an exponential-backoff probe queue that reuses warmup generation requests and restores the original tier after one successful probe.
- **Request-Frequency Adaptive Calibration**: Background calibration no longer probes on a fixed cadence. It follows recent request windows and bursts: skipped while idle, stretched (2–6 hour cap) with jitter when sparse, completed within the interval ceiling when active, and caught up within 15–45 seconds after traffic resumes from a long idle gap.
- **Closed-Loop Real-Time Estimation & Probe Auto-Calibration**: Predicts token consumption per turn via `usage.handle` to update routing tiers in real-time, smoothing EMA token sensitivity $K_{\text{5h}}$ and burn rate $C_{\text{cycle}}$ on adaptively paced Google probes.
- **Dynamic Boost Horizon**: Intelligently computes the required burn horizon to proactively elevate abundant accounts to top priority (`999, 998...`), eliminating end-of-cycle quota waste.
- **Weekly Urgency Balancing**: Quantifies unit-time consumption pressure to smoothly rotate accounts throughout the 7-day cycle.
- **Adaptive Online Learning ($C_{\text{cycle}}$)**: Automatically measures and smooths real consumption velocity without requiring manual coefficient tuning.
- **Self-Healing Dual-Window Soft Fallback**: Both 5-hour burst exhaustion and 7-day weekly exhaustion trigger pure in-memory soft demotion (priority `-1`, keeping `disabled = false`). Credential `disabled` status remains strictly owned by the user/CPA host, and accounts automatically recover once quota resets.
- **5-Hour Short-Window Quota Warm-Up**: Automatically dispatches a natural engineering prompt to the Antigravity consumer surface when Gemini or Claude & GPT 5h windows are 100% full, the 7d pool still has quota, and the countdown is inactive, starting the 5-hour reset cycle proactively to prevent idle quota waste. Gemini warm-ups append a minimum-output constraint so lightweight models still produce accountably sized completions. Once a window's reset time elapses, warm-up is driven by persisted quota evidence rather than the scheduling cycle, so a paused schedule or a plugin restart never skips it. Every credential shares one global warm-up lane: credentials are spaced 60–120 minutes apart at random, while pools of the same credential may warm 30–60 minutes apart, so 5h refresh points never coincide; each overview card's warm-up icon overrides the global state locally (matching the global state restores follow mode and clears that credential's 4h30m debounce when turned on), and Free plans never warm up.
- **Web UI Dynamic Config Center**: CPA host YAML only requires `enabled: true`. Scheduling limits, concurrency, cooldowns, and warm-up settings are visually managed with instant zero-restart hot-reloads.
- **Real Subscription Tier Detection (Ultra / Ultra-lite / Pro / Free)**: Discovers the effective plan from `loadCodeAssist` `paidTier` / `currentTier` with a daily state cache and `paidTier` precedence, rendering accurate `ULTRA` / `ULTRA-LITE` / `PRO` / `FREE` badges; failures fall back to window inference without affecting scheduling.
- **Independent Pro & Free Tier Physical Separation**: Accounts without a 5-hour burst window (Free tier) are automatically isolated into a dedicated fallback tier (fixed baseline priority 10) and barred from Boost elevation, guaranteeing Pro accounts strictly take precedence.
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
| **Adaptive Quota Calibration (`auto_apply`)** | `false` | On / Off | When true, background calibration adapts to request frequency: probes run within the interval ceiling under active traffic, stretch when sparse, and skip while idle. |
| **Active Calibration Max Interval (`interval`)** | `15m` | `5m`, `15m`, `30m`, `1h`, custom | Maximum calibration interval under frequent traffic (with jitter); sparse traffic stretches to 2–6 hours and idle windows skip it. Changes take effect immediately without restarting. |
| **Max Probe Concurrency (`max_concurrency`)** | `6` | `1 ~ 32` | Maximum concurrent goroutines for quota probe requests to Google API. |
| **Urgency Bucket Tolerance (`urgency_tolerance`)** | `0.05` | `0.00 ~ 0.50` | Accounts within this tolerance share the same priority for round-robin balancing. |
| **Adaptive Sample Capacity (`quota_sample_capacity`)** | `6` | `2 ~ 30` | FIFO history retained for both consumption-trend inspection and burn-rate learning. |
| **429 Cooldown Duration (`rate_limit_cooldown_minutes`)** | `5` | `1 ~ 1440` min | Cooldown duration isolating rate-limited quota pool into fallback tier on 429 errors. |
| **Gemini 5h Auto Warm-Up (`warmup.gemini_enabled`)** | `false` | On / Off | Automatically sends a long-form prompt (with a minimum 600-token constraint) when Gemini 5h quota is 100% full, the 7d pool still has quota, and the window is inactive; credentials are spaced 60–120 minutes apart, pools of one credential 30–60 minutes apart. |
| **Gemini Warm-Up Model (`warmup.gemini_model`)** | `gemini-3-flash` | Selectable | Model used to trigger the Gemini 5h reset countdown. |
| **Claude 5h Auto Warm-Up (`warmup.claude_enabled`)** | `false` | On / Off | Automatically sends a short prompt when Claude 5h quota is 100% full, the 7d pool still has quota, and the window is inactive; credentials are spaced 60–120 minutes apart, pools of one credential 30–60 minutes apart. |
| **Claude Warm-Up Model (`warmup.claude_model`)** | `claude-sonnet-5-5-high` | Selectable | Model used to trigger the Claude 5h reset countdown. |
| **Per-Credential Warm-Up Override (`warmup.overrides`)** | empty | Overview card icon | Clicking the icon writes an `auth_index -> true/false` local state; a click that matches the global state clears it and restores follow mode; turning it on clears that credential's 4h30m debounce. |

---

## Management Page and API

The plugin registers **resources** (static management dashboard) and **routes** (dynamic management APIs) with the CPA host via `management.register`.

### Resource Page (Static Web UI)

- `GET /v0/resource/plugins/antigravity-priority/status`
  - **Access**: Click **"Antigravity Priority"** in the CPA management sidebar, or open `http://<CPA_HOST>:<PORT>/v0/resource/plugins/antigravity-priority/status` in your browser.
  - **Key Features**:
    - **Model-Centric Routing Board**: Dropdown to select concrete models (Gemini pool, Argon pool, Claude & GPT pool), instantly rendering call order tiers (Tier #1 · 999 Boost, Tier #2 · 100 Clustered Round-Robin, etc.) and active eligibility; each row can exclude/restore that model, and the selection is remembered.
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
  - Retrieves automatic schedule pause state.
- `POST /v0/management/plugins/antigravity-priority/schedule/config`
  - Toggles automatic schedule pause/resume state.
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
- `GET /v0/management/plugins/antigravity-priority/excluded-models`
  - Retrieves the current globally excluded model list.
- `POST /v0/management/plugins/antigravity-priority/excluded-models`
  - Adds or removes a globally excluded model (body `{"model": "model-id", "excluded": true|false}`); excluded models skip every Antigravity credential and are served by other CPA providers.

---

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
