---
brief: "assay:assay:desktools-v2:44"
title: "Migrate the metrics and calibration verbs to Cobra and Viper"
why: >-
  The metrics commands (opmetrics, deskcalibrate, deskreconcile, muhar) mix global and
  per-verb FlagSets. Cobra gives them consistent help; their report formats stay.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "6c5b81a6-f61b-4c9e-93c7-97e7ca2b7c46"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/44)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/opmetrics: follow-up desktools-v2/44 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskcalibrate: follow-up desktools-v2/44 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskreconcile: follow-up desktools-v2/44 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/muhar: follow-up desktools-v2/44 (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/release.yml: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/release.yml: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/truth-suite.yml: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/iso9001-mapping.md: follow-up desktools-v2/44 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/44 (owned rows flip to migrated)"
---

# Brief 44 — Migrate the metrics and calibration verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/opmetrics` — `opmetrics`; module tools/desk; parser: Go flag: FlagSet; 1840 LOC, ~2 verbs, ~10 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskcalibrate` — `deskcalibrate`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 461 LOC, ~2 verbs, ~9 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskreconcile` — `deskreconcile`; module tools/desk; parser: Go flag: FlagSet; 663 LOC, ~3 verbs, ~7 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/muhar` — `muhar`; module tools/desk; parser: Go flag: global FlagSet; 695 LOC, ~0 verbs, ~3 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS; also run by release.yml gate steps.
- `tools/desk/cmd/deskcalibrate/cli_test.go` (planned), `tools/desk/cmd/deskreconcile/cli_test.go` (planned), `tools/desk/cmd/muhar/cli_test.go` (planned), `tools/desk/cmd/opmetrics/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-44.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-metrics.md` (planned).

facts:
- muhar parses the global flag set; moving it to a command-local tree removes shared global state between invocations.
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

1. Build a Cobra command tree for `opmetrics`, `deskcalibrate`, `deskreconcile`, `muhar`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-44.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/opmetrics/... ./cmd/deskcalibrate/... ./cmd/deskreconcile/... ./cmd/muhar/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/44 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-44.json` | pass exit=0 | sha256:5c307730b3dd | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 5 | `cd tools/desk && go test -count=1 ./cmd/opmetrics/... ./cmd/deskcalibrate/... ./cmd/deskreconcile/... ./cmd/muhar/...` | pass exit=0 | sha256:94264098a232 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/44 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:1c9f9947c07e | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:d9a4c08b96a7 | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:918b3210c47c | 2026-10-29 | assay-verifier-app[bot] @ 5729430698d4 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=5729430698d4 |
Verification-Attestation: medici-finance/assay#4816 run=983c187641d95ca047290f981b1537f36218fd20c64fae3359711c8bebb2121a source=5729430698d44ffd874bb25275d056f5a5404dc6 brief=docs/streams/desktools-v2/brief-44-migrate-the-metrics-and-calibration-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Non-implementer verification on merged main 5729430698d44ffd874bb25275d056f5a5404dc6. The work was delivered by #4762 (merge 6e3d42887efb, first parent cad1291a10d2 passed as the fail-first base). Gate: model. All four risk answers are no. The witness above is `statusgen verifyrun --fail-first` (statusgen v1.0.65), run on the host (darwin/arm64, go1.27.2) from the admitted detached home. Every row is class `check` (none is `check:ci`), so no network sandbox or container bracket applies. The table carries no risk-bearing row tag, so every row is not-selected at base. Rows passed: 9 of 9.

Supplemental host observation (no acceptance credit; it reads the output the witness records only as a hash, same commands at the same source, 2026-10-29, assay-verifier-app[bot]):
- Row 4 exited 0. It printed "Harness healthy: baseline GREEN, positive control CAUGHT.", 12 CAUGHT lines, and the last line "Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec declares 12 mutations (three per owned command: help after a config read, an undeclared key binding, one legacy form dropped), so N equals the spec's count.
- Row 7 printed "--- PASS: TestCLIOwnerMigrated (12.45s)". The registry rows owned by desktools-v2/44 (deskcalibrate, deskreconcile, muhar, the muhar action script, opmetrics) all read state migrated.
- Row 9 exited 0 with "summary: 0 corroborated, 0 disproved, 13 unchecked". The 13 unchecked entries are the brief's 13 consumers; the tool marks them reviewer's-call, not passes.
- Review check (old parser gone): no non-test file in the four packages imports the standard flag package or calls flag.NewFlagSet or flag.Parse.

Risk-bearing values. Risk metadata is present and every answer is no, including irreversible: no. The enumeration covers the delivering diff (the four main.go files, cli_test.go, the mutation spec and the registry), with literals located at 5729430698d4:
- opmetrics usage exit UsageExit = deskkit.ExitRefused @ tools/desk/cmd/opmetrics/main.go:160; deskcalibrate @ tools/desk/cmd/deskcalibrate/main.go:183; deskreconcile @ tools/desk/cmd/deskreconcile/main.go:157; ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51
- muhar usage exit: the adapter default ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar sets no UsageExit)
- muhar j Default: 1 @ tools/desk/cmd/muhar/main.go:266
- deskcalibrate n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126 (DefaultSampleSize = 10, not changed by this diff)
- deskcalibrate seed parse strconv.ParseInt(seed, 0, 64) @ tools/desk/cmd/deskcalibrate/main.go:244
- deskreconcile root Default: "." @ tools/desk/cmd/deskreconcile/main.go:119; issue Default: 0 @ :122; branch Default: defaultBranch @ :121
- opmetrics trend Default: 0 @ tools/desk/cmd/opmetrics/main.go:120
- muhar interrupt os.Exit(130) @ tools/desk/cmd/muhar/main.go:372 (carried over unchanged)

Every entry is a reversible caller contract: an edit and a release fix it. A wrong value would make a caller branch on the wrong integer or sample the wrong size. The usage-exit codes rank first, because scripts branch on them. The defaults rank last.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/opmetrics/main.go:160, tools/desk/cmd/deskcalibrate/main.go:183, tools/desk/cmd/deskreconcile/main.go:157 — the brief's ground rule says "preserve existing exit codes", and at base cad1291a10d2 each of the three tools returned deskkit.ExitRefused when fs.Parse failed (for example, opmetrics main.go:111-112 at base). So 5 is the value that rule fixes.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar) — at base, muhar parsed with the global flag set, whose ExitOnError mode exits 2 on a parse error. The brief's exit-code preservation rule therefore fixes muhar's parse-refusal code at 2, and the code comment at main.go:275-277 says so.
RISK-VALUE: DERIVED — j Default: 1 @ tools/desk/cmd/muhar/main.go:266; n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126; root ".", issue 0, branch defaultBranch @ tools/desk/cmd/deskreconcile/main.go:119-122; trend 0 @ tools/desk/cmd/opmetrics/main.go:120 — each equals the default the base parser declared (muhar flag.Int("j", 1, …), fs.Int("n", DefaultSampleSize, …), fs.String("root", ".", …), fs.Int("issue", 0, …), fs.String("branch", defaultBranch, …), fs.Int("trend", 0, …) at cad1291a10d2), so the migration changes none of them, which is what the brief's "weight: flags +0" and preservation rule require.

**VERIFY: PASS** — 9/9 rows pass at 5729430698d4; the row 4 harness catches all 12 mutations (supplemental read of its last line); gate model; no risk flag.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-44.json` | pass exit=0 | sha256:fdc086b565b8 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 5 | `cd tools/desk && go test -count=1 ./cmd/opmetrics/... ./cmd/deskcalibrate/... ./cmd/deskreconcile/... ./cmd/muhar/...` | pass exit=0 | sha256:fc9623d51ea9 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/44 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:f8a833377d0e | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:8a0c24061763 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:130c97d9d495 | 2026-10-30 | assay-verifier-app[bot] @ 011f14965ef2 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=011f14965ef2 |
Verification-Attestation: medici-finance/assay#5063 run=7c705adb255d66114fe6e374a8db3e2279e2effcff65d1897c037cf13d82b29d source=011f14965ef2cf173ee6be4ec3d4f319bb55ccae brief=docs/streams/desktools-v2/brief-44-migrate-the-metrics-and-calibration-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Non-implementer verification on merged main 011f14965ef2cf173ee6be4ec3d4f319bb55ccae. The work was delivered by #4762 (merge 6e3d42887efb, first parent cad1291a10d2 passed as the fail-first base). Gate: model. All four risk answers are no. The witness above is statusgen verifyrun --fail-first (statusgen v1.0.67), run on the host (darwin/arm64, go1.27.2) from the admitted detached home. No row is class check:ci, so no network sandbox or container bracket applies. The table carries no risk-bearing row tag, so every row is not-selected at base. deskpathguard rederive at the source reported 9 rows, 0 author-added, all from base. Rows passed: 9 of 9. This pass replaces the 5729430698d4 result above as the current observation: between the two sources the four owned packages received no commits, while the shared CLI packages (tools/desk/internal/cli, tools/desk/internal/clicontract) did.

Supplemental host observation (no acceptance credit; it reads what the witness records only as a hash, same commands at the same source, with HOME pointed at a scratch directory outside the home for the go rows, 2026-10-30, assay-verifier-app[bot]):
- Rows 1 to 3 each counted 4 PASS lines for their named test, one per owned package.
- Row 4 exited 0. It printed "Harness healthy: baseline GREEN, positive control CAUGHT." and the last line "Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec declares 12 mutations (three per owned command: help after a config read, an undeclared key binding, one legacy form dropped), so N equals the spec's count.
- Row 7 printed "--- PASS: TestCLIOwnerMigrated (19.54s)". The registry rows owned by desktools-v2/44 (deskcalibrate, deskreconcile, muhar, the muhar action script, opmetrics) all read state migrated.
- Row 9 exited 0 with "summary: 0 corroborated, 0 disproved, 13 unchecked, 0 brief(s) claiming nothing". The 13 unchecked entries are reviewer's-call, not passes.
- Review check (old parser gone): no non-test file in the four packages imports the standard flag package or calls flag.NewFlagSet or flag.Parse.

Risk-bearing values. Risk metadata is present and every answer is no, including irreversible: no. The enumeration covers the delivering diff (the four main.go files, cli_test.go, the mutation spec, the registry, the performance entrypoint states and the muhar action fixture), with literals located at 011f14965ef2:
- UsageExit = deskkit.ExitRefused @ tools/desk/cmd/opmetrics/main.go:160, tools/desk/cmd/deskcalibrate/main.go:183, tools/desk/cmd/deskreconcile/main.go:157; ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51
- muhar usage exit: the adapter default ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar sets no UsageExit)
- j Default: 1 @ tools/desk/cmd/muhar/main.go:266
- n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126; DefaultSampleSize = 10 @ tools/desk/cmd/deskcalibrate/calibrate.go:36 (not changed by this diff)
- seed parse strconv.ParseInt(seed, 0, 64) @ tools/desk/cmd/deskcalibrate/main.go:244
- root Default: "." @ tools/desk/cmd/deskreconcile/main.go:119; branch Default: defaultBranch @ :121; issue Default: 0 @ :122
- trend Default: 0 @ tools/desk/cmd/opmetrics/main.go:120
- muhar interrupt os.Exit(130) @ tools/desk/cmd/muhar/main.go:372 (at base main.go:296, carried over unchanged)

