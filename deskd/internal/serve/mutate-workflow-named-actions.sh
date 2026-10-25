#!/usr/bin/env bash
# mutate-workflow-named-actions.sh — fail-first evidence for
# TestNamedActionsRejectWorkflowControl (the service contract Verify row 102, the
# task-workflow amendment: a workflow work/node/attempt id is not a terminal
# target, and workflow start/resume/handback or a human-decision authorization
# cannot be routed through `send` or `session`).
#
# WHY MUTATION AND NOT ONLY A PRE-FIX RED RUN. Most of the controls the test
# pins already hold at the commit that adds it (the closed allow-list, the
# strict body, the registry's desk lookup), so a pre-fix run shows only the one
# case that was genuinely open (a query string rode a send unread). The
# evidence that the test pins each control is that it goes red when that ONE
# control is removed or widened the way a plausible future edit would — at
# each layer: the contract allow-list, the route table, the handler's path
# match, the strict body, the closed query, and the registry under it all.
#
# It never edits the working tree: the module is copied to a tempdir, the
# mutation is applied to the copy, and the copy is thrown away. Each case also
# checks that its mutation applied — a substitution that matched nothing would
# leave the tree unmutated and "prove" nothing. Offline: no network, no docker.
#
# Usage:  internal/serve/mutate-workflow-named-actions.sh
# Exit:   0 every control proved load-bearing
#         1 a control is not load-bearing (or a mutation failed to apply)
#         2 usage / environment error
set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
repo=$(cd "$here/../.." && pwd)

command -v go >/dev/null 2>&1 || { echo "go is not on PATH" >&2; exit 2; }
[ -f "$repo/go.mod" ] || { echo "no go.mod at $repo" >&2; exit 2; }

tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutate-wf-actions.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT

fails=0
cases=0
test_re='^TestNamedActionsRejectWorkflowControl$'

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

# expect_red <label> <dir> <subtest> — the named subtest must FAIL.
expect_red() {
  local label=$1 dir=$2 sub=$3
  cases=$((cases + 1))
  local out
  out=$(cd "$dir" && GOWORK=off go test -count=1 -timeout 180s -v -run "$test_re" ./internal/serve/ 2>&1)
  if [ $? -eq 0 ]; then
    fails=$((fails + 1))
    printf 'NOT LOAD-BEARING: %s\n  the test still PASSES with the control removed\n' "$label"
  elif ! printf '%s\n' "$out" | grep -q -- "--- FAIL: TestNamedActionsRejectWorkflowControl/$sub "; then
    # A build error, or a red in some other subtest, is not this control's red.
    fails=$((fails + 1))
    printf 'NOT THE EXPECTED RED: %s (wanted subtest %s)\n' "$label" "$sub"
    printf '%s\n' "$out" | grep -E -- '--- FAIL|^#|cannot|undefined' | head -4 | sed 's/^/    /'
  else
    printf 'red as expected: %s\n' "$label"
    printf '%s\n' "$out" | grep -E '^\s+workflow_named_actions_test\.go:[0-9]+' | head -2 | sed 's/^/    /'
  fi
}

contract_go=internal/contract/contract.go
actions_go=internal/serve/v1_actions.go
term_go=internal/serve/v1_terminal_actions.go
registry_go=internal/terminal/registry.go

echo "== allow-list layer: the closed named-action set and the route table =="

d=$(fresh_copy fifth-action)
# Widen the contract's closed set with a workflow verb.
perl -pi -e 's/^var NamedActions = \[\]Action\{ActionAnswer, ActionAck, ActionSend, ActionSession\}$/var NamedActions = []Action{ActionAnswer, ActionAck, ActionSend, ActionSession, Action("resume")}/' "$d/$contract_go"
applied "fifth named action" "$repo/$contract_go" "$d/$contract_go" &&
  expect_red "a workflow verb joins NamedActions" "$d" 'the_allow-list_stays_the_four_and_no_write_route_is_workflow-shaped'

