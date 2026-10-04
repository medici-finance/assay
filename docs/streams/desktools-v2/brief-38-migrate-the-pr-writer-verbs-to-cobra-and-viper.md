---
brief: "assay:assay:desktools-v2:38"
title: "Migrate the PR writer verbs to Cobra and Viper"
why: >-
  deskpr, deskreply, desklabel and deskack write to pull requests as role Apps, each with its
  own FlagSet. Cobra gives them real help and one option grammar; the body scan and every exit
  code stay.
wave: 3
depends: ["desktools-v2/16"]
unblocks: ["desktools-v2/17"]
effort: "L"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "00fc5d67-3565-4dec-b882-99fb0066fc42"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/38)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/deskpr: follow-up desktools-v2/38 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskreply: follow-up desktools-v2/38 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/desklabel: follow-up desktools-v2/38 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskack: follow-up desktools-v2/38 (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt-objective.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/cmd/deskdispatch/references/worker-prompt.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/desk-tools/deskpr.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/codex-smoke-runs/2026-09-13-codex-0.154.0-step5-rerun.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desktools-v2/38 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/38 (owned rows flip to migrated)"
gate-why: >-
  deskpr and deskreply publish under App identity behind outbound content checks and
  audited scan overrides. Human sign-off confirms the migration preserves their refusal
  boundaries, override validation and exit codes.
decision-trigger: "spec"
---

# Brief 38 — Migrate the PR writer verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/deskpr` — `deskpr`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 2741 LOC, ~7 verbs, ~18 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskreply` — `deskreply`; module tools/desk; parser: Go flag: FlagSet; 1034 LOC, ~0 verbs, ~4 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/desklabel` — `desklabel`; module tools/desk; parser: Go flag: FlagSet; 745 LOC, ~3 verbs, ~2 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskack` — `deskack`; module tools/desk; parser: Go flag: FlagSet; 191 LOC, ~0 verbs, ~2 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskack/cli_test.go` (planned), `tools/desk/cmd/desklabel/cli_test.go` (planned), `tools/desk/cmd/deskpr/cli_test.go` (planned), `tools/desk/cmd/deskreply/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms, TestCLIAdmissionBoundary.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-38.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-writers.md` (planned).

facts:
- deskpr and deskreply scan outbound bodies before writing. `--force-scan-override` requires a valid reason and a successful audit write; it never overrides impersonation or withheld-identifier refusals. These checks and their exit codes stay in the domain path, not only the parser.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
single-point-of-failure: outbound content and scan-override admission before publication — retain handler checks and the checked Forge outbound scan; test the domain entry directly because parser validation is not a second authority and cannot replace either check.
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

1. Build a Cobra command tree for `deskpr`, `deskreply`, `desklabel`, `deskack`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: TestCLIAdmissionBoundary exercises each command's existing refusals through Cobra AND directly through its typed domain entry with the adapter bypassed. For deskpr and deskreply, include refused bodies, invalid scan-override reasons, attempted overrides of non-overridable refusals, and override audit-write failure. Assert the specific refusal and existing exit code, zero publication or push, and the existing audit behavior; parse failures cause no domain effect. Include valid controls so an unrelated refusal cannot satisfy a case.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskpr ./cmd/deskreply ./cmd/desklabel ./cmd/deskack -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskpr ./cmd/deskreply ./cmd/desklabel ./cmd/deskack -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskpr ./cmd/deskreply ./cmd/desklabel ./cmd/deskack -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskpr ./cmd/deskreply ./cmd/desklabel ./cmd/deskack -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1 && test "$(grep -c -F -e "--- PASS: TestCLIAdmissionBoundary " "$f")" -eq 4` | exit 0; TestCLIAdmissionBoundary PASS in all four packages; deskpr and deskreply reject refused bodies and invalid scan overrides through Cobra and direct domain entry with matching refusal codes, no publication or push, and preserved audit behavior |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-38.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped; each reddens the owner tests. Additionally, for deskpr and deskreply separately move body-scan or scan-override refusal from the domain path into the adapter, omit the scan before publication, and permit an invalid scan override; each mutation must redden the corresponding admission case |
| 6 | check | `cd tools/desk && go test -count=1 ./cmd/deskpr/... ./cmd/deskreply/... ./cmd/desklabel/... ./cmd/deskack/...` | exit 0; every pre-existing test in the owned packages still passes |
| 7 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskpr ./cmd/deskreply ./cmd/desklabel ./cmd/deskack` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/38 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 9 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 10 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

Help reads config or credentials before printing: row 1 runs with hostile config and effect counters.
Viper's generic precedence replaces a per-key rule or loses an explicit empty: row 2 checks the matrix.
A consumer's old spelling stops parsing: row 3 replays each consumer's forms.
The owner tests pass on the happy path but miss a broken guard: the mutation row plants each break and requires it to redden.
Only imports change while the old parser stays: TestCLIOwnerMigrated and /17's source checks catch it.
A refusal survives only in Cobra, or an override gains broader authority: row 4 bypasses the parser and the mutation row plants those breaks.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: human. No refusal, credential source or exit code may weaken; reject a report that lists one. Check the admission cases and override scope against the domain path, not just Cobra.
