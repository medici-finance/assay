#!/usr/bin/env bash
# pr-monitor.test.sh — the proof that pr-monitor.sh emits the right PR-EVENT for
# every change kind, seeds silently, retains a truncated read, and — the new
# properties — PACES its reads and STOPS the cycle the moment the forge signals a
# secondary rate limit.
#
# No network and no token: `gh` AND `desktoken` are stubbed on PATH per case, so
# the suite is hermetic and runs in CI. The gh stub records every invocation
# (timestamp + repo) to a call log, so pacing and the stop-on-limit rule are
# asserted against what the script ACTUALLY called, not what it printed. The
# desktoken stub records every mint (role + repo) and hands back a per-repo token
# file, so the identity each read ran under is asserted against what gh SAW.
#
# Run all cases:            bash pr-monitor.test.sh
# Run one named case:       bash pr-monitor.test.sh --case <name>
#   cases: seed-silent opened pushed draft-flip state closed push-and-flip
#          truncation rate-limit-stops-cycle pacing-honoured portability
#          origin-forms origin-degraded mint-per-repo mint-failed
#          mint-seed-failed role-unusable mint-bad-custody mint-reread-race
#          role-mapping bare-gh-lint mint-forge-pin-lint forge-gitlab-seed
#          forge-aware-delegation
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="${PR_MONITOR_TEST_SCRIPT:-$HERE/pr-monitor.sh}"

CASE_FILTER=""
if [[ "${1:-}" == "--case" ]]; then CASE_FILTER="${2:-}"; fi
want() { [[ -z "$CASE_FILTER" || "$CASE_FILTER" == "$1" ]]; }

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }
check() { if [[ "$1" -eq 0 ]]; then ok "$2"; else no "$2" "$3"; fi; }
contains() { case "$1" in (*"$2"*) return 0 ;; esac; return 1; }
# exactly one line in $1 matching the fixed substring $2
countsub() { printf '%s\n' "$1" | grep -Fc "$2"; }

# ---- one PR object / an array of them --------------------------------------
pr() { # num sha draft(true|false) state mergeState
  printf '{"number":%s,"headRefOid":"%s","isDraft":%s,"state":"%s","mergeStateStatus":"%s"}' \
    "$1" "$2" "$3" "$4" "$5"
}
arr() { local IFS=,; printf '[%s]' "$*"; }

# ---- the stub gh -----------------------------------------------------------
# make_gh <dir> — a `gh` that records each call and serves per-repo JSON from
#   <dir>/resp/<owner__name>.json (default "[]"), or emits a canned stderr and
#   exits 1 when <dir>/fail/<owner__name> exists.
make_gh() {
  local dir="$1"
  mkdir -p "$dir/resp" "$dir/fail" "$dir/tok" "$dir/mintfail" "$dir/mintblank" "$dir/mintnopath" "$dir/mintrace"
  : > "$dir/calls.log"
  : > "$dir/token.log"
  : > "$dir/mint.log"
  cat > "$dir/gh" <<STUB
#!/usr/bin/env bash
d="$dir"
repo=""; prev=""
for a in "\$@"; do [ "\$prev" = "--repo" ] && repo="\$a"; prev="\$a"; done
san=\$(printf '%s' "\$repo" | sed 's#/#__#g')
printf '%s %s\n' "\$(date +%s)" "\$repo" >> "\$d/calls.log"
printf 'GH_TOKEN=[%s] GITHUB_TOKEN=[%s]\n' "\${GH_TOKEN:-}" "\${GITHUB_TOKEN:-}" >> "\$d/token.log"
if [ -f "\$d/fail/\$san" ]; then cat "\$d/fail/\$san" >&2; exit 1; fi
if [ -f "\$d/resp/\$san.json" ]; then cat "\$d/resp/\$san.json"; else printf '[]'; fi
exit 0
STUB
  chmod +x "$dir/gh"
  # desktoken <role> --repo <slug> — records the mint, then prints the PATH of a
  # per-repo token file holding x-minted-<owner__name> (a placeholder that wears no
  # real credential prefix). Per-repo markers under <dir> select a failure:
  #   mintfail/<san>    stderr text, exit 5 (a refused mint)
  #   mintblank/<san>   exit 0, prints a path to a whitespace-only file
  #   mintnopath/<san>  exit 0, prints nothing
  #   mintrace/<san>    a good token, then another session's re-mint TRUNCATES the
  #                     shared cache file 0.3s later (desktoken rewrites it in place)
  cat > "$dir/desktoken" <<STUB
#!/usr/bin/env bash
d="$dir"
role="\${1:-}"; repo=""; prev=""
for a in "\$@"; do [ "\$prev" = "--repo" ] && repo="\$a"; prev="\$a"; done
san=\$(printf '%s' "\$repo" | sed 's#/#__#g')
printf '%s --repo %s\n' "\$role" "\$repo" >> "\$d/mint.log"
if [ -f "\$d/mintfail/\$san" ]; then cat "\$d/mintfail/\$san" >&2; exit 5; fi
if [ -f "\$d/mintnopath/\$san" ]; then exit 0; fi
if [ -f "\$d/mintblank/\$san" ]; then printf ' \n\t\n' > "\$d/tok/\$san"; else printf 'x-minted-%s\n' "\$san" > "\$d/tok/\$san"; fi
chmod 600 "\$d/tok/\$san"
printf '%s\n' "\$d/tok/\$san"
if [ -f "\$d/mintrace/\$san" ]; then ( sleep 0.3; : > "\$d/tok/\$san" ) >/dev/null 2>&1 & fi
exit 0
STUB
  chmod +x "$dir/desktoken"
}
san() { printf '%s' "$1" | sed 's#/#__#g'; }
resp() { printf '%s' "$3" > "$1/resp/$(san "$2").json"; }          # <dir> <repo> <json>
fail_ratelimit() { # <dir> <repo>
  printf 'HTTP 403: You have exceeded a secondary rate limit. Please wait a few minutes before you try again.' \
    > "$1/fail/$(san "$2")"
}
clear_fail() { rm -f "$1/fail/$(san "$2")"; }
fail_mint() { printf '%s' "$3" > "$1/mintfail/$(san "$2")"; }       # <dir> <repo> <stderr text>
clear_mint() { rm -f "$1/mintfail/$(san "$2")"; }

