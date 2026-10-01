---
brief: assay:assay:cellctl-windows:03
title: Windows Orca and Herdr console adapters
why: >-
  The supported Windows cockpits must open and reconnect cells without tmux or injected shell loops. Explicit adapters also allow teardown to close the cell terminal while preserving unrelated terminals in the same workspace.
wave: 1
depends: ["cellctl-windows/00"]
unblocks: ["cellctl-windows/06"]
effort: L
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 91e2547a-3982-435b-bbd0-6631dd1dbf71
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
  - "tools/desk/internal/cellconsole/: fixed-here"
  - "tools/desk/internal/celllaunch/: fixed-here"
  - "docs/cellctl-windows.md: fixed-here"
  - "cellctl-windows/06: follow-up cellctl-windows/06"
---

# Brief 03 — Windows Orca and Herdr console adapters

## Context

files:
- tools/desk/internal/cellconsole/ (new Orca and Herdr adapters)
- tools/desk/internal/celllaunch/ (consume contract only)
- docs/cellctl-windows.md (backend capability evidence)

facts (freshness checked 2026-10-01):
- up.go invokes Herdr pane run with one command string and Orca terminal creation with a command flag; these contracts are currently inferred from help.
- Orca teardown currently uses an all-terminals worktree operation. The adapter must return and later validate a specific owned handle.
- Actual Windows availability and API shapes are not yet verified. Both backends are acceptance targets; an absent build is a named blocker, not permission to substitute WSL.

single-point-of-failure: persisted launch metadata and adapter claims can be wrong.
Independent layers are actual OS/engine identity verification before mutation and native
end-to-end witnesses that exercise the boundary through compiled child processes.
Neither mocks nor a parsed record alone authorizes a destructive lifecycle action.

Read first: [stream plan](README.md), `docs/brief-rules.md`, and the source files above.

L is retained because the two adapters must satisfy and be compared against the same external command contract; each adapter remains an independent package file, while their common capability tests are reviewed together.

## Ground rules

- Work in an isolated branch; publish a draft PR under the assigned role. No merge,
  ready flip, workflow dispatch, release cut or production operation is authorized here.
- Preserve security controls and Unix behavior; a missing Windows prerequisite is
  could-not-check, never a skip relabelled as success. Use synthetic credentials and local fixtures.
- Stop at implemented; an independent verifier supplies the execution witness.
- Report NEEDS_CONTEXT for contradictions with current source rather than invent an API.

## Task

1. Capture the actual supported native Windows CLI/API contract and version for each cockpit, including terminal creation, argv or command-string semantics, returned identity, inspection, reconnect and single-terminal close. Check in sanitized fixtures and a capability manifest; label unobserved behavior could-not-check.
2. Implement Orca and Herdr adapters against LaunchSpec. When constrained to command text, implement the bounded Go-runner invocation defined by 00, with backend/OS-specific encoding and metacharacter tests. No generated script carries lifecycle logic.
3. Persist and verify exact workspace/tab/pane/terminal ownership. Existing unrelated terminals are never selected by worktree-wide close. Capability failure precedes creation; partial failures report and clean only resources created by this launch.
4. Support detached/noninteractive launches, error retention, attach/reconnect and changed cockpit configuration using stored backend identity. A missing GUI or unsupported version reports a precise error; Windows auto selection chooses an available supported cockpit or refuses, never falls back to tmux.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./internal/cellconsole -run '^TestBackendLaunchContract$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestBackendLaunchContract ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; both adapters deliver exact executable/spec reference through compiled CLI fixtures; unsupported capability refuses before mutation | check +flow +dereference | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./internal/cellconsole -run '^TestOwnedConsoleOnly$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestOwnedConsoleOnly ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; status/reconnect/close use the recorded exact terminal; unrelated same-worktree terminal survives | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./internal/cellconsole -run '^TestConsoleRecovery$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestConsoleRecovery ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; backend restart, malformed output, partial creation and changed auto-selection have explicit recoverable outcomes | check +flow | pwsh |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 5 | `statusgen --root ../.. --consumers cellctl-windows/03` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- A command-string adapter executes model/path text: row 1.
- Down closes unrelated terminals: row 2.
- Unit fixtures misrepresent actual Windows APIs: real-host flow in 07; no row; review-only for fixture/API comparison: reviewer compares fixtures to captured vendor output, independently of 07 runtime witnesses.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **model**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
