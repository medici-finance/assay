#!/usr/bin/env bash
# Brief-06 Verify row 5: has the stamp-allowance backlog cleared?
#
# Reads the write set from ASSAY_ALLOWED_REPOS (the roster grammar: entries split
# on commas, semicolons or whitespace; `owner/name[:policy...]`, or `owner/*`,
# which admits every repository under owner and is enumerated from the forge).
# For each repository, prints `PR <owner>/<repo>#<n> <label>` for every open PR
# whose `dispatched-*` label still stands under a non-App account, then
# `examined <N> PRs in <owner>/<repo>`.
#
# Exit 0: no PR line anywhere, so the backlog has cleared.
# Exit 1 with PR lines: the backlog remains (set after every repository is examined).
# Exit 2: could-not-check (the set is unset, empty or malformed).
# Any other non-zero exit with no PR line is a forge or tool failure: could-not-check.
set -euo pipefail
set -f # an owner/* entry is a roster pattern, never a filename glob
export KUBECONFIG=/dev/null

cnc() { echo "could-not-check: $*" >&2; exit 2; }

[ -n "${ASSAY_ALLOWED_REPOS:-}" ] || cnc "ASSAY_ALLOWED_REPOS is unset or empty"

all=""
for entry in $(printf '%s' "$ASSAY_ALLOWED_REPOS" | tr ',;\t\r\n' '     '); do
    slug=${entry%%:*}
    case "$slug" in
    */*/* | /* | */ | *[!A-Za-z0-9._/*-]*) cnc "ASSAY_ALLOWED_REPOS entry $entry is not owner/name or owner/*" ;;
    ?*/\*)
        owner=${slug%/*}
        case "$owner" in *\**) cnc "ASSAY_ALLOWED_REPOS entry $entry has a '*' in its owner" ;; esac
        listed=$(gh repo list "$owner" --limit 1000 --json nameWithOwner | jq -r '.[].nameWithOwner')
        all="$all$listed"$'\n'
        ;;
    ?*/*\**) cnc "ASSAY_ALLOWED_REPOS entry $entry is not owner/name or owner/*" ;;
    ?*/?*) all="$all$slug"$'\n' ;;
    *) cnc "ASSAY_ALLOWED_REPOS entry $entry is not owner/name or owner/*" ;;
    esac
done
repos=$(printf '%s' "$all" | sed '/^$/d' | sort -u)
[ -n "$repos" ] || cnc "ASSAY_ALLOWED_REPOS names no repository"

# The floor matches stamp labels case-insensitively (deskkit normLabel), so this
# check lower-cases every label name before it compares or keys on it.
found=0
for R in $repos; do
    prs=$(gh pr list -R "$R" --state open --limit 1000 --json number,labels |
        jq -r '.[] | select(any(.labels[]; .name | ascii_downcase | startswith("dispatched-"))) | .number')
    n_prs=0
    for n in $prs; do
        n_prs=$((n_prs + 1))
        hits=$(gh api --paginate --slurp "repos/$R/issues/$n/events" |
            jq -r --arg r "$R" --arg n "$n" '
                add
                | map(select(.event == "labeled" or .event == "unlabeled"))
                | map(.label.name |= ascii_downcase)
                | map(select(.label.name | startswith("dispatched-")))
                | reduce .[] as $e ({};
                    if $e.event == "labeled" then .[$e.label.name] = $e.actor.type
                    else del(.[$e.label.name]) end)
                | to_entries[]
                | select(.value != "Bot")
                | "PR \($r)#\($n) \(.key)"')
        if [ -n "$hits" ]; then
            printf '%s\n' "$hits"
            found=1
        fi
    done
    echo "examined $n_prs PRs in $R"
done
[ "$found" -eq 0 ] || exit 1
