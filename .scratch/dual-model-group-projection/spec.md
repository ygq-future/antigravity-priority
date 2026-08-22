# Spec: Dual Model Group Projection

Status: `completed`

## Problem Statement

CPA administrators need every management action to present the same truthful relationship between Antigravity's two independent Model Groups. The configured Control Model Group is allowed to drive quota-based Host changes, while the other group is a Predicted Model Group that shows what priorities it would receive if it became the control authority.

Today, Production, Probe, SyncHost, Reset, and startup fallback each assemble this relationship separately. Every caller must choose the alternate group, run Planner twice, mark one result as predicted, construct the dual-group response, and decide what empty or cached data means. This duplication can make the dashboard change shape or role semantics depending on the most recent action, and it forces projection tests through large Runtime setups.

## Solution

Deepen the repeated in-process logic into one pure Dual Model Group Projection module. Its interface consumes the configured Control Model Group, current Host inventory, evidence already classified by the Fresh Evidence authority, shared planning inputs, and an explicit projection time. It returns the control Plan needed by the calling use case and one complete dual-group Snapshot in which the control group is Target and the other group is Predicted.

Production, Probe, SyncHost, Reset, and fallback use the same projection interface. The module performs no network requests, persistence, or Host writes; Runtime retains use-case ownership and only Manual Apply or Auto Apply may send the returned control Plan to the Host Transition flow. Missing or failed group data remains unknown and is never filled from the other group.

## User Stories

1. As a CPA administrator, I want the configured Model Group to remain the sole control authority, so that dashboard navigation cannot change Host write-back behaviour.
2. As a CPA administrator, I want the other Model Group clearly marked as Predicted, so that hypothetical priorities cannot be mistaken for applied targets.
3. As a CPA administrator, I want switching the dashboard view to be read-only, so that inspecting Claude/GPT or Gemini cannot trigger a write or network request.
4. As a CPA administrator, I want one quota refresh to update both Model Groups, so that I do not need duplicate Google requests for each view.
5. As a CPA administrator, I want each Model Group to retain its own quota observations and Plan, so that one group's data is never displayed under the other's label.
6. As a CPA administrator, I want a missing or failed group displayed as unknown, so that the dashboard does not invent a prediction from the other group.
7. As a CPA administrator, I want the Predicted Model Group to show what its priorities would be under the same Host inventory and scheduling configuration, so that the comparison is meaningful.
8. As a CPA administrator, I want active cooldowns reflected consistently in both projections, so that a credential-level operational restriction is not hidden in one view.
9. As a CPA administrator, I want Production, Probe, SyncHost, Reset, and initial empty state to return the same dual-group structure, so that cards do not disappear or change roles after an action.
10. As a CPA administrator, I want Probe to update both projections without writing Host, so that refreshing quota remains observational.
11. As a CPA administrator, I want SyncHost to recompute both projections from current Host state and available read-only evidence, so that manual Host changes appear consistently.
12. As a CPA administrator, I want Reset to preserve a truthful read-only projection after removing priorities, so that the dashboard does not confuse reset state with an applied Plan.
13. As a CPA administrator, I want Manual Apply and Auto Apply to write only the Control Model Group Plan, so that Predicted changes can never reach Host.
14. As a CPA administrator, I want changing the configured Control Model Group not to write Host immediately, so that configuration changes remain non-destructive until a fresh Apply round.
15. As a CPA administrator, I want the next successful Manual or Auto Apply to use the newly configured Control Model Group, so that the configuration takes effect predictably.
16. As a CPA administrator, I want historical observations usable for read-only prediction but never promoted to write authority, so that the dashboard remains useful without weakening Fresh Evidence.
17. As an integration client, I want the existing dual-group response contract preserved, so that this architecture change does not require a Management API migration.
18. As a maintainer, I want one projection interface shared by all Runtime use cases, so that group selection, role marking, and empty-state rules have one implementation and one test surface.
19. As a maintainer, I want projection to remain pure, so that its behaviour can be verified without Host callbacks, Google requests, Store files, or Runtime worker setup.
20. As a maintainer, I want every caller to use the shared alternate-group and Predicted-role projection, so that future changes remain consistent across execution paths.

## Implementation Decisions

