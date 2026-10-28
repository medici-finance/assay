---
brief: "assay:assay:desktools-v2:21"
title: "Migrate deskdispatch to Cobra and Viper"
why: >-
  deskdispatch prepares worktrees and claims for every dispatched item and reads
  DESK_ROOTS-style configuration; its flags are a single large FlagSet with mode switches.
  Cobra and declared Viper bindings make the modes discoverable without changing which
  worktree, branch or claim a given invocation produces.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 3
id: "2969ba49-706d-4544-b46b-e8b87b133bde"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "assay:assay:desktools-v2:60 — consumer routing for the 2026-10-16 performance amendment"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/21)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskdispatch: follow-up desktools-v2/21 (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/internal/regression/MANIFEST.md: follow-up desktools-v2/21 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/21 (owned rows flip to migrated)"
---

# Brief 21 — Migrate deskdispatch to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskdispatch` — `deskdispatch`; module tools/desk; parser: Go flag: FlagSet; 4627 LOC, ~5 verbs, ~15 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskdispatch/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-21.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-dispatch.md` (planned).

facts:
- deskdispatch resolves the dispatch-claim helper and the stream-root map from flags and environment; the per-key precedence (flag, then environment, then default) is part of the golden matrix.
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

1. Build a Cobra command tree for `deskdispatch`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

| # | Class | Command | Expect |
|---|---|---|---|
| 10 | check:ci +flow +mutation | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | exit 0; named PASS; compiled help/version with malformed config and a large manifest fixture performs zero operational config/discovery/custody reads; moving discovery before help fails |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Performance reuse amendment — 2026-10-16

Keep this brief's parser ownership and existing compatibility rows. The shared credential
implementation belongs to /58–59, runtime manifest discovery to /60 and timing semantics to
/61–62. Help/version acceptance includes every pre-handler entrypoint step: no config echo,
activation discovery, file/index read, credential lookup or child process may occur first.
Count those effects in the compiled command test, not only inside the handler. If the shared
performance recorder is armed, a local best-effort performance record remains subject to its
existing explicit consent; help must not read operational configuration to classify itself.
Retain the one measurement finalizer when moving to Cobra. Do not reset the timer after
parsing, add a second writer or reintroduce a minter callout in an adapter.


## Perf-v3 delivery amendment — 19 October 2026

The [perf-v3 contract](../../performance-attribution-spec.md) defines future shared performance flags/envs and selected profiles.
/63 owns schema/resolution; /64 owns common adapter/launcher integration and inventory;
/65 owns forge observations. Preserve this brief's existing migration/qualification scope,
help/version, exit and parser rules. Do not implement a competing v2-only collector or copy
work identity into old schemas. Adopt /64's common binding when present; otherwise record
the explicit integration gap for /64, retaining prior behavior until that owner lands.
Final suite conformance /17 must exercise the common precedence/profile fixtures and show
that attribution cannot change operational identity or fail a valid operation.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:001ed55ed082 | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:3fa5dbd99069 | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:aeec296da65a | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | fail exit=1 | sha256:6d336dab41fa | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:25545118df71 | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ e4d474af8bca (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3453 run=28f22f72e30a7ef1523d6d5d9dbb467d019f5951573f50bc87f54a8113678d03 source=e4d474af8bca036b11291259cf6130d98abe5feb brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

### Verifier pass — 2026-10-19 (merged main e4d474af8bca)

Verification-Attestation: medici-finance/assay#3453 run=28f22f72e30a7ef1523d6d5d9dbb467d019f5951573f50bc87f54a8113678d03 source=e4d474af8bca036b11291259cf6130d98abe5feb brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

The execution witness above was run once with writes and twice more as dry runs; each run gave the same per-row results. The by-hand observations below are separate from the witness.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | Verify row 1 command as written | exit 0, one top-level PASS | witness: pass exit=0 in all 3 witness runs; by hand: exit 0, `--- PASS: TestCLIHelpOffline` once, 126 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 command as written | exit 0, one top-level PASS | witness: pass exit=0 in all 3 witness runs; by hand: exit 0, `--- PASS: TestCLIConfigFlow` once, 31 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 command as written | exit 0, one top-level PASS | witness: pass exit=0 in all 3 witness runs; by hand on a quiescent tree: exit 0, `--- PASS: TestCLILegacyForms` once | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command as written | exit 0, `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count | witness: pass exit=0 in all 3 witness runs; by hand: exit 0, last line `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec has 10 mutations plus 1 control | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 command as written | exit 0 | witness: pass exit=0 in all 3 witness runs | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 command as written | exit 0 | witness: pass exit=0 in all 3 witness runs | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 command as written | exit 0, named PASS | witness: pass exit=0 in all 3 witness runs, `--- PASS: TestCLIOwnerMigrated` printed | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 command as written | exit 0, every floor seed passes | witness: FAIL exit=1 in all 3 witness runs. Output captured in a diagnostic dry run: one seed fails, `--- FAIL: TestReg1145ShimCredential`; its cellctl fixture suite gen-shims-gh-token.test.sh reports `2 FAILED` (the two ambient-token assertions). Cause: the house cell's statusgen shim exports CELLCTL_GH_AMBIENT into the witness environment, and that fixture inherits it. By hand, with that variable absent: exit 0, `seed passes=26`, 5 of 5 runs. With a dummy CELLCTL_GH_AMBIENT set: that seed fails with the same 2 FAIL lines. The delivering change (66dbced7188e) touches neither the fixture nor the floor. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 command as written | exit 0 | witness: pass exit=0 in all 3 witness runs | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | row 10 command as written, run by hand under darwin sandbox-exec with a deny-network profile | exit 0, named PASS | witness: could-not-run (the unshare sandbox is Linux-only; this host is darwin). By hand, network-off: exit 0, `--- PASS: TestCLIHelpDoesNotDiscoverManifests (4.83s)`. A network probe in the same profile failed to connect (curl exit 7), which confirms the sandbox. This by-hand row is distinct from the witness; the CI lane owns the network-off rerun. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Review-note checks: no Go standard-library flag import remains in any file of the deskdispatch package (grep exit 1). Domain handlers now receive typed options from one Cobra tree. Syntax errors moved from exit 5 to exit 2, as the shared CLI contract states (cli-contract.md line 23, "Unknown input"); the dispatch CLI matrix records the move (dispatch-cli-matrix.md lines 59-60). The bare no-argument refusal stays at exit 5.

Risk-bearing values: the brief's frontmatter has risk metadata, all four fields "no", and the gate is model. Enumerated anyway over the delivering diff (66dbced7188e^1..66dbced7188e) for the deskdispatch package and the mutation spec:
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18. It now applies to deskdispatch syntax errors, which the old parser refused with 5 (old dispatch.go:231). Reversible.
- tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70. Unchanged from old dispatch.go:196.
- kit default = "worker" @ cli.go:70 and :67. Unchanged from old dispatch.go:197 and stamponly.go:22.
- root default = "." @ cli.go:71. Unchanged from old dispatch.go:199.
- pr and attestation defaults = 0 @ cli.go:57. Unchanged.
- Positional caps = MaximumNArgs(1) and NoArgs @ cli.go:98 and :100.
- GoFlagCompat = true @ cli.go:199.
- Mutation-suite timeout = 120s @ desktools-v2-21.json:2. This is a test knob.
All of these are reversible by an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (cli-contract.md:23), and the dispatch matrix records the move from 5 to 2 for syntax errors only. Domain refusals keep 5 and unverifiable states keep 6, so callers keyed on 5 or 6 for custody outcomes see no change.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70 — byte-identical to the pre-migration default (old dispatch.go:196); the brief requires an unchanged worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: FAIL** — row 8 only, and an environment fault rather than a deliverable defect: TestReg1145ShimCredential fails because CELLCTL_GH_AMBIENT reaches it from the house statusgen shim. With that variable absent the floor passes 26 of 26 seeds. Every other row passes in the witness except row 10, which could not run on this host and passes by hand network-off. Recommend blocked on the fixture's hermeticity, not implementer rework.

### Verifier pass — 2026-10-19 (merged main 8bc28e8edf84, re-verify after medici-finance/assay#3463)

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:198793304723 | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:0d3017d85629 | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:8f1f6a0cbe16 | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:08978cb002c1 | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:026ad47edd53 | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 8bc28e8edf84 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3472 run=cca706615c2cc460cd4348515185b31d29b279bd9804beb0312f280397cf8a7f source=8bc28e8edf8406a285833033dfee8725fa4ccb76 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

The execution witness above was run once with writes, under the house statusgen shim, which exports CELLCTL_GH_AMBIENT into the witness environment. Rows 1 to 9 passed in it and row 10 could not run on this host. The by-hand observations below are separate from the witness. The source includes 53d036291 (medici-finance/assay#3463), which clears inherited CELLCTL_* variables before the floor rows run.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | Verify row 1 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLIHelpOffline` once, 126 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLIConfigFlow` once, 31 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLILegacyForms` once, 856 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command as written | exit 0, `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count | witness: pass exit=0; by hand: exit 0, last line `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec has 10 mutations plus 1 control | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 command as written | exit 0 | witness: pass exit=0 | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 command as written | exit 0 | witness: pass exit=0 | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 command as written | exit 0, named PASS | witness: pass exit=0, `--- PASS: TestCLIOwnerMigrated` printed | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 command as written | exit 0, every floor seed passes | witness: pass exit=0, with CELLCTL_GH_AMBIENT present in the witness environment from the house shim (the condition that failed the 2026-10-19 pass at e4d474af8bca). By hand with no CELLCTL_* names in the environment: exit 0, `seed passes=26`, `--- PASS: TestReg1145ShimCredential`. By hand with a dummy CELLCTL_GH_AMBIENT set: exit 0, `seed passes=26`, `--- PASS: TestReg1145ShimCredential`. The earlier failure no longer reproduces. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 command as written | exit 0 | witness: pass exit=0. A supplementary pinned consumers check (brief named, delivery 66dbced7188e, base its first parent 418592cb5b78) exited 2 with `COULD-NOT-CHECK: brief must resolve uniquely in the delivered snapshot`; it is outside the row and is neither a pass nor a fail | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10's go test, run by hand under darwin sandbox-exec with a deny-network profile, its output written to a scratch file | exit 0, named PASS | witness: could-not-run (the unshare sandbox is Linux-only; this host is darwin). By hand, network-off: test exit 0, `--- PASS: TestCLIHelpDoesNotDiscoverManifests (19.94s)`, grep exit 0. A curl probe in the same profile failed to connect (exit 7), which confirms the sandbox. The row's own mktemp-and-trap wrapper was not used by hand, because a local safety check refused its removal step inside a sandboxed shell script. This by-hand row is distinct from the witness; the CI lane owns the network-off rerun. | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

Review-note checks: no Go standard-library flag import remains in any file of the deskdispatch package (grep exit 1). The dispatch CLI matrix records that syntax errors move from exit 5 to exit 2 and domain refusals stay at 5 (dispatch-cli-matrix.md lines 59-62; cli-contract.md line 23).

Risk-bearing values: frontmatter risk metadata is present with all four fields "no", and the gate is model. Enumerated over the delivering diff (66dbced7188e^1..66dbced7188e) for the deskdispatch package and the mutation spec:
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18. It now applies to deskdispatch syntax errors, which the old parser refused with 5. Reversible.
- tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70. Same as the old dispatch.go:196.
- kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:70 and :67. Same as the old dispatch.go:197.
- root default = "." @ tools/desk/cmd/deskdispatch/cli.go:71. Same as the old dispatch.go:199.
- pr and attestation defaults = 0 @ tools/desk/cmd/deskdispatch/cli.go:57. Same as the old dispatch.go:207.
- Positional caps = MaximumNArgs(1) and NoArgs @ tools/desk/cmd/deskdispatch/cli.go:98 and :100.
- GoFlagCompat = true @ tools/desk/cmd/deskdispatch/cli.go:199.
- Mutation-suite timeout = 120s @ desktools-v2-21.json:2. This is a test knob.
Each of these can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (cli-contract.md:23). The dispatch matrix limits the move from 5 to 2 to syntax errors. Domain refusals keep 5 and unavailable reads keep 6, so callers keyed on 5 or 6 for custody outcomes see no change.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70 — identical to the pre-migration default (old dispatch.go:196). The brief requires the same worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: PASS** — rows 1-9 pass in the witness, and row 8 now passes with CELLCTL_GH_AMBIENT inherited. Row 10 is check:ci: it could not run in the witness on darwin and passes by hand network-off. Its network-off rerun belongs to the CI lane.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 sandbox=container-netns | sha256:eac64beff788 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 sandbox=container-netns | sha256:4ea9bfb7394a | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 sandbox=container-netns | sha256:079e29e7466b | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 sandbox=container-netns | sha256:f88f81ddef3d | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 sandbox=container-netns | sha256:40df24304ba3 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:ec8e26753941 | 2026-10-19 | assay-verifier-app[bot] @ 0e661b6e15de (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3521 run=2df8a7e4a1a23bb45933a1e6effcb0d1514af48e1800dd85641a87351896dc29 source=0e661b6e15de107efcf05ed7d8d0d21983eb3afa brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

### Verifier pass — 2026-10-19 (merged main 0e661b6e15de, re-wake re-verify)

The execution witness above ran at source 0e661b6e15de107efcf05ed7d8d0d21983eb3afa in a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47), started with `--network none`, verifyrun sandbox=container-netns with the engine's default seccomp profile, GOPROXY=off and GOTOOLCHAIN=local. Admission is the Verification-Attestation line above (medici-finance/assay#3521); the dispatcher admission check returned 0 on the pre-witness run and on the post-witness run. This runner follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). The prior Evidence verdict was PASS on 2026-10-19 at 8bc28e8edf84, landed with an environment hold, since the darwin host could not execute row 10. Since then the brief gained the perf-v3 delivery amendment (version 3, Verify table unchanged), and the deskdispatch package changed only in the verifier prompt text and its test (840e269e0, medici-finance/assay#3464). The by-hand runs below were made on the darwin host in this home with network allowed, one at a time, with row 4 run alone; they are separate from the witness.

| # | Command | Expected | Observed | Date / runner |
|---|---|---|---|---|
| 1 | Verify row 1 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLIHelpOffline` once, 126 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 2 | Verify row 2 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLIConfigFlow` once, 31 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 3 | Verify row 3 command as written | exit 0, one top-level PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLILegacyForms` once, 856 subtests PASS, 0 FAIL | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 4 | Verify row 4 command as written | exit 0, `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count | witness: pass exit=0; by hand: exit 0, last line `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec has 10 mutations plus 1 control | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 5 | Verify row 5 command as written | exit 0 | witness: pass exit=0; by hand: exit 0, `ok` for the deskdispatch package | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 6 | Verify row 6 command as written | exit 0 | witness: pass exit=0; by hand: exit 0, no output | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 7 | Verify row 7 command as written | exit 0, named PASS | witness: pass exit=0; by hand: exit 0, `--- PASS: TestCLIOwnerMigrated` printed | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 8 | Verify row 8 command as written | exit 0, every floor seed passes | witness: pass exit=0 network-off; by hand: exit 0, `seed passes=26`, `--- PASS: TestReg1145ShimCredential` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 9 | Verify row 9 command as written | exit 0 | witness: pass exit=0; by hand: exit 0, summary `0 corroborated, 0 disproved, 8 unchecked` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |
| 10 | Verify row 10 command as written | exit 0, named PASS | witness: pass exit=0 under container-netns with `--network none`, the network-off execution this check:ci row requires; by hand with network allowed: exit 0, `--- PASS: TestCLIHelpDoesNotDiscoverManifests` | 2026-10-19 assay-verifier-app[bot] (claude-opus-5-5) (on-behalf-of human:ian) |

No row failed in the witness or by hand, so no failure classification applies.

Risk-bearing values: frontmatter risk metadata is present with all four fields "no", and the gate is model. Enumerated over the delivering diff for the deskdispatch package and the mutation spec, plus the change since the prior pass (8bc28e8edf84..0e661b6e15de, which adds no literal constant to the package; prompt.go:512 only adds the BLOCKED marker to the verifier assignment text):
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18. Syntax errors exit 2; domain refusals keep 5. Reversible.
- tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70. Same as the pre-migration default.
- kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:70. Same as the pre-migration default.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:18 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (cli-contract.md:23), and it is the code Go's flag package already used; domain refusals keep 5 and unavailable reads keep 6, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:70 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: PASS** — all 10 Verify rows pass with 0 fail in the network-off container witness at 0e661b6e15de, row 10 included, and every row also passes by hand.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 sandbox=container-netns | sha256:f6ef0bc66633 | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 sandbox=container-netns | sha256:51e0cd197348 | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 sandbox=container-netns | sha256:0c9e5268a24f | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 sandbox=container-netns | sha256:25342791aba4 | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 sandbox=container-netns | sha256:68dde192c481 | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:87ccea60d99e | 2026-10-20 | assay-verifier-app[bot] @ b787e831d0be (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#3600 run=c4298e4ba80ddd369b46afbbe3b44f8c058b25340813832f1a4cedeabd613946 source=b787e831d0be847f1ca81cc48496c8c0deb01fe5 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

### Verifier pass — 2026-10-20 (merged main b787e831d0be, re-wake re-verify)

The execution witness above ran at source b787e831d0be847f1ca81cc48496c8c0deb01fe5 in a local docker container, image assay-verify:go1.27 (id sha256:955fd6c46c93, built FROM golang:1.27.2-trixie, git 2.47), started with `--network none`, verifyrun sandbox=container-netns with the engine's default seccomp profile. Admission is the Verification-Attestation line above (medici-finance/assay#3600); the dispatcher admission check returned 0 before and after the witness run. This runner follows the driver's 2026-10-19 interim-runner and substance-over-environment rulings (recorded on the private house tracker). This pass was woken because two deskdispatch inputs changed since 0e661b6e15de: the worker prompt reference and the worker prompt objective reference, each gaining a 12-line "Architecture pickup" prose section. No Go source, test, mutation spec or registry row in the deliverable changed, and the Verify table is unchanged.

The substance is the existing by-hand Evidence block of 2026-10-19 at source 0e661b6e15de107efcf05ed7d8d0d21983eb3afa, where every Verify row passed by hand with 0 fail; this pass did not re-run the rows by hand.

Risk-bearing values: frontmatter risk metadata is present with all four fields "no", and the gate is model. Enumeration over the change since the prior pass (0e661b6e15de..b787e831d0be, deliverable paths) found no literal constant: the two reference files gained prose only. The values enumerated and derived in the earlier passes (ExitUsage = 2 at tools/desk/internal/cli/cli.go:18; tier default = "any" at tools/desk/cmd/deskdispatch/cli.go:70) are unchanged at this source.

RISK-VALUE: N/A — enumeration over 0e661b6e15de..b787e831d0be for the deskdispatch package, the clicontract mutation spec and the migration registry found no new or changed literal; the earlier DERIVED entries stand unchanged.

Closure note: row 9's command, as defined, has no explicit --brief, --delivery or --base. The shared consumer-delivery-range lint reports that as a NOTICE today and promotes it to a PROBLEM when a change closes this brief (simulated in scratch with the stream row set to verified: one PROBLEM, on row 9 only). This is a check-definition gap tracked by the consumer-range class issue medici-finance/assay#1915, not a deliverable defect. Outside the Verify table, the pinned form (--brief desktools-v2/21, --delivery 66dbced7188e, --base its first parent 418592cb5b78) exited 0 with summary 8 corroborated, 0 disproved, 0 unchecked.

**VERIFY: PASS** — all 10 Verify rows pass with 0 fail in the network-off container witness at b787e831d0be; the verified closure still needs row 9 re-authored with pinned consumer endpoints (medici-finance/assay#1915).

### Verifier pass — 2026-10-21 (merged main 471fa0febe6d, re-wake re-verify)

Source 471fa0febe6d4370b973cf2380a0513f6a1d7efd. Admission: the dispatcher admission check (`deskdispatch --check-verifier`) returned 0 before any row ran (medici-finance/assay#3985). Why this pass: since b787e831d0be the deliverable changed in Go source. The perf-v3 common binding landed (shared `--perf-*` persistent flags on the root, a typed performance context, `PerfRunMeasured` as the one measurement finalizer, an owner-only performance handoff sidecar). Alias-held claim handling was added, and the mutation spec was re-targeted at the new `PerfRunMeasured` call site. The Verify table is unchanged. Every row was re-run.

Environment: each row ran exactly as written, from the repository root, in a local docker container started with `--network none` and `--rm`, as uid 65534 (passwd entry `nobody`), with GOPROXY=off, GOTOOLCHAIN=local, an offline read-only module cache and a read-only roster mount. The source was a full-history local clone at the verified sha, with origin/main set to it and the origin URL set to the home's. Tree identity with the home was proven by a recursive diff that excluded only the git directory (no differences). statusgen was built in-container from the clone's own statusgen module. Rows 1-7, 9 and 10 ran in image assay-verify:go1.27-py (id sha256:9951b77a257f, go 1.27.2, git 2.47.3). Row 8 ran in image assay-verify:go1.27 (id sha256:955fd6c46c93, same go and git), for the reason given at row 8.

| # | Command | Expect | Observed | Date | Runner |
|---|---|---|---|---|---|
| 1 | Verify row 1 command as written | exit 0, one top-level TestCLIHelpOffline PASS | exit 0; the grep count of the top-level PASS line was 1 | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 2 | Verify row 2 command as written | exit 0, one top-level TestCLIConfigFlow PASS | exit 0; the grep count of the top-level PASS line was 1 | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 3 | Verify row 3 command as written | exit 0, one top-level TestCLILegacyForms PASS | exit 0; the grep count of the top-level PASS line was 1 | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 4 | Verify row 4 command as written | exit 0; last line `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N = spec mutation count | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`; last line `Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec holds 10 mutations | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 5 | Verify row 5 command as written | exit 0 | exit 0; `ok` for the deskdispatch package | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 6 | Verify row 6 command as written | exit 0 | exit 0, no output | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 7 | Verify row 7 command as written | exit 0, named PASS | exit 0; `--- PASS: TestCLIOwnerMigrated` | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 8 | Verify row 8 command as written | exit 0, every floor seed passes | assay-verify:go1.27 image: exit 0, `seed passes=26`. The go1.27-py image gave exit 1 on one fixture assertion only (the no-gh case of the cellctl gen-shims gh-token suite under TestReg1145ShimCredential). That case sets PATH to /usr/bin:/bin and requires gh to be absent, but that image ships gh at /usr/bin/gh. This is an environment precondition, not a deliverable defect. | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 9 | Verify row 9 command as written | exit 0, no consumer routing disproved | exit 0; roster loaded; `consumers: no brief files in the diff against 471fa0febe6d… — nothing to corroborate` | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |
| 10 | Verify row 10 command as written (check:ci) | exit 0, named PASS | exit 0 network-off in the go1.27-py image as uid 65534; `--- PASS: TestCLIHelpDoesNotDiscoverManifests` | 2026-10-21 | assay-verifier-app[bot] @ 471fa0febe6d (claude-opus-5-5 verifier) (on-behalf-of human:ian) |

Totals: 10 rows, 10 pass, 0 fail, 0 could-not-run.

Run notes. A first run used a clone whose origin URL was the local checkout path. Row 5 then failed with 48 tests, all of them in the model-stamp forge resolution step: the resolver reads the current checkout's origin host when the roster has no forge entry. With the origin URL set to the home's, row 5 passed. The deskdispatch tests do not pin that input, so a checkout with a non-forge origin fails them. This is a test-hermeticity gap in a resolver this change did not touch, not a deliverable defect. Row 9's roster was first refused because the mount's parent directory was root-owned; the recorded row ran with an owned, read-only roster directory.

Supplemental, outside the Verify table, discharging no row: the pinned consumer form, with --brief desktools-v2/21, --delivery 66dbced7188e (full sha) and --base its first parent 418592cb5b78 (full sha), exited 0 with summary 8 corroborated, 0 disproved, 0 unchecked. Row 9's command as defined still has no pinned endpoints (medici-finance/assay#1915).

Review ask: the deskdispatch package's non-test Go sources hold no `flag.NewFlagSet` or `"flag"` import, so the old parser is gone. Legacy forms are covered by row 3, and their mutation by row 4.

Risk-bearing values: frontmatter risk metadata is present, with all four fields "no", and the gate is model. Enumerated over b787e831d0be..471fa0febe6d for the deskdispatch package, the shared CLI package, the mutation spec and the migration registry, plus the values named in the earlier passes:
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 (moved from line 18, value unchanged). Reversible.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73 (unchanged values). Reversible.
- MkdirAll mode = 0700 @ tools/desk/cmd/deskdispatch/performance_handoff.go:28; group/other permission refusal mask = 0077 @ performance_handoff.go:32; sidecar file mode = 0o600 @ tools/desk/internal/packet/packet.go:747. Reversible by edit, but an over-wide mode would expose local attribution metadata until fixed.
- expiresBeforeUtc = today + 7 days @ performance_handoff.go:49; deletion cutoff = today - 6 days @ performance_handoff.go:56. Reversible.
- performanceSuffix = ".perf-context.json" @ performance_handoff.go:14; perfNames = 8 canonical `--perf-*` flag names @ tools/desk/internal/cli/performance.go:13; child role mapping implement/review/verify @ tools/desk/cmd/deskdispatch/dispatch.go:229. Reversible naming.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (cli-contract.md:23); domain refusals keep 5 and unavailable reads keep 6, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation.
RISK-VALUE: DERIVED — MkdirAll mode = 0700, refusal mask = 0077 @ tools/desk/cmd/deskdispatch/performance_handoff.go:28 and :32 (file 0o600 @ tools/desk/internal/packet/packet.go:747) — the perf-v3 contract requires private launcher state with 0700 directories and 0600 files (docs/performance-attribution-spec.md:114-115); the mask refuses any group or other bit, which is exactly that requirement.
RISK-VALUE: DERIVED — expiry = today + 7 days, cutoff = today - 6 days @ tools/desk/cmd/deskdispatch/performance_handoff.go:49 and :56 — the same contract caps retention at seven UTC days (docs/performance-attribution-spec.md:115); a sidecar kept from the start of day minus 6 lives under seven UTC days, and its stated expiry is at most seven days after it was written.

Verdict of the 2026-10-21 pass, kept as dated history: PASS — all 10 Verify rows pass with 0 fail at 471fa0febe6d, network-off as uid 65534. Row 10 (check:ci) ran in the go1.27-py image. Row 8 ran in the gh-free go1.27 image, because the go1.27-py image ships gh, which breaks one fixture's no-gh precondition. The re-verification below supersedes it.
Verification-Attestation: medici-finance/assay#3985 run=4a73c7016557fd11f6e672024a6b345e364ea8d6de11a3b7961a54e78fdd48a2 source=471fa0febe6d4370b973cf2380a0513f6a1d7efd brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

### Re-verification — 2026-10-25 (merged main e4d2346d8b51)

Source e4d2346d8b51fbc893551107bc2b48df4d3f2c63. Rows 1-9 are witnessed on the host by `statusgen verifyrun --rows`; row 10 (check:ci) is witnessed in a network-off container inside an admission bracket. Both tables follow.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | could-not-run exit=- — timed out after 10m0s — no verdict was produced | sha256:57cafdc968ab | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:3a3048649324 | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:1dd3616f0465 | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:5f5bea3739f8 | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:5ff365cc4d30 | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4386 run=7c8625276d195b6be30fdd6ccefda1ecf78e99b50d32258f4c1c80813a404333 source=e4d2346d8b51fbc893551107bc2b48df4d3f2c63 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:cf49973f9431 | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4386 run=7c8625276d195b6be30fdd6ccefda1ecf78e99b50d32258f4c1c80813a404333 source=e4d2346d8b51fbc893551107bc2b48df4d3f2c63 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:bd8cc0d2292c | 2026-10-25 | assay-verifier-app[bot] @ e4d2346d8b51+dirty (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:0bc3c269f7d9ffebb82708f4c5019a458705fdc04f7fc798d3cb547056a79da9
Container-Image: sha256:840101ee78c4f508da9d7e035a553fa0c1e5e4f73bf3dd4859091d55dd7687f4
Verification-Attestation: medici-finance/assay#4386 run=7c8625276d195b6be30fdd6ccefda1ecf78e99b50d32258f4c1c80813a404333 source=e4d2346d8b51fbc893551107bc2b48df4d3f2c63 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=7c8625276d195b6be30fdd6ccefda1ecf78e99b50d32258f4c1c80813a404333 source=e4d2346d8b51fbc893551107bc2b48df4d3f2c63 tree=sha256:0bc3c269f7d9ffebb82708f4c5019a458705fdc04f7fc798d3cb547056a79da9 witness=sha256:f502ab5f85ae2f35b7d1b2756019aa36b05f8d11e593854a3a2a767b816121dc opened=2026-10-25T14:45:31Z closed=2026-10-25T14:50:01Z before=receipt after=check-verifier

The first host witness above ran rows 1-9 at source e4d2346d8b51 while the host was heavily loaded by other test runs. Row 4, the mutation suite, hit verifyrun's default 10-minute per-row limit and was recorded as could-not-run. Row 4 was then re-run alone with `statusgen verifyrun --rows 4 --timeout 50m`, which changes only the time limit and leaves the row command as written. That second table is row 4's live witness: pass, exit 0. Row 10 is witnessed in the bracketed table directly above, in a local docker container. The container ran with `--network none`, the engine's default seccomp profile, GOPROXY=off and GOTOOLCHAIN=local, from image sha256:840101ee78c4 (go1.27.2 linux/amd64). The home was bind-mounted read-only. The bracket closed with tree digest match and `--bracket-digest` verdict=match. Since the prior pass (source 471fa0febe6d), the deskdispatch package gained recorded-ruling, review-context and review-round reading, plus the outcome-record path checks and their tests. Every row reruns against that code. The admission check returned 0 before the host witness, after the evidence edit, and at both bracket ends.

No row failed, so no failure classification applies.

Risk-bearing values: frontmatter risk metadata is present with all four fields "no", and the gate is model. Enumerated over the change since the prior pass (471fa0febe6d..e4d2346d8b51) on the deskdispatch package, the mutation spec and the shared CLI package. The deskdispatch rows of the CLI registry are unchanged.
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged; syntax errors exit 2 and domain refusals keep 5.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged from the pre-migration defaults.
- Outcome-record directory created and re-moded 0o700 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :141. Refused when group or other can write it (mask 0o022) @ outcomepath.go:164. Directory name "outcomes" @ outcomepath.go:25. New.
- Review packet diff input cap = 4 MiB @ tools/desk/cmd/deskdispatch/packet_review.go:50, and review-context read limit = 16 MiB @ reviewcontext.go:138 and :142. New size caps.
- Recorded-ruling summary truncation = 160 characters @ tools/desk/cmd/deskdispatch/recordedruling.go:77, decisions directory "docs/streams/decisions" @ recordedruling.go:60, and status binary name "statusgen" @ recordedruling.go:265. New.
- Review preparation version tag = "review-prepare-v2" @ tools/desk/cmd/deskdispatch/reviewcontext.go:49. New.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md:23); domain refusals keep 5 and unavailable reads keep 6, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — outcome-record directory mode 0o700 with a refusal on mask 0o022 @ tools/desk/cmd/deskdispatch/outcomepath.go:135 and :164 — the outcome record is verifier custody input, so an owner-only directory matches the private-state convention, and the group/other-write refusal stops another local user from planting a record there.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: PASS** — all 10 Verify rows pass at merged main e4d2346d8b51. Rows 1-3 and 5-9 pass in the host witness. Row 4 passes on its re-run with a longer time limit. Row 10 passes in the bracketed network-off container witness (bracket verdict match). 0 fail, 0 could-not-run after the re-run.
### Re-verification — 2026-10-27 (merged main 79e67f655798)

Source 79e67f655798746322990d3f8da19cbef8bd1218. Rows 1-9 are witnessed on the host by `statusgen verifyrun --rows 1,...,9 --fail-first --base 418592cb5b78 --timeout 50m`; row 10 (check:ci) is witnessed in a network-off container inside an admission bracket. Both tables follow.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:7a399bd7a43b | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:c91919ec693a | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:a8a3f95a00ad | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:db1eb384de55 | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:4492200b4ea7 | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) | base=- not-selected · head=79e67f655798 |
Verification-Attestation: medici-finance/assay#4695 run=b43c33891ad4774e3cdcff0c1c5fccb1ad244d67e05a1902863b310447bca659 source=79e67f655798746322990d3f8da19cbef8bd1218 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:3d24c285dd8b | 2026-10-27 | assay-verifier-app[bot] @ 79e67f655798 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:0af152f107713b6dc6342a5be7a5e2024c3363254189e510b3d9c6bc86c635aa
Container-Image: sha256:3a3cb3913f1311f1a9bf13ddd53c4f27361566f74ac90c89f78991e5452644b8
Verification-Attestation: medici-finance/assay#4695 run=b43c33891ad4774e3cdcff0c1c5fccb1ad244d67e05a1902863b310447bca659 source=79e67f655798746322990d3f8da19cbef8bd1218 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=b43c33891ad4774e3cdcff0c1c5fccb1ad244d67e05a1902863b310447bca659 source=79e67f655798746322990d3f8da19cbef8bd1218 tree=sha256:0af152f107713b6dc6342a5be7a5e2024c3363254189e510b3d9c6bc86c635aa witness=sha256:581b8d2600cb8286dcda316d2ab50340c10e0189a03e751d81dcd026421d8cd7 opened=2026-10-27T22:52:24Z closed=2026-10-27T22:55:24Z before=receipt after=check-verifier

The fail-first base is 418592cb5b78, the parent of the delivering merge 66dbced7188e (medici-finance/assay#3302, read from the forge). The brief's risk block has every field "no" and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so `--fail-first` selected no row ("no risk-bearing row among the selected rows; every row is recorded not-selected at base"). Every row's Base cell records that. The `--timeout 50m` flag changes only the per-row time limit; no row command was changed, filtered or shortened. Row 4, the mutation suite, finished inside it.

Row 10 ran in the release-pinned runner image named by the v1.0.63 release asset verify-runner-image.txt (checksum matched against checksums.txt): verify-runner v1.0.63 at sha256:3a3cb3913f13, go1.27.2 linux/amd64. The container ran with `--network none`, the engine's default seccomp profile, GOPROXY=off and GOTOOLCHAIN=local. The home was bind-mounted read-only. The host's Go module cache was also bind-mounted read-only at the image's module-cache path. Without it the build cannot resolve: the image's cache predates module versions that main now requires. The bracket closed with tree digest match, and `--bracket-digest` reported verdict=match.

Discarded attempts, disclosed and not landed:
- A first container invocation mis-spelled the home bind mount, so the container saw an empty root. verifyrun refused admission before any row ran ("no Verify rows executed"), and nothing was witnessed.
- A second bracket ran row 10 without the module-cache mount. The witness recorded fail exit=1 (sha256:04e5699ffabc). A separately labelled probe in the same image showed the cause: setup failed with "module lookup disabled by GOPROXY=off" for modules missing from the image's cache, such as golang.org/x/net v0.61.0. That is an environment gap, not a test result, so its bracket was never closed and its witness is not landed.
A third probe, with the module cache mounted, passed the named test. It carries no acceptance credit; the bracketed table above is row 10's witness.

The admission check returned 0 before any row ran, at both bracket ends, and before landing.

No row failed, so no failure classification applies.

Risk-bearing values were enumerated over the change since the prior pass (e4d2346d8b51..79e67f655798), covering the deskdispatch package, the shared CLI package and the CLI registry. The shared CLI package (tools/desk/internal/cli) is unchanged. The deskdispatch CLI definition (cli.go) is unchanged. The registry diff touches only rows owned by other briefs (/22, /23, /35) and two library/test classifications, and this brief's rows are unchanged. The deskdispatch source changes route frontmatter reads through the shared brief parser, add a finished-home adoption flag for rework homes, and bind the verifier prerequisite record.
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged.
- gate value for unreadable frontmatter = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. New; it was "" (no gate) before.
- rework adoption binding: "--adopt-run" with the claim key the run already owns @ tools/desk/cmd/deskdispatch/reviewworktree.go:41. New.
- verifier prerequisite record bound to runnercmds.Commands @ tools/desk/cmd/deskdispatch/verifierattestation.go:62 and :148. New.
- hint discriminator "Not adopted as a finished home" @ tools/desk/cmd/deskdispatch/prompt.go:295. New; it selects hint text only.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md); domain refusals keep their own codes, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated. Reading it as human-gated fails closed: the cost is a spurious decision issue, which is visible and reversible. The alternative "" would let a malformed human-gated brief skip its human gate. An absent file or gate still returns "", and the explicit --gate-human flag is unchanged.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: PASS** — all 10 Verify rows pass at merged main 79e67f655798: rows 1-9 in the host witness, row 10 in the bracketed network-off container witness (bracket verdict match). 0 fail, 0 could-not-run.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
