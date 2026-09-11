#!/usr/bin/env bash
# stamp-plugin-version.test.sh — the proof that stamp-plugin-version.sh writes every file the
# plugin version lives in, refuses what it cannot prove, and that its release gate catches the
# drift #789 shipped (plugin content moved, manifest version did not).
#
#   LIVE  the tree as committed is internally consistent: every file carries the version
#         plugin.json declares. This is the assertion that arms the guard in CI: this suite is
#         globbed by ci.yml's `plugin-shell-suites` job, so a hand edit that moves one file and
#         not the others reddens the PR instead of shipping a half-stamped plugin.
#   S     STAMP — every file carries the new version afterwards; a second stamp is a byte-level
#         no-op (idempotent, so a re-run of the release step commits nothing spurious); only the
#         `assay` entry of a multi-plugin marketplace catalog is touched; a bad tag is refused
#         with nothing written.
#   C     CHECK — an unstamped tree fails naming every stale file; a stamped tree passes.
#   N     COULD-NOT-CHECK — a missing file, or a file whose version token is gone, exits 2 with
#         NOTHING written, never a quiet partial stamp.
#   G     GATE — in a throwaway git repo: a plugins/assay/ change with an unchanged manifest
#         version is refused; the dispatch shape (--have = the version the tree will carry)
#         passes; a stamped tree passes; tag/manifest inequality is refused even with no
#         plugin change; no previous tag skips the diff and says so; an unfetchable previous
#         ref is could-not-check.
#
# Fixtures copy the real files into a temp root and mutate the copy, so a reshaping of the real
# manifests cannot leave the suite testing a stale shape. Hermetic: no network, no token, no jq,
# no Go. Run:  bash plugins/assay/scripts/stamp-plugin-version.test.sh
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/stamp-plugin-version.sh"
REPO=$(cd "$HERE/../../.." && pwd)

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

TMP=$(mktemp -d "${TMPDIR:-/tmp}/stamp-plugin-version.XXXXXX") || exit 1
trap 'rm -rf "$TMP"' EXIT

PATHS=$(bash "$SCRIPT" paths)

# fixture <name> — a fresh root with the real files copied in; echoes the root.
fixture() {
  local root="$TMP/$1" f
  while IFS= read -r f; do
    mkdir -p "$root/$(dirname "$f")"
    cp "$REPO/$f" "$root/$f"
  done <<< "$PATHS"
  printf '%s' "$root"
}

# expect <rc> <label> <args…> — runs the script and checks its exit code.
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

# tree_digest <root> — one digest over the bytes of every stamped file, for idempotency checks.
tree_digest() {
  local root="$1" f
  while IFS= read -r f; do cat "$root/$f"; done <<< "$PATHS" | cksum
}

# count_carrying <root> <version> — how many of the stamped files mention the version string.
count_carrying() {
  local root="$1" ver="$2" f n=0
  while IFS= read -r f; do
    if grep -q -F -e "\"$ver\"" -e "plugin v$ver)" "$root/$f"; then n=$((n + 1)); fi
  done <<< "$PATHS"
  printf '%s' "$n"
}

printf 'stamp-plugin-version.test.sh\n'

