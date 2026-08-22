# Spec: Atomic Host Transition and Truthful Outcomes

Status: `ready-for-agent`

## Problem Statement

CPA administrators need scheduled Apply, 429 Reactive Cooldown, and priority reset to change credential state predictably and report what actually happened. The current implementation writes priority and disabled through separate physical file operations, so one field can commit while the second fails. It also uses different execution and recording paths for ordinary Apply, cooldown, and reset; production audit methods do not persist their inputs, Reset omits failed attempts from its counters, and post-write persistence is repeated with inconsistent error handling.

Users need one credential change to behave as one Host Transition, failures to remain local to the affected credential, and diagnostics to describe the resulting Host state accurately. They do not need a new transaction subsystem, write-ahead journal, or exact process-crash reconstruction when the existing periodic scheduler can observe current Host state and compensate on a later round.

## Solution

Deepen the Apply Layer around one Host Transition module. It accepts explicit expected and target credential state, updates all target fields in one document replacement, preserves unrelated fields, verifies the resulting Host state, and returns a truthful credential-level outcome.

Scheduled Apply, 429 Reactive Cooldown, and priority reset cross the same seam. Planner remains the sole authority for quota-based scheduling decisions; cooldown and reset supply explicit operational targets. Different credentials continue independently, failed transitions are not retried in the same round, and later scheduled rounds provide compensation from current Host state and Fresh Evidence.

Execution records distinguish Host outcome from record-persistence outcome. Round totals and management diagnostics are derived from credential-level transition details, replacing the no-op audit seam, duplicated persistence branches, and hand-maintained counters.

## User Stories

1. As a CPA administrator, I want priority and disabled changes for one credential to commit together, so that an intentional Host Transition cannot stop halfway between those fields.
2. As a CPA administrator, I want transition success to reflect the resulting Host state, so that a callback result is not mistaken for proof of the credential state.
3. As a CPA administrator, I want the plugin to verify a credential after committing it, so that diagnostics reflect what CPA can actually read.
4. As a CPA administrator, I want an external priority or disabled change to invalidate an older transition, so that the scheduler does not overwrite newer Host or manual decisions.
5. As a CPA administrator, I want token refreshes and unrelated credential metadata preserved, so that a priority transition cannot erase Host-maintained data.
6. As a CPA administrator, I want one credential failure to leave later credentials eligible for execution, so that a local file problem does not halt fleet-wide scheduling.
7. As a CPA administrator, I want failed transitions left for a later scheduled round instead of retried immediately, so that execution remains simple and predictable.
8. As a CPA administrator, I want 429 Reactive Cooldown to set priority to `-1` without changing disabled, so that cooldown remains non-destructive.
9. As a CPA administrator, I want a 429 Cooldown fact retained even when its immediate Host demotion fails, so that later scheduling can still compensate.
10. As a CPA administrator, I want priority reset to remove the priority field without changing disabled, so that reset restores default Host selection without re-enabling credentials.
11. As a CPA administrator, I want ordinary Apply, cooldown, and reset to use the same outcome vocabulary, so that diagnostics have consistent meaning.
12. As a CPA administrator, I want to distinguish `no_change`, `committed`, `failed`, `conflict`, and `uncertain`, so that I can understand the Host result without interpreting internal errors.
13. As a CPA administrator, I want Host outcome and execution-record outcome reported separately, so that a committed Host change is not mislabeled when history persistence fails.
14. As a CPA administrator, I want round totals derived from credential outcomes, so that attempted, successful, failed, conflicting, and uncertain counts cannot drift from their details.
15. As a CPA administrator, I want committed transitions to remain committed if the caller is cancelled immediately afterward, so that an applied change is not reported as uncommitted.
16. As a CPA administrator, I want management history and diagnostics fully redacted, so that operational results can be shared without exposing authentication material.
17. As a plugin operator, I want Manual Apply, Auto Apply, 429 Cooldown, and Reset to remain serialized by the Runtime single-flight policy, so that plugin-owned writes do not race each other.
18. As a plugin operator, I want Probe and read-only synchronization to remain non-writing operations, so that observation cannot unexpectedly change credentials.
19. As a maintainer, I want all physical Host changes to cross one module interface, so that transition behaviour has one implementation and one primary test surface.
20. As a maintainer, I want obsolete per-field writes, no-op audit methods, duplicated persistence, and manual counters removed, so that future work cannot bypass the deep module.

