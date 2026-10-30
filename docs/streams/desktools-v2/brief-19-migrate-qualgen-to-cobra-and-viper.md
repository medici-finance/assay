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

### Non-implementer verifier re-run — 2026-10-18 claude-opus-5-5 verifier at 6dfb616ef546 (9/9 rows exit 0; mutation 8/8 caught; consumers row vacuous at merged main)

Verification-Attestation: medici-finance/assay#3393 run=996d6172f1e379c3ba0f0fdec4bc34e6ac479767c5af9b32479e8f5a70222b4c source=6dfb616ef5463fa886e89ab7b07803d30006c58a brief=docs/streams/desktools-v2/brief-19-migrate-qualgen-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Fresh non-implementer run on merged main 6dfb616ef546, in a detached home, offline (KUBECONFIG=/dev/null), with go1.27.2 darwin/arm64 and statusgen v1.0.49. The admission check passed before the first row and again after this section was written. qualgen changed after the 2026-10-16 run: commit 771e3144e (PR #3226) added the mine settings fix-source and defect-labels in cli.go and updated three legacy-form expectations in cli_test.go, and commit 35c5ff081 (PR #3294) added a refix test. So this run re-checks the owned command tree as it stands now. Every row ran verbatim. Row 4 (muhar -j 1) mutates files in place, so it ran alone. After every row, git status in the home was empty, ignored files included. This brief is gate: model with every risk field "no".

