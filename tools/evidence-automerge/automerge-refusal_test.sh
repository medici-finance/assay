#!/usr/bin/env bash
# Offline, network-free unit test for the evidence-automerge refusal decision
# (automerge-refusal.sh). Every case is a pair of files on disk — the mutation's
# output and a status-check rollup — so no GitHub, no token and no runner is
# needed; bash and python3 are the whole toolchain.
#
# Default impl is ../automerge-refusal.sh. Point CHECK_IMPL at
# testdata/old-automerge-refusal.sh to see the NEW behaviour fail against the
# pre-#586 logic — the committed fail-first evidence:
#
#   CHECK_IMPL=testdata/old-automerge-refusal.sh ./automerge-refusal_test.sh  # RED on C4
#   ./automerge-refusal_test.sh                                               # all green
#
# Exit 0 when every case matches, 1 otherwise.
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="${CHECK_IMPL:-$here/automerge-refusal.sh}"
case "$CHECK" in /*) ;; *) CHECK="$here/$CHECK" ;; esac

pass=0; fail=0
ok()  { echo "ok   - $1"; pass=$((pass+1)); }
bad() { echo "FAIL - $1"; fail=$((fail+1)); }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/automerge-refusal-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

# run_case <name> <expected-exit> <mutation-rc> <mutation-output> <rollup-json>
# A rollup of the literal string ABSENT stands for "no rollup file at all".
run_case() {
  local name="$1" want="$2" rc="$3" out="$4" rollup="$5"
  local outf="$WORK/out.txt" rollupf="$WORK/rollup.json" got log
  printf '%s\n' "$out" > "$outf"
  if [ "$rollup" = "ABSENT" ]; then
    rm -f "$rollupf"
  else
    printf '%s\n' "$rollup" > "$rollupf"
  fi
  log="$(MUTATION_RC="$rc" MUTATION_OUT="$outf" ROLLUP_JSON="$rollupf" \
         SELF_CHECK=enable PR=568 bash "$CHECK" 2>&1)"
  got=$?
  if [ "$got" = "$want" ]; then
    ok "$name (exit $got)"
  else
    bad "$name (want exit $want, got $got)"
    printf '%s\n' "$log" | sed 's/^/       | /'
  fi
}

# The two refusal texts GitHub actually returns, as the gh CLI prints them.
ERR_UNSTABLE='gh: Pull request is in unstable status (enablePullRequestAutoMerge)'
ERR_DISABLED='gh: Auto merge is not allowed for this repository (enablePullRequestAutoMerge)'
ERR_ALREADY='gh: ["Pull request Auto merge is already enabled"] (enablePullRequestAutoMerge)'
ERR_PERM='gh: Resource not accessible by integration (enablePullRequestAutoMerge)'
# The refusal a just-flipped approved+green PR gets: it already satisfies every
# merge requirement, so there is nothing for auto-merge to wait on. Sibling of
# ERR_UNSTABLE on the same GraphQL surface, one mergeStateStatus over.
ERR_CLEAN='gh: Pull request is in clean status (enablePullRequestAutoMerge)'
OK_JSON='{"data":{"enablePullRequestAutoMerge":{"pullRequest":{"number":568}}}}'

# Rollups. `enable` is this workflow's own check run; the others are real legs.
ROLLUP_ONLY_ENABLE='{"statusCheckRollup":[
  {"__typename":"CheckRun","name":"enable","status":"COMPLETED","conclusion":"FAILURE"},
  {"__typename":"CheckRun","name":"build-test","status":"COMPLETED","conclusion":"SUCCESS"},
  {"__typename":"CheckRun","name":"changelog","status":"COMPLETED","conclusion":"SUCCESS"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"SUCCESS"}]}'
ROLLUP_OTHER_RED='{"statusCheckRollup":[
  {"__typename":"CheckRun","name":"enable","status":"COMPLETED","conclusion":"FAILURE"},
  {"__typename":"CheckRun","name":"build-test","status":"COMPLETED","conclusion":"FAILURE"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"SUCCESS"}]}'
ROLLUP_STATUS_RED='{"statusCheckRollup":[
  {"__typename":"CheckRun","name":"enable","status":"COMPLETED","conclusion":"FAILURE"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"FAILURE"}]}'
ROLLUP_ENABLE_RERUN='{"statusCheckRollup":[
  {"__typename":"CheckRun","name":"enable","status":"IN_PROGRESS","conclusion":null},
  {"__typename":"CheckRun","name":"build-test","status":"QUEUED","conclusion":null},
  {"__typename":"StatusContext","context":"leak-sweep","state":"SUCCESS"}]}'
ROLLUP_NULL='{"statusCheckRollup":null}'
ROLLUP_GARBAGE='not json at all {{{'

# ── C0: the mutation was accepted → exit 0, and no rollup is consulted.
run_case "C0 success path unchanged" 0 0 "$OK_JSON" ABSENT

# ── C1: repository auto-merge is OFF → benign skip, unchanged by this fix (#579).
run_case "C1 repository auto-merge disabled skips" 0 1 "$ERR_DISABLED" ABSENT

# ── C2: already enabled → benign replay, unchanged by this fix.
run_case "C2 already-enabled replay skips" 0 1 "$ERR_ALREADY" ABSENT

# ── C3: a genuine failure (no permission) → still reddens.
run_case "C3 permission failure reds" 1 1 "$ERR_PERM" ABSENT

# ── C4: unstable, and the ONLY failing check is this workflow's own `enable`
#        → benign skip. THE FIX. The pre-#586 impl reds here, which is the jam:
#        the red publishes the very check that makes the next request refuse.
run_case "C4 enable-only unstable skips" 0 1 "$ERR_UNSTABLE" "$ROLLUP_ONLY_ENABLE"

# ── C5: unstable, and another CHECK RUN is red → real failure, still reds.
run_case "C5 other check-run red reds" 1 1 "$ERR_UNSTABLE" "$ROLLUP_OTHER_RED"

# ── C6: unstable, and a legacy commit STATUS is red → real failure, still reds.
#        (The rollup mixes CheckRun and StatusContext shapes; both are read.)
run_case "C6 other commit-status red reds" 1 1 "$ERR_UNSTABLE" "$ROLLUP_STATUS_RED"

# ── C7: unstable during a re-run — our own check is IN_PROGRESS and another is
#        QUEUED, so nothing is FAILING. Queued and running are not red, so this
#        is still the enable-only case → benign skip.
run_case "C7 rerun with nothing failing skips" 0 1 "$ERR_UNSTABLE" "$ROLLUP_ENABLE_RERUN"

# ── C8: unstable, but the rollup file is absent → could-not-check → reds.
run_case "C8 unstable with no rollup reds (could-not-check)" 1 1 "$ERR_UNSTABLE" ABSENT

# ── C9: unstable, but the rollup is unparseable → could-not-check → reds.
run_case "C9 unstable with garbage rollup reds (could-not-check)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_GARBAGE"

# ── C10: unstable, but statusCheckRollup is null → could-not-check → reds.
#         A null rollup is not "no failing checks"; the question went unanswered.
run_case "C10 unstable with null rollup reds (could-not-check)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_NULL"

# ── C11: the PR already satisfies every requirement (a just-flipped approved,
#         green PR) → GitHub refuses with "clean status". THE FIFTH-CASE FIX for
#         #586: benign skip, exit 0. The pre-fix impl reds here — the refusal
#         publishes the very check that keeps the PR from self-clearing, exactly
#         as the unstable case did. No rollup is consulted; the string alone
#         decides. Run CHECK_IMPL=testdata/old-automerge-refusal.sh to see it red.
run_case "C11 clean-status refusal skips" 0 1 "$ERR_CLEAN" ABSENT

echo
echo "passed: $pass  failed: $fail  (impl: $CHECK)"
[ "$fail" -eq 0 ]
