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
# Which deskchangelog runs: the one on PATH (desk-tools release archive), else one built
# from this checkout's tools/desk into a temporary directory removed afterwards.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
desk_dir="$(cd "$here/.." && pwd)/desk"
if command -v deskchangelog >/dev/null 2>&1; then
  bin=deskchangelog
else
  if ! command -v go >/dev/null 2>&1 || [ ! -d "$desk_dir/cmd/deskchangelog" ]; then
    echo "error: deskchangelog is not on PATH, and it cannot be built here (needs go and ${desk_dir}/cmd/deskchangelog)." >&2
    echo "       Install the desk-tools release, or run from an assay checkout with Go installed." >&2
    exit 127
  fi
  build_dir="$(mktemp -d "${TMPDIR:-/tmp}/deskchangelog.XXXXXX")"
  trap 'rm -rf "$build_dir"' EXIT
  if ! go -C "$desk_dir" build -buildvcs=false -o "$build_dir/deskchangelog" ./cmd/deskchangelog; then
    echo "error: building deskchangelog from ${desk_dir} failed; nothing was checked" >&2
    exit 127
  fi
  bin="$build_dir/deskchangelog"
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