# ------------------------------------------------------------------- LIVE
live_version=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$REPO/plugins/assay/.claude-plugin/plugin.json" | head -1)
if [[ "$live_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  ok "LIVE plugin.json declares a MAJOR.MINOR.PATCH version ($live_version)"
else
  no "LIVE plugin.json declares a MAJOR.MINOR.PATCH version" "got '$live_version'"
fi
expect 0 "LIVE the tree as committed carries $live_version in every file" check "v$live_version" --root "$REPO"
expect 0 "LIVE default --root resolves to the repo root" check "v$live_version"
n=$(printf '%s\n' "$PATHS" | grep -c .)
if [[ "$n" -eq 7 ]]; then ok "paths lists the seven version-bearing files"; else no "paths lists the seven version-bearing files" "got $n"; fi

# ------------------------------------------------------------------- S stamp
r=$(fixture s1)
expect 0 "S1 stamp v9.9.9 exits 0" stamp v9.9.9 --root "$r"
c=$(count_carrying "$r" 9.9.9)
if [[ "$c" -eq 7 ]]; then ok "S1 every one of the seven files carries 9.9.9 afterwards"; else no "S1 every file carries 9.9.9" "only $c of 7 do"; fi
expect 0 "S1 check v9.9.9 passes on the stamped tree" check v9.9.9 --root "$r"
expect 1 "S1 check for a different version fails on the stamped tree" check v9.9.8 --root "$r"

d1=$(tree_digest "$r")
expect 0 "S2 a second stamp of the same version exits 0" stamp v9.9.9 --root "$r"
d2=$(tree_digest "$r")
if [[ "$d1" = "$d2" ]]; then ok "S2 the second stamp is a byte-level no-op (idempotent)"; else no "S2 idempotent" "bytes changed on re-stamp"; fi

r=$(fixture s3)
# A catalog with another plugin BEFORE and AFTER the assay entry, each on its own version.
python_free_catalog='{
  "name": "assay",
  "owner": { "name": "example-owner" },
  "plugins": [
    {
      "name": "example-before",
      "source": "./plugins/example-before",
      "version": "0.0.1",
      "author": { "name": "example-owner" }
    },
    {
      "name": "assay",
      "source": "./plugins/assay",
      "description": "example",
      "version": "0.1.0",
      "author": { "name": "example-owner" }
    },
    {
      "name": "example-after",
      "source": "./plugins/example-after",
      "version": "0.0.2"
    }
  ]
}'
printf '%s\n' "$python_free_catalog" > "$r/.claude-plugin/marketplace.json"
expect 0 "S3 stamp with a multi-plugin catalog exits 0" stamp v9.9.9 --root "$r"
if grep -q '"version": "0.0.1"' "$r/.claude-plugin/marketplace.json" && grep -q '"version": "0.0.2"' "$r/.claude-plugin/marketplace.json"; then
  ok "S3 the other catalog entries keep their own versions"
else
  no "S3 the other catalog entries keep their own versions" "$(tr '\n' '|' < "$r/.claude-plugin/marketplace.json")"
fi
if [[ "$(grep -c '"version": "9.9.9"' "$r/.claude-plugin/marketplace.json")" -eq 1 ]]; then
  ok "S3 exactly the assay entry moved to 9.9.9"
else
  no "S3 exactly the assay entry moved to 9.9.9" "$(grep -c '"version": "9.9.9"' "$r/.claude-plugin/marketplace.json") entries carry it"
fi

r=$(fixture s4)
d1=$(tree_digest "$r")
expect 2 "S4 a bare X.Y.Z (no leading v) is refused" stamp 1.2.3 --root "$r"
expect 2 "S4 a two-part vX.Y is refused" stamp v1.2 --root "$r"
expect 2 "S4 a pre-release suffix is refused" stamp v1.2.3-rc1 --root "$r"
expect 2 "S4 a missing version is refused" stamp --root "$r"
expect 2 "S4 an unknown verb is refused" frobnicate v1.2.3 --root "$r"
d2=$(tree_digest "$r")
if [[ "$d1" = "$d2" ]]; then ok "S4 a refused stamp writes nothing"; else no "S4 a refused stamp writes nothing" "bytes changed"; fi

# ------------------------------------------------------------------- C check
r=$(fixture c1)
out=$(bash "$SCRIPT" check v9.9.9 --root "$r" 2>&1); rc=$?
if [[ "$rc" -eq 1 ]]; then ok "C1 check on an unstamped tree fails (rc=1)"; else no "C1 check on an unstamped tree fails" "rc=$rc"; fi
missing=""
while IFS= read -r f; do
  printf '%s' "$out" | grep -q -F "FAIL $f" || missing="$missing $f"
done <<< "$PATHS"
if [[ -z "$missing" ]]; then ok "C1 every stale file is named"; else no "C1 every stale file is named" "not named:$missing"; fi

r=$(fixture c2)
sed 's/"version": "[^"]*"/"version": "9.9.9"/' "$r/plugins/assay/.claude-plugin/plugin.json" > "$r/x" && mv "$r/x" "$r/plugins/assay/.claude-plugin/plugin.json"
expect 1 "C2 plugin.json moved alone (the hand-edit shape) fails check" check v9.9.9 --root "$r"

# ------------------------------------------------------------------- N could-not-check
r=$(fixture n1)
rm "$r/.claude-plugin/marketplace.json"
d1=$(tree_digest "$r" 2>/dev/null)
expect 2 "N1 a missing marketplace.json is could-not-check (rc=2) on stamp" stamp v9.9.9 --root "$r"
expect 2 "N1 a missing marketplace.json is could-not-check (rc=2) on check" check v9.9.9 --root "$r"
if [[ "$(count_carrying "$r" 9.9.9 2>/dev/null)" -eq 0 ]]; then ok "N1 nothing was written"; else no "N1 nothing was written" "some file carries 9.9.9"; fi

r=$(fixture n2)
sed 's/(assay plugin v[^)]*)/(assay plugin)/' "$r/plugins/assay/hooks/resident-rules.payload.txt" > "$r/x" && mv "$r/x" "$r/plugins/assay/hooks/resident-rules.payload.txt"
expect 2 "N2 a payload whose Header token is gone is could-not-check on stamp" stamp v9.9.9 --root "$r"
if [[ "$(count_carrying "$r" 9.9.9)" -eq 0 ]]; then ok "N2 nothing was written (no half-stamped tree)"; else no "N2 nothing was written" "some file carries 9.9.9"; fi
expect 2 "N2 a payload whose Header token is gone is could-not-check on check" check v9.9.9 --root "$r"

r=$(fixture n3)
sed '/^plugin:/d' "$r/plugins/assay/paired-versions.yaml" > "$r/x" && mv "$r/x" "$r/plugins/assay/paired-versions.yaml"
expect 2 "N3 a paired-versions.yaml with no plugin: key is could-not-check" stamp v9.9.9 --root "$r"

r=$(fixture n4)
sed 's/"name": "assay"/"name": "example-renamed"/' "$r/.claude-plugin/marketplace.json" > "$r/x" && mv "$r/x" "$r/.claude-plugin/marketplace.json"
expect 2 "N4 a catalog with no assay entry is could-not-check" stamp v9.9.9 --root "$r"

# ------------------------------------------------------------------- G gate
if command -v git >/dev/null 2>&1; then
  g="$TMP/gate"
  mkdir -p "$g"
  while IFS= read -r f; do mkdir -p "$g/$(dirname "$f")"; cp "$REPO/$f" "$g/$f"; done <<< "$PATHS"
  mkdir -p "$g/plugins/assay/skills/example-skill"
  printf '# example skill\n' > "$g/plugins/assay/skills/example-skill/SKILL.md"
  gitc() { git -C "$g" -c user.name=example-tester -c user.email=example-tester@example.invalid "$@"; }
  gitc init -q . 2>/dev/null
  bash "$SCRIPT" stamp v1.0.0 --root "$g" >/dev/null 2>&1
  gitc add -A >/dev/null && gitc commit -q -m 'example: plugin 1.0.0' && gitc tag v1.0.0

  # A plugin change without a manifest bump — the #789 shape.
  printf 'changed\n' >> "$g/plugins/assay/skills/example-skill/SKILL.md"
  gitc commit -q -a -m 'example: skill change, no bump'

  expect 1 "G1 plugins/assay changed + manifest unchanged is REFUSED (push-path shape)" gate --tag v1.0.1 --prev v1.0.0 --root "$g"
  expect 0 "G2 the dispatch shape (--have = the version the tree will carry) passes" gate --tag v1.0.1 --prev v1.0.0 --have 1.0.1 --root "$g"
  out=$(bash "$SCRIPT" gate --tag v1.0.1 --prev '' --root "$g" 2>&1); rc=$?
  if [[ "$rc" -eq 1 ]] && printf '%s' "$out" | grep -q 'no previous umbrella tag'; then
    ok "G3 no previous tag: the diff check is skipped and said so; tag/manifest inequality still refuses"
  else
    no "G3 no previous tag" "rc=$rc; output: $(printf '%s' "$out" | tr '\n' '|')"
  fi
  expect 0 "G3 no previous tag + --have equal to the tag passes" gate --tag v1.0.1 --prev '' --have 1.0.1 --root "$g"
  expect 2 "G4 an unfetchable previous ref is could-not-check (rc=2)" gate --tag v1.0.1 --prev v9.9.9 --root "$g"
  expect 2 "G4 gate without --prev is a usage error" gate --tag v1.0.1 --root "$g"
  expect 2 "G4 gate with a malformed --tag is a usage error" gate --tag 1.0.1 --prev v1.0.0 --root "$g"

  bash "$SCRIPT" stamp v1.0.1 --root "$g" >/dev/null 2>&1
  gitc commit -q -a -m 'example: stamp 1.0.1'
  expect 0 "G5 a stamped tree (content moved AND version moved) passes" gate --tag v1.0.1 --prev v1.0.0 --root "$g"
  expect 1 "G6 a stamped tree tagged as a DIFFERENT version is refused" gate --tag v1.0.2 --prev v1.0.0 --root "$g"

  # No plugin change at all, version unchanged: the diff check passes, equality still decides.
  gitc tag v1.0.1
  printf 'unrelated\n' > "$g/README.md"
  gitc add README.md >/dev/null && gitc commit -q -m 'example: unrelated change'
  expect 0 "G7 no plugin change since the previous tag, re-cutting the same manifest version passes the diff check" gate --tag v1.0.1 --prev v1.0.1 --root "$g"
  expect 1 "G7 no plugin change but the tag moved past the manifest is refused (the tree must carry the tag)" gate --tag v1.0.2 --prev v1.0.1 --root "$g"
  expect 0 "G7 …and the dispatch shape, which will stamp 1.0.2, passes" gate --tag v1.0.2 --prev v1.0.1 --have 1.0.2 --root "$g"
else
  printf '  skip G gate cases: git not on PATH\n'
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[[ "$fail" -eq 0 ]]
