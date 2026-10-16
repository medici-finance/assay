---
brief: "assay:assay:desktools-v2:57"
title: "Migrate intake-eval to Cobra and Viper"
why: >-
  intake-eval, the intake-routing scorer, was added outside the Cobra and Viper migration.
  The routing registry must own every executable entrypoint, and this child owns it so the
  scorer moves onto the shared adapter like every other tool, without changing what it
  reads, scores or exits with.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "S"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111, 2980]
schema: "brief-v2"
outcome: none
version: 1
id: "4e0a7993-be21-4987-a80d-432cd6e6898b"
authored: "2026-10-15 by the intake-scorer change (routing the entrypoint that PR added)"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "https://github.com/medici-finance/assay/issues/2980"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/57)"
  - "tools/intake-eval/README.md"
exec-tier: "strong"
exec-tier-why: >-
  CLI and configuration compatibility: the documented flag forms, the --baseline requires
  --thresholds refusal and the 0/1/2 exit split must keep working, and a happy-path parser
  test cannot show that.
domain: "complicated"
consumers:
  - "tools/intake-eval: follow-up desktools-v2/57 (command tree, declared configuration, contract tests)"
  - "tools/intake-eval/README.md: follow-up desktools-v2/57 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/57 (owned row flips to migrated)"
---

# Brief 57 — Migrate intake-eval to Cobra and Viper

## Context

files:
- `tools/intake-eval` — `intake-eval`; module tools/intake-eval; parser: Go flag, one global FlagSet, no verbs; 720 LOC, ~0 verbs, ~6 flags at 193003bc2570; release: module-local, no release or workflow build declaration yet.
- `tools/intake-eval/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-57.json` (planned) — muhar mutation spec for the owned command.
- `docs/streams/desktools-v2/cli-migration.json` — owned row flips to migrated.
- `changelog/cli-intake-eval.md` (planned).

facts:
- The row is owned here because the scorer's change is not a migration child; a dedicated child keeps every existing child's Verify rows and package counts unchanged.
- The gold-set schema, metric definitions, threshold config and report shape are defined by the scorer's README; this brief moves the command surface and does not redefine them.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). The migrated Go row keeps its path and moves to state migrated once the contract tests pass.

layering: Cobra constructs the root command and passes validated typed options into the existing load, score and verdict handlers; a fresh command-local Viper resolves only declared keys. The input lint (exit 2) and the threshold verdict (exit 1) keep their current owners in the handlers.
single-point-of-failure: the fixture preflight lint (malformed gold set exits 2 before scoring) — it stays in the handler; the mutation row plants its removal and requires the owner tests to redden with the parser bypassed.
design-fit:
  owner: the command's own package; scoring semantics stay with its metrics and thresholds files
  contract: none — consumes cli-contract.md delivered by /15
  retires: [the global FlagSet in intake-eval]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve the fixture lint, the --baseline requires --thresholds
refusal, the read-only local-file scope and every exit code. A CLI library is not a new
authority source. Independent verification owns Evidence and lifecycle advancement.
Proposed tests and files below are deliverables, not claims that they already exist.

## Task

1. Build a Cobra command tree for `intake-eval`: the root command and the version forms. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root `-h`, `--help` and version exit 0 before any config read or gold-set read, even with absent or malformed configuration. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config or global Viper.
4. Inventory every legacy form `tools/intake-eval/README.md` uses and keep it: `--f=v`, `--f v` and single-dash forms. Unknown flags and bad values fail before any effect with exit 2, as today. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in the migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry row to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/intake-eval && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS; root help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/intake-eval && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/intake-eval && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS; every documented legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-57.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, the fixture preflight lint removed; each reddens the owner tests |
| 5 | check | `cd tools/intake-eval && go test -count=1 ./...` | exit 0; every pre-existing test in the owned package still passes |
| 6 | check | `cd tools/intake-eval && GOOS=windows go vet .` | exit 0; the owned package compiles for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/57 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in the migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads config or the gold set before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A documented invocation stops parsing, or a bad flag exits 1 (a FAIL verdict) instead of 2: row 3 replays each documented form and the unknown-flag exit.
The owner tests pass on the happy path but miss a broken lint: the mutation row plants each break and requires it to redden.
The fixture preflight lint moves into the parser and disappears from the handler: row 4 plants its removal from the handler.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check legacy-form coverage against the documented commands, that the old parser is gone, and that the fixture lint, the --baseline refusal and the 0/1/2 exit split did not weaken.
