#!/usr/bin/env bash
# tools/create-fleet-gitlab.sh — idempotent Assay fleet provisioning for a
# GitLab top-level group. A transparent adapter: it runs
#
#   deskgitlab create "$@"
#
# with every argument passed through unchanged, and exits with deskgitlab's
# exit status (0 ok · 1 a step failed · 2 usage, before any effect).
#
# The provisioner itself — the seven per-role service accounts, their
# memberships, first PATs and avatars, and the fleet project's protected
# `main`, approval settings, protected tags, merge checks and labels — lives in
# tools/desk/cmd/deskgitlab (desktools-v2/30); `deskgitlab create --help` lists
# its options. The owner credential is read ONLY from GITLAB_TOKEN, never argv.
#
# Which deskgitlab runs: the one on PATH (desk-tools release archive), else one
# built from this checkout's tools/desk with `go build` into a temporary
# directory that is removed afterwards.
#
# Adopter walkthrough: docs/adopting-assay-gitlab.md

set -euo pipefail

if command -v deskgitlab >/dev/null 2>&1; then
  exec deskgitlab create "$@"
fi

desk_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/desk"
if ! command -v go >/dev/null 2>&1 || [ ! -d "$desk_dir/cmd/deskgitlab" ]; then
  echo "error: deskgitlab is not on PATH, and it cannot be built here (needs go and ${desk_dir}/cmd/deskgitlab)." >&2
  echo "       Install the desk-tools release, or run from an assay checkout with Go installed." >&2
  exit 127
fi

build_dir="$(mktemp -d "${TMPDIR:-/tmp}/deskgitlab.XXXXXX")"
trap 'rm -rf "$build_dir"' EXIT
if ! go -C "$desk_dir" build -o "$build_dir/deskgitlab" ./cmd/deskgitlab; then
  echo "error: building deskgitlab from ${desk_dir} failed" >&2
  exit 1
fi
rc=0
"$build_dir/deskgitlab" create "$@" || rc=$?
exit "$rc"
