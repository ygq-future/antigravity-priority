# Spec: Dynamic Model-Scoped Quota Scheduler & Routing Board

Status: `ready-for-agent`

## Problem Statement

Each Google Antigravity OAuth credential owns three independent quota resource pools at the same time:
1. **Standard Gemini Quota Pool (`gemini`)**: 5-hour rolling window (`gemini_5h`) + 7-day weekly window (`gemini_7d`).
2. **Gemini 4 Argon Quota Pool (`argon`)**: 5-hour shared Gemini rolling window (`gemini_5h`) + 7-day Argon weekly window (`argon_7d`, falling back to `gemini_7d` on plans that merge Argon into the shared weekly pool).
3. **Claude & GPT Quota Pool (`claude_gpt`)**: 5-hour rolling window (`claude_5h`) + 7-day weekly window (`claude_7d`).

CPA Host supports native per-request plugin scheduling via `Capabilities.Scheduler` (`scheduler.pick` JSON-RPC) with `SchedulerAcrossPriorities: true` (`sdk/pluginapi/types.go:103-107, 575-649`; `sdk/cliproxy/auth/conductor_selection.go:635-652, 960-1025`). By registering `scheduler.pick`, `antigravity-priority` can evaluate the exact requested `Model` on every inbound request, resolve its corresponding Antigravity quota pool (`gemini`, `argon`, or `claude_gpt`), and dispatch the request to the optimal credential in memory without mutating static `priority` or `disabled` fields on disk.

Correspondingly, the embedded Web Management UI needs to present a **model-driven routing board** aligned with CPA's routing orchestration view:
- Credentials are sorted by their active priority and call order for the selected model;
- The top selector is a **concrete Model selector** (e.g., `gemini-3.8-flash-high`, `gemini-4-argon-high`, `claude-sonnet-5-5-high`, `claude-opus-5-5-high`, `gpt-oss-120b-medium`, etc., discovered from `host.auth.models` + built-in Antigravity catalog);
- Selecting a model immediately displays the exact credential call sequence, tiering, and eligibility for that model;
- Every credential card simultaneously displays all five quota progress bars: **Gemini 5h**, **Gemini 7d**, **Claude 5h**, **Claude 7d**, and **Argon 7d**.

---

## Solution Overview

1. **Three-Pool Quota Domain (`gemini`, `argon`, `claude_gpt`)**:
   - Promote `argon` to a first-class `QuotaPool` (`config.AntigravityModelGroupArgon = "argon"`) alongside `gemini` and `claude_gpt`.
   - A single `retrieveUserQuotaSummary` / `fetchAvailableModels` response already contains all three pools (`internal/provider/antigravity/parse.go`). `ParseAllModelGroups` extracts and persists independent `ProbeResult` and `state.Entry` records (including independent `QuotaSample` history and learned `CycleBurnRate` $C_{\text{cycle}}$) for all three pools in one pass.
   - Each pool independently computes its own `priority.Plan` (Weekly Urgency Index, Dynamic Boost Horizon $T_{\text{required}}$, Equal Priority Clustering, Soft/Hard Depletion, and pool-scoped 429 Cooldown).

2. **In-Memory Per-Request Scheduler (`scheduler.pick`)**:
   - Register `"scheduler": true` and `"scheduler_across_priorities": true` in `plugin.register` / `plugin.reconfigure` (`internal/runtime/runtime.go`).
   - Implement `MethodSchedulerPick = "scheduler.pick"` handler in `Runtime.Handle`:
     - Inspect `req.Provider` / `req.Providers` and `req.Model`. For non-Antigravity requests, return `{"handled": false}` immediately so CPA delegates to its built-in selector.
     - Map `req.Model` (stripping optional `<prefix>/` and thinking suffix `(...)`) to its target quota pool: `argon`, `claude_gpt`, or `gemini`.
     - Map `req.Candidates` (`SchedulerAuthCandidate`) to cached Antigravity credentials via `AuthIndex` (derived from `Attributes["path"]` / `ID` or cached `ID -> AuthIndex` lookup table).
     - Filter out candidates that are depleted ($R_{\text{5h}} \le 0$ or $R_{\text{7d}} \le 0$) or in active 429 cooldown for the resolved quota pool.
     - Select the highest-priority tier from the pool's active `priority.Plan`. Within the top equal-priority cluster, honor `canonical_session_id` affinity if bound to a healthy top-tier peer, otherwise rotate via deterministic Round-Robin cursor per quota pool, and return `{"auth_id": selected.ID, "handled": true}`.

3. **Zero-Disk-Write Pure Memory Scheduling & Background Quota Refresh**:
   - Background ticker and manual Probe refresh quota observations in memory and persist them only to the plugin's own state cache (`data/antigravity-priority-cache.json`), keeping CPA host credential JSON files untouched during normal scheduling.
   - Retain `host.auth.models` (`MethodAuthModels = "host.auth.models"`) to discover per-credential callable models from CPA's global registry (`internal/pluginhost/auth_callbacks.go:121-178`).
   - Reactive 429 handling (`usage.handle` / `filter.*`) records pool-scoped cooldowns (`(authIndex, quotaPool)`) in memory and state cache, isolating rate-limited pools without disabling sibling pools on the same credential.

