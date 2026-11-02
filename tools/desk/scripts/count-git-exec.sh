#!/bin/sh
# count-git-exec.sh — the git-exec RATCHET gate for the desktools-go-git migration.
#
# Counts the git-binary spawn sites the stream is driving down:
#   1. direct spawns:  exec.Command("git" ...) and exec.CommandContext(<ctx>, "git" ...)
#      — the actual process launches (the context form counted since brief 06, whose
#      deleted preflight probe was one; the earlier pattern missed it), and
#   2. per-tool seam call sites: runGit( / gitOut( / execCommand( / execGit(
#      — the named helpers behind which tools route their git argv. The pattern is
#      textual: it also matches some execCommand( seams that launch gh, desktoken or
#      ssh rather than git (a known over-count, named as follow-on work, not hidden).
# Both counts EXCLUDE tools/desk/internal/gitexec (the audited fallback home — its
# spawn is the sanctioned one, and deskmerge's trial-merge fence routes through it)
# and *_test.go files (fixture/harness noise, not production call sites). grep -r
# prints paths relative to tools/desk (internal/gitexec/gitexec.go), so the
# exclusion is anchored at the start of the line: an unanchored '/internal/gitexec/'
# never matched and the sanctioned spawn used to be counted.
#
# RATCHET MODE (brief 08). The gate fails on GROWTH, not on a nonzero count. The
# migrations stopped short of zero, so the count is pinned: scripts/git-exec-pin.txt
# holds the exact count measured on the base commit this pin was set on.
#   count > pin  -> FAIL, exit 1 (a new git-exec site: route it through gitcore, or
#                   through internal/gitexec's audited allowlist, instead)
#   count = pin  -> exit 0
#   count < pin  -> exit 0 with a NOTE: lower the pin to the new count in the same
#                   change, so the space the removed sites leave cannot be refilled
# A missing or unreadable pin fails closed (exit 2), never reads as a pass.
#
#   --count   print the count line only and exit 0 (no pin read). Used to re-measure a
#             PR's base tree with this same script (internal/gitexec/ratchet_test.go),
#             so base and head are counted by identical patterns.
#
# Portable: POSIX sh, grep -E, no GNUisms (macOS + Linux).

set -u
cd "$(dirname "$0")/.." || exit 2

MODE=ratchet
case "${1:-}" in
  --count) MODE=count ;;
  "") ;;
  *) echo "count-git-exec.sh: unknown argument: $1 (want: --count or none)" >&2; exit 2 ;;
esac

DIRECT=$(grep -rEn -e 'exec\.Command\("git"' -e 'exec\.CommandContext\([^,]+, *"git"' cmd internal \
  --include='*.go' \
  | grep -v '^internal/gitexec/' \
  | grep -v '_test\.go:' \
  | wc -l | tr -d ' ')

SEAMS=$(grep -rEn '(runGit|gitOut|execCommand|execGit)\(' cmd internal \
  --include='*.go' \
  | grep -v '^internal/gitexec/' \
  | grep -v '_test\.go:' \
  | wc -l | tr -d ' ')

TOTAL=$((DIRECT + SEAMS))
echo "git-exec sites: $TOTAL (direct spawns: $DIRECT, seam call sites: $SEAMS)"

[ "$MODE" = count ] && exit 0

PIN_FILE=scripts/git-exec-pin.txt
if [ ! -r "$PIN_FILE" ]; then
  echo "FAIL: could-not-check: pin file $PIN_FILE is missing or unreadable" >&2
  exit 2
fi
# The pin is the one non-comment, non-blank line: a bare decimal count.
PIN=$(tr -d '\r' < "$PIN_FILE" | grep -v '^[[:space:]]*#' | grep -v '^[[:space:]]*$' | tr -d ' \t')
case "$PIN" in
  ''|*[!0-9]*)
    echo "FAIL: could-not-check: pin file $PIN_FILE must hold exactly one decimal count, got: $PIN" >&2
    exit 2 ;;
esac

if [ "$TOTAL" -gt "$PIN" ]; then
  echo "FAIL: git-exec sites grew: $TOTAL > pinned $PIN (+$((TOTAL - PIN))). Route new git work through internal/gitcore, or through internal/gitexec's audited allowlist." >&2
  exit 1
fi
if [ "$TOTAL" -lt "$PIN" ]; then
  echo "NOTE: git-exec sites fell below the pin: $TOTAL < $PIN. Lower $PIN_FILE to $TOTAL in this change."
fi
exit 0
