#!/usr/bin/env bash
# evidence-automerge — the "what does this enablePullRequestAutoMerge answer
# mean?" decision, extracted so it is unit-testable offline
# (tools/evidence-automerge/automerge-refusal_test.sh) and so the workflow step
# that calls it stays thin. The workflow makes the mutation and reads the check
# rollup; this script decides, and nothing here touches the network.
#
# WHY IT EXISTS. `enablePullRequestAutoMerge` is a REQUEST, not a merge. Most of
# its refusals say something about the repository or the pull request's current
# state, not about this workflow having malfunctioned — and a refusal that
# reddens the run publishes a FAILED check, which is itself a state that makes
# the next request refuse. That feedback loop is the defect this script closes.
#
# FOUR BENIGN OUTCOMES, exit 0 with a reason printed:
#   1. SUCCESS — the mutation was accepted.
#   2. ALREADY ENABLED — a second synchronize, or a review after a push. GitHub
#      answers with a "... already enabled" style error. Redundant, not wrong.
#   3. REPOSITORY AUTO-MERGE OFF — "Auto merge is not allowed for this
#      repository". Enabling auto-merge is an OPTIMISATION, not the merge itself:
#      the required review and status checks still gate the merge and a human can
#      merge directly, so the setting being off is a skip, not a failure.
#   4. ENABLE-ONLY UNSTABLE — "Pull request is in unstable status" while the ONLY
#      failing check is this workflow's own. This is the self-inflicted case: a
#      previous refusal published a failed check run, the failed check run makes
#      the pull request unstable, and unstable makes the mutation refuse. A
#      re-run can never clear it, because the re-run's own red is the blocker.
#      Exiting 0 here greens that check; the next pull-request or review event
#      finds a clean pull request and enables auto-merge. The blast radius of
#      being wrong is the same as case 3 — a pull request that needs a human's
#      merge click — because nothing here merges anything.
#
# EVERYTHING ELSE REDDENS, exit 1. In particular: a refusal for "unstable" while
# some OTHER check is failing is a real signal — that pull request is not
# mergeable for a reason this workflow did not cause, and the red run says so.
#
# THREE-STATE, and could-not-check is a FAILURE. If the check rollup cannot be
# read or parsed, the "is our own check the only failing one?" question was not
# answered, and an unanswered question is never rounded up to "yes".
#
# INPUTS (env):
#   MUTATION_RC   the mutation command's exit status ("0" = accepted)
#   MUTATION_OUT  path to a file holding the mutation's combined output
#   ROLLUP_JSON   path to a file holding `gh pr view --json statusCheckRollup`
#                 output. Read ONLY for the unstable class; may be empty or
#                 absent otherwise.
#   SELF_CHECK    the name of the check run THIS workflow publishes (its job
#                 name), so the decision cannot drift from the job
#   PR            the pull-request number, for the messages
#
# Needs bash + python3 (the runner ships both; python3 is the same parser
# dependency the changelog gate already has). No git, no gh, no network.
set -uo pipefail

: "${MUTATION_RC:?MUTATION_RC is required}"
: "${MUTATION_OUT:?MUTATION_OUT is required}"
SELF_CHECK="${SELF_CHECK:?SELF_CHECK is required}"
PR="${PR:-?}"

# 1) SUCCESS. The mutation was accepted; say what was requested and stop.
if [ "$MUTATION_RC" = "0" ]; then
  echo "auto-merge requested on PR ${PR}; GitHub will merge once the required review and status checks are satisfied"
  exit 0
fi

out="$(cat "$MUTATION_OUT" 2>/dev/null || true)"

# 2) ALREADY ENABLED — a benign replay.
if printf '%s' "$out" | grep -qi 'already enabled'; then
  echo "auto-merge was already enabled on PR ${PR} — no-op"
  exit 0
fi

# 3) REPOSITORY AUTO-MERGE OFF — a benign skip (see #579).
if printf '%s' "$out" | grep -qi 'not allowed for this repository'; then
  echo "repository auto-merge is disabled — skipping enable on PR ${PR} (benign; see #579)"
  exit 0
fi

# 4) UNSTABLE — benign ONLY when this workflow's own check is the sole red one.
if printf '%s' "$out" | grep -qi 'unstable status'; then
  if [ -z "${ROLLUP_JSON:-}" ] || [ ! -s "${ROLLUP_JSON}" ]; then
    echo "could-not-check: the status-check rollup for PR ${PR} could not be read"
    echo "::error::could not enable auto-merge on PR ${PR} — refused as unstable, and the check rollup that would say whether that is self-inflicted could not be read"
    exit 1
  fi

  # Every FAILING entry's name, one per line. A check that is queued, running,
  # successful, neutral or skipped is not failing and is not listed: "unstable"
  # is GitHub's word for a pull request whose non-required checks include a
  # failure, so a failure is what the question is about.
  #
  # The rollup mixes two node shapes — CheckRun (`name` + `conclusion`) and the
  # legacy StatusContext (`context` + `state`) — and both are read, so a failing
  # commit status is never invisible to this decision.
  if ! failing="$(python3 - "$ROLLUP_JSON" <<'PY'
import json, sys

FAIL_CONCLUSIONS = {
    "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE", "STALE",
}
FAIL_STATES = {"FAILURE", "ERROR"}

try:
    with open(sys.argv[1]) as fh:
        doc = json.load(fh)
except Exception as err:                                  # unreadable / not JSON
    print("rollup could not be parsed: %s" % err, file=sys.stderr)
    sys.exit(2)

rollup = doc.get("statusCheckRollup") if isinstance(doc, dict) else None
if not isinstance(rollup, list):                          # null, missing, wrong shape
    print("rollup JSON carries no statusCheckRollup list", file=sys.stderr)
    sys.exit(2)

for entry in rollup:
    if not isinstance(entry, dict):
        continue
    conclusion = str(entry.get("conclusion") or "").upper()
    state = str(entry.get("state") or "").upper()
    if conclusion in FAIL_CONCLUSIONS or state in FAIL_STATES:
        print(entry.get("name") or entry.get("context") or "(unnamed check)")
PY
  )"; then
    echo "could-not-check: the status-check rollup for PR ${PR} could not be parsed"
    echo "::error::could not enable auto-merge on PR ${PR} — refused as unstable, and the check rollup that would say whether that is self-inflicted could not be parsed"
    exit 1
  fi

  others="$(printf '%s\n' "$failing" | sed '/^$/d' | grep -vxF "$SELF_CHECK" || true)"
  if [ -n "$others" ]; then
    echo "PR ${PR} is unstable and these checks are failing besides '${SELF_CHECK}':"
    printf '%s\n' "$others" | sed 's/^/  /'
    echo "::error::could not enable auto-merge on PR ${PR}"
    exit 1
  fi

  echo "::warning::auto-merge not enabled on PR ${PR}: the request was refused as 'unstable status' and the only failing check is this workflow's own '${SELF_CHECK}'. That is self-inflicted and self-clearing — this run greens that check, and the next pull-request or review event enables auto-merge. Benign; see #586."
  exit 0
fi

# Everything else is a real failure.
echo "::error::could not enable auto-merge on PR ${PR}"
exit 1
