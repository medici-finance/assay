#!/usr/bin/env bash
# Shared PR-CI and mutation invocation; dependency caches are warmed by build/vet.
set -euo pipefail

module_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$module_root"
export GOWORK=off GOPROXY=off GOSUMDB=off KUBECONFIG=/dev/null

go test . -run '^(TestClaimBridgeFlow|TestDeskClaimsNoFallback|TestRoleClaimsPartial)$' -count=1 -timeout=60s -v
