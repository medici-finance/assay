#!/usr/bin/env bash
# early-reader-lint.test.sh — no early-exit reader may sit behind a pipe in a plugin shell script
# that runs under `pipefail`.
#
# DEFECT CLASS (#2988, first filed as #2127). A pipeline `<producer> | <reader>` where the READER
# stops reading before the producer has finished writing: `grep -q`, `grep -m N`, `head`, or
# `sed ...q`. The producer's next write then hits a closed pipe and dies of SIGPIPE; under
# `set -o pipefail` that 141 becomes the pipeline's status, so a line that DID match is reported
# as a failure. It fires only when the producer's output is larger than one pipe write, so the
# same code passes locally and flakes on a loaded runner — assay-inbox.test.sh's
# `printf '%s' "$OUT" | grep -Eq …` failed that way.
#
# THE REMEDY (the one choke point) is to never put the early reader behind a pipe:
#   - a variable        grep -Eq PATTERN <<<"$var"        (a here-string is written before the
#                                                          reader starts; there is no producer)
#   - a command         out=$(cmd); grep -q PATTERN <<<"$out"
#   - the first line    ... | sed -n 1p                   (reads to EOF, unlike `head -1`)
#   - a count           ... | grep -c PATTERN             (reads to EOF)
#
# WHAT THIS GUARD DOES. It scans plugins/assay/scripts/*.sh and plugins/assay/hooks/*.sh — the
# files this repo's `plugin-shell-suites` CI job runs and ships — for the shapes above in every
# script that sets `pipefail`, and fails on any hit. The matcher could silently stop matching, so
# it carries a committed POSITIVE CONTROL: a fixture holding one planted instance of each shape
# that the matcher must flag, and a NEGATIVE CONTROL of the safe forms it must not.
#
# SCOPE. Only the plugin script directories are scanned. Every site in them was fixed with this
# change; sites elsewhere in the tree (tools/, the workflow files) are tracked separately and are
# deliberately not scanned here, so this suite is green on a tree that still has them.
#
# No network and no token. Run:  bash plugins/assay/scripts/early-reader-lint.test.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

# The three early-reader shapes, as one ERE over a single source line. A reader is the stage
# AFTER a single `|` (never `||`); the pattern requires the pipe, so a here-string or a file operand
# (`grep -q X file`) is not a hit.
#   grep -q / -Eq / -qx / --quiet / -m N / --max-count      (also egrep, fgrep, rg)
#   head                                                    (head -1, head -n 3, head -c 200)
#   sed '...q'                                              (the quit command ends the read)
RE_GREP='(^|[^|])[|][[:space:]]*(e|f)?grep[[:space:]]([^|;&)]*[[:space:]])?(-[A-Za-z]*[qm][A-Za-z]*|--quiet|--max-count)([[:space:]=0-9]|$)'
RE_RG='(^|[^|])[|][[:space:]]*rg[[:space:]]([^|;&)]*[[:space:]])?(-[A-Za-z]*q[A-Za-z]*|--quiet|-m[[:space:]]*[0-9]|--max-count)([[:space:]=]|$)'
RE_HEAD='(^|[^|])[|][[:space:]]*head([[:space:]]|$)'
RE_SEDQ="(^|[^|])[|][[:space:]]*sed[[:space:]]([^|;&)]*[[:space:]])?'[^']*[0-9\$/]q[;']"

# sets_pipefail <file> — 0 if the script turns pipefail on (a `set` line naming it, or -o pipefail).
sets_pipefail() {
  grep -Eq '^[[:space:]]*set[[:space:]].*pipefail' "$1"
}

# scan <file> — print `<file>:<line>: <source>` for every early reader behind a pipe, in a script
# that sets pipefail. Comment lines are skipped. Reads whole files; no early-exit reader of its
# own sits behind a pipe.
scan() {
  local f="$1" re hit
  sets_pipefail "$f" || return 0
  for re in "$RE_GREP" "$RE_RG" "$RE_HEAD" "$RE_SEDQ"; do
    hit=$(grep -nE -- "$re" "$f" | grep -vE '^[0-9]+:[[:space:]]*#')
    [[ -n "$hit" ]] && while IFS= read -r l; do printf '%s:%s\n' "$f" "$l"; done <<<"$hit"
  done | sort -t: -k2,2n -u
}