# run <ghdir> <statedir> [ENV=VAL ...] -- <args...> — sets OUT/RC. Defaults to
# ASSAY_MONITOR_PACE_SECONDS=0 for speed and DESK_LOOP=pr-review-desk (the review
# desk's own loop) unless the case overrides them.
run() {
  local ghdir="$1" statedir="$2"; shift 2
  local envs=(ASSAY_MONITOR_PACE_SECONDS=0 DESK_LOOP=pr-review-desk)
  while [[ "${1:-}" != "--" && $# -gt 0 ]]; do envs+=("$1"); shift; done
  shift
  OUT=$(env "${envs[@]}" PR_MONITOR_STATE_DIR="$statedir" \
        PATH="$ghdir:$BASE_PATH" bash "$SCRIPT" "$@" 2>"$ghdir/stderr")
  RC=$?
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

# BASE_PATH — the inherited PATH minus every entry holding an executable
# `deskmonitor`. With one installed, a legacy cycle is handed to `deskmonitor pr`
# (the forge-aware reader); the cases below pin the script's OWN reader, so they
# must never pick up a real install. A case that wants the handoff plants a stub.
BASE_PATH=""
_old_ifs=$IFS; IFS=:
for _p in $PATH; do
  [[ -n "$_p" && -x "$_p/deskmonitor" ]] && continue
  BASE_PATH="${BASE_PATH:+$BASE_PATH:}$_p"
done
IFS=$_old_ifs

echo "pr-monitor regression suite (bash $BASH_VERSION)"
[[ -n "$CASE_FILTER" ]] && echo "  (only case: $CASE_FILTER)"

# ================================================================ seed =======
if want seed-silent; then
  echo "seed-silent — first sight of a repo seeds silently, arms, no PR-EVENT"
  w="$TMPROOT/seed"; st="$TMPROOT/seed-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)" "$(pr 2 bbb true OPEN BLOCKED)")"
  run "$w" "$st" -- o/r
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "a seed run exits 0" "got $RC"
  check "$(contains "$OUT" "PR-EVENT" && echo 1 || echo 0)" \
    "a seed run emits no PR-EVENT lines" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "MONITOR-ARMED" && echo 0 || echo 1)" \
    "a seed run prints MONITOR-ARMED with pace and cap" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "pace 0s, cap all" && echo 0 || echo 1)" \
    "the ARMED line echoes the pace and cap in force" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(grep -c . "$st/o__r.state")" == "2" ]] && echo 0 || echo 1)" \
    "the repo's baseline is seeded with its two PRs" "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"
fi

# ================================================================ opened =====
if want opened; then
  echo "opened — a brand-new PR fires exactly one 'opened' event"
  w="$TMPROOT/op"; st="$TMPROOT/op-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)" "$(pr 2 ccc false OPEN CLEAN)")"
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#2 opened - -> ccc")" -eq 1 ]] && echo 0 || echo 1)" \
    "exactly one 'opened' line with old '-' and the new head sha" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 1 ]] && echo 0 || echo 1)" \
    "no other PR-EVENT line fires for the unchanged PR#1" "stdout: ${OUT:-<empty>}"
fi

# ================================================================ pushed =====
if want pushed; then
  echo "pushed — a moved head sha fires exactly one 'pushed' event old->new"
  w="$TMPROOT/pu"; st="$TMPROOT/pu-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed
  resp "$w" o/r "$(arr "$(pr 1 zzz false OPEN CLEAN)")"
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#1 pushed aaa -> zzz")" -eq 1 ]] && echo 0 || echo 1)" \
    "exactly one 'pushed aaa -> zzz'" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 1 ]] && echo 0 || echo 1)" \
    "no second event for the same PR" "stdout: ${OUT:-<empty>}"
fi

# ============================================================= draft-flip ====
if want draft-flip; then
  echo "draft-flip — a draft<->ready flip fires exactly one 'draft-flip' event"
  w="$TMPROOT/df"; st="$TMPROOT/df-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa true OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed (draft)
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#1 draft-flip draft -> ready")" -eq 1 ]] && echo 0 || echo 1)" \
    "exactly one 'draft-flip draft -> ready'" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 1 ]] && echo 0 || echo 1)" \
    "the sha did not move, so no 'pushed' event" "stdout: ${OUT:-<empty>}"
fi

# ================================================================ state ======
if want state; then
  echo "state — a changed state field fires exactly one 'state' event"
  w="$TMPROOT/stt"; st="$TMPROOT/stt-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed (OPEN)
  resp "$w" o/r "$(arr "$(pr 1 aaa false CLOSED CLEAN)")"
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#1 state OPEN -> CLOSED")" -eq 1 ]] && echo 0 || echo 1)" \
    "exactly one 'state OPEN -> CLOSED'" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 1 ]] && echo 0 || echo 1)" \
    "no other event for the same PR" "stdout: ${OUT:-<empty>}"
fi

# ================================================================ closed =====
if want closed; then
  echo "closed — a PR that leaves the open set fires exactly one 'closed' event"
  w="$TMPROOT/cl"; st="$TMPROOT/cl-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)" "$(pr 2 bbb false OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed (two PRs)
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"    # #2 gone
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#2 closed bbb -> -")" -eq 1 ]] && echo 0 || echo 1)" \
    "exactly one 'closed bbb -> -' for the departed PR" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 1 ]] && echo 0 || echo 1)" \
    "the surviving PR#1 fires no event" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(grep -c . "$st/o__r.state")" == "1" ]] && echo 0 || echo 1)" \
    "baseline advanced to the single surviving PR" "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"
fi

# ===================================================== push-and-flip =========
if want push-and-flip; then
  echo "push-and-flip — a push AND a draft-flip on ONE PR are TWO lines, not one"
  w="$TMPROOT/pf"; st="$TMPROOT/pf-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa true OPEN CLEAN)")"
  run "$w" "$st" -- o/r                                    # seed (sha aaa, draft)
  resp "$w" o/r "$(arr "$(pr 1 bbb false OPEN CLEAN)")"    # sha moved AND readied
  run "$w" "$st" -- o/r
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#1 pushed aaa -> bbb")" -eq 1 ]] && echo 0 || echo 1)" \
    "the push is one line" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(countsub "$OUT" "PR-EVENT: o/r#1 draft-flip draft -> ready")" -eq 1 ]] && echo 0 || echo 1)" \
    "the draft-flip is a SECOND line" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 2 ]] && echo 0 || echo 1)" \
    "exactly two PR-EVENT lines — the changes did NOT collapse to one" "stdout: ${OUT:-<empty>}"
fi

