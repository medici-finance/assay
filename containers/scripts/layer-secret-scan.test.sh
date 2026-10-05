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
# Three further fixture sets (E, F, G, #2256) bake a key into an EARLIER layer
# and then hide it from a merged view of the image with a LATER layer: an
# overwrite at the same path, a sandwiched overwrite, and a replacement by a
# directory. Each must still go RED, because the earlier layer still ships the
# key. layer-secret-scan.mutate.sh runs this test against a merged-view mutant
# of the scan and requires every one of those fixtures to fail under it.
#
# The fail-closed fixtures (H to O) hold the scan to its exit-2 contract: an
# image it cannot read in full is never reported clean. Most are fed through a
# stand-in `docker` on PATH, since docker itself will not build or save them;
# a stand-in clean image is the control that the stand-in alone reads clean.
# Fixture N (unreadable modes) is a real BuildKit build and must go RED with
# the hidden keys named. layer-secret-scan.mutate.sh reverts each fail-closed
# step on its own and requires its fixture, and only its fixture, to fail.
#
# The synthetic secrets are CONSTRUCTED AT RUNTIME (the PEM fence and the key
# prefix are assembled from fragments), so no real-looking key material is ever
# committed to this file — the repository leak-sweep sees only the fragments.
# The fixtures build `FROM scratch` (or are assembled and `docker load`ed
# locally), so the test pulls nothing.
#
# Run:  sh containers/scripts/layer-secret-scan.test.sh
#
# LAYER_SECRET_SCAN=<path> runs the fixtures against another copy of the scan.
# It exists for layer-secret-scan.mutate.sh (and for a fail-first run against
# an older revision); the Verify row runs without it.
set -u

HERE=$(cd "$(dirname "$0")" && pwd)
SCAN="${LAYER_SECRET_SCAN:-$HERE/layer-secret-scan.sh}"

if ! command -v docker >/dev/null 2>&1; then
  echo "FAIL: docker is required to run the layer-secret-scan mutation test" >&2
  exit 2
fi

TAG="layer-secret-scan-fixture-$$"
BAKED_LAYER="${TAG}-baked-layer:test"
BAKED_ENV="${TAG}-baked-env:test"
CLEAN="${TAG}-clean:test"
FALSEPOS="${TAG}-falsepos:test"
OVERWRITE_NS="1 2 3 4 5 6"   # fixture E variants
SANDWICH_NS="1 2 3"          # fixture F variants
DIRSWAP="${TAG}-dirswap:test" # fixture G
UNREAD="${TAG}-unreadable:test" # fixture N
ow_tag() { printf '%s-overwrite-%s:test' "$TAG" "$1"; }
sw_tag() { printf '%s-sandwich-%s:test' "$TAG" "$1"; }

