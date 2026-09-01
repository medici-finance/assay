#!/usr/bin/env bash
# changelog-check — the PR-gate decision, extracted so it is unit-testable
# offline (tools/changelog/check_test.sh) and so the staged workflow stays thin.
#
# THE CONVENTION IT ENFORCES. A notable change records itself by ADDING one
# fragment file under changelog/ (changelog/<slug>.md); a genuinely non-notable
# PR carries the changelog:skip label instead. The retired path — hand-editing
# CHANGELOG.md's `## Unreleased` section — is now REFUSED: that shared section
# was a standing merge-conflict generator, and CHANGELOG.md is written only by
# the release workflow (which aggregates the fragments).
#
# THREE OUTCOMES, decided cheaply from the diff (no semantic classifier):
#   REFUSE  — the PR adds a bullet under `## Unreleased` in CHANGELOG.md
#             (deprecation guard; absolute — checked first, a skip label does
#             not excuse it).
#   PASS    — the changelog:skip label is present, OR the PR adds/updates a
#             fragment file under changelog/ (other than README.md).
#   FAIL    — none of the above: no fragment, no skip.
#
# INPUTS (env):
#   SKIP        "true" when the PR carries the changelog:skip label, else "false"
#   BASE_SHA    the PR base commit
#   HEAD_SHA    the PR head commit
#   CHANGELOG_AGG  path to aggregate.py (default: alongside this script)
#
# It reads git history only; it contacts no network and needs no toolchain
# beyond git + python3.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGG="${CHANGELOG_AGG:-$here/aggregate.py}"
SKIP="${SKIP:-false}"
: "${BASE_SHA:?BASE_SHA is required}"
: "${HEAD_SHA:?HEAD_SHA is required}"

# The Unreleased bullet ENTRIES at a given commit's CHANGELOG.md, or nothing when
# the file is absent there. Piped through aggregate.py so the parse matches the
# release-time aggregator exactly.
unreleased_at() {
  local sha="$1" tmp
  tmp="$(mktemp "${TMPDIR:-/tmp}/cl-XXXXXX")"
  if git show "${sha}:CHANGELOG.md" >"$tmp" 2>/dev/null; then
    python3 "$AGG" unreleased-bullets "$tmp" || true
  fi
  rm -f "$tmp"
}

# 1) DEPRECATION GUARD (absolute, first). Compare the Unreleased bullet sets at
#    base and head; any bullet present at head but not at base is a NEW direct
#    edit of the retired section. Set difference means a pre-existing residual
#    bullet (untouched by this PR) never trips the guard — only an addition does.
base_bul="$(unreleased_at "$BASE_SHA" || true)"
head_bul="$(unreleased_at "$HEAD_SHA" || true)"
added="$(comm -13 <(printf '%s\n' "$base_bul" | sort -u) <(printf '%s\n' "$head_bul" | sort -u) | sed '/^$/d' || true)"
if [ -n "$added" ]; then
  echo "::error title=Unreleased edit refused::This PR adds highlight line(s) under '## Unreleased' in CHANGELOG.md. That section is RETIRED — it was a standing merge-conflict source. Record the change as a fragment file instead: changelog/<slug>.md (one-to-few highlight lines; see changelog/README.md). CHANGELOG.md is written only by the release workflow, which aggregates the fragments at cut time."
  echo "Refused — added under '## Unreleased':"
  printf '%s\n' "$added"
  exit 1
fi

# 2) SKIP label — the escape hatch for a genuinely non-notable PR. Always printed
#    so a skipped changelog is visible in the check record, never silent.
if [ "$SKIP" = "true" ]; then
  echo "::notice title=changelog:skip::PR carries the 'changelog:skip' label — a changelog fragment is not required for this PR."
  echo "SKIP: changelog:skip label present — fragment requirement waived for this PR."
  exit 0
fi

# 3) FRAGMENT added/updated? Any added-or-modified changelog/*.md other than
#    README.md satisfies the gate. --diff-filter=AM: an added fragment (the norm)
#    or a modified one; a deletion never satisfies it.
frag="$(git diff --name-status --diff-filter=AM "${BASE_SHA}...${HEAD_SHA}" -- 'changelog/' \
          | awk '{print $2}' \
          | grep -E '^changelog/.+\.md$' \
          | grep -v '^changelog/README\.md$' || true)"
if [ -n "$frag" ]; then
  echo "PASS: this PR adds/updates changelog fragment(s):"
  printf '%s\n' "$frag"
  exit 0
fi

echo "::error title=missing changelog fragment::This PR adds no changelog fragment. Record the change by adding one file under changelog/ — changelog/<slug>.md, a one-to-few-line human-legible highlight (same notable bar as before; see changelog/README.md) — or label the PR 'changelog:skip' if the change is genuinely non-notable (a typo, a comment-only diff)."
exit 1
