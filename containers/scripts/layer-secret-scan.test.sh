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

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t layerscantest)
# shellcheck disable=SC2329  # invoked indirectly via trap
cleanup() {
  rm -rf "$WORK"
  docker rmi -f "$BAKED_LAYER" "$BAKED_ENV" "$CLEAN" >/dev/null 2>&1 || true
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

# --- run the scan against each fixture ----------------------------------------
sh "$SCAN" "$BAKED_LAYER" > "$WORK/out.baked-layer" 2>&1; RC_BAKED_LAYER=$?
sh "$SCAN" "$BAKED_ENV"   > "$WORK/out.baked-env"   2>&1; RC_BAKED_ENV=$?
sh "$SCAN" "$CLEAN"       > "$WORK/out.clean"       2>&1; RC_CLEAN=$?

fail=0

echo "layer-secret-scan mutation test"
printf '  baked PEM-in-layer fixture -> scan exit %s\n' "$RC_BAKED_LAYER"
printf '  baked key-in-ENV  fixture  -> scan exit %s\n' "$RC_BAKED_ENV"
printf '  clean fixture              -> scan exit %s\n' "$RC_CLEAN"

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

exit "$fail"
