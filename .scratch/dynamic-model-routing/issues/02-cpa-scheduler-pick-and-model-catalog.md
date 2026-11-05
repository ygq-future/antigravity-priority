# 02: CPA `scheduler.pick` Dynamic Dispatch, Model Catalog & Pool-Scoped 429 Cooldown

Status: `ready-for-agent`
Type: `task`
Blocked by: 01

## What to Build

Integrate CPA's `Capabilities.Scheduler` (`scheduler.pick`) and `SchedulerAcrossPriorities: true` into `antigravity-priority`, add `host.auth.models` discovery for per-credential model support and `ID <-> AuthIndex` mapping, isolate 429 cooldowns per quota pool (`gemini`, `argon`, `claude_gpt`), and transition scheduled runs to pure in-memory + state-cache operation without writing static priorities to host auth files.

## Exact File Boundaries & Changes

### 1. Host Adapter & `host.auth.models` (`internal/host/types.go`, `internal/host/client.go`, `main.go`)
- In `internal/host/types.go`:
  - Add `MethodAuthModels = "host.auth.models"`.
  - Add `ID string` (`json:"id,omitempty"`) and `Path string` (`json:"path,omitempty"`) to `AuthFile` (`internal/host/types.go:62`) so `host.auth.list` captures CPA's `ID` and `Path` fields from `HostAuthFileEntry`.
  - Define `AuthModelsEntry` and `AuthModelsResponse`:
    ```go
    type AuthModelsEntry struct {
        ID        string   `json:"id"`
        AuthIndex string   `json:"auth_index"`
        Provider  string   `json:"provider"`
        Models    []string `json:"models"`
    }
    ```
  - Add `ListAuthModels(ctx context.Context) ([]AuthModelsEntry, error)` to `HostCallbacks` and `host.API` / `host.Client` (Gracefully returning `nil, nil` if a test mock or older host does not implement `host.auth.models`).
- In `main.go` & `cmd/devserver/devhost.go`:
  - Implement `ListAuthModels` on `hostCallbackAdapter` (calling `callHostJSON(ctx, host.MethodAuthModels, map[string]any{}, &resp)`) and on `devserver` mock host.

### 2. Plugin Registration & `scheduler.pick` RPC (`internal/runtime/types.go`, `internal/runtime/runtime.go`, `internal/runtime/scheduler.go`)
- In `internal/runtime/types.go`:
  - Add `MethodSchedulerPick = "scheduler.pick"`.
  - Define `SchedulerPickRequest`, `SchedulerOptions`, `SchedulerAuthCandidate`, and `SchedulerPickResponse` matching CPA's `sdk/pluginapi/types.go:587-686`.
- In `internal/runtime/runtime.go`:
  - Update `registrationResult()` capabilities:
    ```go
    Capabilities: map[string]bool{\n        "management_api":              true,\n        "management":                  true,\n        "usage_plugin":                true,\n        "scheduler":                   true,\n        "scheduler_across_priorities": true,\n    }
    ```
  - Add `case MethodSchedulerPick:` in `Runtime.Handle` routing to `r.handleSchedulerPick(ctx, request)`.