WORK=$(mktemp -d 2>/dev/null || mktemp -d -t layerscantest)
# shellcheck disable=SC2329  # invoked indirectly via trap
cleanup() {
  chmod -R u+rwX "$WORK" 2>/dev/null
  rm -rf "$WORK"
  docker rmi -f "$BAKED_LAYER" "$BAKED_ENV" "$CLEAN" "$FALSEPOS" "$DIRSWAP" "$UNREAD" >/dev/null 2>&1 || true
  for _n in $OVERWRITE_NS; do docker rmi -f "$(ow_tag "$_n")" >/dev/null 2>&1 || true; done
  for _n in $SANDWICH_NS; do docker rmi -f "$(sw_tag "$_n")" >/dev/null 2>&1 || true; done
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

# --- fixtures E/F/G: a key in an EARLIER layer, hidden by a LATER one (#2256) --
# containers/secrets.md section 6 forbids a credential left in an earlier layer,
# because that layer ships in the image whatever later layers do to the path.
# A scan that merges every layer into ONE tree before grepping sees only the
# last writer of each path, so it misses all three shapes below. Worse, the
# order it merged in was the blob discovery order, not the layer order, so the
# same image passed or failed depending on content hashes — which is why E and
# F build several variants that differ only in the benign content: a merged
# scan that happens to extract the key's layer last for one variant still
# misses another. G does not depend on order at all: a regular file cannot
# replace a non-empty directory, and a directory replaces a file, so in either
# order a merged tree ends up without the key.
#
# E — the reported shape: COPY the key, then COPY a benign file over it.
for n in $OVERWRITE_NS; do
  mkdir -p "$WORK/e$n"
  {
    printf '%s\n' "$PEM_HDR"
    printf '%s\n' "$PEM_BODY"
    printf '%s\n' "$PEM_FTR"
  } > "$WORK/e$n/secret.txt"
  printf 'scrubbed-%s\n' "$n" > "$WORK/e$n/benign.txt"
  cat > "$WORK/e$n/Dockerfile" <<'DF'
FROM scratch
COPY secret.txt /app/key.pem
COPY benign.txt /app/key.pem
DF
  build "$(ow_tag "$n")" "$WORK/e$n"
done

# F — the key sandwiched between two benign writes of the same path, so a
# merged scan misses it whether it extracts in layer order or in reverse.
for n in $SANDWICH_NS; do
  mkdir -p "$WORK/f$n"
  printf 'placeholder-before\n' > "$WORK/f$n/before.txt"
  {
    printf '%s\n' "$PEM_HDR"
    printf '%s\n' "$PEM_BODY"
    printf '%s\n' "$PEM_FTR"
  } > "$WORK/f$n/secret.txt"
  printf 'scrubbed-after-%s\n' "$n" > "$WORK/f$n/after.txt"
  cat > "$WORK/f$n/Dockerfile" <<'DF'
FROM scratch
COPY before.txt /app/key.pem
COPY secret.txt /app/key.pem
COPY after.txt /app/key.pem
DF
  build "$(sw_tag "$n")" "$WORK/f$n"
done

# G — layer 1 writes the key at /app/key.pem (and a second key at
# /app/gone.pem); layer 2 replaces /app/key.pem with a non-empty DIRECTORY and
# deletes /app/gone.pem with a whiteout entry. A Dockerfile cannot express the
# directory swap without pulling a base image to RUN in, so the two layer tars
# and a minimal docker-archive are assembled here and `docker load`ed.
# /app/gone.pem is the control inside G: a whiteout is only a marker file, so
# even a merged scan still sees the deleted key; /app/key.pem is the case a
# merged scan loses.
sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}
G="$WORK/g"
mkdir -p "$G/l1/app" "$G/l2/app/key.pem" "$G/arc/l1" "$G/arc/l2"
{
  printf '%s\n' "$PEM_HDR"
  printf '%s\n' "$PEM_BODY"
  printf '%s\n' "$PEM_FTR"
} > "$G/l1/app/key.pem"
cp "$G/l1/app/key.pem" "$G/l1/app/gone.pem"
printf 'not a key\n' > "$G/l2/app/key.pem/readme.txt"
: > "$G/l2/app/.wh.gone.pem"
# COPYFILE_DISABLE keeps macOS tar from adding AppleDouble ._* members.
COPYFILE_DISABLE=1 tar -cf "$G/arc/l1/layer.tar" -C "$G/l1" app
COPYFILE_DISABLE=1 tar -cf "$G/arc/l2/layer.tar" -C "$G/l2" app
G_ARCH=$(docker version --format '{{.Server.Arch}}' 2>/dev/null)
if [ -z "$G_ARCH" ]; then
  echo "FAIL: could not read the docker server architecture for fixture G" >&2
  exit 2
fi
printf '{"architecture":"%s","os":"linux","config":{},"rootfs":{"type":"layers","diff_ids":["sha256:%s","sha256:%s"]}}\n' \
  "$G_ARCH" "$(sha256_of "$G/arc/l1/layer.tar")" "$(sha256_of "$G/arc/l2/layer.tar")" > "$G/arc/config.json"
printf '[{"Config":"config.json","RepoTags":["%s"],"Layers":["l1/layer.tar","l2/layer.tar"]}]\n' \
  "$DIRSWAP" > "$G/arc/manifest.json"
COPYFILE_DISABLE=1 tar -cf "$G/image.tar" -C "$G/arc" manifest.json config.json l1 l2
if ! docker load -i "$G/image.tar" > "$WORK/load.log" 2>&1; then
  echo "FAIL: could not load fixture image $DIRSWAP" >&2
  cat "$WORK/load.log" >&2
  exit 2
fi