4. **Model-Centric Routing Board UI & Unified 5-Bar Credential Cards**:
   - Replace the legacy 2-option model-group dropdown in the Overview control bar with a **Model Selector Dropdown** populated from live `host.auth.models` + standard Antigravity models, grouped visually by quota pool (`Gemini` / `Argon` / `Claude & GPT`).
   - Selecting any model dynamically sorts and renders the credential list in exact **call order (Priority descending → Urgency descending → 5h Reset ascending → AuthIndex ascending)** for that model, marking credentials that do not support the selected model (via `host.auth.models` or plan restrictions) or are depleted/cooling in that model's pool.
   - Every credential card renders all **5 quota progress bars** side-by-side / stacked in a compact, high-density layout:
     - **Gemini 5h** (`gemini.short_window_remaining` + reset countdown)
     - **Gemini 7d** (`gemini.long_window_remaining` + reset countdown)
     - **Claude 5h** (`claude_gpt.short_window_remaining` + reset countdown)
     - **Claude 7d** (`claude_gpt.long_window_remaining` + reset countdown)
     - **Argon 7d** (`argon.long_window_remaining` / `argon_window_remaining` + reset countdown)
     with the 2 bars (or 5h+7d pair) governing the currently selected model visually highlighted as active for the current route.

---

## Upstream CPA Contract & Verified Facts

All implementation work MUST adhere to these verified upstream CLIProxyAPI (v8.0.55) contracts:

1. **Plugin Registration (`internal/pluginhost/rpc_schema.go:17-53`)**:
   ```json
   {
     "schema_version": 1,
     "metadata": { ... },
     "capabilities": {
       "management_api": true,
       "management": true,
       "usage_plugin": true,
       "scheduler": true,
       "scheduler_across_priorities": true
     }
   }
   ```
2. **`scheduler.pick` RPC Request & Response (`sdk/pluginabi/types.go:75`, `sdk/pluginapi/types.go:587-686`)**:
   - Method name: `"scheduler.pick"`
   - Request payload (`SchedulerPickRequest`):
     ```go
     type SchedulerPickRequest struct {
         Provider   string                   `json:"Provider"`
         Providers  []string                 `json:"Providers"`
         Model      string                   `json:"Model"`
         Stream     bool                     `json:"Stream"`
         Options    SchedulerOptions         `json:"Options"`
         Candidates []SchedulerAuthCandidate `json:"Candidates"`
     }
     type SchedulerOptions struct {
         Headers  map[string][]string `json:"Headers"`
         Metadata map[string]any      `json:"Metadata"` // includes "canonical_session_id" when present
     }
     type SchedulerAuthCandidate struct {
         ID         string            `json:"ID"`
         Provider   string            `json:"Provider"`
         Priority   int               `json:"Priority"`
         Status     string            `json:"Status"`
         Attributes map[string]string `json:"Attributes"` // includes "path", "source", "email", "account_email", etc.
         Metadata   map[string]any    `json:"Metadata,omitempty"`
     }
     ```
   - Response payload (`SchedulerPickResponse` wrapped in `Envelope{OK: true, Result: ...}`):
     ```go
     type SchedulerPickResponse struct {
         AuthID          string `json:"auth_id,omitempty"`
         DelegateBuiltin string `json:"delegate_builtin,omitempty"`
         Handled         bool   `json:"handled"`
         Reject          bool   `json:"reject,omitempty"`
         RejectReason    string `json:"reject_reason,omitempty"`
         RejectCode      string `json:"reject_code,omitempty"`
     }
     ```
   - **Validation Rule (`internal/pluginhost/scheduler.go:76-109`)**:
     - When `Handled: false`, CPA ignores the plugin scheduler and runs its built-in selector.
     - When `Handled: true` and `AuthID != ""`, `AuthID` **MUST** exactly match one `candidate.ID` in `req.Candidates`; otherwise CPA logs a warning and falls back to built-in selection.
     - `scheduler.pick` is a **pure plugin-call RPC** (`rpcPluginAdapter.Pick` in `internal/pluginhost/rpc_client.go:428-430`); it does **not** open a `host_callback_id` context. Therefore, **`scheduler.pick` MUST NOT invoke `host.*` callbacks synchronously during `Pick`**. It must read exclusively from in-memory runtime state populated by startup/probe/sync workers.

3. **Candidate `ID` to `AuthIndex` Resolution (`sdk/cliproxy/auth/types.go:631-728`, `internal/pluginhost/auth_callbacks.go:196-205, 761-829`)**:
   - In CPA, `HostAuthFileEntry` (returned by `host.auth.list`) and `HostAuthModels` (returned by `host.auth.models`) contain both `ID` (`string`) and `AuthIndex` (`string`, 16-hex SHA-256 prefix of `authType + ":" + cleanAbsPath`).
   - In `SchedulerPickRequest.Candidates`, each candidate carries `ID`, `Provider`, and `Attributes` (including `Attributes["path"]`, `Attributes["email"]`, etc., as `path` and `email` are not filtered by `schedulerAttributeSensitive` in `conductor_selection.go:845-869`).
   - By parsing `ID` in `host.AuthFile` (`internal/host/types.go:62`) from `host.auth.list` (and `host.auth.models`), Runtime maintains an exact in-memory bi-directional index `authID <-> authIndex` (with fallback matching on normalized `Attributes["path"]` and `Name`).

