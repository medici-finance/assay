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
- `tools/desk/cmd/deskchangelog/stamp_readback_test.go` (planned) — new; TestStampReadBack, the one test that observes the read-back: it hands the stamp handler a failing check and a passing check. The bash suite cannot do this (see facts).
- `tools/desk/cmd/deskchangelog/cli_test.go` (planned) — created by /52; TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms gain the stamp verb's cases.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-80.json` (planned) — muhar mutation spec for the stamp verb.
- `docs/streams/desktools-v2/cli-migration.json` — owned row flips to migrated.
- `docs/performance-entrypoints.json` — the row's `parser_state` follows the registry.
- `changelog/cli-spec-header-stamp.md` (planned).

facts:
- The row was added to the registry with the script (#2618, the cut-time header stamp) after the migration children were authored, and named `desktools-v2/47` as its owner, which covers four Go admin verbs and never this script (#5009). This brief owns it so no existing child widens its scope: /47 and /50–/55 are at the five-simple-row ceiling, and /56 and /57 cover unrelated tools.
- The sibling check script is ported into `tools/desk/cmd/deskchangelog` by /52. The stamp reads its result back through that check (same marker line, same first `vX.Y.Z` token), so both verbs live in one package and share one header parse; that is why this brief depends on /52 and not only on /16.
- Callers at 5fdae0d07b58: `.github/workflows/release.yml` and `ci/staged-workflows/release.yml` (two `bash tools/release/stamp-spec-header-version.sh "<tag>" "<spec>"` calls each, plus an error message quoting the one-argument form), `tools/release/release.yml.patch`, `tools/release/release.yml.proposed` and `tools/release/README.md`. Every caller passes positional arguments only; the script takes no flags. Re-enumerate at pickup.
- The script's contract, as `tools/release/README.md` and the script header state it and as run at 5fdae0d07b58: exit 0 stamped or already stamped (idempotent, no rewrite); exit 2 for a usage error (zero or three-or-more arguments, a malformed tag), an unreadable file, no marker line, no token, a failed rewrite or replace, or a failed read-back. Nothing is written on the pre-write refusals (usage, unreadable file, no marker line, no token) and when the rewrite or replace itself fails (the temp copy is removed). A failed read-back is different: the file has already been replaced, so the script exits 2, prints that it wrote the file but the read-back did not pass, and leaves the rewrite in place; there is no rollback. Only the first token on the first marker line changes; `spec-file` defaults to `spec/brief-v1.md` relative to the working directory; the read-back check path is fixed and not redirectable from the environment.
- The script has no `--` form. Run at 5fdae0d07b58: `<tag> -- <file>` is a three-argument call and exits 2 with the usage line; `<tag> -d.md` exits 2 (the file is read by grep, which rejects the dash-leading name, and nothing is written); the same name given with a dot-slash prefix stamps and exits 0. The port keeps all of that and adds no `--` form.
- What the existing bash suite proves: it stamps and then runs the check itself (T1, T4, T9), so it proves stamp and check agree, plus idempotence, byte-identical untouched content and the pre-write refusals (T5 to T8). It has no case that observes the stamp's own read-back: removing the read-back block from a scratch copy of the script still passes all 16 cases. After the port the read-back is in process, so no bash case can plant a failing check beside the script either. The read-back proof is therefore the new TestStampReadBack.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). A refused positional count exits 2 (`ExitUsage`), which matches the script's usage exit.

layering: Cobra adds a stamp verb to the deskchangelog tree and passes the validated tag and spec path into a handler that keeps the script's rewrite, idempotence and read-back. The read-back calls the same Go check /52 ports, in process, never a path taken from configuration. The handler receives the check as an unexported function value that production code binds to that Go check; no flag, key or environment variable reaches it, and only the package's own test passes a different one.
single-point-of-failure: the read-back through the header check — the stamp is only done when the check passes; TestStampReadBack (new, below) fails when the read-back is removed, ignored or its failure not turned into exit 2, and the mutation row plants exactly that against it. The existing bash suite does not cover the read-back and is not cited for it.
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
exit codes, idempotence, the pre-write refusals that write nothing, the rewrite left in place when the read-back fails (exit 2; no rollback is added, and a rollback would be a behaviour change this brief does not make) and the fixed read-back. No configuration
key, flag or environment variable may redirect the read-back check or skip it. A CLI library
is not a new authority source. Independent verification owns Evidence and lifecycle
advancement. Proposed tests and files below are deliverables, not claims that they already
exist.

## Task

1. Port the stamp into a verb of `tools/desk/cmd/deskchangelog` beside the check verb /52 adds, taking `<vX.Y.Z> [spec-file]` positionally with the same default. Reduce `tools/release/stamp-spec-header-version.sh` to a transparent adapter that execs the verb with equivalent arguments; keep its exit codes and messages a caller or test matches on.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any spec-file read, write or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help.
3. Write the golden precedence matrix BEFORE porting. The verb declares no configuration keys unless one is needed; if one is, bind it through a fresh Viper instance per invocation with explicit-empty versus unset, and never bind the read-back path or a skip switch.
4. Keep every legacy form the script has: one or two positionals, a spec file given as a dot-slash relative path whose name starts with a dash, the zero- and three-or-more-argument usage refusals at exit 2 before any file is touched (the three-argument call `<tag> -- <file>` is one of them), and a bare dash-leading spec file exiting 2 with nothing written. Do not add a `--` form: the script has none, so accepting `<tag> -- <file>` would be new behaviour and out of scope. Paths behave natively on Windows and POSIX.
5. Add TestStampReadBack in the new test file listed under files: with a check that fails, the handler exits 2 with the wrote-but-read-back-failed message and the file holds the new tag (the script's behaviour, no rollback); with a check that passes it exits 0 and reports the stamp; and a table case asserts the production binding is the in-process check, not a value read from configuration, flag or environment. It must fail when the read-back call is deleted, when its result is ignored, and when its failure exits 0.
6. Add the stamp verb's cases to TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in the package, using the fixture cases listed in cli-contract.md. Flip this brief's registry row to migrated, with `migrated_to` naming the Go entrypoint, only after they and TestStampReadBack pass; update the row's `parser_state` in `docs/performance-entrypoints.json` to match, then reconcile `tools/release/README.md`'s quoted usage.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS and covers the stamp verb's help under absent and malformed config with zero file reads or writes |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS; the stamp verb's tag and spec path reach the typed handler with the expected values, and the read-back path is not settable |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS; the one- and two-positional forms, a dot-slash relative spec file whose name starts with a dash, Windows and POSIX paths, the zero- and three-argument refusals at exit 2 (including `<tag> -- <file>`), and a bare dash-leading spec file exiting 2 with nothing written all behave as the script did, and no `--` form is accepted |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-80.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after a spec-file read, an undeclared config key binding, the two-positional form dropped, the read-back call removed, the read-back result ignored; the first three redden the owner tests named for them (TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms) and the two read-back plants must redden TestStampReadBack, which the spec names as their catching test |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskchangelog/...` | exit 0; every pre-existing test in the package still passes |
| 6 | check +flow | `bash tools/release/stamp-spec-header-version.test.sh && bash tools/release/check-spec-header-version.test.sh` | exit 0; the script's existing offline suites pass against the adapter: stamp-then-check agreement, only the header token changing, idempotence, the pre-write refusals leaving the file untouched, and the default path. These suites do not exercise the read-back; row 11 does |
| 7 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskchangelog` | exit 0; the package compiles for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/80 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; the row this brief owns is migrated, the registry validates and the contract tests pass in the package |
| 9 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 10 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |
| 11 | check +mutation | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskchangelog -run '^TestStampReadBack$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestStampReadBack " "$f")" -eq 1` | exit 0; TestStampReadBack PASS: a failing check gives exit 2 with the file already rewritten (no rollback), a passing check gives exit 0, and the production binding is the in-process check. Row 4's two read-back plants must turn this test red |

## Pre-mortem

Help reads or writes the spec file before printing: row 1 runs with hostile config and effect counters.
A configuration key or flag lets a caller point the read-back elsewhere or skip it: row 2 asserts it is not settable. The port drops or ignores the read-back: row 11 (TestStampReadBack) fails and row 4 plants both cases against it.
A release workflow's two-argument call stops parsing, or a usage error exits 1 instead of 2: row 3 replays each caller's form and the refusal exits. The port accepts a `--` form the script refuses, or rolls back a failed read-back the script leaves in place: row 3 and row 11 pin the script's behaviour.
The Go port rewrites a token the check does not read, so stamp and check disagree: row 6's suite stamps and then checks the same file (it proves agreement, not the read-back).
Only imports change while the bash parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check that the adapter keeps both release.yml calls working without a workflow edit, that the read-back stays fixed and in process and is proved by TestStampReadBack rather than by the bash suite, that no `--` form or rollback was added, and that the bash parser is gone.
