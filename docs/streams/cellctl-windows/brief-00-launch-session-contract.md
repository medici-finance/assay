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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecRoundTrip$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecRoundTrip ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |
| 2 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestSessionRecordRefusal$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestSessionRecordRefusal ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |
| 3 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecCustody$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecCustody ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | pass exit=0 | sha256:5b2e7d0077ba | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |
| 5 | `$base = git merge-base refs/remotes/origin/main HEAD; if ($LASTEXITCODE -ne 0) { exit 1 }; statusgen --root ../.. --consumers --brief cellctl-windows/00 --base $base; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |
| 6 | `go run ./cmd/muhar -spec internal/celllaunch/mutations.json` | pass exit=0 | sha256:5ac04fbdb030 | 2026-10-02 | assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) (forge-identity) |

### Verification — cellctl-windows/00 (2026-10-02, non-implementer verifier)

- Target: merged main @ b2ba84875c6c
- Runner: assay-verifier-app[bot] (on-behalf-of human:ian), dispatched verifier, not the implementer
- Host: darwin/arm64, go1.27.1, statusgen v1.0.30; PowerShell is not present on this host
- Working directory for every row: tools/desk (the Verify section's stated convention)

| # | command (abbreviated) | exit | key observed output | result |
|---|---|---|---|---|
| 1 | pwsh: go test ./internal/celllaunch -run '^TestLaunchSpecRoundTrip$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | not run on this host (no PowerShell; native Windows runner required). statusgen verifyrun recorded could-not-run: "pwsh not available on this OS (darwin)" | could-not-run — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |
| 2 | pwsh: go test ./internal/celllaunch -run '^TestSessionRecordRefusal$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | not run on this host (no PowerShell; native Windows runner required). verifyrun recorded could-not-run | could-not-run — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |
| 3 | pwsh: go test ./internal/celllaunch -run '^TestLaunchSpecCustody$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | not run on this host (no PowerShell; native Windows runner required). verifyrun recorded could-not-run | could-not-run — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |
| 4 | sh: go test ./cmd/cellctl ./internal/cellcontainer -count=1 | 0 | "ok  .../tools/desk/cmd/cellctl 25.273s" and "ok  .../tools/desk/internal/cellcontainer 0.231s"; verifyrun witness pass exit=0 | pass — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |
| 5 | pwsh: statusgen --root ../.. --consumers --brief cellctl-windows/00 --base (merge-base of origin/main and HEAD) | - | not run on this host (no PowerShell; native Windows runner required). verifyrun recorded could-not-run | could-not-run — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |
| 6 | sh: go run ./cmd/muhar -spec internal/celllaunch/mutations.json | 0 | "Harness healthy: baseline GREEN, positive control CAUGHT."; seven mutations each CAUGHT; "Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate."; verifyrun witness pass exit=0 | pass — 2026-10-02 assay-verifier-app[bot] @ b2ba84875c6c (on-behalf-of human:ian) |

Row 6 detail. Baseline: green. Failing control ("child exit code is lost"): caught. Mutations, all caught: permitted launch equality removed; process identity comparison removed; console identity comparison removed; container identity comparison removed; duplicate JSON guard removed; ambient environment forwarded; reserved device basename guard removed. Survivors: none.

Execution-witness tool. `statusgen verifyrun` dispatches rows marked pwsh only on Windows and records them could-not-run elsewhere; it neither rewrites them for another shell nor records a product failure. With the command root set to tools/desk it recorded rows 1, 2, 3, 5 could-not-run and rows 4, 6 pass (tool exit 2; `--check` reports 2 pass, 0 fail, 4 could-not-run of 6). At its default root (the repository top level) rows 4 and 6 exit 1 with "go.mod file not found", which is a working-directory artefact and not a product result, so the witness was written only with the tools/desk root.

Supplementary neighbouring-platform observation (darwin; NOT the rows, and not counted toward the verdict). The three selected tests were run directly with go test ... -run '^Name$' -count=1 -v from tools/desk: TestLaunchSpecRoundTrip exit 0, "--- PASS: TestLaunchSpecRoundTrip", no SKIP lines; TestSessionRecordRefusal exit 0, "--- PASS: TestSessionRecordRefusal", no SKIP lines; TestLaunchSpecCustody exit 0, "--- PASS: TestLaunchSpecCustody", no SKIP lines. The consumers command of row 5, run under sh on darwin, exited 0 with "summary: 0 corroborated, 0 disproved, 7 unchecked": on merged main the merge-base equals HEAD, so the diff it inspects is empty. Row 5's Expect ("declared consumer routing corroborates the implementation diff") therefore cannot be observed from merged main with the row as written, on any host; it needs a base that precedes the implementation merge.

Related observation, not a row result. The native Windows workflow (celllaunch-windows) triggers on pull requests only; its most recent run succeeded at PR head ec3dd1b4e358, whose contract package and workflow file are byte-identical to merged main. That run was not re-read line by line in this pass and is not on the merged SHA, so it is cited as a pointer only.

Risk-bearing value. The brief carries risk metadata with every field "no" and irreversible "no". Literals enumerated in the contract package: MaxRecordBytes = 128 * 1024 @ tools/desk/internal/celllaunch/spec.go:20; LaunchSchema = "cell-launch-v1" @ spec.go:18; SessionSchema = "cell-session-v1" @ spec.go:19; name pattern length bound {0,95} @ spec.go:23; full identity pattern 64 hex @ spec.go:24; control-character bound r < 32 or r == 127 @ spec.go:159. All are reversible by an edit and a release (an input-size cap, schema labels, input-shape bounds); none is an irreversible value.
RISK-VALUE: NAMED, NOT DERIVED — MaxRecordBytes = 128 * 1024 @ tools/desk/internal/celllaunch/spec.go:20 — reversible input-size cap; no derivation attempted. Whether the merged diff touches a risk-classed path was not checked in this pass.

VERIFY: BLOCKED

Reasoning. Nothing failed: both Unix rows (4 and 6) ran from the stated directory and passed with real output, including a healthy mutation harness with zero survivors. Rows 1, 2, 3 and 5 are authored for PowerShell on a native Windows runner and could not execute on the only available host, so they are recorded could-not-run rather than rounded to pass; the darwin observations above are supplementary and do not stand in for them. Four of six required rows have no witness, so the brief cannot advance on this evidence.

To finish. A native Windows amd64 host with PowerShell 7 (pwsh), the Go toolchain, git and statusgen on PATH, checked out at merged main, running rows 1, 2, 3 and 5 from tools/desk. Row 5 additionally needs its base question settled (see above) before its Expect can be met on merged main.
| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecRoundTrip$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecRoundTrip ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestSessionRecordRefusal$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestSessionRecordRefusal ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | `$log = New-TemporaryFile; go test ./internal/celllaunch -run '^TestLaunchSpecCustody$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestLaunchSpecCustody ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | pass exit=0 | sha256:5ff99274395f | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | `$base = git merge-base refs/remotes/origin/main HEAD; if ($LASTEXITCODE -ne 0) { exit 1 }; statusgen --root ../.. --consumers --brief cellctl-windows/00 --base $base; if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }` | could-not-run exit=- — pwsh not available on this OS (darwin) — the row is marked `pwsh` (PowerShell), which this fix dispatches only on Windows; could-not-run here, never a product-check `fail` and never silently rewritten for another shell (issue #1424) | sha256:e3b0c44298fc | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | `go run ./cmd/muhar -spec internal/celllaunch/mutations.json` | pass exit=0 | sha256:3e14bad9930a | 2026-10-07 | assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |


