# 04: Contract legacy evidence semantics and verify

**What to build:** Remove the caller-owned and contradictory freshness mechanisms after every production path uses the Evidence authority, then verify that Fresh Evidence has one implementation and that all existing scheduling, depletion, cooldown, and projection behaviour remains intact.

**Blocked by:** 02 — Migrate quota runs to the Fresh Evidence authority; 03 — Migrate read-only and operational paths.

**Status:** completed

- [x] Runtime callers no longer implement their own current-round timestamp filter or reconstruct Fresh Evidence from multiple flags.
- [x] Planner no longer treats probe failure as fresh, depletion, disabled state, or a forced quota write.
- [x] Persisted observations can no longer claim scheduling eligibility merely because they were loaded from a successful cache entry.
- [x] Tests that require cached observations to be fresh, probe failure to imply disabled, or callers to assemble matching freshness flags are replaced by behavioural coverage at the Evidence module seam.
- [x] Existing Soft Depletion, Hard Depletion, Weekly Urgency, Dynamic Boost Horizon, Equal Priority Clustering, minimum-change, cooldown, and dual-group behaviour remains covered.
- [x] Repository formatting and static analysis pass.
- [x] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.
