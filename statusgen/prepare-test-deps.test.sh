#!/usr/bin/env bash
# Prepare the cross-module test child's real dependency closure before tests
# disable module lookup. statusgen's own module cannot warm desk's dependencies.
set -euo pipefail
module_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
build_dir="$(mktemp -d)"
trap 'rm -rf -- "$build_dir"' EXIT
cd "$module_root/../tools/desk"
GOWORK=off go test -c -o "$build_dir/deskgit.test" ./cmd/deskgit