# --- fixtures H-O: the scan fails closed on what it cannot read ---------------
# A stand-in `docker` serves a hand-built save tar ($STUB_SAVE) for the cases
# docker will not produce. Its history and image config are benign, so only
# the layer surface decides each result.
STUB="$WORK/stub"
SIGSTUB="$WORK/sigstub"
mkdir -p "$STUB" "$SIGSTUB"
cat > "$STUB/docker" <<'SH'
#!/bin/sh
case "$1" in
  history) echo 'COPY stub /' ;;
  inspect) echo '[{"Config":{"Env":[]}}]' ;;
  save)
    while [ $# -gt 0 ]; do
      [ "$1" = -o ] && exec cp "$STUB_SAVE" "$2"
      shift
    done
    exit 1 ;;
  *) exit 1 ;;
esac
SH
# The signal fixture's stand-in `sort` (the scan calls sort once, after every
# grep): it sends TERM to the scan, its parent, then sorts as usual.
cat > "$SIGSTUB/sort" <<'SH'
#!/bin/sh
kill -TERM "$PPID"
exec "$REAL_SORT" "$@"
SH
chmod +x "$STUB/docker" "$SIGSTUB/sort"
REAL_SORT=$(command -v sort)
export REAL_SORT

pem_to() { # <file>: write the fake PEM there
  mkdir -p "$(dirname "$1")"
  {
    printf '%s\n' "$PEM_HDR"
    printf '%s\n' "$PEM_BODY"
    printf '%s\n' "$PEM_FTR"
  } > "$1"
}
benign_layer() { # <layer.tar>: a layer holding one benign file
  mkdir -p "$WORK/benign/app" "$(dirname "$1")"
  printf 'not a key\n' > "$WORK/benign/app/readme.txt"
  COPYFILE_DISABLE=1 tar -cf "$1" -C "$WORK/benign" app
}
stub_save() { # <dir> <layer>...: <dir>.save.tar from <dir>'s layer tars
  _d="$1"; shift
  _l=""
  for _p in "$@"; do _l="${_l:+$_l,}\"$_p\""; done
  printf '{"os":"linux","rootfs":{"type":"layers"}}\n' > "$_d/config.json"
  printf '[{"Config":"config.json","RepoTags":["stub:test"],"Layers":[%s]}]\n' \
    "$_l" > "$_d/manifest.json"
  (cd "$_d" && COPYFILE_DISABLE=1 tar -cf "$_d.save.tar" manifest.json config.json "$@")
}

# H0 — control: a stand-in image with one benign layer must scan clean.
mkdir -p "$WORK/h0"
benign_layer "$WORK/h0/l1/layer.tar"
stub_save "$WORK/h0" l1/layer.tar

# H1 — the save is not a tar at all.
printf 'not a tar archive\n' > "$WORK/h1.save.tar"

# H2 — the save extracts only in part: l1 (benign) unpacks, then l2/layer.tar,
# which holds the key, sits under a symlink tar refuses to write through.
mkdir -p "$WORK/h2/l1" "$WORK/h2lnk" "$WORK/h2key/l2"
benign_layer "$WORK/h2/l1/layer.tar"
ln -s /nonexistent-layerscan-fixture "$WORK/h2lnk/l2"
pem_to "$WORK/h2k/app/key.pem"
COPYFILE_DISABLE=1 tar -cf "$WORK/h2key/l2/layer.tar" -C "$WORK/h2k" app
stub_save "$WORK/h2" l1/layer.tar
COPYFILE_DISABLE=1 tar -cf "$WORK/h2.save.tar" -C "$WORK/h2" manifest.json \
  config.json l1/layer.tar -C "$WORK/h2lnk" l2 -C "$WORK/h2key" l2/layer.tar

# I — a layer whose key member tar refuses to extract (it sits under the
# layer's own symlink to a path outside the layer).
mkdir -p "$WORK/i/l1" "$WORK/ilnk" "$WORK/ikey"
ln -s /nonexistent-layerscan-fixture "$WORK/ilnk/lnk"
pem_to "$WORK/ikey/lnk/key.pem"
COPYFILE_DISABLE=1 tar -cf "$WORK/i/l1/layer.tar" -C "$WORK/ilnk" lnk \
  -C "$WORK/ikey" lnk/key.pem
stub_save "$WORK/i" l1/layer.tar

# J — a save with no layer at all.
mkdir -p "$WORK/j"
stub_save "$WORK/j"

