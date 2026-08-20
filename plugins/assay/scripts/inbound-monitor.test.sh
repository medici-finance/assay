#!/usr/bin/env bash
# inbound-monitor.test.sh — the proof that inbound-monitor.sh catches every way a
# desk monitor was going SILENTLY BLIND. Every case is
# reproduced against the pre-fix behaviour first and FAILS without its fix.
#
#   A  IDENTITY: a monitor armed from a shell that exported an App GH_TOKEN
#      polled as the App, whose install token 404s the private repo set — and
#      that 404 is byte-identical to "no open issues" once stderr is dropped.
#      A1 pins that the script UNSETS GH_TOKEN/GITHUB_TOKEN so `gh` never sees
#      the inherited App token at all.
#   B  RETENTION: the exact three-step scenario from the issue's own "Verified,
#      not asserted" table — seed, a poll where one repo's read FAILS, then a
#      recovery poll — proving (B1) the failing repo RETAINS its baseline and
#      goes loud, and (B2) recovery emits NO phantom INBOUND. Pre-fix, step 3
#      emitted every retained issue as new.
#      B3 pins the sibling blinding path: a "successful" read that returns ZERO
#      where the repo previously had issues is also retained, not trusted.
#   D  TRUNCATION: a read that comes back == --limit is a moving window `gh`
#      gives no signal for. D1 proves an at-limit read on an established repo is
#      retained (real baseline kept, no phantom INBOUND on the shifted window);
#      D2 proves a truncated SEED establishes no baseline at all.
#   E  PARTIAL LOSS: a "successful", non-empty read that collapses below the
#      retain floor is the middle of the range the zero-check misses. E proves it
#      is retained + loud, the recovery cycle absorbs it, and the floor is
#      off-switchable via INBOUND_MONITOR_RETAIN_FLOOR=0.
#   C  BURST CAP: a mass update collapses to one INBOUND-BURST line.
#   plus: MONITOR-ARMED on first arm, quiet on a no-change poll, real INBOUND on
#   a genuine new issue, and a repo added after arming seeds SILENTLY.
#
# No network and no token: `gh` is stubbed on PATH per case, so the suite is
# hermetic and runs in CI. Run:  bash plugins/assay/scripts/inbound-monitor.test.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/inbound-monitor.sh"

pass=0
fail=0

ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }
check() { if [[ "$1" -eq 0 ]]; then ok "$2"; else no "$2" "$3"; fi; }

contains() { case "$1" in (*"$2"*) return 0 ;; esac; return 1; }

# issue_json <n> [start] — an n-element open-issue array (number,updatedAt), each
# numbered from start (default 100).
issue_json() {
  jq -nc --argjson n "$1" --argjson s "${2:-100}" \
    '[range($n) | {number: ($s + .), updatedAt: "2026-01-01T00:00:00Z"}]'
}

# make_gh <dir> <exit_code> <stdout_payload> <stderr_payload>
# A stub `gh` that also records the GH_TOKEN/GITHUB_TOKEN it was invoked with —
# so a test can prove the script scrubbed its identity before calling.
make_gh() {
  local dir="$1" code="$2" out="$3" err="$4"
  mkdir -p "$dir"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'printf "GH_TOKEN=[%%s] GITHUB_TOKEN=[%%s]\\n" "${GH_TOKEN:-}" "${GITHUB_TOKEN:-}" >> %s\n' \
      "$(printf '%q' "$dir/token.log")"
    if [[ -n "$err" ]]; then printf 'printf "%%s\\n" %s >&2\n' "$(printf '%q' "$err")"; fi
    if [[ -n "$out" ]]; then printf 'printf "%%s" %s\n' "$(printf '%q' "$out")"; fi
    printf 'exit %s\n' "$code"
  } > "$dir/gh"
  chmod +x "$dir/gh"
}

