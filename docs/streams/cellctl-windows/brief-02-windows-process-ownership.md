---
brief: assay:assay:cellctl-windows:02
title: Native process ownership and lifetime on Windows
why: >-
  The existing kill and pkill paths cannot provide safe Windows teardown. A durable identity and owned process tree are needed so restarting a cockpit never kills another cell or leaves child processes behind.
wave: 1
depends: ["cellctl-windows/00"]
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
id: 7a565bb1-bc1e-4838-a93d-c1066486b676
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
  - "tools/desk/internal/cellprocess/: fixed-here"
  - "tools/desk/internal/deskkit/filelock_windows.go, filelock_unix.go: fixed-here"
  - "tools/desk/internal/celllaunch/: fixed-here"
  - "cellctl-windows/06: follow-up cellctl-windows/06"
---

# Brief 02 — Native process ownership and lifetime on Windows

## Context

files:
- tools/desk/internal/cellprocess/ (new platform implementations)
- tools/desk/internal/deskkit/filelock_windows.go, filelock_unix.go (reuse; changes only when required)
- tools/desk/internal/celllaunch/ (consume contract only)

facts (freshness checked 2026-10-01):
- cmd/cellctl/status.go invokes kill -0; down.go invokes pkill -f for deskd. These remain at 1c67aa717.
- deskkit has Windows LockFileEx and owner/DACL checks already. hookprocess_windows.go only kills the immediate process, not a complete job tree.
- Process identity requires more than a PID because the OS can reuse it; a state file is not authorization to terminate.

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

1. Implement platform-specific start, inspect, interrupt, wait and terminate under one Go interface. On Windows use process handles/creation identity and an owned job-object strategy, including nested-job and detached-console failure behavior; never terminate by process-name substring.
2. Implement atomic private session records and lock/recovery semantics using existing deskkit locks. Validate cell/role, executable, creation identity and containment before destructive actions; unknown state reports could-not-check and stops nothing.
3. Define long-lived ownership separately from cockpit lifetime. Reconnect preserves a running owned process; partial start cleans only resources created by that attempt. Shutdown first requests graceful exit, then bounds forced termination to the owned tree.
4. Prove concurrent starts, stale locks, reused PID, permissions denied, parent death and descendant cleanup with real compiled children on Windows. Keep Unix behavior exercised by the same interface tests.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./internal/cellprocess -run '^TestOwnedProcessTree$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestOwnedProcessTree ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; Ctrl+C/graceful stop and bounded termination affect only the owned parent/descendants; nested-job and detached-console cases pass or refuse before spawn without leaks | check +flow | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./internal/cellprocess -run '^TestProcessIdentityRefusal$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestProcessIdentityRefusal ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; reused PID, foreign process, corrupt record and access denial stop nothing | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./internal/cellprocess -run '^TestConcurrentStartAndRecovery$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestConcurrentStartAndRecovery ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; one owner under contention; cockpit death/restart recovers; failed start leaves no orphan | check +flow | pwsh |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 5 | `statusgen --root ../.. --consumers cellctl-windows/02` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- PID reuse terminates an unrelated app: row 2.
- A child survives shutdown: row 1 using real descendant processes.
- Two consoles claim one cell or a crashed parent loses ownership: row 3.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **human**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
