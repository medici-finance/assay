#!/usr/bin/env bash
# inbound-monitor.test.sh — the proof that inbound-monitor.sh catches every way a
# desk monitor was going SILENTLY BLIND. Every case is
# reproduced against the pre-fix behaviour first and FAILS without its fix.
#
#   A  IDENTITY: a monitor armed from a shell that exported an App GH_TOKEN
#      polled as the App, whose install token 404s the private repo set — and
#      that 404 is byte-identical to "no open issues" once stderr is dropped.
#      A1 pins that `gh` never sees the inherited App token at all: it sees the
#      repo's own minted role token instead.
#   B  RETENTION: the exact three-step scenario from the issue's own "Verified,
#      not asserted" table — seed, a poll where one repo's read FAILS, then a
#      recovery poll — proving (B1) the failing repo RETAINS its baseline and
#      goes loud, and (B2) recovery emits NO phantom INBOUND. Pre-fix, step 3
#      emitted every retained issue as new.
#      B3 pins the sibling blinding path: a "successful" read that returns ZERO
#      where the repo previously had issues is also retained, not trusted.
#   D  TRUNCATION: a read holding MORE than the --limit ceiling is a moving
#      window `gh` gives no signal for (the script asks for one row past the
#      ceiling; that extra row is the signal). D1 proves a read past the ceiling
#      on an established repo is retained (real baseline kept, no phantom INBOUND
#      on the shifted window); D2 proves a truncated SEED establishes no baseline
#      at all; D3 proves a set of EXACTLY the ceiling is the whole set — it seeds
#      and diffs — and that `gh` is asked for one row past the ceiling.
#   E  PARTIAL LOSS: a "successful", non-empty read that collapses below the
#      retain floor is the middle of the range the zero-check misses. E proves it
#      is retained + loud, the recovery cycle absorbs it, and the floor is
#      off-switchable via INBOUND_MONITOR_RETAIN_FLOOR=0.
#   F  EXPLICIT READ IDENTITY: under a HOME whose `gh` keyring holds no usable
#      account (a desk cell's), an ambient read 401s every repo. F1 proves
#      `--token-file OWNER=PATH` reads clean there; F2 that without one the
#      read is the minted role token (not the keyring), DEGRADED when that token
#      is refused; F3 that the file is scoped to its owner; F4 that an unusable
#      file is a precondition failure, never a silent fallback.
#   G  PER-REPO MINTED ROLE TOKEN: the read identity is `desktoken <role> --repo
#      <slug>`, minted INSIDE the poll and scoped to that one read. G proves one
#      mint per repo under the loop's role, that a failed/unusable mint (or an
#      unset/unknown $DESK_LOOP) is DEGRADED for that repo with no `gh` call and
#      no ambient fallback, and that recovery emits no phantom INBOUND.
#   C  BURST CAP: a mass update collapses to one INBOUND-BURST line.
#   plus: MONITOR-ARMED on first arm, quiet on a no-change poll, real INBOUND on
#   a genuine new issue, and a repo added after arming seeds SILENTLY.
#
# No network and no token: `gh` AND `desktoken` are stubbed on PATH per case, so
# the suite is hermetic and runs in CI. Run:  bash plugins/assay/scripts/inbound-monitor.test.sh
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
    printf 'printf "%%s\\n" "$*" >> %s\n' "$(printf '%q' "$dir/args.log")"
    if [[ -n "$err" ]]; then printf 'printf "%%s\\n" %s >&2\n' "$(printf '%q' "$err")"; fi
    if [[ -n "$out" ]]; then printf 'printf "%%s" %s\n' "$(printf '%q' "$out")"; fi
    printf 'exit %s\n' "$code"
  } > "$dir/gh"
  chmod +x "$dir/gh"
}

# ensure_desktoken <dir> — a stub `desktoken <role> --repo <slug>` in <dir>: records
# each mint to <dir>/mint.log and prints the PATH of <dir>/tok/<owner__name>, a 0600
# file holding x-minted-<owner__name> (a placeholder that wears no real credential
# prefix). Per-repo markers select a failure:
#   <dir>/mintfail/<san>    stderr text, exit 5 (a refused mint)
#   <dir>/mintblank/<san>   exit 0, path to a whitespace-only file
#   <dir>/mintnopath/<san>  exit 0, prints nothing
#   <dir>/mintrace/<san>    a good token, then another session's re-mint TRUNCATES the
#                           shared cache file 0.3s later (desktoken rewrites it in place)
ensure_desktoken() {
  local dir="$1"
  mkdir -p "$dir" "$dir/tok" "$dir/mintfail" "$dir/mintblank" "$dir/mintnopath" "$dir/mintrace"
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
  [[ -f "$dir/mint.log" ]] || : > "$dir/mint.log"
}
san() { printf '%s' "$1" | sed 's#/#__#g'; }

