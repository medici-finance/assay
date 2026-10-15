---
brief: "assay:assay:desktools-v2:22"
title: "Migrate deskwt to Cobra and Viper"
why: >-
  deskwt is the worktree supervisor (add, prune, interval runs) with one FlagSet per verb and
  manual dispatch. Cobra gives each verb real help; its prune and lock decisions must not
  change.
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
id: "db9e3c41-3e2f-4a8a-8cd5-0ac34f329393"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/22)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
  - "docs/proposals/non-ci-architecture/14-storage-forge-graph.md (merged #2601, 40b45182b3c011cef823309c4e1c8d1aecbf6967)"
  - "docs/proposals/non-ci-architecture/15-regression-from-day-one.md"
  - "https://github.com/medici-finance/assay/pull/2525 (open at 0e0d6de20bbfed443e62fd5cc5a090d311d71df9 on 2026-10-11; refresh before pickup)"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskwt: follow-up desktools-v2/22 (command tree, declared configuration, contract tests)"
  - "docs/deskwt-cli-compat.md: follow-up desktools-v2/22 (baseline and compatibility evidence)"
  - "tools/desk/README.md: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/cellctl/tests/house-cell.test.sh: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/desk-tools/hooks.md: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/internal/deskkit/pushtransport-mutations.json: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/cellctl.md: follow-up desktools-v2/22 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/22 (owned rows flip to migrated)"
---

# Brief 22 — Migrate deskwt to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskwt` — `deskwt`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 4684 LOC, ~7 verbs, ~18 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskwt/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms and TestCLILifecycleParity.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-22.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-worktree.md` (planned).
- `docs/deskwt-cli-compat.md` (planned) — source-pinned command/configuration inventory,
  approved observable changes, and lifecycle follow-up boundaries.

facts:
- deskwt verbs mutate local worktrees; help and parse failures must reach zero worktree operations.
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

## Lifecycle reconciliation — 2026-10-11

At source `40b45182b3c011cef823309c4e1c8d1aecbf6967`, `main.go` dispatches
`add`, `remove`, `prune`, `maintain`, `role-init` and `role-clean`. The older LOC/flag
counts above are historical, not the migration inventory. `maintenance.go` already
owns bounded manual ref/commit-graph work, `prunesingleton.go` owns sweep coordination,
and `lockreclaim.go` still has an opt-in age fallback. The approved architecture in
#2601 changes the target lock-retirement rule; merging the document did not change
that implementation. A parser migration must not claim architecture qualification.

Before changing parsing, record a baseline commit and enumerate its full verb/sub-verb,
flag, default, environment, exit, output and caller inventory in the compatibility
report. Re-read #2525 and its workpad: it owns the proposed object-maintenance and
temporary-ref implementation. If it remains open, record the boundary with its existing
worker and keep its changes out of this port; if it lands, incorporate its command tree,
common-store locking and receipt tests into the baseline. Refresh on each merge of main
before publication so newly landed commands cannot disappear under the old inventory.

Keep policy changes separate and name their follow-up owner in the report: allocation
records, class-aware review retirement, forge-bound branch eligibility, lock retirement,
hook policy and deployment profiles belong to the lifecycle owner. In particular, the
approved target uses age only to trigger a lock check; it does not prove the holder gone.
Do not quietly reinterpret `--lock-ttl` in Viper, or label its legacy behavior as satisfying
the approved target. Preserve the baseline during this parser port except for explicit
CLI-contract changes; a lifecycle semantic change requires its own reviewed implementation.

Reuse the shared CLI contract and `internal/cli` adapter. Keep maintenance/reclamation
opt-ins, duration bounds, dry-run behavior, branch compare-and-delete protections,
unknown-liveness/dirty/unpublished-work holds, recovery retention and borrower protections
in their existing handlers. Do not acquire a lock or run Git for nested help. Keep
sweep scheduling out of parsing/configuration; hooks cannot become a second pruner.

## Task

1. Capture the refreshed source/caller inventory and lifecycle boundary above, then build a Cobra command tree for `deskwt`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Declare TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms and TestCLILifecycleParity in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text. The lifecycle test exercises
   CLI → typed options → existing handler decisions with isolated fixtures and effect
   counters. Cover read-only maintenance versus apply, nested maintenance help, invalid
   duration/opt-in combinations, held resources and unchanged singleton release between
   sweeps. When #2525 is in the baseline, include its object and temporary-ref entrypoints.
   Plant a removed maintenance command, lost explicit opt-in and parse-time lock acquisition
   in the mutation spec; each must fail. No fixture calls the host installed tool or service.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskwt -run '^TestCLIHelpOffline$' > "$f" 2>&1 && test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 1` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskwt -run '^TestCLIConfigFlow$' > "$f" 2>&1 && test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 1` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskwt -run '^TestCLILegacyForms$' > "$f" 2>&1 && test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 1` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-22.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests |
| 5 | check | `cd tools/desk && go test -count=1 ./cmd/deskwt/...` | exit 0; every pre-existing test in the owned packages still passes |
| 6 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskwt` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 7 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/22 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 8 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 9 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |
| 10 | check +flow | `(f=$(mktemp) && trap 'rm -f "$f"' 0 && cd tools/desk && go test -count=1 -v ./cmd/deskwt -run '^TestCLILifecycleParity$' > "$f" 2>&1 && grep -F -e '--- PASS: TestCLILifecycleParity ' "$f")` | exit 0; named PASS; the refreshed command inventory reaches unchanged lifecycle handlers; maintenance stays read-only without apply, invalid input/help has zero effects, and the declared mutation cases fail in row 4. |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
A stale inventory drops newly landed maintenance commands or Viper changes lifecycle defaults: row 10 and its row-4 mutations fail; review checks the baseline against main and the #2525 disposition.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Check legacy-form coverage against the consumers and that the old parser is gone.