### Verification — cellctl-windows/00 (2026-10-07, non-implementer verifier, re-verify of the 2026-10-02 BLOCKED pass)

- Target: merged main @ 91f04b81ba064394aa121a4940cf11a138b55402 (fetched fresh 2026-10-07)
- Runner: assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity), dispatched verifier, not the implementer
- Host: darwin/arm64, go1.27.1, statusgen v1.0.32; PowerShell (pwsh) is not present on this host
- Working directory for every row: tools/desk (the Verify section's stated convention); verifyrun witness written with its command root set to tools/desk

| # | command (abbreviated) | exit | key observed output | result |
|---|---|---|---|---|
| 1 | pwsh: go test ./internal/celllaunch -run '^TestLaunchSpecRoundTrip$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | could-not-run on this host (no PowerShell; native Windows runner required); verifyrun: "pwsh not available on this OS (darwin)" | could-not-run — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 2 | pwsh: go test ./internal/celllaunch -run '^TestSessionRecordRefusal$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | could-not-run on this host (no PowerShell; native Windows runner required); verifyrun recorded could-not-run | could-not-run — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 3 | pwsh: go test ./internal/celllaunch -run '^TestLaunchSpecCustody$' -count=1 -v, with exact PASS-line and no-SKIP assertions | - | could-not-run on this host (no PowerShell; native Windows runner required); verifyrun recorded could-not-run | could-not-run — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 4 | sh: go test ./cmd/cellctl ./internal/cellcontainer -count=1 | 0 | "ok  .../tools/desk/cmd/cellctl 92.275s" and "ok  .../tools/desk/internal/cellcontainer 1.926s"; verifyrun witness pass exit=0 | pass — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 5 | pwsh: statusgen --root ../.. --consumers --brief cellctl-windows/00 --base (merge-base of origin/main and HEAD) | - | could-not-run on this host (no PowerShell; native Windows runner required); verifyrun recorded could-not-run | could-not-run — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |
| 6 | sh: go run ./cmd/muhar -spec internal/celllaunch/mutations.json | 0 | "Harness healthy: baseline GREEN, positive control CAUGHT."; seven mutations each CAUGHT; "Totals: 7 caught, 0 NOT CAUGHT, 0 could-not-mutate."; verifyrun witness pass exit=0 | pass — 2026-10-07 assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity) |