4. **Host Per-Credential Model Discovery (`internal/pluginhost/auth_callbacks.go:121-178`)**:
   - Host RPC method: `"host.auth.models"`
   - Request: `{}` (all credentials) or `{"auth_index": "..."}`
   - Response:
     ```go
     type HostAuthModelsResponse struct {
         Credentials []HostAuthModels `json:"credentials"`
     }
     type HostAuthModels struct {
         ID        string   `json:"id"`
         AuthIndex string   `json:"auth_index"`
         Provider  string   `json:"provider"`
         Models    []string `json:"models"`
     }
     ```
   - Calling `host.auth.models` during Sync/Probe/Startup provides both:
     1. The exact `ID <-> AuthIndex` mapping for every credential;
     2. The live list of callable `Models` registered in CPA's `GlobalRegistry` for each Antigravity credential.

---

## Implementation Decisions & Boundaries

### 1. Quota Pools & Model Classification (`internal/config`, `internal/provider/antigravity`)

1. **Canonical Quota Pools**:
   - Define three canonical quota pools in `config.AntigravityModelGroup`:
     - `AntigravityModelGroupGemini = "gemini"`
     - `AntigravityModelGroupArgon = "argon"`
     - `AntigravityModelGroupClaudeGPT = "claude_gpt"`
   - All three pools are active scheduling authorities (`AllModelGroups = []AntigravityModelGroup{AntigravityModelGroupGemini, AntigravityModelGroupArgon, AntigravityModelGroupClaudeGPT}`).

2. **Model-to-Pool Classifier (`ClassifyModelQuotaPool(model string) AntigravityModelGroup`)**:
   - Normalize `model`:
     1. Trim whitespace and lowercase;
     2. Strip optional credential prefix (`teamA/gemini-4-argon-high` → `gemini-4-argon-high`);
     3. Strip optional thinking suffix (`claude-sonnet-5-5-high(16384)` → `claude-sonnet-5-5-high`).
   - Classification rules (deterministic precedence):
     1. If model contains `"argon"` → `AntigravityModelGroupArgon` (`"argon"`);
     2. If model contains `"claude"`, `"opus"`, `"sonnet"`, `"haiku"`, `"fable"`, `"gpt"`, `"o1"`, `"o3"`, `"o4"`, or `"openai"` → `AntigravityModelGroupClaudeGPT` (`"claude_gpt"`);
     3. Otherwise (all standard `gemini-*` models) → `AntigravityModelGroupGemini` (`"gemini"`).

