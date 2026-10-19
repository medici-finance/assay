#!/usr/bin/env bash
# regression: #3035
# Historical delivery proof only. No fetch, live transport or current ratchet.
set -euo pipefail
root=$(git rev-parse --show-toplevel)
parent=44054ca3756857c34eb818601a4ddcb19a7b4da5
delivered=c93cd7dd8fc99941a588feb6608184330fc08dd3
scratch=$(mktemp -d "${TMPDIR:-/tmp}/fetch-count-proof.XXXXXX")
trap 'rm -rf -- "$scratch"' EXIT
pattern='^git-exec sites: ([0-9]+) \(direct spawns: ([0-9]+), seam call sites: ([0-9]+)\)$'

count_tree() {
 local revision=$1 tree="$scratch/$1" line
 mkdir "$tree"
 # Only the tracked inputs the counter reads; never copy generated output.
 if ! git -C "$root" archive "$revision" tools/desk/cmd tools/desk/internal tools/desk/scripts/count-git-exec.sh | tar -x -C "$tree"; then
  printf 'could-not-check: cannot materialize counter inputs for %s\n' "$revision" >&2
  return 2
 fi
 line=$(bash "$tree/tools/desk/scripts/count-git-exec.sh")
 printf '%s: %s\n' "$revision" "$line" >&2
 if [[ ! $line =~ $pattern ]]; then
  printf 'could-not-check: unexpected counter output for %s\n' "$revision" >&2
  return 2
 fi
 printf '%s %s %s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
}

before=$(count_tree "$parent")
after=$(count_tree "$delivered")
read -r before_total before_direct before_seam <<< "$before"
read -r after_total after_direct after_seam <<< "$after"
# Pin the measured event, not a threshold on an ever-growing later main.
if [[ $before_total -ne 199 || $before_direct -ne 52 || $before_seam -ne 147 ||
      $after_total -ne 192 || $after_direct -ne 52 || $after_seam -ne 140 ||
      $((before_total - after_total)) -ne 7 || $((before_seam - after_seam)) -ne 7 ||
      $before_direct -ne "$after_direct" ]]; then
 printf 'FAIL: expected delivery 199 -> 192, seven seam sites removed and direct count unchanged; observed %s -> %s\n' "$before" "$after" >&2
 exit 1
fi
printf 'fetch delivery count: 199 -> 192; seven seam sites removed; direct spawns unchanged at 52\n'
