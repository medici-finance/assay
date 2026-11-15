---
brief: "assay:assay:desktools-v2:47"
title: "Migrate the small admin verbs to Cobra and Viper"
why: >-
  The small admin verbs (deskdisable, deskmigrate, deskaudit, deskversion) each roll their own
  parser. One adapter gives them consistent help and version output.
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
id: "a7e5f18f-fc1f-4fb1-9ff5-004a8d1a3152"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/47)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskdisable: follow-up desktools-v2/47 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskmigrate: follow-up desktools-v2/47 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskaudit: follow-up desktools-v2/47 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskversion: follow-up desktools-v2/47 (command tree, declared configuration, contract tests)"
  - "tools/desk/component.yaml: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/upgrade-assay/SKILL.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/distribution.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/iso9001-mapping.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "examples/adopter-scaffold/releases/v0.28.0.yaml: follow-up desktools-v2/47 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/47 (owned rows flip to migrated)"
---

# Brief 47 — Migrate the small admin verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskdisable` — `deskdisable`; module tools/desk; parser: Go flag: FlagSet; 624 LOC, ~2 verbs, ~4 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskmigrate` — `deskmigrate`; module tools/desk; parser: Go flag: FlagSet; 242 LOC, ~0 verbs, ~6 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskaudit` — `deskaudit`; module tools/desk; parser: manual os.Args parsing; 162 LOC, ~2 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskversion` — `deskversion`; module tools/desk; parser: Go flag: FlagSet; 120 LOC, ~0 verbs, ~5 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskaudit/cli_test.go` (planned), `tools/desk/cmd/deskdisable/cli_test.go` (planned), `tools/desk/cmd/deskmigrate/cli_test.go` (planned), `tools/desk/cmd/deskversion/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-47.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-admin.md` (planned).

facts:
- deskversion's output is read by install and upgrade checks; it stays byte-compatible.
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

