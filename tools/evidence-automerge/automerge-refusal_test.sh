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

# ── C12–C17: superseded runs (#1959). The rollup lists EVERY run on the head
#    commit; a workflow that cancels an in-flight run when a newer event arrives
#    leaves a CANCELLED run beside its replacement. Only the LATEST run per check
#    is judged, a check run keyed on (workflowName, name) and a status context on
#    its context. The pre-fix impl judges every listed run and reds C12, C13
#    and C17:
#      git show <pre-fix>:tools/evidence-automerge/automerge-refusal.sh > /tmp/pre.sh
#      CHECK_IMPL=/tmp/pre.sh ./automerge-refusal_test.sh
#    gh renders an unset timestamp as Go's zero time; Z0 is that literal.
Z0='0001-01-01T00:00:00Z'

# C12: a CANCELLED `changelog` run superseded by a later SUCCESS run of the same
#      name, beside our own failed `enable` → the enable-only case → benign skip.
ROLLUP_SUPERSEDED_DONE='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-09-30T13:00:11Z","completedAt":"2026-09-30T13:00:24Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-09-30T13:00:04Z","completedAt":"2026-09-30T13:00:06Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-09-30T13:00:12Z","completedAt":"2026-09-30T13:00:25Z"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"SUCCESS","startedAt":"2026-09-30T12:55:02Z"}]}'
run_case "C12 superseded CANCELLED beside a later SUCCESS skips" 0 1 "$ERR_UNSTABLE" "$ROLLUP_SUPERSEDED_DONE"

# C13: the shape at the moment the step reads the rollup — the successor of the
#      CANCELLED run and our own `enable` are both still IN_PROGRESS, so gh
#      renders their completedAt as the zero time. Zero time is "no stamp", not
#      "oldest stamp": startedAt decides, and the in-progress successor wins.
ROLLUP_SUPERSEDED_RUNNING='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T06:54:18Z","completedAt":"2026-10-04T06:54:25Z"},
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"IN_PROGRESS","conclusion":"","startedAt":"2026-10-04T06:59:19Z","completedAt":"'"$Z0"'"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-10-04T06:59:18Z","completedAt":"2026-10-04T06:59:19Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"IN_PROGRESS","conclusion":"","startedAt":"2026-10-04T06:59:22Z","completedAt":"'"$Z0"'"}]}'
run_case "C13 superseded CANCELLED beside an in-progress successor skips" 0 1 "$ERR_UNSTABLE" "$ROLLUP_SUPERSEDED_RUNNING"

# C14: the CANCELLED run is the LATEST of its name (an older SUCCESS precedes
#      it) → that check's current run is cancelled → a real failure, still reds.
ROLLUP_CANCELLED_LATEST='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-10-04T07:00:00Z","completedAt":"2026-10-04T07:00:10Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-10-04T07:05:00Z","completedAt":"2026-10-04T07:05:01Z"}]}'
run_case "C14 CANCELLED as the latest run of its name reds" 1 1 "$ERR_UNSTABLE" "$ROLLUP_CANCELLED_LATEST"

# C15: the CANCELLED run is superseded, but by a FAILURE → that check's latest
#      run is red → still reds. The reduction picks the run; it never forgives it.
ROLLUP_SUPERSEDED_BY_RED='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"CheckRun","workflowName":"ci","name":"build-test","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-10-04T07:00:00Z","completedAt":"2026-10-04T07:00:01Z"},
  {"__typename":"CheckRun","workflowName":"ci","name":"build-test","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:00:05Z","completedAt":"2026-10-04T07:04:00Z"}]}'
run_case "C15 CANCELLED superseded by a FAILURE reds" 1 1 "$ERR_UNSTABLE" "$ROLLUP_SUPERSEDED_BY_RED"

# C16: the only "successor" is a QUEUED run the forge has not stamped (zero time
#      everywhere). A stampless run sorts OLDEST and never displaces a run that
#      actually ran, so the CANCELLED run stays the latest → reds (fail-safe).
ROLLUP_STAMPLESS_SUCCESSOR='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-10-04T07:05:00Z","completedAt":"2026-10-04T07:05:01Z"},
  {"__typename":"CheckRun","workflowName":"changelog-check","name":"changelog","status":"QUEUED","conclusion":"","startedAt":"'"$Z0"'","completedAt":"'"$Z0"'"}]}'
