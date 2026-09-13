#!/bin/sh
# layer-secret-scan.test.sh — the MUTATION proof for layer-secret-scan.sh.
#
# A scan that only ever passes is a blind spot. This test proves the scan
# actually FIRES: it bakes an obviously-fake secret into a throwaway fixture
# image and asserts the scan goes RED, then asserts it stays GREEN on a clean
# fixture. Two baked mutations exercise two surfaces — a PEM in a layer file and
# a model key in an ENV default — so a scan that checked only one surface would
# fail here.
#
# The synthetic secrets are CONSTRUCTED AT RUNTIME (the PEM fence and the key
# prefix are assembled from fragments), so no real-looking key material is ever
# committed to this file — the repository leak-sweep sees only the fragments.
# The fixtures build `FROM scratch`, so the test pulls nothing.
#
# Run:  sh containers/scripts/layer-secret-scan.test.sh
set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SCAN="$HERE/layer-secret-scan.sh"

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker is required to run the layer-secret-scan mutation test" >&2
  exit 2
fi

TAG="layer-secret-scan-fixture-$$"
BAKED_LAYER="${TAG}-baked-layer:test"
BAKED_ENV="${TAG}-baked-env:test"
CLEAN="${TAG}-clean:test"
FALSEPOS="${TAG}-falsepos:test"

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t layerscantest)
# shellcheck disable=SC2329  # invoked indirectly via trap
cleanup() {
  rm -rf "$WORK"
  docker rmi -f "$BAKED_LAYER" "$BAKED_ENV" "$CLEAN" "$FALSEPOS" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

# --- synthetic, obviously-fake secrets, assembled at runtime ------------------
# The five-dash PEM fence and the key prefix are built from fragments so the
# committed source contains no whole PEM block and no whole token.
dash5='-----'
PEM_HDR="${dash5}BEGIN TESTING FAKE PRIVATE KEY${dash5}"
PEM_FTR="${dash5}END TESTING FAKE PRIVATE KEY${dash5}"
PEM_BODY='THIS-IS-NOT-A-REAL-KEY-SYNTHETIC-TEST-FIXTURE-ONLY'
ANT_PREFIX='sk-ant-'
FAKE_MODEL_KEY="${ANT_PREFIX}FAKE00000000000000000000000000000000TESTONLY"
# The scan's own generic-`sk-` pattern (layer-secret-scan.sh's MODELKEY_GENERIC),
# duplicated here only to sanity-check grep's -I vs -a binary handling directly
# against a fixture file — see fixture D point 3 below.
MODELKEY_GENERIC='sk-[A-Za-z0-9]{20,}'

build() { # <tag> <context-dir>
  if ! docker build -t "$1" "$2" > "$WORK/build.log" 2>&1; then
    echo "FAIL: could not build fixture image $1" >&2
    cat "$WORK/build.log" >&2
    exit 2
  fi
}

# --- fixture A: a fake PEM baked into a layer file ----------------------------
mkdir -p "$WORK/a"
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$WORK/a/app.pem"
cat > "$WORK/a/Dockerfile" <<'DF'
FROM scratch
COPY app.pem /run/secrets/assay/app.pem
DF
build "$BAKED_LAYER" "$WORK/a"

# --- fixture B: a fake model key baked into an ENV default --------------------
# A marker file gives the image a real filesystem layer (a layerless scratch
# image cannot be `docker save`d), so the ENV token is detected via the image
# config surface, not by a save error.
mkdir -p "$WORK/b"
printf 'marker\n' > "$WORK/b/marker.txt"
cat > "$WORK/b/Dockerfile" <<DF
FROM scratch
COPY marker.txt /marker.txt
ENV ANTHROPIC_API_KEY=$FAKE_MODEL_KEY
DF
build "$BAKED_ENV" "$WORK/b"

# --- fixture C: clean ---------------------------------------------------------
mkdir -p "$WORK/c"
printf 'no secrets in this fixture\n' > "$WORK/c/clean.txt"
cat > "$WORK/c/Dockerfile" <<'DF'
FROM scratch
COPY clean.txt /clean.txt
DF
build "$CLEAN" "$WORK/c"

# --- fixture D: false-positive regression (#908) -------------------------------
# Proves three things in one image:
#   1. PRECISION — key-shaped material at each allowlisted path (a mimic of the
#      real desk-base false positives: Go's stdlib tree, npm's bundled docs, the
#      gpgv binary path, an OPENSSH-style fence at a libssh2 path, and the real
#      `gh`-binary false positive's exact shape at the `gh` binary's path) must
#      NOT be reported.
#   2. NO BYPASS (PEM/path) — a real fake secret at an ORDINARY, non-allowlisted
#      path in the SAME image must still be caught. The allowlist is a path
#      exemption, not a pattern weakening, and this is what proves it.
#   3. NO BYPASS (generic `sk-` / binary coverage, PR #1011 review correction) —
#      a real fake generic `sk-`-shaped secret embedded in a BINARY at an
#      ordinary, non-`gh`, non-allowlisted path must ALSO still be caught. An
#      earlier revision of this fix gated the whole generic `sk-` class to
#      text-shaped content (grep -I), which would have made this exact case
#      permanently invisible; this fixture is the fail-first proof that the
#      regression is closed — see the two direct-grep assertions right after
#      this fixture is written, then the full-scan assertion further below.
mkdir -p "$WORK/d/go" "$WORK/d/npm" "$WORK/d/gpgv" "$WORK/d/libssh2" "$WORK/d/generic" "$WORK/d/real" "$WORK/d/other"

# 1a. Go-stdlib-shaped PEM at the allowlisted Go source path.
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$WORK/d/go/example-key.pem"

# 1b. npm-doc-shaped PEM mention (the real npm docs use a literal "XXXX" body;
# reproduce that shape, not a real body) at the allowlisted npm docs path.
printf 'key="%sXXXX\nXXXX\n%s"\n' "$PEM_HDR" "$PEM_FTR" > "$WORK/d/npm/config.md"

# 1c. PEM-shaped text at the gpgv binary's exact allowlisted path.
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$WORK/d/gpgv/gpgv"

# 1d. PEM-shaped text at a libssh2-shaped allowlisted path (arch dir + version
# suffix, exercising the glob).
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$WORK/d/libssh2/libssh2.so.1.0.1"

# 1e. A fake generic `sk-` key embedded in a binary-ish blob (NUL bytes) at the
# `gh` binary's path — exercises the LOOSE-pattern text-only gating (-I), not
# the path allowlist.
FAKE_GENERIC_KEY="sk-FAKE000000000000TESTONLY"
{
  printf 'ELF-ish-preamble'
  printf '\000\000'
  printf '%s' "$FAKE_GENERIC_KEY"
  printf '\000trailer\000'
} > "$WORK/d/generic/gh"

# 2. A real fake PEM at an ORDINARY path — never allowlisted, must still fire.
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$WORK/d/real/leaked.pem"

# 3. REGRESSION CLOSURE (PR #1011 review): a real fake generic `sk-`-shaped
# secret embedded in a binary-ish blob (NUL bytes, like the `gh` fixture
# above) at an ORDINARY path that is neither the `gh` binary's path nor any
# other allowlisted path. This is the case the reviewer found missing: the
# suite proved the `gh` false positive was suppressed, but never proved a
# real secret of the same shape, in a binary, elsewhere, still fails the
# build.
FAKE_GENERIC_KEY_ELSEWHERE="sk-FAKE111111111111OTHERPATH"
{
  printf 'some-other-vendored-tool-preamble'
  printf '\000\000'
  printf '%s' "$FAKE_GENERIC_KEY_ELSEWHERE"
  printf '\000trailer\000'
} > "$WORK/d/other/vendored-tool"

# Fail-first proof, independent of the scan script: demonstrate that the
# just-reverted broad `-I` (text-only) gating WOULD have made this fixture
# invisible (grep -I skips it — a binary-classified file, no match), and that
# the fixed script's `-a` (binary-as-text) gating DOES see it (a match).
if grep -IEq "$MODELKEY_GENERIC" "$WORK/d/other/vendored-tool" 2>/dev/null; then
  echo "FAIL: sanity check broken — grep -I unexpectedly matched the binary-shaped fixture (expected a miss, to prove the regression class)" >&2
  exit 2
fi
echo "sanity: grep -I misses the sk--in-binary fixture — confirms the reverted broad gating would have hidden it"
if ! grep -aEq "$MODELKEY_GENERIC" "$WORK/d/other/vendored-tool" 2>/dev/null; then
  echo "FAIL: sanity check broken — grep -a unexpectedly missed the binary-shaped fixture (expected a match)" >&2
  exit 2
fi
echo "sanity: grep -a catches the sk--in-binary fixture — confirms full binary coverage is available to the fixed script"

cat > "$WORK/d/Dockerfile" <<'DF'
FROM scratch
COPY go/example-key.pem /usr/local/go/src/crypto/tls/testdata/example-key.pem
COPY npm/config.md /usr/local/lib/node_modules/npm/docs/content/using-npm/config.md
COPY gpgv/gpgv /usr/bin/gpgv
COPY libssh2/libssh2.so.1.0.1 /usr/lib/x86_64-linux-gnu/libssh2.so.1.0.1
COPY generic/gh /usr/local/bin/gh
COPY real/leaked.pem /opt/app/leaked.pem
COPY other/vendored-tool /opt/app/vendored-tool
DF
build "$FALSEPOS" "$WORK/d"

# --- run the scan against each fixture ----------------------------------------
sh "$SCAN" "$BAKED_LAYER" > "$WORK/out.baked-layer" 2>&1; RC_BAKED_LAYER=$?
sh "$SCAN" "$BAKED_ENV"   > "$WORK/out.baked-env"   2>&1; RC_BAKED_ENV=$?
sh "$SCAN" "$CLEAN"       > "$WORK/out.clean"       2>&1; RC_CLEAN=$?
sh "$SCAN" "$FALSEPOS"    > "$WORK/out.falsepos"    2>&1; RC_FALSEPOS=$?

fail=0

echo "layer-secret-scan mutation test"
printf '  baked PEM-in-layer fixture -> scan exit %s\n' "$RC_BAKED_LAYER"
printf '  baked key-in-ENV  fixture  -> scan exit %s\n' "$RC_BAKED_ENV"
printf '  clean fixture              -> scan exit %s\n' "$RC_CLEAN"
printf '  false-positive fixture     -> scan exit %s\n' "$RC_FALSEPOS"

# The mutation assertion: BOTH baked-secret fixtures must be detected (non-zero).
if [ "$RC_BAKED_LAYER" -ne 0 ] && [ "$RC_BAKED_ENV" -ne 0 ]; then
  echo "RED on baked-key fixture"
else
  echo "FAIL: a baked-secret fixture was NOT detected by the scan" >&2
  [ "$RC_BAKED_LAYER" -eq 0 ] && cat "$WORK/out.baked-layer" >&2
  [ "$RC_BAKED_ENV" -eq 0 ] && cat "$WORK/out.baked-env" >&2
  fail=1
fi

# The clean fixture must pass (exit 0) — no false positive.
if [ "$RC_CLEAN" -eq 0 ]; then
  echo "GREEN on clean fixture"
else
  echo "FAIL: the clean fixture was flagged by the scan (false positive)" >&2
  cat "$WORK/out.clean" >&2
  fail=1
fi

# The false-positive fixture (#908) must RED overall (the real secret at an
# ordinary path must still be caught — no bypass) AND must name only the real
# secret's path, never any of the five allowlisted-path mimics.
if [ "$RC_FALSEPOS" -eq 0 ]; then
  echo "FAIL: false-positive fixture was clean — the real secret at /opt/app/leaked.pem was NOT caught (allowlist created a bypass)" >&2
  cat "$WORK/out.falsepos" >&2
  fail=1
else
  if grep -q '/opt/app/leaked\.pem' "$WORK/out.falsepos"; then
    echo "RED on false-positive fixture's real secret (/opt/app/leaked.pem) — no bypass"
  else
    echo "FAIL: false-positive fixture went RED but not on the real secret at /opt/app/leaked.pem" >&2
    cat "$WORK/out.falsepos" >&2
    fail=1
  fi
  if grep -qE '/usr/local/go/src/|/usr/local/lib/node_modules/npm/|/usr/bin/gpgv|/usr/lib/.*/libssh2\.so|/usr/local/bin/gh' "$WORK/out.falsepos"; then
    echo "FAIL: false-positive fixture reported one of the allowlisted-path mimics (precision regression)" >&2
    cat "$WORK/out.falsepos" >&2
    fail=1
  else
    echo "GREEN on all five allowlisted-path mimics (Go src, npm docs, gpgv, libssh2, gh) — none reported"
  fi

  # REGRESSION CLOSURE (PR #1011 review): the generic `sk-`-shaped secret
  # embedded in a binary at the ORDINARY /opt/app/vendored-tool path (not the
  # gh binary, not allowlisted) must ALSO be reported. This is the case that
  # would have stayed invisible forever under the reverted blanket `-I`
  # (text-only) gating for the whole LOOSE class — see the direct grep -I vs
  # -a sanity check made right after the fixture was written, above. Catching
  # it here proves the fix restores full binary coverage for the generic
  # `sk-` pattern everywhere except the narrow, specific allowlist entries.
  if grep -q '/opt/app/vendored-tool' "$WORK/out.falsepos"; then
    echo "RED on the generic sk--in-binary regression fixture (/opt/app/vendored-tool) — binary coverage restored, no regression"
  else
    echo "FAIL: the generic sk--shaped secret embedded in a binary at an ordinary, non-gh path (/opt/app/vendored-tool) was NOT caught — the #908 binary-coverage regression is still present" >&2
    cat "$WORK/out.falsepos" >&2
    fail=1
  fi
fi

exit "$fail"
