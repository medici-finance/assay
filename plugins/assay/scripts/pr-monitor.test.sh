#!/usr/bin/env bash
# pr-monitor.test.sh — the proof that pr-monitor.sh emits the right PR-EVENT for
# every change kind, seeds silently, retains a truncated read, and — the new
# properties — PACES its reads and STOPS the cycle the moment the forge signals a
# secondary rate limit.
#
# No network and no token: `gh` is stubbed on PATH per case, so the suite is
# hermetic and runs in CI. The stub records every invocation (timestamp + repo)
# to a call log, so pacing and the stop-on-limit rule are asserted against what
# the script ACTUALLY called, not what it printed.
#
# Run all cases:            bash pr-monitor.test.sh
# Run one named case:       bash pr-monitor.test.sh --case <name>
#   cases: seed-silent opened pushed draft-flip state closed push-and-flip
#          truncation rate-limit-stops-cycle pacing-honoured portability
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/pr-monitor.sh"

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
  mkdir -p "$dir/resp" "$dir/fail"
  : > "$dir/calls.log"
  : > "$dir/token.log"
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
}
san() { printf '%s' "$1" | sed 's#/#__#g'; }
resp() { printf '%s' "$3" > "$1/resp/$(san "$2").json"; }          # <dir> <repo> <json>
fail_ratelimit() { # <dir> <repo>
  printf 'HTTP 403: You have exceeded a secondary rate limit. Please wait a few minutes before you try again.' \
    > "$1/fail/$(san "$2")"
}
clear_fail() { rm -f "$1/fail/$(san "$2")"; }

# run <ghdir> <statedir> [ENV=VAL ...] -- <args...> — sets OUT/RC. Defaults to
# ASSAY_MONITOR_PACE_SECONDS=0 for speed unless the case overrides it.
run() {
  local ghdir="$1" statedir="$2"; shift 2
  local envs=(ASSAY_MONITOR_PACE_SECONDS=0)
  while [[ "${1:-}" != "--" && $# -gt 0 ]]; do envs+=("$1"); shift; done
  shift
  OUT=$(env "${envs[@]}" PR_MONITOR_STATE_DIR="$statedir" \
        PATH="$ghdir:$PATH" bash "$SCRIPT" "$@" 2>"$ghdir/stderr")
  RC=$?
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

echo "pr-monitor regression suite ($(bash --version | head -1))"
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
  # It must scrub an inherited App token exactly as the inbound monitor does.
  w="$TMPROOT/tok"; st="$TMPROOT/tok-st"; make_gh "$w"
  resp "$w" o/r "$(arr "$(pr 1 aaa false OPEN CLEAN)")"
  run "$w" "$st" GH_TOKEN=x-bogus-app-token GITHUB_TOKEN=x-bogus-app-token-2 -- o/r
  tokline=$(head -1 "$w/token.log" 2>/dev/null)
  check "$([[ "$tokline" == "GH_TOKEN=[] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
    "gh is invoked with GH_TOKEN and GITHUB_TOKEN unset" \
    "gh saw: ${tokline:-<no invocation recorded>}"
fi

echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
