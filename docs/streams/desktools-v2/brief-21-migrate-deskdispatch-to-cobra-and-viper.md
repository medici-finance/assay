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
| 4 | check +mutation | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-21.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests; the command compares muhar's last line itself (`n` is the spec's mutation count), because muhar exits 0 when a mutation survives or cannot be applied |
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
### Re-verification — 2026-10-30 (merged main 40925e8233c8)

Source 40925e8233c8a63d70682e77ca333e665f1f955e. Rows 1-9 are witnessed on the host by `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 50m`; row 10 (check:ci) is witnessed in a network-off container inside an admission bracket. Both tables follow.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:95e2be966a8c | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:573d373d4532 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:aeec296da65a | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:dae53bf53559 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:ca38fbc4dc1b | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#4961 run=d8ceebfd3633ba5f7109cbd48340a4db67ae2a3c81ca5c162c21bd389643ce45 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:7d99c7631453 | 2026-10-30 | assay-verifier-app[bot] @ 40925e8233c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:e13846df9f19dcadc9c0ba1c72d1efab7ae45eec63f45897c7c2d26fcf9410b8
Container-Image: sha256:29494cbeca300a29295a2eeed7398500675cb63c7911d660a08c899e8584d175
Verification-Attestation: medici-finance/assay#4961 run=d8ceebfd3633ba5f7109cbd48340a4db67ae2a3c81ca5c162c21bd389643ce45 source=40925e8233c8a63d70682e77ca333e665f1f955e brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d8ceebfd3633ba5f7109cbd48340a4db67ae2a3c81ca5c162c21bd389643ce45 source=40925e8233c8a63d70682e77ca333e665f1f955e tree=sha256:e13846df9f19dcadc9c0ba1c72d1efab7ae45eec63f45897c7c2d26fcf9410b8 witness=sha256:bbbcc4e86f2abac0b41ee6988ebb8331180505354d8d8480465eba447dc532f2 opened=2026-10-30T02:41:15Z closed=2026-10-30T02:42:54Z before=receipt after=check-verifier

The fail-first pass was left out on purpose. The brief's risk block has every field "no", and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row has a fail-first obligation; the 2026-10-27 pass reached the same reading with `--fail-first` (every row recorded not-selected at base). The `--timeout 50m` flag changes only the per-row time limit; no row command was changed, filtered or shortened. Row 4 finished inside it.

Row 4 supplemental read, no acceptance credit: the mutation harness exits 0 unless it is broken (red baseline or uncaught positive control). An uncaught mutation alone does not make it exit non-zero, so the witnessed exit 0 does not show the Expect's Totals line by itself. A labelled re-run of the exact row 4 command at the same source and home, after the bracket closed, printed "Harness healthy: baseline GREEN, positive control CAUGHT." and last line "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec lists 10 mutations, so N equals the spec's count. Its output hash differs from the witness (the harness log carries timings), so the witness row above stays row 4's acceptance record and this read only discharges the Totals clause of its Expect. The home's tree was unchanged after the re-run.

Row 10 ran in the release-pinned runner image named by the v1.0.66 release asset verify-runner-image.txt (checksum matched against checksums.txt): verify-runner v1.0.66 at sha256:29494cbeca30, go1.27.3 linux/amd64. The container ran with `--network none`, the engine's default seccomp profile, GOPROXY=off and GOTOOLCHAIN=local. The home and its git directory were bind-mounted read-only. The host's Go module cache was bind-mounted read-only at the image's module-cache path, as in the prior passes. The bracket closed with a tree digest match, and `--bracket-digest` reported verdict=match. No bracket attempt was discarded.

The admission check returned 0 before any row ran and at both bracket ends.

No row failed, so no failure classification applies.

Risk-bearing values were enumerated over the change since the last landed pass (79e67f655798..40925e8233c8), covering the deskdispatch package, the shared CLI package, this brief's mutation spec, the CLI contract and the CLI registry. The shared CLI package, the CLI contract, the mutation spec and the deskdispatch files cli.go and brief.go are unchanged. The deskdispatch production changes (packet_verifier.go, reviewcontext.go, worktree.go) only move the gitcore import from the desk module's internal copy to the shared gitcore module. The test changes move gittest imports and replace a hand-written .git/HEAD fixture with a stub repository. That diff introduces or changes no literal. The registry diff adds rows owned by other briefs (/50, /55, /57 and two other streams), and this brief's rows are unchanged. The brief's standing values are:
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged.
- unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. Unchanged.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed. The cost is a spurious decision issue, which is visible and reversible; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.

**VERIFY: PASS** — all 10 Verify rows pass at merged main 40925e8233c8: rows 1-9 in the host witness, row 10 in the bracketed network-off container witness (bracket verdict match). 0 fail, 0 could-not-run.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:410599008c6f | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | fail exit=1 | sha256:9cebdb086935 | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:614666dc9840 | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:19afe410897c | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:c00160ef48c1 | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5124 run=3f8db7f0d6f95f79d2aad2bcacb5b1c2179e4da5954b0a7daa9978cbe541f896 source=99627c1b8084886957217eb072e4e86e6c337977 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:ba141df78a3d | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5124 run=3f8db7f0d6f95f79d2aad2bcacb5b1c2179e4da5954b0a7daa9978cbe541f896 source=99627c1b8084886957217eb072e4e86e6c337977 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | could-not-run exit=- sandbox=unshare — check:ci hermetic execution requires a network-off sandbox, unavailable on this host: the network sandbox uses `unshare --net`, a Linux facility, and this host is darwin. check:ci rows are re-executed network-off by design (verdict-lane/02, R-6 c.6) — run on a Linux runner that provides `unshare --net` | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 99627c1b8084 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5124 run=3f8db7f0d6f95f79d2aad2bcacb5b1c2179e4da5954b0a7daa9978cbe541f896 source=99627c1b8084886957217eb072e4e86e6c337977 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong




### Re-verification — 2026-11-01 (merged main 99627c1b8084)

Source 99627c1b8084886957217eb072e4e86e6c337977. The three witness tables above belong to this pass: rows 1-9 from `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 50m`, a row 5 re-witness from `--rows 5`, and row 10 from `--rows 10` on the host. The last witness per row wins. No row command was changed, filtered or shortened.

Discarded attempt, disclosed: the first table's row 5 is fail exit=1. A labelled re-run of the same command named TestReworkAdoptsSameHome and TestReworkAdoptsEarlierHome, and each differs only by the path prefix: got the symlink-resolved path, want the unresolved one (`/tmp` is a symlink on macOS). The verifier caused this by setting TMPDIR=/tmp/v21t, and /tmp is a symlink on darwin. The tests compare the unresolved temporary path with the canonical path git reports, so any symlinked TMPDIR reddens them. Row 5 was re-witnessed with no TMPDIR override, under the managed scratch TMPDIR, and passes. Rows 1-4 and 6-9 keep their witnesses from the first table, which ran under TMPDIR=/tmp/v21t. Test-hermeticity note for the desk, not a failed obligation: the two tests assume an unsymlinked TMPDIR.