# run_case <ghdir> <statedir> [env KEY=VAL ...] -- <args...> — sets RC/OUT/ERR.
run_case() {
  local ghdir="$1" statedir="$2"; shift 2
  local envs=()
  while [[ "${1:-}" != "--" && $# -gt 0 ]]; do envs+=("$1"); shift; done
  shift  # drop the --
  # ${a[@]+"${a[@]}"}: an EMPTY array expands to nothing under bash 3.2's set -u
  # instead of aborting the case with "unbound variable".
  ensure_desktoken "$ghdir"
  # DESK_LOOP defaults to the intake desk's own loop; a case overrides it by naming it.
  OUT=$(env DESK_LOOP=intake-desk ${envs[@]+"${envs[@]}"} INBOUND_MONITOR_STATE_DIR="$statedir" \
        PATH="$ghdir:$PATH" bash "$SCRIPT" "$@" 2>"$ghdir/stderr")
  RC=$?
  ERR=$(cat "$ghdir/stderr")
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

echo "inbound-monitor regression suite (bash $BASH_VERSION)"

# ================================================================ A =========
echo "A — the monitor must scrub an inherited App token, not poll as it"

w="$TMPROOT/a-gh"; st="$TMPROOT/a-state"
make_gh "$w" 0 "$(issue_json 3)" ""
# Arm the monitor from a shell that exported a bogus App token, exactly as desk
# work does when it exports a role token into GH_TOKEN. The value only has to be
# non-empty for the scrub assertion; a placeholder that wears NO real credential
# prefix keeps a token-shaped string out of the public tree.
run_case "$w" "$st" GH_TOKEN=x-bogus-app-token GITHUB_TOKEN=x-bogus-app-token-2 -- o/r
tokline=$(sed -n 1p "$w/token.log" 2>/dev/null)
check "$([[ "$tokline" == "GH_TOKEN=[x-minted-o__r] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
  "gh sees the repo's minted role token, never the inherited GH_TOKEN/GITHUB_TOKEN" \
  "gh saw: ${tokline:-<no invocation recorded>}"
check "$(contains "$OUT" "MONITOR-ARMED: 3" && echo 0 || echo 1)" \
  "arm still succeeds on the minted identity" "stdout: ${OUT:-<empty>}"

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

# ---- D: a read PAST the ceiling is TRUNCATED — could-not-check, not ground truth
echo "D — a read holding more than --limit is truncated: retain + degrade"
# D1: past the ceiling on an ESTABLISHED repo retains the real baseline and goes loud,
# and — critically — fires NO phantom INBOUND when the (shifted) window returns
# keys the baseline never held.
w="$TMPROOT/d-gh"; st="$TMPROOT/d-state"
make_gh "$w" 0 "$(issue_json 3)" ""                  # seed 3 (< limit 5)
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
make_gh "$w" 0 "$(issue_json 6 900)" ""              # 6 > limit, a shifted window
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a read past the ceiling => exit 2" "got $RC"
check "$(contains "$OUT" "more than --limit 5" && contains "$OUT" "TRUNCATED" && echo 0 || echo 1)" \
  "the truncation is named as could-not-check" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "INBOUND:" && echo 1 || echo 0)" \
  "a truncated read fires no phantom INBOUND on the shifted window" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$st/o__r.state")" == "3" ]] && echo 0 || echo 1)" \
  "the real 3-line baseline is retained, not overwritten by the slice" \
  "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"

# D2: past the ceiling on the SEED establishes NO baseline (a truncated seed would bake a
# moving window in as if it were the whole repo).
w="$TMPROOT/d2-gh"; st="$TMPROOT/d2-state"
make_gh "$w" 0 "$(issue_json 6)" ""                  # 6 > limit on the very first read
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a truncated seed => exit 2" "got $RC"
check "$(contains "$OUT" "seed returned more than --limit 5" && echo 0 || echo 1)" \
  "the truncated seed is named" "stdout: ${OUT:-<empty>}"
check "$([[ ! -f "$st/o__r.state" ]] && echo 0 || echo 1)" \
  "no baseline file is written for a truncated seed" "a state file was written"

# D3: a set of EXACTLY the ceiling is the whole set — it seeds, then diffs — and
# gh is asked for one row past the ceiling (the truncation signal).
w="$TMPROOT/d3-gh"; st="$TMPROOT/d3-state"
make_gh "$w" 0 "$(issue_json 5)" ""                  # exactly the ceiling
run_case "$w" "$st" INBOUND_MONITOR_LIMIT=5 -- o/r
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "a seed of exactly the ceiling => exit 0" "got $RC"
check "$([[ "$OUT" == "MONITOR-ARMED: 5" ]] && echo 0 || echo 1)" \
  "a seed of exactly the ceiling arms" "stdout: ${OUT:-<empty>}"
check "$([[ "$(grep -c . "$st/o__r.state" 2>/dev/null)" == "5" ]] && echo 0 || echo 1)" \
  "the whole 5-line set is the baseline" "lines: $(grep -c . "$st/o__r.state" 2>/dev/null)"
check "$(grep -q -- '--limit 6 ' "$w/args.log" && echo 0 || echo 1)" \
  "gh is asked for one row past the ceiling" "args: $(cat "$w/args.log" 2>/dev/null)"

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

# ================================================================ F =========
echo "F — an explicit read identity (--token-file OWNER=PATH) outranks the keyring"
# The field defect: under a replaced HOME (a desk cell's), the keyring fallback
# resolves to no usable account, so every read 401s and every repo goes
# DEGRADED. The stub below plays exactly that HOME: with GH_TOKEN unset it fails
# the way `gh` does with no keyring account; with the installation token it
# reads; with any other token it is refused.
F_TOKEN=x-example-installation-token      # placeholder: wears no real credential prefix
make_keyringless_gh() {
  local dir="$1"
  mkdir -p "$dir"
  cat > "$dir/gh" <<STUB
#!/usr/bin/env bash
repo=""
while [ \$# -gt 0 ]; do
  case "\$1" in --repo) shift; repo="\$1" ;; esac
  shift
done
printf '%s GH_TOKEN=[%s] GITHUB_TOKEN=[%s]\n' "\$repo" "\${GH_TOKEN:-}" "\${GITHUB_TOKEN:-}" >> "$dir/token.log"
if [ -z "\${GH_TOKEN:-}" ]; then
  echo "HTTP 401: Requires authentication (https://api.github.com/graphql)" >&2
  exit 1
fi
if [ "\${GH_TOKEN}" != "$F_TOKEN" ]; then
  echo "HTTP 401: Bad credentials (https://api.github.com/graphql)" >&2
  exit 1
fi
jq -nc '[range(3) | {number: (100 + .), updatedAt: "2026-01-01T00:00:00Z"}]'
STUB
  chmod +x "$dir/gh"
}
fhome="$TMPROOT/f-home"; mkdir -p "$fhome"      # a HOME with no gh config at all
ftok="$TMPROOT/f-token"
( umask 077; printf '%s\n' "$F_TOKEN" > "$ftok" )

# F1 — the acceptance row: keyring-less HOME + an explicit token file reads CLEAN.
w="$TMPROOT/f1-gh"; st="$TMPROOT/f1-state"
make_keyringless_gh "$w"
run_case "$w" "$st" HOME="$fhome" -- --token-file "example-org=$ftok" example-org/tracker
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" \
  "keyring-less HOME + --token-file reads clean (exit 0)" "got $RC; stdout: ${OUT:-<empty>}; stderr: ${ERR:-<empty>}"
check "$(contains "$OUT" "MONITOR-ARMED: 3" && echo 0 || echo 1)" \
  "the explicit identity arms the repo" "stdout: ${OUT:-<empty>}"
check "$(grep -qxF "example-org/tracker GH_TOKEN=[$F_TOKEN] GITHUB_TOKEN=[]" "$w/token.log" 2>/dev/null && echo 0 || echo 1)" \
  "gh reads the repo as the token file's identity" "gh saw: $(cat "$w/token.log" 2>/dev/null)"
check "$(contains "$OUT$ERR" "$F_TOKEN" && echo 1 || echo 0)" \
  "the token VALUE is never printed" "stdout/stderr carried the token"

# F2 — the same HOME with NO token file reads as the MINTED role token (never the
# keyring); the stub refuses a token that is not the explicit one, so it DEGRADES.
w="$TMPROOT/f2-gh"; st="$TMPROOT/f2-state"
make_keyringless_gh "$w"
run_case "$w" "$st" HOME="$fhome" GH_TOKEN=x-bogus-app-token -- example-org/tracker
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" \
  "without a token file the keyring-less read is DEGRADED (exit 2)" "got $RC"
check "$(contains "$OUT" "MONITOR-DEGRADED: example-org/tracker seed read FAILED (gh: HTTP 401: Bad credentials" && echo 0 || echo 1)" \
  "the refused credential is named" "stdout: ${OUT:-<empty>}"
check "$(grep -qxF "example-org/tracker GH_TOKEN=[x-minted-example-org__tracker] GITHUB_TOKEN=[]" "$w/token.log" 2>/dev/null && echo 0 || echo 1)" \
  "no token file => the read is the minted role token, not the inherited GH_TOKEN and not the keyring" "gh saw: $(cat "$w/token.log" 2>/dev/null)"

# F3 — the token file is scoped to its OWNER. An installation token belongs to
# one owner's installation, so a repo under another owner is read as ITS OWN
# minted token (never the first owner's file, never an inherited GH_TOKEN).
w="$TMPROOT/f3-gh"; st="$TMPROOT/f3-state"
make_keyringless_gh "$w"
run_case "$w" "$st" HOME="$fhome" GH_TOKEN=x-bogus-app-token -- \
  example-org/tracker --token-file "example-org=$ftok" example-other/agents
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "an unmapped owner still degrades (exit 2)" "got $RC"
check "$(contains "$OUT" "MONITOR-DEGRADED: example-other/agents" && echo 0 || echo 1)" \
  "the unmapped owner's repo is the one named DEGRADED" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "MONITOR-DEGRADED: example-org/tracker" && echo 1 || echo 0)" \
  "the mapped owner's repo reads clean in the same cycle" "stdout: ${OUT:-<empty>}"
check "$(grep -qxF "example-other/agents GH_TOKEN=[x-minted-example-other__agents] GITHUB_TOKEN=[]" "$w/token.log" 2>/dev/null && echo 0 || echo 1)" \
  "the unmapped owner is read as its own minted token, not the other owner's file" \
  "gh saw: $(cat "$w/token.log" 2>/dev/null)"
check "$([[ -f "$st/example-org__tracker.state" ]] && echo 0 || echo 1)" \
  "the mapped repo seeded its baseline" "no state file for example-org/tracker"

# F4 — an explicit identity that cannot be USED is a precondition failure
# (exit 1) BEFORE any read. It never silently falls back to the keyring: that
# would swap the identity the operator named for one they did not.
floose="$TMPROOT/f-token-loose"; printf '%s\n' "$F_TOKEN" > "$floose"; chmod 644 "$floose"
fempty="$TMPROOT/f-token-empty"; ( umask 077; : > "$fempty" )
f4_case() {  # f4_case <label> <expect-in-stderr> <args...>
  local label="$1" want="$2"; shift 2
  w="$TMPROOT/f4-gh-$label"; st="$TMPROOT/f4-state-$label"
  make_keyringless_gh "$w"
  run_case "$w" "$st" HOME="$fhome" -- "$@"
  check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "$label => exit 1 (precondition)" "got $RC; stdout: ${OUT:-<empty>}"
  check "$(contains "$ERR" "$want" && echo 0 || echo 1)" "$label is named on stderr" "stderr: ${ERR:-<empty>}"
  check "$([[ ! -f "$w/token.log" ]] && echo 0 || echo 1)" \
    "$label => gh is never invoked (no fallback to another identity)" "gh saw: $(cat "$w/token.log" 2>/dev/null)"
}
f4_case loose-mode "group/world accessible" --token-file "example-org=$floose" example-org/tracker
f4_case missing "cannot read --token-file" --token-file "example-org=$TMPROOT/nope" example-org/tracker
f4_case empty-file "is empty" --token-file "example-org=$fempty" example-org/tracker
f4_case no-owner "expected OWNER=PATH" --token-file "$ftok" example-org/tracker
f4_case no-value "--token-file needs a value" example-org/tracker --token-file
f4_case dup-owner "given twice" --token-file "example-org=$ftok" --token-file "example-org=$ftok" example-org/tracker

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

# ================================================================ G =========
echo "G — each repo is read under its OWN minted role token; a failed mint is DEGRADED, never ambient"
# regression: #3116
# multi_gh <dir> — a gh that serves 3 issues per repo, logging "<repo> GH_TOKEN=[..]"
# for every call. An EMPTY GH_TOKEN is the ambient-login signature: it is logged to
# <dir>/ambient.log so the failure cases can assert nothing ever ran that way.
multi_gh() {
  local dir="$1"
  mkdir -p "$dir"; : > "$dir/token.log"; : > "$dir/ambient.log"; : > "$dir/calls.log"
  ensure_desktoken "$dir"
  cat > "$dir/gh" <<STUB
#!/usr/bin/env bash
repo=""; prev=""
for a in "\$@"; do [ "\$prev" = "--repo" ] && repo="\$a"; prev="\$a"; done
printf '%s\n' "\$repo" >> "$dir/calls.log"
printf '%s GH_TOKEN=[%s] GITHUB_TOKEN=[%s]\n' "\$repo" "\${GH_TOKEN:-}" "\${GITHUB_TOKEN:-}" >> "$dir/token.log"
[ -z "\${GH_TOKEN:-}" ] && printf '%s\n' "\$repo" >> "$dir/ambient.log"
jq -nc '[range(3) | {number: (100 + .), updatedAt: "2026-01-01T00:00:00Z"}]'
STUB
  chmod +x "$dir/gh"
}
glog() { tr '\n' ';' < "$1" 2>/dev/null; }

# G1 — one mint per repo, as the loop's role, scoped by --repo; each read carries
# only its own repo's token; the token value is never printed.
w="$TMPROOT/g1-gh"; st="$TMPROOT/g1-state"; multi_gh "$w"
run_case "$w" "$st" GH_TOKEN=x-bogus-app-token -- o/a p/b
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "a fully-minted cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
check "$([[ "$(cat "$w/mint.log")" == "issue-loop --repo o/a
issue-loop --repo p/b" ]] && echo 0 || echo 1)" \
  "one mint per repo, as the loop's role (issue-loop), scoped by --repo" "mints: $(glog "$w/mint.log")"
check "$([[ "$(cat "$w/token.log")" == "o/a GH_TOKEN=[x-minted-o__a] GITHUB_TOKEN=[]
p/b GH_TOKEN=[x-minted-p__b] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
  "each gh read carries only ITS repo's token" "gh saw: $(glog "$w/token.log")"
check "$(contains "$OUT$ERR" "x-minted" && echo 1 || echo 0)" \
  "the token VALUE never appears on stdout or stderr" "stdout: ${OUT:-<empty>} stderr: ${ERR:-<empty>}"

# G2 — --token-file still outranks the mint for its owner (and the mint serves the rest).
w="$TMPROOT/g2-gh"; st="$TMPROOT/g2-state"; multi_gh "$w"
run_case "$w" "$st" -- --token-file "example-org=$ftok" example-org/tracker p/b
check "$([[ "$(cat "$w/mint.log")" == "issue-loop --repo p/b" ]] && echo 0 || echo 1)" \
  "an owner with --token-file is never minted; the other owner is" "mints: $(glog "$w/mint.log")"
check "$(grep -qxF "example-org/tracker GH_TOKEN=[$F_TOKEN] GITHUB_TOKEN=[]" "$w/token.log" && echo 0 || echo 1)" \
  "the named file is the read identity for its owner" "gh saw: $(glog "$w/token.log")"

# G3 — a failed mint for one repo: that repo is DEGRADED with its baseline kept and
# NO gh call; the other repo still reads; recovery emits nothing phantom.
w="$TMPROOT/g3-gh"; st="$TMPROOT/g3-state"; multi_gh "$w"
run_case "$w" "$st" -- o/good o/bad                                  # seed both
: > "$w/calls.log"; : > "$w/token.log"; : > "$w/mint.log"
printf '%s' "desktoken: refused: no installation of the role App covers o/bad" > "$w/mintfail/$(san o/bad)"
run_case "$w" "$st" -- o/good o/bad
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a cycle with a failed mint exits 2" "got $RC; stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "MONITOR-DEGRADED: o/bad token mint FAILED" && contains "$OUT" "no installation of the role App covers o/bad" && contains "$OUT" "could-not-check" && echo 0 || echo 1)" \
  "the failed repo names itself, the desktoken cause, and could-not-check" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "keeping its previous 3 issue(s)" && echo 0 || echo 1)" \
  "the degraded line states the retained baseline" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "MONITOR-DEGRADED: o/good" && echo 1 || echo 0)" \
  "the healthy repo is not degraded" "stdout: ${OUT:-<empty>}"
