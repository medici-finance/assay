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

# ── GATE-INTEGRITY: a fragment must carry real content. A file that satisfies the
#    FILENAME but has no highlight bullet records nothing, so it must RED. These
#    three are the fail-first rows for the content-validation fix — under the
#    pre-hardening stub (CHECK_IMPL=testdata/filename-only-check.sh) they PASS
#    (the hole); under check.sh they RED.

# ── C6: a 0-byte fragment → REJECT (new). `touch changelog/x.md` must not pass.
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
: > "$R/changelog/empty.md"; commit "add 0-byte fragment"
run_case "C6 empty (0-byte) fragment rejected" 1

# ── C7: a whitespace-only fragment → REJECT (new).
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
printf '   \n\n\t\n' > "$R/changelog/blank.md"; commit "add whitespace-only fragment"
run_case "C7 whitespace-only fragment rejected" 1

# ── C8: a fragment with prose but NO bullet → REJECT (new): aggregate.py lifts
#        only bullets, so a bullet-less fragment contributes nothing.
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
printf '### Added\n\nsome prose but no bullet line at all\n' > "$R/changelog/prose.md"; commit "add bullet-less fragment"
run_case "C8 bullet-less fragment rejected" 1

# ── C9: a bullet-less fragment PLUS a real-bullet fragment in the same PR → PASS
#        (new): at least one added fragment carries content.
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
printf 'just prose\n' > "$R/changelog/prose.md"
echo '- `sprocket` gained a brake.' > "$R/changelog/real.md"; commit "one empty, one real"
run_case "C9 mixed empty+real fragment greens" 0

# ── C10: the missing-fragment failure NAMES the exact fix path derived from
#        HEAD_REF (message text only — exit code already covered by C2).
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
echo 'x' > "$R/unrelated.txt"; commit "unrelated change"
c10_base="$(git -C "$R" rev-parse base)"; c10_head="$(git -C "$R" rev-parse HEAD)"
c10_out="$( cd "$R" && SKIP=false BASE_SHA="$c10_base" HEAD_SHA="$c10_head" HEAD_REF="feat/my-branch" bash "$CHECK" 2>&1 || true )"
if printf '%s' "$c10_out" | grep -q 'suggested path: changelog/my-branch.md'; then
  ok "C10 missing-fragment message names changelog/<HEAD_REF-basename>.md"
else
  bad "C10 missing-fragment message names changelog/<HEAD_REF-basename>.md"
fi

echo "---"
echo "check_test: $pass passed, $fail failed (impl: $CHECK)"
[ "$fail" = 0 ]
