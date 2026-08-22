# Spec: Fresh Evidence Authority

Status: `completed`

## Problem Statement

CPA administrators need quota-driven scheduling to change credential priority or disabled state only when the plugin has verified quota data from the current scheduling round. Today, successful cache entries are reconstructed as if they were fresh, failed probes can be accepted as scheduling evidence, and callers must combine several flags with a timestamp comparison to decide whether Host write-back is allowed.

This creates a dangerous mismatch between the product rule and the implementation: a transient network, upstream, parsing, or authentication failure can be interpreted as a reason to disable a credential, while historical data can appear eligible for current scheduling. Users need probe failure to mean “quota state unknown,” not “quota depleted,” and they need cached observations to remain useful without ever authorizing a quota-driven Host Transition.

## Solution

Deepen the existing in-process evidence collection and selection logic into one Evidence module. Its small interface returns two semantically separate results for a scheduling round: verified current-round evidence eligible for quota planning, and read-only observations used for diagnostics or prediction. The module owns provenance, round membership, probe success, failure, and cache interpretation so Runtime and Planner callers do not reconstruct Fresh Evidence from independent flags.

Only a successful, validated probe from the current round enters quota planning. A failed probe leaves that credential’s Host state unchanged and is reported as a diagnostic observation; other successfully probed credentials continue through planning. Historical successful observations may support display and alternate Model Group prediction, but never Host write-back. Existing 429 Reactive Cooldown and explicit priority reset retain their separate operational authority and are not represented as Fresh Evidence.

## User Stories

1. As a CPA administrator, I want quota-driven Host changes to require a successful probe from the current scheduling round, so that scheduling decisions use verified current quota.
2. As a CPA administrator, I want a failed probe to preserve the credential’s current Host state, so that temporary infrastructure failures do not disable healthy credentials.
3. As a CPA administrator, I want probe failures reported clearly, so that I can distinguish an unknown quota state from real quota depletion.
4. As a CPA administrator, I want one credential’s probe failure not to block other successfully probed credentials, so that a local failure does not halt fleet-wide scheduling.
5. As a CPA administrator, I want a previous successful observation retained for display when the current probe fails, so that useful historical context remains visible.
6. As a CPA administrator, I want historical observations identified as historical rather than fresh, so that displayed data is not mistaken for a current measurement.
7. As a CPA administrator, I want a credential with no successful observation displayed as unknown after probe failure, so that the UI does not invent quota values.
8. As a CPA administrator, I want cached observations excluded from Host write-back, so that stale quota cannot raise, lower, enable, or disable a credential.
9. As a CPA administrator, I want real Soft Depletion and Hard Depletion to retain their existing behaviour when supported by Fresh Evidence, so that quota protection remains unchanged.
10. As a CPA administrator, I want Manual Apply to use a new probe rather than cached quota, so that an explicit apply reflects current conditions.
11. As a CPA administrator, I want Auto Apply to retain its existing probe retry behaviour and preserve Host state after final failure, so that retry policy does not silently become disable policy.
12. As a CPA administrator, I want Probe to remain non-writing, so that observation and diagnostics cannot change credentials.
13. As a CPA administrator, I want SyncHost to use cached observations only for read-only projection, so that refreshing the dashboard cannot authorize a Host change.
14. As a CPA administrator, I want priority reset to remain an explicit command independent of quota probes, so that reset does not require Fresh Evidence.
15. As a CPA administrator, I want 429 Reactive Cooldown to remain authorized by the observed 429 event, so that immediate non-destructive demotion still works without pretending to be quota evidence.
16. As a CPA administrator, I want the active and alternate Model Groups to preserve their current write and prediction roles, so that only the configured group controls quota-driven Host changes.
17. As a maintainer, I want Fresh Evidence defined by one module interface, so that new scheduling callers cannot accidentally promote cached or failed observations.
18. As a maintainer, I want contradictory freshness flags and caller-owned timestamp filters removed or internalized, so that the domain rule has one implementation and one primary test surface.

## Implementation Decisions

1. **Primary module and seam**
   - Deepen the existing evidence collection and selection implementation into one in-process Evidence module.
   - The module interface accepts the scheduling-round context, credential inventory, probe outcomes, persisted observations, and Model Group needed to classify evidence.
   - It returns a cohesive round result containing scheduling-eligible evidence and read-only observations; callers do not select evidence by inspecting independent freshness fields.
   - Do not introduce a port or adapter because this is in-process domain computation with one real implementation.

