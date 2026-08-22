# 04: Complete Dual Model Group Projection Migration

**What to build:** Make the shared projection interface the single implementation of Model Group selection, planning roles, Predicted marking, response shape, and projection time across Runtime and Management while preserving the existing public response contract.

**Blocked by:** 02 — Use Shared Projection for Quota Runs; 03 — Use Shared Projection for Read-Only Views; Fresh Evidence Authority 04 — Contract legacy evidence semantics and verify.

**Status:** ready-for-agent

- [ ] Production, Probe, SyncHost, Reset, and LatestSnapshot all obtain dual-group results through the shared projection interface.
- [ ] Control authority comes only from Dynamic Config throughout Runtime and Management route handling.
- [ ] Existing Management requests and dual-group response fields remain compatible for integration clients.
- [ ] Projection rules are covered at the pure module seam, with Runtime and Management tests focused on use-case ordering, side effects, and presentation connections.
- [ ] The codebase contains one implementation of alternate-group selection, Predicted role assignment, complete response construction, and projection-time semantics.
- [ ] Existing Planner behaviour for cooldowns, minimum change, clustering, boost, Soft Depletion, and Hard Depletion remains covered.
- [ ] Repository formatting and static analysis pass.
- [ ] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.
