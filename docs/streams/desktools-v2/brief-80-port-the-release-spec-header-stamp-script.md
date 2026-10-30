---
brief: "assay:assay:desktools-v2:80"
title: "Port the release spec-header stamp script"
why: >-
  The release stamps the tag it cuts into spec/brief-v1.md's header with a bash script whose
  registry row had no owner that covered it. Its sibling, the header check, moves into the
  shared changelog and release command under /52, so the stamp follows it there, and the stamp's
  read-back through that check stays one parse with one answer.
wave: 4
depends: ["desktools-v2/16", "desktools-v2/52"]
unblocks: ["desktools-v2/17"]
effort: "S"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111, 5009]
schema: "brief-v2"
outcome: none
version: 1
id: "17f4afcd-c8c2-4907-8fa4-92c9243d6f5f"
authored: "2026-10-30 by worker-desk (routing the row named in #5009)"
sources:
  - "https://github.com/medici-finance/assay/issues/5009"
  - "https://github.com/medici-finance/assay/issues/5009#issuecomment-6531065868"
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/brief-52-port-the-changelog-and-release-check-scripts.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/80)"
  - "freshness-checked 2026-10-30 @ 5fdae0d07b58; the script, its test, the check script, tools/release/README.md and every release.yml caller read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Release-path compatibility: the live and staged release workflows call the script by path
  with positional arguments, its read-back must stay unredirectable, and a happy-path parser
  test cannot show either.
domain: "complicated"
consumers:
  - "tools/release/stamp-spec-header-version.sh: follow-up desktools-v2/80 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskchangelog: follow-up desktools-v2/80 (stamp verb beside the check verb /52 adds)"
  - ".github/workflows/release.yml: follow-up desktools-v2/80 (legacy invocation form kept; no workflow edit)"
  - "ci/staged-workflows/release.yml: follow-up desktools-v2/80 (legacy invocation form kept; no workflow edit)"
  - "tools/release/README.md: follow-up desktools-v2/80 (quoted usage reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/80 (owned row flips to migrated)"
  - "docs/performance-entrypoints.json: follow-up desktools-v2/80 (parser state mirrors the registry row)"
---

# Brief 80 — Port the release spec-header stamp script

## Context

files:
- `tools/release/stamp-spec-header-version.sh` — `stamp-spec-header-version.sh`; module bash; parser: bash positional arguments; 104 lines at 5fdae0d07b58; release: repo tooling; run by release.yml. Target: `tools/desk/cmd/deskchangelog`.
- `tools/release/stamp-spec-header-version.test.sh` — the script's offline suite (111 lines at 5fdae0d07b58); runs against the adapter unchanged.
- `tools/release/check-spec-header-version.sh` — the read-back the stamp calls; ported by /52 into the same command.
- `tools/desk/cmd/deskchangelog/cli_test.go` (planned) — created by /52; TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms gain the stamp verb's cases.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-80.json` (planned) — muhar mutation spec for the stamp verb.
- `docs/streams/desktools-v2/cli-migration.json` — owned row flips to migrated.
- `docs/performance-entrypoints.json` — the row's `parser_state` follows the registry.
- `changelog/cli-spec-header-stamp.md` (planned).

