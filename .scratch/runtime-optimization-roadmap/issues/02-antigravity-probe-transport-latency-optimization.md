# 02 — Antigravity Probe Transport Latency Optimization

Status: ready-for-agent

## What to fix
In `internal/runtime/antigravity_transport.go:68-104`, quota probing currently follows a rigid two-stage fallback:
1. Every probe first queries `antigravityQuotaURLs` (`retrieveUserQuotaSummary`).
2. Google recently moved Gemini quotas exclusively to `fetchAvailableModels`, leaving `retrieveUserQuotaSummary` with only Claude/GPT models.
3. As a result, every Gemini probe observes `results[ModelGroupGemini].Status != StatusReady` on the first request and subsequently fires a second sequential HTTP request to `antigravityAvailableModelsURLs` (`fetchAvailableModels`).

This serial fallback doubles the HTTP round-trip time (RTT) for every Gemini quota check. Under multi-account probing or high network latency, this creates substantial delays and increases timeout risk.

## Confirmed Semantics
1. If the target probe request specifies `ModelGroupGemini` (or the runtime's configured control group is `gemini` and Gemini quota is required), the transport should prioritize or directly query `fetchAvailableModels` where Gemini quota is authoritatively available.
2. If `fetchAvailableModels` returns ready quotas for all requested models (Gemini + Claude/GPT), avoid making an unnecessary second request to `retrieveUserQuotaSummary`.
3. Transport retains fallback capability for unexpected upstream API schema shifts without breaking existing response parsing in `internal/provider/antigravity/parse.go`.

## Acceptance Criteria
- [ ] Probing Gemini does not unconditionally incur two sequential HTTP requests per account.
- [ ] Probe latency for Gemini accounts is cut by ~50% in standard execution.
- [ ] Fallback mechanisms remain intact if a primary endpoint returns non-200 or missing model groups.
- [ ] Full coverage and contract tests pass in `antigravity_transport_test.go` and `probe_evidence_test.go`.
