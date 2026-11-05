# 03: Model-Centric Routing Board UI & Unified 5-Bar Credential Cards

Status: `ready-for-agent`
Type: `task`
Blocked by: 01, 02

## What to Build

Redesign the embedded Web Management UI (`internal/management/feature_overview_*.go`, `feature_config_*.go`, `handler.go`) into a **Model-Centric Routing Board** modeled after CPA's routing orchestration view:
- Replace the 2-group dropdown with a **Model Selector Dropdown** (`#modelSelect`) listing callable Antigravity models grouped by their quota pool (`Gemini` / `Argon` / `Claude & GPT`);
- Sort and tier the credential list by the **call order / dynamic priority** for the currently selected model;
- Render all **5 quota progress bars** (`Gemini 5h`, `Gemini 7d`, `Claude 5h`, `Claude 7d`, `Argon 7d`) on every credential card, highlighting the bars that govern the selected model;
- Remove the obsolete `antigravity_model_group` control-group selector from the Config Center UI.

## Exact File Boundaries & Changes

### 1. Snapshot Payload Enrichment (`internal/apply/apply.go`, `internal/runtime/projection.go`, `internal/management/handler.go`)
- Extend `apply.DualGroupSnapshot` (serialized by `GET /snapshot/latest` and `POST /sync`):
  - `Groups`: contains `"gemini"`, `"argon"`, and `"claude_gpt"` `GroupSnapshot`s.
  - `Models []ModelOption` (`json:"models,omitempty"`): ordered list of available Antigravity models (`{id, display_name, quota_pool}`), combining built-in Antigravity models (`gemini-3.8-flash-high`, `gemini-4-argon-high`, `claude-sonnet-5-5-high`, `claude-opus-5-5-high`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`, `gemini-3-flash`, `gemini-3.5-flash-lite`, `gemini-3.1-flash-lite`, `gemini-3.1-flash-image`, `gpt-oss-120b-medium`) and any extra models discovered from `host.auth.models`.
  - On each `SnapshotItem`:
    - `QuotaWindows AllQuotaWindows` (`json:"quota_windows"`): `gemini_5h`, `gemini_7d`, `claude_5h`, `claude_7d`, `argon_7d` (each with `remaining`, `reset_at`, `stale`).
    - `SupportedModels []string` (`json:"supported_models,omitempty"`): list of model IDs supported by this credential from `host.auth.models` (empty/omitted means use pool-level default eligibility).
- Update `GET /samples` (`internal/management/handler.go`) to accept `model_group=gemini|argon|claude_gpt` (or resolve from `model=`).

### 2. Overview Control Bar & Model Selector (`internal/management/feature_overview_markup.go`, `feature_overview_actions.go`, `feature_shell_control_scripts.go`)
- Replace the 2-option `#modelGroupSelect` / `#customModelGroupSelect` with a custom styled **Model Selector Dropdown** (never raw native `<select>`):
  - Default selected model: `gemini-3.8-flash-high` (or the first model in `snapshot.models`).
  - Custom dropdown menu renders 3 group headers (`Gemini 额度池`, `Argon 额度池`, `Claude & GPT 额度池`) with each model option showing its model ID / display name and pool badge.
  - Trigger button displays `[Pool Badge] Model ID` (e.g., `[Gemini] gemini-3.8-flash-high`, `[Argon] gemini-4-argon-high`, `[Claude/GPT] claude-sonnet-5-5-high`).
- Streamline Control Bar buttons for dynamic routing:
  - `btnRefresh`: `刷新同步` (`POST /sync`)
  - `btnProbe`: `刷新配额` (`POST /run?mode=probe`)
  - `btnReset`: `重置静态优先级` (`POST /reset`, secondary/subtle action for cleaning legacy static file priorities)
  - `scheduleStatusBadge`: `🟢 动态调度运行中` / `⏸️ 自动探测已暂停`

### 3. Routing Order Sorting & 5-Bar Credential Card Rendering (`internal/management/feature_overview_render.go`, `feature_overview_styles.go`, `feature_overview_translations.go`)
- When a model `selectedModel` is chosen:
  1. Resolve its quota pool `pool = resolveModelPool(selectedModel)` (`gemini` | `argon` | `claude_gpt`).
  2. Read `items` from `latestSnapshot.groups[pool].items`.
  3. Determine per-credential model eligibility (`isModelEligible(item, selectedModel, pool)`):
     - If `item.current.disabled` (host disabled): ineligible (`status = "disabled"`).
     - If `item.supported_models` is non-empty and does not contain `selectedModel` (case-insensitive, base model match): ineligible (`status = "unsupported_model"`).
     - If `pool == "claude_gpt"` and `claude_5h.remaining == null && claude_7d.remaining == null` while `gemini` has quota (e.g., Free plan without Claude entitlement): ineligible (`status = "unsupported_model"`).
  4. **Call-Order Sorting & Tiering** (matching `scheduler.pick` behavior):
     - Eligible active credentials (`target.priority > 0` and not in cooldown/depleted) sorted by `target.priority` descending, then `urgency` descending, then `t5h` ascending, then `auth_index` ascending.
     - Assign **Call Tier Rank** (`Tier #1`, `Tier #2`, ...) where credentials sharing the same `target.priority` share the same Tier number (indicating Round-Robin peer load balancing within that tier, just like CPA's Routing Board).
     - Soft-depleted / 429 cooling credentials (`target.priority == -1`) and Weekly-depleted / Host-disabled / Model-unsupported credentials are placed after active tiers in deterministic order.
  5. **Card Layout & 5 Quota Progress Bars**:
     - **Card Top Row**:
       - Call Order Badge: `Tier #1` (or `🚀 Tier #1 · Boost` for 999, `⏸ 冷却避让` / `🚫 耗尽避让` / `⛔ 不支持该模型` for bypassed items)
       - Credential Email / Name + Plan Badge (`PRO` / `FREE` / `ULTRA`)
       - Dynamic Priority Pill (`动态优先级: 100`), Urgency Pill (`紧迫度: 0.85`), Burn Rate Pill (`C_cycle: 0.15`), and action buttons (`⚡ 探测`, `📊 采样`).
     - **5 Quota Progress Bars Grid**:
       Render all 5 quota bars on every credential card:
       1. `Gemini 5h` (`quota_windows.gemini_5h`)
       2. `Gemini 7d` (`quota_windows.gemini_7d`)
       3. `Claude 5h` (`quota_windows.claude_5h`)
       4. `Claude 7d` (`quota_windows.claude_7d`)
       5. `Argon 7d` (`quota_windows.argon_7d`)
       - Highlight the bars that govern the currently selected model's pool with an `.active-pool-meter` class:
         - Selected `gemini` model → highlight `Gemini 5h` + `Gemini 7d`
         - Selected `argon` model → highlight `Gemini 5h` + `Argon 7d`
         - Selected `claude_gpt` model → highlight `Claude 5h` + `Claude 7d`
       - Each bar shows: Label, Effective Remaining `%` (or `N/A` if unavailable on plan), optional real-time estimated deduction badge when `estimated_burn_pct > 0` (e.g. `78% (预估 -2%)` / `78% (est. -2%)`), live reset countdown (`data-reset-time`), and health color (`healthy` >30%, `warning` 11-30%, `danger` <=10%).

### 4. Samples Modal & Config Panel Cleanup (`internal/management/feature_overview_samples.go`, `feature_config_markup.go`, `feature_config_actions.go`)
- In the Samples Modal (`openSamplesModal`), allow viewing samples for the active pool (`gemini`, `argon`, or `claude_gpt`) corresponding to the currently selected model (with a pool switcher pill inside the modal).
- In `feature_config_markup.go` & `feature_config_actions.go`:
  - Remove the obsolete `cfgModelGroup` (`antigravity_model_group` — "配额主控模型组") form row since all 3 quota pools are now simultaneously active and dynamically routed per request model.
  - Remove the obsolete `cfgMinChange` (`min_change` — "最小写回阈值") form row since dynamic `scheduler.pick` operates in memory without mutating host disk files, and equal-priority clustering is governed by `urgency_tolerance`.
  - Update `cfgAutoApply` label/description to **"开启后台定时配额校准 (Automatic Quota Calibration Probe)"** (default `true`).
  - Keep all active dynamic config fields (`auto_apply`, `interval`, `window_enabled`, `window_start`, `window_end`, `max_concurrency`, `urgency_tolerance`, `rate_limit_cooldown_minutes`, `quota_sample_capacity`, `boost_start_priority`, `normal_start_priority`, `free_start_priority`) intact and validated.

## Acceptance Criteria

- [ ] Top dropdown in Overview displays concrete models grouped under `Gemini`, `Argon`, and `Claude & GPT`, using custom styled dropdown primitives (zero raw `<select>`).
- [ ] Selecting `gemini-3.8-flash-high`, `gemini-4-argon-high`, or `claude-sonnet-5-5-high` immediately re-sorts the credential cards by that model's exact call order (`Tier #1`, `Tier #2`, ...) and updates KPI counters.
- [ ] Every credential card displays all 5 quota progress bars (`Gemini 5h`, `Gemini 7d`, `Claude 5h`, `Claude 7d`, `Argon 7d`) with accurate percentages and reset countdowns, visually highlighting the bars governing the selected model.
- [ ] Both `zh-CN` and `en-US` translations are complete and aligned across all modified UI components.
- [ ] All management handler and template tests in `internal/management` pass.
