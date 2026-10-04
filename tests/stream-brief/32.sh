#!/usr/bin/env bash
# Offline acceptance runner for the stream-view contract
# (docs/stream-view-contract.md, statusgen/streamview).
#
#   bash tests/stream-brief/32.sh <case>
#
# Cases: legacy | identity | negative | flow | dereference | mutation | all
#
# Each behavioural case runs its targeted Go tests against the fixtures under
# statusgen/testdata/streamview and requires every named test to report PASS.
# `dereference` additionally checks the contract document's source map against
# this checkout: every listed existing file and symbol must be present, every
# planned one must still be absent. `mutation` corrupts a copy of the fixtures
# one defect at a time and exits 0 only after it has observed, for EVERY
# mutation, a nonzero exit carrying that mutation's own ASSERT-FAIL[<id>]
# marker — and then the genuine fixtures passing.
#
# Offline by construction: no network, no forge, no credentials. git runs only
# on temporary repositories the tests create. A missing prerequisite (bash
# tool, go, the package) exits 2 as could-not-check — never a pass.
set -u -o pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
pkg="$repo/statusgen"
fixtures="$pkg/testdata/streamview"
doc="$repo/docs/stream-view-contract.md"
timeout="${STREAMVIEW_TEST_TIMEOUT:-180s}"

