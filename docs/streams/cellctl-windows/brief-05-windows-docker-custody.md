---
brief: assay:assay:cellctl-windows:05
title: Windows Docker bind paths and credential custody
why: >-
  Windows file modes and Docker Desktop mount paths differ from Unix. Simply accepting drive-letter paths would bypass credential privacy checks or reject safe reconnects; custody and mount identity must be checked with Windows semantics.
wave: 2
depends: ["cellctl-windows/04"]
unblocks: ["cellctl-windows/06"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This changes credential custody or process/isolation enforcement across platform boundaries. The design review must confirm that existing authority and ownership checks remain enforced; no new grants or security bypasses are authorized.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 110cd999-8296-424a-93c8-c907430a4efb
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
  - "tools/desk/internal/cellcontainer/config.go, engine.go: fixed-here"
  - "tools/desk/internal/cellcontainer/paths_windows.go, paths_unix.go: fixed-here"
  - "tools/desk/internal/deskkit/custodyowner_windows.go, custodyowner_unix.go: fixed-here"
  - "cellctl-windows/06: follow-up cellctl-windows/06"
---

# Brief 05 — Windows Docker bind paths and credential custody

## Context

files:
- tools/desk/internal/cellcontainer/config.go, engine.go (path, custody and mount identity portions)
- tools/desk/internal/cellcontainer/paths_windows.go, paths_unix.go (new)
- tools/desk/internal/deskkit/custodyowner_windows.go, custodyowner_unix.go (reuse)

facts (freshness checked 2026-10-01):
- The merged container runtime checks private file permission bits and blocks directory binds containing configured credentials, daemon sockets, the home directory and cell metadata.
- deskkit already provides VerifyCustodyOwnerOnly and LstatCustody for Windows owner/DACL validation; use that policy rather than interpreting mode 0600 on NTFS.
- Docker Desktop may report Linux-VM source paths for Windows host binds. Case folding, drive roots, junctions/reparse points and aliases need identity-aware handling, not removal of mount checks.

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

1. Use existing custody primitives for credential files and private state; reject broad DACL grants and unexpected owners. Do not auto-loosen permissions or copy secrets into more permissive locations. Preserve read-only secret mounts and the existing no-token-in-argv behavior.
2. Canonicalize Windows bind sources safely across drive roots, UNC forms, case, junctions and reparse points. Refuse unsupported/unverifiable path classes explicitly. Keep root/home/cell-directory, every role credential and daemon endpoint excluded from broad mounts; repeat identity checks at launch.
3. Define a conservative mapping between configured host sources and Docker inspect mount sources using captured Windows Docker Desktop evidence. Validate destination, type, source identity and writable bit; ambiguous translation refuses attach/stop rather than ignoring source checks.
4. Keep Linux container UID/GID, capability drop, read-only root, resource limits, network isolation settings and ownership/adoption checks unchanged. Re-run the existing negative cases (TestReconnectRefusesChangedRuntime, TestConfigValidateRefusals, TestRunArgsAreExact and containment tests) and add Windows negative fixtures. Record separate source-guard-removal mutation evidence for ACL, containment and source-identity guards; negative input tables alone are not that evidence.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestWindowsCredentialCustody$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsCredentialCustody ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; private owner-only credential passes; broad ACL, wrong owner and reparse substitutions refuse | check +flow | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestWindowsBindContainment$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsBindContainment ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; drive/UNC roots, case aliases, junctions and other-role credentials cannot bypass containment; safe sibling passes | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./internal/cellcontainer -run '^TestWindowsDockerMountIdentity$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsDockerMountIdentity ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; observed Docker path translation reconnects; changed source/type/writable mode and ambiguity refuse | check +flow | pwsh |
| 4 | `go test ./internal/cellcontainer -count=1 -v` | exit 0; existing isolation, adoption and lifecycle tests pass without removing or skipping assertions | check +flow | pwsh |
| 5 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 6 | `statusgen --root ../.. --consumers cellctl-windows/05` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- POSIX permission bits accept a world-readable Windows credential: row 1.
- Junction/case alias exposes credentials through a directory mount: row 2.
- Path normalization makes an unrelated mount look owned: row 3.
- Porting drops Linux hardening flags: row 4 plus separately recorded source-guard-removal evidence.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **human**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
