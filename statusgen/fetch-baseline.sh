#!/usr/bin/env bash
# Fetch the baseline statusgen release binary that the witness skew tests
# (witnessv2_skew_test.go, brief statusgen/36 Verify rows 5 and 6) compare
# against, verify it against that release's own checksums.txt, and export it
# as SG_OLD_BIN.
#
# Usage: bash statusgen/fetch-baseline.sh [asset] [dest-dir]
#   asset     release asset name (default statusgen-linux-amd64)
#   dest-dir  where the binary lands (default $RUNNER_TEMP, else a new temp dir)
#
# Under GitHub Actions ($GITHUB_ENV set) it appends SG_OLD_BIN=<path> there for
# every later step of the job. It always prints that line on stdout.
#
# The baseline is the tag Verify rows 5 and 6 name. The expected digest is read
# from the release's checksums.txt at run time, never typed in here: a missing
# entry, a duplicate entry or a mismatch exits non-zero before the binary is
# marked executable.
set -euo pipefail

readonly repo="medici-finance/assay"
readonly tag="v1.0.52"
asset="${1:-statusgen-linux-amd64}"
dest="${2:-${RUNNER_TEMP:-}}"
if [ -z "$dest" ]; then dest="$(mktemp -d)"; fi
mkdir -p "$dest"

case "$asset" in
  statusgen-*) ;;
  *) echo "fetch-baseline: asset '$asset' is not a statusgen release binary" >&2; exit 2 ;;
esac

base="https://github.com/${repo}/releases/download/${tag}"
bin="${dest}/statusgen-baseline-${tag}"
sums="${dest}/statusgen-baseline-${tag}.checksums.txt"

curl -fsSL --retry 3 --proto '=https' "${base}/checksums.txt" -o "$sums"
curl -fsSL --retry 3 --proto '=https' "${base}/${asset}" -o "$bin"

# checksums.txt is `<sha256>  <name>` (sha256sum format; `*<name>` in binary
# mode). Exactly one line must name the asset.
want="$(awk -v a="$asset" '$2 == a || $2 == "*" a { print $1 }' "$sums")"
if [ -z "$want" ] || [ "$(printf '%s\n' "$want" | wc -l | tr -d ' ')" != "1" ]; then
  echo "fetch-baseline: ${tag} checksums.txt has no single entry for ${asset}" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  got="$(sha256sum "$bin" | cut -d' ' -f1)"
else
  got="$(shasum -a 256 "$bin" | cut -d' ' -f1)"
fi
if [ "$want" != "$got" ]; then
  echo "fetch-baseline: ${asset} ${tag} sha256 mismatch: checksums.txt ${want}, downloaded ${got}" >&2
  rm -f -- "$bin"
  exit 1
fi
chmod +x "$bin"
echo "fetch-baseline: ${asset} ${tag} sha256 ${got} matches checksums.txt" >&2

echo "SG_OLD_BIN=${bin}"
if [ -n "${GITHUB_ENV:-}" ]; then
  echo "SG_OLD_BIN=${bin}" >> "$GITHUB_ENV"
fi
