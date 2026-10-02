#!/bin/sh
# Run only in an isolated owned worktree; restore the detector on every exit.
set -eu
cd "$(dirname "$0")/.."
backup=$(mktemp "${TMPDIR:-/tmp}/portability-source.XXXXXX")
output=$(mktemp "${TMPDIR:-/tmp}/portability-result.XXXXXX")
cp verifyportability.go "$backup"
trap 'cp "$backup" verifyportability.go; rm -f "$backup" "$output"' EXIT HUP INT TERM
python3 - <<'PYCODE'
from pathlib import Path
p = Path('verifyportability.go')
s = p.read_text()
old = 'return verifyTmpPath.MatchString(command) || verifyShellC.MatchString(command) || verifyFindstr.MatchString(command)'
assert s.count(old) == 1, 'mutation target moved; could-not-check'
p.write_text(s.replace(old, 'return false && (' + old[7:] + ')'))
PYCODE
if go test -run '^TestVerifyRowPortability$' -timeout 45s -count=1 . > "$output" 2>&1; then
 echo 'MUTATION FAIL: disabled detector passed'; exit 1
fi
cat "$output"
grep -q -- '--- FAIL: TestVerifyRowPortability' "$output"
grep -qF 'notice routing = []' "$output"
echo 'MUTATION PASS: disabled detector was rejected'