Row 10 could-not-run. Its check:ci witness needs a network-off sandbox, darwin has no `unshare --net`, and the container bracket refused at its second step. The bracket opened at 2026-11-01T01:33:55Z with tree sha256:fd24111ee51438acadac1771b180adf845d06cd30f8edefaf82b8bb486184547. The container mounted the home, the receipt, a new witness directory and the Go module cache, but not the linked worktree's git directory. The refusal reads: "statusgen verifyrun: container admission refused; no Verify rows executed: the mounted root (the verifier home) digests to sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855, not the sha256:fd24111ee51438acadac1771b180adf845d06cd30f8edefaf82b8bb486184547 the host recorded at the open step." That digest is the SHA-256 of empty input. The likely cause is the missing git directory, which was not tested. Earlier passes on this brief record mounting it read-only. This pass did not widen the mount or retry after the refusal, so the bracket was never closed and no bracket line exists. The image was verify-runner v1.0.67 at sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa, matched against that release's verify-runner-image.txt and checksums.txt. Needed: a re-dispatch with a sanctioned mount for the bracket.

Supplemental host observations, no acceptance credit. Row 10's test on the host with the network available: --- PASS: TestCLIHelpDoesNotDiscoverManifests (7.64s). Row 4's exit code does not by itself show the Expect's Totals line, so a labelled re-run of the exact command at the same source printed the last line "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate." and exited 0. The spec lists 10 mutations.

No fail-first pass was run. The brief's risk block has every field "no" and no Verify row carries a risk-bearing, live, mutating or end-to-end tag. Protected-table re-derivation reported 10 rows, 0 author-added, base 99627c1b8084. The admission check returned 0 before any row ran, and again after the row 5 re-witness and after the row 10 witness.

Risk-bearing values were enumerated over the change since the last landed pass (40925e8233c8..99627c1b8084), covering the deskdispatch package, the shared CLI package, the CLI contract tests and spec directory, the CLI contract and the CLI registry. The registry change flips desktools-v2/46 rows only. New in the review prompt: digest form = 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 and :325, and digest algorithm = SHA-256 @ the same file, :325. Carried from earlier passes, in files this change does not touch: ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20, tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73, and unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs; domain refusals keep their own codes, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed. The cost is a spurious decision issue, which is visible and reversible; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.
RISK-VALUE: DERIVED — digest form = 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 — a SHA-256 digest is 256 bits, which is 64 hex characters, so the form follows from the algorithm the same prompt names at :325.

**VERIFY: BLOCKED** — 9 of 10 Verify rows pass at merged main 99627c1b8084 (rows 1-9), 0 fail, 1 could-not-run: row 10 (check:ci), because the container bracket refused admission. The host read of row 10's test carries no acceptance credit.

### Re-verification — 2026-11-01 (merged main 3758084fc1f0)

Source 3758084fc1f0d63c28457fc1aebf0c518a7c55e5. Rows 1-9 are witnessed on the host by `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 50m`, with a non-symlink TMPDIR outside the verifier home; row 10 (check:ci) is witnessed in a network-off container inside an admission bracket. Both tables follow. This pass reuses no earlier witness.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:dbea09a7232f | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:7ec4a6ca5f50 | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:5730f1e3ed8c | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:0e2b399741fc | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:f6f2da1449fe | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5130 run=435cda4ef53c812a39b91fa86b906f244d3c21d558deda9a930c6ddaa5d9b426 source=3758084fc1f0d63c28457fc1aebf0c518a7c55e5 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:b5d6770a652c | 2026-11-01 | assay-verifier-app[bot] @ 3758084fc1f0 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:b7303e1d73585f85f1f067f8ea8af8dcb6b1e19f69024838663872f06caa5138
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5130 run=435cda4ef53c812a39b91fa86b906f244d3c21d558deda9a930c6ddaa5d9b426 source=3758084fc1f0d63c28457fc1aebf0c518a7c55e5 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=435cda4ef53c812a39b91fa86b906f244d3c21d558deda9a930c6ddaa5d9b426 source=3758084fc1f0d63c28457fc1aebf0c518a7c55e5 tree=sha256:b7303e1d73585f85f1f067f8ea8af8dcb6b1e19f69024838663872f06caa5138 witness=sha256:f1cab527d53576a88ebdfedc4e708531aa1f51e21d6d03a706edece8f6667a55 opened=2026-11-01T02:58:22Z closed=2026-11-01T03:01:02Z before=receipt after=check-verifier

The fail-first pass was left out on purpose. The brief's risk block has every field "no", and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row has a fail-first obligation. The `--timeout` flags change only the per-row time limit; no row command was changed, filtered or shortened.

Row 4 supplemental read, no acceptance credit: the mutation harness exits 0 unless it is broken, so the witnessed exit 0 does not show the Expect's Totals line by itself. A labelled re-run of the exact row 4 command at the same source and home, after the bracket closed, printed "Harness healthy: baseline GREEN, positive control CAUGHT." and last line "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec lists 10 mutations, so N equals the spec's count. The witness row above stays row 4's acceptance record and this read only discharges the Totals clause of its Expect. The re-run left no file changed in the home beyond this Evidence.

Row 10 ran in the release-pinned runner image named by the v1.0.67 release asset verify-runner-image.txt (checksum matched against checksums.txt): verify-runner v1.0.67 at sha256:f073d208c69e, go1.27.3 linux/amd64, statusgen v1.0.67, the same version as the host install. The container ran with `--network none`, the engine's default seccomp profile, a read-only root, GOPROXY=off and GOTOOLCHAIN=local, using the image's own module cache. The verifier home and its git directory were bind-mounted read-only at their host paths and the receipt was mounted read-only. No App keys, tokens or roster were mounted, and the row needed none. The bracket closed with a tree digest match, and `--bracket-digest` reported verdict=match with one each of tree-digest, container-image and attestation in the span.

Attempts disclosed: no bracket step refused and no witness or row result was discarded; each table above comes from the only run of its command under this admission, apart from the labelled row 4 read. Before the container row ran, on the same open receipt, mount probes that ran no Verify row found a spelling error of mine: the shell read the read-only suffix on the home's mount as a path modifier, so the home landed at a wrong path, writable, and the intended path did not exist in the container. The probes ran only git and listing commands and wrote nothing there; the home stayed clean. The mount was respelled and re-probed (home read-only, git toplevel and source correct, a write refused) before the bracket's container step ran. This same fault could also explain the earlier run's empty mounted root recorded in medici-finance/assay PR 5128, though that was not confirmed.

The admission check returned 0 before any row ran and at both bracket ends.

No row failed, so no failure classification applies.

Risk-bearing values were enumerated over the change since the last landed pass (40925e8233c8..3758084fc1f0), covering the deskdispatch package, the shared CLI package, the clicontract mutation specs, the CLI contract and the CLI registry. The shared CLI package, the CLI contract, this brief's mutation spec and the deskdispatch files cli.go and brief.go are unchanged. The registry diff flips five rows owned by desktools-v2/46 to migrated; this brief's rows are unchanged. A new mutation spec belongs to that same brief. The deskdispatch change is the embedded review prompt and its test, which add text for a security fail resting only on the PR body. The only new literal there is the 64-hex digest form below. The standing values are:
- body digest form = SHA-256 / 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 and :325. New in this range; prompt text only, enforced elsewhere.
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged.
- unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. Unchanged.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed. The cost is a spurious decision issue, which is visible and reversible; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.
RISK-VALUE: DERIVED — body digest form = 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 — a SHA-256 digest is 256 bits, which is 64 hex characters; the prompt names that algorithm for the same digest at line 325.

