# 01: Three-Pool Quota Domain, Parser, State & Unified 5-Window Projection

Status: `ready-for-agent`
Type: `task`
Blocked by: none

## What to Build

Expand the domain model, Antigravity quota parser, state store, evidence classifier, and runtime projection from two groups (`gemini`, `claude_gpt`) to **three active quota pools** (`gemini`, `argon`, `claude_gpt`) and attach a unified **5-Window Quota Summary** (`gemini_5h`, `gemini_7d`, `claude_5h`, `claude_7d`, `argon_7d`) to every projected credential item.

## Exact File Boundaries & Changes

### 1. `internal/config/antigravity_model_group.go` & `internal/config/config.go`
- Add `AntigravityModelGroupArgon AntigravityModelGroup = "argon"` alongside `AntigravityModelGroupGemini = "gemini"` and `AntigravityModelGroupClaudeGPT = "claude_gpt"`.
- Define `AllAntigravityModelGroups = []AntigravityModelGroup{AntigravityModelGroupGemini, AntigravityModelGroupArgon, AntigravityModelGroupClaudeGPT}`.
- Update `ParseAntigravityModelGroup` to accept `"gemini"`, `"argon"`, and `"claude_gpt"` (plus aliases like `"claude"`, `"gpt"` -> `"claude_gpt"`).
- Add `ClassifyModelQuotaPool(model string) AntigravityModelGroup`:
  - Strip leading `<prefix>/` (e.g. `teamA/gemini-4-argon-high` -> `gemini-4-argon-high`) and trailing thinking suffix `(...)` (e.g. `claude-sonnet-5-5-high(16384)` -> `claude-sonnet-5-5-high`).
  - Lowercase and match:
    1. Contains `"argon"` -> `AntigravityModelGroupArgon`
    2. Contains `"claude"`, `"opus"`, `"sonnet"`, `"haiku"`, `"fable"`, `"gpt"`, `"o1"`, `"o3"`, `"o4"`, `"openai"` -> `AntigravityModelGroupClaudeGPT`
    3. Default -> `AntigravityModelGroupGemini`

