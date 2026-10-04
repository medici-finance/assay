---
brief: "assay:assay:desktools-v2:32"
title: "Migrate the scan and cluster guards to Cobra and Viper"
why: >-
  The scan and cluster guards refuse unsafe content and cluster operations; they parse
  arguments by hand or with ad-hoc FlagSets. Standard help and option forms must arrive
  without moving any refusal.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "b8831464-61cf-44c3-a94f-89d7fb1832d2"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/32)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/clusterguard: follow-up desktools-v2/32 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/repohardenguard: follow-up desktools-v2/32 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskscanbody: follow-up desktools-v2/32 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskscanuntrusted: follow-up desktools-v2/32 (command tree, declared configuration, contract tests)"
  - "tools/claimguard: follow-up desktools-v2/32 (command tree, declared configuration, contract tests)"
  - "tools/desk/README.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/adopt/SKILL.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/install/SKILL.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/brief-rules.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/desk-tools/deskpr.md: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "fixtures/untrusted-corpus/exfil-178/corpus.yaml: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "fixtures/untrusted-corpus/benign/corpus.yaml: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "fixtures/untrusted-corpus/code-exec/corpus.yaml: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "fixtures/untrusted-corpus/injection-unicode/corpus.yaml: follow-up desktools-v2/32 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/32 (owned rows flip to migrated)"
gate-why: >-
  These are security scanners and guards. Human sign-off confirms every refusal and exit code
  survives the parser move and that no new option can disable a scan.
decision-trigger: "spec"
---

# Brief 32 — Migrate the scan and cluster guards to Cobra and Viper

## Context

files:
- `tools/desk/cmd/clusterguard` — `clusterguard`; module tools/desk; parser: manual os.Args parsing; 557 LOC, ~1 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/repohardenguard` — `repohardenguard`; module tools/desk; parser: Go flag: FlagSet; 1062 LOC, ~0 verbs, ~3 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskscanbody` — `deskscanbody`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 243 LOC, ~5 verbs, ~5 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskscanuntrusted` — `deskscanuntrusted`; module tools/desk; parser: manual os.Args parsing; 320 LOC, ~2 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/claimguard` — `claimguard`; module tools/claimguard; parser: Go flag: FlagSet; 344 LOC, ~0 verbs, ~2 flags at 3ad1ad83c871; release: module-local; no release or workflow build declaration.
- `tools/claimguard/cli_test.go` (planned), `tools/desk/cmd/clusterguard/cli_test.go` (planned), `tools/desk/cmd/deskscanbody/cli_test.go` (planned), `tools/desk/cmd/deskscanuntrusted/cli_test.go` (planned), `tools/desk/cmd/repohardenguard/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms, TestCLIAdmissionBoundary.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-32.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-guards.md` (planned).

facts:
- claimguard is its own module and uses Cobra and Viper directly.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
single-point-of-failure: each scanner's refusal — it stays in the handler; TestCLIAdmissionBoundary submits planted content through Cobra and directly.
design-fit:
  owner: each command's own package; domain owners unchanged
  contract: none — consumes cli-contract.md delivered by /15
  retires: [hand-rolled flag loops and usage strings]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help spellings; refusals +0
  why-add: upstream Cobra and Viper replace bespoke parsing; keep only the compatibility normalisation existing callers need

## Human decision

At pickup, the implementer records the measured compatibility report: every legacy flag form
kept, every deliberate help change, and any configuration key that is bound for the first time.
The decision is whether that report preserves the existing refusals, credential sources and
exit codes exactly. Approval covers only the listed changes; silence never authorises a weaker check.

Options:
1. **Approve the report** — the migration lands with exactly the listed compatibility changes.
2. **Reject or narrow** — the listed change is removed and the report is re-submitted.

Default if no answer: none — blocks until answered.

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve existing domain checks, custody decisions and exit codes; a
CLI library is not a new authority source. Independent verification owns Evidence and
lifecycle advancement. Proposed tests and files below are deliverables, not claims that
they already exist.

## Task

1. Build a Cobra command tree for `clusterguard`, `repohardenguard`, `deskscanbody`, `deskscanuntrusted`, `claimguard`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: TestCLIAdmissionBoundary drives refused inputs through the Cobra tree AND straight into the domain check with the adapter bypassed; both refuse, and no effect occurs on parse failure.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && { (cd tools/desk && go test -count=1 -v ./cmd/clusterguard ./cmd/repohardenguard ./cmd/deskscanbody ./cmd/deskscanuntrusted -run '^TestCLIHelpOffline$'); (cd tools/claimguard && go test -count=1 -v . -run '^TestCLIHelpOffline$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 5` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && { (cd tools/desk && go test -count=1 -v ./cmd/clusterguard ./cmd/repohardenguard ./cmd/deskscanbody ./cmd/deskscanuntrusted -run '^TestCLIConfigFlow$'); (cd tools/claimguard && go test -count=1 -v . -run '^TestCLIConfigFlow$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 5` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && { (cd tools/desk && go test -count=1 -v ./cmd/clusterguard ./cmd/repohardenguard ./cmd/deskscanbody ./cmd/deskscanuntrusted -run '^TestCLILegacyForms$'); (cd tools/claimguard && go test -count=1 -v . -run '^TestCLILegacyForms$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 5` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +flow | `f=$(mktemp) && { (cd tools/desk && go test -count=1 -v ./cmd/clusterguard ./cmd/repohardenguard ./cmd/deskscanbody ./cmd/deskscanuntrusted -run '^TestCLIAdmissionBoundary$'); (cd tools/claimguard && go test -count=1 -v . -run '^TestCLIAdmissionBoundary$'); } > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIAdmissionBoundary " "$f")" -eq 5` | exit 0; TestCLIAdmissionBoundary PASS in every package; refused inputs stay refused through Cobra and with the adapter bypassed |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-32.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, and a refusal moved out of the domain check into the adapter; each reddens the owner tests |
| 6 | check | `(cd tools/desk && go test -count=1 ./cmd/clusterguard/... ./cmd/repohardenguard/... ./cmd/deskscanbody/... ./cmd/deskscanuntrusted/...) && (cd tools/claimguard && go test -count=1 ./...)` | exit 0; every pre-existing test in the owned packages still passes |
| 7 | check | `(cd tools/desk && GOOS=windows go vet ./cmd/clusterguard ./cmd/repohardenguard ./cmd/deskscanbody ./cmd/deskscanuntrusted) && (cd tools/claimguard && GOOS=windows go vet .)` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/32 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 9 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 10 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.
A check moves into the adapter and disappears from the handler: the admission row calls the handler directly.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: human. No refusal, credential source or exit code may weaken; reject a report that lists one.