TMPROOT=$(mktemp -d)
trap 'rm -rf "$TMPROOT"' EXIT

# ---------------------------------------------------------------- ELR1 ------
echo "ELR1 — the matcher flags each early-reader shape (positive control)"

# The planted instances are assembled from fragments so THIS file does not itself contain them
# and stays clean under its own scan.
G='gre''p'
H='hea''d'
S='se''d'
plant="$TMPROOT/planted.sh"
{
  printf '%s\n' 'set -uo pipefail'
  printf '%s\n' "printf '%s' \"\$OUT\" | $G -Eq 'x'"
  printf '%s\n' "git log | $G -qF needle"
  printf '%s\n' "cmd | $G -m1 needle"
  printf '%s\n' "cmd | $H -1"
  printf '%s\n' "cmd | $S -n '1q'"
  printf '%s\n' "cmd | $G --quiet needle"
} > "$plant"
planted_n=$(scan "$plant" | wc -l | tr -d ' ')
if [[ "$planted_n" -eq 6 ]]; then
  ok "all six planted early readers are flagged"
else
  no "all six planted early readers are flagged" "flagged $planted_n of 6: $(scan "$plant" | tr '\n' ';')"
fi

# ---------------------------------------------------------------- ELR2 ------
echo "ELR2 — the matcher leaves the safe forms alone (negative control)"

safe="$TMPROOT/safe.sh"
{
  printf '%s\n' 'set -euo pipefail'
  printf '%s\n' "$G -Eq 'x' <<<\"\$OUT\""
  printf '%s\n' "$G -q needle somefile"
  printf '%s\n' "cmd | $S -n 1p"
  printf '%s\n' "cmd | $G -c needle"
  printf '%s\n' "cmd | $G -v needle"
  printf '%s\n' "out=\$(cmd); $G -q needle <<<\"\$out\""
  printf '%s\n' "# cmd | $G -q needle   (a comment)"
  printf '%s\n' "a || $G -q needle somefile"
} > "$safe"
safe_n=$(scan "$safe" | wc -l | tr -d ' ')
if [[ "$safe_n" -eq 0 ]]; then
  ok "no safe form is flagged"
else
  no "no safe form is flagged" "flagged: $(scan "$safe" | tr '\n' ';')"
fi

nopipe="$TMPROOT/nopipefail.sh"
printf '%s\n' 'set -eu' "cmd | $G -q needle" > "$nopipe"
if [[ -z "$(scan "$nopipe")" ]]; then
  ok "a script that never sets pipefail is out of scope"
else
  no "a script that never sets pipefail is out of scope" "it was flagged"
fi

# ---------------------------------------------------------------- ELR3 ------
echo "ELR3 — the live plugin scripts carry no early reader behind a pipe"

shopt -s nullglob
files=("$HERE"/*.sh "$HERE"/../hooks/*.sh)
shopt -u nullglob

# Three-state: a scan that read nothing has not cleared anything.
if [[ "${#files[@]}" -lt 2 ]]; then
  no "the scan read the plugin scripts" "could-not-check: found ${#files[@]} script(s) under $HERE"
else
  ok "the scan read ${#files[@]} plugin scripts"
fi

live=""
for f in "${files[@]}"; do
  one=$(scan "$f")
  [[ -n "$one" ]] && live+="$one"$'\n'
done
live="${live%$'\n'}"
if [[ -z "$live" ]]; then
  ok "no early-exit reader sits behind a pipe under pipefail"
else
  no "no early-exit reader sits behind a pipe under pipefail" \
    "fix each by feeding the reader a here-string or reading to EOF (see the header):"$'\n'"$live"
fi

echo
echo "$pass passed, $fail failed"
[[ "$fail" -eq 0 ]]
