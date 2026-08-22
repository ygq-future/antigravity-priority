# 05: Contract Legacy Execution Seams and Verify

**What to build:** Complete the architecture deepening by removing obsolete execution paths and tests, leaving one Host Transition interface for all physical credential changes and a fully verified green repository.

**Blocked by:** 04 — Truthful Execution History and Diagnostics.

**Status:** ready-for-agent

- [ ] The production no-op audit seam is removed once all required behaviour is covered by the Host Transition lifecycle.
- [ ] Ordinary Apply, cooldown, and reset no longer call independent priority or disabled write interfaces.
- [ ] Hand-maintained execution counters and superseded duplicate persistence branches are removed.
- [ ] Tests coupled to legacy per-field calls or no-op audit ordering are replaced or deleted rather than layered beneath the new primary test seam.
- [ ] Planner purity, Runtime use-case ownership, Management thinness, and existing domain behaviour remain intact.
- [ ] The implementation is checked against the accepted specification, ADR-0005, the domain glossary, and the repository release checklist.
- [ ] Repository formatting and static analysis complete without warnings.
- [ ] `go build ./...`, `go vet ./...`, `go test -v ./...`, and `go test -race ./...` all pass.
- [ ] No version bump, release, or Git commit is created without the user's separate approval and acceptance.