## Implementation Decisions

1. **Primary module and seam**
   - Build one deep Host Transition module in the Apply Layer.
   - Its interface accepts an immutable round of credential transition intents and returns credential-level details plus a derived round result.
   - Physical document editing, expected-state comparison, verification, redaction, and result derivation remain behind this seam.
   - Do not add a port or adapter unless two real implementations exist.

2. **Transition intent semantics**
   - Each intent contains current-process credential identity, expected decision state, explicit target operations, cause, and metadata required for a redacted result.
   - Priority and disabled independently express `unchanged`, `set`, or `unset`; zero values do not imply an operation.
   - Ordinary Apply converts immutable Planner changes into intents.
   - 429 Reactive Cooldown uses `set priority=-1` and `unchanged disabled`.
   - Priority reset uses `unset priority` and `unchanged disabled`.
   - Planner remains pure and exclusively owns quota-based priority calculation.

3. **One credential, one file replacement**
   - Read the latest credential document once, apply all target fields in memory, and replace the document once.
   - Preserve unknown and unrelated fields from the latest document.
   - Successful file replacement is the transition commit point.
   - Cancellation before commit yields `failed`; cancellation after commit cannot negate the Host change.

4. **Host authority and conflict handling**
   - Before commit, compare credential identity, priority presence/value, and disabled state with the intent's expected state.
   - Decision-relevant differences produce `conflict` and leave Host unchanged.
   - Unrelated metadata differences do not conflict and are preserved.
   - After commit, reread and validate the document plus target fields.
   - A result that cannot be proven after attempted commit becomes `uncertain` and is not retried immediately.

5. **Outcome model**
   - Host outcome uses exactly `no_change`, `committed`, `failed`, `conflict`, and `uncertain`.
   - Stable reason values explain cancellation, I/O, missing documents, conflicts, and verification failures without multiplying outcome states.
   - Record outcome is separate from Host outcome and exposes persistence failure without rewriting Host truth.
   - Round statistics are derived from transition details, never manually incremented by execution paths.

6. **Credential independence and retry policy**
   - A failure, conflict, or uncertain result for one credential does not stop later credentials.
   - Do not add same-round retry, replay, or backoff behaviour.
   - A later scheduled round may compensate only after observing current Host state and producing a new Plan from Fresh Evidence.

7. **Execution recording**
   - Generate one redacted execution result from transition details and project it through the existing Runtime history and state-cache facilities.
   - Surface record-persistence failures without claiming that an already committed Host change failed.
   - Keep the existing bounded display-history behaviour.
   - Do not add a write-ahead journal, pending-intent store, recovery schema, or new persistence path.

8. **429 Cooldown ordering**
   - Persist the 429 Cooldown fact before attempting its Host Transition using the existing cooldown state mechanism.
   - Host demotion failure does not remove cooldown.
   - Record the cooldown cause and Host outcome through the common result projection.

9. **Runtime integration**
   - Runtime retains ownership of Manual Apply, Auto Apply, Probe, SyncHost, Reset, filter events, worker lifecycle, and `runMu`.
   - All Host-changing paths route through the Host Transition module while holding the existing single-flight policy.
   - Probe and read-only synchronization do not create transition intents.
   - Management and CGO adapters remain thin and do not calculate targets or edit credential files.