cnc() { echo "could-not-check: $*" >&2; exit 2; }
fail() { echo "FAIL: $*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || cnc "go is not on PATH"
command -v git >/dev/null 2>&1 || cnc "git is not on PATH"
[ -f "$pkg/go.mod" ] || cnc "statusgen module not found at $pkg"
[ -f "$pkg/streamview/contract.go" ] || cnc "statusgen/streamview/contract.go not found"
[ -f "$fixtures/expect.json" ] || cnc "fixtures not found at $fixtures"

work="$(mktemp -d "${TMPDIR:-/tmp}/streamview-accept.XXXXXX")"
trap 'rm -rf "$work"' EXIT

bin=""
build() {
	[ -n "$bin" ] && return 0
	bin="$work/statusgen.test"
	( cd "$pkg" && go test -c -o "$bin" . ) >"$work/build.log" 2>&1 \
		|| { cat "$work/build.log" >&2; cnc "statusgen test binary did not build"; }
}

# run_tests <fixtures-dir> <log> <TestName>... — runs the named package-main
# tests against the given fixtures; returns the binary's exit code.
run_tests() {
	local fx="$1" log="$2"
	shift 2
	local re
	re="^($(IFS='|'; echo "$*"))\$"
	build
	( cd "$pkg" && STREAMVIEW_FIXTURES="$fx" "$bin" -test.count=1 -test.timeout "$timeout" -test.v -test.run "$re" ) >"$log" 2>&1
}

# require_pass <log> <TestName>... — every named test reported PASS.
require_pass() {
	local log="$1" t
	shift
	for t in "$@"; do
		grep -q -- "^--- PASS: $t " "$log" || { cat "$log" >&2; fail "$t did not report PASS"; }
	done
}

case_tests() {
	local name="$1"
	shift
	local log="$work/$name.log"
	run_tests "$fixtures" "$log" "$@"
	local rc=$?
	cat "$log"
	[ "$rc" -eq 0 ] || fail "$name: tests exited $rc"
	require_pass "$log" "$@"
}

case_unit() {
	local log="$work/unit.log"
	( cd "$pkg" && go test -count=1 -timeout "$timeout" -v -run "$1" ./streamview ) >"$log" 2>&1
	local rc=$?
	cat "$log"
	[ "$rc" -eq 0 ] || fail "streamview unit tests exited $rc"
	shift
	require_pass "$log" "$@"
}

# source_map — every row of the contract document's source map:
#   | `path` | `symbol` | existing |  → file present and symbol found in it
#   | `path` | `symbol` | planned  |  → symbol (or file, when symbol is —) absent
source_map() {
	[ -f "$doc" ] || fail "contract document missing: docs/stream-view-contract.md"
	local rows=0 line path sym state
	while IFS= read -r line; do
		case "$line" in
		'| `'*) ;;
		*) continue ;;
		esac
		path="$(printf '%s' "$line" | cut -d'|' -f2 | tr -d ' `')"
		# shellcheck disable=SC2016 # literal backticks in the sed program
		sym="$(printf '%s' "$line" | cut -d'|' -f3 | sed 's/^ *//; s/ *$//; s/^`//; s/`$//')"
		state="$(printf '%s' "$line" | cut -d'|' -f4 | tr -d ' ')"
		case "$path" in /* | *..*) fail "source map path not repo-relative: $path" ;; esac
		case "$state" in
		existing)
			[ -f "$repo/$path" ] || fail "source map: $path is listed as existing but is absent"
			if [ "$sym" != "—" ]; then
				grep -qF -- "$sym" "$repo/$path" || fail "source map: symbol '$sym' not found in $path"
			fi
			echo "resolved: $path :: $sym"
			;;
		planned)
			if [ "$sym" = "—" ]; then
				[ ! -e "$repo/$path" ] || fail "source map: $path is marked planned but exists"
			elif [ -f "$repo/$path" ] && grep -qF -- "$sym" "$repo/$path"; then
				fail "source map: '$sym' in $path is marked planned but exists"
			fi
			echo "planned: $path :: $sym"
			;;
		*) fail "source map: row state '$state' is not existing|planned: $line" ;;
		esac
		rows=$((rows + 1))
	done < <(sed -n '/^## Source map/,/^## /p' "$doc")
	[ "$rows" -gt 0 ] || fail "source map: no rows found under '## Source map'"
	echo "source map: $rows rows checked"
}

# subst <file> <from> <to> — literal replace; the mutation is void (and the
# runner fails) if <from> is not in the file.
subst() {
	local f="$1" from="$2" to="$3" body
	body="$(cat "$f"; printf x)"
	body="${body%x}"
	case "$body" in *"$from"*) ;; *) fail "mutation anchor not found in ${f#"$work"/}: $from" ;; esac
	printf '%s' "${body//"$from"/"$to"}" >"$f"
}

# Each mutation: id | test | expected assertion | description. apply_mutation
# performs it on a fresh fixture copy.
mutations=(
	"wrong-outcome|TestStreamViewLegacy|legacy-outcome|legacy README tagline differs from the recorded outcome"
	"invented-success|TestStreamViewLegacy|legacy-success-absent|expectation claims a legacy stream has a success criterion"
	"collided-scope|TestStreamViewIdentity|identity-key|second repository's stream declares the first repository"
	"renamed-key|TestStreamViewIdentity|identity-key|renamed stream expected under the old slug"
	"fixed-negative|TestStreamViewNegative|negative-diag|malformed mission corrected — the negative case must notice"
	"accept-version|TestStreamViewNegative|negative-version|expectation treats the supported version as unsupported"
	"wrong-commitment|TestStreamViewFlow|flow-value|authored commitment differs from the expected value"
	"wrong-reference|TestStreamViewDereference|deref-clean|evidence path points at a file that does not exist"
	"missing-file|TestStreamViewDereference|deref-clean|referenced evidence file removed from its repository"
	"unplanned|TestStreamViewDereference|deref-planned|planned output no longer marked planned"
	"wrong-unchecked|TestStreamViewDereference|deref-unchecked|external reference expected under another number"
)

apply_mutation() {
	local id="$1" fx="$2"
	case "$id" in
	wrong-outcome) subst "$fx/legacy/docs/streams/legacy-stream/README.md" "Keep the legacy board readable." "Keep the legacy board tidy." ;;
	invented-success) subst "$fx/expect.json" '"success_count": 0' '"success_count": 1' ;;
	collided-scope) subst "$fx/repo-b/docs/streams/shared/README.md" "priority: P2" "priority: P2"$'\n'"repo: example-org/repo-a" ;;
	renamed-key) subst "$fx/expect.json" '"key": "example-org/repo-a:shared-v2"' '"key": "example-org/repo-a:shared"' ;;
	fixed-negative) subst "$fx/negative/unknown-key/README.md" "  sucess:" "  success:" ;;
	accept-version) subst "$fx/expect.json" '"unsupported_contract": "stream-view/v2"' '"unsupported_contract": "stream-view/v1"' ;;
	wrong-commitment) subst "$fx/repo-a/docs/streams/shared/README.md" "Keep the stream readable without the chat history." "Keep the stream short." ;;
	wrong-reference) subst "$fx/repo-a/docs/streams/shared/README.md" "- docs/evidence/acceptance.md" "- docs/evidence/acceptance-report.md" ;;
	missing-file) rm "$fx/repo-a/docs/evidence/acceptance.md" || fail "missing-file: fixture file absent" ;;
	unplanned) subst "$fx/repo-a/docs/streams/shared/README.md" "planned: true" "planned: false" ;;
	wrong-unchecked) subst "$fx/expect.json" '"mission.success[0].evidence[1]: example-org/repo-a#12"' '"mission.success[0].evidence[1]: example-org/repo-a#13"' ;;
	*) fail "unknown mutation $id" ;;
	esac
}

case_mutation() {
	local entry id test want desc fx log rc observed=0
	for entry in "${mutations[@]}"; do
		IFS='|' read -r id test want desc <<<"$entry"
		fx="$work/mut-$id"
		cp -R "$fixtures" "$fx"
		apply_mutation "$id" "$fx"
		log="$work/mut-$id.log"
		run_tests "$fx" "$log" "$test"
		rc=$?
		if [ "$rc" -eq 0 ]; then
			cat "$log" >&2
			fail "mutation $id ($desc): $test exited 0 — the defect was not detected"
		fi
		if ! grep -qF "ASSERT-FAIL[$want]" "$log"; then
			cat "$log" >&2
			fail "mutation $id: exit $rc but no ASSERT-FAIL[$want] — a build or harness failure is not a detection"
		fi
		echo "RED   $id: $test exit $rc with ASSERT-FAIL[$want] — $desc"
		grep -F "ASSERT-FAIL[$want]" "$log" | head -n 1 | sed 's/^/        /'
		observed=$((observed + 1))
	done
	[ "$observed" -eq "${#mutations[@]}" ] || fail "observed $observed of ${#mutations[@]} mutations"
	log="$work/genuine.log"
	local all=(TestStreamViewLegacy TestStreamViewIdentity TestStreamViewNegative TestStreamViewFlow TestStreamViewDereference)
	run_tests "$fixtures" "$log" "${all[@]}"
	rc=$?
	[ "$rc" -eq 0 ] || { cat "$log" >&2; fail "genuine fixtures did not pass (exit $rc)"; }
	require_pass "$log" "${all[@]}"
	echo "CLEAN genuine fixtures: ${all[*]} pass"
	echo "mutation: $observed mutations detected, genuine fixtures accepted"
}

c="${1:-}"
case "$c" in
legacy) case_tests legacy TestStreamViewLegacy ;;
identity)
	case_tests identity TestStreamViewIdentity
	case_unit '^(TestIdentityHelpers)$' TestIdentityHelpers
	;;
negative)
	case_tests negative TestStreamViewNegative
	case_unit '^(TestDecodeVersionHandling|TestNegotiate|TestValidateRules)$' TestDecodeVersionHandling TestNegotiate TestValidateRules
	;;
flow)
	case_tests flow TestStreamViewFlow
	case_unit '^(TestEncodeDecodeRoundTrip)$' TestEncodeDecodeRoundTrip
	;;
dereference)
	case_tests dereference TestStreamViewDereference
	source_map
	;;
mutation) case_mutation ;;
all)
	for x in legacy identity negative flow dereference mutation; do
		echo "== $x"
		bash "$0" "$x" || exit $?
	done
	;;
*)
	echo "usage: bash tests/stream-brief/32.sh legacy|identity|negative|flow|dereference|mutation|all" >&2
	exit 2
	;;
esac
echo "PASS: $c"
