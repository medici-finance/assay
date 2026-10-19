#!/bin/sh
# Exact-match regression tests. A local docker stand-in serves real layer tars;
# no daemon, downloads, or credentials are needed. Prefixes are split so the
# test source itself contains no complete key-shaped fixture.
set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
SCAN=${LAYER_SECRET_SCAN:-$HERE/layer-secret-scan.sh}
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/layer" "$WORK/archive"
cat > "$WORK/bin/docker" <<'SH'
#!/bin/sh
case "$1" in
  history) printf '%s\n' "${LABEL_HISTORY:-COPY fixture /}" ;;
  inspect) printf '[{"Config":{"Labels":{"fixture":"%s"}}}]\n' "${LABEL_CONFIG:-clean}" ;;
  save)
    while [ $# -gt 0 ]; do
      [ "$1" = -o ] && exec cp "$LABEL_ARCHIVE" "$2"
      shift
    done
    exit 1 ;;
  *) exit 1 ;;
esac
SH
chmod 755 "$WORK/bin/docker"
export LABEL_ARCHIVE="$WORK/image.tar"
export PATH="$WORK/bin:$PATH"
FLIP="sk-"decidedhasPreviousPageGITLAB
PR="sk-"decidedunverifiablecannot
PR_NEW="sk-"decidedunverifiablecheck
DIGEST="sk-"decidedunclassifiedunverifiable
KEY="sk-"FAKE000000000000000000000000000TESTONLY
GHKEY="ghp_"FAKE000000000000000000000000000TESTONLY
fail=0

reset() {
  rm -rf "$WORK/layer"
  mkdir -p "$WORK/layer/usr/local/bin"
  LABEL_HISTORY=clean LABEL_CONFIG=clean
  export LABEL_HISTORY LABEL_CONFIG
}
blob() { # <relative image path> <string>...
  dest="$WORK/layer/$1"; shift
  mkdir -p "$(dirname "$dest")"
  printf 'ELF-fixture\000' > "$dest"
  printf '%s\000' "$@" >> "$dest"
}
check() { # <name> <expected exit> [required output]
  COPYFILE_DISABLE=1 tar -cf "$WORK/archive/layer.tar" -C "$WORK/layer" .
  printf '[{"Config":"config.json","Layers":["layer.tar"]}]\n' > "$WORK/archive/manifest.json"
  printf '{}\n' > "$WORK/archive/config.json"
  COPYFILE_DISABLE=1 tar -cf "$LABEL_ARCHIVE" -C "$WORK/archive" .
  rc=0
  sh "$SCAN" labels-fixture > "$WORK/out" 2>&1 || rc=$?
  if [ "$rc" -eq "$2" ] && { [ $# -lt 3 ] || grep -qF "$3" "$WORK/out"; }; then
    echo "PASS labels: $1"
  else
    echo "FAIL labels: $1 (expected $2, got $rc)" >&2
    cat "$WORK/out" >&2
    fail=1
  fi
}

# regression: #3203 — exact pairs are a bounded correction, not a path exemption.
for pair in flip pr-old pr-new digest; do
  case "$pair" in
    flip) path=usr/local/bin/deskflip; value=$FLIP ;;
    pr-old) path=usr/local/bin/deskpr; value=$PR ;;
    pr-new) path=usr/local/bin/deskpr; value=$PR_NEW ;;
    digest) path=usr/local/bin/deskdigest; value=$DIGEST ;;
  esac
  reset
  blob "$path" "$value"
  check "$pair exact path/string pair" 0 'clean:'

  reset
  blob "$path" "$value" "$KEY"
  check "$pair generic key after label" 1 "/$path"

  reset
  blob "$path" "$KEY" "$value"
  check "$pair generic key before label" 1 "/$path"

  reset
  blob "$path" "$value" "$GHKEY"
  check "$pair strict token after label" 1 "/$path"

  reset
  blob "$path" "$GHKEY" "$value"
  check "$pair strict token before label" 1 "/$path"

  reset
  blob "$path" "$value" "BEGIN TESTING FAKE PRIVATE"' KEY'
  check "$pair PEM marker after label" 1 "/$path"

  reset
  blob "$path" "BEGIN TESTING FAKE PRIVATE"' KEY' "$value"
  check "$pair PEM marker before label" 1 "/$path"

  reset
  blob "$path" "${value}EXTRA"
  check "$pair extended sequence" 1 "/$path"

  reset
  blob "$path" "${value%?}Z"
  check "$pair changed sequence" 1 "/$path"

  reset
  blob opt/app/other "$value"
  check "$pair unrelated path" 1 '/opt/app/other'

  reset
  wrong=usr/local/bin/deskpr
  [ "$path" != "$wrong" ] || wrong=usr/local/bin/deskflip
  blob "$wrong" "$value"
  check "$pair wrong desk path" 1 "/$wrong"

  reset
  blob marker clean
  LABEL_HISTORY=$value; export LABEL_HISTORY
  check "$pair build history" 1 'build history'

  reset
  blob marker clean
  LABEL_CONFIG=$value; export LABEL_CONFIG
  check "$pair image config" 1 'image config'
done

# A benign match in one file must not finish the whole scan either.
reset
blob usr/local/bin/deskpr "$PR_NEW"
blob opt/app/other "$KEY"
check 'separate file after benign match' 1 '/opt/app/other'

exit "$fail"
