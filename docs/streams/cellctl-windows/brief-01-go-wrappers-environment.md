---
brief: assay:assay:cellctl-windows:01
title: Go wrappers and native Windows cell environment
why: >-
  Generated Bash wrappers remain on every desk tool path even after the outer launcher became Go. Windows needs executable dispatch that preserves credential precedence and the scrubbed environment without requiring a Unix shell.
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
id: de21518d-e55c-4dda-a116-dd18a0cce095
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
  - "tools/desk/cmd/cellctl/shims.go, env.go, launch.go: fixed-here"
  - "tools/desk/cmd/cellctl/cell.go, policy_enforce.go, check.go: fixed-here"
  - "tools/desk/internal/celllaunch/: fixed-here"
  - "cellctl-windows/06: follow-up cellctl-windows/06"
---

# Brief 01 — Go wrappers and native Windows cell environment

## Context

files:
- tools/desk/cmd/cellctl/shims.go, env.go, launch.go (environment helpers only), new.go
- tools/desk/cmd/cellctl/cell.go, policy_enforce.go, check.go (platform/config portions)
- tools/desk/internal/celllaunch/ (consume contract only; interface changes owned by 00)

facts (freshness checked 2026-10-01):
- shims.go emits Bash desk-tool and gh wrappers, selects files by executable bits and creates symlinks. new.go also assumes symlink-based setup.
- env.go assumes Bash, POSIX PATH and /dev/null; launch.go environment updates compare names case-sensitively. cell.go defaults to /opt/desk-tools/bin.
- policy_enforce.go creates shell command callbacks and discovers managed-settings roots; a Windows port must preserve the blocking exit behavior and administrator-policy checks.

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

1. Implement the production launch-spec runner specified in 00, including launch-time credential resolution and trusted-spec/executable validation. Replace wrapper generation with Go executable dispatch (a dedicated shipped helper or copies of the same binary with explicit dispatch metadata). Do not depend on extensionless shell scripts, symlink privileges or developer mode. Preserve arguments, stdio, Ctrl+C behavior and exit codes.
2. Resolve native Windows executable/config/temp locations and PATHEXT/.exe behavior. If a vendor executable resolves to .cmd/.bat, use a documented native executable entrypoint or explicitly tested bounded Windows invocation, otherwise refuse with a precise prerequisite error; discovery alone is not launch support. Compose PATH with os.PathListSeparator and deduplicate Windows environment names case-insensitively. Keep a declared allowlist of required OS variables for scrubbed sessions.
3. Preserve ambient forge-credential acquisition before HOME changes and explicit role-token precedence. Never place tokens in argv, launch-spec files or logs; do not copy host authentication stores into scrubbed cells. Keep Git-owned hook mechanics outside this runtime-wrapper scope.
4. Make cell.env parsing/writing round-trip Windows drive, backslash, UNC and Unicode values without shell execution; retain compatibility with existing files and backups. Replace /dev/null quarantine with a verified inert platform-appropriate kubeconfig mechanism.
5. Port policy callback invocation and managed-settings discovery to Windows with the same refuse-on-missing-runner semantics. Use compiled fixture tools rather than Bash test doubles.

## Verify

Commands below run from `tools/desk` on the stated native platform. New test names are
required deliverables, not claims that tests already exist. Each selected test must appear
as PASS with zero skips; a command matching no tests is a failure. Windows-specific rows
must run on Windows; Unix regression rows run separately. The Shell column is explicit:
Windows witnesses use pwsh, not the row runner's default sh. Each PowerShell command
asserts the exact selected PASS line and rejects skips as well as a nonzero exit. No live credentials are needed.

| # | Command | Expect | Class | Shell |
|---|---------|--------|-------|-------|
| 1 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsWrapperFlow$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsWrapperFlow ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; native executable dispatch preserves argv, cwd, stdio and exit status without Bash or symlinks | check +flow | pwsh |
| 2 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsCredentialPrecedence$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsCredentialPrecedence ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; explicit role credential wins; ambient credential is scoped to forge child; no secret in argv/records/logs | check +flow | pwsh |
| 3 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsEnvironmentAndConfig$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsEnvironmentAndConfig ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; PATH, mixed-case env keys, OS variables, drive/UNC paths, inert kubeconfig and config roundtrip pass | check +flow | pwsh |
| 4 | `$log = New-TemporaryFile; go test ./cmd/cellctl -run '^TestWindowsPolicyFailClosed$' -count=1 -v *> $log; if ($LASTEXITCODE -ne 0) { Get-Content $log; exit 1 }; if (-not (Select-String -Path $log -SimpleMatch '--- PASS: TestWindowsPolicyFailClosed ')) { exit 1 }; if (Select-String -Path $log -SimpleMatch '--- SKIP:') { exit 1 }` | exit 0; missing hook runner, conflicting managed policy and denied model block launch; positive allowed case passes | check +flow | pwsh |
| 5 | `go test ./cmd/cellctl ./internal/cellcontainer -count=1` | exit 0; pre-existing Unix regression assertions remain effective; run on Unix as the neighboring platform | check +neighbour | sh |
| 6 | `statusgen --root ../.. --consumers cellctl-windows/01` | exit 0; declared consumer routing corroborates the implementation diff | check | pwsh |

## Threat model / pre-mortem

- A Go shim changes credential authority: row 2.
- Host credentials or cluster context leak through Windows environment: rows 2 and 3.
- A missing policy runner silently permits a model: row 4.
- Only shell-backed test doubles work: row 1 plus the shell-denial harness in 07.

## Evidence

<!-- No implementation runs yet. Independent verifier records command, exit, output,
date, exact SHA, OS/architecture and backend versions. Record could-not-check honestly. -->

## Review

Gate: **human**. Confirm scope, truthful native-runtime evidence, preserved security
boundaries and the failure cases above. No readiness or support claim from compilation alone.