run_case "C16 stampless queued successor does not displace (reds)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_STAMPLESS_SUCCESSOR"

# C17: a REAL rollup, captured from a public Evidence PR stranded by this defect
#      (#1959's third report): four superseded CANCELLED `changelog` runs, each
#      followed by a SUCCESS, and the latest `enable` red. Benign skip.
run_case "C17 real stranded-PR rollup skips" 0 1 "$ERR_UNSTABLE" \
  "$(cat "$here/testdata/rollup-superseded-cancelled.json")"

# C18: two DIFFERENT workflows each run a job named `build`. Workflow A's run is
#      an older FAILURE; workflow B's is a later SUCCESS. They are two checks, not
#      one, so A's FAILURE is still that check's latest run → reds. A key on the
#      job name alone would collapse them, let B's SUCCESS hide A's FAILURE and
#      skip — a loosening.
ROLLUP_SAME_NAME_TWO_WORKFLOWS='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"CheckRun","workflowName":"workflow-a","name":"build","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:00:00Z","completedAt":"2026-10-04T07:02:00Z"},
  {"__typename":"CheckRun","workflowName":"workflow-b","name":"build","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-10-04T07:01:00Z","completedAt":"2026-10-04T07:05:00Z"}]}'
run_case "C18 same job name in two workflows: later SUCCESS never hides a FAILURE (reds)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_SAME_NAME_TWO_WORKFLOWS"

# C19: a check run WITHOUT a workflowName has no identity to prove "same check"
#      with, so it is never de-duplicated (FAIL-CLOSED): a CANCELLED run beside a
#      later SUCCESS of the same name, neither carrying a workflowName, is judged
#      as listed → the CANCELLED run reds. (With workflowName this is C12, a skip.)
ROLLUP_NO_WORKFLOW_NAME='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-09-30T13:00:11Z","completedAt":"2026-09-30T13:00:24Z"},
  {"__typename":"CheckRun","name":"changelog","status":"COMPLETED","conclusion":"CANCELLED","startedAt":"2026-09-30T13:00:04Z","completedAt":"2026-09-30T13:00:06Z"},
  {"__typename":"CheckRun","name":"changelog","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-09-30T13:00:12Z","completedAt":"2026-09-30T13:00:25Z"}]}'
run_case "C19 check run without workflowName is not de-duplicated (fail-closed, reds)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_NO_WORKFLOW_NAME"

# C20: a commit status reported twice under one context — an older ERROR, then
#      a later SUCCESS — is one check keyed on its context: the latest is green,
#      so with only our own `enable` red → benign skip. A status context never
#      collides with a check run of the same name (C21).
ROLLUP_STATUS_SUPERSEDED='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"ERROR","startedAt":"2026-10-04T07:00:00Z"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"SUCCESS","startedAt":"2026-10-04T07:15:00Z"}]}'
run_case "C20 status context superseded by a later SUCCESS skips" 0 1 "$ERR_UNSTABLE" "$ROLLUP_STATUS_SUPERSEDED"

# C21: a FAILED commit status and a later SUCCESS check run share the name
#      `leak-sweep`. Different kinds, different keys → the status still reds.
ROLLUP_STATUS_VS_RUN='{"statusCheckRollup":[
  {"__typename":"CheckRun","workflowName":"evidence-automerge","name":"enable","status":"COMPLETED","conclusion":"FAILURE","startedAt":"2026-10-04T07:10:00Z","completedAt":"2026-10-04T07:10:09Z"},
  {"__typename":"StatusContext","context":"leak-sweep","state":"FAILURE","startedAt":"2026-10-04T07:00:00Z"},
  {"__typename":"CheckRun","workflowName":"leak-sweep","name":"leak-sweep","status":"COMPLETED","conclusion":"SUCCESS","startedAt":"2026-10-04T07:05:00Z","completedAt":"2026-10-04T07:06:00Z"}]}'
run_case "C21 status context and same-named check run stay separate (reds)" 1 1 "$ERR_UNSTABLE" "$ROLLUP_STATUS_VS_RUN"