### 2. `internal/provider/antigravity/types.go` & `internal/provider/antigravity/parse.go`
- Add `ModelGroupArgon = config.AntigravityModelGroupArgon`.
- Update `ParseAllModelGroups(raw []byte, observedAt time.Time) map[ModelGroup]ProbeResult` to return all 3 keys: `ModelGroupGemini`, `ModelGroupArgon`, and `ModelGroupClaudeGPT`.
- In `parseGroupFromResponse(response, observedAt, group)`:
  - When `group == ModelGroupArgon`:
    - Collect windows from `Gemini Models` and `Gemini 4 Argon` groups (and Argon/Gemini models/buckets).
    - `ShortWindowRemaining` / `ShortWindowResetAt`: use the conservative 5h window from `Gemini Models` (since Antigravity's 5h burst window is shared across Gemini/Argon).
    - `LongWindowRemaining` / `LongWindowResetAt`: prefer the conservative `argonWindows` weekly/any window (`isArgon == true`). If no dedicated Argon bucket is present in a valid Gemini response (e.g. Ultra or Free plan where Argon shares the main Gemini weekly bucket), fall back to `normalWindows` weekly window.
    - Populate `ArgonWindowRemaining` and `ArgonWindowResetAt` with the resolved Argon weekly values.
    - Set `PlanType` consistently from the Gemini/Argon windows.

### 3. `internal/state/store.go` & `internal/runtime/probe_evidence.go`
- In `collectFreshEvidence` (`internal/runtime/probe_evidence.go`):
  - Loop over all 3 groups in `config.AllAntigravityModelGroups` (`gemini`, `argon`, `claude_gpt`) when classifying `evidence.Classify` and enriching `CycleBurnRate`.
  - When `runProbeJobs` executes a single HTTP probe via `ParseAllModelGroups`, emit `evidence.ProbeObservation` entries for all 3 groups (`gemini`, `argon`, `claude_gpt`) and persist all 3 to `state.Store` via `RecordProbeSuccess` / `RecordProbeFailure`.
- Ensure `store.LearnCycleBurnRate` works identically for `argon` (comparing $\Delta R_{\text{5h}} \ge 5\%$ and $\Delta R_{\text{7d}} > 0$ on the `argon` entry's `LongWindowRemaining`).

### 4. `internal/apply/audit.go`, `internal/apply/apply.go` & `internal/runtime/projection.go`
- Add a unified `QuotaWindows` struct on `apply.SnapshotItem` (or top-level fields on `SnapshotItem`) so every item in any pool snapshot carries all 5 quota bars for that credential:
  ```go
  type QuotaWindowBar struct {
      Remaining *int64     `json:"remaining,omitempty"`
      ResetAt   *time.Time `json:"reset_at,omitempty"`
      Stale     bool       `json:"stale,omitempty"`
  }
  type AllQuotaWindows struct {
      Gemini5h QuotaWindowBar `json:"gemini_5h"`
      Gemini7d QuotaWindowBar `json:"gemini_7d"`
      Claude5h QuotaWindowBar `json:"claude_5h"`
      Claude7d QuotaWindowBar `json:"claude_7d"`
      Argon7d  QuotaWindowBar `json:"argon_7d"`
  }
  ```
- Update `ProjectDualModelGroups` (and its types in `internal/runtime/projection.go` / `internal/apply/apply.go`):
  - Compute active `priority.Plan` and `apply.PlanSnapshot` for all 3 groups (`gemini`, `argon`, `claude_gpt`).
  - Mark all 3 groups as active (`IsPredicted = false`, no `"predicted: "` prefix on reasons).
  - Populate `QuotaWindows` (`AllQuotaWindows`) on every `SnapshotItem` across all 3 group snapshots by cross-referencing the credential's `gemini`, `claude_gpt`, and `argon` `PlanItem` observations.


### 5. `internal/priority/urgency.go`: Elapsed Reset-Time Auto-Recovery in Pure Memory Mode
- Because pure in-memory dynamic scheduling does not wait for a host file rewrite to re-enable a soft-depleted credential when its 5-hour window resets between 15-minute probes:
  - In `extractR5h(evidence QuotaEvidence, fallbackR7d float64, now time.Time)` and `isShortDepletedEvidence(evidence QuotaEvidence, r5h float64, now time.Time)`:
    - If `evidence.ShortWindowRemaining != nil && *evidence.ShortWindowRemaining <= 0`, check `evidence.ShortWindowResetAt`: if `evidence.ShortWindowResetAt != nil && !evidence.ShortWindowResetAt.IsZero() && !evidence.ShortWindowResetAt.After(now)` (i.e. `now >= ShortWindowResetAt`, the 5h reset window has already passed), treat the 5h window as reset (`R5h = 1.0`, `isShortDepleted = false`) until the next probe observes the new remaining percentage!
  - Similarly, in `extractR7d(evidence QuotaEvidence, now time.Time)` and `isWeeklyDepletedEvidence(evidence QuotaEvidence, r7d float64, now time.Time)`:
    - If `evidence.LongWindowRemaining != nil && *evidence.LongWindowRemaining <= 0` and `evidence.LongWindowResetAt != nil && !evidence.LongWindowResetAt.IsZero() && !evidence.LongWindowResetAt.After(now)`, treat the 7d window as reset (`R7d = 1.0`, `isWeeklyDepleted = false`) so weekly-depleted credentials automatically recover at the exact weekly reset timestamp.

### 6. `internal/state/estimator.go` & `internal/state/store.go`: Real-Time Usage Deduction & Probe Calibration
- Extend `state.Entry` (`internal/state/store.go:35`) with real-time estimation fields:
  - `PendingWeightedTokens float64` (`json:"pending_weighted_tokens,omitempty"`): weighted tokens consumed since `ObservedAt`.
  - `EstimatedShortBurnPct float64` (`json:"estimated_short_burn_pct,omitempty"`): estimated 5h percentage points consumed since `ObservedAt`.
  - `EstimatedLongBurnPct float64` (`json:"estimated_long_burn_pct,omitempty"`): estimated 7d percentage points consumed since `ObservedAt`.
  - `TokensPerShortPct float64` (`json:"tokens_per_short_pct,omitempty"`): learned weighted tokens per 1% of 5h window consumption ($K_{\text{5h}}$).
  - `LastUsageAt time.Time` (`json:"last_usage_at,omitempty"`).
- In `internal/state/estimator.go`:
  - Add `CalculateWeightedTurnTokens(inputTokens, outputTokens, reasoningTokens, cacheReadTokens, totalTokens int64) float64`:
    - `uncachedInput = max(0, inputTokens - cacheReadTokens)`
    - `weighted = float64(uncachedInput) + 0.1*float64(cacheReadTokens) + 4.0*float64(outputTokens + reasoningTokens)`
    - If `weighted <= 0`, fall back to `max(float64(totalTokens), 2000.0)`.
  - Add `RecordUsageConsumption(authIndex, modelGroup string, weightedTokens float64, now time.Time)` on `Store`:
    - Accumulate `PendingWeightedTokens` on the target `modelGroup`.
    - Because `gemini` and `argon` share the 5h short window, when `modelGroup == "gemini"`, also update `EstimatedShortBurnPct` on `"argon"` (and vice versa).
    - Compute `EstimatedShortBurnPct = PendingWeightedTokens / effectiveTokensPerShortPct(entry)` and `EstimatedLongBurnPct = EstimatedShortBurnPct * effectiveCycleBurnRate(entry)`.
    - When constructing `QuotaEvidence` (in `GetHistoricalEvidence` and live runtime projection), subtract `math.Round(EstimatedShortBurnPct)` from `ShortWindowRemaining` (clamped to $\ge 0$) and `math.Round(EstimatedLongBurnPct)` from `LongWindowRemaining` (clamped to $\ge 0$), and expose `EstimatedShortBurnPct` / `EstimatedLongBurnPct` on `SnapshotItem` so the UI can display real-time estimated consumption (e.g. `78% (预估 -2%)`).
  - In `MarkProbeSuccess`:
    - Before resetting `PendingWeightedTokens`, if the previous `ObservedAt` was in the same `ShortWindowResetAt` cycle and actual 5h drop $\Delta R_{\text{5h,actual}} = R_{\text{5h,prev}} - R_{\text{5h,curr}} \ge 2$ percentage points while `PendingWeightedTokens > 0`:
      - Update `TokensPerShortPct` via EMA ($\alpha = 0.3$): $K_{\text{new}} = 0.3 \times \frac{\text{PendingWeightedTokens}}{\Delta R_{\text{5h,actual}}} + 0.7 \times K_{\text{prev}}$ (clamped to `[5000, 500000]`).
    - Reset `PendingWeightedTokens = 0`, `EstimatedShortBurnPct = 0`, `EstimatedLongBurnPct = 0` so the probe calibrates the state to exact server truth.
## Acceptance Criteria

- [ ] `ClassifyModelQuotaPool` accurately classifies `gemini-3.8-flash-high` -> `gemini`, `gemini-4-argon-high` -> `argon`, `claude-sonnet-5-5-high` -> `claude_gpt`, `gpt-oss-120b-medium` -> `claude_gpt`, including prefixed (`my-prefix/gemini-4-argon-high`) and thinking-suffixed (`claude-opus-5-5-high(32768)`) model strings.
- [ ] `ParseAllModelGroups` returns `gemini`, `argon`, and `claude_gpt` `ProbeResult`s from a single HTTP response decode pass.
- [ ] Probing an account with separate `Gemini Models` (5h=99%, 7d=79%), `Gemini 4 Argon` (7d=71%), and `Claude and GPT Models` (5h=80%, 7d=45%) produces 3 independent `priority.Plan`s where `argon` uses `R5h=0.99, R7d=0.71` and `gemini` uses `R5h=0.99, R7d=0.79`.
- [ ] Every `SnapshotItem` in `Snapshot.Groups["gemini"]`, `Snapshot.Groups["argon"]`, and `Snapshot.Groups["claude_gpt"]` contains populated `QuotaWindows` for all 5 bars (`gemini_5h`, `gemini_7d`, `claude_5h`, `claude_7d`, `argon_7d`).
- [ ] When a credential has `ShortWindowRemaining = 0` with `ShortWindowResetAt` in the past (`now >= ShortWindowResetAt`), `ExtractQuotaMetrics(evidence, now)` automatically clears `IsShortDepleted` (`R5h = 1.0`) without waiting for the next HTTP probe.
- [ ] Recording token usage via `RecordUsageConsumption` immediately deducts estimated 5h and 7d remaining quota in memory, and the subsequent `MarkProbeSuccess` calibrates `TokensPerShortPct` via EMA and resets pending estimated burn to zero.
- [ ] Unit tests in `internal/config`, `internal/provider/antigravity`, `internal/state`, and `internal/runtime` pass.
