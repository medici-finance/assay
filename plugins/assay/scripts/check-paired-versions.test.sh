#!/usr/bin/env bash
# check-paired-versions.test.sh — the proof that check-paired-versions.sh actually catches the
# drift it exists for, and that the tree as committed is clean.
#
#   LIVE  the repo as committed passes. This is the assertion that arms the guard in CI: this
#         suite is globbed by ci.yml's `plugin-shell-suites` job, so a future re-pin that leaves
#         the two files disagreeing reddens the PR instead of shipping a stale tool to adopters.
#   A     PAIRING drift — the exact pre-fix state: a manifest sitting on an older plugin version
#         than plugin.json. Passed silently before the guard; must fail now.
#   B     TAG drift — three shapes: one platform line left behind on an older tag, the
#         desk-tools section on a different tag from statusgen, and a wholesale two-tag file.
#   C     HASH shape — truncated, upper-cased, stray-placeholder digests FAIL; the one
#         reserved <harvest-after-release> placeholder (a tag not yet cut) PASSES.
#   D     COULD-NOT-CHECK — missing file, missing `plugin:` key, missing `version` field, and a
#         file with no pin lines at all. Each must FAIL, never quietly pass.
#
# Fixtures are built by copying the real two files into a temp root and mutating the copy, so a
# reshaping of the real manifest cannot leave the suite testing a stale shape. Hermetic: no
# network, no token, no jq. Run:  bash plugins/assay/scripts/check-paired-versions.test.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/check-paired-versions.sh"
REPO=$(cd "$HERE/../../.." && pwd)

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

TMP=$(mktemp -d "${TMPDIR:-/tmp}/check-paired-versions.XXXXXX") || exit 1
trap 'rm -rf "$TMP"' EXIT

# fixture <name> — a fresh root with the real two files copied in; echoes the root.
fixture() {
  local root="$TMP/$1"
  mkdir -p "$root/plugins/assay/scripts" "$root/plugins/assay/.claude-plugin"
  cp "$REPO/plugins/assay/paired-versions.yaml" "$root/plugins/assay/paired-versions.yaml"
  cp "$REPO/plugins/assay/.claude-plugin/plugin.json" "$root/plugins/assay/.claude-plugin/plugin.json"
  printf '%s' "$root"
}

# expect <rc> <label> [--root <dir>] — runs the script and checks its exit code.
expect() {
  local want="$1" label="$2"; shift 2
  local out rc
  out=$(bash "$SCRIPT" "$@" 2>&1); rc=$?
  if [[ "$rc" -eq "$want" ]]; then
    ok "$label"
  else
    no "$label" "wanted rc=$want, got rc=$rc; output: $(printf '%s' "$out" | tr '\n' '|')"
  fi
}

printf 'check-paired-versions.test.sh\n'

# ------------------------------------------------------------------- LIVE
expect 0 "LIVE the repo as committed passes" --root "$REPO"
expect 0 "LIVE default --root resolves to the repo root"

# ------------------------------------------------------------------- A pairing
r=$(fixture a1)
sed -i.bak 's/^plugin: .*/plugin: "0.4.0"/' "$r/plugins/assay/paired-versions.yaml"
expect 1 "A1 manifest plugin behind plugin.json version fails" --root "$r"

r=$(fixture a2)
sed -i.bak 's/"version": "[^"]*"/"version": "9.9.9"/' "$r/plugins/assay/.claude-plugin/plugin.json"
expect 1 "A2 plugin.json bumped without re-pinning the manifest fails" --root "$r"

# ------------------------------------------------------------------- B single tag
r=$(fixture b1)
# One platform pin line left behind on an older tag; the section tags still agree.
awk 'BEGIN{done=0}
     /^    darwin-amd64: statusgen-/ && !done {sub(/ v[0-9][^ ]* /, " v0.13.0 "); done=1}
     {print}' \
  "$r/plugins/assay/paired-versions.yaml" > "$r/x" && mv "$r/x" "$r/plugins/assay/paired-versions.yaml"
