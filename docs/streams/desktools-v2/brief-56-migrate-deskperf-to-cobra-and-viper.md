---
brief: "assay:assay:desktools-v2:56"
title: "Migrate deskperf to Cobra and Viper"
why: >-
  deskperf, the read verb over the opt-in local perf history, shipped with desk-tools/23
  outside the Cobra and Viper migration. The routing registry must own every executable
  entrypoint, and this child owns it so the command moves onto the shared adapter like
  every other desk verb, without changing what it reads or reports.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "S"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 2
id: "0c579884-688c-46f0-90ae-5960c789df38"
authored: "2026-10-14 by desk-tools/23 (routing the entrypoint that PR added)"
sources:
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/56)"
  - "docs/telemetry.md"
exec-tier: "strong"
exec-tier-why: >-
  CLI and configuration compatibility: the documented report and boot forms, the 1-7 day
  window refusal and the kill-switch guard must keep working, and a happy-path parser test
  cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskperf: follow-up desktools-v2/56 (command tree, declared configuration, contract tests)"
  - "tools/desk/README.md: follow-up desktools-v2/56 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/telemetry.md: follow-up desktools-v2/56 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/56 (owned row flips to migrated)"
---

# Brief 56 — Migrate deskperf to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskperf` — `deskperf`; module tools/desk; parser: Go flag, one FlagSet per verb with manual verb dispatch; 339 LOC, ~2 verbs (`report`, `boot`), ~8 flags at 2ebdde50982a; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskperf/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-56.json` (planned) — muhar mutation spec for the owned command.
- `docs/streams/desktools-v2/cli-migration.json` — owned row flips to migrated.
- `changelog/cli-deskperf.md` (planned).

facts:
- The row is owned here because desk-tools/23 is not a migration child; a dedicated child keeps every existing child's Verify rows and package counts unchanged.
- The perf record format, its retention window and the no-PII field set are defined by desk-tools/23 and `docs/telemetry.md`; this brief moves the command surface and does not redefine them.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). The migrated Go row keeps its path and moves to state migrated once the contract tests pass.

layering: Cobra constructs the command tree and passes validated typed options into the existing report and boot handlers; a fresh command-local Viper resolves only declared keys. The kill-switch guard and the day-window refusal keep their current owners in the handlers.
single-point-of-failure: the day-window refusal (`--days` outside 1-7) — it stays in the handler; the mutation row plants its removal and requires the owner tests to redden with the parser bypassed.
design-fit:
  owner: the command's own package; record semantics stay with deskkit's perf reader
  contract: none — consumes cli-contract.md delivered by /15
  retires: [manual verb dispatch and per-verb FlagSets in deskperf]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve the guard, the day-window refusal, the read-only local-file
scope and every exit code. A CLI library is not a new authority source. Independent
verification owns Evidence and lifecycle advancement. Proposed tests and files below are
deliverables, not claims that they already exist.

## Task

1. Build a Cobra command tree for `deskperf`: root, `report`, `boot`, and the version forms. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config read, guard check or perf-file read, even with absent or malformed configuration. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config or global Viper.
4. Inventory every legacy form `tools/desk/README.md` and `docs/telemetry.md` use and keep it: `--f=v`, `--f v`, single-dash forms, and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in the migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry row to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskperf -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskperf -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskperf -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS; every documented legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-56.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, the day-window refusal removed; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskperf/...` | exit 0; every pre-existing test in the owned package still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskperf` | exit 0; the owned package compiles for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/56 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in the migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

| 10 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskperf/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | exit 0; named PASS; compiled help/version with malformed config and a large manifest fixture performs zero operational config/discovery/custody reads; moving discovery before help fails |

## Pre-mortem

Help reads config, checks the guard or opens a perf file before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A documented invocation stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
The day-window refusal moves into the parser and disappears from the handler: row 4 plants its removal from the handler.

## Performance reuse amendment — 2026-10-16

Keep this brief's parser ownership and existing compatibility rows. The shared credential
implementation belongs to /58–59, runtime manifest discovery to /60 and timing semantics to
/61–62. Help/version acceptance includes every pre-handler entrypoint step: no config echo,
activation discovery, file/index read, credential lookup or child process may occur first.
Count those effects in the compiled command test, not only inside the handler. If the shared
performance recorder is armed, a local best-effort performance record remains subject to its
existing explicit consent; help must not read operational configuration to classify itself.
Retain the one measurement finalizer when moving to Cobra. Do not reset the timer after
parsing, add a second writer or reintroduce a minter callout in an adapter.


## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check legacy-form coverage against the documented commands, that the old parser is gone, and that the guard and the day-window refusal did not weaken.
