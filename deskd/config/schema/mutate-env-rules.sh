#!/usr/bin/env bash
# mutate-env-rules.sh — fail-first evidence for the dev-only environment
# validator (the service contract, worker-prompt kit §9).
#
# WHY THIS EXISTS, and why a plain "run the test before the fix" run does not
# do the job here.
#
# The obvious fail-first for a new validator is "check out the pre-fix commit
# and watch the red fixtures fail to be rejected". That run is WORTHLESS for
# this change, and knowing why is the whole point of this script: before this
# brief, `environments:` was an unknown field, and cells.go decodes with
# yaml.Decoder.KnownFields(true). So every one of the four red fixtures was
# already rejected by the pre-fix code — with rc=1, looking exactly like a
# pass — for the entirely unrelated reason that the block did not parse. A
# fail-first run that goes red for the wrong reason is worse than none: it
# certifies a rule nobody has actually observed working.
#
# So the red is produced by MUTATION instead. Each case below widens exactly
# one rule — the way a future edit plausibly would — and asserts that the
# matching fixture then VALIDATES. If a fixture is still rejected after its
# rule is removed, that rule is not what was rejecting it, and the check is
# not pinning what it claims to pin. This script fails in that case.
#
# It never edits the working tree: the module is copied to a tempdir, the
# mutation is applied to the copy, and the copy is thrown away.
#
# Usage:  config/schema/mutate-env-rules.sh
# Exit:   0 every rule proved load-bearing
#         1 a rule is not load-bearing (or a control assertion regressed)
#         2 usage / environment error
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-env.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0

# fresh_copy <name> — a pristine copy of the module, path echoed.
fresh_copy() {
  local d="$tmp/$1"
  rm -rf "$d"
  mkdir -p "$d"
  # .git is deliberately excluded: nothing here needs history, and copying it
  # into a tempdir a trap deletes is a good way to lose a worktree by typo.
  (cd "$repo" && tar --exclude='./.git' -cf - .) | (cd "$d" && tar -xf -)
  echo "$d"
}

# validates <dir> <fixture> — 0 if cellsvalidate ACCEPTS the fixture.
validates() {
  (cd "$1" && go run ./cmd/cellsvalidate "$2" >/dev/null 2>&1)
}

# case <label> <fixture> <sed-expr...> — assert the fixture is REJECTED by the
# real tree and ACCEPTED once the named rule is mutated away.
mutation_case() {
  local label="$1" fixture="$2"; shift 2
  cases=$((cases + 1))
  echo "== $label"
  echo "   fixture: $fixture"

  # Control: the fixture must be red in the unmutated tree. Without this the
  # mutation half proves nothing — a fixture that is green both before and
  # after tells you only that the file is valid.
  if validates "$repo" "testdata/$fixture"; then
    echo "   FAIL control: $fixture is ACCEPTED by the real validator — the fixture is not red at all" >&2
    fails=$((fails + 1))
    return
  fi
  echo "   ok   control: rejected by the real validator"

  local d; d=$(fresh_copy "$label")
  local expr
  for expr in "$@"; do
    if ! sed -i.bak "$expr" "$d/config/schema/cells.go"; then
      echo "   FAIL could not apply mutation: $expr" >&2
      fails=$((fails + 1))
      return
    fi
  done
  rm -f "$d/config/schema/cells.go.bak"

  if (cd "$d" && ! go build ./... >/dev/null 2>&1); then
    echo "   FAIL mutated tree does not compile — the mutation is malformed, not the rule" >&2
    fails=$((fails + 1))
    return
  fi

  if validates "$d" "testdata/$fixture"; then
    echo "   ok   mutation: rule removed -> fixture ACCEPTED (the rule is what rejects it)"
  else
    echo "   FAIL mutation: rule removed and the fixture is STILL rejected —" >&2
    echo "        this rule is not what pins the fixture; the check does not prove what it claims" >&2
    (cd "$d" && go run ./cmd/cellsvalidate "testdata/$fixture" 2>&1 | sed 's/^/        /') >&2
    fails=$((fails + 1))
  fi
}

# 1. tier enum closure. Widening the enum is the single most plausible future
#    edit — someone adds the tier they need and nothing else objects.
mutation_case tier-enum env-tier-prod.yaml \
  's|^var validEnvironmentTiers = map\[EnvironmentTier\]bool{EnvTierDev: true}$|var validEnvironmentTiers = map[EnvironmentTier]bool{EnvTierDev: true, EnvironmentTier("prod"): true}|'

# 2. the namespace dev-only pair. Both halves are neutered together because
#    `example-prod-app` is caught by either one; removing only one leaves the
#    other to reject it and the mutation would prove nothing.
mutation_case namespace-dev-only env-prod-namespace.yaml \
  's|^var devNamespaceRE = regexp.MustCompile(.*)$|var devNamespaceRE = regexp.MustCompile(`.*`)|' \
  's|^var forbiddenNamespaceRE = regexp.MustCompile(.*)$|var forbiddenNamespaceRE = regexp.MustCompile(`\\bmatch-nothing-sentinel\\b`)|'

# 3. access enum closure — a level the schema cannot name cannot be granted.
mutation_case access-enum env-access-cluster-admin.yaml \
  's|^\tAccessLogs:         true,$|\tAccessLogs: true,\n\tAccessLevel("cluster-admin"): true,|'

# 4. the empty-namespace fail-closed. The mutation makes the list optional,
#    which is exactly the "be permissive by default" shape the rule refuses.
mutation_case bare-context env-bare-context.yaml \
  's|^\tif len(e.Namespaces) == 0 {$|\tif false {|'

# 5. the canton exec gate (the operator namespace isolation policy, the authorization contract).
#    The mutation makes the canton regex match nothing — the shape this rule
#    would take if someone "simplified" it, or if a namespace naming change
#    quietly stopped matching. The fixture then validates, which is the whole
#    danger: exec into a Canton namespace across the namespace isolation boundary.
mutation_case canton-exec-gate env-exec-canton.yaml \
  's|^var cantonNamespaceRE = regexp.MustCompile(.*)$|var cantonNamespaceRE = regexp.MustCompile(`\\bmatch-nothing-sentinel\\b`)|'

echo
if [ "$fails" -eq 0 ]; then
  echo "FAIL-FIRST: all $cases rules proved load-bearing (fixture red with the rule, green without it)"
  exit 0
fi
echo "FAIL-FIRST: $fails of $cases rules are NOT load-bearing" >&2
exit 1
