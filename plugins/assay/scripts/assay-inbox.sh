#!/usr/bin/env bash
# assay-inbox.sh — the "inbox": a derived gh query, not a stored roll-up.
#
# Queries open issues across the configured repos for the escalation-contract
# labels, merges the results, sorts urgency-then-age, and prints a terminal table.
#
# Read-only: shells `gh issue list` only. Never writes to any issue. Terminal-agnostic — works
# with plain git + gh + any terminal.
#
# Repo resolution order:
#   1. repo args on the command line (owner/name ...)
#   2. ./.assay/repos.txt (one owner/name per line; blank lines and #-comments ignored)
#   3. the current repo's `origin` remote, parsed to owner/name
#
# Exit codes (a queue tool that cannot distinguish "nothing waiting" from "the query
# died" is worse than no tool at all — a review found this early on):
#   0  every query succeeded (the printed table is complete)
#   1  precondition failure — gh/jq missing, or no repos resolvable
#   2  at least one gh query FAILED; the table, if any, is PARTIAL
#
# PORTABILITY: bash 3.2 (stock /bin/bash on macOS) is a supported target — the brief's
# premise is "plain git + gh + any terminal". No bash-4-only builtins (mapfile/readarray,
# associative arrays, ${var^^}). assay-inbox.test.sh pins this.
set -euo pipefail

# The four escalation label tokens this inbox folds into one view.
#
# PROVENANCE, precisely (an earlier review left this unchecked; diffed since):
# the escalation contract creates and owns exactly ONE of these — `needs-decision`, whose
# spelling matches it verbatim. The other three are stock GitHub labels folded into the same
# inbox; the contract does not define them, so do not "re-sync" them against it and find them
# missing.
#
# These are the interface seam — keep `needs-decision` in exact sync with the escalation
# contract; do not add/reorder the set without re-reading that contract.
LABELS=("urgent" "needs-decision" "question" "help wanted")

# `gh issue list` defaults to --limit 30 and keeps the THIRTY NEWEST, discarding everything
# older. This tool's whole ordering is oldest-first-within-rank, so the default cap discards
# precisely the items it exists to surface (caught in review). Query well
# above any realistic per-label ceiling, and shout if we ever hit it.
LIMIT="${ASSAY_INBOX_LIMIT:-500}"

