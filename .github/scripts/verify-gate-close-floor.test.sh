#!/usr/bin/env bash
# Fixture test for the verifier-floor refusal branch of
# .github/workflows/verify-gate-close.yml (the two-stamp model, #1170).
#
# The workflow's "Advance the brief to done" step runs
# `statusgen --close-verify <brief>` and classifies a refusal by the phrase
# in statusgen's own output: `status is "done"` is a benign replay, a line
# carrying `verifier floor` is a below-floor Verified cell (REOPEN the card,
# comment runner + floor + remedy, commit nothing), anything else is a
# generic refusal (left closed). This script mirrors that classification
# EXACTLY — same command, same greps, same order — and runs it, with a real
# statusgen built from this tree, against the verifier-floor fixtures under
# statusgen/testdata/verifyfloor. It also mirrors the enforce step's
# allowlist normalisation so every case below is one an ALLOWLISTED HUMAN
# closed: that is the case that used to flip and must now refuse.
#
# It is NOT invoked by the workflow itself, just kept parallel to it, like
# verify-gate-close-extract.test.sh.
#
# Usage: bash .github/scripts/verify-gate-close-floor.test.sh [statusgen-binary]
# Exits non-zero if any case fails. With no argument it builds statusgen
# from ./statusgen into a temp dir (needs a Go toolchain).

set -u

here=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/verify-gate-floor.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

statusgen="${1:-}"
if [ -z "$statusgen" ]; then
  ( cd "$here/statusgen" && go build -o "$tmp/statusgen" . ) || { echo "FAIL: could not build statusgen"; exit 1; }
  statusgen="$tmp/statusgen"
fi

# allowed_human ALLOWED_CLOSERS SENDER_TYPE CLOSER
# Mirrors the enforce step: reduce the roster value (login:id pairs, comma-
# or space-separated) to bare logins and require type=User + membership.
allowed_human() {
  local allowed_closers="$1" sender_type="$2" closer="$3" allowed_logins
  allowed_logins=$(printf '%s' "$allowed_closers" \
    | tr ',' ' ' \
    | tr -s ' ' '\n' \
    | sed 's/:.*$//' \
    | grep -v '^$' \
    | tr '\n' ' ')
  [ -n "$allowed_logins" ] || return 1
  [ "$sender_type" = "User" ] || return 1
  case " $allowed_logins " in
    *" $closer "*) return 0 ;;
  esac
  return 1
}

# classify_close ROOT BRIEF
# Runs the workflow's close-verify command against ROOT and prints the branch
# the workflow would take on a FIRST attempt:
#   flipped        — statusgen wrote the done row
#   already-done   — benign replay (statusgen: status is "done")
#   floor-refused  — Verified cell below the verifier floor → reopen + comment
#   refused        — any other refusal → left closed
classify_close() {
  local root="$1" brief="$2" out
  if ! out=$(cd "$root" && "$statusgen" --root . --close-verify "$brief" 2>&1); then
    if printf '%s' "$out" | grep -q 'status is "done"'; then
      printf 'already-done'; return 0
    fi
    if printf '%s' "$out" | grep -q 'verifier floor'; then
      # The relayed comment must name the runner, the floor and the remedy —
      # the workflow posts $out verbatim, so assert on it here.
      for want in 'verifier floor' 'two-stamp' 're-verif' 'close the card again'; do
        printf '%s' "$out" | grep -q "$want" || { printf 'floor-refused-but-missing:%s' "$want"; return 0; }
      done
      printf 'floor-refused'; return 0
    fi
    printf 'refused'; return 0
  fi
  printf 'flipped'
}

pass=0
fail=0

# check DESC BRIEF EXPECT_BRANCH
# Each case is a fresh copy of the fixture tree closed by an allowlisted human.
check() {
  local desc="$1" brief="$2" expect="$3" root got status
  root="$tmp/case-$RANDOM$RANDOM"
  mkdir -p "$root"
  cp -R "$here/statusgen/testdata/verifyfloor/." "$root/"

  if ! allowed_human "reviewer:1001, other:1002" "User" "reviewer"; then
    echo "FAIL: $desc -> the enforce step did not admit the allowlisted human closer"
    fail=$((fail + 1)); return
  fi

  got=$(classify_close "$root" "$brief")
  status=$(grep -E "^\| ${brief#vf/} \|" "$root/docs/streams/vf/README.md" | awk -F'|' '{gsub(/ /,"",$6); print $6}')

  if [ "$got" = "$expect" ]; then
    case "$expect" in
      flipped)
        if [ "$status" = "done" ]; then
          echo "PASS: $desc -> $got (row now done)"; pass=$((pass + 1))
        else
          echo "FAIL: $desc -> $got but README status is '$status', want done"; fail=$((fail + 1))
        fi ;;
      *)
        if [ "$status" != "done" ]; then
          echo "PASS: $desc -> $got (row left at $status, nothing written)"; pass=$((pass + 1))
        else
          echo "FAIL: $desc -> $got but README row was written to done"; fail=$((fail + 1))
        fi ;;
    esac
  else
    echo "FAIL: $desc -> got '$got', want '$expect' (README status '$status')"
    fail=$((fail + 1))
  fi
}

check "allowlisted human closes a card whose Verified cell is a local-tier (below-floor) stamp" \
  'vf/01' floor-refused

check "allowlisted human closes after a floor-tier re-verify stamp leads the cell" \
  'vf/02' flipped

check "allowlisted human closes an implemented card whose only recorded pass is below the floor" \
  'vf/03' floor-refused

check "allowlisted human closes a card whose cell clears but whose Evidence rows never did" \
  'vf/04' floor-refused

echo ""
echo "$pass passed, $fail failed"
if [ "$fail" -ne 0 ]; then
  exit 1
fi
exit 0
