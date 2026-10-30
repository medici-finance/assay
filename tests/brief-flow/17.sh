#!/usr/bin/env bash
# Offline acceptance runner for the wait, context and human-intervention
# records of brief-flow-event/v1 (spec/brief-flow-event-v1.md §14–§18,
# statusgen/briefintervention.go) and their two projection profiles.
#
#   bash tests/brief-flow/17.sh <mode>
#
# Modes: compatibility | contract-context | intervention-contract | dedup |
#        identity-sinks | publication-profile | flow | mutation | all
#
# Each behavioural mode runs its targeted Go tests against the synthetic
# fixtures under statusgen/testdata/brief-flow/interventions and requires every
# named test to report PASS. `compatibility` also requires the 07.sh contract
# tests, so the earlier contract is re-proved under the extended schema.
# `mutation` copies the module, corrupts one reducer or guard at a time and
# exits 0 only after it has observed, for EVERY mutation, a nonzero exit
# carrying that mutation's own ASSERT-FAIL[<id>] markers — then the genuine
# code passing.
#
# Exit codes: 0 pass, 1 fail, 2 could-not-check. A missing prerequisite, a
# missing fixture, a missing test or a void mutation anchor never passes.
# Offline by construction: no network, no forge, no credentials.
set -u -o pipefail

here17="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# The helpers (build, run_tests, require_pass, mode_tests, subst,
# copy_replace_targets, dereference, cnc, fail) and the /07 prerequisite checks.
# shellcheck source=07.sh
BRIEFFLOW_LIB=1 . "$here17/07.sh"

ifixtures="$pkg/testdata/brief-flow/interventions"
for f in briefintervention.go briefintervention_test.go history.go backfill.go; do
	[ -f "$pkg/$f" ] || cnc "statusgen/$f not found"
done
for f in contract.jsonl failed.jsonl dedup.jsonl resubmit.jsonl waits.jsonl flow.jsonl context.jsonl expect.json; do
	[ -f "$ifixtures/$f" ] || cnc "fixture $f not found under statusgen/testdata/brief-flow/interventions"
done
[ -d "$pkg/testdata/goodrepo" ] || cnc "statusgen/testdata/goodrepo not found"

# id ^ test ^ expected assertion(s, comma-separated) ^ file ^ from ^ to ^ description
mutations17=(
	'retry-dedup^TestActDedup^act-dedup^briefintervention.go^if !ok { // act-dedup^if true || !ok { // act-dedup^a retry or mirror of one act is stored as a new act'
	'multi-brief-union^TestActDedup^act-fleet-union^briefintervention.go^if seen[id] { // fleet-union^if false { // fleet-union^an act linked to two briefs counted twice fleet-wide'
	'missing-as-zero^TestActContract^act-missing-zero^briefintervention.go^known := cov == "observed" || cov == "partial" // coverage-known^known := true // coverage-known^unknown intervention coverage rendered as zero counts'
	'wait-missing-zero^TestFlowWaits^wait-no-data^briefintervention.go^if len(durs) == 0 { // wait-no-data^if false { // wait-no-data^an edge with no completed wait rendered as 0'
	'forbidden-field^TestActPublication^pub-allowlist^briefintervention.go^"as_of":           leaf{}, // committed-as-of^"as_of": leaf{}, "act_source": leaf{}, // committed-as-of^the source pointer admitted to the committed profile'
	'policy-off^TestActPublication^pub-held^briefintervention.go^if !policy.permits(source) {^if false && !policy.permits(source) {^interventions published with no recorded publication policy'
	'historian-read^TestActSinks^sinks-historian-read^history.go^if err := refuseHumanActLine([]byte(line)); err != nil { // historian-sink-read^if err := refuseHumanActLine(nil); err != nil { // historian-sink-read^the historian reads a human-act line'
	'record-scan-off^TestActSinks^sinks-login^briefevent.go^if err := scanPerson(m, ""); err != nil {^if err := scanPerson(nil, ""); err != nil {^person-identifier scan disabled for every record kind'
)

mode_mutation17() {
	local entry id test wants want want_list file from to desc copy mbin log rc observed=0
	for entry in "${mutations17[@]}"; do
		IFS='^' read -r id test wants file from to desc <<<"$entry"
		copy="$work/mut17-$id"
		mkdir -p "$copy/docs/streams/graph-execution"
		cp -R "$pkg" "$copy/statusgen" || cnc "could not copy the module"
		cp -R "$repo/schemas" "$copy/schemas" || cnc "could not copy schemas"
		copy_replace_targets "$repo" statusgen "$copy"
		cp "$runrec" "$copy/docs/streams/graph-execution/" || cnc "could not copy the run-record brief"
		subst "$copy/statusgen/$file" "$from" "$to"
		mbin="$work/mut17-$id.test"
		build "$copy/statusgen" "$mbin" || { cat "$work/build.log" >&2; fail "mutation $id did not compile — a build failure is not a detection"; }
		log="$work/mut17-$id.log"
		run_tests "$copy/statusgen" "$mbin" "$log" "$test"
		rc=$?
		if [ "$rc" -eq 0 ]; then
			cat "$log" >&2
			fail "mutation $id ($desc): $test exited 0 — the defect was not detected"
		fi
		echo "RED   $id: $test exit $rc — $desc"
		IFS=',' read -r -a want_list <<<"$wants"
		for want in "${want_list[@]}"; do
			if ! grep -qF "ASSERT-FAIL[$want]" "$log"; then
				cat "$log" >&2
				fail "mutation $id: exit $rc but no ASSERT-FAIL[$want]"
			fi
			grep -F "ASSERT-FAIL[$want]" "$log" | head -n 1 | sed 's/^ */        /'
		done
		observed=$((observed + 1))
		rm -rf "$copy" "$mbin"
	done
	[ "$observed" -eq "${#mutations17[@]}" ] || fail "observed $observed of ${#mutations17[@]} mutations"
	mode_tests genuine TestActDedup TestActContract TestFlowWaits TestActPublication TestActSinks
	echo "PASS  mutation: $observed of ${#mutations17[@]} mutations detected; genuine code passes"
}

mode17="${1:-}"
case "$mode17" in
compatibility) mode_tests compatibility TestFlowCompat TestBriefEventCompat TestBriefEventSchemaParity ;;
contract-context) mode_tests contract-context TestFlowContext ;;
intervention-contract) mode_tests intervention-contract TestActContract ;;
dedup) mode_tests dedup TestActDedup ;;
identity-sinks) dereference && mode_tests identity-sinks TestActSinks TestBriefEventRoles ;;
publication-profile) mode_tests publication-profile TestActPublication TestFlowWaits ;;
flow) mode_tests flow TestActFlow TestFlowWaits ;;
mutation) mode_mutation17 ;;
all)
	mode_tests compatibility TestFlowCompat TestBriefEventCompat TestBriefEventSchemaParity
	mode_tests contract-context TestFlowContext
	mode_tests intervention-contract TestActContract
	mode_tests dedup TestActDedup
	dereference
	mode_tests identity-sinks TestActSinks TestBriefEventRoles
	mode_tests publication-profile TestActPublication TestFlowWaits
	mode_tests flow TestActFlow TestFlowWaits
	mode_mutation17
	;;
*) echo "usage: $0 compatibility|contract-context|intervention-contract|dedup|identity-sinks|publication-profile|flow|mutation|all" >&2; exit 2 ;;
esac
