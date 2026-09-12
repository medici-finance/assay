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
#   CHECK_IMPL=testdata/old-check.sh ./check_test.sh   # RED set below
#   ./check_test.sh                                     # all green
#
# Under testdata/old-check.sh the RED set is C1, C4, C5, C9, C10 (the rows the
# fragment rework itself was fail-first for) PLUS P1 and P7 — the two fail-first
# rows for the PROXY path: the retired gate has no notion of a fragment landed on
# the base branch (P1), and its failure message cannot mention one (P7). P5 also
# reds there, but for the SAME pre-existing reason C1 does — the retired gate
# cannot see a fragment file at all — so it carries no information about the
# proxy path; P1 and P7 are the rows that do.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${CHECK_IMPL:-$here/check.sh}"
case "$CHECK" in /*) ;; *) CHECK="$here/$CHECK" ;; esac
export CHANGELOG_AGG="$here/aggregate.py"   # the parser check.sh leans on

# This test may itself be running inside a GitHub Actions pull_request job, in
# which case GITHUB_BASE_REF is set in the real environment. Unset it here so
# every case starts from a clean slate and controls it explicitly via run_case's
# GITHUB_BASE_REF argument/env — otherwise an inherited value would silently
# leak into cases that mean to test the fully-unset path (e.g. P10).
unset GITHUB_BASE_REF || true

pass=0; fail=0
ok()   { echo "ok   - $1"; pass=$((pass+1)); }
bad()  { echo "FAIL - $1"; fail=$((fail+1)); }

# run_case <name> <expected-exit> — the caller has staged a repo in $R with base
# at tag 'base' and head at HEAD; env SKIP, PR_NUMBER, BASE_REF and
# GITHUB_BASE_REF are read from the environment (`SKIP=true run_case …`,
# `PR_NUMBER=77 run_case …`, `BASE_REF=main run_case …`,
# `GITHUB_BASE_REF=main run_case …`). PR_NUMBER is forwarded as the EMPTY STRING
# when the caller does not set it, which is how check.sh sees an unsupplied PR
# number — the P3 row asserts that the verdict is then exactly what it was
# before the proxy path existed. BASE_REF and GITHUB_BASE_REF are forwarded the
# same way: both empty means "neither supplied", which sends the proxy path to
# its origin/HEAD leg (best-effort only) or, failing that, to its degraded
# BASE_SHA fallback (P10). GITHUB_BASE_REF set with BASE_REF unset is the
# zero-config CI path (P9); both set proves BASE_REF wins (P12).
run_case() {
  local name="$1" want="$2"
  local base head got
  base="$(git -C "$R" rev-parse base)"
  head="$(git -C "$R" rev-parse HEAD)"
  ( cd "$R" && SKIP="${SKIP:-false}" PR_NUMBER="${PR_NUMBER:-}" BASE_REF="${BASE_REF:-}" GITHUB_BASE_REF="${GITHUB_BASE_REF:-}" BASE_SHA="$base" HEAD_SHA="$head" bash "$CHECK" ) >/dev/null 2>&1
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

# ── PROXY FRAGMENT (fork PRs): a maintainer lands changelog/pr-<N>-<slug>.md on
#    the BASE branch on behalf of a PR whose branch they cannot commit to. These
#    are the fail-first rows for that path — P1 REDs under the retired gate
#    (testdata/old-check.sh), which has no notion of a proxy at all.

# proxyrepo <fragment-body> — a repo whose BASE already carries
# changelog/pr-77-fix.md, and whose HEAD adds only an unrelated file (the fork
# PR contributes no fragment of its own, which is the whole premise).
proxyrepo() {
  newrepo
  printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"
  printf '%s' "$1" > "$R/changelog/pr-77-fix.md"
  commit init; git -C "$R" tag base
  echo 'x' > "$R/unrelated.txt"; commit "the fork PR's actual change"
}

PROXY_WITH_BULLET=$'### Fixed\n- `widget` no longer drops the last frame. (#77, thanks @someone)\n'
PROXY_NO_BULLET=$'### Fixed\n\nprose describing the fix, but no bullet line\n'

# ── P1: proxy on base, matching PR_NUMBER → PASS (new behaviour).
proxyrepo "$PROXY_WITH_BULLET"
PR_NUMBER=77 run_case "P1 proxy on base for this PR greens" 0

# ── P2: the same proxy, a DIFFERENT PR_NUMBER → FAIL. The <N> in the filename is
#        what binds a proxy to exactly one PR; it must never green another.
proxyrepo "$PROXY_WITH_BULLET"
PR_NUMBER=78 run_case "P2 proxy for another PR does not green this one" 1

# ── P3: the same proxy, PR_NUMBER UNSET → FAIL, i.e. the verdict is byte-for-byte
#        what it was before the proxy path existed. This is the degradation row:
#        the script half may merge before the workflow half that supplies
#        PR_NUMBER, and until it does nothing about the gate changes.
proxyrepo "$PROXY_WITH_BULLET"
run_case "P3 proxy ignored when PR_NUMBER is unset" 1

# ── P4: proxy on base with NO highlight bullet → FAIL. Same content bar as an
#        in-PR fragment: the filename is not enough.
proxyrepo "$PROXY_NO_BULLET"
PR_NUMBER=77 run_case "P4 bullet-less proxy rejected" 1

# ── P5: the proxy-shaped file added by the PR ITSELF (head only, nothing on base)
#        → PASS by the ordinary added-fragment rule, not by the proxy path. The
#        proxy name is a plain fragment name; nothing about it is special when the
#        PR can add it, and that ordinary path is still the first one checked.
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
printf '%s' "$PROXY_WITH_BULLET" > "$R/changelog/pr-77-fix.md"; commit "PR adds its own fragment"
PR_NUMBER=77 run_case "P5 proxy-named fragment added by the PR greens the ordinary way" 0

# ── P6: a non-integer PR_NUMBER is treated as UNSET → FAIL. An un-updated
#        workflow passes the literal unexpanded expression, and that must degrade
#        to today's behaviour rather than being parsed into a regex.
proxyrepo "$PROXY_WITH_BULLET"
PR_NUMBER=abc run_case "P6 non-integer PR_NUMBER treated as unset" 1

# ── P7: the missing-fragment failure TELLS a fork PR about the proxy path
#        (message text only — exit code already covered by C2).
newrepo
printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"; commit init; git -C "$R" tag base
echo 'x' > "$R/unrelated.txt"; commit "unrelated change"
p7_base="$(git -C "$R" rev-parse base)"; p7_head="$(git -C "$R" rev-parse HEAD)"
p7_out="$( cd "$R" && SKIP=false BASE_SHA="$p7_base" HEAD_SHA="$p7_head" bash "$CHECK" 2>&1 || true )"
if printf '%s' "$p7_out" | grep -q 'changelog/pr-<N>-<slug>.md on the base branch'; then
  ok "P7 missing-fragment message documents the proxy path"
else
  bad "P7 missing-fragment message documents the proxy path"
fi

# ── PROXY READ FROM THE LIVE BASE TIP, not the recorded base sha. GitHub stamps
#    github.event.pull_request.base.sha when the PR is OPENED and never advances
#    it as the base branch moves — so a proxy merged AFTER the fork PR opened,
#    which is the only case the proxy path exists for, is invisible in that tree.
#    These rows are the fail-first evidence: P8/P9 RED against a check.sh that
#    reads BASE_SHA (the proxy exists only on the live tip), P10 pins the
#    degraded fallback, P11 pins that the live-tip read did not loosen the
#    one-proxy-one-PR binding.

# liveproxyrepo <fragment-body> <pr-number> [set-origin-head] — a repo whose tag
# 'base' is the OLD base sha (no proxy in it) and whose refs/remotes/origin/main
# carries the proxy, i.e. the real shape: the proxy landed on the base branch
# after this PR was opened. The PR's own branch (HEAD) adds only an unrelated
# file. refs/remotes/origin/main is created with update-ref rather than a real
# clone — it is the exact ref check.sh resolves, so the fixture is faithful and
# stays offline.
liveproxyrepo() {
  local body="$1" n="$2" set_origin_head="${3:-}"
  newrepo
  printf '%s' "$CL_EMPTY_UNREL" > "$R/CHANGELOG.md"
  commit init
  git -C "$R" tag base                       # the sha GitHub recorded at PR-open
  local base_sha; base_sha="$(git -C "$R" rev-parse base)"

  # The base branch moves on: a maintainer merges the proxy fragment. Built on a
  # side branch so 'base' and the PR head both stay where they are.
  git -C "$R" checkout -q -b live-base "$base_sha"
  printf '%s' "$body" > "$R/changelog/pr-${n}-fix.md"
  commit "maintainer lands the proxy on the base branch, after the PR opened"
  local live_sha; live_sha="$(git -C "$R" rev-parse HEAD)"
  git -C "$R" update-ref refs/remotes/origin/main "$live_sha"
  [ -n "$set_origin_head" ] && git -C "$R" update-ref refs/remotes/origin/HEAD "$live_sha"

  # Back to the PR's own branch, which contributes no fragment.
  git -C "$R" checkout -q -b pr-branch "$base_sha"
  rm -f "$R/changelog/pr-${n}-fix.md"
  echo 'x' > "$R/unrelated.txt"; commit "the fork PR's actual change"
}

# ── P8: proxy only on the LIVE base tip, BASE_REF names the branch → PASS.
#        Reading BASE_SHA (the bug) cannot see the fragment at all.
liveproxyrepo "$PROXY_WITH_BULLET" 77
PR_NUMBER=77 BASE_REF=main run_case "P8 proxy landed after PR-open greens via BASE_REF live tip" 0

# ── P9: the ZERO-CONFIG CI path — BASE_REF unset, GITHUB_BASE_REF set (as
#        GitHub Actions sets it on every pull_request run, no workflow change
#        needed), and refs/remotes/origin/HEAD absent (the realistic case: an
#        actions/checkout run never creates that symref) → still PASS, because
#        check.sh resolves BASE_REF from GITHUB_BASE_REF. This is the row that
#        actually proves "works with no workflow change" — origin/HEAD is
#        deliberately NOT set up for this case.
liveproxyrepo "$PROXY_WITH_BULLET" 77
PR_NUMBER=77 GITHUB_BASE_REF=main run_case "P9 proxy on live tip greens via GITHUB_BASE_REF with BASE_REF unset, no origin/HEAD" 0

# ── P10: neither ref resolvable → the proxy lookup DEGRADES to BASE_SHA, which
#         does not carry the proxy, so the PR reds — and says so in a NOTICE
#         rather than failing mutely on the ordinary missing-fragment path alone.
liveproxyrepo "$PROXY_WITH_BULLET" 77
git -C "$R" update-ref -d refs/remotes/origin/main
p10_base="$(git -C "$R" rev-parse base)"; p10_head="$(git -C "$R" rev-parse HEAD)"
p10_out="$( cd "$R" && SKIP=false PR_NUMBER=77 BASE_SHA="$p10_base" HEAD_SHA="$p10_head" bash "$CHECK" 2>&1 )"
p10_rc=$?
if [ "$p10_rc" = 1 ] && printf '%s' "$p10_out" | grep -q 'proxy lookup degraded'; then
  ok "P10 no live-base ref: falls back to BASE_SHA, reds, and announces the degradation"
else
  bad "P10 no live-base ref: falls back to BASE_SHA, reds, and announces the degradation (rc=$p10_rc)"
fi

# ── P11: the live tip carries a proxy for PR 78; this is PR 77 → FAIL. Reading
#         the live tip must not loosen the one-proxy-one-PR binding P2 pins.
liveproxyrepo "$PROXY_WITH_BULLET" 78
PR_NUMBER=77 BASE_REF=main run_case "P11 live-tip proxy for another PR does not green this one" 1

# ── P12: BOTH BASE_REF (explicit override) and GITHUB_BASE_REF (zero-config
#         default) set, to DIFFERENT branch names → BASE_REF wins. Point
#         GITHUB_BASE_REF at a branch that does not resolve at all, so a pass
#         can only mean BASE_REF's leg was the one taken.
liveproxyrepo "$PROXY_WITH_BULLET" 77
PR_NUMBER=77 BASE_REF=main GITHUB_BASE_REF=other run_case "P12 explicit BASE_REF wins over GITHUB_BASE_REF" 0

echo "---"
echo "check_test: $pass passed, $fail failed (impl: $CHECK)"
[ "$fail" = 0 ]
