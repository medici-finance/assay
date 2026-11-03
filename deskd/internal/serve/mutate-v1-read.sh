#!/usr/bin/env bash
# mutate-v1-read.sh — fail-first evidence for the guards on the /v1 local-state
# read marker and the named-action body decoder (the service contract increment 4,
# worker-prompt kit §9).
#
# WHY MUTATION AND NOT ONLY A PRE-FIX RED RUN.
#
# The per-item repo binding and the authentication-before-existence ordering
# of handleV1Read are CORRECT at the commit whose tests were too weak to pin
# them — so a pre-fix red run of the strengthened tests shows nothing: they
# pass against correct code. The evidence that a test pins a guard is that it
# goes red when the guard is removed. Each case below removes or inverts
# exactly ONE control, the way a plausible future edit would, and asserts the
# matching test then FAILS.
#
# It never edits the working tree: the module is copied to a tempdir, the
# mutation is applied to the copy, and the copy is thrown away. Each case also
# checks that its mutation actually applied — a perl substitution that matched
# nothing would leave the tree unmutated and "prove" nothing.
#
# Usage:  internal/serve/mutate-v1-read.sh
# Exit:   0 every guard proved load-bearing
#         1 a guard is not load-bearing (or a mutation failed to apply)
#         2 usage / environment error
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-v1-read.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0

# fresh_copy <name> — a pristine copy of the module, path echoed.
fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"; mkdir -p "$d"
  (cd "$repo" && tar --exclude='./.git' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

# applied <label> <pristine> <mutated> — refuse a mutation that changed nothing.
applied() {
  if cmp -s "$2" "$3"; then
    fails=$((fails + 1)); cases=$((cases + 1))
    printf 'MUTATION DID NOT APPLY: %s (the source no longer matches the pattern — update this script)\n' "$1"
    return 1
  fi
}

# expect_red <label> <dir> <pkg> <test-regex>
expect_red() {
  local label=$1 dir=$2 pkg=$3 re=$4
  cases=$((cases + 1))
  local out
  out=$(cd "$dir" && go test -count=1 -timeout 180s -run "$re" "$pkg" 2>&1)
  if [ $? -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT LOAD-BEARING: %s\n  %s %s still PASSES with the guard removed\n' "$label" "$pkg" "$re"
  elif ! printf '%s\n' "$out" | grep -q -- '--- FAIL'; then
    # A build error is not a red: it proves nothing about the test.
    fails=$((fails + 1))
    printf 'NO TEST RED (build or setup failure): %s\n' "$label"
    printf '%s\n' "$out" | head -4 | sed 's/^/    /'
  else
    printf 'red as expected: %s\n' "$label"
    printf '%s\n' "$out" | grep -E '^\s+[a-z_0-9]+\.go:[0-9]+' | head -3 | sed 's/^/    /'
  fi
}

local_go=internal/serve/v1_local.go
actions_go=internal/serve/v1_actions.go

echo "== F-read-item-binding: the repo comes from the index row, never the request =="

d=$(fresh_copy query-on)
# Correctness-lane mutation: authorize on request-supplied o/n.
# (`_, _ = owner, name` keeps the now-unused index values compiling.)
perl -pi -e 's/^\t\tauthenticated, allowed := authz\.LocalStateAllowed\(r, owner, name\)$/\t\t_, _ = owner, name\n\t\tauthenticated, allowed := authz.LocalStateAllowed(r, r.URL.Query().Get("o"), r.URL.Query().Get("n"))/' "$d/$local_go"
applied "request query o/n" "$repo/$local_go" "$d/$local_go" &&
  expect_red "request query o/n steers the authorized repo" "$d" ./internal/serve/ 'TestV1ReadBindsIndexedRepo'

d=$(fresh_copy query-repo)
# Security-lane mutation: a ?repo= override before the gate.
perl -pi -e 's/^(\t\tauthenticated, allowed := authz\.LocalStateAllowed\(r, owner, name\))$/\t\tif q := r.URL.Query().Get("repo"); q != "" {\n\t\t\towner, name, _ = strings.Cut(q, "\/")\n\t\t}\n$1/' "$d/$local_go"
applied "request ?repo=" "$repo/$local_go" "$d/$local_go" &&
  expect_red "request ?repo= steers the authorized repo" "$d" ./internal/serve/ 'TestV1ReadBindsIndexedRepo'

d=$(fresh_copy header-repo)
# The same shape through a header.
perl -pi -e 's/^(\t\tauthenticated, allowed := authz\.LocalStateAllowed\(r, owner, name\))$/\t\tif q := r.Header.Get("X-Desk-Repo"); q != "" {\n\t\t\towner, name, _ = strings.Cut(q, "\/")\n\t\t}\n$1/' "$d/$local_go"
applied "request header" "$repo/$local_go" "$d/$local_go" &&
  expect_red "a request header steers the authorized repo" "$d" ./internal/serve/ 'TestV1ReadBindsIndexedRepo'

echo
echo "== F-read-existence-before-authn: authentication is decided before existence =="

d=$(fresh_copy found-first)
# Move `case !found:` (404) ahead of `case !authenticated:` (401).
perl -0pi -e 's/(\t\tcase !authenticated:\n(?:.*\n)*?\t\t\treturn\n)(\t\tcase idxErr != nil:\n(?:.*\n)*?\t\t\treturn\n)(\t\tcase !found:\n(?:.*\n)*?\t\t\treturn\n)/$3$1$2/' "$d/$local_go"
applied "404 before 401" "$repo/$local_go" "$d/$local_go" &&
  expect_red "an unauthenticated caller can tell a missing id (404) from a present one (401)" "$d" ./internal/serve/ 'TestV1ReadRouteIsGatedPerItem'

d=$(fresh_copy idxerr-first)
# Move the index-failure 500 ahead of authentication (the pre-fix order).
perl -0pi -e 's/(\t\tcase !authenticated:\n(?:.*\n)*?\t\t\treturn\n)(\t\tcase idxErr != nil:\n(?:.*\n)*?\t\t\treturn\n)/$2$1/' "$d/$local_go"
applied "500 before 401" "$repo/$local_go" "$d/$local_go" &&
  expect_red "an index failure is answered before authentication" "$d" ./internal/serve/ 'TestV1ReadAuthnBeforeIndexErr'

echo
echo "== A-decode-one-value / A-413-any-read-error: the body decoder =="

d=$(fresh_copy more-only)
# Revert the tail check to Decoder.More, which is false before a closing } or ].
perl -pi -e 's/^\tif _, err := dec\.Token\(\); err != io\.EOF \{$/\tif dec.More() {/' "$d/$actions_go"
applied "More()-only tail check" "$repo/$actions_go" "$d/$actions_go" &&
  expect_red "trailing } or ] after the value is refused (and never writes)" "$d" ./internal/serve/ 'TestAnswerRefusesNonContractBodies'

d=$(fresh_copy all-413)
# Map every body-read error to 413 again.
perl -pi -e 's/^\t\treturn http\.StatusBadRequest, fmt\.Errorf\("request body unreadable: %v", err\)$/\t\treturn http.StatusRequestEntityTooLarge, fmt.Errorf("request body unreadable: %v", err)/' "$d/$actions_go"
applied "every read error is 413" "$repo/$actions_go" "$d/$actions_go" &&
  expect_red "a non-size read error is 400, not 413" "$d" ./internal/serve/ 'TestActionBodyReadErrStatus'

echo
if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails of $cases guard(s) are not load-bearing"
  exit 1
fi
echo "OK: all $cases guards proved load-bearing (each test went red with its guard removed)"