- Create `internal/runtime/scheduler.go`:
  - **Precomputed O(1) Routing Table (`PoolRoutingTable`)**:
    - Precompute and cache the 3 quota pool routing tables (`plansByPool map[config.AntigravityModelGroup]PoolRoutingTable`) outside `scheduler.pick` whenever state changes:
      1. Plugin initialization / startup cache load;
      2. Background / manual quota probe completion (`runProductionTask`);
      3. Host synchronization (`SyncHost`);
      4. 429 cooldown trigger or recovery (`triggerCooldown` / `restoreCooldown`);
      5. Config update (`UpdateConfig`).
    - Each `PoolRoutingTable` stores:
      - `DecidedAt time.Time`
      - `NextTransitionAt time.Time`: the earliest upcoming timestamp among all credentials' `ShortWindowResetAt`, `LongWindowResetAt`, and active `CooldownUntil` in that pool.
      - `ByAuthIndex map[string]PrecomputedCandidate`: O(1) lookup map from `AuthIndex` to `{Priority, Urgency, QuotaState, Reason}`.
    - **Lazy Window-Boundary Refresh**:
      - In `scheduler.pick`, if `!poolTable.NextTransitionAt.IsZero() && !now.Before(poolTable.NextTransitionAt)` (e.g., a 5h reset or 429 cooldown just expired), perform an in-memory re-plan (`PlanWithEvidence` from cached memory observations, zero disk/network I/O) and atomically update `plansByPool[pool]`.
      - Otherwise, `scheduler.pick` reads directly from the precomputed `poolTable.ByAuthIndex` in O(1) time.
  - Maintain in-memory routing state on `Runtime`:
    - `plansByPool map[config.AntigravityModelGroup]PoolRoutingTable`
    - `authIDToIndex map[string]string` (maps CPA `Auth.ID`, filename, and normalized file path to `AuthIndex`)
    - `supportedModelsByAuth map[string]map[string]struct{}` (per `AuthIndex` set of canonical model IDs from `host.auth.models`)
    - `modelCatalog []ModelCatalogItem` (merged built-in Antigravity models + live models discovered from `host.auth.models`, each annotated with `ID`, `DisplayName`, `QuotaPool`)
    - `rrCursorByPool map[config.AntigravityModelGroup]string` (last picked `Candidate.ID` per quota pool for deterministic successor Round-Robin)
    - `sessionAffinity map[string]sessionBinding` (`key = string(pool) + "::" + sessionID`, storing `CandidateID` and `ExpiresAt`)
  - **`handleSchedulerPick` O(1) Fast-Path & Mixed-Provider Fallback Algorithm**:
    1. If `!r.cfg.Enabled`, return `SchedulerPickResponse{Handled: false}`.
    2. Partition `req.Candidates` (`candidate.ID != ""`) into:
       - `agCandidates`: `strings.EqualFold(candidate.Provider, "antigravity")`
       - `otherCandidates`: `!strings.EqualFold(candidate.Provider, "antigravity")` (non-Antigravity credentials supporting the same model in `"mixed"` routes).
       If `len(agCandidates) == 0`, return `SchedulerPickResponse{Handled: false}`.
    3. Resolve `pool := config.ClassifyModelQuotaPool(req.Model)` and fetch `poolTable := r.getOrRefreshPoolTable(pool, now)`.
    4. Score each candidate in `agCandidates` via O(1) lookup `poolTable.ByAuthIndex[authIndex]`:
       - Healthy active (`Priority > 0` and not in pool cooldown): tier score = `Priority` (`999`, `100..`, `10..`).
       - Cold-start / unprobed candidate (`!hasEvidence`): tier score = `0`.
       - Soft-depleted ($R_{\text{5h}} \le 0, R_{\text{7d}} > 0$) or in 429 cooldown: tier score = `-1`.
       - Weekly Hard-depleted ($R_{\text{7d}} \le 0$): tier score = `-2`.
    5. Find the highest tier score `bestScore` among `agCandidates`:
       - **Branch A (`bestScore >= 0` — Healthy or Cold-Start Antigravity Available)**:
         - Collect all `agCandidates` with `score == bestScore` (sorted by `ID` ascending).
         - **Session Affinity**: If `sessionID` is present in `req.Options.Metadata["canonical_session_id"]` (or session headers) and bound to a candidate in this winning cluster (`now.Before(binding.ExpiresAt)`), select that candidate and refresh TTL.
         - **Round-Robin**: Otherwise select via `successorIndex(winners, rrCursorByPool[pool])`, update `rrCursorByPool[pool] = picked.ID`, record session binding if `sessionID != ""`, and return `SchedulerPickResponse{AuthID: picked.ID, Handled: true}`.
       - **Branch B (`bestScore < 0` — All Antigravity Credentials Depleted/Cooling for `pool`)**:
         - **Subcase B1 (`len(otherCandidates) > 0` — User has Non-Antigravity Credentials for this Model)**:
           - Because CPA's built-in selector (`selectorAuths` in `conductor_selection.go:1969`) still contains the depleted Antigravity credentials (since their static disk `disabled` is `false`), returning bare `Handled: false` would cause CPA's built-in selector to still route requests to depleted Antigravity credentials!
           - Therefore, when `bestScore < 0` and `len(otherCandidates) > 0`, select from `otherCandidates` (filtering to the highest static `candidate.Priority` tier among `otherCandidates` and rotating via Round-Robin) and return `SchedulerPickResponse{AuthID: pickedOther.ID, Handled: true}` so 100% of traffic cleanly fails over to the user's non-Antigravity credentials!
         - **Subcase B2 (`len(otherCandidates) == 0` — Antigravity-Only Route)**:
           - Return `SchedulerPickResponse{Handled: false}` (or if soft-depleted `-1` candidates exist and no hard-depleted `-2`, fall back toCPA's selector).
### 3. Pool-Scoped 429 Cooldown & Pure In-Memory Scheduling (`internal/runtime/rate_limit.go`, `internal/runtime/production_runner.go`, `internal/state/store.go`)
- Update `modelGroupForUsage(model string)` in `internal/runtime/runtime.go:1091` to return `string(config.ClassifyModelQuotaPool(model))` (`"gemini"`, `"argon"`, or `"claude_gpt"`).
- Key `pending429` and `activeCooldowns` in `Runtime` and `Store.cooldowns` by `cooldownKey(authIndex, modelGroup)` (`authIndex + "::" + modelGroup`), with backward-compatible loading of legacy bare `authIndex` keys from disk.
- In `triggerCooldown` and `restoreCooldown`:
  - Record and clear cooldowns in memory and `state.Store` without mutating host credential JSON files (`PatchPriority`), and immediately refresh the in-memory `plansByPool[modelGroup]` and `latestDualSnapshot` so `scheduler.pick` and the dashboard reflect the cooldown instantaneously.
- In `runProductionTask` (`internal/runtime/production_runner.go`):
  - Execute quota probing, state persistence, model discovery (`ListAuthModels`), and 3-pool planning/projection updates in memory and state cache without writing static `priority` or `disabled` back to CPA host credential files.
  - Keep `ResetAllPriorities` (`POST /reset`) wired to `apply.NewHostTransition(client)` so users can still clear any legacy static priorities from their credential files on demand.


### 4. Real-Time Usage Consumption Hook (`internal/runtime/runtime.go`)
- Expand `usageEvent` in `internal/runtime/runtime.go:1053` to decode `Detail`:
  ```go
  type usageDetail struct {
      InputTokens         int64 `json:"InputTokens"`
      OutputTokens        int64 `json:"OutputTokens"`
      ReasoningTokens     int64 `json:"ReasoningTokens"`
      CachedTokens        int64 `json:"CachedTokens"`
      CacheReadTokens     int64 `json:"CacheReadTokens"`
      CacheCreationTokens int64 `json:"CacheCreationTokens"`
      TotalTokens         int64 `json:"TotalTokens"`
  }
  ```
- In `observeUsageSuccess(ctx, authIndex, modelGroup, detail)`:
  1. Clear any pending 429 count / active cooldown for `(authIndex, modelGroup)`.
  2. Compute `weightedTokens := state.CalculateWeightedTurnTokens(...)` and call `store.RecordUsageConsumption(authIndex, modelGroup, weightedTokens, now)`.
  3. Immediately recompute the in-memory `PoolRoutingTable` (`plansByPool`) and `latestDualSnapshot` so both `scheduler.pick` and the Web UI reflect the real-time quota deduction immediately after every conversation turn!
## Acceptance Criteria

- [ ] `plugin.register` and `plugin.reconfigure` return `"scheduler": true` and `"scheduler_across_priorities": true`.
- [ ] Calling `scheduler.pick` with `model: "gemini-3.8-flash-high"`, `model: "gemini-4-argon-high"`, and `model: "claude-sonnet-5-5-high"` dynamically selects different credentials according to each credential's `gemini`, `argon`, and `claude_gpt` quota plans respectively.
- [ ] Equal-priority credentials in the winning tier rotate via Round-Robin when no session ID is present, and stick to the bound credential when `Options.Metadata["canonical_session_id"]` is present and the bound credential remains in the winning tier.
- [ ] A 429 rate-limit event on `claude-sonnet-5-5-high` places the credential into cooldown for `claude_gpt` only; `scheduler.pick` continues to select that credential for `gemini-3.8-flash-high` and `gemini-4-argon-high`.
- [ ] Non-Antigravity `scheduler.pick` requests return `{"ok":true,"result":{"handled":false}}`.
- [ ] When all Antigravity candidates are depleted/cooling (`bestScore < 0`) in a mixed-provider route (`len(otherCandidates) > 0`), `scheduler.pick` cleanly fails over to the highest-priority non-Antigravity candidate without routing traffic to depleted Antigravity accounts; in an Antigravity-only route, it returns `{"handled": false}`.
- [ ] `scheduler.pick` uses the precomputed in-memory `PoolRoutingTable` in O(1) time without running full planner sorting on every request, while lazily refreshing the table when `now >= NextTransitionAt` (e.g., 5h reset or cooldown expiry).
- [ ] Every successful `usage.handle` event with token details immediately deducts estimated quota for the credential's active pool and updates the in-memory `PoolRoutingTable` and UI snapshot without waiting for the next HTTP probe.
- [ ] `scheduler.pick` performs zero `host.*` C ABI callbacks during execution.
