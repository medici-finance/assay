#!/usr/bin/env bash
# Deterministic, offline check for worker-kit fixture task 06-workpad-upsert.
# Usage: check.sh <run-dir>   (run-dir holds a copy of testdata/start, possibly edited)
set -euo pipefail
dir="${1:?usage: check.sh <run-dir>}"
cd "$dir"
[ -f go.mod ] || go mod init fixture-06-workpad-upsert >/dev/null
GOFLAGS=-mod=mod GOWORK=off go test ./... -count=1
grep -qx 'Status: implemented' STATUS.md
