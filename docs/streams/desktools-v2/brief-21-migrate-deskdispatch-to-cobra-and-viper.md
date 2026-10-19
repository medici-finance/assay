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
version: 2
id: "2969ba49-706d-4544-b46b-e8b87b133bde"
authored: "2026-10-04 by desktools-v2/15"
sources:
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

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
