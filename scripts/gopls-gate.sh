#!/usr/bin/env bash
# gopls commit gate: reject commits that introduce gopls warnings or
# simplification hints in added or modified Go files.
#
# Usage:
#   scripts/gopls-gate.sh                 # staged Go files (pre-commit default)
#   scripts/gopls-gate.sh <file>...       # explicit Go files (manual audit)
#   scripts/gopls-gate.sh --install-hook  # install the pre-commit hook once per clone
#
# Exit codes: 0 clean, 1 diagnostics found or gopls unavailable.
set -euo pipefail

if [ "${1:-}" = "--install-hook" ]; then
  hook_path="$(git rev-parse --git-path hooks/pre-commit)"
  mkdir -p "$(dirname "$hook_path")"
  cat > "$hook_path" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
exec bash "$(git rev-parse --show-toplevel)/scripts/gopls-gate.sh"
EOF
  chmod +x "$hook_path"
  echo "gopls gate: installed pre-commit hook at $hook_path"
  exit 0
fi

if ! command -v gopls >/dev/null 2>&1; then
  echo "gopls gate: gopls is not installed (go install golang.org/x/tools/gopls@latest)" >&2
  exit 1
fi

if [ "$#" -gt 0 ]; then
  candidates=("$@")
else
  candidates=()
  # -z keeps paths with spaces intact; --no-renames makes a rename a D+A pair.
  while IFS= read -r -d '' file; do
    candidates+=("$file")
  done < <(git diff --cached --name-only -z --no-renames --diff-filter=ACMR -- '*.go')
fi

files=()
for file in "${candidates[@]}"; do
  [ -f "$file" ] && files+=("$file")
done

if [ "${#files[@]}" -eq 0 ]; then
  exit 0
fi

echo "gopls gate: checking ${#files[@]} changed Go file(s) with -severity=hint"

# gopls check exits 0 even when it reports diagnostics, so any output is a
# failure. stderr is folded in to fail closed on analysis errors as well.
if ! output="$(gopls check -severity=hint "${files[@]}" 2>&1)"; then
  printf '%s\n' "$output" >&2
  echo "gopls gate: gopls check failed" >&2
  exit 1
fi

if [ -n "$output" ]; then
  printf '%s\n' "$output" >&2
  echo "gopls gate: fix the warnings/hints above before committing" >&2
  exit 1
fi
