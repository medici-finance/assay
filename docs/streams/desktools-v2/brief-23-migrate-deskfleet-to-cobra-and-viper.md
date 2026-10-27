---
brief: "assay:assay:desktools-v2:23"
title: "Migrate deskfleet to Cobra and Viper"
why: >-
  deskfleet's verbs hand-dispatch FlagSets. Cobra gives each verb standard help and option
  forms while fleet output and exit codes stay the same.
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
id: "314a1bbe-f498-4396-9ada-bb0cbd377cce"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/23)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskfleet: follow-up desktools-v2/23 (command tree, declared configuration, contract tests)"
  - "plugins/assay/skills/install/SKILL.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/internal/regression/MANIFEST.md: follow-up desktools-v2/23 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/23 (owned rows flip to migrated)"
---

# Brief 23 — Migrate deskfleet to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskfleet` — `deskfleet`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 1980 LOC, ~8 verbs, ~15 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskfleet/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-23.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-fleet.md` (planned).

facts:
- deskfleet reads fleet configuration; only declared keys bind through Viper.
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

1. Build a Cobra command tree for `deskfleet`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-23.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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
### Independent verification — 2026-10-26 (run 1, source f2e232039, host)

Runner: assay-verifier-app[bot], model claude-opus-5-5, admitted tier strong; independent non-implementer, fresh context. Observed source: merged main f2e232039c99455b0b9a7dacfe639f9f68b164d2, detached home admitted by `deskdispatch --check-verifier` (medici-finance/assay#4555).

Delivery: the work landed in the merge of medici-finance/assay#4500 (3f15da841717fa597b7473eb8c0efbbd46e594c3; first parent 90da708cb3a192d0fd32b7b85185c0c3fead5392), an ancestor of the observed source.

Runner class: host, darwin-arm64, go1.27.2, statusgen v1.0.63. Every row is class check (none is check:ci), so no network-off sandbox applies and the rows ran on the host through `statusgen verifyrun`. The brief's risk answers are all "no" and no row carries a risk-bearing, live, mutating or end-to-end tag, so no row owes a fail-first witness. Row 4's harness mutates the owned source in place and restores it; the home was confirmed clean after the run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskfleet -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-23.json` | pass exit=0 | sha256:5336c78b84d1 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskfleet/...` | pass exit=0 | sha256:f4805e5aea45 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskfleet` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/23 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:da17a9e0b20a | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:f18a505d9ed3 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:2ec831d2fa39 | 2026-10-26 | assay-verifier-app[bot] @ f2e232039c99 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4555 run=bf50e3bd21fdcbdfa6cc07a228b4005e503cc8070139639bb1a7e5fe8fd8a687 source=f2e232039c99455b0b9a7dacfe639f9f68b164d2 brief=docs/streams/desktools-v2/brief-23-migrate-deskfleet-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Supplemental observations — diagnostics only, no acceptance credit for any row. The same commands re-run on the same source with their output kept, to name the key line behind each digest above.

| Probe | Command or check | Observed | Date | Runner |
|---|---|---|---|---|
| S1 | rows 1–3 test commands, output kept | --- PASS: TestCLIHelpOffline (2.86s); --- PASS: TestCLIConfigFlow (0.05s); --- PASS: TestCLILegacyForms (0.02s) | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S2 | row 4 command, output kept | exit 0; "Harness healthy: baseline GREEN, positive control CAUGHT."; last line "Totals: 11 caught, 0 NOT CAUGHT, 0 could-not-mutate." — 11 equals the spec's mutation count | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S3 | row 5 command, output kept | exit 0; ok for the deskfleet package | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S4 | row 7 command, output kept | exit 0; --- PASS: TestCLIOwnerMigrated (3.75s) | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S5 | row 8 command, output kept | exit 0; last line "seed passes=26" | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S6 | row 9 command, output kept | exit 0; "no brief files in the diff against f2e232039c99… — nothing to corroborate": on merged main the default three-dot diff is empty, so this row is green without judging anything | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S7 | statusgen --root . --consumers --brief desktools-v2/23 pinned to the delivery merge (base its first parent) | exit 0; "summary: 7 corroborated, 0 disproved, 0 unchecked"; all seven declared consumers corroborated against the delivered snapshot | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |
| S8 | source read of the deskfleet package at the observed source | no import of the standard flag package and no FlagSet left in non-test source; parsing goes through the Cobra command tree in cli.go only | 2026-10-26 | assay-verifier-app[bot] (on-behalf-of human:ian) |

Row 9 as written is non-discriminating after merge (S6). It is not a risk-bearing row and its Expect (exit 0, nothing disproved) is met; S7 shows the consumer routing holds when judged against the delivery itself.

Risk-bearing values. The brief's risk answers are all "no", gate model, not irreversible, and no risk-classed path is touched. Enumeration over the delivery diff's non-test Go source (cli.go, main.go, provision.go, labels.go, avatars.go of the deskfleet command) found these literals:

- RISK-VALUE: DERIVED — pat-expiry-days Default = 7 @ tools/desk/cmd/deskfleet/cli.go:136 — the brief's design-fit allows "flags +0" and Task 4 requires existing behaviour kept; the replaced Go-flag parser declared the same default 7 (provision.go:55 at the delivery's first parent), the 1–365 bound at provision.go:72 is unchanged, and the legacy-forms test pins the default to 7.
- RISK-VALUE: DERIVED — exitUsage = 2 @ tools/desk/cmd/deskfleet/main.go:90, wired as UsageExit @ tools/desk/cmd/deskfleet/cli.go:252 — the brief requires exit codes preserved ("refusals +0"); the command's documented contract is "2 usage", the replaced parser returned exitUsage on every parse error, and row 4's positive control (UsageExit changed to 3) is caught.

Verdict accounting: 9 required rows; 9 pass; 0 fail; 0 could-not-run. No risk-bearing row, so no fail-first witness is owed.

**VERIFY: PASS** — 9/9 rows; mutation harness 11/11 caught (supplemental).

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
