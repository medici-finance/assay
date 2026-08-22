#!/bin/sh
# count-git-exec.sh — advisory counting gate for the desktools-go-git migration.
#
# Counts the git-binary spawn sites the stream is driving to zero:
#   1. direct spawns:  exec.Command("git" ...)  — the actual process launches, and
#   2. per-tool seam call sites: runGit( / gitOut( / execCommand( / execGit(
#      — the named helpers behind which tools route their git argv.
# Both counts EXCLUDE tools/desk/internal/gitexec (the audited fallback home — its
# spawn is the sanctioned one) and *_test.go files (fixture/harness noise, not
# production call sites).
#
# Counting (advisory) mode: prints the baseline and exits 0. The gate flips to
# FAILING in brief 08, once migrations have driven the count down — until then a
# nonzero count is the recorded baseline, not a failure.
#
# Portable: POSIX sh, grep -E, no GNUisms (macOS + Linux).

set -u
cd "$(dirname "$0")/.." || exit 2

DIRECT=$(grep -rEn 'exec\.Command\("git"' cmd internal \
  --include='*.go' \
  | grep -v '/internal/gitexec/' \
  | grep -v '_test\.go:' \
  | wc -l | tr -d ' ')

SEAMS=$(grep -rEn '(runGit|gitOut|execCommand|execGit)\(' cmd internal \
  --include='*.go' \
  | grep -v '/internal/gitexec/' \
  | grep -v '_test\.go:' \
  | wc -l | tr -d ' ')

TOTAL=$((DIRECT + SEAMS))
echo "git-exec sites: $TOTAL (direct spawns: $DIRECT, seam call sites: $SEAMS)"
exit 0