# ============================================================= truncation ====
if want truncation; then
  echo "truncation — a read == --limit is could-not-check: degrade + retain baseline"
  w="$TMPROOT/tr"; st="$TMPROOT/tr-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)" "$(pr 2 bbb false OPEN CLEAN)" "$(pr 3 ccc false OPEN CLEAN)")"
  run "$w" "$st" PR_MONITOR_LIMIT=5 -- o/r                 # seed 3 (< limit 5)
  # A shifted 5-key window == the limit.
  resp "$w" o/r "$(arr "$(pr 90 p90 false OPEN CLEAN)" "$(pr 91 p91 false OPEN CLEAN)" "$(pr 92 p92 false OPEN CLEAN)" "$(pr 93 p93 false OPEN CLEAN)" "$(pr 94 p94 false OPEN CLEAN)")"
  run "$w" "$st" PR_MONITOR_LIMIT=5 -- o/r
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "an at-limit read => exit 2" "got $RC"
  check "$(contains "$OUT" "== --limit 5" && contains "$OUT" "TRUNCATED" && echo 0 || echo 1)" \
    "the truncation is named as could-not-check" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "PR-EVENT" && echo 1 || echo 0)" \
    "a truncated read fires no phantom PR-EVENT on the shifted window" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(grep -c . "$st/o__r.state")" == "3" ]] && echo 0 || echo 1)" \
    "the real 3-line baseline is retained, not overwritten by the slice" \
    "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"
fi

# =================================================== rate-limit-stops-cycle ==
if want rate-limit-stops-cycle; then
  echo "rate-limit-stops-cycle — 403 on repo 2 of 4 degrades 2–4 and stops all reads"
  w="$TMPROOT/rl"; st="$TMPROOT/rl-st"; make_gh "$w"
  resp "$w" o/one "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  fail_ratelimit "$w" o/two                                # repo 2 trips the limit
  resp "$w" o/three "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  resp "$w" o/four "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" -- o/one o/two o/three o/four
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a rate-limited cycle exits 2" "got $RC"
  for r in o/two o/three o/four; do
    check "$([[ "$(countsub "$OUT" "MONITOR-DEGRADED: $r rate-limited, skipped")" -eq 1 ]] && echo 0 || echo 1)" \
      "$r is degraded 'rate-limited, skipped'" "stdout: ${OUT:-<empty>}"
  done
  check "$(contains "$OUT" "MONITOR-DEGRADED: o/one" && echo 1 || echo 0)" \
    "the healthy repo 1 (read before the trip) is NOT degraded" "stdout: ${OUT:-<empty>}"
  ncalls=$(grep -c . "$w/calls.log")
  check "$([[ "$ncalls" -eq 2 ]] && echo 0 || echo 1)" \
    "exactly TWO gh calls happened — repos 3 and 4 were never called" "calls: $ncalls"
  check "$([[ "$(printf '%s\n' "$OUT" | grep -c '^PR-EVENT:')" -eq 0 ]] && echo 0 || echo 1)" \
    "no PR-EVENT lines on a rate-limited seed cycle" "stdout: ${OUT:-<empty>}"
fi

# =============================================================== pacing =======
if want pacing-honoured; then
  echo "pacing-honoured — with pace 1 and three repos, the call log spans >= 2s"
  w="$TMPROOT/pc"; st="$TMPROOT/pc-st"; make_gh "$w"
  resp "$w" o/a "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  resp "$w" o/b "$(arr "$(pr 1 bbb false OPEN CLEAN)")"
  resp "$w" o/c "$(arr "$(pr 1 ccc false OPEN CLEAN)")"
  run "$w" "$st" ASSAY_MONITOR_PACE_SECONDS=1 -- o/a o/b o/c
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the paced cycle exits 0" "got $RC"
  ncalls=$(grep -c . "$w/calls.log")
  first=$(head -1 "$w/calls.log" | cut -d' ' -f1)
  last=$(tail -1 "$w/calls.log" | cut -d' ' -f1)
  span=$((last - first))
  check "$([[ "$ncalls" -eq 3 ]] && echo 0 || echo 1)" "all three repos were read" "calls: $ncalls"
  check "$([[ "$span" -ge 2 ]] && echo 0 || echo 1)" \
    "the first and last calls are >= 2s apart (two 1s paces between three reads)" "span: ${span}s"
fi