check "$([[ "$(cat "$w/calls.log")" == "o/good" ]] && echo 0 || echo 1)" \
  "gh was called for o/good ONLY: no read for o/bad, ambient or otherwise" "gh calls: $(glog "$w/calls.log")"
check "$([[ ! -s "$w/ambient.log" ]] && echo 0 || echo 1)" \
  "no gh call ever ran without a minted GH_TOKEN (no ambient fallback)" "ambient: $(glog "$w/ambient.log")"
check "$([[ "$(grep -c . "$st/o__bad.state")" == "3" ]] && echo 0 || echo 1)" \
  "o/bad's 3-issue baseline is retained, not zeroed" "lines: $(grep -c . "$st/o__bad.state" 2>/dev/null)"
rm -f "$w/mintfail/$(san o/bad)"
run_case "$w" "$st" -- o/good o/bad
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the recovery cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "INBOUND" && echo 1 || echo 0)" \
  "recovery emits NO phantom INBOUND (the outage was absorbed)" "stdout: ${OUT:-<empty>}"

# G4 — a failed mint on first sight establishes no baseline and never arms.
w="$TMPROOT/g4-gh"; st="$TMPROOT/g4-state"; multi_gh "$w"
printf '%s' "desktoken: custody for role issue-loop is unavailable" > "$w/mintfail/$(san o/r)"
run_case "$w" "$st" -- o/r
check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "a failed-mint seed exits 2" "got $RC; stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "MONITOR-DEGRADED: o/r token mint FAILED" && contains "$OUT" "no baseline established" && echo 0 || echo 1)" \
  "the seed names the mint failure and that no baseline exists" "stdout: ${OUT:-<empty>}"
