# 01 — Repository Foundation & Minimal CGO Tracer Bullet

**What to build:** Initialize the root Go module `antigravity-priority`, quality toolchain, core domain entities, configuration parser, and a minimal CGO plugin skeleton (`main.go`) that compiles to a shared dynamic library and successfully handles `plugin.register` handshake and buffer deallocation on the local development platform.

**Blocked by:** None — can start immediately.

**Status:** ready-for-agent

## Scope
- Initialize `go.mod` (Go 1.25+), `.gitignore` (ignoring `example-src/`, `dist/`, `package/`), and `.golangci.yml`.
- Define core domain types in `internal/core/credential.go` (Credential, Provider, ModelGroup, PlanType, Freshness).
- Implement configuration parser in `internal/config/config.go` with YAML/JSON decoding, schema validation, and sensible defaults.
- Implement minimal `internal/runtime/` and `main.go` exporting CGO ABI symbols (`cliproxy_plugin_init`, `antigravityPriorityPluginCall`, `antigravityPriorityPluginFreeBuffer`, `antigravityPriorityPluginShutdown`) handling `plugin.register`.

## Explicit Non-Goals
- No Google quota probing.
- No priority planning or comparator algorithm.
- No Web Management UI.

## Acceptance Criteria
- [ ] `go mod tidy` and `go test ./...` pass with zero failures.
- [ ] `golangci-lint run` passes with zero warnings.
- [ ] `go build -buildmode=c-shared -o test_plugin.dll .` (or `.so`/`.dylib` on non-Windows) compiles successfully.
- [ ] Exported CGO ABI symbols (`cliproxy_plugin_init`, `antigravityPriorityPluginCall`, `antigravityPriorityPluginFreeBuffer`, `antigravityPriorityPluginShutdown`) are verified present.
- [ ] Calling `plugin.register` through CGO ABI returns valid JSON plugin metadata, and calling `free_buffer` frees memory without leak or double-free.

## Required Tests
- [ ] Unit tests for `config.LoadBytes` (default values, duration parsing, invalid schema errors).
- [ ] CGO ABI smoke test validating register response and buffer lifecycle.
