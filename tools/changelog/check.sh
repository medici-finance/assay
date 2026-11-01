#!/usr/bin/env bash
# tools/changelog/check.sh — the pull-request changelog gate. A transparent adapter: it runs
#
#   deskchangelog check --reconcile tools/changelog/reconcile.py
#
# and exits with its status (0 pass · 1 refused, missing, or BASE_SHA/HEAD_SHA unset;
# 127 when no deskchangelog can be found or built, before any check). Arguments are
# ignored, as they always were.
#
# The gate itself — the retired-`## Unreleased` refusal, the flat-fragment rule, the
# board-reconciliation exception, the changelog:skip label, the fragment-with-a-bullet
# rule and the fork-PR proxy path, with every message and exit code — lives in
# tools/desk/cmd/deskchangelog (desktools-v2/52); `deskchangelog check --help` lists
# its inputs. Inputs are the environment, unchanged: SKIP, BASE_SHA, HEAD_SHA, BASE_REF
# (else GITHUB_BASE_REF), HEAD_REF, PR_NUMBER, CHANGELOG_AGG, and the runner metadata
# reconcile.py reads. Run from the repository checkout.
#
# Which deskchangelog runs: the one on PATH (desk-tools release archive), else one built
# from this checkout's tools/desk with `go build` into a temporary directory that is
# removed afterwards (and put first on PATH, so a CHANGELOG_AGG=aggregate.py run uses
# the same build).
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
  PATH="$build_dir:$PATH"
  export PATH
fi
rc=0
"$bin" check --reconcile "$here/reconcile.py" || rc=$?
exit "$rc"