1. Build a Cobra command tree for `deskdisable`, `deskmigrate`, `deskaudit`, `deskversion`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-47.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests; the command compares muhar's last line itself (`n` is the spec's mutation count), because muhar exits 0 when a mutation survives or cannot be applied |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskdisable/... ./cmd/deskmigrate/... ./cmd/deskaudit/... ./cmd/deskversion/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/47 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-47.json` | pass exit=0 | sha256:2af1fb2dc8e3 | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdisable/... ./cmd/deskmigrate/... ./cmd/deskaudit/... ./cmd/deskversion/...` | pass exit=0 | sha256:7df2120e07a4 | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/47 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:12f2d6334941 | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:48c82cb1a8c9 | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:4996d4604fe0 | 2026-11-01 | assay-verifier-app[bot] @ f548ba572add (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5155 run=6f2550806dca8f728651805cc53b34b5d749b73dd3ea5d739bc171bf001b7f7c source=f548ba572add507a5624560a640e1fb69598ebfb brief=docs/streams/desktools-v2/brief-47-migrate-the-small-admin-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

**VERIFY: BLOCKED** — rows 1 to 8 observed passing; row 9 exited 0 but checked nothing, so it carries no acceptance credit. On the clean admitted tree its output (reproduced at the same digest, sha256:4996d4604fe0) ends `consumers: no brief files in the diff against f548ba572add507a5624560a640e1fb69598ebfb — nothing to corroborate`. Needed: a reviewed amendment of row 9 that names the delivery. Nothing observed fails the brief.

Supplemental host observations, no acceptance credit; each was run outside the witness:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| S4 | row 4's command, rerun to read its last line | exit 0 | `Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec declares 12 mutations | 2026-11-01 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S8 | row 8's command, rerun to read its last line | exit 0 | `seed passes=26` | 2026-11-01 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9a | row 9's command with only the Evidence edit in the tree | exit 0 | 0 corroborated, 0 disproved, 13 unchecked: the brief is in the diff but its consumers lines are unchanged since the merge-base | 2026-11-01 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9b | `statusgen --root . --consumers --delivery 0ce631327963a306078f6965a595685c8944b1e4 --base eeaa65d08810d6b9db1422b9ffb2f4bdc779cb8b --brief desktools-v2/47` | exit 0 | 13 corroborated, 0 disproved, 0 unchecked. All 13 are follow-ups naming this brief; the delivery diff touches the 4 command packages and the registry, and none of the 8 consumer documents | 2026-11-01 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-47.json` | pass exit=0 | sha256:be446fe1e1dc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdisable/... ./cmd/deskmigrate/... ./cmd/deskaudit/... ./cmd/deskversion/...` | pass exit=0 | sha256:c0da43f3c528 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/47 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:255fe463102e | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:d38ad0057184 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:aa21a4eea11a | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5632 run=a43de77f9f58bed250590ed3905a23f4e37e86c7b4bae0830ca1bb2e650c11d9 source=bc078e9179c862dde02b367d35177f8ffdf1b421 brief=docs/streams/desktools-v2/brief-47-migrate-the-small-admin-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Re-verification at bc078e9179c8 (statusgen v1.0.75, host verifyrun, all rows class check, no check:ci row). `statusgen verifyrun --check` reports 9 pass, 0 fail, 0 could-not-run/missing of 9 Verify rows. Fail-first: no row is risk-bearing (the risk block is all "no" and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag), so the fail-first base run has no row to run.

Supplemental host observations, no acceptance credit; each was run outside the witness:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| S4 | row 4's command, rerun to read its last line | exit 0 | `Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec declares 12 mutations | 2026-11-06 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S8 | row 8's command, rerun to read its last line | exit 0 | `seed passes=26` | 2026-11-06 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9a | row 9's command with only the Evidence edit in the tree | exit 0 | 0 corroborated, 0 disproved, 13 unchecked: the brief is in the diff but its consumers lines are unchanged since the merge-base | 2026-11-06 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9b | `statusgen --root . --consumers --delivery 0ce631327963a306078f6965a595685c8944b1e4 --base eeaa65d08810d6b9db1422b9ffb2f4bdc779cb8b --brief desktools-v2/47` | exit 0 | 13 corroborated, 0 disproved, 0 unchecked | 2026-11-06 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9c | row 9's command in a clean scratch clone of the admitted source bc078e9179c8 | exit 0 | output digest sha256:aa21a4eea11a, equal to the row 9 witness; last line `consumers: no brief files in the diff against bc078e9179c862dde02b367d35177f8ffdf1b421 — nothing to corroborate` | 2026-11-06 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing values, enumerated over the non-test diff of the delivering merge (#4996, 0ce631327963 against first parent eeaa65d08810) in the four command packages:

- RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5, exitcodes.go:51) @ tools/desk/cmd/deskaudit/main.go:184, tools/desk/cmd/deskdisable/main.go:190, tools/desk/cmd/deskmigrate/main.go:176, tools/desk/cmd/deskversion/main.go:178 — at the first parent each package returned deskkit.ExitRefused when its old parser refused the command line, and the ground rules require exit codes kept, so a parse refusal staying at 5 is the right value. Reversible by an edit.
- tailMax = 10000 @ tools/desk/cmd/deskaudit/main.go:213 is carried over unchanged from the first parent (line 110 there), so it is not introduced or changed by this item.

Attempts: one run of the Verify table (2026-11-06T18:11:20Z to 2026-11-06T18:13:42Z, 9 rows), credited.

**VERIFY: BLOCKED** — rows 1 to 8 observed passing; row 9 exited 0 but checked nothing at the admitted source, so it carries no acceptance credit (S9c reproduces its witness digest and ends `consumers: no brief files in the diff against bc078e9179c862dde02b367d35177f8ffdf1b421 — nothing to corroborate`). Row 9 still names no pinned delivery; its definition repair is tracked in medici-finance/assay#4560. Nothing observed fails the brief.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-47.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:8fc2a57f26d4 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdisable/... ./cmd/deskmigrate/... ./cmd/deskaudit/... ./cmd/deskversion/...` | pass exit=0 | sha256:4bb9e26bfe28 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/47 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:3754e5d58243 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:6a312bb81307 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:938a538575ae | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6028 run=290a362a8cdfe3c53b49c1efb3d3a54306894b249db7263bccd39e016c147cda source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desktools-v2/brief-47-migrate-the-small-admin-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Re-verification at 612750a12406 (statusgen v1.0.81, host verifyrun on darwin-arm64, all nine rows class check with no check:ci row, so no container bracket applies). `statusgen verifyrun --check` over this witness reports 9 pass, 0 fail, 0 could-not-run/missing of 9 Verify rows, 0 author-added. `deskpathguard rederive` reports 9 rows, 0 author-added, every row from-base; the amended row 4 (the Verify-row amendment for medici-finance/assay#5805) is the row that ran. The delivering merge is #4996 (0ce631327963, first parent eeaa65d08810) on the first-parent chain of the verified head, and the four command packages are unchanged between that merge and the verified head. Fail-first: no row is risk-bearing (the risk block is all "no" and no row's Command or Expect carries a risk-bearing, live, mutating or end-to-end tag), so the fail-first base run has no row to run.

Supplemental host observations, no acceptance credit; each was run outside the witness, in a scratch clone of 612750a12406 unless stated:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| S1 | rows 1 to 3's go test commands, rerun to read their output | exit 0 each | 4 top-level PASS lines for each of TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms, no FAIL line; 20, 12 and 20 subtest PASS lines respectively | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S4 | row 4's muhar command, rerun to read its last line | exit 0 | `Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec declares 12 mutations | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S8 | row 8's command, rerun to read its last line | exit 0 | `seed passes=26` | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9a | row 9's command, rerun in the admitted home with the brief restored to the admitted source | exit 0 | output digest sha256:938a538575ae, equal to the row 9 witness; last line `consumers: no brief files in the diff against 612750a12406f19a96a83e6ea7687d794f772f9f — nothing to corroborate` | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9b | `statusgen --root . --consumers --delivery 0ce631327963a306078f6965a595685c8944b1e4 --base eeaa65d08810d6b9db1422b9ffb2f4bdc779cb8b --brief desktools-v2/47`, in the admitted home | exit 0 | 13 corroborated, 0 disproved, 0 unchecked | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S10 | parser inventory of the four command packages' non-test sources | observed | each main.go imports cobra; none imports the flag package or builds a FlagSet | 2026-11-10 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing values, enumerated over the non-test diff of the delivering merge (#4996, 0ce631327963 against first parent eeaa65d08810) in the four command packages:

- RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5, exitcodes.go:51) @ tools/desk/cmd/deskaudit/main.go:184, tools/desk/cmd/deskdisable/main.go:190, tools/desk/cmd/deskmigrate/main.go:176, tools/desk/cmd/deskversion/main.go:178 — at the first parent each package returned deskkit.ExitRefused when its old parser refused the command line (fs.Parse error paths in deskdisable, deskmigrate and deskversion; the manual argument checks in deskaudit), and the ground rules require exit codes kept, so a parse refusal staying at 5 is the right value. Reversible by an edit.
- tailMax = 10000 @ tools/desk/cmd/deskaudit/main.go:213 is carried over unchanged from the first parent (line 110 there), so it is not introduced or changed by this item.

Attempts: one run of the Verify table (2026-11-10T10:29:32Z to 2026-11-10T10:32:17Z, 9 rows), credited.

**VERIFY: BLOCKED** — rows 1 to 8 observed passing; row 9 exited 0 but checked nothing at the admitted source, so it carries no acceptance credit (S9a reproduces its witness digest and ends `consumers: no brief files in the diff against 612750a12406f19a96a83e6ea7687d794f772f9f — nothing to corroborate`). Row 9 still names no pinned delivery; its definition repair is tracked in medici-finance/assay#4560. Nothing observed fails the brief.
### Non-implementer re-verification — 2026-11-13 at 4b7942836f5d (claude-opus-5-5)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-47.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:147c7447ed65 | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdisable/... ./cmd/deskmigrate/... ./cmd/deskaudit/... ./cmd/deskversion/...` | pass exit=0 | sha256:8166f6ba25a9 | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdisable ./cmd/deskmigrate ./cmd/deskaudit ./cmd/deskversion` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/47 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:9852fb8c9645 | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:53f36df7675d | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:40351f659864 | 2026-11-13 | assay-verifier-app[bot] @ 4b7942836f5d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6357 run=6f8fc85ba6005731d37b8ac403792ec692a80f5aa3aa0a96781a46a507e485b7 source=4b7942836f5d809de264179926543417401fdc06 brief=docs/streams/desktools-v2/brief-47-migrate-the-small-admin-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Re-verification at merged main 4b7942836f5d under desk tools v1.0.83. The pass was woken because the migration registry docs/streams/desktools-v2/cli-migration.json gained entries since the last admitted run at 612750a12406 (1457c23b9, the #5910 merge 4d9553dda, and bc4e007b3). Those entries are owned by other briefs. The four entries owned by desktools-v2/47 (deskaudit, deskdisable, deskmigrate, deskversion, each `state: migrated`) hash identically at both ends. The four command packages, the desktools-v2-47 mutation spec and the shared tools/desk/internal/cli adapter have no commit in that range. The home was detached at 4b7942836f5d and admitted by deskdispatch --check-verifier (attestation medici-finance/assay#6357). The witness table above comes from one host statusgen verifyrun over all 9 rows, 2026-11-13 02:54:07Z to 02:56:03Z, on darwin arm64 with a minimal PATH (go, git, jq and the desk tools only), no forge credential and no Kubernetes config. The home read clean afterwards. No row is a check:ci row, so no container bracket applies. deskpathguard rederive reads 9 rows, all from-base, 0 author-added. The delivering merge is #4996 (0ce631327963, first parent eeaa65d08810). It is on the first-parent chain of the verified head, and the four command packages are unchanged between that merge and the verified head. Fail-first: no row is risk-bearing (the risk block is all "no" and the row tags are +dereference, +flow and +mutation), so no fail-first witness is owed.

Supplemental host observations, no acceptance credit; each was run in the admitted home outside the witness, with the same minimal PATH:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| S1 | rows 1 to 3's go test commands, rerun to read their output | exit 0 each | 4 top-level PASS lines for each of TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms, no FAIL line; 20, 12 and 20 subtest PASS lines respectively | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S4 | row 4's muhar command, rerun to read its last line | exit 0 | `Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec declares 12 mutations | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S8 | row 8's command, rerun to read its last line | exit 0 | `seed passes=26` | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9a | row 9's command, rerun in the clean admitted home | exit 0 | output digest sha256:40351f659864, equal to the row 9 witness; last line `consumers: no brief files in the diff against 4b7942836f5d809de264179926543417401fdc06 — nothing to corroborate` | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S9b | `statusgen --root . --consumers --delivery 0ce631327963a306078f6965a595685c8944b1e4 --base eeaa65d08810d6b9db1422b9ffb2f4bdc779cb8b --brief desktools-v2/47` | exit 0 | `summary: 13 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing` | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| S10 | parser inventory of the four command packages' non-test sources | observed | each package imports cobra; none imports the flag package or builds a FlagSet | 2026-11-13 | assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Risk-bearing values were enumerated over the non-test diff of the delivering merge (#4996, 0ce631327963 against first parent eeaa65d08810) in the four command packages and re-read at 4b7942836f5d:

- RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5, tools/desk/internal/deskkit/exitcodes.go:51) @ tools/desk/cmd/deskaudit/main.go:184, tools/desk/cmd/deskdisable/main.go:190, tools/desk/cmd/deskmigrate/main.go:176, tools/desk/cmd/deskversion/main.go:178 — at the first parent each package returned deskkit.ExitRefused when its old parser refused the command line, and the ground rules require exit codes kept, so a parse refusal staying at 5 is the right value. Reversible by an edit.
- tailMax = 10000 @ tools/desk/cmd/deskaudit/main.go:213 is carried over unchanged from the first parent (line 110 there), so this item did not introduce or change it.

Attempts: one run of the Verify table (2026-11-13T02:54:07Z to 2026-11-13T02:56:03Z, 9 rows), credited.

**VERIFY: BLOCKED** — rows 1 to 8 observed passing. Row 9 exited 0 but checked nothing at the admitted source, so it carries no acceptance credit (S9a reproduces its witness digest and ends `consumers: no brief files in the diff against 4b7942836f5d809de264179926543417401fdc06 — nothing to corroborate`). Row 9 still names no pinned delivery, and its definition repair is tracked in medici-finance/assay#4560 (open, read 2026-11-13). Nothing observed fails the brief.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
