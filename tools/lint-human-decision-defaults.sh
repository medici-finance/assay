#!/usr/bin/env bash
# tools/lint-human-decision-defaults.sh — repo-wide guard for the mandatory-default rule
# on `gate: human` briefs. A transparent adapter: it runs
#
#   deskchangelog lint-decision-defaults -- <ROOT>
#
# where ROOT is the first argument, defaulting (when absent or empty) to the directory
# this script's parent lives in — the repo root. Further arguments are ignored, as they
# always were. Exit codes: 0 clean · 1 at least one brief whose `## Human decision`
# section has no parseable default (each offender printed to stdout) · 2 refused:
# ROOT/docs/streams does not exist · 127 when no deskchangelog can be found or built.
#
# The lint — the section extraction, the spec-trigger placeholder exemption and the two
# default grammars — lives in tools/desk/cmd/deskchangelog (desktools-v2/52);
# `deskchangelog lint-decision-defaults --help` documents it.
#
# Which deskchangelog runs: the one on PATH (desk-tools release archive), else one built
# from this checkout's tools/desk into a temporary directory removed afterwards.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="${1:-$(cd "$here/.." && pwd)}"
desk_dir="$here/desk"
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
"$bin" lint-decision-defaults -- "$root" || rc=$?
exit "$rc"