facts:
- The row was added to the registry with the script (#2618, the cut-time header stamp) after the migration children were authored, and named `desktools-v2/47` as its owner, which covers four Go admin verbs and never this script (#5009). This brief owns it so no existing child widens its scope: /47 and /50–/55 are at the five-simple-row ceiling, and /56 and /57 cover unrelated tools.
- The sibling check script is ported into `tools/desk/cmd/deskchangelog` by /52. The stamp reads its result back through that check (same marker line, same first `vX.Y.Z` token), so both verbs live in one package and share one header parse; that is why this brief depends on /52 and not only on /16.
- Callers at 5fdae0d07b58: `.github/workflows/release.yml` and `ci/staged-workflows/release.yml` (two `bash tools/release/stamp-spec-header-version.sh "<tag>" "<spec>"` calls each, plus an error message quoting the one-argument form), `tools/release/release.yml.patch`, `tools/release/release.yml.proposed` and `tools/release/README.md`. Every caller passes positional arguments only; the script takes no flags. Re-enumerate at pickup.
- The script's contract: exit 0 stamped or already stamped (idempotent, no rewrite); exit 2 for a malformed tag, an unreadable file, no marker line, no token, a failed write or a failed read-back; nothing is written on any could-not-check; only the first token on the first marker line changes; `spec-file` defaults to `spec/brief-v1.md` relative to the working directory; the read-back check path is fixed and not redirectable from the environment.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). A refused positional count exits 2 (`ExitUsage`), which matches the script's usage exit.

layering: Cobra adds a stamp verb to the deskchangelog tree and passes the validated tag and spec path into a handler that keeps the script's rewrite, idempotence and read-back. The read-back calls the same Go check /52 ports, in process, never a path taken from configuration.
single-point-of-failure: the read-back through the header check — the stamp is only done when the check passes; the existing offline suite and the mutation row each plant its removal and must redden.
design-fit:
  owner: the deskchangelog package /52 creates; header semantics stay with the check it already owns
  contract: none — consumes cli-contract.md delivered by /15
  retires: [the bash positional parser and usage string in stamp-spec-header-version.sh]
  weight: verbs +1 (the stamp verb in deskchangelog, replacing a script entrypoint); flags +0 except conventional help spellings; refusals +0
  why-add: one verb beside the ported check keeps the stamp and its read-back on one parse; a separate binary would duplicate the header parser /52 ports

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge, and no edit to a workflow file: the script stays at its path as a
transparent adapter so both release.yml copies keep calling it unchanged. Preserve the
exit codes, idempotence, write-nothing-on-failure and the fixed read-back. No configuration
key, flag or environment variable may redirect the read-back check or skip it. A CLI library
is not a new authority source. Independent verification owns Evidence and lifecycle
advancement. Proposed tests and files below are deliverables, not claims that they already
exist.

## Task

1. Port the stamp into a verb of `tools/desk/cmd/deskchangelog` beside the check verb /52 adds, taking `<vX.Y.Z> [spec-file]` positionally with the same default. Reduce `tools/release/stamp-spec-header-version.sh` to a transparent adapter that execs the verb with equivalent arguments; keep its exit codes and messages a caller or test matches on.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any spec-file read, write or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help.
3. Write the golden precedence matrix BEFORE porting. The verb declares no configuration keys unless one is needed; if one is, bind it through a fresh Viper instance per invocation with explicit-empty versus unset, and never bind the read-back path or a skip switch.
4. Keep every legacy form the callers use: one or two positionals, a dash-leading or path-like spec file after `--`, and the zero- and three-argument usage refusals at exit 2 before any file is touched. Paths behave natively on Windows and POSIX.
5. Add the stamp verb's cases to TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in the package, using the fixture cases listed in cli-contract.md. Flip this brief's registry row to migrated, with `migrated_to` naming the Go entrypoint, only after they pass; update the row's `parser_state` in `docs/performance-entrypoints.json` to match, then reconcile `tools/release/README.md`'s quoted usage.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS and covers the stamp verb's help under absent and malformed config with zero file reads or writes |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS; the stamp verb's tag and spec path reach the typed handler with the expected values, and the read-back path is not settable |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS; the one- and two-positional forms, `--` before a dash-leading path, Windows and POSIX paths, and the zero- and three-argument refusals at exit 2 behave as the script did |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-80.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after a spec-file read, an undeclared config key binding, the two-positional form dropped, the read-back removed; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskchangelog/...` | exit 0; every pre-existing test in the package still passes |
| 6 | check +flow | `bash tools/release/stamp-spec-header-version.test.sh && bash tools/release/check-spec-header-version.test.sh` | exit 0; the script's existing offline suite, including the read-back, idempotence and write-nothing cases, passes against the adapter |
| 7 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskchangelog` | exit 0; the package compiles for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/80 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; the row this brief owns is migrated, the registry validates and the contract tests pass in the package |
| 9 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 10 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads or writes the spec file before printing: row 1 runs with hostile config and effect counters.
A configuration key or flag lets a caller point the read-back elsewhere or skip it: row 2 asserts it is not settable and row 4 plants its removal.
A release workflow's two-argument call stops parsing, or a usage error exits 1 instead of 2: row 3 replays each caller's form and the refusal exits.
The Go port rewrites a token the check does not read, so stamp and check disagree: row 6's suite stamps and then checks the same file.
Only imports change while the bash parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check that the adapter keeps both release.yml calls working without a workflow edit, that the read-back stays fixed and in process, and that the bash parser is gone.
