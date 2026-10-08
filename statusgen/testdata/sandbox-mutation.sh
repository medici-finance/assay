#!/bin/sh
# Mutation check for the --sandbox=container-netns mode (issue #2350).
# Three mutations, each applied alone and reverted: drop the loopback-only
# precheck, drop the witness sandbox field, drop the CI-lane refusal. Each must
# turn a TestSandbox* test red. Run only in an isolated owned worktree; the
# sources are restored on every exit.
set -eu
cd "$(dirname "$0")/.."
bk_run=$(mktemp "${TMPDIR:-/tmp}/sandbox-verifyrun.XXXXXX")
bk_sb=$(mktemp "${TMPDIR:-/tmp}/sandbox-verifysandbox.XXXXXX")
output=$(mktemp "${TMPDIR:-/tmp}/sandbox-result.XXXXXX")
cp verifyrun.go "$bk_run"
cp verifysandbox.go "$bk_sb"
restore() { cp "$bk_run" verifyrun.go; cp "$bk_sb" verifysandbox.go; }
trap 'restore; rm -f "$bk_run" "$bk_sb" "$output"' EXIT HUP INT TERM

# mutate <name> <file> <old> <new> <failing test>
mutate() {
  name=$1 file=$2 old=$3 new=$4 want=$5
  OLD=$old NEW=$new FILE=$file python3 - <<'PYCODE'
import os
from pathlib import Path
p = Path(os.environ['FILE'])
s = p.read_text()
old, new = os.environ['OLD'], os.environ['NEW']
assert s.count(old) == 1, 'mutation target moved; could-not-check'
p.write_text(s.replace(old, new))
PYCODE
  if go test -run '^TestSandbox' -timeout 120s -count=1 . > "$output" 2>&1; then
    echo "MUTATION FAIL ($name): the mutant passed"; exit 1
  fi
  grep -E -- "verifysandbox_test.go:[0-9]+:" "$output" || true
  grep -- "--- FAIL: $want" "$output"
  echo "MUTATION PASS ($name): rejected by $want"
  restore
}

mutate drop-precheck verifyrun.go \
  'if ok, why := loopbackOnlyPrecheck(listInterfacesFn); !ok {' \
  'if ok, why := true, ""; !ok {' \
  TestSandboxRunRefusedNoExec

mutate drop-witness-field verifyrun.go \
  'result += " sandbox=" + w.Sandbox' \
  '_ = w.Sandbox' \
  TestSandboxWitnessRecordsMode

mutate drop-ci-refusal verifysandbox.go \
  'if ciLane(ciFlag) {' \
  'if false && ciLane(ciFlag) {' \
  TestSandboxCILaneRefuses

echo 'ALL MUTATIONS REJECTED'