check "$(contains "$OUT" "MONITOR-ARMED" && echo 1 || echo 0)" "a degraded seed never prints MONITOR-ARMED" "stdout: ${OUT:-<empty>}"
check "$([[ ! -f "$st/o__r.state" ]] && echo 0 || echo 1)" "no baseline file is written" "a state file was written"
check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" "gh was never called" "gh calls: $(glog "$w/calls.log")"

# G5 — an unset or unknown $DESK_LOOP degrades EVERY repo: no mint, no gh, no state.
for loop in "" "not-a-desk-loop"; do
  w="$TMPROOT/g5-gh"; st="$TMPROOT/g5-state"; rm -rf "$w" "$st"; multi_gh "$w"
  run_case "$w" "$st" "DESK_LOOP=$loop" GH_TOKEN=x-bogus-app-token -- o/a o/b
  label="DESK_LOOP='${loop}'"
  check "$([[ "$RC" -eq 2 ]] && echo 0 || echo 1)" "$label exits 2" "got $RC; stdout: ${OUT:-<empty>}"
  for r in o/a o/b; do
    check "$(contains "$OUT" "MONITOR-DEGRADED: $r token mint FAILED" && contains "$OUT" "DESK_LOOP" && echo 0 || echo 1)" \
      "$label: $r is degraded, naming DESK_LOOP" "stdout: ${OUT:-<empty>}"
  done
  check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" \
    "$label: gh is NEVER invoked (no ambient poll)" "gh calls: $(glog "$w/calls.log")"
  check "$([[ ! -s "$w/mint.log" ]] && echo 0 || echo 1)" \
    "$label: no mint is attempted for a role that cannot be named" "mints: $(glog "$w/mint.log")"
  check "$([[ ! -f "$st/o__a.state" && ! -f "$st/o__b.state" ]] && echo 0 || echo 1)" \
    "$label: no baseline is written" "state files present"
