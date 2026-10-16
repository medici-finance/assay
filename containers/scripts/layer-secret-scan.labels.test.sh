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

reset
blob usr/local/bin/deskflip "$FLIP"
blob usr/local/bin/deskpr "$PR"
blob usr/local/bin/deskdigest "$DIGEST"
check 'three exact path/string pairs' 0 'clean:'

reset
blob usr/local/bin/deskpr "$PR" "$KEY"
check 'real generic key after known label in same binary' 1 '/usr/local/bin/deskpr'

reset
blob usr/local/bin/deskpr "$KEY" "$PR"
check 'real generic key before known label in same binary' 1 '/usr/local/bin/deskpr'

reset
blob usr/local/bin/deskflip "$FLIP" "$GHKEY"
check 'strict token after known label in same binary' 1 '/usr/local/bin/deskflip'

reset
blob usr/local/bin/deskdigest "$DIGEST" "BEGIN TESTING FAKE PRIVATE"' KEY'
check 'PEM marker after known label in same binary' 1 '/usr/local/bin/deskdigest'

reset
blob usr/local/bin/deskpr "${PR}EXTRA"
check 'longer match sharing the benign prefix' 1 '/usr/local/bin/deskpr'

reset
blob opt/app/other "$PR"
check 'exact string at an unrelated path' 1 '/opt/app/other'

reset
blob usr/local/bin/deskpr "$FLIP"
check 'exact string at another desk binary path' 1 '/usr/local/bin/deskpr'

reset
blob marker clean
LABEL_HISTORY=$PR; export LABEL_HISTORY
check 'known string in build history' 1 'build history'

reset
blob marker clean
LABEL_CONFIG=$PR; export LABEL_CONFIG
check 'known string in image config' 1 'image config'

exit "$fail"
