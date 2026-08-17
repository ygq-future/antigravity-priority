# 0001: Antigravity-Exclusive Specialized Architecture

## Context
The legacy `credential-priority` plugin attempted to support OpenAI Codex, xAI, and Antigravity within a single codebase. This introduced massive complexity: 429 backoff state machines, OAuth refresh heuristics, multiple conflicting priority policies, and bloated binary size. Antigravity operates on deterministic multi-window quotas (5h + 7d) that require precise pacing rather than generic error-driven fallbacks.

## Decision
Strip out all Codex and xAI code entirely to build a dedicated, high-performance plugin exclusively for Antigravity (`antigravity-priority`). The plugin operates as a single-provider Go module supporting `gemini` and `claude_gpt` model groups.

## Consequences
- Significant reduction in binary size and memory footprint.
- Zero residual complexity from unrelated providers.
- Enables deep, specialized mathematical optimization tailored to Antigravity's multi-window quota model.
