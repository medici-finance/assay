#!/usr/bin/env bash
# tools/changelog/credit-identity.sh — is this pull-request author somebody the release
# notes should thank? A transparent adapter: it runs
#
#   deskchangelog credit-identity classify -- <login> [<body-file>]
#
# (any other first argument: `deskchangelog credit-identity -- <args>`, the usage line and
# `skip:unknown`), so every argument stays an operand, and exits with its status: 0
# always, as before — one word on stdout: credit, skip:roster, skip:bot, skip:opt-out or
# skip:unknown. 127 when no deskchangelog can be found or built (callers already map a
# failure to skip:unknown).
#
# The classifier — the roster it reads (ASSAY_TRUSTED_LOGINS, ASSAY_TRUSTED_BOT_SLUGS,
# ASSAY_HUMAN_LOGIN_MAP), the bot shapes and the `<!-- changelog-credit: no -->` opt-out
# marker — lives in tools/desk/cmd/deskchangelog (desktools-v2/52);
# `deskchangelog credit-identity classify --help` documents it.
#
# Which deskchangelog runs: the tree under check decides. When this checkout carries
# tools/desk/cmd/deskchangelog, the adapter builds it with `go build` into a temporary
# directory removed afterwards and runs that build; a deskchangelog on PATH is never used
# then, and a missing go or a failed build exits 127. Only a tree without the source (an
# installed desk-tools bundle) runs the deskchangelog on PATH.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
desk_dir="$(cd "$here/.." && pwd)/desk"
if [ -d "$desk_dir/cmd/deskchangelog" ]; then
  # This tree carries the source, so its own build decides; PATH is never consulted.
  if ! command -v go >/dev/null 2>&1; then
    echo "error: this tree carries ${desk_dir}/cmd/deskchangelog, but go is not installed, so it cannot be built; nothing was checked." >&2
    echo "       A deskchangelog on PATH is not used for a tree that has the source." >&2
    exit 127
  fi
  build_dir="$(mktemp -d "${TMPDIR:-/tmp}/deskchangelog.XXXXXX")"
  trap 'rm -rf "$build_dir"' EXIT
  if ! go -C "$desk_dir" build -buildvcs=false -o "$build_dir/deskchangelog" ./cmd/deskchangelog; then
    echo "error: building deskchangelog from ${desk_dir} failed; nothing was checked." >&2
    echo "       A deskchangelog on PATH is not used for a tree that has the source." >&2
    exit 127
  fi
  bin="$build_dir/deskchangelog"
elif command -v deskchangelog >/dev/null 2>&1; then
  # No source here (an installed desk-tools bundle): the deskchangelog on PATH.
  bin=deskchangelog
else
  echo "error: deskchangelog is not on PATH, and this tree has no ${desk_dir}/cmd/deskchangelog to build it from." >&2
  echo "       Install the desk-tools release, or run from an assay checkout with Go installed." >&2
  exit 127
fi
if [ "${1:-}" = classify ]; then
  shift
  set -- classify -- "$@"
else
  set -- -- "$@"
fi
rc=0
"$bin" credit-identity "$@" || rc=$?
exit "$rc"
