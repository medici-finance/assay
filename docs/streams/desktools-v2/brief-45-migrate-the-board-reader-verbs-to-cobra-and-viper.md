---
brief: "assay:assay:desktools-v2:45"
title: "Migrate the board reader verbs to Cobra and Viper"
why: >-
  deskinbox, issueboard and deskread are the board readers; all three parse os.Args by hand.
  Cobra gives them real help; their machine output stays byte-compatible for the desks that
  parse it.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17", "desktools-v2/29"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "348c1486-fc38-4c85-b7a0-a285f9197ea7"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/45)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskinbox: follow-up desktools-v2/45 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/issueboard: follow-up desktools-v2/45 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskread: follow-up desktools-v2/45 (command tree, declared configuration, contract tests)"
  - "plugins/assay/commands/inbox.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/ask-decision/SKILL.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/scripts/assay-inbox.sh: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "topology.yaml: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/statusgen-lint-reach.md: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "statusgen/bench/lintbench.sh: follow-up desktools-v2/45 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/45 (owned rows flip to migrated)"
---

# Brief 45 — Migrate the board reader verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskinbox` — `deskinbox`; module tools/desk; parser: manual os.Args parsing; 3009 LOC, ~7 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/issueboard` — `issueboard`; module tools/desk; parser: manual os.Args parsing; 1314 LOC, ~4 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskread` — `deskread`; module tools/desk; parser: manual os.Args parsing; 605 LOC, ~2 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskinbox/cli_test.go` (planned), `tools/desk/cmd/deskread/cli_test.go` (planned), `tools/desk/cmd/issueboard/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-45.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-readers.md` (planned).

facts:
- desktools-v2/29 builds the assay-inbox launcher's forms on deskinbox after this brief lands.
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