d=$(fresh_copy approve-alias)
# Mount an "approve" decision alias beside ack.
perl -pi -e 's/^(\t\t\tmux\.HandleFunc\("POST "\+r\.Path, actions\.handle\(contract\.ActionAck\)\))$/$1\n\t\t\tmux.HandleFunc("POST "+contract.Prefix+"\/escalations\/{id}\/approve", actions.handle(contract.ActionAck))/' "$d/$actions_go"
applied "approve alias" "$repo/$actions_go" "$d/$actions_go" &&
  expect_red "a decision authorization is reachable as /escalations/{id}/approve" "$d" 'workflow_lifecycle_and_decision_control_routes_do_not_exist'

d=$(fresh_copy any-terminal-verb)
# Widen the send handler's path match: any /terminal/<verb> becomes a send.
perl -pi -e 's/^\t\tid, ok := strings\.CutSuffix\(strings\.TrimPrefix\(r\.URL\.Path, prefix\), suffix\)$/\t\t_ = suffix\n\t\tid, _, ok := strings.Cut(strings.TrimPrefix(r.URL.Path, prefix), "\/terminal\/")/' "$d/$term_go"
applied "any /terminal/<verb>" "$repo/$term_go" "$d/$term_go" &&
  expect_red "/terminal/start, /resume, /handback route to send" "$d" 'workflow_lifecycle_and_decision_control_routes_do_not_exist'

echo
echo "== per-action scope: the send/session handlers =="

d=$(fresh_copy normalize-id)
# Treat a workflow id under a desk as that desk (work/node/attempt suffix trimmed).
perl -pi -e 's/^(\t\tcell := cellOfDesk\(id\))$/\t\tif p := strings.SplitN(id, "\/", 3); len(p) == 3 {\n\t\t\tid = p[0] + "\/" + p[1]\n\t\t}\n$1/' "$d/$term_go"
applied "workflow id normalized to its desk" "$repo/$term_go" "$d/$term_go" &&
  expect_red "assay/dispatch/attempt/7f3c9a reaches the assay/dispatch terminal" "$d" 'a_workflow_id_is_not_a_terminal_target'

d=$(fresh_copy lax-body)
# Drop the strict decoder's unknown-field refusal on the terminal writes.
perl -pi -e 's/^\tdec\.DisallowUnknownFields\(\)$/\t\/\/ unknown fields tolerated/' "$d/$term_go"
applied "unknown body fields tolerated" "$repo/$term_go" "$d/$term_go" &&
  expect_red "work_id / attempt_id / decision ride a send or open" "$d" 'control_payloads_in_the_body_are_refused'

d=$(fresh_copy open-query)
# Remove the closed-query refusal (the pre-fix behaviour: a query was ignored).
perl -0pi -e 's/(func refuseQuery\(w http\.ResponseWriter, r \*http\.Request\) bool \{\n)/$1\tif true {\n\t\treturn false\n\t}\n/' "$d/$term_go"
applied "query string ignored" "$repo/$term_go" "$d/$term_go" &&
  expect_red "?action=resume&attempt=… rides a send, open or close" "$d" 'control_payloads_in_the_query_are_refused'

echo
echo "== lower layer: the registry, with every HTTP layer bypassed =="

d=$(fresh_copy registry-normalize)
# The registry resolves a workflow id to the desk it names.
perl -0pi -e 's/(func \(r \*Registry\) handle\(id string\) \*Handle \{\n\tr\.mu\.Lock\(\)\n\tdefer r\.mu\.Unlock\(\)\n)/$1\tif p := strings.SplitN(id, "\/", 3); len(p) == 3 \&\& r.handles[id] == nil {\n\t\tid = p[0] + "\/" + p[1]\n\t}\n/' "$d/$registry_go"
applied "registry resolves a workflow id to a desk" "$repo/$registry_go" "$d/$registry_go" &&
  expect_red "the registry sends to / opens on assay/dispatch/attempt/7f3c9a" "$d" 'the_registry_refuses_a_workflow_id_with_the_HTTP_layers_bypassed'

echo
if [ "$fails" -ne 0 ]; then
  echo "FAIL: $fails of $cases control(s) are not load-bearing"
  exit 1
fi
echo "OK: all $cases controls proved load-bearing (each went red with its control removed)"