expect 1 "B1 one platform pin line stranded on an older tag fails" --root "$r"

r=$(fixture b2)
# desk-tools section on a different tag from statusgen (the two are cut together).
awk 'BEGIN{seen=0}
     /^  tag: / {seen++; if (seen==2) {print "  tag: v0.13.0"; next}}
     {print}' \
  "$r/plugins/assay/paired-versions.yaml" > "$r/x" && mv "$r/x" "$r/plugins/assay/paired-versions.yaml"
expect 1 "B2 desk-tools section tag differing from statusgen fails" --root "$r"

r=$(fixture b3)
sed -i.bak 's/statusgen-linux-amd64  v[0-9][^ ]*/statusgen-linux-amd64  v0.24.0/' \
  "$r/plugins/assay/paired-versions.yaml"
expect 1 "B3 a second tag anywhere in the file fails" --root "$r"

# ------------------------------------------------------------------- C hash shape
# The committed manifest may carry the reserved <harvest-after-release> placeholder
# (a tag whose release is not yet cut), so the C mutations SET the arm64 hash field
# directly rather than assuming a 64-hex value is there to bite.
setarmhash() { # <root> <value>
  sed -i.bak "s|\(statusgen-darwin-arm64 v[^ ]* \)[^ ]*|\1$2|" \
    "$1/plugins/assay/paired-versions.yaml"
}

r=$(fixture c1)
setarmhash "$r" "deadbeef"
expect 1 "C1 truncated sha256 fails" --root "$r"

r=$(fixture c2)
# A 64-char UPPER-case value — correct length, wrong case, the shape a hand-copied pin takes.
# Built at runtime (never a 64-char literal in this file, which a secret-scan would flag).
upper64=$(printf 'A%.0s' $(seq 1 64))
setarmhash "$r" "$upper64"
expect 1 "C2 upper-cased sha256 fails" --root "$r"

r=$(fixture c3)
# A STRAY placeholder that is NOT the one reserved sentinel — still fails (the allowance is a
# specific token, never a wildcard for any non-hex value).
setarmhash "$r" "TODO-harvest-from-the-release"
expect 1 "C3 stray (non-reserved) placeholder fails" --root "$r"

r=$(fixture c4)
# The RESERVED placeholder is the one legitimate not-yet-64-hex state — a manifest paired to a
# tag whose release is not yet cut. It must PASS (derived-board/06).
setarmhash "$r" "<harvest-after-release>"
expect 0 "C4 reserved <harvest-after-release> placeholder passes" --root "$r"

# ------------------------------------------------------------------- D could-not-check
r=$(fixture d1)
rm -f "$r/plugins/assay/paired-versions.yaml"
expect 1 "D1 missing paired-versions.yaml fails (not a quiet pass)" --root "$r"

r=$(fixture d2)
rm -f "$r/plugins/assay/.claude-plugin/plugin.json"
expect 1 "D2 missing plugin.json fails (not a quiet pass)" --root "$r"

r=$(fixture d3)
sed -i.bak '/^plugin: /d' "$r/plugins/assay/paired-versions.yaml"
expect 1 "D3 no top-level plugin: key fails" --root "$r"

r=$(fixture d4)
sed -i.bak 's/"version": "[^"]*",//' "$r/plugins/assay/.claude-plugin/plugin.json"
expect 1 "D4 no version field in plugin.json fails" --root "$r"

r=$(fixture d5)
# Strip every pin line: a manifest that pins nothing must not report green.
grep -v -E '^[[:space:]]+[a-z0-9-]+:[[:space:]]+[A-Za-z0-9._-]+[[:space:]]+v[0-9]' \
  "$r/plugins/assay/paired-versions.yaml" > "$r/x" && mv "$r/x" "$r/plugins/assay/paired-versions.yaml"
expect 1 "D5 a manifest with no pin lines fails" --root "$r"

expect 2 "E1 an unknown argument is a usage error, not a pass" --bogus

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
