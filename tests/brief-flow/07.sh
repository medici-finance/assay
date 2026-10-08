#!/usr/bin/env bash
# Offline acceptance runner for the brief-flow event contract
# (spec/brief-flow-event-v1.md, schemas/brief-flow-event-v1.json,
# statusgen/briefevent.go, statusgen/briefstage.go).
#
#   bash tests/brief-flow/07.sh <mode>
#
# Modes: identity | stage | compatibility | flow | mutation | role-identity | all
#
# Each behavioural mode runs its targeted Go tests against the synthetic
# fixtures under statusgen/testdata/brief-flow/contract and requires every
# named test to report PASS. `role-identity` additionally dereferences the
# run-record role list it is compared against. `mutation` copies the module,
# corrupts the reducer one defect at a time and exits 0 only after it has
# observed, for EVERY mutation, a nonzero exit carrying that mutation's own
# ASSERT-FAIL[<id>] marker — and then the genuine code passing.
#
# Exit codes: 0 pass, 1 fail, 2 could-not-check. A missing prerequisite, a
# missing fixture, a missing test or a void mutation anchor never passes.
# Offline by construction: no network, no forge, no credentials.
set -u -o pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
pkg="$repo/statusgen"
fixtures="$pkg/testdata/brief-flow/contract"
runrec="$repo/docs/streams/graph-execution/brief-06-run-records-and-replay.md"
timeout="${BRIEFFLOW_TEST_TIMEOUT:-180s}"

cnc() { echo "could-not-check: $*" >&2; exit 2; }
fail() { echo "FAIL: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || cnc "go is not on PATH"
[ -f "$pkg/go.mod" ] || cnc "statusgen module not found at $pkg"
for f in briefevent.go briefstage.go briefevent_test.go; do
	[ -f "$pkg/$f" ] || cnc "statusgen/$f not found"
done
[ -f "$repo/schemas/brief-flow-event-v1.json" ] || cnc "schemas/brief-flow-event-v1.json not found"
for f in events.jsonl receipt.jsonl history.jsonl expect.json; do
	[ -f "$fixtures/$f" ] || cnc "fixture $f not found under statusgen/testdata/brief-flow/contract"
done

work="$(mktemp -d "${TMPDIR:-/tmp}/brief-flow-accept.XXXXXX")"
trap 'rm -rf "$work"' EXIT

# build <module-dir> <binary> — compile the package's test binary.
build() {
	( cd "$1" && GOWORK=off go test -c -o "$2" . ) >"$work/build.log" 2>&1
}

bin="$work/statusgen.test"
built=""
build_genuine() {
	[ -n "$built" ] && return 0
	build "$pkg" "$bin" || { cat "$work/build.log" >&2; cnc "statusgen test binary did not build"; }
	built=1
}

# run_tests <module-dir> <binary> <log> <TestName>... — returns the binary's exit code.
run_tests() {
	local dir="$1" b="$2" log="$3"
	shift 3
	local re
	re="^($(IFS='|'; echo "$*"))\$"
	( cd "$dir" && BRIEFFLOW_FIXTURES="$fixtures" "$b" -test.count=1 -test.timeout "$timeout" -test.v -test.run "$re" ) >"$log" 2>&1
}

# require_pass <log> <TestName>... — every named test reported PASS.
require_pass() {
	local log="$1" t
	shift
	for t in "$@"; do
		grep -q -- "^--- PASS: $t " "$log" || { cat "$log" >&2; fail "$t did not report PASS"; }
	done
}

mode_tests() {
	local name="$1"
	shift
	local log="$work/$name.log" rc
	build_genuine
	run_tests "$pkg" "$bin" "$log" "$@"
	rc=$?
	cat "$log"
	[ "$rc" -eq 0 ] || fail "$name: tests exited $rc"
	require_pass "$log" "$@"
	echo "PASS  $name"
}

# dereference — the run-record role list this contract claims to agree with
# is present where the tests read it.
dereference() {
	[ -f "$runrec" ] || cnc "run-record brief not found: docs/streams/graph-execution/brief-06-run-records-and-replay.md"
	# shellcheck disable=SC2016 # literal backticks
	grep -qF '`desk | reviewer | verifier | worker | human`' "$runrec" \
		|| fail "run-record role list not found in graph-execution/06 — the role vocabulary may have drifted"
	grep -q '"role"' "$repo/schemas/workflow-pattern-v1.json" 2>/dev/null \
		|| cnc "schemas/workflow-pattern-v1.json has no role enum to compare"
	echo "resolved: graph-execution/06 role list; workflow-pattern-v1 role enum"
}

# subst <file> <from> <to> — literal replace; a missing anchor voids the
# mutation and fails the runner.
subst() {
	local f="$1" from="$2" to="$3" body
	body="$(cat "$f"; printf x)"
	body="${body%x}"
	case "$body" in *"$from"*) ;; *) fail "mutation anchor not found in ${f##*/}: $from" ;; esac
	printf '%s' "${body//"$from"/"$to"}" >"$f"
}