2. **Fresh Evidence invariant**
   - Fresh Evidence is a verified quota observation from a successful probe in the current scheduling round.
   - Round membership, provider readiness, required quota values, and successful validation are enforced inside the Evidence module.
   - Missing, failed, historical, malformed, or otherwise unverified observations are never scheduling-eligible.
   - Callers do not reproduce round membership with timestamp equality or construct arbitrary freshness combinations.

3. **Planner contract**
   - Planner remains a pure function and receives only scheduling-eligible quota evidence through its planning input.
   - A credential without Fresh Evidence remains in the Plan with its current Host state and cannot produce a quota-driven Change.
   - Successful credentials continue through the existing urgency, boost, clustering, cooldown, and depletion calculations.
   - Probe failure is not converted into Hard Depletion, disabled state, or a forced write.

4. **Probe failure semantics**
   - A current-round probe failure produces a diagnostic observation with no quota-driven Host authority.
   - The failed credential preserves its current priority and disabled state.
   - Other credentials with Fresh Evidence continue independently in the same round.
   - Existing Manual Apply and Auto Apply probe-attempt behaviour is preserved; no additional retry, backoff, or recovery flow is introduced.

5. **Historical observation semantics**
   - Persisted successful observations remain available for display, trends, burn-rate learning, and read-only prediction.
   - Historical observations retain their actual observation time and cannot be labeled or represented as Fresh Evidence.
   - A current failure may be displayed alongside the last successful historical observation, but the two facts remain distinct.
   - A credential with no successful historical observation is represented as unknown rather than assigned fallback quota.

6. **Use-case behaviour**
   - Manual Apply and Auto Apply request current-round probe evidence before quota planning and Host write-back.
   - Probe collects observations and updates read-only snapshots without producing Host transitions.
   - SyncHost and post-reset snapshot reconstruction may use historical observations only for read-only projection.
   - Priority reset is authorized by the explicit operator command, not by quota evidence.
   - 429 Reactive Cooldown is authorized by the observed runtime event, not by quota evidence.

7. **Dual Model Group behaviour**
   - Evidence is classified independently for each Model Group returned by the Antigravity probe.
   - The configured active Model Group retains authority over quota-driven Host planning.
   - The alternate Model Group remains a read-only predicted projection.
   - Historical alternate-group data cannot become write-qualified through projection or snapshot construction.

8. **Representation contraction**
   - Replace or internalize overlapping freshness, probe-status, evidence-status, and caller-owned round checks so they cannot encode contradictory public states.
   - Keep only the information needed to express scheduling eligibility and diagnostic observation truth at the module interface.
   - Exact internal type and field names are implementation details and must not expand the interface.

9. **Persistence and external contracts**
   - Reuse the existing state cache, quota samples, Runtime history, and management projection.
   - No new persistence file, schema migration, database, queue, or configuration option is required.
   - Preserve existing scheduling configuration and public management actions.
   - Diagnostics must distinguish current failure, historical observation, and current Fresh Evidence using the existing response vocabulary where possible.

10. **Relationship to Host Transition work**
    - This specification decides whether quota evidence may authorize a planned Host change; the Host Transition module decides how an authorized change is committed and recorded.
    - 429 Cooldown and reset remain explicit operational intents and must not be modeled as synthetic Fresh Evidence.
    - Implementation may proceed independently where possible, while integration must preserve the Host Transition outcomes and authority model already specified.

11. **Migration and contraction**
    - Route production planning, probe snapshots, SyncHost projections, reset projections, and dual-group projection through the same evidence classification semantics.
    - Remove the Planner behaviour that treats probe failure as fresh and disables the credential.
    - Remove or internalize Runtime-only current-round filtering once the Evidence module enforces the invariant.
    - Change cached-evidence construction so historical observations cannot claim scheduling eligibility.
    - Replace tests that intentionally assemble contradictory freshness combinations after equivalent behaviour is covered at the Evidence module seam.

## Testing Decisions

1. **Primary test seam**
   - Test through the Evidence module’s round-classification interface using deterministic credential inventories, probe outcomes, persisted observations, Model Groups, and round identities.
   - Assert on scheduling-eligible evidence and read-only observations, not private helpers, field-by-field flag assembly, cache implementation details, or timestamp comparison mechanics.