# ── G1–G3: CLASS GUARD for #1959. THE CLASS: a non-Go consumer of a status-check
#    rollup (a shell or Python script, or a workflow `run:` block) that judges
#    failing conclusions over EVERY listed run instead of the latest run per
#    check (a check run keyed on (workflowName, name), a status context on its
#    context; C18 and C19 pin that key). A file is a CANDIDATE when it reads
#    `statusCheckRollup` and names a failing conclusion as a quoted literal; a
#    candidate is CLEAN only when it calls `latest_run_per_check(` (a `def` line
#    alone is not a call).
#    The Go consumers reduce through deskkit.LatestRunPerName and are out of
#    this guard's scope.
#
# scan_rollup_judges <root> → prints "CANDIDATE <path>" for every candidate and
# "UNREDUCED <path>" for every candidate that does not reduce.
scan_rollup_judges() {
  local root="$1" f
  find "$root" \( -name .git -o -name node_modules -o -name testdata \) -prune -o \
       -type f \( -name '*.sh' -o -name '*.bash' -o -name '*.py' -o -name '*.yml' -o -name '*.yaml' \) \
       ! -name '*_test.sh' -print 2>/dev/null | LC_ALL=C sort | while IFS= read -r f; do
    grep -q 'statusCheckRollup' "$f" 2>/dev/null || continue
    grep -qE "[\"'](FAILURE|CANCELLED|TIMED_OUT|STARTUP_FAILURE|ACTION_REQUIRED|ERROR)[\"']" "$f" 2>/dev/null || continue
    echo "CANDIDATE ${f#"$root"/}"
    if ! grep -v 'def latest_run_per_check(' "$f" | grep -q 'latest_run_per_check('; then
      echo "UNREDUCED ${f#"$root"/}"
    fi
  done
}

# G1: the real tree is clean, AND the guard still SEES the one real site (a
#     matcher that silently stopped matching would otherwise report clean).
repo_root="$(cd "$here/../.." && pwd)"
g1="$(scan_rollup_judges "$repo_root")"
if printf '%s\n' "$g1" | grep -qxF 'CANDIDATE tools/evidence-automerge/automerge-refusal.sh' \
   && ! printf '%s\n' "$g1" | grep -q '^UNREDUCED '; then
  ok "G1 class guard: real tree clean, real site seen ($(printf '%s\n' "$g1" | grep -c '^CANDIDATE ') candidate(s))"
else
  bad "G1 class guard on the real tree"
  printf '%s\n' "$g1" | sed 's/^/       | /'
fi

# G2: a PLANTED second instance — a script the fix does not touch, judging the
#     rollup without the reduction — is flagged, by name, beside a clean copy of
#     the real site.
plant="$WORK/plant"
mkdir -p "$plant/tools/evidence-automerge" "$plant/scripts"
cp "$CHECK" "$plant/tools/evidence-automerge/automerge-refusal.sh"
cat > "$plant/scripts/ci-gate.sh" <<'SH'
#!/usr/bin/env bash
gh pr view "$1" --json statusCheckRollup |
  jq -r '.statusCheckRollup[] | select(.conclusion == "CANCELLED" or .conclusion == "FAILURE") | .name'
SH
g2="$(scan_rollup_judges "$plant")"
if [ "$(printf '%s\n' "$g2" | grep '^UNREDUCED ')" = "UNREDUCED scripts/ci-gate.sh" ]; then
  ok "G2 class guard flags a planted second instance (scripts/ci-gate.sh)"
else
  bad "G2 class guard did not flag exactly the planted instance"
  printf '%s\n' "$g2" | sed 's/^/       | /'
fi

# G3: the real site with its reduction call reverted (judging `rollup` again,
#     the definition left in place) is flagged — a def alone is not a call.
mut="$WORK/mut"
mkdir -p "$mut/tools/evidence-automerge"
sed 's/in latest_run_per_check(rollup)/in rollup/' "$CHECK" > "$mut/tools/evidence-automerge/automerge-refusal.sh"
g3="$(scan_rollup_judges "$mut")"
if printf '%s\n' "$g3" | grep -qxF 'UNREDUCED tools/evidence-automerge/automerge-refusal.sh'; then
  ok "G3 class guard flags the real site with its reduction call reverted"
else
  bad "G3 class guard missed the reverted real site"
  printf '%s\n' "$g3" | sed 's/^/       | /'
fi

echo
echo "passed: $pass  failed: $fail  (impl: $CHECK)"
[ "$fail" -eq 0 ]