# id | test | expected assertion | from | to | description
mutations=(
	"alias-dedup|TestBriefEventIdentity|identity-uuid-key|ev.BriefUUID // identity-key|ev.Alias|briefs keyed by alias instead of uuid"
	"partial-complete|TestBriefEventStage|stage-13|case merged == len(live):|case merged > 0:|one merged PR of three counts as the whole brief merged"
	"seed-birth|TestBriefEventIdentity|identity-births-03|p.Seeded = true // seed is not a birth|p.Seeded = true; p.Births++|historian seed counted as a birth"
)

mode_mutation() {
	local entry id test want from to desc copy mbin log rc observed=0
	for entry in "${mutations[@]}"; do
		IFS='|' read -r id test want from to desc <<<"$entry"
		copy="$work/mut-$id"
		mkdir -p "$copy/docs/streams/graph-execution"
		cp -R "$pkg" "$copy/statusgen" || cnc "could not copy the module"
		cp -R "$repo/schemas" "$copy/schemas" || cnc "could not copy schemas"
		cp "$runrec" "$copy/docs/streams/graph-execution/" 2>/dev/null || true
		subst "$copy/statusgen/briefstage.go" "$from" "$to"
		mbin="$work/mut-$id.test"
		build "$copy/statusgen" "$mbin" || { cat "$work/build.log" >&2; fail "mutation $id did not compile — a build failure is not a detection"; }
		log="$work/mut-$id.log"
		run_tests "$copy/statusgen" "$mbin" "$log" "$test"
		rc=$?
		if [ "$rc" -eq 0 ]; then
			cat "$log" >&2
			fail "mutation $id ($desc): $test exited 0 — the defect was not detected"
		fi
		if ! grep -qF "ASSERT-FAIL[$want]" "$log"; then
			cat "$log" >&2
			fail "mutation $id: exit $rc but no ASSERT-FAIL[$want]"
		fi
		echo "RED   $id: $test exit $rc with ASSERT-FAIL[$want] — $desc"
		grep -F "ASSERT-FAIL[$want]" "$log" | head -n 1 | sed 's/^ */        /'
		observed=$((observed + 1))
		rm -rf "$copy" "$mbin"
	done
	[ "$observed" -eq "${#mutations[@]}" ] || fail "observed $observed of ${#mutations[@]} mutations"
	mode_tests genuine TestBriefEventIdentity TestBriefEventStage
	echo "PASS  mutation: $observed of ${#mutations[@]} mutations detected; genuine code passes"
}

mode="${1:-}"
case "$mode" in
identity) mode_tests identity TestBriefEventIdentity ;;
stage) mode_tests stage TestBriefEventStage ;;
compatibility) mode_tests compatibility TestBriefEventCompat TestBriefEventSchemaParity ;;
flow) mode_tests flow TestBriefEventFlow ;;
role-identity) dereference && mode_tests role-identity TestBriefEventRoles ;;
mutation) mode_mutation ;;
all)
	mode_tests identity TestBriefEventIdentity
	mode_tests stage TestBriefEventStage
	mode_tests compatibility TestBriefEventCompat TestBriefEventSchemaParity
	mode_tests flow TestBriefEventFlow
	dereference
	mode_tests role-identity TestBriefEventRoles
	mode_mutation
	;;
*) echo "usage: $0 identity|stage|compatibility|flow|mutation|role-identity|all" >&2; exit 2 ;;
esac