**VERIFY: PASS** — all 10 Verify rows pass at merged main 3758084fc1f0: rows 1-9 in the host witness, row 10 in the bracketed network-off container witness (bracket verdict match). 0 fail, 0 could-not-run.
### Re-verification — 2026-11-01 (merged main 40a7aace82c8)

Source 40a7aace82c8601e6f4042476cb32556a3a12dfc. Rows 1-9 are witnessed on the host by `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 50m`, with a non-symlink TMPDIR outside the verifier home; row 10 (check:ci) is witnessed in a network-off container inside an admission bracket. Both tables follow. This pass reuses no earlier witness.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:0c2ae0c50306 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:8684997ca78f | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:a9f47ef50d36 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:2b4c5f6828d2 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:7dcbe594ced3 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5172 run=d4d68122b8db86295f3d5ac07af7b3ea72e4213d1b83201239f77174dc428954 source=40a7aace82c8601e6f4042476cb32556a3a12dfc brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong


| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:71ff7f9efa78 | 2026-11-01 | assay-verifier-app[bot] @ 40a7aace82c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:68a3d294f0d4cfae0cc869ccca75fce4d4af89f6d4afcec96c8149dddc2dd160
Container-Image: sha256:f073d208c69e1e2f5ce5b680ad536d7a3cef3c80756823b9643cab4f7d1068aa
Verification-Attestation: medici-finance/assay#5172 run=d4d68122b8db86295f3d5ac07af7b3ea72e4213d1b83201239f77174dc428954 source=40a7aace82c8601e6f4042476cb32556a3a12dfc brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=d4d68122b8db86295f3d5ac07af7b3ea72e4213d1b83201239f77174dc428954 source=40a7aace82c8601e6f4042476cb32556a3a12dfc tree=sha256:68a3d294f0d4cfae0cc869ccca75fce4d4af89f6d4afcec96c8149dddc2dd160 witness=sha256:d92182e87ab3fa4ff5ca47a51b92623947545ea264d4af0edcfd0dceb6ae9c30 opened=2026-11-01T19:54:59Z closed=2026-11-01T20:00:58Z before=receipt after=check-verifier

The fail-first pass was left out on purpose. The brief's risk block has every field "no", and no Verify row carries a risk-bearing, live, mutating or end-to-end tag, so no row has a fail-first obligation. The `--timeout` flag changes only the per-row time limit; no row command was changed, filtered or shortened.

Row 4 supplemental read, no acceptance credit: the mutation harness exits 0 unless it is broken, so the witnessed exit 0 does not show the Expect's Totals line by itself. A labelled re-run of the exact row 4 command at the same source and home, after the bracket closed, printed "Harness healthy: baseline GREEN, positive control CAUGHT." and last line "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec lists 10 mutations, so N equals the spec's count. The witness row above stays row 4's acceptance record; this read only discharges the Totals clause of its Expect. Labelled re-runs of rows 5, 7 and 8 after the bracket, also no acceptance credit, printed "ok github.com/medici-finance/assay/tools/desk/cmd/deskdispatch 69.833s", "--- PASS: TestCLIOwnerMigrated (7.12s)" and "seed passes=26". None of these reads changed a file in the home.

Row 9 disclosure: the witnessed command passes as written (exit 0, nothing disproved), but on merged main it has no delivery range to judge. A labelled re-run printed "consumers: no brief files in the diff against 40a7aace82c8601e6f4042476cb32556a3a12dfc — nothing to corroborate". To give the row's Expect real content, a supplemental read with no acceptance credit pinned the delivering change, medici-finance/assay PR 3302 (merge 66dbced7188e, first parent 418592cb5b78): `statusgen --root . --consumers --brief desktools-v2/21 --delivery 66dbced7188e740cf469e893ee023824397c230b --base 418592cb5b784056ccba5b9979a98920cf7f6f02` exited 0 with "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". The home lint carries a consumer-delivery-range NOTICE on row 9 for the missing pinned delivery and base. An earlier pass recorded that this NOTICE becomes a PROBLEM when a change closes the brief; that was not re-simulated here, so the verified flip may need row 9 re-authored in the pinned form. The same lint run exits 1 on one PROBLEM that belongs to another brief: "desktools-v2/65: protected table: acceptance transition: could-not-check human authority: --acceptance-forge is required".

Row 10 ran in the release-pinned runner image named by the v1.0.67 release asset verify-runner-image.txt (checksum matched against checksums.txt): verify-runner v1.0.67 at sha256:f073d208c69e, go1.27.3 linux/amd64 under emulation on an arm64 host, statusgen v1.0.67, the same version as the host install. The container ran with `--network none`, the engine's default seccomp profile, a read-only root with a tmpfs at /tmp, GOPROXY=off and GOTOOLCHAIN=local, using the image's own module cache. The verifier home and its git common directory were bind-mounted read-only at their host paths and the receipt was mounted read-only. No App keys, tokens or roster were mounted, and the row needed none. Engine disclosure: this host's only docker engine is OrbStack. The run used the docker context named default, whose socket path is a link to the OrbStack engine socket. No Kubernetes context or cluster was used. The bracket closed with a tree digest match, and `--bracket-digest` reported verdict=match with one each of tree-digest, container-image and attestation in the span.

Attempts disclosed: under this same open receipt the container row ran twice. The first run carried an extra GOFLAGS=-mod=mod in the container environment, left by a failed edit of my launch script. It passed (output sha256:61059aa5028a), but it was set aside unused because that variable is not part of the row's environment. The second run, whose bytes appear above, used the clean environment. The first close-bracket call, issued from a working directory outside the home, refused with exit 2 and appended nothing: "statusgen verifyrun --close-bracket: refused, nothing appended: the receipt admits brief docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md, but this run's brief is docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md". The identical command re-issued from inside the home closed the bracket. Nothing else was retried or discarded.

The admission check returned 0 before any row ran and again after the bracket closed.

No row failed, so no failure classification applies.

