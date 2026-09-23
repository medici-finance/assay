#!/usr/bin/env bash
# assay-install.test.sh — the proof that assay-install.sh acquires a pinned binary with NO forge
# CLI, verifies it against the pin file, and refuses in every way the digest can fail.
#
#   A  ACQUIRE — the happy path installs the verified bytes; it runs under a PATH that has no
#      `gh` and no `glab` at all, and again under a PATH whose `gh`/`glab` are tripwires that
#      record any call (none may happen).
#   N  NEGATIVE — every one of these installs NOTHING (the --dest binary must be absent):
#      a digest MISMATCH (exit 5); the platform's pin line REMOVED (exit 5 — the "expected
#      digest could not be read" failure, distinct from a mismatch); a placeholder, truncated or
#      upper-cased digest; a placeholder or floating tag; two competing lines; an absent pin
#      file; a line for another platform only; a failed fetch (exit 6); a non-https URL; and a
#      host with no sha256 tool (exit 6 — could-not-check is never a pass).
#   D  DESK-TOOLS — the tarball arm: verified tarball installs its binaries; a mismatch
#      installs none.
#   P  PIN — manifest → pin file: absent line appended; identical line untouched; scaffold
#      placeholder replaced; a DIFFERENT real line refused with the file byte-identical.
#   C  CLASSIFY — fresh / partial / adopted; adopted refuses (the refuse-not-clobber property).
#   R  REHEARSE — the whole flow against a GitLab-remote target with no forge CLI on PATH,
#      using a stub statusgen: exit 0, the real target untouched; an adopted target refuses
#      before anything is fetched; a binary whose --version does not name the pinned tag fails
#      the proof even though its digest matched (the second, independent layer).
#
# REAL (opt-in: `bash assay-install.test.sh --real`, needs the Go toolchain) — builds the real
# statusgen from this checkout stamped with a fixture tag and rehearses the flow end to end:
# `statusgen init` must scaffold `.gitlab-ci.yml` for the GitLab-remote target. The default run
# (CI's plugin-shell-suites job, which has no Go toolchain) does not select it and SAYS so on
# its last line; it is never counted as passed there.
#
# Hermetic: the "release" is a file:// tree built per case — no network, no token, no forge.
set -uo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
SCRIPT="$HERE/assay-install.sh"
REPO=$(cd "$HERE/../../.." && pwd)
REAL=0
[ "${1:-}" = "--real" ] && REAL=1

pass=0
fail=0
ok() { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
no() { printf '  FAIL %s\n     %s\n' "$1" "$2"; fail=$((fail + 1)); }

TMP=$(mktemp -d "${TMPDIR:-/tmp}/assay-install-test.XXXXXX") || exit 1
trap 'rm -rf "$TMP"' EXIT

HOME_REPO="example-org/example-repo"
TAG="v0.0.0-fixture"

plat=$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')
case "$plat" in windows-*) SG_ASSET="statusgen-$plat.exe" ;; *) SG_ASSET="statusgen-$plat" ;; esac
DT_ASSET="desk-tools-$plat.tar.gz"

sha() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
  else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# farm <dir> [tool...] — a PATH directory holding ONLY symlinks to the named tools. No gh, no