1. Build a Cobra command tree for `deskinbox`, `issueboard`, `deskread`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 3` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 3` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 3` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-45.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskinbox/... ./cmd/issueboard/... ./cmd/deskread/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/45 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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

### Verifier pass — 2026-10-16, assay-verifier-app[bot] (verify-desk, non-implementer), merged main 1948dc087

Delivering change: 186d262f6 (squash commit carrying the `Brief: desktools-v2/45` trailer, #2978). No later commit on main touches the three owned packages. Host: darwin, go1.27.2, git 2.56.0, statusgen v1.0.45. Each row ran from the repo root of a detached checkout at 1948dc087, with KUBECONFIG=/dev/null, a throwaway HOME and TMPDIR outside the checkout, and the real Go caches. Before running anything I wrote down what each Expect cell predicts. For row 4 that meant counting the plants the brief names, 3 owned commands × 3 plants = 9. The spec at 1948dc087 holds exactly those 9 mutations plus one positive control.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLIHelpOffline` in deskinbox, issueboard and deskread, each package `ok` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | Verify row 2 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLIConfigFlow` in all three packages, each `ok` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | Verify row 3 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLILegacyForms` in all three packages, each `ok` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | Verify row 4 | exit 0; last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` then 9 CAUGHT (help-after-config-read, undeclared env binding and dropped legacy form, for each of issueboard, deskinbox and deskread); last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` The harness restored the tree in place afterwards, and git status was clean | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | Verify row 5 | exit 0 | exit 0; `ok` for deskinbox, issueboard and deskread | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | Verify row 6 | exit 0 | exit 0; no vet output for GOOS=windows | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | Verify row 7 | exit 0; PASS | exit 0; `--- PASS: TestCLIOwnerMigrated (14.02s)`. The test skips only when CLI_OWNER is unset, and the row sets it. The registry has three rows owned by desktools-v2/45 (deskinbox, deskread, issueboard), all `migrated` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | Verify row 8 | exit 0 | exit 0; last line `seed passes=26` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | Verify row 9 | exit 0; none disproved | exit 0; `consumers: no brief files in the diff against 1948dc0873705930b202f0e747047df844010d1a — nothing to corroborate`. At merged main the diff is empty, so this row proves only that nothing is disproved. Supplementary run in a throwaway clone at 186d262f6 with the brief's authoring parent as base: exit 0, `summary: 15 corroborated, 0 disproved, 0 unchecked`. That span also contains unrelated commits, so it is weak support. Against the delivering commit's own parent, the brief file is outside the diff, and statusgen reports could-not-check (exit 2) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Review-gate reading (the brief's Review section): main.go in all three packages now passes os.Args only into execute, which builds a cli.Run (Cobra) tree. The hand-rolled loops have no remaining `case "--` arms or flag.Parse in non-test source.

Risk-bearing value. The brief carries risk metadata, all four fields "no", and is not irreversible. The tools are read-only board readers. I enumerated anyway, over the main.go diffs of the three owned packages in 186d262f6:
- `UsageExit = deskkit.ExitRefused` (ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51) @ tools/desk/cmd/deskinbox/main.go:199, tools/desk/cmd/issueboard/main.go:205, tools/desk/cmd/deskread/main.go:316. If this is wrong, callers that branch on the exit code misread a bad-argument refusal. An edit and a release undo it.
- `max-parallel Default = "6"` @ tools/desk/cmd/deskread/main.go:270. Reversible concurrency knob, unchanged from the pre-change `readOpts{maxParallel: 6}`.
- `sla-days Default = strconv.Itoa(escalateSLADays)`, escalateSLADays = 6 @ tools/desk/cmd/issueboard/board.go:91. Reversible alarm threshold, pre-existing constant, not changed by this diff.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/issueboard/main.go:205 (same in deskinbox main.go:199 and deskread main.go:316) — the Ground rules require existing exit codes to be preserved. Before the change, every bad-argument path in all three parsers returned deskkit.ExitRefused, and the deskkit contract defines that as 5, "refused". The issueboard help text documents "5 refused (bad arguments)". The muhar control (5 → 6) is caught, so the tests pin this value.

Verification-Attestation: medici-finance/assay#3131 run=e776bd95590d87e3541cb5b48da80152d3e404adde9dcb3c153b496e2deb5173 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/desktools-v2/brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=any

**VERIFY: PASS** — 9/9 rows pass at merged main 1948dc087.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-45.json` | pass exit=0 | sha256:b7d0c1bbc0e7 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskinbox/... ./cmd/issueboard/... ./cmd/deskread/...` | pass exit=0 | sha256:d18d5f5025d7 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/45 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:c9fedf29cdf4 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:fcf4457a8b93 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:6ae32eba5297 | 2026-10-16 | assay-verifier-app[bot] @ 1948dc087370+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3131 run=e776bd95590d87e3541cb5b48da80152d3e404adde9dcb3c153b496e2deb5173 source=1948dc0873705930b202f0e747047df844010d1a brief=docs/streams/desktools-v2/brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=any

### Verifier pass — 2026-10-18, assay-verifier-app[bot] (verify-desk, non-implementer, strong tier), merged main 6dfb616ef

Re-verification at merged main 6dfb616ef. The delivering change is still 186d262f6 (#2978). Since the 2026-10-16 pass, three later commits touched the owned packages: #2541 (issue label history in issueboard and deskread), #3331 (test oracle) and #3332 (perf attribution in main.go). None of them changes the Cobra tree, the declared keys or the mutation spec. Host: darwin, go1.27.2, git 2.56.0, statusgen v1.0.49. Each row ran from the repo root of the detached home at 6dfb616ef, with KUBECONFIG=/dev/null, a throwaway HOME and TMPDIR in the session scratch outside the home, and the real Go caches. Before running anything I wrote down what each Expect cell predicts. Row 4's spec holds 9 mutations (3 owned commands × 3 plants) plus one positive control.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLIHelpOffline` in deskinbox (2.19s), issueboard (2.12s) and deskread (3.47s), each package `ok` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | Verify row 2 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLIConfigFlow` in all three packages, each `ok` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | Verify row 3 | exit 0; 3 PASS lines | exit 0; count 3 — `--- PASS: TestCLILegacyForms` in all three packages, each `ok` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | Verify row 4 | exit 0; last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.` then 9 CAUGHT (help after a config read, an undeclared env binding and a dropped legacy form, for each of issueboard, deskinbox and deskread); last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` git status was clean afterwards | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | Verify row 5 | exit 0 | exit 0; `ok` for deskinbox, issueboard and deskread | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | Verify row 6 | exit 0 | exit 0; no vet output for GOOS=windows | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | Verify row 7 | exit 0; PASS | exit 0; `--- PASS: TestCLIOwnerMigrated (16.43s)`, clicontract `ok`. The registry has three rows owned by desktools-v2/45 (deskinbox, deskread, issueboard), all in state `migrated` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | Verify row 8 | exit 0 | exit 0; last line `seed passes=26` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | Verify row 9 | exit 0; none disproved | exit 0; `consumers: no brief files in the diff against 6dfb616ef5463fa886e89ab7b07803d30006c58a — nothing to corroborate`. At merged main the diff is empty, so this row shows only that nothing is disproved. Supplementary run with base 186d262f6^ and the filter for desktools-v2/45: exit 0, `summary: 0 corroborated, 0 disproved, 15 unchecked`. Every entry is unchecked because no consumer line changed since that base, so it adds no positive evidence | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Review-gate reading (the brief's Review section): in all three packages the non-test source now reads os.Args only to pass it into deskkit.PerfRun and then execute, which builds the cli.Run (Cobra) tree. No flag.Parse, flag.NewFlagSet or `case "--` arm is left in non-test source.

Risk-bearing value. The brief has risk metadata, with all four fields "no" and irreversible "no". The tools are read-only board readers. I enumerated anyway, over the main.go diffs of the three owned packages in 186d262f6, and re-resolved line numbers at 6dfb616ef:
- `UsageExit = deskkit.ExitRefused` (ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51) @ tools/desk/cmd/deskinbox/main.go:199, tools/desk/cmd/issueboard/main.go:209, tools/desk/cmd/deskread/main.go:323. If this is wrong, callers that branch on the exit code misread a bad-argument refusal. An edit and a release undo it.
- execute-failure return `deskkit.ExitUnverifiable` (= 6 @ tools/desk/internal/deskkit/exitcodes.go:55), in each main.go. This is the same value the pre-change parsers returned (for example, issueboard main.go:109 at 186d262f6^). Reversible.
- `max-parallel Default = "6"` @ tools/desk/cmd/deskread/main.go:277, with lower bound `n < 1` refused @ tools/desk/cmd/deskread/main.go:563. Both are unchanged from the pre-change `readOpts{maxParallel: 6}` and its `n < 1` check. Reversible concurrency knob.
- `sla-days Default = strconv.Itoa(escalateSLADays)`, with escalateSLADays = 6 @ tools/desk/cmd/issueboard/board.go:91. This alarm threshold already existed and this diff did not change it. Reversible.
- `root Default = "."` (issueboard persistent flag). This is the same default as before. Reversible.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/issueboard/main.go:209 (same in deskinbox main.go:199 and deskread main.go:323) — the Ground rules require existing exit codes to be preserved. Before the change, every bad-argument path in the three parsers returned deskkit.ExitRefused, and the deskkit contract defines that as 5, "refused". The issueboard help text at main.go:85 documents "5 refused (bad arguments)". The muhar positive control (5 → 6) was CAUGHT in row 4, so the owner tests pin this value.

Verification-Attestation: medici-finance/assay#3394 run=2c511c0bd0c1674ba2cbae721ec7a3e650d621670d4bfb3565c40549ea196fa6 source=6dfb616ef5463fa886e89ab7b07803d30006c58a brief=docs/streams/desktools-v2/brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 9/9 rows pass at merged main 6dfb616ef.

### Verifier re-pass — 2026-10-19, assay-verifier-app[bot] (verify-desk, non-implementer), merged main 50bef70d5

Supersedes the "Verifier pass — 2026-10-18 … merged main 6dfb616ef" block above (attestation medici-finance/assay#3394, source 6dfb616ef). Correction (review finding F-witness-at-source on medici-finance/assay#3403): that block has no `statusgen verifyrun` witness rows of its own. The claim, in the PR description and the outcome-record note, that all 9 Verify rows carry passing execution witnesses for the 2026-10-18 pass is wrong. The only witness table on the branch is the 2026-10-16 one, at 1948dc087370+dirty under the tier=any attestation medici-finance/assay#3131. This block carries its own witness: a non-dry `statusgen verifyrun` run at the clean attested source 50bef70d5. Its 9 rows, dated 2026-10-19 at runner stamp 50bef70d518d with no +dirty, follow the verdict line below, together with its strong-tier attestation. The hand rows below re-run every Verify row at 50bef70d5 and copy nothing from either earlier block.

Delivering change: 186d262f6 (the `Brief: desktools-v2/45` trailer). Since the earlier pass at 1948dc087, three commits on main touched the owned packages: eded7ba6e (perf verb attribution in issueboard main.go), e28201023 (test fixtures) and 5e8a17073 (a new deskread `label-events` kind plus issueboard label-history code). None of them reintroduces a hand-rolled parser. Host: darwin, go1.27.2, git 2.56.0, statusgen v1.0.50. Each row ran from the repo root of the detached admitted home at 50bef70d5, with KUBECONFIG=/dev/null, a throwaway HOME and TMPDIR outside the home, and the real Go caches. git status --porcelain --ignored in the home was empty after every row, the in-place mutation harness of row 4 included.

| # | Command | Expected | Observed | Date / runner |
|---|---------|----------|----------|---------------|
| 1 | Verify row 1 | exit 0; 3 PASS lines | exit 0; count 3. `--- PASS: TestCLIHelpOffline` in deskinbox (2.62s), issueboard (2.67s) and deskread (3.82s), each package `ok` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 | exit 0; 3 PASS lines | exit 0; count 3. `--- PASS: TestCLIConfigFlow` in all three packages, each `ok` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 | exit 0; 3 PASS lines | exit 0; count 3. `--- PASS: TestCLILegacyForms` in all three packages, each `ok` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 | exit 0; last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` (the spec at 50bef70d5 holds 9 mutations, 3 per owned command, plus one control) | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`, then 9 CAUGHT (help after a config read, an undeclared env binding and one dropped legacy form, for each of issueboard, deskinbox and deskread); last line `Totals: 9 caught, 0 NOT CAUGHT, 0 could-not-mutate.` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 | exit 0 | exit 0; `ok` for deskinbox (9.29s), issueboard (9.98s) and deskread (19.69s), which includes the label-events tests added after the earlier pass | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 | exit 0 | exit 0; zero bytes of vet output for GOOS=windows | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | Verify row 7 | exit 0; PASS | exit 0; `--- PASS: TestCLIOwnerMigrated (34.85s)`. CLI_OWNER is set by the row, so the test ran rather than skipped. The registry holds three entrypoints owned by desktools-v2/45 (deskinbox, deskread, issueboard), each with state `migrated` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | Verify row 8 | exit 0 | exit 0; last line `seed passes=26`, with no FAIL line in the output | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | Verify row 9 | exit 0; none disproved | exit 0; `consumers: no brief files in the diff against 50bef70d518db5d73a6a2defd956f7bfcf346024 — nothing to corroborate`. At merged main the diff is empty, so this proves only that nothing is disproved. Supplementary run in the same home, `statusgen --root . --consumers --brief desktools-v2/45 --delivery 186d262f6 (full SHA) --base e29b1335d (its first parent)`: exit 0, `summary: 15 corroborated, 0 disproved, 0 unchecked`, all 15 declared consumers CORROBORATED against the immutable delivered snapshot | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Review-gate reading (the brief's Review section), at 50bef70d5: the non-test source of all three packages has no flag.Parse, flag.NewFlagSet or `case "--` / `case "-` arms. Each main.go builds a cli.Run tree with `UsageExit: deskkit.ExitRefused`. The later deskread `label-events` kind is a new value of the existing `--kind` flag. It adds no parser.

Risk-bearing value. The risk metadata is present and all four fields read "no", and the brief is not irreversible. The owned tools are read-only board readers. I enumerated anyway, over the three owned main.go files at 50bef70d5 and the 1948dc087..50bef70d5 diff of the owned packages:
- `UsageExit = deskkit.ExitRefused` @ tools/desk/cmd/deskinbox/main.go:199, tools/desk/cmd/issueboard/main.go:209, tools/desk/cmd/deskread/main.go:323, with `ExitRefused = 5` @ tools/desk/internal/deskkit/exitcodes.go:51. If this is wrong, callers that branch on the exit code misread a bad-argument refusal. An edit and a release undo it.
- `max-parallel Default = "6"` @ tools/desk/cmd/deskread/main.go:277. A reversible concurrency knob, unchanged in value from the earlier pass.
- `escalateSLADays = 6` @ tools/desk/cmd/issueboard/board.go:91. A reversible alarm threshold, a pre-existing constant this brief did not introduce.
- The later diff adds `perfVerbs` (a verb-name list for perf attribution) and a `label-events` kind entry. Neither is a numeric bound or an authority binding.

RISK-VALUE: DERIVED — UsageExit = deskkit.ExitRefused (5) @ tools/desk/cmd/issueboard/main.go:209 (same at deskinbox main.go:199 and deskread main.go:323; ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51) — the Ground rules require existing exit codes to be kept. Before the change, every bad-argument path in all three hand-rolled parsers returned deskkit.ExitRefused, which the deskkit exit-code contract defines as 5, "refused". The value is unchanged at 50bef70d5, and the row-4 harness's positive control (5 → 6) is caught, so the owner tests pin it.

Verification-Attestation: medici-finance/assay#3448 run=e2d4aef58595a9d903ff553ddff4de64afd9165eb22cf5070418b7db87b685ac source=50bef70d518db5d73a6a2defd956f7bfcf346024 brief=docs/streams/desktools-v2/brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

**VERIFY: PASS** — 9/9 rows pass at merged main 50bef70d5. Row 9 runs `statusgen --consumers` unpinned (no --brief, --delivery or --base), so that row alone shows only that nothing is disproved. The pinned supplementary run above (delivery 186d262f6, first parent e29b1335d) corroborates all 15 consumers. The rows pass, but closure is blocked on row 9's definition (class check-definition). With the stream row at verified, `statusgen --lint` raises PROBLEM [consumer-delivery-range] on row 9 (the rule from medici-finance/assay#3428). The brief's author lane must pin row 9 to `--brief desktools-v2/45 --delivery 186d262f60d60ffe80cb13f2f660b90cc76eab71 --base e29b1335dfa9cf24735fe48e485fcb74ef3eb419` before this block can carry an implemented → verified flip.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 3` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-45.json` | pass exit=0 | sha256:123c8cca9f9d | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskinbox/... ./cmd/issueboard/... ./cmd/deskread/...` | pass exit=0 | sha256:a300842ce071 | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskinbox ./cmd/issueboard ./cmd/deskread` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/45 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:6f03af26a2ef | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:aea4f54fa8bd | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:3ca1d5adb9a1 | 2026-10-19 | assay-verifier-app[bot] @ 50bef70d518d (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3448 run=e2d4aef58595a9d903ff553ddff4de64afd9165eb22cf5070418b7db87b685ac source=50bef70d518db5d73a6a2defd956f7bfcf346024 brief=docs/streams/desktools-v2/brief-45-migrate-the-board-reader-verbs-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