Every entry is a reversible caller contract: an edit and a release fix it. A wrong value would make a caller branch on the wrong integer or sample the wrong size. The usage-exit codes rank first, because scripts branch on them. The defaults rank next. The seed parse base and the interrupt code rank last and carry no derivation here.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/opmetrics/main.go:160, tools/desk/cmd/deskcalibrate/main.go:183, tools/desk/cmd/deskreconcile/main.go:157 — the brief's ground rule says to preserve existing exit codes, and at base cad1291a10d2 each tool refused a parse failure with code 5: opmetrics main.go:111-112 and deskreconcile main.go:106-111 returned deskkit.ExitRefused, and deskcalibrate main.go:115-116 returned deskkit.Refused, whose code is ExitRefused. So 5 is the value that rule fixes.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar) — at base, muhar parsed with the global flag set (main.go:213-217), whose ExitOnError mode exits 2 on a parse error. The brief's exit-code preservation rule therefore fixes muhar's parse-refusal code at 2.
RISK-VALUE: DERIVED — j Default: 1 @ tools/desk/cmd/muhar/main.go:266; n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126; root ".", branch defaultBranch, issue 0 @ tools/desk/cmd/deskreconcile/main.go:119-122; trend 0 @ tools/desk/cmd/opmetrics/main.go:120 — each equals the default the base parser declared at cad1291a10d2 (muhar flag.Int("j", 1, …) main.go:214, deskcalibrate fs.Int("n", DefaultSampleSize, …) main.go:114, deskreconcile fs.String("root", ".", …), fs.String("branch", defaultBranch, …), fs.Int("issue", 0, …) main.go:99-102, opmetrics fs.Int("trend", 0, …) main.go:106), so the migration changes none of them, which is what the brief's "weight: flags +0" and preservation rule require.