# K — a benign layer, then a gzip layer holding the key, cut short so tar
# cannot list it.
mkdir -p "$WORK/k/l1" "$WORK/k/l2" "$WORK/kk"
benign_layer "$WORK/k/l1/layer.tar"
pem_to "$WORK/kk/app/key.pem"
COPYFILE_DISABLE=1 tar -czf "$WORK/kk.tgz" -C "$WORK/kk" app
head -c 40 "$WORK/kk.tgz" > "$WORK/k/l2/layer.tar"
stub_save "$WORK/k" l1/layer.tar l2/layer.tar

# L — a benign layer, then a zstd-framed blob tar cannot list.
mkdir -p "$WORK/l/l1" "$WORK/l/l2"
benign_layer "$WORK/l/l1/layer.tar"
printf '\050\265\057\375not-a-readable-frame' > "$WORK/l/l2/layer.tar"
stub_save "$WORK/l" l1/layer.tar l2/layer.tar

# M — the key in a file whose name is an allowlisted path plus a newline.
mkdir -p "$WORK/m/l1" "$WORK/mm/usr/bin"
NLNAME=$(printf 'gpgv\nx')
NLNAME=${NLNAME%x}
pem_to "$WORK/mm/usr/bin/$NLNAME"
COPYFILE_DISABLE=1 tar -cf "$WORK/m/l1/layer.tar" -C "$WORK/mm" usr
stub_save "$WORK/m" l1/layer.tar

# N — a real build: a key in a mode-000 file, and a key under a directory with
# no search bit. The scan must read both and go RED with each named.
mkdir -p "$WORK/n/locked"
printf 'not a key\n' > "$WORK/n/readme.txt"
pem_to "$WORK/n/secret.txt"
pem_to "$WORK/n/locked/key.pem"
cat > "$WORK/n/Dockerfile" <<'DF'
FROM scratch
COPY readme.txt /app/readme.txt
COPY --chmod=000 secret.txt /app/key.pem
COPY --chmod=600 locked /app/locked
DF
build "$UNREAD" "$WORK/n"

# O — a signal mid-scan (the stand-in sort, over the clean H0 image).

# --- run the scan against each fixture ----------------------------------------
sh "$SCAN" "$BAKED_LAYER" > "$WORK/out.baked-layer" 2>&1; RC_BAKED_LAYER=$?
sh "$SCAN" "$BAKED_ENV"   > "$WORK/out.baked-env"   2>&1; RC_BAKED_ENV=$?
sh "$SCAN" "$CLEAN"       > "$WORK/out.clean"       2>&1; RC_CLEAN=$?
sh "$SCAN" "$FALSEPOS"    > "$WORK/out.falsepos"    2>&1; RC_FALSEPOS=$?
sh "$SCAN" "$DIRSWAP"     > "$WORK/out.dirswap"     2>&1; RC_DIRSWAP=$?
for n in $OVERWRITE_NS; do
  sh "$SCAN" "$(ow_tag "$n")" > "$WORK/out.overwrite-$n" 2>&1
  echo "$?" > "$WORK/rc.overwrite-$n"
done
for n in $SANDWICH_NS; do
  sh "$SCAN" "$(sw_tag "$n")" > "$WORK/out.sandwich-$n" 2>&1
  echo "$?" > "$WORK/rc.sandwich-$n"
done
sh "$SCAN" "$UNREAD" > "$WORK/out.unread" 2>&1; RC_UNREAD=$?
stub_scan() { # <fixture> <save.tar> [<extra PATH dir>]: scan via the stand-in
  PATH="${3:+$3:}$STUB:$PATH" STUB_SAVE="$2" sh "$SCAN" stub:test \
    > "$WORK/out.$1" 2>&1
  echo "$?" > "$WORK/rc.$1"
}
stub_scan h0 "$WORK/h0.save.tar"
stub_scan h1 "$WORK/h1.save.tar"
stub_scan h2 "$WORK/h2.save.tar"
stub_scan i "$WORK/i.save.tar"
stub_scan j "$WORK/j.save.tar"
stub_scan k "$WORK/k.save.tar"
stub_scan l "$WORK/l.save.tar"
stub_scan m "$WORK/m.save.tar"
stub_scan o "$WORK/h0.save.tar" "$SIGSTUB"

fail=0

