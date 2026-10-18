#!/usr/bin/env bash
# Re-run the stored-artifact reducer; never write its report into the source tree.
set -euo pipefail
export KUBECONFIG=/dev/null
root=$(git rev-parse --show-toplevel)
arms=${1:-"$root/docs/streams/desk-supervision/08-arms"}
report=$(mktemp "${TMPDIR:-/tmp}/worker-kit-report.XXXXXX")
cd "$root/tools/skillbench"
GOWORK=off GOPROXY=off go run . --arms "$arms" --out "$report" --date 2026-09-17
grep -Fq '### Task-check pass rate' "$report"
if [ "$(grep -Fc '100% (n=15/15)' "$report")" -ne 2 ]; then
    echo 'task-check safety floor differs from the stored 15/15 in both arms' >&2
    exit 1
fi
grep -F 'Safety floor (task-check pass rate): held' "$report"