2. **Fresh Evidence classification**
   - Verify a successful, validated current-round observation is scheduling-eligible.
   - Verify a successful observation from an older round is read-only.
   - Verify current and historical probe failures are never scheduling-eligible.
   - Verify malformed, incomplete, unknown-provider, or wrong-group observations are never scheduling-eligible.
   - Verify the same physical probe response is classified independently for both Model Groups.

3. **Failure and cache behaviour**
   - Verify current probe failure produces diagnostics while preserving any last successful historical observation separately.
   - Verify a never-successful credential remains unknown after failure.
   - Verify cached observations retain their real observation time and never acquire write authority during load or projection.
   - Verify no failure observation is converted into disabled state or quota depletion.

4. **Planner behaviour through its public interface**
   - Verify a credential with Fresh Evidence can produce the existing quota-driven target and Change.
   - Verify a credential without Fresh Evidence keeps its current priority and disabled state and produces no quota-driven Change.
   - Verify a mixed round continues planning successful credentials while failed credentials remain unchanged.
   - Retain behavioural coverage for Soft Depletion, Hard Depletion, Weekly Urgency, Dynamic Boost Horizon, Equal Priority Clustering, minimum change, and active cooldowns.

5. **Runtime use-case integration**
   - Verify Manual Apply uses current probe results and leaves a failed credential unchanged.
   - Verify Auto Apply retains its existing attempt count and leaves a credential unchanged after final failure.
   - Verify a failed credential does not block successfully probed credentials in Manual or Auto Apply.
   - Verify Probe remains non-writing and reports failure without a false target-disabled state.
   - Verify SyncHost and post-reset projection can display historical observations without creating write-qualified Changes.
   - Verify post-probe Host reconciliation remains authoritative for additions, removals, disabled state, and external priority changes.

6. **Operational authority integration**
   - Verify 429 Reactive Cooldown still demotes priority while preserving disabled state without Fresh Evidence.
   - Verify explicit priority reset still removes priority while preserving disabled state without Fresh Evidence.
   - Verify neither path exposes synthetic Fresh Evidence in snapshots or diagnostics.

7. **Dual Model Group projection**
   - Verify only the configured active Model Group can contribute to quota-driven Host changes.
   - Verify the alternate Model Group remains predicted and read-only even when it has a current successful observation.
   - Verify historical or failed observations in either group cannot become write-qualified through snapshot construction.

8. **Replace, do not layer**
   - Replace tests that assert cached observations are fresh, failed probes imply disabled, or callers must build multiple matching freshness flags.
   - Retain state persistence tests for observation truth, Planner tests for scheduling outcomes, and Runtime tests for use-case ownership.
   - Prefer one table-driven contract suite at the Evidence module seam over duplicate rule tests in Store, Runtime, and Planner.

9. **Quality gates**
   - Require repository formatting and static analysis.
   - Require `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` to pass.
   - Use deterministic clocks and explicit round identities rather than timing-dependent sleeps.

## Out of Scope

- Error taxonomies that classify probe failures as permanent, temporary, authentication, transport, parsing, or retryable policy categories.
- New Fresh Evidence TTL, failure-policy, retry-count, backoff, or recovery configuration.
- Additional Auto Apply retries or changes to existing retry timing.
- A generic evidence framework, provider-neutral port, cross-provider abstraction, or additional adapter.
- A new database, cache file, queue, event log, recovery worker, replay mechanism, or schema migration.
- Using cached quota to authorize Host write-back under any condition.
- Disabling credentials solely because quota probing failed.
- Changing 429 Reactive Cooldown, explicit reset, Soft Depletion, Hard Depletion, Weekly Urgency, Dynamic Boost Horizon, or Equal Priority Clustering business rules.
- Changing which Model Group is active or allowing alternate-group prediction to write Host.
- Implementing the Host Transition specification or the remaining second-round architecture proposals.
- Reorganizing management-page assets.
- Expanding beyond Google Antigravity.
- Committing, releasing, or changing version numbers as part of this specification.

## Further Notes

- This specification implements ADR-0006 and the canonical Fresh Evidence definition in the domain glossary.
- ADR-0003 continues to define real Soft Depletion and Hard Depletion; probe failure is neither condition.
- ADR-0005 remains responsible for committing and truthfully recording an authorized Host Transition.
- The Evidence module is intentionally in-process and has one primary test seam; a port or adapter would add indirection without real variation.
- The design deliberately contracts contradictory evidence states instead of adding more policy or recovery machinery.
- The work should be split into a small number of blocker-aware tracer-bullet tickets before implementation.
