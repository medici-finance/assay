#!/usr/bin/env bash
# Shared PR-CI and mutation invocation; dependency caches are warmed by build/vet.
set -euo pipefail

module_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$module_root"
export GOWORK=off GOPROXY=off GOSUMDB=off KUBECONFIG=/dev/null
# TestMain isolates HOME before compiling the cross-module child. Keep the
# preceding build/vet sweep's caches explicit across that fixture boundary.
GOMODCACHE="$(go env GOMODCACHE)"
GOCACHE="$(go env GOCACHE)"
export GOMODCACHE GOCACHE

go test . -run '^(TestClaimBridgeFlow|TestDeskClaimsNoFallback|TestRoleClaimsPartial)$' -count=1 -timeout=60s -v