done

# G6 — desktoken exit 0 with no usable token file is still a failed mint.
for mode in mintblank mintnopath; do
  w="$TMPROOT/g6-gh-$mode"; st="$TMPROOT/g6-state-$mode"; multi_gh "$w"
  ensure_desktoken "$w"; : > "$w/$mode/$(san o/r)"
  run_case "$w" "$st" GH_TOKEN=x-bogus-app-token -- o/r
  check "$([[ "$RC" -eq 2 ]] && contains "$OUT" "MONITOR-DEGRADED: o/r token mint FAILED" && echo 0 || echo 1)" \
    "$mode: degraded as a failed mint" "rc=$RC stdout: ${OUT:-<empty>}"
  check "$([[ ! -s "$w/calls.log" ]] && echo 0 || echo 1)" \
    "$mode: gh is never invoked (an empty GH_TOKEN would read as the host's login)" "gh calls: $(glog "$w/calls.log")"
done

# G7 — $DESK_LOOP maps to the App role exactly as the compiled loop-to-role table does.
for pair in the-desk:desk worker-desk:worker batch-fanout:worker pr-review-desk:reviewer \
            verify-desk:verifier intake-desk:issue-loop issue-loop:issue-loop; do
  loop="${pair%%:*}"; role="${pair#*:}"
  w="$TMPROOT/g7-gh"; st="$TMPROOT/g7-state"; rm -rf "$w" "$st"; multi_gh "$w"
  run_case "$w" "$st" "DESK_LOOP=$loop" -- o/r
  check "$([[ "$(cat "$w/mint.log")" == "$role --repo o/r" ]] && echo 0 || echo 1)" \
    "$loop mints as role '$role'" "mints: $(glog "$w/mint.log")"
