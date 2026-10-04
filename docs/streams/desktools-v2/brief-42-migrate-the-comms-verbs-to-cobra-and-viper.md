---
brief: "assay:assay:desktools-v2:42"
title: "Migrate the comms verbs to Cobra and Viper"
why: >-
  The comms commands (commsgw, deskcomms, commsloop) carry signed cross-desk messages. commsgw
  takes no argv today. The move gives them standard help while signing keys and the gateway
  address stay sourced exactly as now.
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
id: "6e384661-b193-4ab4-804e-a9cde6dace44"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/42)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/commsgw: follow-up desktools-v2/42 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskcomms: follow-up desktools-v2/42 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/commsloop: follow-up desktools-v2/42 (command tree, declared configuration, contract tests)"
  - "topology.yaml: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - ".claude/guardrails/GUARDRAILS.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/intake-desk/SKILL.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/verify-desk/SKILL.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/cmd/commsgw/README.md: follow-up desktools-v2/42 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/42 (owned rows flip to migrated)"
gate-why: >-
  The comms lane signs messages with a key read from the environment. Human sign-off confirms
  the key source, gateway address and signature checks are unchanged and no key appears in
  argv, help or errors.
decision-trigger: "spec"
---

# Brief 42 — Migrate the comms verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/commsgw` — `commsgw`; module tools/desk; parser: no argv; environment only; 1835 LOC, ~3 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskcomms` — `deskcomms`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 919 LOC, ~3 verbs, ~5 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/commsloop` — `commsloop`; module tools/desk; parser: Go flag: FlagSet; 2763 LOC, ~5 verbs, ~3 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/commsgw/cli_test.go` (planned), `tools/desk/cmd/commsloop/cli_test.go` (planned), `tools/desk/cmd/deskcomms/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms, TestCLIAdmissionBoundary.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-42.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-comms.md` (planned).

facts:
- commsgw is configured by environment only; it binds those keys through declared Viper bindings and gains no new flags.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
single-point-of-failure: signature verification on receipt — it stays in the handler; TestCLIAdmissionBoundary sends an unsigned message through Cobra and directly.
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

1. Build a Cobra command tree for `commsgw`, `deskcomms`, `commsloop`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: TestCLIAdmissionBoundary drives refused inputs through the Cobra tree AND straight into the domain check with the adapter bypassed; both refuse, and no effect occurs on parse failure.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/commsgw ./cmd/deskcomms ./cmd/commsloop -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 3` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/commsgw ./cmd/deskcomms ./cmd/commsloop -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 3` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/commsgw ./cmd/deskcomms ./cmd/commsloop -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 3` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/commsgw ./cmd/deskcomms ./cmd/commsloop -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIAdmissionBoundary " "$f")" -eq 3` | exit 0; TestCLIAdmissionBoundary PASS in every package; refused inputs stay refused through Cobra and with the adapter bypassed |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-42.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, and a refusal moved out of the domain check into the adapter; each reddens the owner tests |
| 6 | check | `cd tools/desk && go test -count=1 ./cmd/commsgw/... ./cmd/deskcomms/... ./cmd/commsloop/...` | exit 0; every pre-existing test in the owned packages still passes |
| 7 | check | `cd tools/desk && GOOS=windows go vet ./cmd/commsgw ./cmd/deskcomms ./cmd/commsloop` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/42 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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
