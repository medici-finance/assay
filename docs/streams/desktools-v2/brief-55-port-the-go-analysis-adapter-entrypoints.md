---
brief: "assay:assay:desktools-v2:55"
title: "Port the go-analysis adapter entrypoints"
why: >-
  The pinned Go analysis adapter (a Python runner, its bash shim and the composite action's
  step script) shipped with desk-tools/34 outside the Cobra and Viper migration. The routing
  registry must own every executable entrypoint, and no existing child has budget left for
  three more. This child owns them and moves their parsing behind one Go command without
  changing a single analysis verdict.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "0d023d7f-6d22-4cf1-93fb-169ef559aba1"
authored: "2026-10-13 by desk-tools/34 (routing the entrypoints that PR added)"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/55)"
  - "docs/go-analysis.md"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: the action, the harness and the
  documented commands must keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/go-analysis/runner.py: follow-up desktools-v2/55 (command tree, declared configuration, contract tests)"
  - "scripts/go-analysis.sh: follow-up desktools-v2/55 (command tree, declared configuration, contract tests)"
  - "actions/go-analysis/run.py: follow-up desktools-v2/55 (command tree, declared configuration, contract tests)"
  - "actions/go-analysis/action.yml: follow-up desktools-v2/55 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/go-analysis.md: follow-up desktools-v2/55 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/55 (owned rows flip to migrated)"
---

# Brief 55 — Port the go-analysis adapter entrypoints

## Context

files:
- `tools/go-analysis/runner.py` — `runner.py`; module python3; parser: Python argparse (`acquire`, `lint`); release: repo tooling; run by the go-analysis action and its harness. Target: `tools/desk/cmd/deskgoanalysis`.
- `scripts/go-analysis.sh` — `go-analysis.sh`; module bash; parser: arguments passed through to the runner; 5 lines; release: repo tooling; documented as `bash scripts/go-analysis.sh`. Target: `tools/desk/cmd/deskgoanalysis`.
- `actions/go-analysis/run.py` — `run.py`; module python3; parser: environment variables set by action.yml; release: composite GitHub Action (actions/go-analysis); run at the consumer's pinned ref. Target: `tools/desk/cmd/deskgoanalysis`.
- `tools/desk/cmd/deskgoanalysis/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-55.json` (planned) — muhar mutation spec for the owned command.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-go-analysis.md` (planned).

facts:
- The three rows are owned here because desk-tools/34 is not a migration child and every existing child is at its budget of five simple rows.
- The adapter, ratchet identity, baseline format and report schema are defined by desk-tools/34 and `docs/go-analysis-report-v1.md`; this brief moves their command surface and does not redefine them.
- The adapter's test-only companions (the harness, its mutation runner and the action's wrapper tests) are classified exclusions in the registry, not rows.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). A migrated script row names its Go entrypoint in `migrated_to`, and the script itself must name that entrypoint (it delegates); a new Go entrypoint gets its own registry row, owned by this brief, state migrated.

layering: Cobra constructs the command tree and passes validated typed options into the existing analysis handlers; a fresh command-local Viper resolves only declared keys. The pinned tool checksums, offline lint and trusted-base ratchet keep their current owners.
single-point-of-failure: the ratchet comparison against the trusted base baseline — it moves into the Go handler unchanged; the harness's guard mutations and TestCLIAdmissionBoundary-style direct handler calls prove the lower layer refuses with the adapter bypassed.
design-fit:
  owner: the command's own package; analysis semantics stay with the pinned upstream analyzers
  contract: none — consumes cli-contract.md delivered by /15
  retires: [hand-rolled argument handling in the Python runner and the action step script]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve existing checksum, offline, trusted-base and exit-code
behaviour; a CLI library is not a new authority source. Independent verification owns
Evidence and lifecycle advancement. Proposed tests and files below are deliverables, not
claims that they already exist.

## Task

1. Port the runner's `acquire` and `lint` behaviour into `tools/desk/cmd/deskgoanalysis` with a Cobra command tree. Reduce `scripts/go-analysis.sh` and `actions/go-analysis/run.py` to transparent adapters that exec the target with equivalent arguments, or retire them in the same change that removes every consumer. Add a registry row for the new Go entrypoint.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, network read, download, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config or global Viper.
4. Inventory every legacy form the action, the harness and `docs/go-analysis.md` use and keep it: `--f=v`, `--f v`, `--` termination, repeated flags and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: the harness's checksum, offline-transport, baseline-growth and policy-migration controls pass against the Go command, and a guard removed from the handler turns a harness case red.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in the migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskgoanalysis -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskgoanalysis -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskgoanalysis -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-55.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check +flow | `bash scripts/tests/go-analysis.sh lint` | exit 0; the committed ratchet, checksum, offline and guard-mutation controls pass against the delegating adapters |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskgoanalysis` | exit 0; the owned package compiles for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/55 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in the migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads config or performs a download before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
The action or a documented command stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
A ratchet or checksum check moves into the adapter and disappears from the handler: row 5 runs the harness controls against the delegating adapters.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check legacy-form coverage against the action and the harness, that the old parsers are gone, and that no checksum, offline or trusted-base refusal weakened.