Row 6 detail. Baseline green; failing control ("child exit code is lost") caught; caught: permitted launch equality removed, process identity comparison removed, console identity comparison removed, container identity comparison removed, duplicate JSON guard removed, ambient environment forwarded, reserved device basename guard removed. Survivors: none.

Prior blocker status. The 2026-10-02 pass was BLOCKED on "no native Windows host with pwsh for rows 1, 2, 3, 5". On current main that blocker is NOT resolved: no commit since b2ba84875c6c touches the celllaunch package, the celllaunch-windows workflow or docs/cellctl-windows.md (git log and diff --stat both empty); the celllaunch-windows workflow still triggers on pull_request only, so no native-Windows witness exists for any merged-main SHA; this host has no pwsh. The brief's Verify table is unchanged since c37d1310.

Row 5 — changed behaviour, now a latent FAIL as written. Under statusgen v1.0.32 the row-5 consumers command, run under sh on darwin from merged main, exits 2: "COULD-NOT-CHECK: assay:assay:cellctl-windows:00 is not in the diff against 91f04b81ba06 ... no entry was corroborated and none was disproved" (merge-base of origin/main and HEAD equals HEAD). The 2026-10-02 pass saw exit 0 with "0 corroborated, 0 disproved, 7 unchecked" under v1.0.30. Because row 5 ends in exit $LASTEXITCODE, it would now exit 2 on a Windows host too; it cannot pass from merged main on any host until the row pins a base that precedes the implementation merge. Supplementary corroboration (not the row): with the worktree detached at the implementation merge 2a6460e85c78 and base set to its parent ffdc7f69c8c3, the same consumers command exited 0 with "summary: 7 corroborated, 0 disproved, 0 unchecked" (all four fixed-here sites and all four follow-ups corroborated). Worktree restored to 91f04b81ba06 afterwards.

Supplementary neighbouring-platform observation (darwin; NOT the rows, not counted toward the verdict). The three selected tests run directly with go test -run '^Name$' -count=1 -v from tools/desk: TestLaunchSpecRoundTrip exit 0, "--- PASS: TestLaunchSpecRoundTrip (0.02s)", 0 SKIP lines; TestSessionRecordRefusal exit 0, "--- PASS: TestSessionRecordRefusal (0.00s)", 0 SKIP lines; TestLaunchSpecCustody exit 0, "--- PASS: TestLaunchSpecCustody (0.04s)", 0 SKIP lines.

Grounding note. The release-note fragment named in Context (changelog/cellctl-windows-00.md) is absent by design: ec260632 aggregated fragments into CHANGELOG.md under v1.0.31 (line 169, "Define Go launch and session contracts for the native Windows cellctl work ...").

