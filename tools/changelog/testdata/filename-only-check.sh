#!/usr/bin/env bash
# PRE-HARDENING changelog-check logic, preserved ONLY as fail-first evidence for
# check_test.sh's malformed-fragment cases. It is the fragment gate BEFORE the
# content-validation fix: it PASSES on the mere PRESENCE of an added/modified
# changelog/*.md file, without checking that the file carries a real highlight
# bullet. So an empty / whitespace-only / bullet-less fragment PASSES under it —
# the GATE-INTEGRITY hole. Running
#   CHECK_IMPL=testdata/filename-only-check.sh ./check_test.sh
# reddens exactly the malformed-fragment cases; the default (check.sh) rejects
# them. Deprecation guard + skip are identical to check.sh so only the hole
# differs. Not wired into any workflow.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGG="${CHANGELOG_AGG:-$here/../aggregate.py}"
SKIP="${SKIP:-false}"
: "${BASE_SHA:?}"; : "${HEAD_SHA:?}"

unreleased_at() {
  local sha="$1" tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/cl-XXXXXX")"
  if git show "${sha}:CHANGELOG.md" >"$tmp" 2>/dev/null; then
    python3 "$AGG" unreleased-bullets "$tmp" || true
  fi
  rm -f "$tmp"
}

base_bul="$(unreleased_at "$BASE_SHA" || true)"
head_bul="$(unreleased_at "$HEAD_SHA" || true)"
added="$(comm -13 <(printf '%s\n' "$base_bul" | sort -u) <(printf '%s\n' "$head_bul" | sort -u) | sed '/^$/d' || true)"
if [ -n "$added" ]; then
  echo "(pre-hardening) Unreleased edit refused"; exit 1
fi
if [ "$SKIP" = "true" ]; then
  echo "(pre-hardening) SKIP label present"; exit 0
fi
# THE HOLE: filename presence only — no content check.
frag="$(git diff --name-status --diff-filter=AM "${BASE_SHA}...${HEAD_SHA}" -- 'changelog/' \
          | awk '{print $2}' | grep -E '^changelog/.+\.md$' | grep -v '^changelog/README\.md$' || true)"
if [ -n "$frag" ]; then
  echo "(pre-hardening) PASS on fragment filename presence (no content check)"; exit 0
fi
echo "(pre-hardening) no fragment"; exit 1