done

# G8 — pacing: a repo that makes no call is never paced around and not counted as a read.
w="$TMPROOT/g8-gh"; st="$TMPROOT/g8-state"; multi_gh "$w"
printf '%s' "desktoken: refused" > "$w/mintfail/$(san o/bad)"
t0=$(date +%s)
run_case "$w" "$st" ASSAY_MONITOR_PACE_SECONDS=2 -- o/bad o/good
t1=$(date +%s)
check "$([[ $((t1 - t0)) -lt 2 ]] && echo 0 || echo 1)" \
  "a failed mint before the only real read costs no pacing sleep" "took $((t1 - t0))s"
check "$([[ "$(cat "$w/calls.log")" == "o/good" ]] && echo 0 || echo 1)" \
  "the read after the failed mint still happens" "gh calls: $(glog "$w/calls.log")"

# G9 — F1/S1: the token that was CHECKED is the token gh gets, even if the shared
# cache file is emptied during the pacing sleep (desktoken rewrites it in place).
# regression: #3116
w="$TMPROOT/g9-gh"; st="$TMPROOT/g9-state"; multi_gh "$w"
: > "$w/mintrace/$(san o/b)"
run_case "$w" "$st" ASSAY_MONITOR_PACE_SECONDS=1 GH_TOKEN=x-bogus-app-token -- o/a o/b
check "$([[ "$RC" -eq 0 ]] && echo 0 || echo 1)" "the cycle exits 0" "got $RC; stdout: ${OUT:-<empty>}"
check "$([[ "$(cat "$w/token.log")" == "o/a GH_TOKEN=[x-minted-o__a] GITHUB_TOKEN=[]
o/b GH_TOKEN=[x-minted-o__b] GITHUB_TOKEN=[]" ]] && echo 0 || echo 1)" \
  "gh sees the value that was validated for each repo, never an emptied re-read" "gh saw: $(glog "$w/token.log")"
