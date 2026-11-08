---
brief: "assay:assay:desktools-v2:46"
title: "Migrate the release and provenance verbs to Cobra and Viper"
why: >-
  The release and provenance commands (deskrelease, deskrebaseline, deskmanifest,
  deskprovenance, deskpins) parse arguments by hand or with ad-hoc FlagSets. Standard help and
  option forms arrive without changing manifests, pins or provenance output.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17", "desktools-v2/78", "desktools-v2/79"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "e8ed7b00-6327-415b-a8cb-efd3409f2547"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/46)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskrelease: follow-up desktools-v2/46 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskrebaseline: follow-up desktools-v2/46 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskmanifest: follow-up desktools-v2/46 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskprovenance: follow-up desktools-v2/46 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskpins: follow-up desktools-v2/46 (command tree, declared configuration, contract tests)"
  - "tools/desk/README.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/release-on-merge.yml: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "statusgen/README.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "topology.yaml: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/rebaseline.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/brief-rules.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "components/KEYS.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contributor-provenance.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/distribution.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/iso9001-mapping.md: follow-up desktools-v2/46 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/46 (owned rows flip to migrated)"
---

# Brief 46 — Migrate the release and provenance verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskrelease` — `deskrelease`; module tools/desk; parser: manual os.Args parsing; 1154 LOC, ~3 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskrebaseline` — `deskrebaseline`; module tools/desk; parser: manual os.Args parsing; 1283 LOC, ~0 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskmanifest` — `deskmanifest`; module tools/desk; parser: Go flag: FlagSet; 861 LOC, ~1 verbs, ~2 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskprovenance` — `deskprovenance`; module tools/desk; parser: Go flag: FlagSet; 268 LOC, ~0 verbs, ~7 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskpins` — `deskpins`; module tools/desk; parser: manual os.Args parsing; 115 LOC, ~0 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskmanifest/cli_test.go` (planned), `tools/desk/cmd/deskpins/cli_test.go` (planned), `tools/desk/cmd/deskprovenance/cli_test.go` (planned), `tools/desk/cmd/deskrebaseline/cli_test.go` (planned), `tools/desk/cmd/deskrelease/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-46.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-release.md` (planned).

facts:
- Release workflows call these by name; every flag spelling they use is a legacy form to keep.
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

