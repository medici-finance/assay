#!/bin/sh
# Row 3 covers three landed changes. Each source addition belongs to the
# first-parent diff that introduced it. The native report covers #3593.
set -eu
export GIT_NO_REPLACE_OBJECTS=1

extraction=8c2f9706c9d5b26b14ad3a8123878563896e27fd
extraction_base=85fcd48feaf08c69a0c878939702b30f2b608546
observation=790003516c1fb4ea99d0a6d5eb6e67e9123b3c2a
observation_base=4b7f4847f7f3531185fdfb23fbf978ae9551a242
selection=f9cba0a88d39085831c86717aab04736c562a774
selection_base=1e7e43ad33d52e68f2234e979235ef8d911bff10

range() {
    delivery=$1
    base=$2
    parent=$(git rev-parse --verify "$delivery^1")
    if [ "$parent" != "$base" ]; then
        echo "Wrong first parent for $delivery: expected $base, found $parent" >&2
        exit 1
    fi
    git merge-base --is-ancestor "$delivery" HEAD
}

addition() {
    path=$1
    needle=$2
    patch=$(git diff --no-ext-diff --no-textconv --no-renames --color=never -U0 "$base" "$delivery" -- "$path")
    if ! printf '%s\n' "$patch" | awk -v needle="$needle" '
        /^\+/ && !/^\+\+\+/ && index(substr($0, 2), needle) { found = 1 }
        END { exit !found }
    '; then
        echo "Missing addition in $base..$delivery at $path: $needle" >&2
        exit 1
    fi
}

# #3570 declared three self-routes. Test the source edges without crediting
# that report or treating deskdispatch as an implemented adapter.
range "$extraction" "$extraction_base"
addition tools/desk/cmd/deskboard/board.go 'return reviewflow.Classify(in.flow(), reviewerBotDisplay())'
addition tools/desk/cmd/reviewloop/flow.go 'var Coalesce = reviewflow.Coalesce'

# #3593 routed two real adapters and two follow-ups. The report's exit code
# is checked by the caller, before this helper sees its stdout.
range "$observation" "$observation_base"
printf '%s\n' "$1" | grep -Fx "consumers corroboration — base $base..$delivery (immutable delivered snapshot), 1 brief(s)"
printf '%s\n' "$1" | grep -Fx 'summary: 4 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing'
for route in 'tools/desk/cmd/deskboard/: fixed-here' 'tools/desk/cmd/reviewloop/: fixed-here' \
             'tools/desk/internal/deskclient/: follow-up graph-execution/27' \
             'tools/desk/cmd/deskdispatch/: follow-up desk-tools/41'; do
    printf '%s\n' "$1" | grep -F "  CORROBORATED  $route"
done
addition tools/desk/cmd/deskboard/board.go 'in := (reviewflow.ClassificationFacts{'
addition tools/desk/cmd/reviewloop/plan.go 'planned := reviewflow.PlanBoard(board, now)'

# #4235 added four selection routes. The previous adapters are inherited;
# their proof above must not be attributed to this later change.
range "$selection" "$selection_base"
addition tools/desk/internal/reviewflow/selection.go 'func PlanCandidates('
addition tools/desk/internal/reviewtest/selection_test.go 'rf.PlanCandidates('
addition spec/review-observation-v1.md 'PlanCandidates(observation, freshness, selectionPolicy, capacity)'
addition docs/contracts.md 'PlanCandidates'

echo 'Deferred implementation and adoption remain for review: graph-execution/27 and desk-tools/40, /41, /43.'