check "$([[ ! -s "$w/ambient.log" ]] && echo 0 || echo 1)" \
  "no gh call ever ran with an empty GH_TOKEN (the ambient-login signature)" "ambient: $(glog "$w/ambient.log")"

# ================================================================ H =========
echo "H — no gh call in the script runs under the ambient identity (defect-class lint)"
# regression: #3116
# bare_gh_calls <file> — the non-comment lines that run a `gh` subcommand WITHOUT a
# same-line `GH_TOKEN=` assignment of a plain variable: an ambient-identity read, the
# defect class of #3116. A `gh` read here is only ever `GH_TOKEN="$var" gh ...` where
# $var was validated earlier. `GH_TOKEN="$(cat f)"` is flagged too: it re-reads (and
# so does not check) the value it passes, which can come back empty.
bare_gh_calls() {
  grep -nE '(^|[[:space:](])gh (pr|issue|api|auth|repo|run|search) ' "$1" \
    | grep -vE '^[0-9]+:[[:space:]]*#' | grep -vE 'GH_TOKEN="?\$[A-Za-z_{]' || true
}

# The planted second instance: a bare ambient read added to a copy of the script
# must be caught by the same lint (so the lint cannot rot into a no-op).
planted="$TMPROOT/planted-bare-gh.sh"
cp "$SCRIPT" "$planted"
printf '%s\n' 'extra=$(gh issue list --repo o/r --state open)' >> "$planted"
check "$([[ -n "$(bare_gh_calls "$planted")" ]] && echo 0 || echo 1)" \
  "the lint catches a planted bare gh call" "lint returned nothing for a script with a bare gh read"
printf '%s\n' 'extra=$(GH_TOKEN="$t" gh issue list --repo o/r --state open)' > "$planted.ok"
check "$([[ -z "$(bare_gh_calls "$planted.ok")" ]] && echo 0 || echo 1)" \
  "the lint passes a token-scoped gh call" "lint flagged a GH_TOKEN-scoped call"
