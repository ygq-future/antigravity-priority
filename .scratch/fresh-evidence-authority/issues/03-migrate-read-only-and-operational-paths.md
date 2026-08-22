# 03: Migrate read-only and operational paths

**What to build:** Apply the same evidence semantics to SyncHost, post-reset snapshots, and dual Model Group projection while preserving 429 Reactive Cooldown and explicit priority reset as independent operational authorities rather than synthetic Fresh Evidence.

**Blocked by:** 01 — Establish the Fresh Evidence authority.

**Status:** completed

- [x] SyncHost can display historical observations and current failure diagnostics without producing write-qualified quota Changes.
- [x] Post-reset snapshots use historical observations only for read-only projection and do not reinterpret old failures as target-disabled decisions.
- [x] Only the configured active Model Group can contribute Fresh Evidence to quota-driven Host planning; the alternate group remains predicted and read-only.
- [x] Historical or failed observations in either Model Group cannot become write-qualified through snapshot construction.
- [x] 429 Reactive Cooldown remains authorized by the observed 429 event and preserves disabled state without exposing synthetic Fresh Evidence.
- [x] Explicit priority reset remains authorized by the operator command and preserves disabled state without requiring or exposing Fresh Evidence.
- [x] No new persistence path, schema migration, policy configuration, or recovery mechanism is introduced.
