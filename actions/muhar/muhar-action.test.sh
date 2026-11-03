#!/usr/bin/env bash
# muhar-action.test.sh — offline suite for muhar-action.sh (the logic behind action.yml).
#
# No network, no Go toolchain, no real muhar: a stub `muhar` replays a committed
# fixture report (testdata/<name>.report), stderr (testdata/<name>.stderr, when present)
# and exit code (testdata/<name>.rc), chosen by the spec's file name, and records the
# arguments it was given. What it proves:
#
#   exit 0      a clean report passes, publishes caught / not-caught / could-not-mutate
#               and writes every verdict line and the Totals line to the job summary
#   NOT CAUGHT  fails the step by default; passes (with a warning) when
#               fail-on-not-caught is false
#   could-not-mutate  fails by default; passes when fail-on-could-not-mutate is false
#   exit 2      always fails with exit 2, worded "harness broken — not clean", publishes
#               no counts, and stays failed with both fail-on inputs false; an exit 2
#               with no HARNESS BROKEN report (a rejected flag) says so
#   exit 1      usage/spec error fails, with muhar's stderr in the summary; any other
#               exit is passed through
#   partial shard     -shard and -j reach muhar, and the summary says the sweep is partial
#   parsing     only an unindented Totals line counts (a mutation NAMED like one does
#               not), and a report with no Totals line or two of them is could-not-check
#   several specs     counts are summed; one broken spec fails the whole step
#   inputs      bad jobs / shard / boolean values and an empty spec list are refused
#               with result=error, and a non-empty muhar-version refuses (release
#               install not implemented)
#   action.yml  the result output falls back to error when the step failed early
#
# Run: bash actions/muhar/muhar-action.test.sh  — prints "muhar-action.test.sh: OK".
# The asserts are single-quoted on purpose: check() evals them at assert time.
# shellcheck disable=SC2016,SC2034
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
script="$here/muhar-action.sh"
export MUHAR_FIXTURES="$here/testdata"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/muhar-action-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

stub="$tmp/muhar"
cat >"$stub" <<'STUB'
#!/usr/bin/env bash
# Stub muhar: replay testdata/<spec basename>.{report,rc}; log argv one per line.
printf '%s\n' "$@" >>"$MUHAR_ARGLOG"
printf -- '--\n' >>"$MUHAR_ARGLOG"
# A suite under test inherits muhar's environment: if the workflow file commands reached
# it, it could forge the action's outputs or summary. Try to, so the suite can see it fail.
for f in "${GITHUB_OUTPUT:-}" "${GITHUB_STEP_SUMMARY:-}" "${GITHUB_ENV:-}" "${GITHUB_PATH:-}" "${GITHUB_STATE:-}"; do
	[ -n "$f" ] && printf 'caught=777\nFORGED\n' >>"$f"
done
spec=
while [ $# -gt 0 ]; do
	case $1 in -spec) spec=$2; shift ;; -shard) echo "muhar: shard $2 — stub" >&2; shift ;; esac
	shift
done
name=$(basename "$spec" .json)
cat "$MUHAR_FIXTURES/$name.report"
[ -f "$MUHAR_FIXTURES/$name.stderr" ] && cat "$MUHAR_FIXTURES/$name.stderr" >&2
exit "$(cat "$MUHAR_FIXTURES/$name.rc")"
STUB
chmod +x "$stub"