**VERIFY: PASS** — 9/9 rows pass at 011f14965ef2; the row 4 harness catches all 12 mutations (supplemental read of its last line); gate model; no risk flag.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-44.json` | pass exit=0 | sha256:87aba2e22659 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 5 | `cd tools/desk && go test -count=1 ./cmd/opmetrics/... ./cmd/deskcalibrate/... ./cmd/deskreconcile/... ./cmd/muhar/...` | pass exit=0 | sha256:84585af4a411 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/opmetrics ./cmd/deskcalibrate ./cmd/deskreconcile ./cmd/muhar` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/44 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:c9269c133575 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:5096bed17453 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:7dcbe594ced3 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=40a7aace82c8 |
Verification-Attestation: medici-finance/assay#5175 run=ab9665d82572c46de50a083ea39a2eec5061176d6902a1e247550f0e6f331b17 source=40a7aace82c8601e6f4042476cb32556a3a12dfc brief=docs/streams/desktools-v2/brief-44-migrate-the-metrics-and-calibration-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Non-implementer verification on merged main 40a7aace82c8601e6f4042476cb32556a3a12dfc, admitted by deskdispatch --check-verifier (attestation medici-finance/assay#5175). The work was delivered by #4762 (merge 6e3d42887efb, first parent cad1291a10d2 passed as the fail-first base). Gate: model. All four risk answers are no. The witness above is statusgen verifyrun --fail-first (statusgen v1.0.67), run on the host (darwin/arm64, go1.27.2) from the admitted detached home. No row is class check:ci, so no network sandbox or container bracket applies. The table carries no risk-bearing row tag, so every row is not-selected at base. deskpathguard rederive at the source reported 9 rows, 0 author-added; the Verify section is byte-identical at cad1291a10d2, at the delivery merge and at the source, and #4762 carries no wrote-to-the-test label. Rows passed: 9 of 9. Between 6e3d42887efb and the source the four owned packages received no commits. This pass is the current observation and replaces the 011f14965ef2 result above.

