---
brief: assay:assay:cellctl-windows:07
title: Native Windows acceptance, packaged runtime proof and support documentation
why: >-
  Cross-compilation and Unix mocks cannot establish native Windows support. Real Windows processes, both cockpit applications and Docker Desktop must complete the lifecycle from the shipped archive before users are told the platform works.
wave: 4
depends: ["cellctl-windows/06"]
unblocks: []
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: >-
  The implementation stages CI execution and a native runtime support claim. A maintainer approves workflow promotion and the real-host evidence; the authoring PR changes neither.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 61523901-12f9-4807-9aa8-83e7ec41347e
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
  - "tools/desk/cmd/cellctl/windows_acceptance_test.go and compiled fixtures: fixed-here"
  - "ci/staged-workflows/cellctl-windows.yml: fixed-here"
  - "docs/cellctl-windows.md, docs/cellctl.md, docs/adopting-assay.md: fixed-here"
  - "scripts/bootstrap-windows.ps1: fixed-here"
---

# Brief 07 — Native Windows acceptance, packaged runtime proof and support documentation

## Context

files:
- tools/desk/cmd/cellctl/windows_acceptance_test.go and compiled fixtures (new)
- ci/staged-workflows/cellctl-windows.yml (new proposal)
- docs/cellctl-windows.md, docs/cellctl.md, docs/adopting-assay.md
- scripts/bootstrap-windows.ps1 (only if packaged helper installation needs adjustment)

facts (freshness checked 2026-10-01):
- The release matrix already builds cellctl.exe for windows/amd64 and windows/arm64. This brief proves executable behavior and helper packaging; it does not duplicate the release matrix.
- windows-port/14 covers desk-role pollers/hooks rather than cellctl. Existing Windows CI does not prove this launcher.
- Hosted Windows unit tests are distinct from an attended Windows machine with real Orca/Herdr and Docker Desktop Linux containers; do not treat Linux CI Docker success as Windows integration evidence.

single-point-of-failure: persisted launch metadata and adapter claims can be wrong.
Independent layers are actual OS/engine identity verification before mutation and native
end-to-end witnesses that exercise the boundary through compiled child processes.
Neither mocks nor a parsed record alone authorizes a destructive lifecycle action.

Read first: [stream plan](README.md), `docs/brief-rules.md`, and the source files above.

## Human decision

<!-- decision-trigger: spec. The executor records concrete platform custody/ownership
alternatives (or workflow-promotion/evidence scope for 07) after design, in a self-contained
decision section, and files it through the normal decision tool. The approved design record
must be linked before this brief enters in-progress. No implementation weakens existing
security controls while waiting for a decision. -->

## Ground rules

- Work in an isolated branch; publish a draft PR under the assigned role. No merge,
  ready flip, workflow dispatch, release cut or production operation is authorized here.
- Preserve security controls and Unix behavior; a missing Windows prerequisite is
  could-not-check, never a skip relabelled as success. Use synthetic credentials and local fixtures.
- Stop at implemented; an independent verifier supplies the execution witness.
- Report NEEDS_CONTEXT for contradictions with current source rather than invent an API.

## Task

1. Add a native Windows unit/integration job using PowerShell only as the CI driver and compiled Go fixtures as children. Assert Bash/sh/tmux/WSL/Python cannot be invoked; record attempted subprocess launches so a hidden shell dependency fails. Allow only the declared bounded native-shell cockpit boundary when that backend requires it, with an exact invocation-shape assertion. Unit tests use no real credentials or live cluster/forge endpoints.
2. Add explicitly opted-in real acceptance tests on a disposable native Windows machine with installed Orca, Herdr and Docker Desktop Linux engine. Test each cockpit separately with both a compiled native Windows host child and a synthetic local Linux image with named volume: new/check/up/reconnect/status/down, model/harness override, console/app restart, concurrent up, partial failures, unrelated terminal preservation and retained volume. For host roles, prove exact argv/model, process-tree cleanup and preservation of unrelated processes. For Docker roles, include incoming/config paths with spaces and Unicode and a synthetic owner-only host credential; the fixed non-root Linux UID must read its read-only bind, and its marker must never appear in argv, logs or serialized state.
3. Capture exact source SHA, OS/architecture, cockpit/Docker versions, backend capability outputs, command exits and container/process identities as sanitized evidence. Missing machine/app/daemon is could-not-check and blocks the affected runtime support claim; never convert a skip or emulator/cross-compile into a Windows pass.
4. Stage CI workflow changes for the normal maintainer promotion path. Require native amd64 evidence; label arm64 build-only until an independent native arm64 runtime witness passes. Do not trigger workflows, cut releases or change machine credentials as part of authoring.
5. Test a checksum-verified release candidate archive containing every required helper, then independently repeat against an identified published release. Require explicit archive path, checksum-file path, expected version and source SHA as inputs; validate them before extraction/execution and record the original artifact URL/tag in evidence. No source-tree-only binaries may satisfy packaging. Candidate evidence can support implemented status, but the released-support claim remains unverified until the published-artifact witness lands through the normal release process. Update prerequisites and limitations with evidence for each supported backend/architecture, and remove native Windows Bash/tmux instructions only for paths actually proven.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsNoShellDependency$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsNoShellDependency ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; on native Windows: no forbidden host subprocess launch is attempted; declared bounded native-shell invocation matches the contract; compiled fixtures cover full native path | check +flow +dereference | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsAcceptanceOrca$' -count=1 -v -args -cellctl-acceptance=orca *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsAcceptanceOrca ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; on explicitly provisioned Windows integration host: actual Orca with native host child and Docker lifecycle passes, including private read-only credential binds; absent prerequisites fail, not skip | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsAcceptanceHerdr$' -count=1 -v -args -cellctl-acceptance=herdr *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsAcceptanceHerdr ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; same host and Docker evidence independently for actual Herdr; volume and immutable ID survive console loss | check +flow | pwsh |
| 4 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsPackagedCellctl$' -count=1 -v -args -cellctl-artifact=$env:CELLCTL_TEST_ARCHIVE -cellctl-checksums=$env:CELLCTL_TEST_CHECKSUMS -cellctl-version=$env:CELLCTL_TEST_VERSION -cellctl-source-sha=$env:CELLCTL_TEST_SOURCE_SHA *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsPackagedCellctl ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; all four inputs required; checksum-verified archive runs outside source tree; cellctl/helper versions and SHA agree; record separate candidate and published-artifact witnesses | check +flow | pwsh |
| 5 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 6 | `statusgen --root ../.. --consumers cellctl-windows/07` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- PATH filtering misses an absolute shell invocation: row 1 checks subprocess attempts, not PATH alone.
- Mocked cockpit or Linux daemon run is presented as native Windows proof: rows 2 and 3 record OS and real backend versions.
- Source tree contains a helper missing from the shipped archive: row 4.
- Desktop integration cannot run on hosted CI: separate explicitly provisioned host evidence; unavailable is could-not-check.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **human**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