1. **Primary module and seam**
   - Build one pure in-process Dual Model Group Projection module owned by Runtime.
   - Its interface accepts all values required for deterministic projection rather than reading configuration, clocks, Store state, or Host state itself.
   - It returns the Control Model Group Plan plus one complete dual-group Snapshot.
   - The Predicted Model Group Plan remains internal to the module and is exposed only as a predicted Snapshot, so Apply callers cannot accidentally write it.
   - Do not introduce a port or adapter because projection has no external dependency and only one real implementation.

2. **Control authority**
   - Dynamic Config is the only source of the Control Model Group.
   - Runtime query parameters and dashboard view state are not projection authority.
   - The Control Model Group Plan is the only Plan eligible to reach quota-driven Host write-back, and only when the calling use case permits Apply and Fresh Evidence requirements are satisfied.
   - The Predicted Model Group never supplies Changes to the Host Transition flow.

3. **Projection calculation**
   - Calculate both Model Groups from the same reconciled Host inventory, projection time, scheduling configuration, and active credential cooldowns.
   - Use each group's own classified evidence and learned values; never copy evidence, metrics, reasons, Plan type, or targets across groups.
   - Delegate all priority calculation to the existing pure Planner.
   - Consume Fresh Evidence and historical observation semantics from the Evidence authority rather than reconstructing freshness inside projection.

4. **Role marking and response shape**
   - The Snapshot always identifies the configured Control Model Group and contains entries for both canonical Model Groups.
   - Control items and changes are Target, never Predicted.
   - Non-control items and changes are Predicted throughout the complete group projection.
   - Preserve the existing public response structure and group keys; internal type contraction must not require a Management API migration.

5. **Missing, failed, and empty data**
   - Missing or failed evidence for one group remains unknown for that group and cannot be replaced with the other group's data.
   - A group with no projectable observations still appears in the response using a stable empty or unknown representation.
   - Startup fallback must use the same two-group shape and must not relabel a legacy single-group result as another Model Group.
   - Do not add fallback quota values or synthetic Fresh Evidence.

6. **Time semantics**
   - Use one explicit projection-generation time across both group Plans and the dual-group Snapshot.
   - Preserve source observation times as evidence truth and do not rewrite them to the projection time.
   - Keep the existing public timestamp field compatible while making its generation semantics consistent across Production, Probe, SyncHost, Reset, and fallback.

7. **Runtime use-case integration**
   - Production builds one projection after post-probe Host reconciliation and applies only the returned control Plan for Manual Apply or Auto Apply.
   - Probe stores observations and updates the shared projection without Host mutation.
   - SyncHost rereads Host and updates the shared projection without Google requests or Host mutation.
   - Reset performs its independently authorized Host operation, then rebuilds a read-only projection from the resulting Host inventory.
   - LatestSnapshot returns the most recent shared projection or the shared stable fallback; it does not implement a second projection algorithm.

8. **Control-group configuration changes**
   - Saving a new Control Model Group persists and hot-applies configuration but does not trigger a Host write.
   - A subsequent synchronized projection reflects the newly configured control and predicted roles without swapping or relabeling stale group contents.
   - The next Manual Apply or Auto Apply may write the new control Plan only after obtaining Fresh Evidence.
   - Preserve Management route compatibility while treating accepted Model Group query parameters as view input and Dynamic Config as control authority.

9. **Relationship to adjacent architecture work**
   - The Evidence authority determines which observations are current, historical, failed, or write-qualified.
   - Planner determines priorities and immutable Plans.
   - Dual Model Group Projection determines group roles and produces the shared control Plan plus Snapshot.
   - Host Transition commits and records an authorized control Plan.
   - Runtime continues to coordinate use-case order and single-flight execution.

10. **Migration and contraction**
    - Migrate Production, Probe, SyncHost, Reset, and fallback to the shared projection interface.
    - Remove duplicate alternate-group selection, Planner invocation, Predicted marking, and dual-map assembly from callers once migrated.
    - Internalize or remove parameters that appear to select the control group while preserving external compatibility where required.
    - Replace duplicated caller tests with projection contract tests plus thin Runtime integration coverage.

## Testing Decisions

1. **Primary test seam**
   - Test through the pure Dual Model Group Projection interface with deterministic Host credentials, classified evidence for both groups, configuration, cooldowns, and projection time.
   - Observe the returned control Plan and dual-group Snapshot; do not assert on private helpers, map construction order, reason-prefix loops, or caller-specific assembly.

2. **Control and predicted roles**
   - Verify both control directions: Gemini control with Claude/GPT predicted, and Claude/GPT control with Gemini predicted.
   - Verify the returned control Plan always belongs to the configured Control Model Group.
   - Verify every non-control item and change is Predicted and cannot be obtained as an Apply Plan.
   - Verify dashboard or request view parameters cannot change projection authority.

