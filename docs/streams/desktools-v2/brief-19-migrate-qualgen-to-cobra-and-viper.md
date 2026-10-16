---
brief: "assay:assay:desktools-v2:19"
title: "Migrate qualgen to Cobra and Viper"
why: >-
  qualgen's verbs (mine, report, pr, check, sweep, init) each hand-roll a FlagSet and a usage
  string. Moving them onto Cobra gives consistent help and option handling for the quality
  reports without changing their output files or exit codes.
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
id: "d0adfaf6-22bb-4e59-a177-3d27d14ecd30"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/19)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "qualgen: follow-up desktools-v2/19 (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/assay-qualgen.yml: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/ci-load/activation/assay-qualgen.yml: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/release.yml: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/release.yml: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/report-packs.md: follow-up desktools-v2/19 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/19 (owned rows flip to migrated)"
---

# Brief 19 — Migrate qualgen to Cobra and Viper

## Context

files:
- `qualgen` — `qualgen`; module qualgen; parser: Go flag: one FlagSet per verb with manual verb dispatch; 8654 LOC, ~7 verbs, ~22 flags at 3ad1ad83c871; release: release.yml standalone binary assets (qualgen-<os>-<arch>).
- `qualgen/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-19.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-qualgen.md` (planned).

facts:
- `qualgen/main.go` switches on the first argument and accepts `-h`, `--help` and `help` itself; those spellings stay valid.
- qualgen is a separate module and released binary; it uses Cobra and Viper directly, not the desk adapter.
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

1. Build a Cobra command tree for `qualgen`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd qualgen && go test -count=1 ./...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd qualgen && GOOS=windows go vet .` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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
### Non-implementer verifier run — 2026-10-16 claude-opus-5-5 verifier at 16831ecad72b (9/9 rows exit 0; mutation 8/8 caught; consumers row vacuous at merged main)

Verification-Attestation: medici-finance/assay#3172 run=38985d98bdff92c03fdc7df6d26fd38a5a7ccf2b52b9830e5a62b3efe8e6286c source=16831ecad72b9f697954a2ac8c80584d0d385aa8 brief=docs/streams/desktools-v2/brief-19-migrate-qualgen-to-cobra-and-viper.md model=claude-opus-5-5 tier=any

Fresh non-implementer run on merged main 16831ecad72b (delivery: PR #3152, commit 8122f1904), detached home, offline (KUBECONFIG=/dev/null), throwaway HOME and short private TMPDIR, GIT_CONFIG_NOSYSTEM=1, go1.27.2 darwin/arm64, statusgen v1.0.45. The admission check passed before the first row and again before this section was written. Every row ran verbatim; the home held no change other than this section afterwards. This brief is gate: model with every risk field "no".

| # | Command | Expected | Observed | Run |
|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; one PASS line for the help-offline test | exit 0; the log held exactly one top-level PASS line for TestCLIHelpOffline and `ok github.com/medici-finance/assay/qualgen`; no FAIL or SKIP lines | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; one PASS line for the config-flow test | exit 0; the log held exactly one top-level PASS line for TestCLIConfigFlow and `ok github.com/medici-finance/assay/qualgen`; no FAIL or SKIP lines | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; one PASS line for the legacy-forms test | exit 0; the log held exactly one top-level PASS line for TestCLILegacyForms and `ok github.com/medici-finance/assay/qualgen`; no FAIL or SKIP lines | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | exit 0; last line `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N = the spec's mutation count | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`; last line `Totals: 8 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec lists 8 mutations, so N = 8. The caught set covers help after a config read (2), an undeclared config key binding (2), a dropped legacy form (3) and a reopened completion entrypoint (1) | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | `cd qualgen && go test -count=1 ./...` | exit 0; every pre-existing test passes | exit 0; 11 `ok` package lines and no FAIL line, including `ok github.com/medici-finance/assay/qualgen 16.040s` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | `cd qualgen && GOOS=windows go vet .` | exit 0; compiles for Windows | exit 0; no output | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; owner-migrated test PASS | exit 0; printed one PASS line for TestCLIOwnerMigrated (4.13s); `ok github.com/medici-finance/assay/tools/desk/internal/clicontract 4.221s` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every floor seed passes | exit 0; last line `seed passes=26` | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | `statusgen --root . --consumers` | exit 0; no consumer routing disproved | exit 0; `consumers: no brief files in the diff against 16831ecad72b9f697954a2ac8c80584d0d385aa8 — nothing to corroborate`. At merged main the diff is empty (known shape, #1915), so this is vacuous. An extra read-only run with `--base 8122f1904^ --brief desktools-v2/19` exited 2 with 0 corroborated and 0 disproved, because the delivery commit did not touch the brief file, so that diff carries no claims to judge | 2026-10-16 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Additional checks (the brief's Review note: legacy-form coverage against consumers, old parser gone):
- The qualgen main package at 16831ecad72b has no Go flag package import, flag.NewFlagSet or flag.Parse left; the delivery removed every per-verb FlagSet. No AutomaticEnv, config search or global Viper; each invocation builds a fresh Viper (qualgen/cli.go line 501), and Cobra's completion command is disabled (line 121).
- The consumer invocations re-enumerated at this commit are `qualgen report --out .`, `QUALGEN_QUALITY_WRITER=ci qualgen report --out . --write` (assay-qualgen workflow in .github/workflows, ci/staged-workflows and tools/ci-load/activation), `qualgen --version` (release.yml) and `qualgen init` (report-packs doc). The legacy-forms test exercises each of them (cli_test.go lines 355-356, 376, 394; init dry-run at line 300).
- Every flag default in the delivery diff matches its old FlagSet default (repo ".", root ".", block-min DefaultBlockMin, churn-window-days DefaultChurnWindowDays, the rest "" or false).
- A binary built from 16831ecad72b, run with an empty HOME: `--help`, `-help`, `help mine`, `help pr`, `mine --help` and `pr --help` exited 0; `--version`, `-version` and `version` printed `dev` and exited 0; `completion bash`, an unknown mode and `report --nope` exited 2 with the unknown-mode or unknown-flag message.

RISK-VALUE: DERIVED — ciWriterToken = "ci" @ qualgen/report.go:38, bound through the env-only setting quality-writer (env QUALGEN_QUALITY_WRITER, no flag, no default) @ qualgen/cli.go:226 — this is the same authority the old code used (getenv(ciWriterEnv) == ciWriterToken, before 8122f1904). The new binding has no flag source, so `--quality-writer=ci` is refused (cli_test.go line 437). Unset and empty both resolve to "", which does not authorize. The CI workflows set exactly QUALGEN_QUALITY_WRITER=ci.
RISK-VALUE: DERIVED — exitUsage = 2 @ qualgen/cli.go:35 — the old dispatcher and Go's flag package both exited 2 on a usage error (the delivery replaced `return 2` lines), and cli-contract.md line 23 says an unknown input exits 2. The binary smoke run above observed 2.
RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 and DefaultChurnWindowDays = 14 @ qualgen/churn.go:22 — both are unchanged by the delivery and are still wired as the mine defaults (cli.go lines 190-191), the same as in the old FlagSet. They are reversible metric knobs.

**VERIFY: PASS** — 9/9 rows exit 0 (row 9 vacuous at merged main, #1915); gate: model

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
