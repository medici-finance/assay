---
brief: assay:assay:cellctl-windows:00
title: Go launch and session contracts for native Windows
why: >-
  The launcher passes shell text between several components, so replacing individual scripts leaves quoting and ownership bugs in place. One explicit contract lets platform and cockpit work proceed independently without creating another launcher.
wave: 0
depends: []
unblocks: ["cellctl-windows/01", "cellctl-windows/02", "cellctl-windows/03", "cellctl-windows/04"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: 33672111-6df2-43c3-bc6e-38c81151c505
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
  - ".github/workflows/celllaunch-windows.yml: fixed-here (native Windows amd64 execution of the three contract witnesses)"
  - "tools/desk/internal/celllaunch/: fixed-here (versioned contracts, parsers, compiled fixtures and mutation specification)"
  - "docs/cellctl-windows.md: fixed-here (handoff protocol, lifecycle semantics and capability evidence checklist)"
  - "cellctl-windows/01: follow-up cellctl-windows/01 (production runner and private store consume the launch contract)"
  - "cellctl-windows/02: follow-up cellctl-windows/02 (process supervisor consumes scoped process identity and observations)"
  - "cellctl-windows/03: follow-up cellctl-windows/03 (cockpit adapters consume RunnerRequest and exact console identity)"
  - "cellctl-windows/04: follow-up cellctl-windows/04 (Docker endpoint work supplies the frozen engine identity)"
---

# Brief 00 — Go launch and session contracts for native Windows

## Context

files:
- .github/workflows/celllaunch-windows.yml (native amd64 contract witnesses for Verify rows 1–3)
- tools/desk/internal/celllaunch/ (new contract and tests)
- docs/cellctl-windows.md (new implementation contract)
- changelog/cellctl-windows-00.md (release note)

facts (freshness checked 2026-10-01):
- At 1c67aa717, cmd/cellctl/up.go roleCmd returns shell text; firstWindow contains a shell health loop; Herdr and Orca adapters consume command strings.
- At the same revision, down.go re-resolves the current cockpit and Orca teardown closes all worktree terminals; native containers use tmux in container_native.go.
- Orca and Herdr are required target cockpits. Their Windows binaries, versions and launch/close API shapes must be measured; this plan does not assert those capabilities from Unix help.

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

1. Define versioned LaunchSpec and SessionRecord types: executable, argv, nonsecret env plus typed credential-resolution references, cwd, cell/role, intent (host process or Linux container), OS, cockpit, exact terminal handle, process identity and immutable Docker identity. Reject unknown schema and malformed records; never store token values in durable specs. The runner accepts only validated cell-owned private records and permitted executable/operation identities, not an arbitrary executable from an untrusted path. Resolve credentials at launch time within the existing role authority; a serializable env map cannot contain secret values.
2. Specify private, atomic launch-spec handoff and a Go runner consuming it. This brief implements the schema/parser and compiled test runner; 01 owns the production runner entrypoint and packaged dispatch helper. Adapter work in 03 uses the test runner, so it can proceed independently of 01 until integration in 06. Prefer cockpit argv APIs; where only a command string exists, constrain that boundary to invoking the Go runner with an opaque private spec reference, with platform-correct escaping. No user/model/repo text becomes executable shell syntax. No generated Bash or PowerShell lifecycle program.
3. Specify adapter Create/Inspect/Attach/Close and process ownership interfaces. Define terminal-close versus stop, partial-start rollback, duplicate up, stale identity, configured-model override, backend changes and crash recovery. Docker lifetime is independent of its console. Persistent state is untrusted input, never sufficient proof of ownership.
4. Add compiled Go fixture children and contract tests, including exact argv/env/cwd roundtrip and nonzero exit propagation. Write the capability/evidence checklist for actual Windows Orca, Herdr and Docker; unsupported capabilities are explicit blockers for that backend, never tmux/WSL fallback.
5. Record a proposed design-decision entry in the implementation PR for credential/process boundaries covered by downstream human-gated briefs; keep it proposed until the authorized human ratifies it. Reference it from those briefs before they enter in-progress.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecRoundTrip$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecRoundTrip ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; exact argv/env/cwd survives spaces, apostrophes, ampersands, percent signs, parentheses and Unicode | check +flow +dereference | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestSessionRecordRefusal$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestSessionRecordRefusal ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; unknown schema, foreign cell/role, reused identity and incomplete launch state refuse | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecCustody$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecCustody ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; token values are absent from serialized records and logs; untrusted spec reference refuses | check +flow | pwsh |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 5 | `$base = git merge-base refs/remotes/origin/main HEAD; if ($LASTEXITCODE -ne 0) { exit 1 }; statusgen --root ../.. --consumers --brief cellctl-windows/00 --base $base; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |
| 6 | `go run ./cmd/muhar -spec internal/celllaunch/mutations.json` | exit 0; baseline passes, failing control is caught, all seven guard mutations are caught with no survivors; run on Unix using the existing development mutation harness | check +mutation | sh |

## Threat model / pre-mortem

- Shell metacharacters become code: row 1.
- A stale or foreign record authorizes shutdown: row 2.
- Secrets become durable launch metadata: row 3.
- A third-party API is assumed rather than measured: capability review and real-host evidence in cellctl-windows/07; not established by this contract.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **model**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
