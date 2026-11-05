# 04: Domain Docs, Bilingual READMEs, XERJ Sync & Quality Gates

Status: `closed`
Type: `task`
Blocked by: 01, 02, 03

## What to Build

Align domain documentation (`CONTEXT.md`, `AGENTS.md`, `README.md`, `README.en.md`, and `cmd/devserver`) with the Dynamic Model-Scoped Quota Scheduler architecture, verify UI behavior in a real browser via `cmd/devserver`, synchronize the XERJ index, and pass all release quality gates (`golangci-lint`, `go build`, `go vet`, `go test -race`).

## Exact File Boundaries & Changes

### 1. Domain & User Documentation (`CONTEXT.md`, `README.md`, `README.en.md`, `AGENTS.md`)
- Update `CONTEXT.md`:
  - Update **Quota Pool / Model Group** definition to include all three active pools (`gemini`, `argon`, `claude_gpt`).
  - Document **Dynamic Model-Scoped Scheduler (`scheduler.pick`)**, **Session-Sticky Equal-Priority Clustering**, and **5-Window Unified Quota Projection**.
- Update `README.md` and `README.en.md` (strictly bilingual and free of internal `(REQ-xx)` tags):
  - Document how `antigravity-priority` hooks into CPA's `scheduler.pick` (`scheduler_across_priorities: true`) to dynamically route requests per model across `Gemini`, `Argon`, and `Claude & GPT` quota pools without static priority conflicts.
  - Document the redesigned Model-Centric Routing Board UI and 5-bar credential quota cards.
- Update `cmd/devserver/devhost.go`:
  - Ensure `devserver` mock data includes realistic multi-pool quota buckets (`Gemini 5h/7d`, `Claude 5h/7d`, `Argon 7d`) and `ListAuthModels` responses so `go run ./cmd/devserver` showcases the full 5-bar cards and model routing order out of the box.

### 2. Verification, XERJ Index Sync & Post-Impl Checklist
- Run `cmd/devserver` and verify the Web UI in Chromium (`eval` browser):
  - Verify switching models (`gemini-3.8-flash-high` vs `gemini-4-argon-high` vs `claude-sonnet-5-5-high`) reorders the credential cards and updates active pool highlights and tier badges;
  - Verify all 5 quota bars (`Gemini 5h`, `Gemini 7d`, `Claude 5h`, `Claude 7d`, `Argon 7d`) render cleanly in both light and dark themes;
  - Verify language toggle (`zh-CN` / `en-US`) and Samples modal across all 3 pools.
- Run XERJ index sync:
  - `xerj autoindex . --prefix agp --no-graph --code-analyzer code --yes`
- Run full quality gate suite:
  - `golangci-lint run --timeout=5m`
  - `go build ./...`
  - `go vet ./...`
  - `go test ./...`
  - `go test -race ./...`
- Execute `skill://post-impl-checklist` 7-dimensional convergence audit.

## Acceptance Criteria

- [x] `CONTEXT.md`, `README.md`, and `README.en.md` accurately describe the 3-pool dynamic `scheduler.pick` architecture and 5-bar model routing board.
- [x] `cmd/devserver` smoke test confirms interactive model switching, call-order tier sorting, and 5-bar rendering.
- [x] `xerj autoindex . --prefix agp --no-graph --code-analyzer code --yes` exits cleanly with `code_files_indexed > 0`.
- [x] `golangci-lint run --timeout=5m`, `go build ./...`, `go vet ./...`, `go test ./...`, and `go test -race ./...` pass with zero warnings and zero data races.