# ============================================================ origin-forms ===
# regression: #2724
if want origin-forms; then
  echo "origin-forms — the origin fallback resolves every github.com URL form to owner/name"
  w="$TMPROOT/of"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  n=0
  for url in \
    "https://github.com/o/r.git" "https://github.com/o/r" "https://github.com/o/r/" \
    "https://github.com:443/o/r.git" "https://github.com:443/o/r" \
    "https://user@github.com/o/r.git" "https://user:pw@github.com:443/o/r.git" \
    "http://github.com/o/r.git" "git://github.com/o/r.git" \
    "ssh://git@github.com/o/r.git" "ssh://git@github.com:22/o/r.git" "ssh://github.com/o/r" \
    "git@github.com:o/r.git" "git@github.com:o/r" "github.com:o/r"; do
    n=$((n + 1))
    repodir="$TMPROOT/of-repo-$n"; mkdir -p "$repodir"
    git -C "$repodir" init -q 2>/dev/null
    git -C "$repodir" remote add origin "$url"
    : > "$w/calls.log"
    OUT=$(cd "$repodir" && env ASSAY_MONITOR_PACE_SECONDS=0 DESK_LOOP=pr-review-desk PR_MONITOR_STATE_DIR="$TMPROOT/of-st-$n" \
          PATH="$w:$BASE_PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
    check "$([[ "$RC" -eq 0 && "$(awk '{print $2}' "$w/calls.log")" == "o/r" ]] && echo 0 || echo 1)" \
      "origin '$url' resolves to o/r and is polled" \
      "rc=$RC calls=$(cat "$w/calls.log") stdout=${OUT:-<empty>} stderr=$(cat "$w/stderr")"
  done
  # A numeric-looking owner after the scp colon is an owner, not a port.
  repodir="$TMPROOT/of-repo-num"; mkdir -p "$repodir"; git -C "$repodir" init -q 2>/dev/null
  git -C "$repodir" remote add origin "git@github.com:123/r.git"
  resp "$w" 123/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"; : > "$w/calls.log"
  OUT=$(cd "$repodir" && env ASSAY_MONITOR_PACE_SECONDS=0 DESK_LOOP=pr-review-desk PR_MONITOR_STATE_DIR="$TMPROOT/of-st-num" \
        PATH="$w:$BASE_PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
  check "$([[ "$RC" -eq 0 && "$(awk '{print $2}' "$w/calls.log")" == "123/r" ]] && echo 0 || echo 1)" \
    "scp form with a numeric owner (git@github.com:123/r) is an owner, not a port" \
    "rc=$RC calls=$(cat "$w/calls.log")"
fi

# ========================================================= origin-degraded ===
# regression: #2724
if want origin-degraded; then
  echo "origin-degraded — an unresolvable repo set prints MONITOR-DEGRADED on stdout, then exits 1"
  w="$TMPROOT/od"; make_gh "$w"
  # (a) an origin that is not github.com: not a valid owner/name -> degraded, no gh call.
  repodir="$TMPROOT/od-repo-a"; mkdir -p "$repodir"; git -C "$repodir" init -q 2>/dev/null
  pw=s3cretpw # kept out of the URL literal: no userinfo password sits in the source
  git -C "$repodir" remote add origin "https://user:${pw}@example.com:8443/o/r.git"
  OUT=$(cd "$repodir" && env PR_MONITOR_STATE_DIR="$TMPROOT/od-st-a" PATH="$w:$BASE_PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
  ERR=$(cat "$w/stderr")
  check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "an unparseable origin exits 1" "got $RC"
  check "$(grep -q '^MONITOR-DEGRADED: invalid repo' <<<"$OUT" && echo 0 || echo 1)" \
    "stdout carries a MONITOR-DEGRADED line (what the watch filter matches)" "stdout: ${OUT:-<empty>}"
  check "$([[ "$OUT" == "MONITOR-DEGRADED: invalid repo 'https://example.com:8443/o/r' (expected owner/name)" ]] && echo 0 || echo 1)" \
  "credential-stripped display is exact on both Bash versions" "stdout: $OUT"
  check "$(contains "$OUT$ERR" "s3cretpw" && echo 1 || echo 0)" \
    "the origin's credential never appears on stdout or stderr" "stdout: $OUT stderr: $ERR"
  check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" "no gh call was made" "calls: $(cat "$w/calls.log")"
  # (b) no repo args, no repos.txt, no origin remote at all.
  repodir="$TMPROOT/od-repo-b"; mkdir -p "$repodir"; git -C "$repodir" init -q 2>/dev/null
  OUT=$(cd "$repodir" && env PR_MONITOR_STATE_DIR="$TMPROOT/od-st-b" PATH="$w:$BASE_PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
  check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "no resolvable repos exits 1" "got $RC"
  check "$(grep -q '^MONITOR-DEGRADED: no repos to query' <<<"$OUT" && echo 0 || echo 1)" \
    "stdout carries MONITOR-DEGRADED: no repos to query" "stdout: ${OUT:-<empty>}"
  # (c) an explicit invalid arg takes the same path.
  run "$w" "$TMPROOT/od-st-c" -- 'not a repo'
  check "$([[ "$RC" -eq 1 ]] && contains "$OUT" "MONITOR-DEGRADED: invalid repo" && echo 0 || echo 1)" \
    "an invalid repo argument also prints MONITOR-DEGRADED on stdout" "rc=$RC stdout: ${OUT:-<empty>}"
fi

# ============================================================ portability ====
if want portability; then
  echo "portability — bash 3.2 (stock /bin/bash on macOS)"
  for builtin in mapfile readarray; do
    if grep -qE "^[^#]*\\b$builtin\\b" "$SCRIPT"; then
      no "no bash-4-only builtin '$builtin'" "found in $SCRIPT"
    else ok "no bash-4-only builtin '$builtin'"; fi
  done
  for decl in "declare -A" "local -A"; do
    if grep -qE "^[^#]*$decl" "$SCRIPT"; then
      no "no bash-4-only associative-array declaration '$decl'" "found in $SCRIPT"
    else ok "no bash-4-only associative-array declaration '$decl'"; fi
  done
  if grep -qE '^[^#]*\$\{[A-Za-z_][A-Za-z0-9_]*[,^]{1,2}\}' "$SCRIPT"; then
    no "no bash-4-only case-modification expansion" "found in $SCRIPT"
  else ok "no bash-4-only case-modification expansion"; fi
  # An inherited App token must never reach gh: the read runs as the per-repo
  # minted role token instead (the mint-per-repo case pins that in full).
  w="$TMPROOT/tok"; st="$TMPROOT/tok-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" GH_TOKEN=x-bogus-app-token GITHUB_TOKEN=x-bogus-app-token-2 -- o/r
  tokline=$(head -1 "$w/token.log" 2>/dev/null)
  check "$([[ "$tokline" == "GH_TOKEN=[x-minted-o__r] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
    "gh sees the minted role token, never the inherited GH_TOKEN/GITHUB_TOKEN" \
    "gh saw: ${tokline:-<no invocation recorded>}"
fi

# ============================================================ mint-per-repo ==
# regression: #3116
if want mint-per-repo; then
  echo "mint-per-repo — each repo is read under its OWN minted role token, scoped to that one read"
  w="$TMPROOT/mp"; st="$TMPROOT/mp-st"; make_gh "$w"
  resp "$w" o/a "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  resp "$w" p/b "$(arr "$(pr 1 bbb false OPEN CLEAN)")"
  run "$w" "$st" GH_TOKEN=x-bogus-app-token -- o/a p/b
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "a fully-minted cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
  check "$([[ "$(cat "$w/mint.log")" == "reviewer --repo o/a
reviewer --repo p/b" ]] && echo 0 || echo 1)" \
    "one mint per repo, as the loop's role (reviewer), scoped by --repo" "mints: $(tr '\n' ';' < "$w/mint.log")"
  check "$([[ "$(cat "$w/token.log")" == "GH_TOKEN=[x-minted-o__a] GITHUB_TOKEN=[]
GH_TOKEN=[x-minted-p__b] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
    "each gh read carries only ITS repo's token (no token reused across owners)" "gh saw: $(tr '\n' ';' < "$w/token.log")"
  check "$(contains "$OUT$(cat "$w/stderr")" "x-minted" && echo 1 || echo 0)" \
    "the token VALUE never appears on stdout or stderr" "stdout: ${OUT:-<empty>}"
fi

# ============================================================== mint-failed ==
# regression: #3116
if want mint-failed; then
  echo "mint-failed — a failed mint is DEGRADED (could-not-check) for that repo, never empty, never ambient"
  w="$TMPROOT/mf"; st="$TMPROOT/mf-st"; make_gh "$w"
  resp "$w" o/good "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  resp "$w" o/bad "$(arr "$(pr 7 ggg false OPEN CLEAN)" "$(pr 8 hhh false OPEN CLEAN)")"
  run "$w" "$st" -- o/good o/bad                           # seed both
  : > "$w/calls.log"; : > "$w/token.log"; : > "$w/mint.log"
  fail_mint "$w" o/bad "desktoken: refused: no installation of the role App covers o/bad"
  resp "$w" o/good "$(arr "$(pr 1 aaa false OPEN CLEAN)" "$(pr 2 ccc false OPEN CLEAN)")"
  run "$w" "$st" -- o/good o/bad
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a cycle with a failed mint exits 2" "got $RC; stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "MONITOR-DEGRADED: o/bad token mint FAILED" && contains "$OUT" "no installation of the role App covers o/bad" && contains "$OUT" "could-not-check" && echo 0 || echo 1)" \
    "the failed repo names itself, the desktoken cause, and could-not-check" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "keeping its previous 2 PR(s)" && echo 0 || echo 1)" \
    "the degraded line states the retained baseline" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "o/bad#" && echo 1 || echo 0)" \
    "a failed mint is NOT shown as an empty result: no 'closed' PR-EVENT for o/bad" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(grep -c . "$st/o__bad.state")" == "2" ]] && echo 0 || echo 1)" \
    "o/bad's 2-PR baseline is retained, not zeroed" "lines: $(grep -c . "$st/o__bad.state" 2>/dev/null)"
  check "$(contains "$OUT" "PR-EVENT: o/good#2 opened - -> ccc" && echo 0 || echo 1)" \
    "the healthy repo still reads and reports in the same cycle" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "MONITOR-DEGRADED: o/good" && echo 1 || echo 0)" \
    "the healthy repo is not degraded" "stdout: ${OUT:-<empty>}"
  check "$([[ "$(awk '{print $2}' "$w/calls.log")" == "o/good" ]] && echo 0 || echo 1)" \
    "gh was called for o/good ONLY: no read for o/bad, ambient or otherwise" "gh calls: $(tr '\n' ';' < "$w/calls.log")"
  check "$(grep -q 'GH_TOKEN=\[\]' "$w/token.log" && echo 1 || echo 0)" \
    "no gh call ever ran without a minted GH_TOKEN (no ambient fallback)" "gh saw: $(tr '\n' ';' < "$w/token.log")"
  # Recovery: the mint works again, the real set returns, the outage is absorbed.
  clear_mint "$w" o/bad
  run "$w" "$st" -- o/good o/bad
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the recovery cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "o/bad#" && echo 1 || echo 0)" \
    "recovery emits NO phantom PR-EVENT for o/bad (the outage was absorbed)" "stdout: ${OUT:-<empty>}"