# run_case <ghdir> <statedir> [env KEY=VAL ...] -- <args...> — sets RC/OUT/ERR.
run_case() {
  local ghdir="$1" statedir="$2"; shift 2
  local envs=()
  while [[ "${1:-}" != "--" && $# -gt 0 ]]; do envs+=("$1"); shift; done
  shift  # drop the --
  OUT=$(env "${envs[@]}" INBOUND_MONITOR_STATE_DIR="$statedir" \
        PATH="$ghdir:$PATH" bash "$SCRIPT" "$@" 2>"$ghdir/stderr")
  RC=$?
  ERR=$(cat "$ghdir/stderr")
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

echo "inbound-monitor regression suite ($(bash --version | head -1))"

# ================================================================ A =========
echo "A — the monitor must scrub an inherited App token, not poll as it"

w="$TMPROOT/a-gh"; st="$TMPROOT/a-state"
make_gh "$w" 0 "$(issue_json 3)" ""
# Arm the monitor from a shell that exported a bogus App token, exactly as desk
# work does when it exports a role token into GH_TOKEN. The value only has to be
# non-empty for the scrub assertion; a placeholder that wears NO real credential
# prefix keeps a token-shaped string out of the public tree.
run_case "$w" "$st" GH_TOKEN=x-bogus-app-token GITHUB_TOKEN=x-bogus-app-token-2 -- o/r
tokline=$(cat "$w/token.log" 2>/dev/null | head -1)
check "$([[ "$tokline" == "GH_TOKEN=[] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
  "gh is invoked with GH_TOKEN and GITHUB_TOKEN unset" \
  "gh saw: ${tokline:-<no invocation recorded>}"
check "$(contains "$OUT" "MONITOR-ARMED: 3" && echo 0 || echo 1)" \
  "arm still succeeds on the keyring identity" "stdout: ${OUT:-<empty>}"

# ================================================================ B =========
echo "B — the issue's own three-step table: seed, one repo fails, recover"

# Two repos; o/dominant dominates (94), the other is small (2). A global
# 'too few records' floor would never notice o/dominant vanishing — that is
# the whole point of per-source state.
w="$TMPROOT/b-gh"; st="$TMPROOT/b-state"
mkdir -p "$w"
# A stub whose o/dominant leg can be flipped to fail via a marker file.
cat > "$w/gh" <<STUB
#!/usr/bin/env bash
printf 'GH_TOKEN=[%s] GITHUB_TOKEN=[%s]\n' "\${GH_TOKEN:-}" "\${GITHUB_TOKEN:-}" >> "$w/token.log"
repo=""
while [ \$# -gt 0 ]; do
  case "\$1" in --repo) shift; repo="\$1" ;; esac
  shift
done
case "\$repo" in
  o/dominant)
    if [ -f "$w/FAIL_AT" ]; then
      echo "GraphQL: Could not resolve to a Repository with the name 'o/dominant'." >&2
      exit 1
    fi
    jq -nc '[range(94) | {number: (1000 + .), updatedAt: "2026-07-12T00:00:00Z"}]' ;;
  *)
    jq -nc '[range(2) | {number: (10 + .), updatedAt: "2026-01-01T00:00:00Z"}]' ;;
esac
STUB
chmod +x "$w/gh"

# Step 1 — SEED (real gh). MONITOR-ARMED: 96, o/dominant state = 94 lines.
run_case "$w" "$st" -- o/dominant o/small
seedstate="$st/o__dominant.state"
check "$(contains "$OUT" "MONITOR-ARMED: 96" && echo 0 || echo 1)" \
  "seed prints MONITOR-ARMED across both repos" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$seedstate")" == "94" ]] && echo 0 || echo 1)" \
  "o/dominant state seeded with 94 lines" "lines: $(grep -c . "$seedstate" 2>/dev/null)"

# Step 2 — POLL with o/dominant's read FAILING. It must RETAIN 94 and go loud.
touch "$w/FAIL_AT"
run_case "$w" "$st" -- o/dominant o/small
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a degraded cycle exits 2 (loud)" "got $RC"
check "$(contains "$OUT" "MONITOR-DEGRADED: o/dominant" && echo 0 || echo 1)" \
  "the failing repo names itself in a MONITOR-DEGRADED line" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "keeping its previous 94" && echo 0 || echo 1)" \
  "the degraded line states it retained the 94" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$seedstate")" == "94" ]] && echo 0 || echo 1)" \
  "state is still 94, NOT rewritten to 0" "lines: $(grep -c . "$seedstate" 2>/dev/null)"
check "$(contains "$OUT" "INBOUND:" && echo 1 || echo 0)" \
  "a failed read emits no INBOUND lines" "stdout: ${OUT:-<empty>}"

# Step 3 — RECOVERY poll (real gh). The outage must be ABSORBED: NO INBOUND.
rm -f "$w/FAIL_AT"
run_case "$w" "$st" -- o/dominant o/small
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "recovery cycle exits 0" "got $RC"
check "$(contains "$OUT" "INBOUND" && echo 1 || echo 0)" \
  "recovery emits NO phantom INBOUND (the outage was absorbed)" \
  "stdout leaked phantom inbound: ${OUT:-<empty>}"

