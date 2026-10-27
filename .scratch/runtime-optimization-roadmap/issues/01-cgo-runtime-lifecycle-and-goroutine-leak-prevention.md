# 01 — CGO Runtime Lifecycle and Goroutine Leak Prevention

Status: resolved

## What to fix
In `main.go`, `var cpaRuntime = pluginruntime.New(...)` is instantiated eagerly during package initialization (line 82). When the host application (CPA) initializes the shared library through `cliproxy_plugin_init` (lines 87-95), `cpaRuntime` is reassigned to a newly created `pluginruntime.New(...)` instance without shutting down the earlier instance.

Because `pluginruntime.New` launches a background rate limit worker goroutine (`go rt.runRateLimitWorker()` in `internal/runtime/runtime.go:139`) that listens on `rootCtx.Done()`, the initial instance's goroutine remains running indefinitely, causing a goroutine leak upon plugin initialization. Furthermore, package initialization runs disk cache operations (`state.Load` and `store.SaveAtomic`) before CPA host callbacks are fully registered.

## Confirmed Semantics
1. `cpaRuntime` must not start active background goroutines before the host passes valid host callbacks and invokes `cliproxy_plugin_init`.
2. If `cpaRuntime` is re-initialized (e.g. repeated `cliproxy_plugin_init` or host reload), any existing runtime instance must be cleanly stopped via `Shutdown(ctx)` before the reference is replaced.
3. Access to `cpaRuntime` across CGO exported functions (`antigravityPriorityPluginCall`, `antigravityPriorityPluginShutdown`) must safely handle uninitialized or shutdown states.

## Acceptance Criteria
- [x] No background worker goroutine is spawned during package load / `init()` time.
- [x] `cliproxy_plugin_init` safely initializes or re-initializes `cpaRuntime` without leaking prior worker goroutines.
- [x] `antigravityPriorityPluginShutdown` terminates the active runtime instance and frees associated resources.
- [x] Single and repeated initialization cycles pass clean without goroutine leaks verified by test/race checks.
