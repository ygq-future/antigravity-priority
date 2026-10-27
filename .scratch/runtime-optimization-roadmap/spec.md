# Runtime Robustness, Probe Latency, and State Storage Optimization Roadmap

## Context
Following comprehensive architecture and runtime analysis of `antigravity-priority` v1.2.21, three concrete operational and architectural improvements were identified in the host integration, quota transport, and persistence layers:
1. **CGO Runtime Lifecycle & Goroutine Leak Prevention**: Global package-level runtime instantiation creates an orphaned background rate-limit worker goroutine when `cliproxy_plugin_init` re-instantiates the runtime.
2. **Antigravity Probe Transport Latency Optimization**: Serial fallback from `retrieveUserQuotaSummary` to `fetchAvailableModels` doubles HTTP round-trip latency for all Gemini quota probes due to upstream Google quota endpoint changes.
3. **State Store Persistence Consolidation & Atomic Temp Safety**: Multiple full-state disk writes during a single task execution amplify I/O, while static `.tmp` temp paths introduce collision risks on Windows.

## Optimization Goals
- Eliminate background goroutine leaks during CGO dynamic library loading and hot reloads.
- Reduce Gemini quota probing latency by up to 50% through target-aware direct endpoint routing.
- Consolidate persistence cycles in `runProductionTask` and harden atomic temp file creation with `os.CreateTemp`.

## Issues
- `01-cgo-runtime-lifecycle-and-goroutine-leak-prevention.md`: Safe lifecycle management for `cpaRuntime` in `main.go`.
- `02-antigravity-probe-transport-latency-optimization.md`: Direct endpoint routing for Gemini/ClaudeGPT quota queries.
- `03-state-store-persistence-consolidation-and-temp-safety.md`: Transactional/batched state persistence and dynamic temp naming.
