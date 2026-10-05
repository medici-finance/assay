---
brief: assay:assay:cellctl-windows:06
title: Integrate shell-free Windows host and Docker lifecycle
why: >-
  Portable helpers do not make cellctl usable until new, check, up, desk, status and down share the same ownership model. This integration removes the remaining tmux and shell orchestration from the Windows paths and proves recovery across console loss.
wave: 3
depends: ["cellctl-windows/01", "cellctl-windows/02", "cellctl-windows/03", "cellctl-windows/05"]
unblocks: ["cellctl-windows/07"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: >-
  This changes credential custody or process/isolation enforcement across platform boundaries. The design review must confirm that existing authority and ownership checks remain enforced; no new grants or security bypasses are authorized.
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: 88edc8f8-c4a3-4712-b0a4-854a038c634e
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
  - "tools/desk/cmd/cellctl/up.go, cockpit.go, launch.go, down.go, status.go: fixed-here"
  - "tools/desk/cmd/cellctl/container_native.go, deskd.go, check.go, usage.go: fixed-here"
  - "tools/desk/cmd/cellctl/*_test.go: fixed-here"
  - "docs/cellctl.md: fixed-here"
  - "cellctl-windows/07: follow-up cellctl-windows/07"
---

# Brief 06 — Integrate shell-free Windows host and Docker lifecycle

## Context

files:
- tools/desk/cmd/cellctl/up.go, cockpit.go, launch.go, down.go, status.go
- tools/desk/cmd/cellctl/container_native.go, deskd.go, check.go, usage.go
- tools/desk/cmd/cellctl/*_test.go (compiled integration fixtures)
- docs/cellctl.md

facts (freshness checked 2026-10-01):
- Native container console lifetime currently belongs to tmux; container-run holds the optional coordinator host_lock while foreground docker attach/run is connected.
- Closing a console can release that advisory lock while the container survives. The Windows implementation must not claim coordinator exclusion from a vanished console process.
- The common contracts and platform/backend implementations are delivered by cellctl-windows/00 through /05; this brief alone owns their CLI wiring to avoid parallel edits to up/down.

single-point-of-failure: persisted launch metadata and adapter claims can be wrong.
Independent layers are actual OS/engine identity verification before mutation and native
end-to-end witnesses that exercise the boundary through compiled child processes.
Neither mocks nor a parsed record alone authorizes a destructive lifecycle action.

Read first: [stream plan](README.md), `docs/brief-rules.md`, and the source files above.

L is retained for the cross-component CLI integration: dividing its ownership state transitions between workers would recreate the split lifetime the brief fixes. All lower-level implementations land separately first.

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

1. Wire new/check/up/desk/status/down through the common Go launch, environment, process and console adapters for house and scrubbed host cells, the local launcher portion of k8s cells, and native Linux-container cells. The k8s fixture is inert and grants no cluster contact. Keep Unix tmux as an explicit Unix backend; Windows must work with Orca or Herdr without tmux/Bash/WSL/Python. A documented bounded native-shell adapter invocation is allowed only to invoke the Go runner, never to implement lifecycle logic or interpolate user data. Preserve old external-launcher registrations as explicitly external compatibility, outside the native-support claim.
2. Move first-window deskd readiness/watch behavior from shell loops into Go with timeouts, cancellation and visible errors. Preserve attended token-minting gates, enabled-role/model policy and container isolation; no new live cluster probes or implicit network checks.
3. Make lifecycle state authoritative only after validating the actual process/terminal/container. Repeated up reconnects; changed model/harness arguments require explicit restart; status/down handle valid prior overrides. Persist exact returned handles and endpoint/immutable container IDs. Aggregate per-role shutdown failures and retain diagnostics.
4. Decouple Docker lifetime from console presentation. Preserve coordinator exclusion across console loss using a validated owner/recovery mechanism; verify both host and container launch paths consult it. Down stops only verified resources and never removes workspace volumes. If exclusion cannot be proved after a crash, refuse a competing launch until explicit recovery.
5. Update migration/help and failure messages. Existing cells migrate atomically with a recoverable backup, no silent startup-policy changes and no key copying. Make stale Bash wrappers inert by replacing owned wrapper files only, preserving unmanaged files.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsNativeLifecycle$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsNativeLifecycle ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; new/check/up/desk/status/down reaches compiled child through both cockpit adapters with exact model/harness; repeated up reconnects | check +flow +dereference | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestContainerConsoleLossAndExclusion$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestContainerConsoleLossAndExclusion ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; lost console preserves container ID and volume; competing host/container coordinator refuses; reconnect restores the same container | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsStopOwnership$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsStopOwnership ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; changed cockpit selection, foreign terminal, reused PID and same-name foreign container never authorize teardown | check +flow | pwsh |
| 4 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsMigrationAndFailure$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsMigrationAndFailure ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; legacy config remains readable; failed migration/start is recoverable; missing policy/credentials/capability fails before launch | check +flow | pwsh |
| 5 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 6 | `statusgen --root ../.. --consumers cellctl-windows/06` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- Console loss destroys workspace or starts a duplicate coordinator: row 2.
- Lifecycle stops unrelated processes/terminals/containers: row 3.
- Port enables unattended credential minting or weakens role/model policy: rows 1 and 4, plus review of preserved attended gates.
- Migration overwrites user-owned files: row 4.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **human**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
