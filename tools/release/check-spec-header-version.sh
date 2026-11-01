#!/usr/bin/env bash
# tools/release/check-spec-header-version.sh — refuse to cut a release whose
# spec/brief-v1.md header names a stale statusgen version. A transparent adapter: it runs
#
#   deskchangelog check-spec-header -- <vX.Y.Z> [spec-file]
#
# (every argument an operand) and exits with its status: 0 match · 1 mismatch (the
# release-blocking case) · 2 usage, a malformed tag, or could-not-check (an unreadable
# file, a missing header line, an unparseable version — never a quiet pass). 127 when
# no deskchangelog can be found or built.
#
# spec-file defaults to spec/brief-v1.md, resolved against the CURRENT working
# directory. The check itself lives in tools/desk/cmd/deskchangelog (desktools-v2/52);
# tools/release/stamp-spec-header-version.sh writes the line this proves.
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
rc=0
"$bin" check-spec-header -- "$@" || rc=$?
exit "$rc"