printf '%s\n' 'extra=$(GH_TOKEN="$(cat "$f")" gh issue list --repo o/r --state open)' > "$planted.reread"
check "$([[ -n "$(bare_gh_calls "$planted.reread")" ]] && echo 0 || echo 1)" \
  "the lint catches a planted unchecked re-read at the call site" "lint passed GH_TOKEN=\"\$(cat ...)\""
bare=$(bare_gh_calls "$SCRIPT")
check "$([[ -z "$bare" ]] && echo 0 || echo 1)" \
  "no gh call in the script runs without a same-line GH_TOKEN= (no ambient read)" "bare: $bare"

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
ensure_desktoken "$w"
OUT=$(cd "$w/norepo" && DESK_LOOP=intake-desk INBOUND_MONITOR_STATE_DIR="$st" PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
ERR=$(cat "$w/stderr")
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "no resolvable repos => exit 1" "got $RC"
check "$(contains "$ERR" "unbound variable" && echo 1 || echo 0)" \
  "the no-repo path is bash-3.2 safe under set -u" "set -u tripped: $ERR"

run_case "$w" "$st" -- 'o/r;touch pwned'
check "$([[ "$RC" -eq 1 ]] && echo 0 || echo 1)" "invalid repo token => exit 1" "got $RC"
check "$([[ ! -e "$w/pwned" && ! -e pwned ]] && echo 0 || echo 1)" \
  "the token never reaches a shell" "pwned marker created"

# ======================================================= origin fallback ====
# regression: #2724
echo "Origin fallback — every github.com URL form resolves; an unresolvable set is loud on stdout"
w="$TMPROOT/of-gh"; make_gh "$w" 0 "$(issue_json 1)" ""
n=0
ensure_desktoken "$w"
for url in "https://github.com/o/r.git" "https://github.com:443/o/r.git" "https://u:pw@github.com:443/o/r" \
           "ssh://git@github.com/o/r.git" "ssh://git@github.com:22/o/r" "git@github.com:o/r.git" "https://github.com/o/r/"; do
  n=$((n + 1)); rd="$TMPROOT/of-repo-$n"; mkdir -p "$rd"; git -C "$rd" init -q 2>/dev/null
  git -C "$rd" remote add origin "$url"; rm -f "$w/args.log"
  OUT=$(cd "$rd" && DESK_LOOP=intake-desk INBOUND_MONITOR_STATE_DIR="$TMPROOT/of-st-$n" PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
  check "$([[ "$RC" -eq 0 ]] && grep -q -- '--repo o/r ' "$w/args.log" 2>/dev/null && echo 0 || echo 1)" \
    "origin '$url' resolves to o/r and is polled" "rc=$RC args=$(cat "$w/args.log" 2>/dev/null) stderr=$(cat "$w/stderr")"
done
rd="$TMPROOT/of-repo-bad"; mkdir -p "$rd"; git -C "$rd" init -q 2>/dev/null
git -C "$rd" remote add origin "https://user:s3cretpw@example.com:8443/o/r.git"
OUT=$(cd "$rd" && INBOUND_MONITOR_STATE_DIR="$TMPROOT/of-st-bad" PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
ERR=$(cat "$w/stderr")
check "$([[ "$RC" -eq 1 ]] && contains "$OUT" "MONITOR-DEGRADED: invalid repo" && echo 0 || echo 1)" \
  "an unparseable origin prints MONITOR-DEGRADED on stdout and exits 1" "rc=$RC stdout: ${OUT:-<empty>}"
check "$([[ "$OUT" == "MONITOR-DEGRADED: invalid repo 'https://example.com:8443/o/r' (expected owner/name)" ]] && echo 0 || echo 1)" \
  "credential-stripped display is exact on both Bash versions" "stdout: $OUT"
check "$(contains "$OUT$ERR" "s3cretpw" && echo 1 || echo 0)" \
  "the origin's credential never appears on stdout or stderr" "stdout: $OUT stderr: $ERR"
rd="$TMPROOT/of-repo-none"; mkdir -p "$rd"; git -C "$rd" init -q 2>/dev/null
OUT=$(cd "$rd" && INBOUND_MONITOR_STATE_DIR="$TMPROOT/of-st-none" PATH="$w:$PATH" bash "$SCRIPT" 2>"$w/stderr"); RC=$?
check "$([[ "$RC" -eq 1 ]] && contains "$OUT" "MONITOR-DEGRADED: no repos to query" && echo 0 || echo 1)" \
  "no resolvable repos prints MONITOR-DEGRADED on stdout and exits 1" "rc=$RC stdout: ${OUT:-<empty>}"

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