fi

# ========================================================= mint-seed-failed ==
# regression: #3116
if want mint-seed-failed; then
  echo "mint-seed-failed — a failed mint on first sight establishes NO baseline and is never armed"
  w="$TMPROOT/ms"; st="$TMPROOT/ms-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  fail_mint "$w" o/r "desktoken: custody for role reviewer is unavailable"
  run "$w" "$st" -- o/r
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a failed-mint seed exits 2" "got $RC; stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "MONITOR-DEGRADED: o/r token mint FAILED" && contains "$OUT" "no baseline established" && echo 0 || echo 1)" \
    "the seed names the mint failure and that no baseline exists" "stdout: ${OUT:-<empty>}"
  check "$(contains "$OUT" "MONITOR-ARMED" && echo 1 || echo 0)" "a degraded seed never prints MONITOR-ARMED" "stdout: ${OUT:-<empty>}"
  check "$([[ ! -f "$st/o__r.state" ]] && echo 0 || echo 1)" "no baseline file is written" "a state file was written"
  check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" "gh was never called" "gh calls: $(tr '\n' ';' < "$w/calls.log")"
fi

# ============================================================ role-unusable ==
# regression: #3116
if want role-unusable; then
  echo "role-unusable — an unset or unknown \$DESK_LOOP degrades EVERY repo; no mint, no gh, no state"
  for loop in "" "not-a-desk-loop"; do
    w="$TMPROOT/ru"; st="$TMPROOT/ru-st"; rm -rf "$w" "$st"; make_gh "$w"
    resp "$w" o/a "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
    resp "$w" o/b "$(arr "$(pr 1 bbb false OPEN CLEAN)")"
    run "$w" "$st" "DESK_LOOP=$loop" GH_TOKEN=x-bogus-app-token -- o/a o/b
    label="DESK_LOOP='${loop}'"
    check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "$label exits 2" "got $RC; stdout: ${OUT:-<empty>}"
    for r in o/a o/b; do
      check "$(contains "$OUT" "MONITOR-DEGRADED: $r token mint FAILED" && contains "$OUT" "DESK_LOOP" && echo 0 || echo 1)" \
        "$label: $r is degraded, naming DESK_LOOP" "stdout: ${OUT:-<empty>}"
    done
    check "$([[ ! -s "$w/calls.log" && ! -s "$w/token.log" ]] && echo 0 || echo 1)" \
      "$label: gh is NEVER invoked (no ambient poll)" "gh calls: $(tr '\n' ';' < "$w/calls.log")"
    check "$([[ ! -s "$w/mint.log" ]] && echo 0 || echo 1)" \
      "$label: no mint is attempted for a role that cannot be named" "mints: $(tr '\n' ';' < "$w/mint.log")"
    check "$([[ ! -f "$st/o__a.state" && ! -f "$st/o__b.state" ]] && echo 0 || echo 1)" \
      "$label: no baseline is written" "state files present"
  done
fi