echo "layer-secret-scan mutation test"
printf '  baked PEM-in-layer fixture -> scan exit %s\n' "$RC_BAKED_LAYER"
printf '  baked key-in-ENV  fixture  -> scan exit %s\n' "$RC_BAKED_ENV"
printf '  clean fixture              -> scan exit %s\n' "$RC_CLEAN"
printf '  false-positive fixture     -> scan exit %s\n' "$RC_FALSEPOS"
for n in $OVERWRITE_NS; do
  printf '  overwrite fixture %s        -> scan exit %s\n' "$n" "$(cat "$WORK/rc.overwrite-$n")"
done
for n in $SANDWICH_NS; do
  printf '  sandwich fixture %s         -> scan exit %s\n' "$n" "$(cat "$WORK/rc.sandwich-$n")"
done
printf '  directory-swap fixture     -> scan exit %s\n' "$RC_DIRSWAP"

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

# --- E/F/G (#2256): a key in an earlier layer is caught whatever a later layer
# did to its path. Each variant must exit 1 (a hit — not 2, could-not-scan) AND
# name /app/key.pem, so a RED for some other reason does not count.
layered_red() { # <label> <rc> <output-file>
  if [ "$2" -eq 1 ] && grep -q 'layer file: /app/key\.pem ' "$3"; then
    echo "RED on $1 — an earlier layer's /app/key.pem was caught"
  else
    echo "FAIL: $1 — the key an earlier layer wrote at /app/key.pem was NOT caught (scan exit $2)" >&2
    cat "$3" >&2
    fail=1
  fi
}
for n in $OVERWRITE_NS; do
  layered_red "overwrite fixture $n" "$(cat "$WORK/rc.overwrite-$n")" "$WORK/out.overwrite-$n"
done
for n in $SANDWICH_NS; do
  layered_red "sandwich fixture $n" "$(cat "$WORK/rc.sandwich-$n")" "$WORK/out.sandwich-$n"
done
layered_red "directory-swap fixture" "$RC_DIRSWAP" "$WORK/out.dirswap"
if grep -q 'layer file: /app/gone\.pem ' "$WORK/out.dirswap"; then
  echo "RED on directory-swap fixture's whited-out /app/gone.pem"
else
  echo "FAIL: directory-swap fixture — the key at /app/gone.pem, deleted by a later layer's whiteout, was NOT caught" >&2
  cat "$WORK/out.dirswap" >&2
  fail=1
fi

# --- H-O: fail-closed. The control must be clean (exit 0), or every exit 2
# below could be the stand-in's fault rather than the scan's.
printf '  stand-in clean control     -> scan exit %s\n' "$(cat "$WORK/rc.h0")"
if [ "$(cat "$WORK/rc.h0")" -eq 0 ]; then
  echo "GREEN on stand-in clean control — the stand-in alone reads clean"
else
  echo "FAIL: stand-in clean control — expected exit 0 (scan exit $(cat "$WORK/rc.h0"))" >&2
  cat "$WORK/out.h0" >&2
  fail=1
fi
closed() { # <fixture> <label>: exit 2 and no "clean" line
  _rc=$(cat "$WORK/rc.$1")
  printf '  %-26s -> scan exit %s\n' "$2" "$_rc"
  if [ "$_rc" -eq 2 ] && ! grep -q '^clean:' "$WORK/out.$1"; then
    echo "CLOSED on $2 — exit 2, not reported clean"
  else
    echo "FAIL: $2 — the scan did not fail closed (scan exit $_rc)" >&2
    cat "$WORK/out.$1" >&2
    fail=1
  fi
}
closed h1 "bad save fixture"
closed h2 "partial save fixture"
closed i "refused member fixture"
closed j "no layer fixture"
closed k "truncated gzip fixture"
closed l "zstd blob fixture"
closed m "newline name fixture"
closed o "signal fixture"

# N: both unreadable keys are read and named — detection, not just exit 2.
printf '  unreadable fixture         -> scan exit %s\n' "$RC_UNREAD"
if [ "$RC_UNREAD" -eq 1 ] \
   && grep -q 'layer file: /app/key\.pem ' "$WORK/out.unread" \
   && grep -q 'layer file: /app/locked/key\.pem ' "$WORK/out.unread"; then
  echo "RED on unreadable fixture — both keys read and named"
else
  echo "FAIL: unreadable fixture — a key in a mode-000 file or untraversable directory was NOT caught (scan exit $RC_UNREAD)" >&2
  cat "$WORK/out.unread" >&2
  fail=1
fi

exit "$fail"