1. Build a Cobra command tree for `deskrelease`, `deskrebaseline`, `deskmanifest`, `deskprovenance`, `deskpins`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-46.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskrelease/... ./cmd/deskrebaseline/... ./cmd/deskmanifest/... ./cmd/deskprovenance/... ./cmd/deskpins/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/46 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `o=$(statusgen --root . --consumers --brief desktools-v2/46 --base 02d17896bad026f191583de4100e783e89c01c33 --delivery 3f2b1fc589b4b7c6854a9cf84cb6745d4117c717); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | exit 0; exact `summary: 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing` — the command itself exits non-zero on any other summary or any statusgen exit but 0. All 20 entries are routed `follow-up desktools-v2/46`, this brief itself, so statusgen settles each from the stream tables (the target brief exists and refers back) by reading this brief's own file: they pass by self-reference, carry no diff corroboration, and this row gives no evidence about the 20 named paths (12 of them are untouched by the delivery); whatever evidence they have comes from rows 1–8 and review. The delivery range (`3f2b1fc5`, the merge of #4793 that delivered this brief, against its first parent `02d17896`) is pinned because the `consumer-delivery-range` lint requires it; the counts do not depend on it. #5313, a later fix, is outside this range. Record the result as 20 self-routed passes, never as diff corroboration and never as 20 corroborated routing claims. The pin guards only the summary counts: a change in the per-entry lines that keeps those counts still passes, so read the printed entries. No unavailable check is recorded as a pass: missing history or an unreadable delivery makes statusgen exit 2 (could-not-check), which this command turns into a non-zero exit. |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-46.json` | pass exit=0 | sha256:e8629fac92b5 | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskrelease/... ./cmd/deskrebaseline/... ./cmd/deskmanifest/... ./cmd/deskprovenance/... ./cmd/deskpins/...` | pass exit=0 | sha256:e5fd64e0e1c0 | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/46 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:cd934883d0fc | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:dc3f54bd891f | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:f164ff219c2c | 2026-10-31 | assay-verifier-app[bot] @ a288c581d4e1 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5085 run=5e1e65757243147186a1e70902e400303acaa5956855b957624b83e6b106678b source=a288c581d4e1f86f3b7679f3528492bf52456654 brief=docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Observed output for each row, at a288c581d4e1. The command for each row is the one in the witness table above.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1 | exit 0; TestCLIHelpOffline PASS in every package; root and nested help and version with zero effects and zero config reads | exit 0; 5 of 5 packages PASS, 25 subtests pass, 0 fail. The command passes. The nested-help obligation does not hold for one verb: the deskrelease test removes every form starting with cut from the shared help forms, so cut -h and cut --help are outside this row, and S1 shows them refused | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 | exit 0; TestCLIConfigFlow PASS in every package | exit 0; 5 of 5 packages PASS, 15 subtests pass, 0 fail | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 | exit 0; TestCLILegacyForms PASS in every package | exit 0; 5 of 5 packages PASS, 23 subtests pass, 0 fail | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 | exit 0; Totals line with N equal to the spec's mutation count | exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; "Totals: 17 caught, 0 NOT CAUGHT, 0 could-not-mutate."; the spec holds 17 mutations | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 | exit 0; existing tests still pass | exit 0; ok for all 5 packages | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 | exit 0; owned packages compile for Windows | exit 0; no output from go vet | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 | exit 0; TestCLIOwnerMigrated PASS | exit 0; "--- PASS: TestCLIOwnerMigrated (10.47s)" | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 | exit 0; every floor seed passes | exit 0; "seed passes=26" | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 | exit 0; no declared consumer routing disproved by the change | exit 0; "consumers: no brief files in the diff against a288c581d4e1f86f3b7679f3528492bf52456654 — nothing to corroborate". The row is vacuous on merged main; S2 is the run that judged the claims | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S1 | Supplemental, no acceptance credit: deskrelease built from this source outside the home, run with a throwaway HOME, as cut --help and cut -h, then help cut and --help | Task 2 and the CLI contract: verb help forms exit 0 before any config read, lock or effect | cut --help and cut -h: exit 5, 0 bytes on stdout, the config echo on stderr, "deskrelease: refused: unrecognised flag" for --help, and new files audit.jsonl and audit.lock under the throwaway HOME, with config absent, disabling and malformed. help cut and --help: exit 0, no new files | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S2 | Supplemental, no acceptance credit: statusgen --consumers --root . --brief desktools-v2/46 --base 02d17896bad026f191583de4100e783e89c01c33 --delivery 3f2b1fc589b4b7c6854a9cf84cb6745d4117c717 | exit 0; no claim disproved at the delivering merge | exit 0; "summary: 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". The tool's limit applies: an omitted consumer is invisible to it | 2026-10-31 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

The old parsers are gone from the five non-test sources: a search for flag.NewFlagSet, the flag import and flag.Parse finds none. Cut keeps its own parseCutArgs as its one parser, behind DisableFlagParsing.

The refusal is deliberate. PR medici-finance/assay#4793 declares it as Desk-decided item 3, and the README states it. The base refused these forms with exit 5 and an audit row, and the security review required either a recorded driver ruling or that refusal; the worker restored the refusal. The correctness review recorded that it departs from the brief's wording and needs the driver's acceptance. The PR carries no human comment or ruling, and neither this brief nor the CLI contract was amended. A human merged the PR, and this verifier does not read that merge as the acceptance the review asked for.

RISK-VALUE: DERIVED — UsageExit (deskrelease) = cli.ExitUsage = 2 @ tools/desk/cmd/deskrelease/main.go:250, tools/desk/internal/cli/cli.go:20 — the pre-migration dispatcher returned 2 for an unknown subcommand.
RISK-VALUE: DERIVED — UsageExit (deskpins, deskrebaseline) = deskkit.ExitRefused = 5 @ tools/desk/cmd/deskpins/main.go:129, tools/desk/cmd/deskrebaseline/main.go:195, tools/desk/internal/deskkit/exitcodes.go:51 — both pre-migration parsers returned ExitRefused on a bad command line.
RISK-VALUE: DERIVED — UsageExit (deskmanifest, deskprovenance) = 2 @ tools/desk/cmd/deskmanifest/lint.go:21, tools/desk/cmd/deskprovenance/main.go:198 — both pre-migration parsers returned 2 on a bad command line.

**VERIFY: FAIL** — all 9 Verify commands exit 0, but the row 1 and Task 2 nested-help obligation fails for deskrelease: cut --help and cut -h exit 5, read config and write an audit row. Either the driver accepts this and amends the brief and contract, or the code changes.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5327 run=ce5759efccf983ebd77c3f1203f3055bdd829d564e73768f7e4a0166b4fe5108 source=591842e462ad44578019ae88df9a871033d3711b brief=docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-46.json` | pass exit=0 | sha256:8e1060de592b | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5327 run=ce5759efccf983ebd77c3f1203f3055bdd829d564e73768f7e4a0166b4fe5108 source=591842e462ad44578019ae88df9a871033d3711b brief=docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskrelease/... ./cmd/deskrebaseline/... ./cmd/deskmanifest/... ./cmd/deskprovenance/... ./cmd/deskpins/...` | pass exit=0 | sha256:ba44729f2f2d | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/46 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:ea7807546405 | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:15c0d6f3169a | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:07bb07c2533c | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5327 run=ce5759efccf983ebd77c3f1203f3055bdd829d564e73768f7e4a0166b4fe5108 source=591842e462ad44578019ae88df9a871033d3711b brief=docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Re-verification at 591842e462ad, after medici-finance/assay#5313 (merge 9de6a9f8566e) implemented the driver's ruling at https://github.com/medici-finance/assay/issues/5095#issuecomment-6583473388. The three witness tables above are the tool-written rows for all 9 Verify rows, run by statusgen verifyrun on the host in three row selections (1-3, 4, 5-9); none of the rows is a check:ci row, so no container bracket applies. The row 4 selection ran with a per-row limit of 9m instead of the 10m default and finished in about 2m22s. Key output for each row, from a second capture of the same command at the same source:

| # | Command | Exit | Key output | Date | Runner |
|---|---------|------|------------|------|--------|
| 1 | Verify row 1 | 0 | 5 of 5 packages PASS TestCLIHelpOffline, 25 subtests pass, 0 fail; the deskrelease help forms now include cut -h and cut --help (clicontract.HelpForms of cut) under absent, malformed and unreadable roster states, with zero stderr, no child process and no file change | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 2 | Verify row 2 | 0 | 5 of 5 packages PASS TestCLIConfigFlow, 15 subtests pass, 0 fail | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 3 | Verify row 3 | 0 | 5 of 5 packages PASS TestCLILegacyForms, 24 subtests pass, 0 fail | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 4 | Verify row 4 | 0 | "Harness healthy: baseline GREEN, positive control CAUGHT." and last line "Totals: 20 caught, 0 NOT CAUGHT, 0 could-not-mutate."; the spec holds 20 mutations, including mutations 17-19 for the ruling (cut -h and cut --help refused again, cut help reading the roster first, a help word beside another word answered as help) | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 5 | Verify row 5 | 0 | ok for all 5 packages | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 6 | Verify row 6 | 0 | no output from go vet | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 7 | Verify row 7 | 0 | TestCLIOwnerMigrated PASS (15.44s) | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 8 | Verify row 8 | 0 | "seed passes=26" | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| 9 | Verify row 9 | 0 | "summary: 0 corroborated, 0 disproved, 20 unchecked"; on merged main the diff holds only this Evidence edit, so the row is vacuous; S2 judges the claims at the #5313 delivery | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| S1 | Supplemental, no acceptance credit: deskrelease built from this source outside the home (sourceSHA=591842e462ad), run under env -i with a throwaway HOME as cut --help, cut -h, help cut, --help, -h and cut v9.9.9 --help, with the roster absent, disabling (ASSAY_DESKS_ENABLED=0) and malformed | 0 for help forms | cut --help, cut -h and help cut: exit 0, 1595 bytes of cut help on stdout, 0 bytes on stderr, no new file under HOME or cwd, in all three states; --help and -h: exit 0, same. cut v9.9.9 --help (a help word beside another word): exit 5 with the config echo and an audit.jsonl/audit.lock write, the audited refusal the ruling keeps for that case | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |
| S2 | Supplemental, no acceptance credit: statusgen --consumers --root . --brief desktools-v2/46 --base ba16a0ae3716 --delivery 9de6a9f8566e | 0 | "summary: 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing"; the tool's limit applies, an omitted consumer is invisible to it | 2026-11-02 | assay-verifier-app[bot] @ 591842e462ad (on-behalf-of human:ian) (forge-identity) |

Judgement of the row 1 / Task 2 nested-help obligation against the ruling: the earlier FAIL (medici-finance/assay#5085) was cut --help and cut -h exiting 5 after a config read and an audit write. At this source both forms exit 0 as cut's help before the startup, the kill switch, any lock or any audit write (isHelpWord in the deskrelease main source, line 322, checked inside cut's RunE before gate runs), the row 1 test now holds them to that under hostile config, mutations 17 and 18 redden it when the refusal or the roster read returns, and S1 shows the built binary behaving so. The audited refusal is dropped only for those two forms alone, as the ruling accepted; a help word beside any other word stays refused with 5 and audited (mutation 19, S1). The old parsers stay gone: a search of the five packages' non-test sources for flag.NewFlagSet, the flag import and flag.Parse finds none, and the five owned registry entries read migrated.

Risk-bearing values (risk metadata present, all "no"; enumerated over the #5313 diff and the owned usage-exit bindings from the original delivery a345a06daf6c):

RISK-VALUE: DERIVED — isHelpWord arity and words = len(rest) == 1 with "-h" or "--help" @ tools/desk/cmd/deskrelease/main.go:323 — the ruling makes exactly the bare help word after cut the verb's help; any other arity keeps the cut parser's audited refusal (mutation 19 reddens a widening). Reversible by an edit and a release.
RISK-VALUE: DERIVED — UsageExit (deskrelease) = cli.ExitUsage = 2 @ tools/desk/cmd/deskrelease/main.go:252, tools/desk/internal/cli/cli.go:20 — the pre-migration dispatcher returned 2 for an unknown subcommand (a345a06daf6c parent, deskrelease main source line 164).
RISK-VALUE: DERIVED — UsageExit (deskpins, deskrebaseline) = deskkit.ExitRefused = 5 @ tools/desk/cmd/deskpins/main.go:129, tools/desk/cmd/deskrebaseline/main.go:195, tools/desk/internal/deskkit/exitcodes.go:51 — both pre-migration parsers returned ExitRefused on a bad command line.
RISK-VALUE: DERIVED — UsageExit (deskmanifest) = exitCouldNotCheck = 2 @ tools/desk/cmd/deskmanifest/main.go:144, tools/desk/cmd/deskmanifest/lint.go:21; UsageExit (deskprovenance) = 2 @ tools/desk/cmd/deskprovenance/main.go:198 — both pre-migration parsers returned 2 on a bad command line.

**VERIFY: PASS** — 9 of 9 Verify rows pass at 591842e462ad, and the row 1 / Task 2 nested-help obligation now holds for deskrelease under the driver's ruling at https://github.com/medici-finance/assay/issues/5095#issuecomment-6583473388: cut --help and cut -h exit 0 as help before any config read, lock or audit write.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-46.json` | pass exit=0 | sha256:d92e407877f5 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskrelease/... ./cmd/deskrebaseline/... ./cmd/deskmanifest/... ./cmd/deskprovenance/... ./cmd/deskpins/...` | pass exit=0 | sha256:0cea1c73bb26 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskrelease ./cmd/deskrebaseline ./cmd/deskmanifest ./cmd/deskprovenance ./cmd/deskpins` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/46 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:1c8ded26d075 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:5c48bb8c4725 | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
| 9 | `o=$(statusgen --root . --consumers --brief desktools-v2/46 --base 02d17896bad026f191583de4100e783e89c01c33 --delivery 3f2b1fc589b4b7c6854a9cf84cb6745d4117c717); s=$?; printf '%s\n' "$o"; test "$s" -eq 0 && case "$o" in *'summary: 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing'*) ;; *) exit 1;; esac` | pass exit=0 | sha256:50374019de9c | 2026-11-08 | assay-verifier-app[bot] @ 2e1ddcbd72c2 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5799 run=9b0461b112dca92828311201852e90d89ca2b6efb10c48a0dadae33c4e7d1a4c source=2e1ddcbd72c2dbe2674821bd7eac6c16c6189e0f brief=docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Re-verification at merged main 2e1ddcbd72c2 under desk tools v1.0.78, woken by the change to Verify row 9 (it now pins the delivery range and asserts the exact summary). The witness table above is one host statusgen verifyrun over all 9 rows, 2026-11-08 06:14:29Z to 06:18:05Z; no row is a check:ci row, so no container bracket applies. statusgen verifyrun --check reads 9 pass, 0 fail, 0 could-not-run/missing of 9 Verify rows. Since the last admitted run at 591842e462ad, the owned packages changed only in one comment in the deskrelease cut source; the clicontract package, the shared cli package, the muhar harness, the mutation spec and this brief's registry rows are unchanged.