# glab: `command -v gh` in a shell on this PATH prints nothing.
BASE_TOOLS="bash sh awk grep sed tr uname mktemp rm mkdir cp cat install tar gzip curl basename dirname chmod env head git ls"
SHA_TOOLS="sha256sum shasum openssl perl"
farm() {
  local dir="$1" t p; shift
  mkdir -p "$dir"
  for t in "$@"; do
    p=$(command -v "$t" 2>/dev/null) || continue
    case "$p" in /*) ln -sf "$p" "$dir/$t" ;; esac
  done
}
NOCLI="$TMP/farm-nocli"
farm "$NOCLI" $BASE_TOOLS $SHA_TOOLS
NOSHA="$TMP/farm-nosha"
farm "$NOSHA" $BASE_TOOLS

# stub_statusgen <file> <version> — a tiny stand-in binary: --version prints <version>,
# `init --root R` scaffolds a GitLab CI file + a stream README, --lint exits 0.
stub_statusgen() {
  cat > "$1" <<EOF
#!/bin/sh
case "\$1" in
  --version) echo "$2" ;;
  init) shift; r=""
        while [ \$# -gt 0 ]; do case "\$1" in --root) r="\$2"; shift 2 ;; *) shift ;; esac; done
        : > "\$r/.gitlab-ci.yml"; mkdir -p "\$r/docs/streams/example"; echo stub > "\$r/docs/streams/example/README.md" ;;
  *) exit 0 ;;
esac
EOF
  chmod 0755 "$1"
}

# release <root> <asset> <src> — lay <src> out as <asset> of $TAG under a file:// release tree.
release() {
  mkdir -p "$1/$HOME_REPO/releases/download/$TAG"
  cp "$2" "$1/$HOME_REPO/releases/download/$TAG/$3"
}

# runp <path-dir> <out-prefix> args... — run the script under a PATH of exactly <path-dir>.
runp() {
  local pathdir="$1" pfx="$2"; shift 2
  env -i HOME="$TMP" TMPDIR="$TMP" PATH="$pathdir" bash "$SCRIPT" "$@" >"$pfx.out" 2>"$pfx.err"
}

printf 'assay-install.test.sh\n'

# ------------------------------------------------------------------ fixture release
REL="$TMP/rel"
SRC="$TMP/statusgen-src"
stub_statusgen "$SRC" "$TAG"
release "$REL" "$SRC" "$SG_ASSET"
GOOD=$(sha "$SRC")
BASE="file://$REL"

pins() { printf '# fixture pins\n%s\n' "$1" > "$2"; }

# ------------------------------------------------------------------ A acquire
case_dir() { local d="$TMP/case-$1"; mkdir -p "$d"; printf '%s' "$d"; }

d=$(case_dir A1)
pins "$SG_ASSET $TAG $GOOD" "$d/pins"
if env -i PATH="$NOCLI" bash -c 'command -v gh; command -v glab' | grep -q .; then
  no "A0 the no-CLI PATH really has no gh/glab" "command -v found one"
else
  ok "A0 the no-CLI PATH really has no gh/glab"
fi
runp "$NOCLI" "$d/r" acquire --pins "$d/pins" --dest "$d/bin" --release-home "$HOME_REPO" --base-url "$BASE"; rc=$?
if [ "$rc" -eq 0 ] && [ -x "$d/bin/statusgen" ] && [ "$("$d/bin/statusgen" --version)" = "$TAG" ] && grep -q "verified: sha256 $GOOD" "$d/r.out"; then
  ok "A1 acquire with no gh/glab on PATH installs the verified binary"
else
  no "A1 acquire with no gh/glab on PATH installs the verified binary" "rc=$rc out=$(tr '\n' '|' < "$d/r.out") err=$(tr '\n' '|' < "$d/r.err")"
fi

d=$(case_dir A2)
pins "$SG_ASSET $TAG $GOOD" "$d/pins"
TRIP="$TMP/farm-trip"; farm "$TRIP" $BASE_TOOLS $SHA_TOOLS
for c in gh glab; do printf '#!/bin/sh\necho "$0 $*" >> "%s/cli-called"\nexit 1\n' "$d" > "$TRIP/$c"; chmod +x "$TRIP/$c"; done
runp "$TRIP" "$d/r" acquire --pins "$d/pins" --dest "$d/bin" --release-home "$HOME_REPO" --base-url "$BASE"; rc=$?
if [ "$rc" -eq 0 ] && [ ! -e "$d/cli-called" ]; then
  ok "A2 acquire never invokes gh or glab even when they are on PATH"
else
  no "A2 acquire never invokes gh or glab even when they are on PATH" "rc=$rc calls=$(cat "$d/cli-called" 2>/dev/null)"
fi

# ------------------------------------------------------------------ N negative paths
# neg <label> <want-rc> <grep-in-stderr> <pins-content|@absent> [extra args...]
neg() {
  local label="$1" want="$2" pat="$3" content="$4" pathdir="$NOCLI"; shift 4
  local d; d=$(case_dir "$(printf '%s' "$label" | cut -d' ' -f1)")
  if [ "$content" != "@absent" ]; then printf '%s\n' "$content" > "$d/pins"; fi
  local args=(acquire --pins "$d/pins" --dest "$d/bin" --release-home "$HOME_REPO" --base-url "$BASE")
  if [ "${1:-}" = "--path" ]; then pathdir="$2"; shift 2; fi
  runp "$pathdir" "$d/r" "${args[@]}" "$@"; local rc=$?
  if [ "$rc" -eq "$want" ] && [ ! -e "$d/bin/statusgen" ] && grep -q -- "$pat" "$d/r.err"; then
    ok "$label"
  else
    no "$label" "want rc=$want + '$pat' + no binary; got rc=$rc bin=$(ls "$d/bin" 2>/dev/null) err=$(tr '\n' '|' < "$d/r.err")"
  fi
}

BAD=$(printf '%s' "$GOOD" | sed 's/^./0/; s/^00/11/')
[ "$BAD" = "$GOOD" ] && BAD=$(printf '%064d' 7)
neg "N1 a digest MISMATCH refuses and installs nothing"            5 "MISMATCH"                 "$SG_ASSET $TAG $BAD"
neg "N2 the platform's pin line REMOVED refuses (digest unreadable)" 5 "no pin line for $SG_ASSET" "# nothing for this platform"
neg "N3 a placeholder digest refuses"                              5 "could not be read"        "$SG_ASSET $TAG REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS"
neg "N4 a truncated digest refuses"                                5 "could not be read"        "$SG_ASSET $TAG ${GOOD:0:40}"
neg "N5 an upper-cased digest refuses"                             5 "could not be read"        "$SG_ASSET $TAG $(printf '%s' "$GOOD" | tr a-f A-F)"
neg "N6 the digest field missing entirely refuses"                 5 "could not be read"        "$SG_ASSET $TAG"
neg "N7 a placeholder tag refuses"                                 5 "not a pin"                "$SG_ASSET REPLACE_WITH_TAG $GOOD"
neg "N8 a floating 'latest' tag refuses"                           5 "not a pin"                "$SG_ASSET latest $GOOD"
neg "N9 two competing pin lines refuse"                            5 "competing"                "$(printf '%s %s %s\n%s %s %s' "$SG_ASSET" "$TAG" "$GOOD" "$SG_ASSET" "$TAG" "$BAD")"
neg "N10 an absent pin file refuses"                               5 "absent or unreadable"     "@absent"
neg "N11 a pin for another platform only refuses"                  5 "no pin line"              "statusgen-plan9-mips $TAG $GOOD"
neg "N12 a fetch that fails is could-not-check"                    6 "fetch failed"             "$SG_ASSET v9.9.9 $GOOD"
neg "N13 a non-https URL refuses"                                  5 "only https"               "$SG_ASSET $TAG $GOOD" --path "$NOCLI" --base-url "http://example.invalid"
neg "N14 no sha256 tool on PATH is could-not-check"                6 "no sha256 tool"           "$SG_ASSET $TAG $GOOD" --path "$NOSHA"
neg "N15 a path-traversal tag refuses"                             5 "unusable tag"             "$SG_ASSET ../../x $GOOD"

# ------------------------------------------------------------------ D desk-tools tarball
d=$(case_dir D1)
mkdir -p "$d/stage/hooks"
printf '#!/bin/sh\necho deskboard\n' > "$d/stage/deskboard"; printf '#!/bin/sh\necho deskpr\n' > "$d/stage/deskpr"
printf '#!/bin/sh\n' > "$d/stage/hooks/pre-push"
tar -C "$d/stage" -czf "$d/$DT_ASSET" .
release "$REL" "$d/$DT_ASSET" "$DT_ASSET"
DT_SHA=$(sha "$d/$DT_ASSET")
pins "$DT_ASSET $TAG $DT_SHA" "$d/pins"
runp "$NOCLI" "$d/r" acquire --kind desk-tools --pins "$d/pins" --dest "$d/bin" --release-home "$HOME_REPO" --base-url "$BASE"; rc=$?
if [ "$rc" -eq 0 ] && [ -x "$d/bin/deskboard" ] && [ -x "$d/bin/deskpr" ]; then
  ok "D1 a verified desk-tools tarball installs its binaries"
else
  no "D1 a verified desk-tools tarball installs its binaries" "rc=$rc err=$(tr '\n' '|' < "$d/r.err")"
fi
d=$(case_dir D2)
pins "$DT_ASSET $TAG $BAD" "$d/pins"
runp "$NOCLI" "$d/r" acquire --kind desk-tools --pins "$d/pins" --dest "$d/bin" --release-home "$HOME_REPO" --base-url "$BASE"; rc=$?
if [ "$rc" -eq 5 ] && [ -z "$(ls "$d/bin" 2>/dev/null)" ]; then
  ok "D2 a desk-tools digest mismatch installs nothing"
else
  no "D2 a desk-tools digest mismatch installs nothing" "rc=$rc bin=$(ls "$d/bin" 2>/dev/null)"
fi

# ------------------------------------------------------------------ P pin
MAN="$TMP/manifest.yaml"
cat > "$MAN" <<EOF
schema: paired-versions-v1
plugin: "0.0.0"
statusgen:
  release_home: $HOME_REPO
  tag: $TAG
  platforms:
    $plat: $SG_ASSET $TAG $GOOD
desk-tools:
  release_home: $HOME_REPO
  tag: $TAG
  platforms:
    $plat: $DT_ASSET $TAG $DT_SHA
EOF
d=$(case_dir P1)
runp "$NOCLI" "$d/r" pin --manifest "$MAN" --pins "$d/pins"; rc=$?
if [ "$rc" -eq 0 ] && grep -qx "$SG_ASSET $TAG $GOOD" "$d/pins"; then ok "P1 an absent pin line is written from the manifest"
else no "P1 an absent pin line is written from the manifest" "rc=$rc pins=$(cat "$d/pins" 2>/dev/null)"; fi
cp "$d/pins" "$d/pins.before"
runp "$NOCLI" "$d/r2" pin --manifest "$MAN" --pins "$d/pins"; rc=$?
if [ "$rc" -eq 0 ] && cmp -s "$d/pins" "$d/pins.before" && grep -q "left untouched" "$d/r2.out"; then ok "P2 an identical pin line is left untouched"
else no "P2 an identical pin line is left untouched" "rc=$rc"; fi
d=$(case_dir P3)
printf '# scaffold\n%s  REPLACE_WITH_TAG  REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS\n' "$SG_ASSET" > "$d/pins"
runp "$NOCLI" "$d/r" pin --manifest "$MAN" --pins "$d/pins"; rc=$?
if [ "$rc" -eq 0 ] && grep -qx "$SG_ASSET $TAG $GOOD" "$d/pins" && ! grep -q REPLACE_WITH "$d/pins"; then ok "P3 a scaffold placeholder line is replaced"
else no "P3 a scaffold placeholder line is replaced" "rc=$rc pins=$(tr '\n' '|' < "$d/pins")"; fi
d=$(case_dir P4)
printf '%s v0.0.1 %s\n' "$SG_ASSET" "$BAD" > "$d/pins"; cp "$d/pins" "$d/pins.before"
runp "$NOCLI" "$d/r" pin --manifest "$MAN" --pins "$d/pins"; rc=$?
if [ "$rc" -eq 5 ] && cmp -s "$d/pins" "$d/pins.before"; then ok "P4 a differing real pin line is refused, file untouched"
else no "P4 a differing real pin line is refused, file untouched" "rc=$rc"; fi

# ------------------------------------------------------------------ C classify
mkrepo() { mkdir -p "$1"; git -C "$1" init -q; git -C "$1" remote add origin "https://gitlab.example.com/example-org/example-repo.git"; }
d=$(case_dir C1); mkrepo "$d/repo"
runp "$NOCLI" "$d/r" classify --root "$d/repo"; rc=$?
[ "$rc" -eq 0 ] && grep -q '^assay-install: fresh' "$d/r.out" && ok "C1 an empty repo classifies fresh" || no "C1 an empty repo classifies fresh" "rc=$rc"
mkdir -p "$d/repo/docs/streams/svc"; echo x > "$d/repo/docs/streams/svc/README.md"
runp "$NOCLI" "$d/r2" classify --root "$d/repo"; rc=$?
[ "$rc" -eq 0 ] && grep -q '^assay-install: partial' "$d/r2.out" && ok "C2 streams without a pin classifies partial" || no "C2 streams without a pin classifies partial" "rc=$rc"
printf 'statusgen-linux-amd64  REPLACE_WITH_TAG  REPLACE_WITH_SHA256_FROM_RELEASE_CHECKSUMS\n' > "$d/repo/.assay-versions"
runp "$NOCLI" "$d/r3" classify --root "$d/repo"; rc=$?
[ "$rc" -eq 0 ] && grep -q '^assay-install: partial' "$d/r3.out" && ok "C3 a placeholder pin is not an adoption (partial)" || no "C3 a placeholder pin is not an adoption (partial)" "rc=$rc"
printf 'statusgen-linux-amd64 v1.2.3 %s\n' "$GOOD" > "$d/repo/.assay-versions"
runp "$NOCLI" "$d/r4" classify --root "$d/repo"; rc=$?
[ "$rc" -eq 5 ] && grep -q 'already adopted' "$d/r4.err" && ok "C4 streams + a real pin classifies adopted and REFUSES" || no "C4 streams + a real pin classifies adopted and REFUSES" "rc=$rc err=$(tr '\n' '|' < "$d/r4.err")"

# ------------------------------------------------------------------ R rehearse (stub statusgen)
d=$(case_dir R1); mkrepo "$d/repo"; echo "# example" > "$d/repo/README.md"
( cd "$d/repo" && find . -path ./.git -prune -o -type f -print | sort ) > "$d/before"
runp "$NOCLI" "$d/r" rehearse --target "$d/repo" --manifest "$MAN" --base-url "$BASE" --workdir "$d/work"; rc=$?
( cd "$d/repo" && find . -path ./.git -prune -o -type f -print | sort ) > "$d/after"
if [ "$rc" -eq 0 ] \
   && grep -q 'command -v gh   -> (absent)' "$d/r.out" && grep -q 'command -v glab -> (absent)' "$d/r.out" \
   && grep -q "verified: sha256 $GOOD" "$d/r.out" && grep -q 'scaffolded: .gitlab-ci.yml' "$d/r.out" \
   && grep -q 'rehearsal PROVEN' "$d/r.out" && cmp -s "$d/before" "$d/after"; then
  ok "R1 rehearse: GitLab-remote target, no forge CLI, acquired + verified + scaffolded + proven; real target untouched"
else
  no "R1 rehearse end to end" "rc=$rc out=$(tr '\n' '|' < "$d/r.out") err=$(tr '\n' '|' < "$d/r.err")"
fi

d=$(case_dir R2); mkrepo "$d/repo"
mkdir -p "$d/repo/docs/streams/svc"; echo x > "$d/repo/docs/streams/svc/README.md"
printf '%s %s %s\n' "$SG_ASSET" "$TAG" "$GOOD" > "$d/repo/.assay-versions"
runp "$NOCLI" "$d/r" rehearse --target "$d/repo" --manifest "$MAN" --base-url "$BASE" --workdir "$d/work"; rc=$?
if [ "$rc" -eq 5 ] && grep -q 'already adopted' "$d/r.err" && [ ! -e "$d/work/bin/statusgen" ]; then
  ok "R2 rehearse on an already-adopted repo REFUSES before acquiring anything"
else
  no "R2 rehearse on an already-adopted repo REFUSES" "rc=$rc err=$(tr '\n' '|' < "$d/r.err")"
fi

d=$(case_dir R3); mkrepo "$d/repo"
LIAR="$TMP/statusgen-liar"; stub_statusgen "$LIAR" "v6.6.6"
REL2="$TMP/rel2"; REL_SAVE="$REL"; REL="$REL2"; release "$REL2" "$LIAR" "$SG_ASSET"; REL="$REL_SAVE"
LIAR_SHA=$(sha "$LIAR")
sed "s/$GOOD/$LIAR_SHA/" "$MAN" > "$d/manifest.yaml"
runp "$NOCLI" "$d/r" rehearse --target "$d/repo" --manifest "$d/manifest.yaml" --base-url "file://$REL2" --workdir "$d/work"; rc=$?
if [ "$rc" -eq 5 ] && grep -q "verified: sha256 $LIAR_SHA" "$d/r.out" && grep -q 'NOT proven' "$d/r.err"; then
  ok "R3 a digest-verified binary that does not name the pinned tag fails the proof (second layer)"
else
  no "R3 second-layer proof" "rc=$rc out=$(tr '\n' '|' < "$d/r.out") err=$(tr '\n' '|' < "$d/r.err")"
fi

# ------------------------------------------------------------------ REAL (opt-in)
if [ "$REAL" -eq 1 ]; then
  d=$(case_dir REAL)
  REALTAG="v0.0.0-rehearsal"
  if ! command -v go >/dev/null 2>&1; then
    no "REAL rehearse with the real statusgen" "could-not-check: no Go toolchain on PATH to build statusgen"
  elif ! (cd "$REPO/statusgen" && go build -ldflags "-X main.statusgenVersion=$REALTAG" -o "$d/statusgen-real" .); then
    no "REAL rehearse with the real statusgen" "could-not-check: go build of statusgen failed"
  else
    RR="$d/rel"; mkdir -p "$RR/$HOME_REPO/releases/download/$REALTAG"
    cp "$d/statusgen-real" "$RR/$HOME_REPO/releases/download/$REALTAG/$SG_ASSET"
    RSHA=$(sha "$d/statusgen-real")
    printf 'statusgen:\n  release_home: %s\n  tag: %s\n  platforms:\n    %s: %s %s %s\n' \
      "$HOME_REPO" "$REALTAG" "$plat" "$SG_ASSET" "$REALTAG" "$RSHA" > "$d/manifest.yaml"
    mkrepo "$d/repo"
    runp "$NOCLI" "$d/r" rehearse --target "$d/repo" --manifest "$d/manifest.yaml" --base-url "file://$RR" --workdir "$d/work"; rc=$?
    cat "$d/r.out"
    if [ "$rc" -eq 0 ] && [ -f "$d/work/target/.gitlab-ci.yml" ] && grep -q "statusgen --version -> $REALTAG" "$d/r.out"; then
      ok "REAL rehearse: real statusgen acquired + verified, init scaffolded .gitlab-ci.yml, --version == $REALTAG, --lint == 0"
    else
      no "REAL rehearse with the real statusgen" "rc=$rc err=$(tr '\n' '|' < "$d/r.err")"
    fi
  fi
fi

printf '\n%d passed, %d failed\n' "$pass" "$fail"
[ "$REAL" -eq 1 ] || printf 'REAL case not selected (default run) — run with --real for the real-statusgen rehearsal\n'
[ "$fail" -eq 0 ]