# ---- B3: a 'successful' read that returns ZERO where it had issues ----------
echo "B3 — a repo that returns 0 where it had issues is retained, not trusted"
w="$TMPROOT/b3-gh"; st="$TMPROOT/b3-state"
make_gh "$w" 0 "$(issue_json 5)" ""
run_case "$w" "$st" -- o/r                          # seed 5
make_gh "$w" 0 '[]' ""                              # now returns empty, exit 0
run_case "$w" "$st" -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "empty-where-it-had-issues => exit 2" "got $RC"
check "$(contains "$OUT" "returned 0 (had 5)" && echo 0 || echo 1)" \
  "a silent empty read is caught and named" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$st/o__r.state")" == "5" ]] && echo 0 || echo 1)" \
  "the 5 lines are retained, not zeroed" "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"

# ---- D: an AT-LIMIT read is TRUNCATED — could-not-check, not ground truth ----
echo "D — a read that comes back == --limit is truncated: retain + degrade"
# D1: at-limit on an ESTABLISHED repo retains the real baseline and goes loud,
# and — critically — fires NO phantom INBOUND when the (shifted) window returns
# keys the baseline never held.
w="$TMPROOT/d-gh"; st="$TMPROOT/d-state"
make_gh "$w" 0 "$(issue_json 3)" ""                  # seed 3 (< limit 5)
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
make_gh "$w" 0 "$(issue_json 5 900)" ""              # 5 == limit, a shifted window
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "an at-limit read => exit 2" "got $RC"
check "$(contains "$OUT" "== --limit 5" && contains "$OUT" "TRUNCATED" && echo 0 || echo 1)" \
  "the truncation is named as could-not-check" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "INBOUND:" && echo 1 || echo 0)" \
  "a truncated read fires no phantom INBOUND on the shifted window" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$st/o__r.state")" == "3" ]] && echo 0 || echo 1)" \
  "the real 3-line baseline is retained, not overwritten by the slice" \
  "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"

# D2: at-limit on the SEED establishes NO baseline (a truncated seed would bake a
# moving window in as if it were the whole repo).
w="$TMPROOT/d2-gh"; st="$TMPROOT/d2-state"
make_gh "$w" 0 "$(issue_json 5)" ""                  # 5 == limit on the very first read
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a truncated seed => exit 2" "got $RC"
check "$(contains "$OUT" "seed returned 5 == --limit 5" && echo 0 || echo 1)" \
  "the truncated seed is named" "stdout: ${OUT:-<empty>}"
check "$([[ ! -f "$st/o__r.state" ]] && echo 0 || echo 1)" \
  "no baseline file is written for a truncated seed" "a state file was written"

# ---- E: a PARTIAL read (below the retain floor) is retained, not trusted -----
echo "E — a successful read that collapses below the floor is retained + loud"
# The middle of the range the bare zero-check leaves open. Seed 94, then a read
# returns only 3 (a different, shifted window). Trust it and the recovery cycle
# floods 94 stale items as inbound; retain it and the outage is absorbed.
w="$TMPROOT/e-gh"; st="$TMPROOT/e-state"
make_gh "$w" 0 "$(issue_json 94)" ""                 # seed 94
run_case "$w" "$st" -- o/r
make_gh "$w" 0 "$(issue_json 3 900)" ""              # 3 keys, none in the baseline
run_case "$w" "$st" -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a below-floor read => exit 2" "got $RC"
check "$(contains "$OUT" "under 50% of its previous 94" && echo 0 || echo 1)" \
  "the partial read is named against its previous count" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "INBOUND:" && echo 1 || echo 0)" \
  "a partial read fires no INBOUND (the 3 shifted keys are not surfaced)" \
  "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$st/o__r.state")" == "94" ]] && echo 0 || echo 1)" \
  "the 94-line baseline is retained, not overwritten by the 3-line slice" \
  "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"
# Recovery: the real set returns and the outage is ABSORBED — no phantom flood.
make_gh "$w" 0 "$(issue_json 94)" ""
run_case "$w" "$st" -- o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the recovery cycle exits 0" "got $RC"
check "$(contains "$OUT" "INBOUND" && echo 1 || echo 0)" \
  "recovery emits NO phantom INBOUND (the partial read was absorbed)" \
  "stdout leaked phantom inbound: ${OUT:-<empty>}"