Risk-bearing values were enumerated over the change since the last landed pass (3758084fc1f0..40a7aace82c8). It touches the CLI registry (rows owned by desktools-v2/47, desktools-v2/80 and desk-containers/31; this brief's row is unchanged), the clicontract inventory test (its imports-only migration case now picks a pending row at run time), and a new mutation spec for desktools-v2/47. The shared CLI package, the CLI contract, this brief's mutation spec and the deskdispatch package are unchanged, so no new literal reaches this brief's deliverable. The standing values are:
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged.
- unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. Unchanged.
- body digest form = SHA-256 / 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 and :325. Unchanged; prompt text only.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes, so callers keyed on custody outcomes see no change.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed. The cost is a spurious decision issue, which is visible and reversible; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default; the brief requires the same worktree, branch and claim per invocation, and this default is part of that.
RISK-VALUE: DERIVED — body digest form = 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 — a SHA-256 digest is 256 bits, which is 64 hex characters; the prompt names that algorithm for the same digest at line 325.

**VERIFY: PASS** — all 10 Verify rows pass at merged main 40a7aace82c8: rows 1-9 in the host witness, row 10 in the bracketed network-off container witness (bracket verdict match). 0 fail, 0 could-not-run. Row 9 passes as written but judges no delivery range on merged main; the pinned supplemental read corroborates 8 and disproves 0.

Desk note (2026-11-01): the implemented→verified flip is held. A simulated flip in a scratch clone adds one statusgen lint PROBLEM, `[consumer-delivery-range]` on Verify row 9: the unpinned `--consumers` form has no explicit `--base`. That is a check-definition blocker tracked at medici-finance/assay#1927. The outcome is recorded as blocked (check-definition), and the row stays implemented until the row is re-authored.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 4 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-21.json` | pass exit=0 | sha256:6f6ebee0997b | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:ee6e8c9520a8 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:990ba0f08335 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:150e92308fc3 | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:aa21a4eea11a | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#5620 run=42b98ae91b4957f255cda4e841912e4e4bad86371872cdc069c57d6ae43316a9 source=bc078e9179c862dde02b367d35177f8ffdf1b421 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:81835e45b36e | 2026-11-06 | assay-verifier-app[bot] @ bc078e9179c8 (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:5ecad7390f9dc3c6c9dc1f0374e1fca8a4276f291c3c5345e8c6868e083c00f4
Container-Image: sha256:732786a33ebfa6dc1d80d396781df1f31c5331f67a8c0bd97518785f5a72b173
Verification-Attestation: medici-finance/assay#5620 run=42b98ae91b4957f255cda4e841912e4e4bad86371872cdc069c57d6ae43316a9 source=bc078e9179c862dde02b367d35177f8ffdf1b421 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=42b98ae91b4957f255cda4e841912e4e4bad86371872cdc069c57d6ae43316a9 source=bc078e9179c862dde02b367d35177f8ffdf1b421 tree=sha256:5ecad7390f9dc3c6c9dc1f0374e1fca8a4276f291c3c5345e8c6868e083c00f4 witness=sha256:11ad334e02614f6aa07c3dc2d654143a6327dbe6ef1dc1dc1398cabe63db7fa9 opened=2026-11-06T17:54:36Z closed=2026-11-06T17:55:21Z before=receipt after=check-verifier

Re-verification on 2026-11-06 at merged main bc078e9179c862dde02b367d35177f8ffdf1b421, by a non-implementer verifier: model claude-opus-5-5, tier strong, statusgen and deskdispatch v1.0.75, darwin-arm64. The packet named the same commit. `deskdispatch --check-verifier` returned 0 before any row ran (attestation medici-finance/assay#5620, run 42b98ae91b49). `deskpathguard rederive` reported 10 rows, 0 author-added, base bc078e9179c8. Rows 1-9 (class check) ran on the host through `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 50m` under the normal HOME, with GH_TOKEN unset and KUBECONFIG=/dev/null; their witness is the 9-row table above. Row 10 (class check:ci) cannot use the unshare sandbox on darwin, so it ran inside a closed admission bracket; its witness, with its Admission-Bracket line, is the 1-row block directly above this note.

Row 10 ran in verify-runner v1.0.75 at sha256:732786a33ebf, the digest pinned in the house runner image lock: an arm64 image with go1.27.3, git 2.56.0 and statusgen v1.0.75, the same version as the host install. The container ran with `--network none`, a read-only root with a tmpfs at /tmp, GOPROXY=off, GOTOOLCHAIN=local and a GOCACHE inside that tmpfs. The home and its git common directory were bind-mounted read-only at their host paths, the receipt was mounted read-only, and the only writable mount was the witness directory outside the home. No App keys, tokens or roster were mounted, and the row needed none. The docker context is orbstack. No Kubernetes context or cluster was used. The bracket opened at 17:54:36Z and closed at 17:55:21Z, the close issued from inside the home, and `--bracket-digest` reported verdict=match with one each of tree-digest, container-image and attestation in the span.

Attempts: one assembled attempt covers the table, host rows 1-9 (17:48:15Z to 17:54:19Z) and then the bracketed container row 10 (17:54:36Z to 17:55:21Z). Nothing was retried or discarded. No row failed, so no failure classification applies.

Fail-first: no row owes a fail-first witness. All four risk fields are "no", and the rows carry only the +dereference, +flow and +mutation tags; none is tagged risk-bearing, live, mutating or end-to-end.

Row 4 supplemental read, no acceptance credit: a labelled re-run of the exact row 4 command at the same source and home, after the bracket closed, exited 0 and printed "Harness healthy: baseline GREEN, positive control CAUGHT." and last line "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.". The spec lists 10 mutations. The harness restored every mutated file; afterwards the home showed only this brief modified.

Row 9 disclosure: the witnessed command exits 0 and disproves nothing, but on merged main its default base is HEAD, so it judges no delivery range. A labelled re-run of the exact command printed "summary: 0 corroborated, 0 disproved, 8 unchecked, 0 brief(s) claiming nothing", each entry marked unchanged since the merge-base, and the tool states that unchecked entries are not passes. A supplemental read with no acceptance credit pinned the delivering change, medici-finance/assay PR 3302 (merge 66dbced7188e, first parent 418592cb5b78): `statusgen --root . --consumers --brief desktools-v2/21 --delivery 66dbced7188e740cf469e893ee023824397c230b --base 418592cb5b784056ccba5b9979a98920cf7f6f02` exited 0 with "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". A supplemental probe discharges no original row. The home lint at this source still reports "Verify row 9 [consumer-delivery-range] consumer command has no explicit --base (the default becomes HEAD on merged main)" as a NOTICE, and that rule is promoted to a PROBLEM on a change that closes the brief; this is the finding the 2026-11-01 desk note records. Row 9's definition is unchanged since then. Row 9's obligation is therefore not established by its admitted command. This is a check-definition blocker, medici-finance/assay#1927; the next actor is the brief owner, who re-authors row 9 in the pinned form through a reviewed check-definition change, followed by a fresh verify dispatch. The forge state of that issue was not re-read in this pass.

Risk-bearing values were enumerated over the change since the last pass (40a7aace82c8..bc078e9179c8). New: tools/desk/cmd/deskdispatch/dispatchinstructions.go quotes the role's dispatch-instruction file after the class kit; it hashes the file and refuses on a mismatch. There are small additions to dispatch.go, dispatchrecord.go (DispatchInstructionsSHA256) and prompt.go, an Attempts section in verifier-prompt.md, changed cli-migration.json rows owned by desktools-v2/35, desktools-v2/57 and performance-collection/01 (this brief's rows unchanged), and a changed desktools-v2-46 mutation spec. This brief's mutation spec, the shared CLI package, cli.go and brief.go are unchanged. Ranked literals:
- dispatch-instruction digest = SHA-256, exact hex equality with the launch record @ tools/desk/cmd/deskdispatch/dispatchinstructions.go:99 (compare at :101). New.
- ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51. Newly reached by the dispatch-instruction refusals; the value is unchanged.
- ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20. Unchanged.
- unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62. Unchanged.
- tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73. Unchanged.
- body digest form = SHA-256 / 64-hex @ tools/desk/cmd/deskdispatch/references/review-prompt.md:322 and :325. Unchanged; prompt text only.
Each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — dispatch-instruction digest = SHA-256 exact match @ tools/desk/cmd/deskdispatch/dispatchinstructions.go:99 — the file is re-hashed at dispatch and compared with the digest recorded at launch (DispatchInstructionsSHA256), so any edit after launch refuses before anything is claimed. A mismatch costs a relaunch, which is visible and reversible; a weaker or truncated comparison would let a session-edited instruction file reach a dispatched prompt.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 — the new refusals reuse the shared desk refusal code through deskkit.Refused instead of minting one, so callers keyed on a custody refusal see the existing code, and it stays distinct from ExitUsage = 2 as the CLI contract requires.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration default, which the brief requires to stay the same per invocation.

**VERIFY: BLOCKED** — rows 1-8 and 10 pass at merged main bc078e9179c8 (host witness rows 1-8; bracketed network-off container witness row 10, bracket verdict match); row 9 exits 0 as written but its admitted command judges no delivery range on merged main (0 corroborated, 8 unchecked), check-definition blocker medici-finance/assay#1927. 0 fail.
### Re-verification — 2026-11-10 (merged main 612750a12406)

Re-verification on 2026-11-10 at merged main 612750a12406f19a96a83e6ea7687d794f772f9f, by a non-implementer verifier: model claude-opus-5-5, tier strong, statusgen and deskdispatch v1.0.81, go1.27.3 darwin-arm64, git 2.56.0. The packet named the same commit. `deskdispatch --check-verifier` returned 0 before any row ran (attestation medici-finance/assay#6003, run 6059c9babcc5), again after the host run's in-home write was restored, and again after this section was drafted. `deskpathguard rederive` reported 10 rows, 0 author-added, all from-base, so the row 4 command amended on 2026-11-08 is the acceptance command run here. Rows 1-9 (class check) ran on the host through `statusgen verifyrun --rows 1,2,3,4,5,6,7,8,9 --timeout 20m` from inside the home, under the normal HOME, with no forge credential and no Kubernetes config in the environment; the 9-row table below is the tool's own write into the brief, copied here byte for byte, after which that one path was restored and the home (ignored files included) read clean. Row 10 (class check:ci) cannot use the unshare sandbox on darwin, so it ran inside a closed admission bracket; its witness, with its Admission-Bracket line, is the 1-row block after the host table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-21.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:615fdff8b8ed | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 | sha256:1d6920a87e57 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 | sha256:e3b0c44298fc | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 | sha256:1a6f00614577 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:bd9771a3f8a4 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 | sha256:938a538575ae | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6003 run=6059c9babcc52e24d0fd0b56b41651a551de41875a52ef2e0d0ff64db8859595 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:2a7fd9638e20 | 2026-11-10 | assay-verifier-app[bot] @ 612750a12406 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6
Container-Image: sha256:41ac76c853fbb5b5feebd9b1f302de531d879080e9dafe10b086691751de6f74
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6003 run=6059c9babcc52e24d0fd0b56b41651a551de41875a52ef2e0d0ff64db8859595 source=612750a12406f19a96a83e6ea7687d794f772f9f brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=6059c9babcc52e24d0fd0b56b41651a551de41875a52ef2e0d0ff64db8859595 source=612750a12406f19a96a83e6ea7687d794f772f9f tree=sha256:bbf58e1999a2879bff899fd22c5d41834fbc884451654c5085000f4eedc075b6 witness=sha256:f9366dff148b1dc86733616baa98fbe71b9a6d7a845e1de9adc71a799a539385 opened=2026-11-10T08:47:54Z closed=2026-11-10T08:48:15Z before=receipt after=check-verifier

Row 10 ran in verify-runner v1.0.81, the index digest the v1.0.81 release asset names (checksum-verified against the release checksums), resolved to the arm64 image sha256:41ac76c853fb: go1.27.3, git 2.56.0 and statusgen v1.0.81, the same version as the host install. The container ran with `--network none`, a read-only root, tmpfs home and temp directories, GOPROXY=off, GOTOOLCHAIN=local, a read-only module cache digested at bracket open (sha256:7f8fa9800956), and a GOCACHE inside the tmpfs. The home and its git common directory were bind-mounted read-only at their host paths, the receipt was mounted read-only, and the only writable mount was the witness directory outside the home. No App keys, credentials or roster were mounted, and the row needed none. No Kubernetes context or cluster was used. The bracket opened at 08:47:54Z and closed at 08:48:15Z, the close issued from inside the home, and `--bracket-digest` reported verdict=match with one each of tree-digest, container-image and attestation in the span.

Key output per row (the witness keeps only an output digest; sha256:e3b0c44298fc is the empty output of rows 1, 2, 3 and 6, whose commands send the test log to a file):
- Rows 1, 2 and 3: exit 0, so each log carried exactly one top-level PASS line for its test. The home lint's `[gotest-run-vacuous]` NOTICE on these three rows does not apply: each command counts the `--- PASS:` line itself and requires exactly one.
- Row 4: exit 0. The amended command reads the spec's mutation count (10) and requires muhar to exit 0 and its last line to equal "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.", so exit 0 is that Totals line.
- Rows 5 to 8: exit 0 as witnessed.
- Row 9: exit 0, but vacuous at merged main (see the row 9 disclosure below).

Attempts: three, in order, none overlapping.
1. 08:31:13Z to 08:32:36Z, a bracketed container run of row 10 alone (pass, verdict match). Discarded, superseded: it preceded attempt 2, and the credited attempt must be the last one.
2. 08:33:02Z to 08:39:37Z, host rows 1-9 run with `--dry-run` (all pass). Discarded, superseded: a dry-run's printed table is never a witness, so the rows were re-run with the tool's in-home write.
3. 08:39:52Z to 08:48:15Z, credited: host rows 1-9 (the 9-row table) and then a fresh bracket for row 10 (the 1-row block). It covers all 10 rows.
Before attempt 2, one host invocation that also passed `--witness` was refused as a usage error (exit 2) before any row ran; it ran nothing and is not an attempt. No row failed in any attempt, so no failure classification applies.

Fail-first: no row owes a fail-first witness. All four risk fields are "no", and the rows carry only the +dereference, +flow and +mutation tags; none is tagged risk-bearing, live, mutating or end-to-end.

Row 9 disclosure: the witnessed command exits 0 and disproves nothing, but on merged main it judges no delivery range. A labelled re-run of the exact command, no acceptance credit, reproduced the witnessed output digest (sha256:938a538575ae) and printed "consumers: no brief files in the diff against 612750a12406f19a96a83e6ea7687d794f772f9f — nothing to corroborate". A supplemental read, also with no acceptance credit, pinned the delivering change, medici-finance/assay PR 3302 (merge 66dbced7188e, first parent 418592cb5b78): `statusgen --root . --consumers --brief desktools-v2/21 --delivery 66dbced7188e740cf469e893ee023824397c230b --base 418592cb5b784056ccba5b9979a98920cf7f6f02` exited 0 with "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". A supplemental probe discharges no original row. The home lint at this source still reports "Verify row 9 [consumer-delivery-range] consumer command has no explicit --base (the default becomes HEAD on merged main)" as a NOTICE, the rule the 2026-11-01 desk note found promoted to a PROBLEM on a change that closes the brief. Row 9's definition is unchanged. Its obligation is therefore not established by its admitted command. This is the check-definition blocker medici-finance/assay#1927, re-read on the forge on 2026-11-10: OPEN, last updated 2026-11-08, the check-definition re-authoring tracker that names dt/21. The next actor is the brief owner, who re-authors row 9 in the pinned form through a reviewed check-definition change, followed by a fresh verify dispatch.

Risk-bearing values were enumerated over the change since the last pass (bc078e9179c8..612750a12406). In this brief's deliverable paths only text changed: references/verifier-prompt.md gained a module-cache clause; deskkit/exitcodes.go gained a boolean field with no literal; cli-migration.json changed rows owned by other briefs, with this brief's row unchanged; muhar gained a budget feature, and the row 4 run still reached its Totals line. This brief's mutation spec, the shared CLI package, cli.go, brief.go and dispatchinstructions.go are unchanged. Every literal below was re-read at 612750a12406, and each can be reversed with an edit and a redeploy.
RISK-VALUE: DERIVED — dispatch-instruction digest = SHA-256 exact match @ tools/desk/cmd/deskdispatch/dispatchinstructions.go:99 — the file is re-hashed at dispatch and compared at :101 with the digest recorded at launch, so any edit after launch refuses before anything is claimed. A mismatch costs a relaunch, which is visible and reversible; a weaker or truncated comparison would let a session-edited instruction file reach a dispatched prompt.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 — the dispatch refusals reuse the shared desk refusal code, so callers keyed on a custody refusal see the existing code, and it stays distinct from ExitUsage = 2 as the CLI contract requires.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration defaults, which the brief requires to stay the same per invocation.

**VERIFY: BLOCKED** — rows 1-8 and 10 pass at merged main 612750a12406 (host witness rows 1-8; bracketed network-off container witness row 10, bracket verdict match); row 9 exits 0 as written but its admitted command judges no delivery range on merged main (nothing to corroborate), check-definition blocker medici-finance/assay#1927. 0 fail.
### Re-verification — 2026-11-13 (merged main c2c5d43ca3d8, statusgen v1.0.83)

Re-verification on 2026-11-13 at merged main c2c5d43ca3d8a51cdb2ac7e3f9c974eb63bb7bda by a non-implementer verifier: model claude-opus-5-5, tier strong, statusgen and deskdispatch v1.0.83, go1.27.4 darwin-arm64 and git 2.56.1 on the host. The packet named the same commit. `deskdispatch --check-verifier` returned 0 before any row ran (attestation medici-finance/assay#6346, run ec04cf8de3ae) and again after this section was drafted. `deskpathguard rederive` reported 10 rows, 0 author-added, all from-base, so every command run here is the acceptance command. The pass was woken by input changes since the 2026-11-10 pass (612750a12406): 22 files under tools/desk/cmd/deskdispatch changed (among them cli.go, dispatch.go, exec.go, main.go, pickup.go, prompt.go and three prompt references), and cli-migration.json gained a row owned by another brief; this brief's row, its mutation spec and the shared CLI package are unchanged.

The credited witness has two parts. Row 4 ran on the host through `statusgen verifyrun --rows 4` from inside the home, with no forge credential and no Kubernetes config in the environment; the 1-row table directly below is the tool's own in-home write, copied byte for byte, after which the brief was restored and the home left clean. Rows 1-3 and 5-10 then ran through `statusgen verifyrun --rows 1,2,3,5,6,7,8,9,10 --sandbox=container-netns` inside one closed admission bracket; that 9-row block follows the host table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-21.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:d4ac276f19eb | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6346 run=ec04cf8de3ae80b75c825528537c64da71a38dc53ac709b53dbc4fc9027924bf source=c2c5d43ca3d8a51cdb2ac7e3f9c974eb63bb7bda brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 sandbox=container-netns | sha256:2e7d3cae6e07 | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 sandbox=container-netns | sha256:5dfb809e85ac | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 sandbox=container-netns | sha256:3d2618bb6040 | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 sandbox=container-netns | sha256:d3b4198a933a | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:1a7db22cf124 | 2026-11-13 | assay-verifier-app[bot] @ c2c5d43ca3d8 (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:b711b8bdbb01c913c1eba3f688aa4f673b8d1f50056f8d876faf3437e47d4cb0
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6346 run=ec04cf8de3ae80b75c825528537c64da71a38dc53ac709b53dbc4fc9027924bf source=c2c5d43ca3d8a51cdb2ac7e3f9c974eb63bb7bda brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=ec04cf8de3ae80b75c825528537c64da71a38dc53ac709b53dbc4fc9027924bf source=c2c5d43ca3d8a51cdb2ac7e3f9c974eb63bb7bda tree=sha256:b711b8bdbb01c913c1eba3f688aa4f673b8d1f50056f8d876faf3437e47d4cb0 witness=sha256:4f32d07e47bccda4ec984f7b5be9026c84b79c0efb7646a406686c57b2392569 opened=2026-11-13T02:06:41Z closed=2026-11-13T02:08:54Z before=receipt after=check-verifier

The container was the release-pinned verify-runner v1.0.83, index digest sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4, the value the v1.0.83 release notes name, run natively as linux/arm64 with statusgen v1.0.83, go1.27.3 and git 2.56.0 inside. It ran with `--network none`, a read-only root, tmpfs home and temp directories, GOPROXY=off, GOTOOLCHAIN=local, a fresh module cache downloaded and digested at bracket open and mounted read-only (sha256:7f8fa9800956), and a GOCACHE inside the tmpfs. The home and its git common directory were bind-mounted read-only at their host paths; the receipt and a copy of the verifier's roster configuration (row 9 reads it) were mounted read-only; the only writable mount was the witness directory outside the home. No App keys or forge credentials were mounted, and no Kubernetes context or cluster was used. The bracket opened at 02:06:41Z and closed at 02:08:54Z; `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match`.

Each Runner cell carries the tool's own read-set annotation (`recorder is Linux-only (darwin)` on the host, `no read-set recorder in this build` in the container), left unchanged. It does not affect any row's pass result.

Key output per row (the witness keeps only an output digest; sha256:e3b0c44298fc is the empty output of rows 1, 2, 3 and 6, whose commands send the test log to a file):
- Rows 1, 2 and 3: exit 0, so each log carried exactly one top-level PASS line for its test. Each command counts the `--- PASS:` line itself and requires exactly one.
- Row 4: exit 0. The command reads the spec's mutation count (10) and requires muhar to exit 0 and its last line to equal "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.", so exit 0 is that Totals line.
- Rows 5 to 8 and 10: exit 0 as witnessed.
- Row 9: exit 0, but vacuous at merged main (see the row 9 disclosure below).

Attempts: three, in order, none overlapping.
1. 01:49:19Z to 01:52:34Z, a bracketed container run of all 10 rows. Rows 1-3 and 5-10 passed; row 4 exited 1. A labelled diagnostic re-run of row 4 in the same read-only envelope showed why: muhar's positive control could not restore the file it mutates ("restore cmd/deskdispatch/cli.go: open cmd/deskdispatch/cli.go: read-only file system") and muhar reported "HARNESS BROKEN — run discarded", so the row judged no mutation. That is an executor-envelope limit (muhar mutates the tree in place; the bracket mounts the home read-only), not an observed product failure. Discarded, superseded by attempts 2 and 3, which run row 4 where the tree is writable.
2. 01:53:33Z to 01:59:13Z, host rows 1-9. Rows 1-4, 6, 7 and 9 passed; rows 5 and 8 exited 1. Labelled diagnostic re-runs isolated both to the decision-gate tests in tools/desk/cmd/deskdispatch (TestDecisionChild*, reached by row 5 directly and by the row 8 floor seed): on this verifier host a `deskdecision` wrapper on PATH changes the child those tests launch. That is the known host fault medici-finance/assay#6279; the same tests pass in the network-off container (attempt 3, rows 5 and 8). Discarded, env-fault, medici-finance/assay#6279.
3. 02:02:02Z to 02:08:54Z, credited: host row 4 (the 1-row table) and then a fresh bracket for rows 1-3 and 5-10 (the 9-row block). It covers all 10 rows, and every row passes.

Fail-first: no row owes a fail-first witness. All four risk fields are "no", and the rows carry only the +dereference, +flow and +mutation tags; none is tagged risk-bearing, live, mutating or end-to-end.

Row 9 disclosure: the witnessed command exits 0 and disproves nothing, but on merged main it judges no delivery range. A labelled re-run of the exact command, no acceptance credit, printed "consumers: no brief files in the diff against c2c5d43ca3d8a51cdb2ac7e3f9c974eb63bb7bda — nothing to corroborate". A supplemental read, also with no acceptance credit, pinned the delivering change, medici-finance/assay PR 3302 (merge 66dbced7188e, first parent 418592cb5b78): `statusgen --root . --consumers --brief desktools-v2/21 --delivery 66dbced7188e740cf469e893ee023824397c230b --base 418592cb5b784056ccba5b9979a98920cf7f6f02` exited 0 with "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". A supplemental probe discharges no original row. The check-definition tracker medici-finance/assay#1927 is still open and row 9's Command is unchanged at this source.

Risk-bearing values were enumerated over the change since the last pass (612750a12406..c2c5d43ca3d8). The new literals in this brief's deliverable paths are two string flags, `context-snapshot` and `context-receipt`, both defaulting to "" @ tools/desk/cmd/deskdispatch/cli.go:84-85: an empty default leaves the context recheck off unless the caller names a snapshot, which is the pre-change behaviour, so neither is a numeric bound or a security threshold. Every literal below was re-read at c2c5d43ca3d8 at the same line with the same value, and each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — dispatch-instruction digest = SHA-256 exact match @ tools/desk/cmd/deskdispatch/dispatchinstructions.go:99 — the file is re-hashed at dispatch and compared at :101 with the digest recorded at launch, so any edit after launch refuses before anything is claimed. A mismatch costs a relaunch, which is visible and reversible; a weaker or truncated comparison would let a session-edited instruction file reach a dispatched prompt.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 — the dispatch refusals reuse the shared desk refusal code, so callers keyed on a custody refusal see the existing code, and it stays distinct from ExitUsage = 2 as the CLI contract requires.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration defaults, which the brief requires to stay the same per invocation.

**VERIFY: BLOCKED** — rows 1-8 and 10 pass at merged main c2c5d43ca3d8 (host witness row 4; bracketed network-off container witness rows 1-3 and 5-10, bracket verdict match); row 9 exits 0 as written but its admitted command judges no delivery range on merged main (nothing to corroborate), check-definition blocker medici-finance/assay#1927. 0 fail.

### Re-verification — 2026-11-13 (merged main a49aa8cb6a2d, statusgen v1.0.83)

Re-verification on 2026-11-13 at merged main a49aa8cb6a2da82207cde13b8c935e652ce29662 by a non-implementer verifier: model claude-opus-5-5, tier strong, statusgen and deskdispatch v1.0.83, go1.27.4 darwin-arm64 and git 2.56.1 on the host. The packet named the same commit. `deskdispatch --check-verifier` returned 0 before any row ran (attestation medici-finance/assay#6425, run 91ff894e37fd), and again before the bracket opened; after this section was drafted it was refused with "refused: heartbeat stale (result=disabled) — HEARTBEAT stale (age 24h0m6s > 24h0m0s)". After the desk heartbeat was refreshed at 15:09:20Z it returned 0 again at 15:23:43Z, before this landed. `deskpathguard rederive` reported 10 rows, 0 author-added, all from-base, so every command run here is the acceptance command. The pass was woken by input changes since the c2c5d43ca3d8 pass: five changelog entries were added and tools/desk/cmd/deskdispatch/worktree.go now calls the shared deskkit sanctioned-worktree-prefix rule instead of its own copy of it (medici-finance/assay PR 6317). This brief's mutation spec, its cli-migration.json row and the shared CLI package are unchanged. All rows were run fresh; no earlier result is carried forward.

The credited witness has two parts. Row 4 ran on the host through `statusgen verifyrun --rows 4` from inside the home, with no forge credential and no Kubernetes config in the environment; the 1-row table directly below is the tool's own in-home write, copied byte for byte, after which the brief was restored and the home left clean. Rows 1-3 and 5-10 then ran through `statusgen verifyrun --rows 1,2,3,5,6,7,8,9,10 --sandbox=container-netns` inside one closed admission bracket; that 9-row block follows the host table.

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 4 | `(cd tools/desk && s=internal/clicontract/testdata/mutations/desktools-v2-21.json && n=$(jq -e '.mutations \| length' "$s") && test "$n" -gt 0 && o=$(mktemp) && trap 'rm -f "$o"' 0 && { go run ./cmd/muhar -j 1 -spec "$s" > "$o"; r=$?; cat "$o"; test "$r" -eq 0; } && test "$(tail -n 1 "$o")" = "Totals: $n caught, 0 NOT CAUGHT, 0 could-not-mutate.")` | pass exit=0 | sha256:3a7c70044589 | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#6425 run=91ff894e37fd3ab11a97aa0028b6ac243c9bc86034d38f428ded88ca811295e7 source=a49aa8cb6a2da82207cde13b8c935e652ce29662 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskdispatch -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 5 | `cd tools/desk && go test -count=1 ./cmd/deskdispatch/...` | pass exit=0 sandbox=container-netns | sha256:a2a8b0072e02 | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 6 | `cd tools/desk && GOOS=windows go vet ./cmd/deskdispatch` | pass exit=0 sandbox=container-netns | sha256:e3b0c44298fc | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 7 | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/21 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | pass exit=0 sandbox=container-netns | sha256:058652519b74 | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 8 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 sandbox=container-netns | sha256:c6654d8c7742 | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers` | pass exit=0 sandbox=container-netns | sha256:dd7a0a1e499b | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
| 10 | `(cd tools/desk && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestCLIHelpDoesNotDiscoverManifests$' ./cmd/deskdispatch/ > "$out" && grep -F -- '--- PASS: TestCLIHelpDoesNotDiscoverManifests ' "$out")` | pass exit=0 sandbox=container-netns | sha256:efac7049c4a5 | 2026-11-13 | assay-verifier-app[bot] @ a49aa8cb6a2d (reads could-not-check: no read-set recorder in this build) (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:1d0ccbb1d9817abf4e359c1968c41661ab5805a0a80b15fd2950aa3a2fcc7ebe
Container-Image: sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#6425 run=91ff894e37fd3ab11a97aa0028b6ac243c9bc86034d38f428ded88ca811295e7 source=a49aa8cb6a2da82207cde13b8c935e652ce29662 brief=docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=91ff894e37fd3ab11a97aa0028b6ac243c9bc86034d38f428ded88ca811295e7 source=a49aa8cb6a2da82207cde13b8c935e652ce29662 tree=sha256:1d0ccbb1d9817abf4e359c1968c41661ab5805a0a80b15fd2950aa3a2fcc7ebe witness=sha256:c9440043c99122b55a9bf234eaf8929aceec6d9ebcef53ceca1dce4d2616757b opened=2026-11-13T15:03:04Z closed=2026-11-13T15:05:06Z before=receipt after=check-verifier

The container was the release-pinned verify-runner v1.0.83, index digest sha256:472a3fce50e18c742c36ac54d6898331ea7edad409c2499dc2d8855e8dac71b4, run natively as linux/arm64 with statusgen v1.0.83, go1.27.3 and git 2.56.0 inside. It ran with `--network none`, a read-only root, all capabilities dropped, no-new-privileges, a tmpfs home and temp directory, GOPROXY=off, GOTOOLCHAIN=local, a fresh module cache downloaded and digested at bracket open and mounted read-only (sha256:7f8fa9800956), and a GOCACHE inside the tmpfs. The home and its git common directory were bind-mounted read-only at their host paths; the receipt and a copy of the verifier's roster configuration (row 9 reads it) were mounted read-only; the only writable mount was the witness directory outside the home. No App keys or forge credentials were mounted, and no Kubernetes context or cluster was used. The bracket opened at 15:03:04Z and closed at 15:05:06Z; `statusgen verifyrun --bracket-digest` on the closed witness reports `verdict=match`.

Each Runner cell carries the tool's own read-set annotation (`recorder is Linux-only (darwin)` on the host, `no read-set recorder in this build` in the container), left unchanged. It does not affect any row's pass result.

Key output per row (the witness keeps only an output digest; sha256:e3b0c44298fc is the empty output of rows 1, 2, 3 and 6, whose commands send the test log to a file):
- Rows 1, 2 and 3: exit 0, so each log carried exactly one top-level PASS line for its test. Each command counts the `--- PASS:` line itself and requires exactly one.
- Row 4: exit 0. The command reads the spec's mutation count (10) and requires muhar to exit 0 and its last line to equal "Totals: 10 caught, 0 NOT CAUGHT, 0 could-not-mutate.", so exit 0 is that Totals line. muhar's test selector names only the owner CLI tests, so the host decision-gate fault (medici-finance/assay#6279) cannot reach it.
- Rows 5 to 8 and 10: exit 0 as witnessed, in the container, where the host decision-gate fault does not apply.
- Row 9: exit 0, but vacuous at merged main (see the row 9 disclosure below).

Attempts: one. 14:56:38Z to 15:05:06Z, uncredited (credit-unchecked: `statusgen verifyrun --bracket-digest --credit` on this brief printed credit=could-not-check, "cannot place docs/streams/desktools-v2/brief-21-migrate-deskdispatch-to-cobra-and-viper.md inside its repository", while the third `--check-verifier` was refused on a stale desk heartbeat): host row 4 (the 1-row table, 14:56:38Z to 15:02:14Z) and then one bracket for rows 1-3 and 5-10 (the 9-row block, 15:03:04Z to 15:05:06Z). It covers all 10 rows, and every row exits 0. No run was discarded. Row 4 was planned for the host from the start because muhar mutates the tree in place and the bracket mounts the home read-only. GIT_CONFIG_GLOBAL was unset on the host, so medici-finance/assay#6389 did not apply.

Fail-first: no row owes a fail-first witness. All four risk fields are "no", and the rows carry only the +dereference, +flow and +mutation tags; none is tagged risk-bearing, live, mutating or end-to-end.

Row 9 disclosure: the witnessed command exits 0 and disproves nothing, but on merged main it judges no delivery range. A labelled re-run of the exact command on the host, no acceptance credit, printed "consumers: no brief files in the diff against a49aa8cb6a2da82207cde13b8c935e652ce29662 — nothing to corroborate". A supplemental read, also with no acceptance credit, pinned the delivering change, medici-finance/assay PR 3302 (merge 66dbced7188e, first parent 418592cb5b78): `statusgen --root . --consumers --brief desktools-v2/21 --delivery 66dbced7188e740cf469e893ee023824397c230b --base 418592cb5b784056ccba5b9979a98920cf7f6f02` exited 0 with "summary: 8 corroborated, 0 disproved, 0 unchecked, 0 brief(s) claiming nothing". A supplemental probe discharges no original row. The check-definition tracker medici-finance/assay#1927 is still open (read 2026-11-13) and row 9's Command is unchanged at this source.

Risk-bearing values were enumerated over the change since the last pass (c2c5d43ca3d8..a49aa8cb6a2d) in this brief's deliverable paths. The only code change is worktree.go: the operator-worktree prefix check now calls deskkit.InSanctionedWorktreePrefix, which deskwt also calls (tools/desk/cmd/deskwt/deskwt.go:167), and the tmp base comes from a shared constant. The changelog entries carry no literal. Every literal below was re-read at a49aa8cb6a2d at the line given, and each can be reversed with an edit and a redeploy.

RISK-VALUE: DERIVED — SanctionedWorktreeTmpBase = "/private/tmp" @ tools/desk/internal/deskkit/worktreeprefix.go:23 (read by worktreeTmpBase @ tools/desk/cmd/deskdispatch/worktree.go:73) — the same literal worktree.go:73 held at c2c5d43ca3d8, and the parent deskwt's removal allowlist checks, so a dispatched home can be placed only where the sanctioned removal path can reach it; it is a compiled constant, never env- or flag-relocatable, and the comparison is lexical against the resolved base, so an unresolved /tmp spelling is refused, which is the safe direction.
RISK-VALUE: DERIVED — SanctionedWorktreeLeafPrefix = "tracker-" @ tools/desk/internal/deskkit/worktreeprefix.go:27 — identical to the "tracker-" literal the removed copy held at c2c5d43ca3d8 worktree.go:143. The shared function keeps the same two clauses (a direct child of the tmp base whose leaf starts with the prefix, or strictly under the resolved .claude/worktrees directory), so the allowlist is neither widened nor narrowed, and the two tools can no longer drift apart.
RISK-VALUE: DERIVED — dispatch-instruction digest = SHA-256 exact match @ tools/desk/cmd/deskdispatch/dispatchinstructions.go:99 — the file is re-hashed at dispatch and compared at :101 with the digest recorded at launch, so any edit after launch refuses before anything is claimed. A mismatch costs a relaunch, which is visible and reversible; a weaker or truncated comparison would let a session-edited instruction file reach a dispatched prompt.
RISK-VALUE: DERIVED — ExitRefused = 5 @ tools/desk/internal/deskkit/exitcodes.go:51 — the dispatch refusals reuse the shared desk refusal code, so callers keyed on a custody refusal see the existing code, and it stays distinct from ExitUsage = 2 as the CLI contract requires.
RISK-VALUE: DERIVED — ExitUsage = 2 @ tools/desk/internal/cli/cli.go:20 — the shared CLI contract fixes exit 2 for unknown input before any handler runs (docs/streams/desktools-v2/cli-contract.md, "Unknown input" row); domain refusals keep their own codes.
RISK-VALUE: DERIVED — unreadable-frontmatter gate = "human" @ tools/desk/cmd/deskdispatch/brief.go:58 and :62 — a brief whose metadata cannot be parsed cannot show it is model-gated, so reading it as human-gated fails closed; "" would let a malformed human-gated brief skip its gate.
RISK-VALUE: DERIVED — tier default = "any" and kit default = "worker" @ tools/desk/cmd/deskdispatch/cli.go:73 — identical to the pre-migration defaults, which the brief requires to stay the same per invocation. The context-snapshot and context-receipt string flags @ cli.go:84-85 still default to "", which leaves the context recheck off unless the caller names a snapshot.

**VERIFY: BLOCKED** — rows 1-8 and 10 pass at merged main a49aa8cb6a2d (host witness row 4; bracketed network-off container witness rows 1-3 and 5-10, bracket verdict match); row 9 exits 0 as written but its admitted command judges no delivery range on merged main (nothing to corroborate), check-definition blocker medici-finance/assay#1927, next actor the brief owner (re-author row 9 with a pinned delivery range, then a fresh dispatch). 0 fail.

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
