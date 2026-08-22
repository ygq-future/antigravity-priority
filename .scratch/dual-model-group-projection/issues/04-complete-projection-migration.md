# 04: Complete Dual Model Group Projection Migration

**What to build:** Make the shared projection interface the single implementation of Model Group selection, planning roles, Predicted marking, response shape, and projection time across Runtime and Management while preserving the existing public response contract.

**Blocked by:** 02 — Use Shared Projection for Quota Runs; 03 — Use Shared Projection for Read-Only Views; Fresh Evidence Authority 04 — Contract legacy evidence semantics and verify.

**Status:** completed

- [x] Production, Probe, SyncHost, Reset, and LatestSnapshot all obtain dual-group results through the shared projection interface.
- [x] Control authority comes only from Dynamic Config throughout Runtime and Management route handling.
- [x] Existing Management requests and dual-group response fields remain compatible for integration clients.
- [x] Projection rules are covered at the pure module seam, with Runtime and Management tests focused on use-case ordering, side effects, and presentation connections.
- [x] The codebase contains one implementation of alternate-group selection, Predicted role assignment, complete response construction, and projection-time semantics.
- [x] Existing Planner behaviour for cooldowns, minimum change, clustering, boost, Soft Depletion, and Hard Depletion remains covered.
- [x] Repository formatting and static analysis pass.
- [x] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.

## Completion

`runtime.ProjectDualModelGroups` is now the shared pure projection seam for all Runtime paths. The existing `ActiveModelGroup`, `ObservedAt`, and `Groups` response fields remain unchanged, while Management view/query input cannot change Dynamic Config control authority.
