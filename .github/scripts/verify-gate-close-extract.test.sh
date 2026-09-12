#!/usr/bin/env bash
# Fixture test for the brief-id extraction + normalisation + grammar check
# used by .github/workflows/verify-gate-close.yml. This mirrors the workflow
# step's shell exactly (same extraction pipeline, same normalisation, same
# grammar regex) so a change to one is easy to keep in sync with the other —
# it is NOT invoked by the workflow itself, just kept parallel to it.
#
# Usage: bash .github/scripts/verify-gate-close-extract.test.sh
# Exits non-zero if any case fails.

set -u

# extract_brief ISSUE_BODY
# Prints the normalised brief id on stdout and returns 0 on a grammar pass,
# or prints nothing and returns 1 on a grammar failure / no marker found.
extract_brief() {
  local issue_body="$1"
  local brief

  brief=$(printf '%s' "$issue_body" \
    | grep -o '<!-- verify-gate: [^>]* -->' \
    | head -1 \
    | sed -E 's/<!-- verify-gate: (.*) -->/\1/' \
    | tr -d '[:space:]') || true

  if [ -z "$brief" ]; then
    return 1
  fi

  # Normalise the brief-v2 key <org>:<alias>:<stream>:<NN>[a] to the legacy
  # <stream>/<NN>[a] form before the grammar check, same as the workflow.
  if printf '%s' "$brief" | grep -qE '^[a-zA-Z0-9_-]+:[a-zA-Z0-9_-]+:[a-zA-Z0-9_-]+:[0-9]+[a-z]?$'; then
    brief=$(printf '%s' "$brief" | awk -F: '{print $3 "/" $4}')
  fi

  if ! printf '%s' "$brief" | grep -qE '^[a-zA-Z0-9_-]+/[0-9]+[a-z]?$'; then
    return 1
  fi

  printf '%s' "$brief"
  return 0
}

pass=0
fail=0

# check DESC ISSUE_BODY EXPECT_ACCEPT EXPECT_VALUE
check() {
  local desc="$1" body="$2" expect_accept="$3" expect_value="${4:-}"
  local got rc

  got=$(extract_brief "$body")
  rc=$?

  if [ "$expect_accept" = "accept" ]; then
    if [ "$rc" -eq 0 ] && [ "$got" = "$expect_value" ]; then
      echo "PASS: $desc -> $got"
      pass=$((pass + 1))
    else
      echo "FAIL: $desc -> rc=$rc got='$got' want='$expect_value'"
      fail=$((fail + 1))
    fi
  else
    if [ "$rc" -ne 0 ]; then
      echo "PASS: $desc -> rejected as expected"
      pass=$((pass + 1))
    else
      echo "FAIL: $desc -> accepted unexpectedly as '$got'"
      fail=$((fail + 1))
    fi
  fi
}

check "legacy stream/NN form" \
  '<!-- verify-gate: desk-tools/17 -->' \
  accept 'desk-tools/17'

check "brief-v2 key, hyphenated stream" \
  '<!-- verify-gate: assay:assay:desktools-go-git:02 -->' \
  accept 'desktools-go-git/02'

check "brief-v2 key, consumer alias, hyphenated stream" \
  '<!-- verify-gate: assay:mp:openbao-resilience:03 -->' \
  accept 'openbao-resilience/03'

check "legacy form with trailing letter suffix" \
  '<!-- verify-gate: dora-restore/01a -->' \
  accept 'dora-restore/01a'

check "path traversal payload rejected" \
  '<!-- verify-gate: ../../etc/passwd -->' \
  reject

check "shell metacharacter payload rejected" \
  '<!-- verify-gate: assay:assay:x:1; rm -rf / -->' \
  reject

check "brief-v2 key missing the trailing NN segment rejected" \
  '<!-- verify-gate: assay:assay:desk-tools -->' \
  reject

echo ""
echo "$pass passed, $fail failed"
if [ "$fail" -ne 0 ]; then
  exit 1
fi
exit 0