usage() {
  cat <<'EOF'
Usage: assay-inbox.sh [owner/repo ...]

Prints open issues across the given repos (or ./.assay/repos.txt, or the current repo's
origin remote) carrying any of the escalation-contract labels: urgent, needs-decision,
question, help wanted. Sorted urgency-then-age (most urgent, then oldest, first).

Read-only — this never writes to any issue.

Environment:
  ASSAY_INBOX_LIMIT   per-repo-per-label fetch cap passed to `gh issue list` (default 500).

Exit codes:
  0  all queries succeeded          1  gh/jq missing or no repos resolvable
  2  one or more queries FAILED — the table is partial; see stderr
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

resolve_repos() {
  if [[ $# -gt 0 ]]; then
    printf '%s\n' "$@"
    return
  fi

  if [[ -f "./.assay/repos.txt" ]]; then
    grep -vE '^[[:space:]]*(#|$)' "./.assay/repos.txt"
    return
  fi

  local origin_url owner_name
  origin_url=$(git remote get-url origin 2>/dev/null || true)
  if [[ -z "$origin_url" ]]; then
    # Runs in a process-substitution subshell — `exit` here only ends the subshell, so just
    # emit nothing; the caller detects an empty REPOS array and reports + exits.
    return
  fi
  owner_name=$(printf '%s' "$origin_url" \
    | sed -E 's#^git@github\.com:##; s#^https://github\.com/##; s#\.git$##')
  printf '%s\n' "$owner_name"
}

command -v gh >/dev/null 2>&1 || { echo "assay-inbox: gh CLI not found" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "assay-inbox: jq not found" >&2; exit 1; }

# bash 3.2 has no `mapfile`; a `while read` fed by process substitution (NOT a pipe) keeps
# the appends in the current shell, so REPOS survives the loop.
REPOS=()
while IFS= read -r _line; do
  [[ -z "${_line//[[:space:]]/}" ]] && continue
  REPOS+=("$_line")
done < <(resolve_repos "$@")

# `${#REPOS[@]}` on an empty array trips `set -u` on bash < 4.4; the +x guard is the
# portable form.
if [[ -z "${REPOS[*]+x}" || ${#REPOS[@]} -eq 0 ]]; then
  echo "assay-inbox: no repos to query (no repo args, no ./.assay/repos.txt, and no git origin remote found)" >&2
  exit 1
fi

# Every repo token is validated before any gh call (a review finding): a metacharacter-laden token must never reach a shell, and garbage from a
# non-GitHub origin remote is clearer rejected here than as a gh failure downstream.
# The charset is GitHub's own owner/name alphabet.
for _repo in "${REPOS[@]}"; do
  if [[ ! "$_repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]]; then
    echo "assay-inbox: invalid repo '$_repo' (expected owner/name)" >&2
    exit 1
  fi
done

TMP_JSON=$(mktemp)
TMP_ERR=$(mktemp)
TMP_TSV=$(mktemp)
trap 'rm -f "$TMP_JSON" "$TMP_ERR" "$TMP_TSV"' EXIT
echo "[]" > "$TMP_JSON"

query_failures=0
truncated=0

for repo in "${REPOS[@]}"; do
  [[ -z "$repo" ]] && continue
  for label in "${LABELS[@]}"; do
    # `if hits=$(...)` puts the command in a condition context, so `set -e` does not fire and
    # we get to inspect the status ourselves. gh's diagnostics are CAPTURED, not discarded —
    # they are the only thing that tells an operator their token expired.
    if hits=$(gh issue list --repo "$repo" --state open --label "$label" \
        --limit "$LIMIT" --json number,title,labels,createdAt,url 2>"$TMP_ERR"); then
      :
    else
      gh_status=$?
      query_failures=$((query_failures + 1))
      printf 'assay-inbox: QUERY FAILED for %s [%s] (gh exit %s): %s\n' \
        "$repo" "$label" "$gh_status" "$(tr '\n' ' ' <"$TMP_ERR")" >&2
      continue
    fi
    [[ -z "$hits" ]] && hits="[]"

    n=$(printf '%s' "$hits" | jq 'length')
    if [[ "$n" -ge "$LIMIT" ]]; then
      truncated=$((truncated + 1))
      printf 'assay-inbox: WARNING %s [%s] returned %s items == --limit %s; results are TRUNCATED (oldest dropped). Raise ASSAY_INBOX_LIMIT.\n' \
        "$repo" "$label" "$n" "$LIMIT" >&2
    fi

    merged=$(jq -s --arg repo "$repo" \
      '.[0] + (.[1] | map(. + {repo: $repo}))' "$TMP_JSON" <(printf '%s' "$hits"))
    printf '%s' "$merged" > "$TMP_JSON"
  done
done

# Dedupe by repo+number (an issue can carry more than one contract label), rank by the most
# urgent label it carries (lower rank = more urgent), sort urgency-then-age (oldest createdAt
# first within the same rank — waiting longest surfaces first), print a table. urgent-labeled
# items (rank 0) are marked so they're visually first even at a glance.
#
# Display hygiene (a review finding): issue titles and labels are
# attacker-supplied for any repo where a stranger can open an issue, and `@tsv` escapes only
# tab/LF/CR/backslash — so a raw ESC (0x1b) in a title would otherwise reach the operator's
# terminal intact. clean() strips every C0/DEL byte (gsub \p{Cc}); rank is still computed on
# the un-cleaned label names, so cleaning is display-only. Titles are also cut to the
# 60-column field — %-60s pads but never truncates — so an over-long title cannot break the
# layout.
jq -r --argjson rankorder '["urgent","needs-decision","question","help wanted"]' '
  def clean: gsub("\\p{Cc}";"");
  unique_by(.repo + "#" + (.number | tostring))
  | map(. + {
      rank: ([ .labels[].name as $n | ($rankorder | index($n)) ] | map(select(. != null)) | min // 99)
    })
  | sort_by([.rank, .createdAt])
  | .[]
  | [.rank, ([ .labels[].name | clean ] | join(",")), .repo, ("#" + (.number|tostring)),
     (.title | clean | if length > 57 then .[:57] + "..." else . end), .createdAt, .url]
  | @tsv
' "$TMP_JSON" > "$TMP_TSV"

# Read from a FILE, not a pipe: a `while read` on the right of `|` runs in a subshell, so
# any count accumulated inside it would be lost.
item_count=0
while IFS=$'\t' read -r rank labelnames repo number title createdat url; do
  item_count=$((item_count + 1))
  now_epoch=$(date -u +%s)
  created_epoch=$(date -u -d "$createdat" +%s 2>/dev/null || date -u -j -f "%Y-%m-%dT%H:%M:%SZ" "$createdat" +%s 2>/dev/null || echo "")
  if [[ -n "$created_epoch" ]]; then
    age_days=$(( (now_epoch - created_epoch) / 86400 ))
    age="${age_days}d"
  else
    age="$createdat"
  fi
  marker="  "
  [[ "$rank" == "0" ]] && marker="**"
  printf '%s %-14s %-45s %-8s %-60s %-5s %s\n' \
    "$marker" "$labelnames" "$repo" "$number" "$title" "$age" "$url"
done < "$TMP_TSV"

# A TERMINATING SUMMARY is what makes "0 items" positively distinguishable from "the run
# died before printing anything" — without it the two are byte-identical (finding 1).
summary="assay-inbox: ${item_count} item(s) across ${#REPOS[@]} repo(s)"
[[ "$truncated" -gt 0 ]] && summary="${summary}; ${truncated} query(s) TRUNCATED at --limit ${LIMIT}"
if [[ "$query_failures" -gt 0 ]]; then
  echo "${summary}; ${query_failures} query(s) FAILED — THIS INBOX IS INCOMPLETE (see stderr)"
  exit 2
fi
echo "$summary"