10. **Migration and contraction**
    - Migrate ordinary Apply, 429 Reactive Cooldown, and Reset to the common module in controlled slices.
    - Replace production no-op audit methods and repeated post-write persistence branches with one result-projection path.
    - Remove direct per-field writes from migrated execution paths.
    - Preserve Planner gates, Fresh Evidence, ForceWrite, minimum change, cooldown policy, and immutable Plan output.
    - Preserve existing user-visible behaviour except where results and diagnostics become more accurate.

## Testing Decisions

1. **Primary test seam**
   - Test through the Host Transition module's round interface using real temporary credential documents.
   - Observe returned outcomes and final Host files; do not assert on private helper calls, temporary-file names, serialization order, or internal document functions.

2. **Atomic credential behaviour**
   - Verify priority and disabled change together and are both visible after success.
   - Verify `set`, `unset`, and `unchanged`, including reset and cooldown semantics.
   - Verify unrelated fields survive and an already-satisfied target returns `no_change`.
   - Verify pre-commit and post-commit cancellation semantics.

3. **Conflict and verification behaviour**
   - Verify changed priority, disabled, or credential identity produces `conflict` without overwrite.
   - Verify unrelated metadata changes are preserved without conflict.
   - Verify post-commit mismatch or unreadable final state produces `uncertain` without immediate retry.
   - Verify missing, malformed, or inaccessible documents produce fact-based outcomes.

4. **Credential independence and derived results**
   - Verify one credential failure, conflict, or uncertainty does not stop later credentials.
   - Verify all counts derive from details and cannot disagree with them.
   - Verify Host outcome and record outcome remain independently observable.

5. **Runtime integration**
   - Prove Manual Apply and Auto Apply convert Planner changes and use the common lifecycle under `runMu`.
   - Prove 429 Cooldown remains persisted when Host demotion fails and preserves disabled.
   - Prove Reset removes priority, preserves disabled, and reports every attempted credential accurately.
   - Prove Probe and read-only Sync remain non-writing.

6. **Recording and management projection**
   - Verify all three paths project consistent redacted history.
   - Verify record-persistence failure is visible without changing a committed Host outcome.
   - Verify management counts and diagnostics derive from transition details.
   - Verify identifiers and authentication material remain redacted.

7. **Replace, do not layer**
   - Replace tests tied to separate field calls, no-op audit ordering, and manual counters after equivalent behaviour is covered at the Host Transition seam.
   - Retain Planner tests for scheduling decisions and Runtime tests for use-case ownership without inspecting transition implementation.

8. **Quality gates**
   - Require repository formatting and static analysis.
   - Require `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` to pass.
   - Use deterministic concurrency and cancellation controls rather than timing-dependent sleeps.

## Out of Scope

- Write-ahead or sidecar journals.
- Exact process-crash reconstruction, pending-intent coordination, replay, or recovery schemas.
- Same-round automatic retries, configurable retry policy, or backoff.
- Distributed or cross-process transactions with CPA, users, or other processes.
- Additional persistence paths, databases, message brokers, checksums, journal versions, or migration machinery.
- Changing Weekly Urgency, Dynamic Boost Horizon, Equal Priority Clustering, Fresh Evidence, Soft Depletion, or Hard Depletion calculations.
- Implementing the other second-round architecture proposals.
- Reorganizing management-page assets.
- Re-enabling credentials during priority reset.
- Expanding beyond Google Antigravity.
- Committing, releasing, or changing version numbers as part of this specification.

## Further Notes

- This specification implements ADR-0005 and uses the canonical Host Transition term from the domain glossary.
- The design deliberately prefers the smallest module that fixes demonstrated partial-write, reporting, and duplication problems.
- Periodic scheduling and current Host observation are the compensation mechanism; speculative crash-recovery infrastructure is intentionally deferred.
- The work should be split into a small number of blocker-aware tickets before coding.
- The primary behavioural test seam is the Host Transition module; Runtime and management tests verify connection and presentation only.
