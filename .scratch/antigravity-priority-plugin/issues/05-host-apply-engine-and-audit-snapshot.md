# 05 — Host Apply Engine & Audit Snapshot

**What to build:** Given an immutable `Plan`, apply canonical Fresh Evidence gating (`EvidenceFresh == true` or `ForceWrite == true`) and `min_change` threshold filtering, execute atomic updates against the CPA Host via `PatchPriority` and `PatchDisabled`, and record a structured, redacted audit snapshot and execution run history.

**Blocked by:** 04 (Weekly Urgency Planner & Pure Dry-Run)

**Status:** ready-for-agent

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
- [ ] Credentials lacking Fresh Evidence and without `ForceWrite` are strictly skipped from host write-back.
- [ ] Credential changes below `min_change` with unchanged disabled status are skipped and marked `skipped`.
- [ ] Priority changes invoke `PatchPriority`; disabled changes invoke `PatchDisabled`.
- [ ] Execution snapshot redacts all sensitive credential values to `[REDACTED]`.

## Required Tests
- [ ] Unit tests with mock `Host` and mock `Auditor`.
- [ ] Test Fresh Evidence gating (stale evidence skipped).
- [ ] Test `min_change` threshold skipping.
- [ ] Test successful patch of priority and disabled flags.
- [ ] Test redaction of authorization headers, tokens, and cookie values.
- [ ] Automated test coverage $\ge 90\%$ for `internal/apply`.
