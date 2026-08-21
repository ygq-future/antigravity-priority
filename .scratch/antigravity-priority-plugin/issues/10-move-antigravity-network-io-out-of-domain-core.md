# 10 — Move Antigravity Network I/O out of Domain Core

**Status:** completed

**What to fix:** Restore the documented architecture boundary by moving Google request execution out of `internal/provider/antigravity`; keep Antigravity response parsing and normalization in the domain package.

## Confirmed Requirements
- `internal/provider/antigravity` must not call CPA Host HTTP callbacks or perform external network I/O.
- Runtime/Application orchestration owns request execution and passes the response payload plus observation time into the Antigravity parser/normalizer.
- One Google response must still populate both `gemini` and `claude_gpt` model-group evidence.
- Existing redaction, retry, error classification, and testability must be preserved.

## Acceptance Criteria
- [x] No production code under `internal/provider/antigravity` imports `internal/host` or invokes `HTTPDo`.
- [x] Runtime tests cover request execution, fallback endpoints, and error propagation.
- [x] Provider tests operate on supplied response data without external I/O.
- [x] Existing probe behavior and dual-group parsing remain unchanged from the user's perspective.