Risk-bearing value. Risk metadata is present with every field "no" (irreversible: no; gate: model), so the trigger fires only through the enumeration duty. Enumerated over the celllaunch package (spec.go, session.go): LaunchSchema = "cell-launch-v1" @ tools/desk/internal/celllaunch/spec.go:18; SessionSchema = "cell-session-v1" @ spec.go:19; MaxRecordBytes = 128 * 1024 @ spec.go:20; namePattern length bound {0,95} @ spec.go:23; fullID = 64 lowercase hex @ spec.go:24; len(Args) > 256, len(Env) > 64, len(Credentials) > 8 @ spec.go:80; per-arg 16384 bytes @ spec.go:84; per-env-value 16384 bytes @ spec.go:91; forge providers "github"/"gitlab" @ spec.go:98; publicEnv allowlist (25 names, HOME .. ASSAY_COCKPIT) @ spec.go:109-116; OS set windows/darwin/linux @ spec.go:150; cockpit set orca/herdr/tmux (tmux not on windows) @ spec.go:152; control-character bound r < 32 or r == 127 @ spec.go:159; JSON nesting depth > 16 @ spec.go:208; reserved device basenames CON/PRN/AUX/NUL and COM1-9/LPT1-9 @ session.go:26-27. Ranked: the publicEnv allowlist (authority binding over what env NAMES may enter a durable record) and fullID (the custody digest binding) rank highest; every entry is reversible by an edit and a release, and no production runner consumes the contract yet (that is cellctl-windows/01).
RISK-VALUE: DERIVED — fullID = ^[a-f0-9]{64}$ @ tools/desk/internal/celllaunch/spec.go:24 — the digest it gates is hex.EncodeToString(sha256.Sum256(data)) at session.go:40, and a SHA-256 digest is 32 bytes = exactly 64 lowercase hex characters, so the pattern admits every honest digest and nothing else.
RISK-VALUE: DERIVED — publicEnv = {HOME, USERPROFILE, SYSTEMROOT, WINDIR, APPDATA, LOCALAPPDATA, PATH, PATHEXT, TEMP, TMP, TMPDIR, LANG, TERM, KUBECONFIG, ASSAY_CONFIG_HOME, GH_CONFIG_DIR, GIT_CONFIG_GLOBAL, GIT_CONFIG_NOSYSTEM, GIT_TERMINAL_PROMPT, CODEX_HOME, CLAUDE_CONFIG_DIR, DESK_LOOP, DESK_SESSION, DESK_ROOTS, ASSAY_COCKPIT} @ tools/desk/internal/celllaunch/spec.go:109-116 — every name is a path, locale, terminal or session-routing variable; none is a credential-carrying name (no GH_TOKEN/GITHUB_TOKEN/*_API_KEY), which is what task 1 ("a serializable env map cannot contain secret values") requires of an allowlist; values additionally need an exact independently-built Permit match (spec.go:119-124).
RISK-VALUE: NAMED, NOT DERIVED — MaxRecordBytes = 128 * 1024 @ tools/desk/internal/celllaunch/spec.go:20 — reversible input-size cap; no first-principles sizing exists in the brief. Observation for the desk (not a row result): the reserved-basename guard at session.go:26-27 admits COM0 and LPT0 (digit range '1'..'9'); current Microsoft file-naming guidance also lists COM0/LPT0 as reserved. Whether they resolve as devices could not be checked on this darwin host.

VERIFY: BLOCKED

Reasoning. Nothing failed: both Unix rows (4 and 6) ran from the stated directory on merged main and passed with real output. Rows 1, 2, 3 and 5 are authored for pwsh on native Windows and could not execute on the only available host; they are recorded could-not-run, not rounded to pass. Four of six required rows have no witness, so the brief cannot advance. To finish: (a) a native Windows amd64 host with pwsh, Go, git and statusgen at merged main runs rows 1, 2, 3, 5 from tools/desk; (b) row 5 is re-authored to pin the implementation merge's parent as its base (under statusgen v1.0.32 the row as written exits 2 COULD-NOT-CHECK on merged main on any host).

Desk note: row 5 is recorded on #1915 (issuecomment-6029842170). Rows 1, 2, 3 and 5 still need a native Windows amd64 run at merged main; the celllaunch Windows workflow runs on pull_request only, so no merged-main SHA carries that witness yet.
Desk note (2026-10-07, review response on #2330, findings C1 and C3a/b). This note appends; it does not reword the text above.

- **Which table belongs to which pass.** The six-row `| # | Command | Result | Output | Date | Runner |` table just above the "### Verification — cellctl-windows/00 (2026-10-07 …)" heading is the **2026-10-07** pass's verifyrun witness, at merged main 91f04b81ba06. It sits after the 2026-10-02 "To finish." paragraph only because the witness appends at the end of the section. It is not part of the 2026-10-02 cycle.
- **Exact commands.** The summary table under the 2026-10-07 heading abbreviates its commands and merges Date and Runner into its result cell. The exact runnable commands are in the witness table's rows 1-6, verbatim from the Verify table. That table has split Date and Runner columns: 2026-10-07, assay-verifier-app[bot] @ 91f04b81ba06 (on-behalf-of human:ian) (forge-identity). Both tables record the same run.
- **Outcome record.** `docs/streams/verify-outcomes/cellctl-windows/00-20261007T130245Z-8c077859f851.json` records outcome blocked, 2/6, rows [1, 2, 3, 5], blocker_kind environment, blocker_ref #2045.

**VERIFY: BLOCKED** (unchanged)

## Review

Gate: **model**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
