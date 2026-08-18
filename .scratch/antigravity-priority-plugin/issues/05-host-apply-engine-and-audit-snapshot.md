# 05 — Host Apply Engine & Audit Snapshot

**What to build:** Given an immutable `Plan`, apply canonical Fresh Evidence gating (`EvidenceFresh == true` or `ForceWrite == true`) and `min_change` threshold filtering, execute atomic updates against the CPA Host via `PatchPriority` and `PatchDisabled`, and record a structured, redacted audit snapshot and execution run history.

**Blocked by:** 04 (Weekly Urgency Planner & Pure Dry-Run)

**Status:** completed

## Scope
- Implement host apply and audit logic in `internal/apply/` (`apply.go`, `audit.go`) and `internal/host/client.go`.
- Fresh Evidence gating: strictly verify `EvidenceFresh == true` (current round probe success) or `ForceWrite == true` (peer uniqueness shift).
- Delta threshold gating: skip write-back if $|\text{Priority}_{\text{new}} - \text{Priority}_{\text{old}}| < \text{min\_change}$ and `Disabled` state is unchanged.
- Host modification: invoke `Host.PatchPriority` and `Host.PatchDisabled`.
- Audit snapshot: generate structured `PlanSnapshot` and `AuditEvent` with full redaction of tokens, authorization headers, and API keys.

## Explicit Non-Goals
- No custom priority calculation (must consume `Plan` from Ticket 04 as single source of truth).
- No Web UI presentation (handled in Ticket 07).

## Acceptance Criteria
- [x] Credentials lacking Fresh Evidence and without `ForceWrite` are strictly skipped from host write-back.
- [x] Credential changes below `min_change` with unchanged disabled status are skipped and marked `skipped`.
- [x] Priority changes invoke `PatchPriority`; disabled changes invoke `PatchDisabled`.
- [x] Execution snapshot redacts all sensitive credential values to `[REDACTED]`.

## Required Tests
- [x] Unit tests with mock `Host` and mock `Auditor`.
- [x] Test Fresh Evidence gating (stale evidence skipped).
- [x] Test `min_change` threshold skipping.
- [x] Test successful patch of priority and disabled flags.
- [x] Test redaction of authorization headers, tokens, and cookie values.
- [x] Automated test coverage $\ge 90\%$ for `internal/apply`.