# ========================================================== mint-bad-custody ==
# regression: #3116
if want mint-bad-custody; then
  echo "mint-bad-custody — desktoken exit 0 with no usable token file is still a failed mint"
  for mode in mintblank mintnopath; do
    w="$TMPROOT/bc-$mode"; st="$TMPROOT/bc-st-$mode"; make_gh "$w"
    resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
    : > "$w/$mode/o__r"
    run "$w" "$st" GH_TOKEN=x-bogus-app-token -- o/r
    check "$([[ "$RC" -eq 2 ]] && contains "$OUT" "MONITOR-DEGRADED: o/r token mint FAILED" && echo 0 || echo 1)" \
      "$mode: degraded as a failed mint" "rc=$RC stdout: ${OUT:-<empty>}"
    check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" \
      "$mode: gh is never invoked (an empty GH_TOKEN would read as the host's login)" "gh calls: $(tr '\n' ';' < "$w/calls.log")"
  done
fi

# ========================================================= mint-reread-race ==
# regression: #3116 (F1/S1: the token was checked in the file, then re-read at the call)
if want mint-reread-race; then
  echo "mint-reread-race — the token that was CHECKED is the token gh gets, even if the cache is emptied during the pacing sleep"
  w="$TMPROOT/rr"; st="$TMPROOT/rr-st"; make_gh "$w"
  resp "$w" o/a "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  resp "$w" o/b "$(arr "$(pr 1 bbb false OPEN CLEAN)")"
  # Synchronize the planted rewrite with the actual pacing boundary, after
  # validation and before gh. A wall-clock background sleep could fire before
  # validation under CPU load, testing a different (correctly refused) condition.
  cat > "$w/sleep" <<STUB
#!/bin/sh
: > "$w/tok/o__b"
STUB
  chmod +x "$w/sleep"
  run "$w" "$st" ASSAY_MONITOR_PACE_SECONDS=1 GH_TOKEN=x-bogus-app-token -- o/a o/b
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
  check "$([[ "$(cat "$w/token.log")" == "GH_TOKEN=[x-minted-o__a] GITHUB_TOKEN=[]
GH_TOKEN=[x-minted-o__b] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
    "gh sees the value that was validated for each repo, never an emptied re-read" "gh saw: $(tr '\n' ';' < "$w/token.log")"
  check "$(contains "$(cat "$w/token.log")" "GH_TOKEN=[]" && echo 1 || echo 0)" \
    "no gh call ever ran with an empty GH_TOKEN (the ambient-login signature)" "gh saw: $(tr '\n' ';' < "$w/token.log")"
fi

# bare_gh_calls <file> — the non-comment lines that run a `gh` subcommand WITHOUT a
# same-line `GH_TOKEN=` assignment of a plain variable: an ambient-identity read, the
# defect class of #3116. A `gh` read here is only ever `GH_TOKEN="$var" gh ...` where
# $var was validated earlier. `GH_TOKEN="$(cat f)"` is flagged too: it re-reads (and
# so does not check) the value it passes, which can come back empty.
bare_gh_calls() {
  grep -nE '(^|[[:space:](])gh (pr|issue|api|auth|repo|run|search) ' "$1" \
    | grep -vE '^[0-9]+:[[:space:]]*#' | grep -vE 'GH_TOKEN="?\$[A-Za-z_{]' || true
}

# regression: #3116
if want bare-gh-lint; then
  echo "bare-gh-lint — no gh call in the script runs under the ambient identity"
  # The planted second instance: a bare ambient read added to a copy of the script
  # must be caught by the same lint (so the lint cannot rot into a no-op).
  planted="$TMPROOT/planted-bare-gh.sh"
  cp "$SCRIPT" "$planted"
  printf '%s\n' 'extra=$(gh pr list --repo o/r --state open)' >> "$planted"
  check "$([[ -n "$(bare_gh_calls "$planted")" ]] && echo 0 || echo 1)" \
    "the lint catches a planted bare gh call" "lint returned nothing for a script with a bare gh read"
  printf '%s\n' 'extra=$(GH_TOKEN="$t" gh pr list --repo o/r --state open)' > "$planted.ok"
  check "$([[ -z "$(bare_gh_calls "$planted.ok")" ]] && echo 0 || echo 1)" \
    "the lint passes a token-scoped gh call" "lint flagged a GH_TOKEN-scoped call"
  printf '%s\n' 'extra=$(GH_TOKEN="$(cat "$f")" gh pr list --repo o/r --state open)' > "$planted.reread"
  check "$([[ -n "$(bare_gh_calls "$planted.reread")" ]] && echo 0 || echo 1)" \
    "the lint catches a planted unchecked re-read at the call site" "lint passed GH_TOKEN=\"\$(cat ...)\""
  bare=$(bare_gh_calls "$SCRIPT")
  check "$([[ -z "$bare" ]] && echo 0 || echo 1)" \
    "no gh call in the script runs without a same-line GH_TOKEN= (no ambient read)" "bare: $bare"
fi

# unpinned_mints <file> — every desktoken INVOCATION (at line start, after `$(`, or
# after an env prefix like ASSAY_CONFIG_ECHO=full) that does not pin --forge github.
# A shipped script's only reader is `gh` (GitHub only); a mint left to follow the
# roster's forge hands it another forge's credential.
unpinned_mints() {
  grep -nE '(^[[:space:]]*|\$\(|[A-Z_]+=[^[:space:]]*[[:space:]]+)desktoken[[:space:]]' "$1" \
    | grep -vE '^[0-9]+:[[:space:]]*#' | grep -vF -- '--forge github' || true
}

# regression: #5889
if want mint-forge-pin-lint; then
  echo "mint-forge-pin-lint — every shipped script's desktoken mint pins --forge github (its reader is gh)"
  planted="$TMPROOT/planted-unpinned-mint.sh"
  printf '%s\n' 'out=$(ASSAY_CONFIG_ECHO=full desktoken "$ROLE" --repo "$repo")' > "$planted"
  printf '%s\n' 'tok=$(desktoken worker --repo o/r)' >> "$planted"
  check "$([[ "$(unpinned_mints "$planted" | grep -c .)" == 2 ]] && echo 0 || echo 1)" \
    "the lint catches both planted unpinned mints" "got: $(unpinned_mints "$planted")"
  printf '%s\n' 'out=$(ASSAY_CONFIG_ECHO=full desktoken "$ROLE" --forge github --repo "$repo")' \
    '  # desktoken <role> --repo <slug> (prose)' 'MINT_WHY="desktoken $ROLE exited $rc"' \
    'command -v desktoken >/dev/null' > "$planted.ok"
  check "$([[ -z "$(unpinned_mints "$planted.ok")" ]] && echo 0 || echo 1)" \
    "the lint passes a pinned mint, prose and diagnostics" "flagged: $(unpinned_mints "$planted.ok")"
  scripts_dir="${PR_MONITOR_TEST_SCRIPTS_DIR:-$HERE}"
  n=0; found=""
  for s in "$scripts_dir"/*.sh; do
    case "$s" in (*.test.sh) continue ;; esac
    [[ -f "$s" ]] || continue
    n=$((n + 1))
    hit=$(unpinned_mints "$s")
    [[ -n "$hit" ]] && found="$found $(basename "$s"):$hit"
  done
  check "$([[ "$n" -gt 0 && -z "$found" ]] && echo 0 || echo 1)" \
    "no shipped script mints for gh without --forge github ($n scanned)" "unpinned:$found"
fi

# ============================================================== role-mapping ==
# regression: #3116
if want role-mapping; then
  echo "role-mapping — \$DESK_LOOP maps to the App role exactly as the compiled loop-to-role table does"
  for pair in the-desk:desk worker-desk:worker batch-fanout:worker pr-review-desk:reviewer \
              verify-desk:verifier intake-desk:issue-loop issue-loop:issue-loop; do
    loop="${pair%%:*}"; role="${pair#*:}"
    w="$TMPROOT/rm"; st="$TMPROOT/rm-st"; rm -rf "$w" "$st"; make_gh "$w"
    resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
    run "$w" "$st" "DESK_LOOP=$loop" -- o/r
    check "$([[ "$(cat "$w/mint.log")" == "$role --repo o/r" ]] && echo 0 || echo 1)" \
      "$loop mints as role '$role'" "mints: $(tr '\n' ';' < "$w/mint.log")"
  done
fi

if want classified-version-hold; then
  echo "classified-version-hold — unsupported producer never falls back or retains a reusable artifact"
  w="$TMPROOT/classified"; st="$TMPROOT/classified-st"; make_gh "$w"
  for tool in deskboard reviewloop; do
    cat > "$w/$tool" <<'STUB'
#!/bin/sh
printf 'tool releaseTag=v1.0.58\n'
STUB
    chmod +x "$w/$tool"
  done
  artifact="$w/review.json"
  printf '{"prior":"valid-looking"}' > "$artifact"
  run "$w" "$st" "PR_MONITOR_REVIEW_OBSERVATION=$artifact" 'PR_MONITOR_REVIEW_POLICY={}' PR_MONITOR_REVIEW_CAPACITY=1 -- o/r
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "unsupported selected producer holds the cycle" "$OUT"
  check "$(contains "$OUT" 'requires released v1.0.59' && echo 0 || echo 1)" "hold gives the supported producer recovery" "$OUT"
  check "$([[ ! -s "$artifact" && ! -s "$w/calls.log" && ! -s "$w/mint.log" ]] && echo 0 || echo 1)" "hold invalidates output without legacy GH or credential calls" "$OUT"
  # Explicit rollback on a later cycle chooses only the established recipe.
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" PR_MONITOR_REVIEW_OBSERVATION= -- o/r
  check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "legacy rollback recovers full monitor population" "$OUT"
fi

if want classified-invalidation-failure; then
  echo "classified-invalidation-failure — output IO failures still report a held, non-reusable cycle"
  for fault in mktemp mv; do
    w="$TMPROOT/invalidation-$fault"; st="$TMPROOT/invalidation-$fault-st"; make_gh "$w"
    for tool in deskboard reviewloop; do
      cat > "$w/$tool" <<'STUB'
#!/bin/sh
printf 'tool releaseTag=v1.0.58\n'
STUB
      chmod +x "$w/$tool"
    done
    cat > "$w/$fault" <<'STUB'
#!/bin/sh
exit 1
STUB
    chmod +x "$w/$fault"
    artifact="$w/review.json"
    printf 'prior-observation' > "$artifact"
    printf 'prior-plan' > "$artifact.plan"
    run "$w" "$st" "PR_MONITOR_REVIEW_OBSERVATION=$artifact" 'PR_MONITOR_REVIEW_POLICY={}' PR_MONITOR_REVIEW_CAPACITY=1 -- o/r
    check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "$fault failure exits held" "$OUT"
    check "$(contains "$OUT" 'MONITOR-DEGRADED: complete review cycle held' && echo 0 || echo 1)" "$fault failure reports monitor degradation" "$OUT"
    check "$(contains "$OUT" 'artifact invalidation failed' && contains "$OUT" 'must not be reused' && echo 0 || echo 1)" "$fault failure explicitly rejects surviving artifacts" "$OUT"
    check "$([[ "$(cat "$artifact")" == prior-observation && "$(cat "$artifact.plan")" == prior-plan && ! -s "$w/calls.log" && ! -s "$w/mint.log" ]] && echo 0 || echo 1)" "$fault failure retains data safely and makes no legacy read" "$OUT"
  done
fi

# regression: medici-finance/assay#4311
if want classified-unwritable-hold; then
  echo "classified-unwritable-hold — a real unwritable output directory still reports a held, non-reusable cycle"
  w="$TMPROOT/classified-ro"; st="$TMPROOT/classified-ro-st"; make_gh "$w"
  for tool in deskboard reviewloop; do
    cat > "$w/$tool" <<'STUB'
#!/bin/sh
printf 'tool releaseTag=v1.0.58\n'
STUB
    chmod +x "$w/$tool"
  done
  ro="$w/ro"; mkdir -p "$ro"; artifact="$ro/review.json"
  printf '{"prior":"valid-looking"}' > "$artifact"
  chmod 555 "$ro"
  run "$w" "$st" "PR_MONITOR_REVIEW_OBSERVATION=$artifact" 'PR_MONITOR_REVIEW_POLICY={}' PR_MONITOR_REVIEW_CAPACITY=1 -- o/r
  chmod 755 "$ro"
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "unwritable artifact directory still holds the cycle" "$OUT"
  check "$(contains "$OUT" 'MONITOR-DEGRADED: complete review cycle held' && echo 0 || echo 1)" \
    "the hold line prints even when no invalidating file can be made" "$OUT"
  check "$(contains "$OUT" 'artifact invalidation failed' && contains "$OUT" 'must not be reused' && echo 0 || echo 1)" \
    "a surviving prior artifact is explicitly rejected, never left silently" "$OUT"
  check "$([[ ! -s "$w/calls.log" && ! -s "$w/mint.log" ]] && echo 0 || echo 1)" \
    "the hold makes no legacy GH or credential call" "$OUT"
fi

# ======================================================== forge-gitlab-seed ===
# regression: #5889
# A roster that maps the repo to a non-GitHub forge: the minter, left to follow the
# roster, hands back that forge's credential, and `gh` (GitHub only) answers 401.
# The script's own reader must ask for a GitHub credential explicitly, so such a repo
# is could-not-check at the mint — never a foreign credential sent to GitHub, never
# a stored baseline, never an empty queue.
if want forge-gitlab-seed; then
  echo "forge-gitlab-seed — a gitlab-mapped repo never sends its credential to gh; could-not-check, no baseline"
  w="$TMPROOT/fgl"; st="$TMPROOT/fgl-st"; make_gh "$w"
  : > "$w/mintargs.log"
  # desktoken that follows ASSAY_REPO_FORGES the way the real minter does: a repo
  # mapped to gitlab gets a PAT-shaped placeholder unless --forge github is asked
  # for, and --forge github on that repo is refused (no App installation there).
  cat > "$w/desktoken" <<STUB
#!/usr/bin/env bash
d="$w"
printf '%s\n' "\$*" >> "\$d/mintargs.log"
role="\${1:-}"; repo=""; forge=""; prev=""
for a in "\$@"; do
  [ "\$prev" = "--repo" ] && repo="\$a"; [ "\$prev" = "--forge" ] && forge="\$a"; prev="\$a"
done
san=\$(printf '%s' "\$repo" | sed 's#/#__#g')
printf '%s --repo %s\n' "\$role" "\$repo" >> "\$d/mint.log"
mapped=""
case ",\${ASSAY_REPO_FORGES:-}," in (*",\$repo=gitlab,"*) mapped=gitlab ;; esac
if [ "\$mapped" = gitlab ]; then
  if [ "\$forge" = github ]; then
    echo "desktoken: no GitHub App installation for \$repo" >&2; exit 5
  fi
  printf 'x-gitlab-pat-%s\n' "\$san" > "\$d/tok/\$san"
else
  printf 'x-minted-%s\n' "\$san" > "\$d/tok/\$san"
fi
chmod 600 "\$d/tok/\$san"; printf '%s\n' "\$d/tok/\$san"; exit 0
STUB
  chmod +x "$w/desktoken"
  # gh that refuses any non-GitHub credential with the forge's 401.
  cat > "$w/gh" <<STUB
#!/usr/bin/env bash
d="$w"
repo=""; prev=""
for a in "\$@"; do [ "\$prev" = "--repo" ] && repo="\$a"; prev="\$a"; done
printf '%s %s\n' "\$(date +%s)" "\$repo" >> "\$d/calls.log"
printf 'GH_TOKEN=[%s]\n' "\${GH_TOKEN:-}" >> "\$d/token.log"
case "\${GH_TOKEN:-}" in (x-gitlab-pat-*) echo "HTTP 401: Bad credentials (https://api.github.com/graphql)" >&2; exit 1 ;; esac
printf '[{"number":1,"headRefOid":"aaa","isDraft":false,"state":"OPEN","mergeStateStatus":"CLEAN"}]'
STUB
  chmod +x "$w/gh"
  run "$w" "$st" ASSAY_REPO_FORGES=o/r=gitlab -- o/r
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a gitlab-mapped repo exits 2 (degraded)" "rc=$RC stdout: ${OUT:-<empty>}"
  check "$(grep -q 'x-gitlab-pat' "$w/token.log" && echo 1 || echo 0)" \
    "no gh read ever carries the non-GitHub credential" "token.log: $(cat "$w/token.log")"
  check "$(grep -q -- '--forge github' "$w/mintargs.log" && echo 0 || echo 1)" \
    "the script's own reader asks the minter for a GitHub credential" "mint args: $(cat "$w/mintargs.log")"
  check "$(grep -q '^MONITOR-DEGRADED: o/r token mint FAILED' <<<"$OUT" && echo 0 || echo 1)" \
    "the repo is reported could-not-check at the mint, not as a 401 read" "stdout: ${OUT:-<empty>}"
  check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" "no gh call was made" "calls: $(cat "$w/calls.log")"
  check "$([[ ! -e "$st/o__r.state" ]] && echo 0 || echo 1)" \
    "no baseline is stored for the unread repo" "state: $(cat "$st/o__r.state" 2>/dev/null)"
  check "$(grep -q 'MONITOR-ARMED' <<<"$OUT" && echo 1 || echo 0)" \
    "the unread repo is never reported as an armed (empty) queue" "stdout: ${OUT:-<empty>}"
  # A github-mapped neighbour in the same cycle still reads and seeds.
  rm -rf "$st"; : > "$w/calls.log"; : > "$w/token.log"
  run "$w" "$st" ASSAY_REPO_FORGES=o/r=gitlab,o/g=github -- o/r o/g
  check "$([[ "$RC" -eq 2 && -s "$st/o__g.state" && ! -e "$st/o__r.state" ]] && echo 0 || echo 1)" \
    "a github-mapped repo in the same cycle still seeds; the gitlab one stays unread" \
    "rc=$RC stdout: ${OUT:-<empty>}"
fi

# ================================================== forge-aware-delegation ===
# regression: #5889
# With `deskmonitor` installed, the legacy cycle runs on `deskmonitor pr` — the
# reader that resolves each repo's forge through the shared Go resolver. Its
# 0/1/2 pass through untouched; any other exit is could-not-check, never quiet.
if want forge-aware-delegation; then
  echo "forge-aware-delegation — legacy mode hands the cycle to deskmonitor pr; out-of-contract exits are degraded"
  w="$TMPROOT/fad"; st="$TMPROOT/fad-st"; make_gh "$w"
  cat > "$w/deskmonitor" <<STUB
#!/usr/bin/env bash
d="$w"
printf '%s\n' "\$*" >> "\$d/dm.log"
printf 'STATE=%s LOOP=%s\n' "\${PR_MONITOR_STATE_DIR:-}" "\${DESK_LOOP:-}" >> "\$d/dm-env.log"
rc=\$(cat "\$d/dm.rc" 2>/dev/null || echo 0)
[ "\$rc" = 0 ] && echo "MONITOR-ARMED: 1 repo(s) (stub)"
[ "\$rc" = 2 ] && echo "MONITOR-DEGRADED: o/r seed read FAILED (stub)"
exit "\$rc"
STUB
  chmod +x "$w/deskmonitor"
  for want_rc in 0 1 2; do
    printf '%s' "$want_rc" > "$w/dm.rc"; : > "$w/dm.log"
    run "$w" "$st" ASSAY_REPO_FORGES=o/r=gitlab PR_MONITOR_LIMIT=7 -- o/r o/s
    check "$([[ "$RC" -eq "$want_rc" && "$(cat "$w/dm.log")" == "pr o/r o/s" ]] && echo 0 || echo 1)" \
      "deskmonitor exit $want_rc passes through; called as 'pr <args>'" "rc=$RC args=$(cat "$w/dm.log")"
  done
  check "$([[ ! -s "$w/calls.log" && ! -s "$w/mint.log" ]] && echo 0 || echo 1)" \
    "the handed-off cycle makes no gh read and no mint of its own" "calls=$(cat "$w/calls.log") mints=$(cat "$w/mint.log")"
  check "$(grep -q "STATE=$st LOOP=pr-review-desk" "$w/dm-env.log" && echo 0 || echo 1)" \
    "deskmonitor sees the same state dir and loop" "env: $(cat "$w/dm-env.log")"
  # An inactive verb (exit 6) or a crash prints nothing the watch filter matches.
  for bad_rc in 6 127; do
    printf '%s' "$bad_rc" > "$w/dm.rc"
    run "$w" "$st" -- o/r
    check "$([[ "$RC" -eq 2 ]] && grep -q '^MONITOR-DEGRADED: .*could-not-check' <<<"$OUT" && echo 0 || echo 1)" \
      "deskmonitor exit $bad_rc is MONITOR-DEGRADED could-not-check, exit 2" "rc=$RC stdout: ${OUT:-<empty>}"
  done
  # Qualified mode never hands off (its one collector is review-queue).
  : > "$w/dm.log"; printf '0' > "$w/dm.rc"
  run "$w" "$st" "PR_MONITOR_REVIEW_OBSERVATION=$w/obs.json" 'PR_MONITOR_REVIEW_POLICY={}' PR_MONITOR_REVIEW_CAPACITY=1 -- o/r
  check "$([[ ! -s "$w/dm.log" ]] && echo 0 || echo 1)" \
    "qualified mode never calls deskmonitor" "dm.log: $(cat "$w/dm.log")"
fi

echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