# And the floor is OFF-switchable: with RETAIN_FLOOR=0 the same drop passes as a
# healthy read (the zero-check and at-limit guard still stand).
w="$TMPROOT/e0-gh"; st="$TMPROOT/e0-state"
make_gh "$w" 0 "$(issue_json 94)" ""
run_case "$w" "$st" INBOUND_MONITOR_RETAIN_FLOOR=0 -- o/r
make_gh "$w" 0 "$(issue_json 3 900)" ""
run_case "$w" "$st" INBOUND_MONITOR_RETAIN_FLOOR=0 -- o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "RETAIN_FLOOR=0 disables the proportional floor" "got $RC"

# ================================================================ C =========
echo "C — a mass update collapses to one INBOUND-BURST line"
w="$TMPROOT/c-gh"; st="$TMPROOT/c-state"
make_gh "$w" 0 "$(issue_json 2)" ""
run_case "$w" "$st" -- o/r                          # seed 2
make_gh "$w" 0 "$(issue_json 40)" ""                # 38 new, over the 25 cap
run_case "$w" "$st" -- o/r
burstlines=$(printf '%s\n' "$OUT" | grep -c '^INBOUND-BURST:')
inboundlines=$(printf '%s\n' "$OUT" | grep -c '^INBOUND:')
check "$(contains "$OUT" "INBOUND-BURST: o/r 38 over 25 — listing suppressed" && echo 0 || echo 1)" \
  "a >cap burst prints one INBOUND-BURST line" "stdout: ${OUT:-<empty>}"
check "$([[ "$inboundlines" -eq 0 ]] && echo 0 || echo 1)" \
  "the individual INBOUND lines are suppressed under a burst" "INBOUND lines: $inboundlines"

# ============================================================ basics =========
echo "Basics — arm, quiet, a genuine new issue, and silent seed on expansion"

# Quiet: a no-change poll prints no INBOUND and exits 0.
w="$TMPROOT/q-gh"; st="$TMPROOT/q-state"
make_gh "$w" 0 "$(issue_json 3)" ""
run_case "$w" "$st" -- o/r                          # arm
run_case "$w" "$st" -- o/r                          # identical poll
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "an unchanged poll exits 0" "got $RC"
check "$(contains "$OUT" "INBOUND" && echo 1 || echo 0)" \
  "an unchanged poll is quiet (no INBOUND, no ARMED)" "stdout: ${OUT:-<empty>}"

# A genuine new issue after arming fires exactly one INBOUND.
make_gh "$w" 0 "$(issue_json 4)" ""                 # one more issue (num 103)
run_case "$w" "$st" -- o/r
newcount=$(printf '%s\n' "$OUT" | grep -c '^INBOUND: o/r#103')
check "$([[ "$newcount" -eq 1 ]] && echo 0 || echo 1)" \
  "a genuinely new issue fires exactly one INBOUND" "stdout: ${OUT:-<empty>}"

# A repo added to an already-armed set seeds SILENTLY — never re-floods.
w="$TMPROOT/x-gh"; st="$TMPROOT/x-state"
make_gh "$w" 0 "$(issue_json 3)" ""
run_case "$w" "$st" -- o/r                          # arm with one repo
make_gh "$w" 0 "$(issue_json 3)" ""
run_case "$w" "$st" -- o/r o/added                  # add a second repo
check "$(contains "$OUT" "MONITOR-ARMED" && echo 1 || echo 0)" \
  "adding a repo does not re-print MONITOR-ARMED" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "INBOUND" && echo 1 || echo 0)" \
  "the newly added repo seeds silently (no INBOUND flood)" "stdout: ${OUT:-<empty>}"
check "$([[ -f "$st/o__added.state" ]] && echo 0 || echo 1)" \
  "the added repo got its own state file" "no state file for o/added"

# ============================================================ preconditions ==
echo "Preconditions — no repos, invalid token"
w="$TMPROOT/p-gh"; st="$TMPROOT/p-state"
make_gh "$w" 0 '[]' ""
mkdir -p "$w/norepo"
OUT=$(cd "$w/norepo" && INBOUND_MONITOR_STATE_DIR="$st" PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
ERR=$(cat "$w/stderr")
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "no resolvable repos => exit 1" "got $RC"
check "$(contains "$ERR" "unbound variable" && echo 1 || echo 0)" \
  "the no-repo path is bash-3.2 safe under set -u" "set -u tripped: $ERR"

run_case "$w" "$st" -- 'o/r;touch pwned'
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "invalid repo token => exit 1" "got $RC"
check "$([[ ! -e "$w/pwned" && ! -e pwned ]] && echo 0 || echo 1)" \
  "the token never reaches a shell" "pwned marker created"

# ============================================================ portability ====
echo "Portability — bash 3.2 (stock /bin/bash on macOS)"
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

echo
printf '%s passed, %s failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]] || exit 1