Supplemental host observation (no acceptance credit; a second run of the same commands at the same source, 2026-11-01, assay-verifier-app[bot], read for the output the witness records only as a hash):
- Rows 1 to 3 each exited 0 and counted 4 PASS lines for their named test, one per owned package.
- Row 4 exited 0. It printed "Harness healthy: baseline GREEN, positive control CAUGHT." and the last line "Totals: 12 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec at the source declares 12 mutations (three per owned command: help after a config read, an undeclared key binding, one legacy form dropped), so N equals the spec's count.
- Row 5 exited 0 with ok for opmetrics, deskcalibrate, deskreconcile and muhar.
- Row 7 printed "--- PASS: TestCLIOwnerMigrated (19.25s)". The registry rows owned by desktools-v2/44 (deskcalibrate, deskreconcile, muhar, the muhar action script, opmetrics) all read state migrated at the source.
- Row 8 exited 0 and printed "seed passes=26".
- Row 9 exited 0 and printed "consumers: no brief files in the diff against 40a7aace82c8601e6f4042476cb32556a3a12dfc — nothing to corroborate". On merged main the diff against the merge-base is empty, so this row disproves nothing and corroborates nothing; the 13 declared consumers stay reviewer's call.
- Review check (old parser gone): no non-test file in the four packages imports the standard flag package or calls flag.NewFlagSet or flag.Parse.

