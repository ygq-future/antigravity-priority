# 08 — CI/CD Matrix, Release Packaging & Store Registry

**What to build:** Establish GitHub Actions workflows (`ci.yml` for automated linting and race-detector tests; `release.yml` for multi-platform CGO shared library matrix compilation across the exact 7 target platforms), package release zip archives with SHA-256 `checksums.txt`, and provide validated `registry.json` and documentation (`README.md`, `README.en.md`) conforming to `CLIProxyAPI-Plugins-Store` submission standards.

**Blocked by:** 06 (Plugin Runtime, Scheduler & CGO ABI), 07 (CPA-Themed Embedded Management UI)

**Status:** ready-for-agent

## Scope
- Create `.github/workflows/ci.yml` (Lint, `go test -v ./...`, `go test -race ./...` on supported runners).
- Create `.github/workflows/release.yml` with the exact 7-target release matrix:
  1. `linux_amd64` (Ubuntu GCC) $\to$ `antigravity-priority_{ver}_linux_amd64.zip` (`antigravity-priority.so`)
  2. `linux_arm64` (Ubuntu ARM GCC) $\to$ `antigravity-priority_{ver}_linux_arm64.zip` (`antigravity-priority.so`)
  3. `darwin_arm64` (macOS Clang) $\to$ `antigravity-priority_{ver}_darwin_arm64.zip` (`antigravity-priority.dylib`)
  4. `darwin_amd64` (macOS Intel Clang) $\to$ `antigravity-priority_{ver}_darwin_amd64.zip` (`antigravity-priority.dylib`)
  5. `windows_amd64` (MSYS2 UCRT64 GCC) $\to$ `antigravity-priority_{ver}_windows_amd64.zip` (`antigravity-priority.dll`)
  6. `windows_arm64` (Ubuntu + Zig cc `aarch64-windows-gnu`) $\to$ `antigravity-priority_{ver}_windows_arm64.zip` (`antigravity-priority.dll`)
  7. `freebsd_amd64` (FreeBSD VM GCC) $\to$ `antigravity-priority_{ver}_freebsd_amd64.zip` (`antigravity-priority.so`)
- Create `registry.json` (schema version 1, plugin ID `antigravity-priority`).
- Create comprehensive `README.md` and `README.en.md`.

## Explicit Non-Goals
- No functional Go application code changes.

## Acceptance Criteria
- [ ] `ci.yml` passes lint, unit tests, and race detector on supported platforms.
- [ ] `release.yml` matrix compiles all 7 target platforms, packages zip archives, and generates valid `checksums.txt`.
- [ ] `registry.json` conforms to `CLIProxyAPI-Plugins-Store` schema.
- [ ] Documentation clearly describes the mathematical model, double-window scheduling, adaptive burn rate, and configuration options.

## Required Tests
- [ ] Schema validation of `registry.json`.
- [ ] Workflow syntax validation.