3. **Three-Pool Quota Extraction (`ParseAllModelGroups` in `internal/provider/antigravity/parse.go`)**:
   - `ParseAllModelGroups(raw []byte, observedAt time.Time)` returns a map with all three keys: `gemini`, `argon`, and `claude_gpt`.
   - **`gemini` pool**:
     - `ShortWindowRemaining` / `ShortWindowResetAt`: conservative 5h window from `Gemini Models` group (or non-Argon Gemini models).
     - `LongWindowRemaining` / `LongWindowResetAt`: conservative non-Argon 7d window from `Gemini Models` group.
     - `ArgonWindowRemaining` / `ArgonWindowResetAt`: populated for reference when present.
   - **`argon` pool**:
     - `ShortWindowRemaining` / `ShortWindowResetAt`: shared 5h window from `Gemini Models` group (because Antigravity gates Argon burst traffic by the account's 5h Gemini short window when present).
     - `LongWindowRemaining` / `LongWindowResetAt`: dedicated 7d weekly window from `Gemini 4 Argon` group / `argon` buckets (`ArgonWindowRemaining` / `ArgonWindowResetAt`).
     - **Ultra / Shared-Bucket Fallback**: As documented in Google's `Gemini 4 Argon` group description (*"For paid Pro plans, Argon quota is separate. Higher Argon quota (included as part of the larger Gemini quota bucket) is available on Ultra plans"*), if a credential has a valid `Gemini Models` quota response but no separate `argon` weekly bucket, `argon` pool falls back to the shared `gemini` 7d window (`LongWindowRemaining` / `LongWindowResetAt`) so Ultra/Free accounts remain schedulable for Argon models.
   - **`claude_gpt` pool**:
     - `ShortWindowRemaining` / `ShortWindowResetAt`: conservative 5h window from `Claude and GPT Models` group.
     - `LongWindowRemaining` / `LongWindowResetAt`: conservative 7d window from `Claude and GPT Models` group.

### 2. State Store, Adaptive Burn Rate & Evidence Authority (`internal/state`, `internal/evidence`)

1. **Per-Pool State Entries**:
   - `state.Store` keys entries by `(provider, modelGroup, authIndex)`.
   - During every probe round, `RecordProbeSuccess` is called for each of the 3 pools (`gemini`, `argon`, `claude_gpt`).
   - Each pool independently tracks its own `Samples []QuotaSample`, `LearningBaselineSequence`, and EMA `CycleBurnRate` $C_{\text{cycle}}$.
   - For `argon`, when 5h short window and Argon 7d window both advance ($\Delta R_{\text{5h}} \ge 5\%, \Delta R_{\text{7d,argon}} > 0$), $C_{\text{cycle}}$ learns Argon's specific burn ratio; otherwise it uses `DefaultCycleBurnRate` (0.15).

2. **Pool-Scoped Cooldowns**:
   - 429 reactive cooldowns in `state.Store` and `Runtime` are keyed by `cooldownKey(authIndex, modelGroup)` (or stored per `(authIndex, modelGroup)`).
   - When `usage.handle` or `filter.*` observes a 429 for `payload.Model`, `modelGroupForUsage(payload.Model)` classifies the model into `gemini`, `argon`, or `claude_gpt` via `ClassifyModelQuotaPool`.
   - Triggering a 429 cooldown on `claude_gpt` demotes that credential to `Priority = -1` (`Reason429Cooldown`) **only** in the `claude_gpt` plan; its `gemini` and `argon` plans remain unaffected and continue serving traffic at their normal priorities.
   - Subsequent successful usage on `(authIndex, modelGroup)` clears the pending 429 counter and recovers active cooldown for that specific `modelGroup`.

### 3. Multi-Pool Projection & Model Routing Table (`internal/runtime`, `internal/priority`, `internal/apply`)

1. **All-Active Multi-Pool Projection**:
   - Replace the two-group `Control vs Predicted` dichotomy with **Multi-Pool Active Projection** across all 3 canonical pools (`gemini`, `argon`, `claude_gpt`).
   - Every pool's `Plan` is a live, authoritative scheduling plan (`IsPredicted` is always `false`).
   - Each credential in the snapshot carries a unified **5-Window Quota Summary** (`QuotaWindows`) alongside its per-pool scheduling evaluation:
     - `gemini_5h` (`remaining`, `reset_at`)
     - `gemini_7d` (`remaining`, `reset_at`)
     - `claude_5h` (`remaining`, `reset_at`)
     - `claude_7d` (`remaining`, `reset_at`)
     - `argon_7d` (`remaining`, `reset_at`)
     so that any view in the UI can render all 5 progress bars on every credential card without cross-referencing separate group arrays.

2. **Built-in & Live Model Catalog (`ModelCatalog`)**:
   - Maintain a canonical list of Antigravity models and their metadata:
     - Built-in defaults (matching CPA `internal/registry/models/models.json` `antigravity` catalog):
       - **Gemini Pool**: `gemini-3.8-flash-high`, `gemini-3-flash`, `gemini-3.5-flash-lite`, `gemini-3.1-flash-lite`, `gemini-3.1-flash-image`
       - **Argon Pool**: `gemini-4-argon-high`
       - **Claude & GPT Pool**: `claude-sonnet-5-5-high`, `claude-opus-5-5-high`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`, `gpt-oss-120b-medium`
     - Live discovery via `host.auth.models`: merge any additional models dynamically advertised by Antigravity credentials in CPA Host, and record per-credential supported model sets (`supportedModelsByAuthIndex map[string]map[string]struct{}`).
     - If `host.auth.models` is unavailable (e.g., older host or test stub) or returns an empty model list for a credential whose probe succeeded, fall back to pool-level eligibility (e.g., Free plan credentials without Claude quota are marked ineligible for `claude_gpt` models when `claude_gpt` probe reports unavailable).

3. **Precomputed O(1) Routing Table vs. Real-Time `scheduler.pick` Execution Engine**:
   - **Two-Phase Architecture (Asynchronous Precomputation + O(1) Synchronous Lookup)**:
     - **Phase A — Background / Event-Driven Precomputation (Outside `scheduler.pick`)**:
       Whenever any state changes—(1) Plugin Startup, (2) Scheduled or Manual Quota Probe (`runProductionTask`), (3) Host Sync (`SyncHost`), (4) 429 Cooldown Triggered/Restored (`triggerCooldown` / `restoreCooldown`), or (5) Config Reconfigure—Runtime runs `priority.PlanWithEvidence` for all 3 quota pools (`gemini`, `argon`, `claude_gpt`) and publishes an immutable **Precomputed Routing Table** (`routingTable` pointer) into memory:
       ```go
       type PoolRoutingTable struct {
           DecidedAt      time.Time
           ByAuthIndex    map[string]PrecomputedCandidateState // O(1) lookup by AuthIndex
           NextWindowEnds time.Time                            // Earliest 5h/7d ResetAt or Cooldown expiry across all credentials in this pool
       }
       type PrecomputedCandidateState struct {
           AuthIndex          string
           Priority           int       // 999 (Boost), 100.. (Normal), 10.. (Free), 0 (Unknown), -1 (Soft-Depleted/Cooldown), -2 (Weekly Hard-Depleted)
           Urgency            float64
           ShortWindowResetAt time.Time
           LongWindowResetAt  time.Time
           CooldownUntil      time.Time
           Reason             string
       }
       ```
     - **Phase B — Lazy Window-Boundary Refresh**:
       Even between 15-minute probes, a credential's 5h short window (`ShortWindowResetAt`) or 429 cooldown (`CooldownUntil`) can expire, or time advancing can push an account into the Dynamic Boost Horizon ($T_{\text{7d}} \le T_{\text{required}}$).
       - If `now >= poolTable.NextWindowEnds` (or >60s has elapsed since `poolTable.DecidedAt`), `scheduler.pick` performs a fast in-memory re-evaluation of `PlanWithEvidence` using the cached `QuotaEvidence` at `now` (taking <50µs for 50 credentials, zero I/O, zero Host RPC) and atomically updates `poolTable`.
       - Specifically, if a credential was soft-depleted ($R_{\text{5h}} = 0$) or in 429 cooldown, and `now >= ShortWindowResetAt` (or `now >= CooldownUntil`), it automatically promotes out of `-1` in memory without waiting for the next HTTP probe!
     - **Phase C — O(1) `scheduler.pick` Hot Path**:
       1. **Provider & Enabled Check**: If `!r.cfg.Enabled`, return `SchedulerPickResponse{Handled: false}`. Partition `req.Candidates` into `antigravityCandidates` (`candidate.Provider == "antigravity"`) and `otherProviderCandidates` (`candidate.Provider != "antigravity"`). If `len(antigravityCandidates) == 0`, return `SchedulerPickResponse{Handled: false}`.
       2. **Pool Lookup**: `pool := ClassifyModelQuotaPool(req.Model)`, then read `poolTable := r.getRoutingTable(pool, now)`.
       3. **O(1) Candidate Scoring**: Map each `candidate` in `antigravityCandidates` via `authIDToIndex[candidate.ID]` to `poolTable.ByAuthIndex[authIndex]` to read its precomputed `Priority`.
       4. **Cross-Provider Fallback & Depletion Hand-Off Boundary (`Handled: false` vs `Handled: true`)**:
          - **Case 1 — Healthy Antigravity Candidate Exists (`bestPriority > 0`)**:
            At least one Antigravity credential has positive quota in `pool` (`999`, `100..`, or `10..`) or cold-start unknown state (`0`). Select from the `bestPriority` cluster (applying Session Affinity if `canonical_session_id` is bound to a member of that cluster, else Round-Robin via `rrCursorByPool[pool]`) and return `SchedulerPickResponse{AuthID: picked.ID, Handled: true}`.
          - **Case 2 — All Antigravity Candidates Depleted / Cooling (`bestPriority < 0`) AND Non-Antigravity Candidates Exist (`len(otherProviderCandidates) > 0`)**:
            When the user also configured other providers for the same model (e.g., Anthropic / OpenAI / Vertex / Gemini CLI alongside Antigravity in a `"mixed"` route) and **all** Antigravity credentials have exhausted their quota or are in 429 cooldown for `pool`, **immediately return `SchedulerPickResponse{Handled: false}`**!
            - **Important Nuance in Mixed Mode**: When returning `Handled: false`, CPA's built-in `selector.Pick` receives `selectorAuths` (`conductor_selection.go:1969`). To prevent CPA's built-in selector from accidentally picking one of the depleted Antigravity credentials anyway, if `len(otherProviderCandidates) > 0` and all Antigravity candidates are depleted (`bestPriority < 0`), our plugin can either:
              - Directly pick from `otherProviderCandidates` (respecting their highest static `candidate.Priority` and Round-Robin) and return `{AuthID: bestOther.ID, Handled: true}`, **OR**
              - If `len(otherProviderCandidates) == 0` (Antigravity-only route), return `{Handled: false}` when all are hard-depleted ($R_{\text{7d}} \le 0$) or let CPA's retry loop fail fast.
            - **Why picking from `otherProviderCandidates` when `bestAntigravity < 0` is superior to bare `Handled: false` in Mixed Mode**:
              In CPA `conductor_selection.go:1969`, when a plugin returns `Handled: false`, CPA calls `selector.Pick(..., selectorAuths)`, where `selectorAuths` **still contains the Antigravity credentials** (because CPA Host doesn't know their Antigravity pool is depleted in our plugin memory!). If we merely returned `Handled: false`, CPA's built-in Round-Robin selector would still route 50% of requests to the depleted Antigravity credentials! By contrast, when all Antigravity candidates are depleted (`bestPriority < 0`) and `len(otherProviderCandidates) > 0`, selecting from the highest-priority `otherProviderCandidates` (or delegating among `otherProviderCandidates`) guarantees **100% of traffic cleanly fails over to the healthy non-Antigravity provider** without hitting depleted Antigravity accounts!
          - **Case 3 — Antigravity-Only Route (`len(otherProviderCandidates) == 0`) & All Antigravity Candidates Depleted (`bestPriority < 0`)**:
            - If all Antigravity candidates are **Hard-Depleted** ($R_{\text{7d}} \le 0$) or in active 429 cooldown, return `SchedulerPickResponse{Handled: false}` (or `Reject: true, RejectCode: "quota_depleted"` if appropriate, or `Handled: false` to let CPA default handling take over).
            - Note: If an Antigravity candidate fails upstream during a request, CPA's `executeMixedOnce` adds `tried[auth.ID] = struct{}{}` (`conductor_execution.go:535`) and automatically re-invokes `scheduler.pick` with the failed credential removed from `req.Candidates`, so failover across credentials and across providers is completely automatic within a single request!
### 4. Real-Time Quota Consumption Estimation & Periodic Probe Calibration (`internal/state`, `internal/runtime`)

1. **Core Concept ("Real-Time Usage Deduction + Periodic Probe Calibration")**:
   - Instead of waiting up to 15 minutes for the next HTTP probe to see quota bars move, the plugin intercepts every completed Antigravity request via `usage.handle` (`pluginapi.UsageRecord`, delivered by CPA `adapters_usage_translation.go:580-666` immediately after every streaming or non-streaming turn).
   - `UsageRecord.Detail` provides exact token telemetry:
     - `InputTokens`, `OutputTokens`, `ReasoningTokens`, `CachedTokens`, `CacheReadTokens`, `CacheCreationTokens`, `TotalTokens`.
   - Each `state.Entry` (per `(authIndex, quotaPool)`) tracks:
     - **Calibrated Baseline (`ShortWindowRemaining`, `LongWindowRemaining`)**: The authoritative server-reported remaining percentages from the most recent Google probe (`retrieveUserQuotaSummary`).
     - **Accumulated Uncalibrated Usage (`PendingShortTokens`, `PendingLongTokens`, `EstimatedShortBurnPct`, `EstimatedLongBurnPct`)**: Token usage and estimated percentage burned since `ObservedAt`.
     - **Adaptive Token-to-Quota Sensitivity (`TokensPerShortPct` $K_{\text{5h}}$)**: Learned per `(planType, quotaPool)` / per credential whenever a new HTTP probe arrives!
   - **Closed-Loop Auto-Calibration**:
     1. **During Conversation (`usage.handle` arrives)**:
        - Compute Weighted Equivalent Tokens ($W_{\text{turn}}$) for the turn:
          $$W_{\text{turn}} = (\text{InputTokens} - \text{CacheReadTokens}) + 0.1 \times \text{CacheReadTokens} + 4.0 \times (\text{OutputTokens} + \text{ReasoningTokens})$$
          (Fallback: if `TotalTokens == 0`, use a minimum turn floor $W_{\text{floor}} = 2000$ equivalent tokens).
        - Accumulate `PendingWeightedTokens += W_turn` on the credential's active `quotaPool` (and if `quotaPool == argon`, also accumulate on `gemini` 5h short window since Gemini and Argon share the 5h burst window; and vice versa).
        - Estimate percentage burned since last probe:
          $$\Delta \hat{R}_{\text{5h}} = \frac{\text{PendingWeightedTokens}}{K_{\text{5h}}}, \quad \Delta \hat{R}_{\text{7d}} = \Delta \hat{R}_{\text{5h}} \times C_{\text{cycle}}$$
        - Effective real-time remaining quota displayed on UI and used by `PoolRoutingTable`:
          $$R_{\text{5h,eff}} = \max\left(0, R_{\text{5h,probe}} - \Delta \hat{R}_{\text{5h}}\right), \quad R_{\text{7d,eff}} = \max\left(0, R_{\text{7d,probe}} - \Delta \hat{R}_{\text{7d}}\right)$$
        - Immediately update the in-memory `PoolRoutingTable` and `DualGroupSnapshot`! If $R_{\text{5h,eff}}$ drops a credential into a lower priority tier or `0%` soft-depletion, `scheduler.pick` switches to the next healthier credential **on the very next turn** without waiting for a 429 or a 15-minute probe!
     2. **When Periodic / Manual Probe Arrives (`MarkProbeSuccess` — Calibration Step)**:
        - Compare the actual server-reported quota drop $\Delta R_{\text{5h,actual}} = R_{\text{5h,prev\_probe}} - R_{\text{5h,new\_probe}}$ against the `PendingWeightedTokens` accumulated during that interval.
        - If $\Delta R_{\text{5h,actual}} \ge 2\%$ and `PendingWeightedTokens > 0` within the same 5h reset window:
          - Compute observed sensitivity $K_{\text{obs}} = \frac{\text{PendingWeightedTokens}}{\Delta R_{\text{5h,actual}}}$ and smooth `TokensPerShortPct` via EMA ($\alpha = 0.3$, clamped to reasonable plan bounds).
        - Reset `PendingWeightedTokens = 0`, `EstimatedShortBurnPct = 0`, `EstimatedLongBurnPct = 0`, and snap the quota bars to the exact Google server truth (`R_5h = R_5h,new_probe`, `R_7d = R_7d,new_probe`).

### 5. Configuration Audit: Removed, Repurposed & Retained Settings (`internal/config`, `internal/management`)

1. **Settings to REMOVE from UI Config Center & `buildMetadata()`**:
   - ❌ **`antigravity_model_group` ("配额主控模型组 / Control Model Group")**:
     - **Why obsolete**: All three quota pools (`gemini`, `argon`, `claude_gpt`) are now simultaneously active and dynamically selected per request model via `scheduler.pick`.
   - ❌ **`min_change` ("最小写回阈值 / Minimum Priority Change Threshold")**:
     - **Why obsolete**: `min_change` was previously an anti-thrashing gate to avoid frequent disk writes (`host.auth.save`) when priority fluctuated by 1–2 points. In pure in-memory `scheduler.pick` mode, there are zero disk writes to CPA credential files, and intra-tier load balancing is controlled by `urgency_tolerance` (Equal Priority Clustering). Removing `min_change` from the UI eliminates a dead concept.
   - *(Backward Compatibility Note: Both `antigravity_model_group` and `min_change` remain accepted as optional/ignored fields in Go JSON/YAML unmarshaling so existing `config.yaml` and `antigravity-priority-cache.json` files upgrade with zero errors).*

2. **Settings to REPURPOSE (Keep Key, Update Semantics & UI Label)**:
   - 🔄 **`auto_apply` + `interval` ("开启后台定时配额校准 / Automatic Quota Calibration Probe")**:
     - Previously labeled "开启后台自动调度 (Auto Apply)" (default `false`).
     - Now controls the **Background Periodic Quota Calibration Probe** (default `true`, default `interval = "15m"`), which periodically fetches authoritative quota from Google to calibrate the real-time usage estimator.

3. **Settings to RETAIN (Still Essential)**:
   - ✅ **`schedule.window_enabled` / `window_start` / `window_end`**: Controls active time window for background calibration probes.
   - ✅ **`max_concurrency`**: Max concurrent HTTP requests during quota calibration probes (1–32, default 6).
   - ✅ **`urgency_tolerance`**: Equal-priority clustering tolerance (default `0.05`). Directly controls which credentials share the same priority tier for Round-Robin / Session Affinity in `scheduler.pick`.
   - ✅ **`rate_limit_cooldown_minutes`**: 429 reactive cooldown duration per quota pool (default `5` min).
   - ✅ **`quota_sample_capacity`**: Sliding window sample capacity for EMA learning of $C_{\text{cycle}}$ and token sensitivity $K_{\text{5h}}$ (default `6`).
   - ✅ **`priority_rules` (`boost_start_priority: 999`, `normal_start_priority: 100`, `free_start_priority: 10`)**: Base priority scores for the three scheduling tiers.

4. **Do We Need Any New User-Facing Config Keys?**
   - **No mandatory new config keys are required!** Following Core Principle #2 (*Adaptive learning with zero manual configuration burden*), the real-time quota consumption estimator is enabled out-of-the-box with sensible pool-specific cold-start baselines (`DefaultWeightedTokensPerShortPct`: e.g. `40,000` weighted tokens per 1% 5h quota for `claude_gpt` / `argon`, `80,000` for `gemini`) and automatically self-calibrates via EMA on every probe round.

### 6. Web Management UI Redesign: Model-Centric Routing Board (`internal/management`)

The Overview dashboard (`feature_overview_*.go`) will be thoroughly redesigned to mirror CPA's Routing Board experience while showcasing Antigravity's multi-pool quota telemetry:

1. **Top Control Bar (Model Selector & Actions)**:
   - **Model Selector Dropdown (`#modelRouteSelect` / `#customModelSelect`)**:
     - Replaces the old 2-group (`Gemini` / `Claude & GPT`) dropdown.
     - Lists concrete callable models grouped under three pool headers:
       - **Gemini 额度池 (Gemini Quota Pool)**: `gemini-3.8-flash-high`, `gemini-3-flash`, `gemini-3.5-flash-lite`, `gemini-3.1-flash-lite`, `gemini-3.1-flash-image`, plus any custom/live Gemini models from `host.auth.models`.
       - **Argon 额度池 (Argon Quota Pool)**: `gemini-4-argon-high`, plus any live Argon models.
       - **Claude & GPT 额度池 (Claude & GPT Quota Pool)**: `claude-sonnet-5-5-high`, `claude-opus-5-5-high`, `claude-sonnet-4-6`, `claude-opus-4-6-thinking`, `gpt-oss-120b-medium`, plus any live Claude/GPT models.
     - Displays a pool pill badge next to the selected model name in the trigger button (e.g., `[Gemini 池] gemini-3.8-flash-high`, `[Argon 池] gemini-4-argon-high`, `[Claude 池] claude-sonnet-5-5-high`).
   - **Action Buttons**:
     - `🔄 刷新同步 (Sync Host)`: Calls `POST /sync` to sync credentials & models from CPA Host (`host.auth.list` + `host.auth.models`) and recompute all 3 pool plans.
     - `📡 刷新配额 (Probe Quotas)`: Calls `POST /run?mode=probe` to fetch fresh quota from Google Antigravity for all credentials and recompute all 3 pool plans.
     - `🧹 清除静态优先级 (Reset Static Priorities)`: Optional cleanup button (`POST /reset`) to clear legacy static priority fields from host auth files.
     - `🟢 动态调度运行中 / ⏸️ 已暂停`: Toggles automatic background probing schedule.

2. **KPI Summary Cards (Scoped to Selected Model)**:
   - **可用路由凭证 (Eligible Credentials)**: `X / Y` credentials eligible to serve the currently selected model right now.
   - **🚀 优先提权 (Dynamic Boosted)**: Count of credentials in the `999` Boost tier for the selected model's pool.
   - **耗尽 / 冷却避让 (Depleted / Cooling)**: Count of credentials bypassed due to 5h/7d depletion or 429 cooldown for the selected model's pool.
   - **当前生效调度池 (Active Routing Pool & Next Probe)**: Displays the resolved quota pool for the selected model (`Gemini` / `Argon` / `Claude & GPT`), current top tier priority, and next auto-probe countdown.

3. **Routing Order Credential List / Cards**:
   - **Default & Authoritative Sort Order**:
     1. Eligible & active credentials for the selected model first, sorted by **Priority descending** (e.g., `999` Boost → `100` Normal → `10` Free), then **Weekly Urgency descending**, then **5h Reset countdown ascending**, then **AuthIndex ascending**;
     2. Credentials with equal priority share the same **Call Order Tier Badge** (e.g., `Tier #1 · Priority 100 · 轮询分流`), clearly indicating CPA's dispatch order;
     3. Soft-depleted / 429 cooling credentials (`Priority = -1`), followed by Weekly Hard-Depleted / Host-Disabled / Model-Unsupported credentials at the bottom with dim/muted styling.
   - **Unified 5-Progress-Bar Credential Card**:
     Each card displays:
     - **Header**: Call Order Rank badge (`#1`, `#2`, ... or `⏸ 避让`), Credential Email/Name, Plan Type badge (`PRO` / `FREE` / `ULTRA`), Status badge (`🚀 优先提权` / `正常活跃` / `⏳ 429 冷却中` / `5h短窗耗尽` / `周额度耗尽` / `不支持该模型` / `已禁用`), current model's **Dynamic Priority Score**, **Urgency**, **Burn Rate $C_{\text{cycle}}$**, and quick action buttons (`⚡ 单独探测`, `📊 采样历史`).
     - **All 5 Quota Progress Bars (Always Visible)**:
       1. **Gemini 5h** (`gemini_5h` % + reset countdown)
       2. **Gemini 7d** (`gemini_7d` % + reset countdown)
       3. **Claude 5h** (`claude_5h` % + reset countdown)
       4. **Claude 7d** (`claude_7d` % + reset countdown)
       5. **Argon 7d** (`argon_7d` % + reset countdown)
       - The quota bars belonging to the **currently selected model's pool** (e.g., `Gemini 5h` + `Argon 7d` when `gemini-4-argon-high` is selected; `Claude 5h` + `Claude 7d` when `claude-sonnet-5-5-high` is selected; `Gemini 5h` + `Gemini 7d` when `gemini-3.8-flash-high` is selected) are highlighted with an active indicator border/badge so the administrator immediately sees which bars govern the current model's routing order, while still having full visibility into the account's other quotas.
       - If a pool is unavailable on an account (e.g., Claude 5h/7d on a Free-tier account), its bar renders in a clean muted `不可用 / N/A` state rather than hiding or breaking card alignment.

4. **Samples Modal (`📊 采样`)**:
   - Allows switching between `Gemini`, `Argon`, and `Claude & GPT` sample histories inside the modal (defaulting to the currently selected model's pool).

---

## Ticket Decomposition

Implementation is divided into 4 sequential, self-contained tickets under `.scratch/dynamic-model-routing/issues/`:

1. **`01-three-pool-domain-and-state.md`**:
   Expand domain model, parser, state store, evidence classifier, and planner projections to support three independent quota pools (`gemini`, `argon`, `claude_gpt`) and unified 5-window quota snapshots from a single probe pass.
2. **`02-cpa-scheduler-pick-and-model-catalog.md`**:
   Register `scheduler` + `scheduler_across_priorities` capabilities, integrate `host.auth.models` discovery and `ID <-> AuthIndex` mapping, implement `scheduler.pick` per-model dynamic dispatch with session affinity and round-robin clustering, and isolate 429 cooldowns per quota pool.
3. **`03-model-routing-board-and-five-bar-cards.md`**:
   Redesign the embedded Web Management UI Overview and Config panels: replace group selector with the concrete Model Selector dropdown, sort credentials by per-model call order/priority, render all 5 quota progress bars (`Gemini 5h`, `Gemini 7d`, `Claude 5h`, `Claude 7d`, `Argon 7d`) on every card, and update bilingual translations.
4. **`04-docs-convergence-and-quality-gates.md`**:
   Update `CONTEXT.md`, ADRs, `README.md` / `README.en.md`, run XERJ index sync, and pass all static analysis and race-detector test suites (`golangci-lint`, `go vet`, `go test -race ./...`).
