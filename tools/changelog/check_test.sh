#!/usr/bin/env bash
# Offline, network-free unit test for the changelog PR-gate (check.sh).
#
# It builds throwaway git repositories under a temp dir and runs the check
# against real base/head commits, so the git-diff plumbing is exercised end to
# end. No cluster, no GitHub, no toolchain beyond git + bash + python3.
#
# Default impl is ../check.sh (the new fragment gate). Point CHECK_IMPL at
# testdata/old-check.sh to see the two NEW behaviours fail against the retired
# logic — the committed fail-first evidence:
#   CHECK_IMPL=testdata/old-check.sh ./check_test.sh   # RED on C1 and C4
#   ./check_test.sh                                     # all green
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${CHECK_IMPL:-$here/check.sh}"
case "$CHECK" in /*) ;; *) CHECK="$here/$CHECK" ;; esac
export CHANGELOG_AGG="$here/aggregate.py"   # the parser check.sh leans on

pass=0; fail=0
ok()   { echo "ok   - $1"; pass=$((pass+1)); }
bad()  { echo "FAIL - $1"; fail=$((fail+1)); }

# run_case <name> <expected-exit> — the caller has staged a repo in $R with base
# at tag 'base' and head at HEAD; env SKIP is read from the environment.
run_case() {
  local name="$1" want="$2"
  local base head got
  base="$(git -C "$R" rev-parse base)"
  head="$(git -C "$R" rev-parse HEAD)"
  ( cd "$R" && SKIP="${SKIP:-false}" BASE_SHA="$base" HEAD_SHA="$head" bash "$CHECK" ) >/dev/null 2>&1
  got=$?
  if [ "$got" = "$want" ]; then ok "$name (exit $got)"; else bad "$name (want exit $want, got $got)"; fi
}

newrepo() {
  R="$(mktemp -d "${TMPDIR:-/tmp}/clcheck-XXXXXX")"
  git -C "$R" init -q
  git -C "$R" config user.email t@t; git -C "$R" config user.name t
  mkdir -p "$R/changelog"
}
commit() { git -C "$R" add -A; git -C "$R" commit -q -m "$1"; }

CL_EMPTY_UNREL=$'# Changelog\n\n## Unreleased\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n'

# ── C1: fragment added → PASS (new); the retired gate FAILS this (no CHANGELOG bullet)
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
echo '- `widget` gained a turbo mode.' > "$R/changelog/feat-widget-turbo.md"; commit "add fragment"
run_case "C1 fragment-added greens" 0

# ── C2: no fragment, no skip → FAIL (new AND retired)
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
echo 'x' > "$R/unrelated.txt"; commit "unrelated change"
run_case "C2 no-fragment reds" 1

# ── C3: changelog:skip label, no fragment → PASS (new AND retired)
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
echo 'x' > "$R/unrelated.txt"; commit "unrelated change"
SKIP=true run_case "C3 skip-label greens" 0

# ── C4: PR adds a bullet under ## Unreleased → REFUSE (new); the retired gate PASSES it
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
printf '# Changelog\n\n## Unreleased\n\n### Added\n- someone hand-edited Unreleased.\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n' > "$R/CHANGELOG.md"
commit "edit Unreleased directly"
run_case "C4 Unreleased-edit refused" 1

# ── C5: a residual Unreleased bullet already at base, PR only adds a fragment →
#        PASS (new): the guard is a set-difference, so a pre-existing bullet the
#        PR did not add never trips it.
newrepo
printf '# Changelog\n\n## Unreleased\n\n### Changed\n- pre-existing residual bullet.\n\n## v0.1.0 — 2026-01-01\n\n### Added\n- seed\n' > "$R/CHANGELOG.md"
commit init; git -C "$R" tag base
echo '- `gadget` learned to whistle.' > "$R/changelog/feat-gadget-whistle.md"; commit "add fragment, leave Unreleased alone"
run_case "C5 pre-existing residual not a false-positive" 0

echo "---"
echo "check_test: $pass passed, $fail failed (impl: $CHECK)"
[ "$fail" = 0 ]