fails=0
ok() { printf '  ok    %s\n' "$1"; }
bad() { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check() { if eval "$2"; then ok "$1"; else bad "$1"; fi; }

# run_case NAME SPECS [VAR=VALUE ...] — runs `muhar-action.sh run` with fresh summary,
# output and argv-log files; leaves $rc, $log, $sum, $outf, $args for the asserts.
run_case() {
	local name=$1 specs=$2
	shift 2
	sum="$tmp/$name.summary" outf="$tmp/$name.output" args="$tmp/$name.args" log="$tmp/$name.log"
	: >"$sum"; : >"$outf"; : >"$args"
	env MUHAR_BIN="$stub" MUHAR_SPECS="$specs" MUHAR_WORKDIR="$tmp" MUHAR_JOBS=1 MUHAR_SHARD= \
		MUHAR_FAIL_ON_NOT_CAUGHT=true MUHAR_FAIL_ON_COULD_NOT_MUTATE=true \
		GITHUB_STEP_SUMMARY="$sum" GITHUB_OUTPUT="$outf" MUHAR_ARGLOG="$args" RUNNER_TEMP="$tmp" \
		GITHUB_ENV="$tmp/$name.env" GITHUB_PATH="$tmp/$name.path" GITHUB_STATE="$tmp/$name.state" \
		"$@" bash "$script" run >"$log" 2>&1
	rc=$?
}
has() { grep -qF -- "$2" "$1"; }
out_is() { grep -qxF -- "$1" "$outf"; }

echo "== exit 0, every mutation caught"
run_case clean "specs/clean.json"
check "clean run exits 0" '[ "$rc" -eq 0 ]'
check "caught output is 3" 'out_is "caught=3"'
check "not-caught output is 0" 'out_is "not-caught=0"'
check "could-not-mutate output is 0" 'out_is "could-not-mutate=0"'
check "result output is clean" 'out_is "result=clean"'
check "the run under test gets no GITHUB_ENV / GITHUB_PATH / GITHUB_STATE" '[ ! -e "$tmp/clean.env" ] && [ ! -e "$tmp/clean.path" ] && [ ! -e "$tmp/clean.state" ]'
check "summary carries the Totals line" 'has "$sum" "Totals: 3 caught, 0 NOT CAUGHT, 0 could-not-mutate."'
check "summary carries each verdict line" 'has "$sum" "CAUGHT           fee cap"'
check "a healthy run's summary carries no stderr section" '! has "$sum" "muhar stderr"'
check "spec and -j reach muhar" '[ "$(head -4 "$args" | tr "\n" " ")" = "-spec specs/clean.json -j 1 " ]'
check "no -shard when shard is empty" '! has "$args" "-shard"'
check "full sweep is not called partial" '! has "$sum" "Partial sweep"'

echo "== NOT CAUGHT"
run_case nc "specs/notcaught.json"
check "NOT CAUGHT fails by default" '[ "$rc" -eq 1 ]'
check "NOT CAUGHT still publishes its counts" 'out_is "not-caught=1" && out_is "caught=2"'
check "result output is not-caught" 'out_is "result=not-caught"'
check "NOT CAUGHT is reported as an error" 'has "$log" "::error::muhar: 1 mutation(s) NOT CAUGHT"'
run_case nc_off "specs/notcaught.json" MUHAR_FAIL_ON_NOT_CAUGHT=false
check "fail-on-not-caught=false passes" '[ "$rc" -eq 0 ]'
check "and warns instead" 'has "$log" "::warning::muhar: 1 mutation(s) NOT CAUGHT"'

echo "== could-not-mutate"
run_case cnm "specs/cnm.json"
check "could-not-mutate fails by default" '[ "$rc" -eq 1 ]'
check "could-not-mutate output is 1" 'out_is "could-not-mutate=1"'
check "result output is could-not-mutate" 'out_is "result=could-not-mutate"'
check "summary escapes HTML in verdict lines" 'has "$sum" "generic &lt;T&gt; guard" && has "$sum" "lib/a&amp;b.go" && ! has "$sum" "<T>"'
run_case cnm_off "specs/cnm.json" MUHAR_FAIL_ON_COULD_NOT_MUTATE=false
check "fail-on-could-not-mutate=false passes" '[ "$rc" -eq 0 ]'
check "and still says it is not clean" 'has "$log" "never tested, not clean"'

echo "== exit 2 — harness broken"
run_case broken "specs/broken.json"
check "broken harness exits 2" '[ "$rc" -eq 2 ]'
check "worded harness broken — not clean" 'has "$log" "::error::muhar: harness broken — not clean (specs/broken.json)"'
check "summary says harness broken — not clean" 'has "$sum" "harness broken — not clean"'
check "summary carries muhar's own reason" 'has "$sum" "baseline suite is RED on pristine source"'
check "no counts published for a broken run" '! grep -qE "^(not-)?caught=|^could-not-mutate=" "$outf"'
check "the run under test cannot write the action's outputs or summary" '! has "$outf" "FORGED" && ! has "$sum" "FORGED"'
check "result output is harness-broken" 'out_is "result=harness-broken"'
check "a broken run's error names the baseline" 'has "$log" "The baseline was red or the positive control was not caught"'
run_case broken_lax "specs/broken.json" MUHAR_FAIL_ON_NOT_CAUGHT=false MUHAR_FAIL_ON_COULD_NOT_MUTATE=false
check "exit 2 fails even with both fail-on inputs false" '[ "$rc" -eq 2 ]'
run_case flagreject "specs/flagreject.json"
check "exit 2 with no HARNESS BROKEN report still exits 2" '[ "$rc" -eq 2 ] && out_is "result=harness-broken"'
check "and says muhar printed no HARNESS BROKEN report" 'has "$log" "without a HARNESS BROKEN report" && ! has "$log" "The baseline was red"'
check "and its summary carries muhar's stderr" 'has "$sum" "muhar stderr (last 40 lines)" && has "$sum" "flag: strconv.ParseInt: parsing"'

echo "== other muhar exits"
run_case usage "specs/usage.json"
check "muhar exit 1 fails the step" '[ "$rc" -eq 1 ]'
check "exit 1 is named a usage or spec error" 'has "$log" "usage, spec or IO error"'
check "no counts for exit 1" '! grep -qE "^(not-)?caught=|^could-not-mutate=" "$outf"'
check "exit 1's summary carries muhar's stderr, HTML-escaped" 'has "$sum" "muhar stderr (last 40 lines)" && has "$sum" "unexpected end of JSON input &lt;in specs/usage.json&gt;"'
check "muhar's stderr still reaches the log" 'has "$log" "muhar: parse spec: unexpected end of JSON input"'
run_case signal "specs/signal.json"
check "an unexpected muhar exit is passed through" '[ "$rc" -eq 130 ]'

echo "== partial shard"
run_case shard "specs/shard.json" MUHAR_SHARD=1/3 MUHAR_JOBS=0
check "shard run exits 0" '[ "$rc" -eq 0 ]'
check "-shard reaches muhar" '[ "$(head -6 "$args" | tr "\n" " ")" = "-spec specs/shard.json -j 0 -shard 1/3 " ]'
check "summary says the sweep is partial" 'has "$sum" "Partial sweep — shard 1/3"'
check "the shard's own Totals are published" 'out_is "caught=2"'

echo "== parsing the Totals line"
run_case decoy "specs/decoy.json"
check "an indented line named like Totals is not read as Totals" 'out_is "caught=1" && ! out_is "caught=99"'
run_case nototals "specs/nototals.json"
check "exit 0 with no Totals line is could-not-check" '[ "$rc" -eq 1 ] && has "$log" "could-not-check, not clean"'
check "and publishes no counts" '! grep -qE "^(not-)?caught=|^could-not-mutate=" "$outf"'
run_case twototals "specs/twototals.json"
check "two Totals lines are could-not-check" '[ "$rc" -eq 1 ] && has "$log" "no single Totals line"'
pout=$(bash "$script" parse "$MUHAR_FIXTURES/cnm.report")
check "parse prints the three counts" '[ "$pout" = "caught=1 not-caught=0 could-not-mutate=1" ]'
bash "$script" parse "$MUHAR_FIXTURES/broken.report" >/dev/null 2>&1
prc=$?
check "parse of a broken report exits 3" '[ "$prc" -eq 3 ]'

echo "== several specs"
run_case multi "$(printf '%s\n' "specs/clean.json" "" "  # a comment line" "  specs/shard.json  ")"
check "two healthy specs pass" '[ "$rc" -eq 0 ]'
check "counts are summed across specs" 'out_is "caught=5"'
check "blank and comment lines are skipped, spaces trimmed" '[ "$(grep -c -- "^-spec$" "$args")" -eq 2 ] && has "$args" "specs/shard.json"'
run_case multi_broken "$(printf '%s\n' specs/clean.json specs/broken.json specs/notcaught.json)" MUHAR_FAIL_ON_NOT_CAUGHT=false
check "one broken spec fails the whole step with 2" '[ "$rc" -eq 2 ]'
check "every spec still ran" '[ "$(grep -c -- "^-spec$" "$args")" -eq 3 ]'

echo "== input validation"
run_case badjobs "specs/clean.json" MUHAR_JOBS=two
check "non-integer jobs is refused" '[ "$rc" -eq 1 ] && has "$log" "jobs must be an integer from 0 to 9999" && [ ! -s "$args" ]'
check "a refused input still sets result=error" 'out_is "result=error"'
run_case hugejobs "specs/clean.json" MUHAR_JOBS=99999999999999999999
check "a jobs value muhar's flag would reject is refused up front" '[ "$rc" -eq 1 ] && has "$log" "jobs must be an integer from 0 to 9999" && [ ! -s "$args" ] && out_is "result=error"'
run_case badshard "specs/clean.json" MUHAR_SHARD=1of3
check "malformed shard is refused" '[ "$rc" -eq 1 ] && has "$log" "shard must be empty or" && [ ! -s "$args" ]'
run_case badbool "specs/clean.json" MUHAR_FAIL_ON_NOT_CAUGHT=yes
check "non-boolean fail-on-not-caught is refused" '[ "$rc" -eq 1 ] && has "$log" "must be true or false"'
run_case nospec "  "
check "an empty spec list is refused" '[ "$rc" -eq 1 ] && has "$log" "no spec given"'
run_case nobin "specs/clean.json" MUHAR_BIN="$tmp/absent"
check "a missing muhar binary is could-not-check" '[ "$rc" -eq 1 ] && has "$log" "could-not-check: muhar binary" && out_is "result=error"'

echo "== install"
ilog="$tmp/install.log"
env MUHAR_VERSION=v9.9.9 RUNNER_TEMP="$tmp" bash "$script" install >"$ilog" 2>&1
irc=$?
check "muhar-version refuses: release install not implemented" '[ "$irc" -eq 1 ] && has "$ilog" "installing a release binary is not implemented yet"'
env MUHAR_VERSION= MUHAR_ACTION_PATH="$tmp/nowhere" RUNNER_TEMP="$tmp" bash "$script" install >"$ilog" 2>&1
irc=$?
check "install outside its own checkout is could-not-check" '[ "$irc" -eq 1 ] && has "$ilog" "could-not-check: muhar source not found"'

echo "== action.yml"
want_result="value: \${{ steps.run.outputs.result || 'error' }}"
check "result output reads error when the step failed before run set it" 'grep -qF -- "$want_result" "$here/action.yml"'

if [ "$fails" -ne 0 ]; then
	echo "muhar-action.test.sh: $fails FAILED"
	exit 1
fi
echo "muhar-action.test.sh: OK"