| # | Command | Expected | Observed | Date | Runner |
|---|---------|----------|----------|------|--------|
| 1 | Verify row 1 | exit 0; TestCLIHelpOffline PASS in all 5 packages | exit 0; the command itself asserts 5 PASS lines | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 | exit 0; TestCLIConfigFlow PASS in all 5 packages | exit 0; the command itself asserts 5 PASS lines | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 | exit 0; TestCLILegacyForms PASS in all 5 packages | exit 0; the command itself asserts 5 PASS lines | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 | exit 0 and last line Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate. with N = 20 | exit 0, output sha256:d92e407877f5. Exit 0 shows the harness was not broken (baseline green, positive control caught). The Totals clause is not observed: muhar returns 0 whatever the NOT CAUGHT count (it returns 2 only for a broken harness), the row does not assert the line, and verifyrun records only a digest of the output | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 | exit 0; existing tests pass | exit 0 | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 | exit 0; owned packages vet for Windows | exit 0 | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 | exit 0; TestCLIOwnerMigrated PASS | exit 0; the command greps the PASS line | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 | exit 0; every floor seed passes | exit 0 | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 | exit 0; exact summary 20 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing | exit 0; the command fails on any other summary, so the summary matched. As the row says, these are 20 self-routed passes, not diff corroboration; the per-entry lines are not retained by verifyrun | 2026-11-08 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Row 4 is the one obligation without current evidence. A hand run of the row is outside this dispatch, so the Totals line was not captured. The repair is a check-definition change: make row 4 assert the Totals line itself, as other muhar rows in this repo do (for example a case match on the exact Totals text), then re-dispatch. No obligation was observed to fail.

