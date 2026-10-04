---
brief: "assay:assay:desktools-v2:48"
title: "Migrate the small standalone module CLIs to Cobra and Viper"
why: >-
  Five small standalone modules (loopadmin/mutate, bugs-gc, freshness, loopresolve, winparity)
  use the global flag set or a single FlagSet. Moving each onto Cobra and command-local Viper
  gives consistent help across the suite.
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
id: "d5b963d4-7a5f-4918-96e3-2e6aaaf74067"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/48)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "loopadmin/mutate: follow-up desktools-v2/48 (command tree, declared configuration, contract tests)"
  - "tools/bugs-gc: follow-up desktools-v2/48 (command tree, declared configuration, contract tests)"
  - "tools/freshness: follow-up desktools-v2/48 (command tree, declared configuration, contract tests)"
  - "tools/loopresolve: follow-up desktools-v2/48 (command tree, declared configuration, contract tests)"
  - "tools/winparity: follow-up desktools-v2/48 (command tree, declared configuration, contract tests)"
  - "loopadmin/README.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "Dockerfile: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/iso9001-mapping.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "freshness.yaml: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "README.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "scripts/build-windows.ps1: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/winparity.yml: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/winparity.yml: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/README.md: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "Makefile: follow-up desktools-v2/48 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/48 (owned rows flip to migrated)"
---

# Brief 48 — Migrate the small standalone module CLIs to Cobra and Viper

## Context

files:
- `loopadmin/mutate` — `mutate`; module loopadmin; parser: Go flag: global FlagSet; 158 LOC, ~0 verbs, ~2 flags at 3ad1ad83c871; release: module-local; no release or workflow build declaration.
- `tools/bugs-gc` — `bugs-gc`; module tools/bugs-gc; parser: Go flag: global FlagSet; 106 LOC, ~0 verbs, ~2 flags at 3ad1ad83c871; release: module-local; no release or workflow build declaration.
- `tools/freshness` — `freshness`; module tools/freshness; parser: Go flag: FlagSet; 286 LOC, ~0 verbs, ~3 flags at 3ad1ad83c871; release: module-local; no release or workflow build declaration.
- `tools/loopresolve` — `loopresolve`; module tools/loopresolve; parser: Go flag: FlagSet; 128 LOC, ~0 verbs, ~4 flags at 3ad1ad83c871; release: module-local; no release or workflow build declaration.
- `tools/winparity` — `winparity`; module tools/winparity; parser: Go flag: global FlagSet; 287 LOC, ~0 verbs, ~1 flags at 3ad1ad83c871; release: module-local; run by winparity.yml.
- `loopadmin/mutate/cli_test.go` (planned), `tools/bugs-gc/cli_test.go` (planned), `tools/freshness/cli_test.go` (planned), `tools/loopresolve/cli_test.go` (planned), `tools/winparity/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-48.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-modules.md` (planned).

facts:
- Each is its own Go module; each imports Cobra and Viper directly and copies the fixture cases listed in cli-contract.md.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
design-fit:
  owner: each command's own package; domain owners unchanged
  contract: none — consumes cli-contract.md delivered by /15
  retires: [hand-rolled flag loops and usage strings]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve existing domain checks, custody decisions and exit codes; a
CLI library is not a new authority source. Independent verification owns Evidence and
lifecycle advancement. Proposed tests and files below are deliverables, not claims that
they already exist.

## Task

1. Build a Cobra command tree for `mutate`, `bugs-gc`, `freshness`, `loopresolve`, `winparity`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && { (cd loopadmin && go test -count=1 -v ./mutate -run '^TestCLIHelpOffline$'); (cd tools/bugs-gc && go test -count=1 -v . -run '^TestCLIHelpOffline$'); (cd tools/freshness && go test -count=1 -v . -run '^TestCLIHelpOffline$'); (cd tools/loopresolve && go test -count=1 -v . -run '^TestCLIHelpOffline$'); (cd tools/winparity && go test -count=1 -v . -run '^TestCLIHelpOffline$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && { (cd loopadmin && go test -count=1 -v ./mutate -run '^TestCLIConfigFlow$'); (cd tools/bugs-gc && go test -count=1 -v . -run '^TestCLIConfigFlow$'); (cd tools/freshness && go test -count=1 -v . -run '^TestCLIConfigFlow$'); (cd tools/loopresolve && go test -count=1 -v . -run '^TestCLIConfigFlow$'); (cd tools/winparity && go test -count=1 -v . -run '^TestCLIConfigFlow$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && { (cd loopadmin && go test -count=1 -v ./mutate -run '^TestCLILegacyForms$'); (cd tools/bugs-gc && go test -count=1 -v . -run '^TestCLILegacyForms$'); (cd tools/freshness && go test -count=1 -v . -run '^TestCLILegacyForms$'); (cd tools/loopresolve && go test -count=1 -v . -run '^TestCLILegacyForms$'); (cd tools/winparity && go test -count=1 -v . -run '^TestCLILegacyForms$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-48.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `(cd loopadmin && go test -count=1 ./mutate/...) && (cd tools/bugs-gc && go test -count=1 ./...) && (cd tools/freshness && go test -count=1 ./...) && (cd tools/loopresolve && go test -count=1 ./...) && (cd tools/winparity && go test -count=1 ./...)` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `(cd loopadmin && GOOS=windows go vet ./mutate) && (cd tools/bugs-gc && GOOS=windows go vet .) && (cd tools/freshness && GOOS=windows go vet .) && (cd tools/loopresolve && GOOS=windows go vet .) && (cd tools/winparity && GOOS=windows go vet .)` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/48 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