3. **Independent group data**
   - Verify each group retains its own evidence, quota windows, learned burn rate, Plan type, urgency, reason, and target priority.
   - Verify missing or failed data in one group does not copy, swap, or relabel the other group's data.
   - Verify a single physical probe result can populate both group projections while retaining independent classifications.

4. **Shared planning inputs**
   - Verify both groups use the same post-reconciliation Host inventory, projection time, scheduling configuration, and active cooldowns.
   - Verify existing minimum-change, priority-start, clustering, boost, Soft Depletion, and Hard Depletion behaviour remains delegated to Planner.
   - Verify projection does not mutate credentials, evidence, options, Plans, or returned Snapshots across repeated calls.

5. **Stable response and fallback**
   - Verify every result contains both canonical group keys and the configured control identifier.
   - Verify groups with no data use the stable empty or unknown representation.
   - Verify fallback has the same shape and role semantics as normal projection and never relabels legacy group contents.
   - Verify the public response remains backward compatible.

6. **Time behaviour**
   - Verify one explicit projection-generation time is used consistently for both Plans and the top-level Snapshot.
   - Verify source observation timestamps remain unchanged.
   - Verify repeated reads of the same stored projection do not silently replace its generation time with query time.

7. **Runtime integration**
   - Verify Production applies only the returned control Plan after post-probe Host reconciliation.
   - Verify Probe updates a complete dual Snapshot and performs no Host writes.
   - Verify SyncHost updates a complete dual Snapshot without Google requests or Host writes.
   - Verify Reset rebuilds a complete read-only Snapshot from post-reset Host state.
   - Verify LatestSnapshot uses the shared result or shared fallback rather than independent assembly.
   - Verify saving a new Control Model Group performs no Host write and the next synchronized projection assigns roles correctly.

8. **Management presentation contract**
   - Verify view switching preserves the configured control identifier and Target/Predicted roles.
   - Verify Refresh recomputes both groups while preserving the user's selected display group.
   - Verify Apply remains unavailable from the Predicted Model Group view.
   - Keep response-handler tests focused on transport while the shared projection seam covers projection rules.

9. **Replace, do not layer**
   - Replace tests tied to separate Production, SyncHost, Reset, or fallback assembly after equivalent behaviour is covered at the projection seam.
   - Retain Runtime tests for use-case ordering, network and Host side effects, single-flight ownership, and connection to projection.
   - Prefer one table-driven projection contract suite over repeated role and shape assertions across callers.

10. **Quality gates**
    - Require repository formatting and static analysis.
    - Require `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` to pass.
    - Use explicit clocks and immutable inputs rather than timing-dependent sleeps.

## Out of Scope

- Supporting a third Model Group or an arbitrary number of groups.
- A provider-neutral or generic projection framework.
- Automatic control-group failover, takeover, rotation, or fallback based on quota or probe health.
- New projection, switching, or write-authority configuration.
- Network probing, State Store persistence, Host writes, or execution recording inside the projection module.
- New databases, cache layers, background refresh workers, queues, or recovery mechanisms.
- Copying one Model Group's evidence or Plan into the other group when data is missing.
- Allowing the Predicted Model Group to produce Host writes.
- Immediate Host write-back when Control Model Group configuration changes.
- Changing Fresh Evidence, Planner algorithms, Host Transition outcomes, cooldown policy, or reset behaviour.
- Redesigning management-page assets, run history, or the public Snapshot response schema.
- Removing externally accepted query parameters when compatibility requires retaining them.
- Implementing the other second-round architecture proposals.
- Expanding beyond Google Antigravity.
- Committing, releasing, or changing version numbers as part of this specification.

## Further Notes

- This specification implements the accepted third proposal without introducing a new ADR because it consolidates existing behaviour rather than selecting a new architectural trade-off.
- The canonical domain terms are Control Model Group and Predicted Model Group; “active,” “alternate,” and “selected” should not be used interchangeably.
- The projection module is intentionally pure and in-process, with one primary test seam and no adapter.
- Fresh Evidence authority should be implemented before or alongside projection migration so this module consumes classified evidence rather than reproducing freshness rules.
- Host Transition work remains independently responsible for committing and truthfully recording the returned control Plan.
- The work should be split into a small number of blocker-aware tracer-bullet tickets before implementation.