Fail-first: no row is risk-bearing (every risk answer is "no"; the row tags are +dereference, +flow and +mutation, none of risk-bearing, live, mutating or end-to-end), so no fail-first witness is owed.

Risk-bearing values, enumerated over the diff since 591842e462ad in the owned scope (the brief's row 9 command; a comment-only edit in the deskrelease cut source) plus the usage-exit bindings the delivery introduced:

RISK-VALUE: DERIVED — row 9 summary count = 20 @ docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md:108 — the frontmatter consumers list holds exactly 20 entries, each routed to this brief.
RISK-VALUE: DERIVED — row 9 base = 02d17896bad026f191583de4100e783e89c01c33, delivery = 3f2b1fc589b4b7c6854a9cf84cb6745d4117c717 @ docs/streams/desktools-v2/brief-46-migrate-the-release-and-provenance-verbs-to-cobra-and-viper.md:108 — git shows 3f2b1fc5 is the merge of medici-finance/assay#4793 and 02d17896 is its first parent.
RISK-VALUE: DERIVED — isHelpWord = len(rest) == 1 with "-h" or "--help" @ tools/desk/cmd/deskrelease/main.go:323 — unchanged since the last run; the ruling at https://github.com/medici-finance/assay/issues/5095#issuecomment-6583473388 makes only the bare help word after cut the verb's help.
RISK-VALUE: DERIVED — UsageExit (deskrelease) = cli.ExitUsage = 2 @ tools/desk/cmd/deskrelease/main.go:252, tools/desk/internal/cli/cli.go:20 — the pre-migration dispatcher returned 2 for an unknown subcommand.
RISK-VALUE: DERIVED — UsageExit (deskpins, deskrebaseline) = deskkit.ExitRefused = 5 @ tools/desk/cmd/deskpins/main.go:129, tools/desk/cmd/deskrebaseline/main.go:195, tools/desk/internal/deskkit/exitcodes.go:51 — both pre-migration parsers returned ExitRefused on a bad command line.
RISK-VALUE: DERIVED — UsageExit (deskmanifest, deskprovenance) = 2 @ tools/desk/cmd/deskmanifest/main.go:144, tools/desk/cmd/deskmanifest/lint.go:21, tools/desk/cmd/deskprovenance/main.go:198 — both pre-migration parsers returned 2 on a bad command line.

**VERIFY: BLOCKED** — 9 of 9 Verify commands exit 0 at 2e1ddcbd72c2 and verifyrun --check credits all 9, but row 4's Totals clause (0 NOT CAUGHT of 20) is not observed: muhar exits 0 whatever the NOT CAUGHT count and the row does not assert the line. No obligation was observed to fail; the next step is a check-definition repair to row 4.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
