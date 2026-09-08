#!/usr/bin/env bash
# FAIL-FIRST REFERENCE — the decision exactly as the workflow made it BEFORE
# #586, transcribed from the "Request auto-merge" step's inline shell so the
# suite can be run against it and show which cases were red.
#
# It is a fixture, not a shipped path: nothing calls it but
# automerge-refusal_test.sh. It knows only two benign refusals (already
# enabled, repository auto-merge off); an "unstable status" refusal falls
# through to the ::error:: exit 1 that this repo observed on two Evidence pull
# requests, where the only failing check was this workflow's own — the failure
# that cannot clear itself.
#
#   CHECK_IMPL=testdata/old-automerge-refusal.sh ./automerge-refusal_test.sh
#
# It reads the same env contract as the real script so the harness is shared.
set -uo pipefail

: "${MUTATION_RC:?MUTATION_RC is required}"
: "${MUTATION_OUT:?MUTATION_OUT is required}"
PR="${PR:-?}"

if [ "$MUTATION_RC" = "0" ]; then
  echo "auto-merge requested on PR ${PR}; GitHub will merge once the required review and status checks are satisfied"
  exit 0
fi

out="$(cat "$MUTATION_OUT" 2>/dev/null || true)"

if printf '%s' "$out" | grep -qi 'already enabled'; then
  echo "auto-merge was already enabled on PR ${PR} — no-op"
  exit 0
fi
if printf '%s' "$out" | grep -qi 'not allowed for this repository'; then
  echo "repository auto-merge is disabled — skipping enable on PR ${PR} (benign; see #579)"
  exit 0
fi
echo "::error::could not enable auto-merge on PR ${PR}"
exit 1