Risk-bearing values. Risk metadata is present and every answer is no, including irreversible: no. The enumeration covers the delivering diff (the four main.go files, the cli_test.go and gitquiet_test.go files, the mutation spec, the registry, the performance entrypoint states, the changelog and the muhar action fixture), with literals located at 40a7aace82c8:
- UsageExit = deskkit.ExitRefused @ tools/desk/cmd/opmetrics/main.go:160, tools/desk/cmd/deskcalibrate/main.go:183, tools/desk/cmd/deskreconcile/main.go:157; ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51
- muhar usage exit: the adapter default ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar sets no UsageExit)
- muhar resolve-failure exit exitErr(1) @ tools/desk/cmd/muhar/main.go:291
- j Default: 1 @ tools/desk/cmd/muhar/main.go:266
- n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126; DefaultSampleSize = 10 @ tools/desk/cmd/deskcalibrate/calibrate.go:36 (not changed by this diff)
- seed parse strconv.ParseInt(seed, 0, 64) @ tools/desk/cmd/deskcalibrate/main.go:244
- root Default: "." @ tools/desk/cmd/deskreconcile/main.go:119; branch Default: defaultBranch @ :121; issue Default: 0 @ :122; dry-run Default: false @ :125
- trend Default: 0 @ tools/desk/cmd/opmetrics/main.go:120; stdout Default: false @ :123
- select Default: false @ tools/desk/cmd/muhar/main.go:270
- muhar interrupt os.Exit(130) @ tools/desk/cmd/muhar/main.go:372 (carried over unchanged from base)

Every entry is a reversible caller contract: an edit and a release fix it. A wrong value would make a caller branch on the wrong integer or sample the wrong size. The usage and refusal exit codes rank first, because scripts branch on them. The flag defaults rank next. The seed parse base and the interrupt code rank last.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/opmetrics/main.go:160, tools/desk/cmd/deskcalibrate/main.go:183, tools/desk/cmd/deskreconcile/main.go:157 — the brief's ground rule says to preserve existing exit codes, and at base cad1291a10d2 each tool refused a parse failure with code 5: opmetrics main.go:111-112 and deskreconcile main.go:106-111 returned deskkit.ExitRefused, and deskcalibrate main.go:115-116 returned deskkit.Refused, whose code is ExitRefused (exitcodes.go:172). So 5 is the value that rule fixes.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (muhar), exitErr(1) @ tools/desk/cmd/muhar/main.go:291 — at base, muhar parsed with the global flag set (main.go:213-217), whose ExitOnError mode exits 2 on a parse error, and it exited 1 when -spec was missing (main.go:219-221). The preservation rule fixes 2 for a parse refusal and 1 for a missing required setting, and the head keeps both.
RISK-VALUE: DERIVED — j Default: 1 @ tools/desk/cmd/muhar/main.go:266; select false @ :270; n Default: DefaultSampleSize @ tools/desk/cmd/deskcalibrate/main.go:126; root ".", branch defaultBranch, issue 0, dry-run false @ tools/desk/cmd/deskreconcile/main.go:119-125; trend 0, stdout false @ tools/desk/cmd/opmetrics/main.go:120-123 — each equals the default the base parser declared at cad1291a10d2 (muhar flag.Int("j", 1, …) and flag.Bool("select", false, …) main.go:214-216; deskcalibrate fs.Int("n", DefaultSampleSize, …) main.go:114; deskreconcile fs.String("root", ".", …), fs.String("branch", defaultBranch, …), fs.Int("issue", 0, …), fs.Bool("dry-run", false, …) main.go:99-105; opmetrics fs.Int("trend", 0, …) and fs.Bool("stdout", false, …) main.go:106-109), so the migration changes none of them, which is what the brief's "weight: flags +0" and preservation rule require.
RISK-VALUE: DERIVED — seed parse strconv.ParseInt(seed, 0, 64) @ tools/desk/cmd/deskcalibrate/main.go:244 — at base the seed was fs.Int64("seed", 0, …) (main.go:107), and the standard flag package's Int64 value parses with strconv.ParseInt(s, 0, 64); base 0 and 64 bits keep the accepted spellings (decimal, 0x, 0o, 0b) identical.

**VERIFY: PASS** — 9/9 rows pass at 40a7aace82c8; the row 4 harness catches all 12 mutations (supplemental read of its last line); gate model; no risk flag.

Desk note (2026-11-01): the implemented→verified flip is held. A simulated flip in a scratch clone adds one statusgen lint PROBLEM, `[consumer-delivery-range]` on Verify row 9: the unpinned `--consumers` form has no explicit `--base`. That is a check-definition blocker tracked at medici-finance/assay#1927. The outcome is recorded as blocked (check-definition), and the row stays implemented until the row is re-authored.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
