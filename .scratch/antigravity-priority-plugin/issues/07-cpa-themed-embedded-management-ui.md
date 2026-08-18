# 07 — CPA-Themed Embedded Management UI

**What to build:** Deliver a self-contained, zero-external-CDN HTML/CSS/JS web dashboard embedded in the binary and served at `GET /status`. Automatically adhere to CPA host dark and light themes (using CSS variables for `:root`, `@media (prefers-color-scheme: dark)`, and `[data-theme="dark"]`). Display real-time 5h/7d double window meters, countdowns, adaptive $C_{\text{cycle}}$ metrics, Urgency scores, and Boost badges, with interactive Model Group switching (`gemini` vs `claude_gpt`) and manual Dry-Run/Apply triggers.

**Blocked by:** 06 (Plugin Runtime, Scheduler & CGO ABI)

**Status:** completed

## Scope
- Implement embedded web UI in `internal/management/templates.go` and `internal/management/handler.go` (`GET /status`).
- Dual-theme CSS token system seamlessly matching CPA host's light/dark modes without redundant toggle buttons.
- Real-time visual meters for 5-hour short window and 7-day weekly window with countdown clocks.
- Displays learned $C_{\text{cycle}}$, Weekly Urgency index, and 🚀 Boost badges per credential.
- UI controls: Model Group dropdown (`gemini` / `claude_gpt`), Dry-Run button (previewing diffs with high-contrast badge styling), Apply button (executing write-back).
- Pure native JS frontend calling backend Management APIs (`GET /status`, `POST /run`).

## Explicit Non-Goals
- No client-side quota calculation in JS (frontend strictly renders backend API responses).
- No external CDN dependencies (fonts, stylesheets, or JS libraries).

## Acceptance Criteria
- [x] Single HTML template embedded in binary with zero external network requests (CSP strict compliant).
- [x] Automatically renders dark theme on `data-theme="dark"` or dark system preference; renders clean light theme otherwise.
- [x] Renders 5h and 7d progress bars, reset countdowns, adaptive $C_{\text{cycle}}$, urgency index, and boost status.
- [x] Dry-Run action previews planned priority changes with clear diff highlights; Apply action executes host write-back and refreshes status.

## Required Tests
- [x] Handler test verifying `GET /status` returns HTTP 200 with HTML content-type.
- [x] CSS token completeness test ensuring every ground and text token is defined for both light and dark modes.
- [x] Security validation ensuring no remote URLs (`http://`, `https://`) exist in the template.
