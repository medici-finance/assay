#!/usr/bin/env bash
# Provide the baseline statusgen binary that the witness skew tests
# (witnessv2_skew_test.go, brief statusgen/36 Verify rows 5 and 6) compare
# against, and export it as SG_OLD_BIN. Two modes, one baseline (v1.0.52):
#
#   bash statusgen/fetch-baseline.sh [asset] [dest-dir]
#     Download the release asset (default statusgen-linux-amd64) and verify it
#     against that release's own checksums.txt. The expected digest is read at
#     run time, never typed in here: a missing entry, a duplicate entry or a
#     mismatch exits non-zero before the binary is marked executable.
#
#   bash statusgen/fetch-baseline.sh --from-source <commit-sha> [dest-dir]
#     Build the baseline from source at the tag's commit, for runners that
#     cannot reach GitHub's release-asset CDN (git over github.com and the Go
#     module proxy are enough). <commit-sha> must be the full 40-hex commit
#     v1.0.52 points at, equal to tag_commit below; any other value exits 2.
#     The commit is fetched from origin only when absent locally (a full clone
#     is never made shallow), and must resolve to exactly that commit.
#
# dest-dir defaults to $RUNNER_TEMP, else a new temp dir. Under GitHub Actions
# ($GITHUB_ENV set) SG_OLD_BIN=<path> is appended there for every later step of
# the job. The line is always printed on stdout.
set -euo pipefail

readonly repo="medici-finance/assay"
readonly tag="v1.0.52"
# The commit v1.0.52 points at (git ls-remote ... 'refs/tags/v1.0.52^{}').
readonly tag_commit="2718fc978b7454f0f2aa422e1236fb6ad543e0d2"

export_bin() {
  echo "SG_OLD_BIN=$1"
  if [ -n "${GITHUB_ENV:-}" ]; then
    echo "SG_OLD_BIN=$1" >> "$GITHUB_ENV"
  fi
}

default_dest() {
  local d="${1:-${RUNNER_TEMP:-}}"
  if [ -z "$d" ]; then d="$(mktemp -d)"; fi
  mkdir -p "$d"
  printf '%s\n' "$d"
}

if [ "${1:-}" = "--from-source" ]; then
  sha="${2:-}"
  if ! printf '%s' "$sha" | grep -Eq '^[0-9a-f]{40}$'; then
    echo "fetch-baseline: --from-source needs a full 40-hex commit sha, got '${sha}'" >&2
    exit 2
  fi
  if [ "$sha" != "$tag_commit" ]; then
    echo "fetch-baseline: --from-source ${sha} is not ${tag} (${tag_commit})" >&2
    exit 2
  fi
  dest="$(default_dest "${3:-}")"
  top="$(git -C "$(dirname -- "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
  if ! git -C "$top" cat-file -e "${sha}^{commit}" 2>/dev/null; then
    git -C "$top" fetch --quiet --no-tags --depth 1 origin "$sha"
  fi
  got="$(git -C "$top" rev-parse --verify "${sha}^{commit}")"
  if [ "$got" != "$sha" ]; then
    echo "fetch-baseline: ${sha} resolved to ${got}" >&2
    exit 1
  fi
  src="${dest}/statusgen-baseline-${tag}-src"
  bin="${dest}/statusgen-baseline-${tag}"
  rm -rf -- "$src"
  mkdir -p "$src"
  git -C "$top" archive "$sha" | tar -x -C "$src"
  (cd "$src/statusgen" && GOWORK=off go build -o "$bin" .)
  rm -rf -- "$src"
  echo "fetch-baseline: built statusgen ${tag} from source at ${sha}" >&2
  export_bin "$bin"
  exit 0
fi

asset="${1:-statusgen-linux-amd64}"
case "$asset" in
  statusgen-*) ;;
  *) echo "fetch-baseline: asset '$asset' is not a statusgen release binary" >&2; exit 2 ;;
esac
dest="$(default_dest "${2:-}")"

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
export_bin "$bin"