| # | Command | Expected | Observed | Run |
|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; one PASS line for the help-offline test | exit 0; the log held exactly one `--- PASS: TestCLIHelpOffline (0.00s)` line and `ok github.com/medici-finance/assay/qualgen 0.148s`; zero FAIL or SKIP lines | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 2 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; one PASS line for the config-flow test | exit 0; the log held exactly one `--- PASS: TestCLIConfigFlow (0.01s)` line and `ok github.com/medici-finance/assay/qualgen 0.165s`; zero FAIL or SKIP lines | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 3 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; one PASS line for the legacy-forms test | exit 0; the log held exactly one `--- PASS: TestCLILegacyForms (0.01s)` line and `ok github.com/medici-finance/assay/qualgen 0.184s`; zero FAIL or SKIP lines | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | exit 0; last line `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N = the spec's mutation count | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`; last line `Totals: 8 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec lists 8 mutations, so N = 8. Caught: help after a config read (2: the writer variable read before parsing, a root pre-run hook reading the environment), an undeclared config key binding (2: mine repo from an environment variable, a resolver reading every environment name), a dropped legacy form (3: single-dash Go-flag spellings, the pr positional number, a bare release tag on --version) and a reopened completion entrypoint (1) | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 5 | `cd qualgen && go test -count=1 ./...` | exit 0; every pre-existing test passes | exit 0; 11 `ok` package lines and no FAIL line, including `ok github.com/medici-finance/assay/qualgen 15.408s` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 6 | `cd qualgen && GOOS=windows go vet .` | exit 0; compiles for Windows | exit 0; no output | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; owner-migrated test PASS | exit 0; printed `--- PASS: TestCLIOwnerMigrated (2.71s)`; `ok github.com/medici-finance/assay/tools/desk/internal/clicontract 2.826s` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every floor seed passes | exit 0; last line `seed passes=26` | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |
| 9 | `statusgen --root . --consumers` | exit 0; no consumer routing disproved | exit 0; `consumers: no brief files in the diff against 6dfb616ef5463fa886e89ab7b07803d30006c58a — nothing to corroborate`. At merged main the diff is empty (the known shape, #1915), so this row is vacuous: it disproves nothing and corroborates nothing | 2026-10-18 assay-verifier-app[bot] (claude-opus-5-5 verifier) |

Additional checks at 6dfb616ef546 (the brief's Review note: legacy-form coverage against the consumers, and the old parser gone):
- The non-test Go files in qualgen have no Go flag package import and no flag.NewFlagSet or flag.Parse. They also have no AutomaticEnv, no config search and no global Viper call. Each invocation builds its own Viper (viper.NewWithOptions at qualgen/cli.go line 505). Cobra's completion command is disabled (DisableDefaultCmd at line 121).
- The consumer run lines found again at this commit: `report --out .` and `QUALGEN_QUALITY_WRITER=ci … report --out . --write` (the assay-qualgen workflow in .github/workflows, its ci/staged-workflows twin and tools/ci-load/activation), `qualgen --version` (release.yml) and `qualgen init` (report-packs doc). The legacy-forms and config-flow tests exercise each of these forms. The new defect-labels default from PR #3226 is folded into the three mine expectations.
- A binary built from 6dfb616ef546 into scratch outside the home, run with an empty HOME and an XDG_CONFIG_HOME that does not exist: `--help`, `-help`, `help mine`, `mine --help` and `pr --help` exited 0. `--version`, `-version` and `version` exited 0, printing `dev`. `completion bash` and an unknown mode exited 2 with `unknown mode "…" (want one of: mine, report, pr, check, sweep, init)`. `report --nope` exited 2 with `unknown flag: --nope`.

Risk-bearing values. The enumeration covers the delivery diff (8122f1904) plus the later qualgen changes in the owned command tree, up to 6dfb616ef546.
RISK-VALUE: DERIVED — ciWriterToken = "ci" @ qualgen/report.go:38, bound only through the env setting quality-writer (env QUALGEN_QUALITY_WRITER, no flag, no default) @ qualgen/cli.go:230. This is the same authority the pre-migration code used (getenv(ciWriterEnv) == ciWriterToken). The binding has no flag source, so `--quality-writer=ci` is refused as an unknown flag (cli_test.go line 437). Unset and empty both resolve to "", which does not authorize. The CI workflows set exactly QUALGEN_QUALITY_WRITER=ci.
RISK-VALUE: DERIVED — exitUsage = 2 @ qualgen/cli.go:35. The old dispatcher and Go's flag package both exited 2 on a usage error, and cli-contract.md line 23 says an unknown input exits 2. The binary smoke run above observed 2.
RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 and DefaultChurnWindowDays = 14 @ qualgen/churn.go:22. The migration left both unchanged; they are still the mine defaults (cli.go lines 190-191), as in the old FlagSet. They are reversible metric knobs.
RISK-VALUE: NAMED, NOT DERIVED — defect-labels default = "bug,defect,incident" @ qualgen/cli.go:195. PR #3226 added it after this brief's delivery, so it is outside this brief's contract. It ranks last: it is a reversible label filter for the offline defect trace, active only with fix-source. Deriving it belongs to that change's own brief, not this migration.

**VERIFY: PASS** — 9/9 rows exit 0 at 6dfb616ef546 (row 9 vacuous at merged main, #1915); gate: model

### Non-implementer verifier re-run — 2026-10-21 claude-opus-5-5 verifier at 48532629922d (8/9 rows exit 0; row 4 could-not-check: mutation harness broken at merged main)

Verification-Attestation: medici-finance/assay#3954 run=35cbb26f44f70a5c1d7ac9e627143f5ee9907d5994d88c3470a3876d0a94642d source=48532629922d99cb26077c8baa2ea901af811110 brief=docs/streams/desktools-v2/brief-19-migrate-qualgen-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Fresh non-implementer run on merged main 48532629922d in a detached home, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local, GOFLAGS=-mod=readonly), go1.27.2 darwin/arm64, statusgen v1.0.54. The admission check passed before the first row and again after this section was written. qualgen has no commit since the 2026-10-18 run at 6dfb616ef546; the tools that rows 4, 7 and 8 drive did change (muhar in #3272, clicontract and the regression floor in later commits), so this run re-checks them. Every row ran verbatim; row 4 ran alone because muhar mutates files in place. After every row, git status in the home was empty, ignored files included. This brief is gate: model with every risk field "no".

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; one PASS line for the help-offline test | exit 0; the log held exactly one `--- PASS: TestCLIHelpOffline (0.01s)` line and `ok github.com/medici-finance/assay/qualgen 0.269s`; zero FAIL lines | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; one PASS line for the config-flow test | exit 0; the log held exactly one `--- PASS: TestCLIConfigFlow (0.01s)` line and `ok github.com/medici-finance/assay/qualgen 0.223s`; zero FAIL lines | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; one PASS line for the legacy-forms test | exit 0; the log held exactly one `--- PASS: TestCLILegacyForms (0.04s)` line and `ok github.com/medici-finance/assay/qualgen 0.271s`; zero FAIL lines | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | exit 0 and last line `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N = the spec's mutation count | could-not-check: the harness broke before any mutation was judged. exit 1 (muhar itself exit status 2), twice. The baseline printed `ok github.com/medici-finance/assay/qualgen`, then `HARNESS BROKEN — run discarded.` and `positive control could not be validated (mutated package does not compile)`. The compile check is the build muhar derives since #3272: the spec's test command starts with a cd into the qualgen module, which the derivation does not parse, so muhar falls back to building the package path relative to tools/desk and go reports `directory ../../qualgen outside main module or its selected dependencies`. No Totals line was printed. Instrument defect, not a qualgen finding | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | `cd qualgen && go test -count=1 ./...` | exit 0; every pre-existing test passes | exit 0; 11 `ok` package lines and no FAIL line, including `ok github.com/medici-finance/assay/qualgen 17.539s` | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | `cd qualgen && GOOS=windows go vet .` | exit 0; compiles for Windows | exit 0; no output | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; owner-migrated test PASS | exit 0; printed `--- PASS: TestCLIOwnerMigrated (4.31s)`; `ok github.com/medici-finance/assay/tools/desk/internal/clicontract 4.487s` | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every floor seed passes | exit 0; last line `seed passes=26` | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | `statusgen --root . --consumers` | exit 0; no consumer routing disproved | exit 0; `consumers: no brief files in the diff against 48532629922d99cb26077c8baa2ea901af811110 — nothing to corroborate`. At merged main the diff is empty (the known shape, #1915), so this row is vacuous: it disproves nothing and corroborates nothing | 2026-10-21 | assay-verifier-app[bot] @ 48532629922d99cb26077c8baa2ea901af811110 (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Row accounting: 9 required rows; 8 checked-clean (rows 1-3 and 5-9, row 9 vacuous); 0 checked-failed; 1 could-not-check (row 4).

Supplemental diagnosis for row 4 (earns no credit): the derived build command alone, run from tools/desk (go test -vet=off -c -o /dev/null ./../../qualgen), fails with the same `outside main module` message. The same spec and the same qualgen tree, driven by the muhar source from 6dfb616ef546 (before #3272) in a scratch copy outside the home, exited 0 with `Harness healthy: baseline GREEN, positive control CAUGHT.` and `Totals: 8 caught, 0 NOT CAUGHT, 0 could-not-mutate.` (the spec holds 1 control and 8 mutations). This spec is the only mutation spec in the tree whose test command starts with a cd. A muhar fix (derive the build in the test command's directory) or a spec build field restores the row; either needs a fresh dispatch after it lands.

Risk-bearing values. Enumeration over the delivery diff (8122f1904) and later qualgen changes up to 48532629922d; qualgen is unchanged since 6dfb616ef546, so the set is the 2026-10-18 one, with every line re-read at this commit.
RISK-VALUE: DERIVED — ciWriterToken = "ci" @ qualgen/report.go:38, bound only through the env setting quality-writer (env QUALGEN_QUALITY_WRITER, no flag, no default) @ qualgen/cli.go:230. This is the same authority the pre-migration code used (getenv(ciWriterEnv) == ciWriterToken). With no flag source, a quality-writer flag is refused as unknown; unset and empty both resolve to "", which does not authorize. The CI workflows set exactly QUALGEN_QUALITY_WRITER=ci.
RISK-VALUE: DERIVED — exitUsage = 2 @ qualgen/cli.go:35. The old dispatcher and Go's flag package both exited 2 on a usage error, and cli-contract.md line 23 says an unknown input exits 2.
RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 and DefaultChurnWindowDays = 14 @ qualgen/churn.go:22. The migration left both unchanged and they are still the mine defaults (cli.go lines 190-191), as in the old FlagSet. Reversible metric knobs.
RISK-VALUE: NAMED, NOT DERIVED — defect-labels default = "bug,defect,incident" @ qualgen/cli.go:195. Added by PR #3226 after this brief's delivery, outside this brief's contract; a reversible label filter active only with fix-source. Its derivation belongs to that change's own brief.

**VERIFY: BLOCKED** — 8/9 rows exit 0, 0 failed, row 4 could-not-check (muhar derived-build regression from #3272 breaks the harness for this cross-module spec); gate: model

### Citation addendum — 2026-10-22 (no row run again; cites the blocker for the 2026-10-21 section)

This addendum is append-only. It changes nothing in the sections above, and no Verify row was run for it. It records the blocker issue that the section "Non-implementer verifier re-run — 2026-10-21 claude-opus-5-5 verifier at 48532629922d (8/9 rows exit 0; row 4 could-not-check: mutation harness broken at merged main)" names but does not cite.

Blocker: row 4 could-not-check is tracked on #4117 (muhar: a spec test command starting with cd breaks the derived build since #3272); re-dispatch once it lands. #4117 is open. Its text names the same cause recorded in row 4 and the supplemental diagnosis of the 2026-10-21 section: the derived build does not follow a leading cd, falls back to the bare-package build relative to tools/desk, and go reports `directory ../../qualgen outside main module or its selected dependencies`. Its done-when list has this brief re-dispatched for verification once the fix lands, with row 4 run again as written.

The 2026-10-21 section's rows, row accounting and risk-bearing values stand as recorded.

**VERIFY: BLOCKED** — 8/9 rows exit 0, 0 failed, row 4 could-not-check (muhar derived-build regression from #3272, tracked on #4117); gate: model
### Non-implementer verifier re-run — 2026-10-27 claude-opus-5-5 verifier at 79e67f655798 (8/9 rows exit 0; row 4 harness still broken at merged main, #4117 open)

Fresh non-implementer run on merged main 79e67f655798 in a detached home, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local), go1.27.2 darwin/arm64, statusgen v1.0.63. The admission check passed before the first row. The witness below is the bytes `statusgen verifyrun --fail-first --base 5a657ba5824d` wrote, copied unchanged. The base is the parent of the delivering merge 8122f1904 (#3152), read from the forge. The tool reported no risk-bearing row in this table, so every row reads not-selected at base. This brief is gate: model with every risk field "no". qualgen has no commit since 6dfb616ef546. The muhar source has no code change since the 2026-10-21 run (only its README and an anchors test changed), and #4117 is still open.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 2 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 3 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | fail exit=1 | sha256:21cde9ae4fbe | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 5 | `cd qualgen && go test -count=1 ./...` | pass exit=0 | sha256:5349484605a4 | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 6 | `cd qualgen && GOOS=windows go vet .` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:5ba21151501c | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:469d3a36d825 | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:4492200b4ea7 | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
Verification-Attestation: medici-finance/assay#4693 run=f8f14b7f67d06c45d04e0ed18cce33be5200372b15bcfee3a1a14379dd829e3e source=79e67f655798746322990d3f8da19cbef8bd1218 brief=docs/streams/desktools-v2/brief-19-migrate-qualgen-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Key output per row (the witness keeps only an output digest; sha256:e3b0c44298fc is the empty output of rows 1, 2, 3 and 6, whose commands send the test log to a file):
- Rows 1, 2 and 3: exit 0, so each log held exactly one top-level PASS line for its test.
- Row 4: exit 1. muhar's own exit status was 2. The baseline printed `ok github.com/medici-finance/assay/qualgen`, then the derived compile check failed with `directory ../../qualgen outside main module or its selected dependencies` and `FAIL ./../../qualgen [setup failed]`, then `HARNESS BROKEN — run discarded.` and `positive control could not be validated (mutated package does not compile) — discard the run`. No Totals line was printed and no mutation was judged. This is the same instrument defect the 2026-10-21 section recorded and #4117 tracks: the build muhar derives does not follow the spec's leading cd into the qualgen module. It is not a qualgen finding.
- Row 5: exit 0; 11 `ok` package lines and no FAIL line.
- Row 6: exit 0; no output.
- Row 7: exit 0; printed one PASS line for TestCLIOwnerMigrated.
- Row 8: exit 0; last line `seed passes=26`.
- Row 9: exit 0. The home's brief was unchanged when the row ran, so the diff against merged main was empty and the row is vacuous (the known shape, #1915).

The row 4 detail and the row 5, 7 and 8 lines come from a supplemental re-run of those exact commands in the same home straight after the witness. That re-run earns no credit; it only shows what the digests stand for. After every run, git status in the home showed only this brief changed, ignored files included.

Row accounting: 9 required rows. 8 checked-clean (rows 1-3 and 5-9; row 9 vacuous). 0 failed acceptance obligations. 1 could-not-check: row 4. Its witness reads fail exit=1, but the harness discarded itself before any mutation was planted, so the row gives no acceptance result. Row 4 needs a muhar fix, or a build field in the spec, then a fresh dispatch once that lands.

Risk-bearing values. The enumeration covers the delivery diff (8122f1904) and later qualgen changes up to 79e67f655798. qualgen is unchanged since 6dfb616ef546, so the set matches the earlier sections. Every line was re-read at this commit.
RISK-VALUE: DERIVED — ciWriterToken = "ci" @ qualgen/report.go:38, bound only through the env setting quality-writer (env QUALGEN_QUALITY_WRITER, no flag, no default) @ qualgen/cli.go:230. The pre-migration code at 5a657ba5824d used the same authority (getenv(ciWriterEnv) == ciWriterToken, report.go line 47). The binding has no flag source, so a quality-writer flag is refused as an unknown flag (cli_test.go line 437). Unset and empty both resolve to "", which does not authorize. The CI workflow sets exactly QUALGEN_QUALITY_WRITER=ci.
RISK-VALUE: DERIVED — exitUsage = 2 @ qualgen/cli.go:35. The old dispatcher returned 2 on a usage error (main.go lines 37 and 59 at 5a657ba5824d), as Go's flag package does. cli-contract.md line 23 says an unknown input exits 2.
RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 and DefaultChurnWindowDays = 14 @ qualgen/churn.go:22. The migration did not change either value, and both are still the mine defaults (cli.go lines 190-191), as in the old FlagSet. They are reversible metric knobs.
RISK-VALUE: NAMED, NOT DERIVED — defect-labels default = "bug,defect,incident" @ qualgen/cli.go:195. PR #3226 added it after this brief's delivery, so it is outside this brief's contract. It is a reversible label filter and is active only with fix-source. Deriving it belongs to that change's own brief.

**VERIFY: BLOCKED** — 8/9 rows exit 0, 0 acceptance obligations failed, row 4 could-not-check (the muhar derived-build regression from #3272 still breaks the harness for this cross-module spec, tracked on #4117); gate: model
### Non-implementer verifier re-run — 2026-10-29 claude-opus-5-5 verifier at 1304d40f5c32 (8/9 rows exit 0; row 4 harness still broken at merged main, #4117 open)

Fresh non-implementer run on merged main 1304d40f5c32 in a detached home, offline (KUBECONFIG=/dev/null, GOPROXY=off, GOTOOLCHAIN=local, no forge token in the run environment), go1.27.2 darwin/arm64, statusgen v1.0.65. The admission check passed before the first row and again after this section was written. The witness below is the bytes `statusgen verifyrun` wrote, unchanged. qualgen has no commit since 6dfb616ef546. Since the 2026-10-27 run, muhar moved to the shared CLI adapter (b11eb73f4) and gained version flags (794c18f85); neither commit touches the derived build that #4117 describes, and the mutation spec is unchanged.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd qualgen && go test -count=1 -v . -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-19.json` | fail exit=1 | sha256:545010f744b0 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd qualgen && go test -count=1 ./...` | pass exit=0 | sha256:630dcc780185 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd qualgen && GOOS=windows go vet .` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/19 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:927fa0395e1f | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:0d2e849a35d8 | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:ec256dd0157f | 2026-10-29 | assay-verifier-app[bot] @ 1304d40f5c32 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4911 run=59245165dea152d266e9bf5ace08c01c80393895c4e00278953e2919d776d1c1 source=1304d40f5c32cae5136e87f6b1f0cbe5a008c72a brief=docs/streams/desktools-v2/brief-19-migrate-qualgen-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

Key output per row (the witness keeps only an output digest; sha256:e3b0c44298fc is the empty output of rows 1, 2, 3 and 6, whose commands send the test log to a file):
- Rows 1, 2 and 3: exit 0, so each log held exactly one top-level PASS line for its test.
- Row 4: exit 1. muhar's own exit status was 2. The baseline printed `ok github.com/medici-finance/assay/qualgen`, then the derived compile check failed with `directory ../../qualgen outside main module or its selected dependencies` and `FAIL ./../../qualgen [setup failed]`, then `HARNESS BROKEN — run discarded.` and `positive control could not be validated (mutated package does not compile) — discard the run`. No mutation was planted or judged.
- Row 5: exit 0; 11 `ok` package lines and no FAIL line, including `ok github.com/medici-finance/assay/qualgen 12.286s`.
- Row 6: exit 0; no output.
- Row 7: exit 0; printed `--- PASS: TestCLIOwnerMigrated (2.70s)`.
- Row 8: exit 0; last line `seed passes=26`.
- Row 9: exit 0. The home's brief was unchanged when the row ran, so the diff against merged main was empty and the row is vacuous (the known shape, #1915).

The row 4 detail and the row 5, 7 and 8 lines come from a supplemental re-run of those exact commands in the same home straight after the witness. That re-run earns no credit; it only shows what the digests stand for. After every run, git status in the home showed only this brief changed, ignored files included.

Row accounting: 9 required rows. 8 checked-clean (rows 1-3 and 5-9; row 9 vacuous). 0 failed acceptance obligations. 1 could-not-check: row 4. Its witness reads fail exit=1, but the harness discarded itself before any mutation was planted, so the row gives no acceptance result. Row 4 needs the muhar fix tracked on #4117 (open, checked 2026-10-29), or a build field in the spec, then a fresh dispatch once that lands.

Risk-bearing values. The enumeration covers the delivery diff (8122f1904) and later qualgen changes up to 1304d40f5c32. qualgen is unchanged since 6dfb616ef546, so the set matches the earlier sections. Every line was re-read at this commit.
RISK-VALUE: DERIVED — ciWriterToken = "ci" @ qualgen/report.go:38, bound only through the env setting quality-writer (env QUALGEN_QUALITY_WRITER, no flag, no default) @ qualgen/cli.go:230. The pre-migration code at 5a657ba5824d used the same authority (getenv(ciWriterEnv) == ciWriterToken, report.go line 47). The binding has no flag source, so a quality-writer flag is refused as an unknown flag (cli_test.go line 437); unset and empty both resolve to "", which does not authorize the committed-view write.
RISK-VALUE: DERIVED — exitUsage = 2 @ qualgen/cli.go:35. The old dispatcher returned 2 on a usage error (main.go lines 37 and 59 at 5a657ba5824d), as Go's flag package does. cli-contract.md line 23 says an unknown input exits 2.
RISK-VALUE: DERIVED — DefaultBlockMin = 4 @ qualgen/taxonomy.go:40 and DefaultChurnWindowDays = 14 @ qualgen/churn.go:22. The migration did not change either value, and both are still the mine defaults (cli.go lines 190-191), as in the old FlagSet (mine.go lines 31-32 at 5a657ba5824d). They are reversible metric knobs.
RISK-VALUE: NAMED, NOT DERIVED — defect-labels default = "bug,defect,incident" @ qualgen/cli.go:195. PR #3226 added it after this brief's delivery, so it is outside this brief's contract. It is a reversible label filter and is active only with fix-source. Deriving it belongs to that change's own brief.

**VERIFY: BLOCKED** — 8/9 rows exit 0, 0 acceptance obligations failed, row 4 could-not-check (the muhar derived-build regression from #3272 still breaks the harness for this cross-module spec, tracked on #4117); gate: model

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
