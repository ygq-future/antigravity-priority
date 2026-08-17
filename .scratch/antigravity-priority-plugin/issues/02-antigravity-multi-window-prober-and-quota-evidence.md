# 02 — Antigravity Multi-Window Prober & Quota Evidence

**What to build:** Given a credential context and target model group (`gemini` | `claude_gpt`), query Google's `retrieveUserQuotaSummary` endpoint via host HTTP calls, parse multi-window models/buckets/groups JSON payloads, and output normalized `ProbeEvidence` containing $R_{\text{5h}}, T_{\text{5h}}, R_{\text{7d}}, T_{\text{7d}}$, window classifications, observed timestamps, and readiness status.

**Blocked by:** 01 (Repository Foundation & Minimal CGO Tracer Bullet)

**Status:** ready-for-agent

## Scope
- Implement prober and multi-window parser in `internal/provider/antigravity/` (`parse.go`, `probe.go`, `types.go`).
- Parse models, buckets, and quota groups from upstream Google JSON responses.
- Accurate window token classification: distinct matching for `5h` short window (rejecting `15h`/`25hr` false positives) and `weekly`/`7d` long window.
- Model group filtering: restrict extracted quota strictly to `gemini` or `claude_gpt`.
- Safe error handling: non-200 status codes, network errors, and malformed JSON safely map to `Status = probe_failed` without panic.

## Explicit Non-Goals
- No local persistent caching on disk (handled in Ticket 03).
- No priority ranking or host patching.

## Acceptance Criteria
- [ ] Successfully extracts both 5h short window ($R_{\text{5h}}, T_{\text{5h}}$) and 7d weekly window ($R_{\text{7d}}, T_{\text{7d}}$) from standard Google JSON payloads.
- [ ] When probing for `gemini`, only Gemini model quotas are parsed; when probing for `claude_gpt`, only Claude/GPT model quotas are parsed.
- [ ] Window classifier accurately separates `5h`/`5hr` from `15h`/`25hr` and `5d 15h`.
- [ ] Prober handles upstream 401, 403, 429, and network timeouts returning safe `ProbeResult` with status `probe_failed`.

## Required Tests
- [ ] Table-driven JSON decoding test covering standard dual-window, 5h-only, weekly-only, and bucket/group variants.
- [ ] Prober integration test with mock `HTTPDoer` covering successful queries, URL failover, and error codes.
- [ ] Model group filtering verification tests for `gemini` vs `claude_gpt`.
- [ ] Automated test coverage $\ge 90\%$ for `internal/provider/antigravity`.
