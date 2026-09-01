#!/usr/bin/env bash
# RETIRED changelog-check logic, preserved ONLY as fail-first evidence for
# check_test.sh. This is the pre-fragment gate: it PASSES iff the PR adds a
# bullet line to CHANGELOG.md (a `## Unreleased` edit) OR carries changelog:skip.
# It has no concept of fragments and no deprecation guard — so under it the two
# NEW behaviours (green-on-fragment, refuse-Unreleased-edit) are wrong. Running
#   CHECK_IMPL=testdata/old-check.sh ./check_test.sh
# reddens exactly those cases; the default (check.sh) greens them. Not wired into
# any workflow.
set -euo pipefail
SKIP="${SKIP:-false}"
: "${BASE_SHA:?}"; : "${HEAD_SHA:?}"
if [ "$SKIP" = "true" ]; then
  echo "SKIP: changelog:skip label present."
  exit 0
fi
added="$(git diff "${BASE_SHA}...${HEAD_SHA}" -- CHANGELOG.md | grep -E '^\+[[:space:]]*- ' || true)"
if [ -n "$added" ]; then
  echo "PASS (retired): CHANGELOG.md highlight line added."
  exit 0
fi
echo "::error::(retired) no '## Unreleased' highlight line in CHANGELOG.md"
exit 1
