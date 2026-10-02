---
brief: assay:assay:cellctl-windows:04
title: Windows-local Docker endpoint and runtime selection
why: >-
  The Go container engine only accepts Unix sockets, so it cannot reach a native Windows Docker Desktop engine. Endpoint selection must become portable while preserving the local-only boundary and deterministic reconnect behavior.
wave: 1
depends: ["cellctl-windows/00"]
unblocks: ["cellctl-windows/05"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 46e1b86d-f55b-45ef-b189-777aef9f76e1
authored: 2026-10-01 by assay-worker-app (Windows cellctl planning)
sources:
  - "Driver request 2026-10-01: native Windows cellctl; assume Orca, Herdr and Docker; plan and publish briefs before implementation."
  - "freshness-checked 2026-10-01 @ 1c67aa717 (origin/main); source gaps listed in Context."
  - "docs/streams/windows-port/README.md: existing install/release scope explicitly excludes cockpits."
  - "PR #1981 merged at d4a7a053a0acf392e72a9f772573ee48a243be03: native Go container lifecycle baseline."
exec-tier: strong
exec-tier-why: >-
  Correctness depends on cross-component launch, identity and failure semantics; an isolated unit green can hide a broken runtime flow.
domain: complicated
outcome: none
consumers:
  - "tools/desk/internal/cellcontainer/endpoint.go, endpoint_windows.go, endpoint_unix.go: fixed-here"
  - "tools/desk/internal/cellcontainer/config.go, engine.go: fixed-here"
  - "docs/cellctl.md: fixed-here"
  - "cellctl-windows/05: follow-up cellctl-windows/05"
---

# Brief 04 — Windows-local Docker endpoint and runtime selection

## Context

files:
- tools/desk/internal/cellcontainer/endpoint.go, endpoint_windows.go, endpoint_unix.go (new)
- tools/desk/internal/cellcontainer/config.go, engine.go (endpoint and subprocess environment only)
- docs/cellctl.md (endpoint configuration contract)

facts (freshness checked 2026-10-01):
- PR #1981 merged as d4a7a053a0acf392e72a9f772573ee48a243be03; its runtime exists on fresh main 1c67aa717. It is a satisfied source prerequisite, not an open-PR dependency.
- Config.Validate only accepts unix:///; Docker subprocesses use a minimal Unix environment and KUBECONFIG=/dev/null.
- Windows host support targets Docker Desktop running Linux containers. A Linux daemon platform and its UID/GID security settings remain container properties, independent of Windows host identity.

single-point-of-failure: persisted launch metadata and adapter claims can be wrong.
Independent layers are actual OS/engine identity verification before mutation and native
end-to-end witnesses that exercise the boundary through compiled child processes.
Neither mocks nor a parsed record alone authorizes a destructive lifecycle action.

Read first: [stream plan](README.md), `docs/brief-rules.md`, and the source files above.

## Ground rules

- Work in an isolated branch; publish a draft PR under the assigned role. No merge,
  ready flip, workflow dispatch, release cut or production operation is authorized here.
- Preserve security controls and Unix behavior; a missing Windows prerequisite is
  could-not-check, never a skip relabelled as success. Use synthetic credentials and local fixtures.
- Stop at implemented; an independent verifier supplies the execution witness.
- Report NEEDS_CONTEXT for contradictions with current source rather than invent an API.

## Task

1. Add a typed local endpoint selector: existing Unix socket on Unix and local named pipe on Windows. If explicit named contexts are supported, resolve metadata locally, reject remote tcp/ssh endpoints, freeze the resolved local endpoint for the operation and record its identity. Never silently use the ambient current context.
2. Keep Docker invocation as Go exec argv with an explicit endpoint and Windows-safe minimal environment, including necessary OS variables and the inert kubeconfig choice from the contract. DRY_RUN performs no engine contact; print the resolved target before the first real query.
3. Preflight daemon OSType and platform compatibility. Refuse Windows-container daemons for this Linux runtime and report unsupported image/architecture cleanly. Do not add implicit pulls, context switching or engine reconfiguration.
4. Preserve immutable-ID inspection/attach/stop and all existing runtime validation. A change of configured engine cannot redirect lifecycle operations on an old session to a same-named container on another daemon.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestLocalEndpointSelection$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLocalEndpointSelection ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; Unix and Windows local endpoints resolve; remote contexts/endpoints and ambiguous selectors refuse without contact | check +flow +dereference | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestDockerWindowsInvocation$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestDockerWindowsInvocation ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; compiled docker fixture receives exact argv and Windows runtime environment; dry-run contacts nothing | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestDaemonAndEndpointIdentity$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestDaemonAndEndpointIdentity ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; wrong daemon OS and changed endpoint refuse; stopped/running identity semantics remain intact | check +flow | pwsh |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 5 | `statusgen --root ../.. --consumers cellctl-windows/04` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- Ambient context sends operations to a remote engine: row 1.
- Same container name on a different daemon is stopped: row 3.
- Windows-container mode quietly loses Linux isolation controls: row 3.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **model**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
