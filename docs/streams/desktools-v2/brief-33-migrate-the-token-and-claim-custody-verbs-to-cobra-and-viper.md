---
brief: "assay:assay:desktools-v2:33"
title: "Migrate the token and claim custody verbs to Cobra and Viper"
why: >-
  desktoken, deskclaim-ref, deskclaim and deskboot mint tokens, hold claims and boot desks.
  Their parsers decide which identity and which claim ref an invocation touches, so the
  migration must prove identical custody while gaining real help.
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
id: "147d5b80-d8b8-459e-8698-ff5b0c7bd9eb"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/33)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/cmd/desktoken: follow-up desktools-v2/33 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskclaim-ref: follow-up desktools-v2/33 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskclaim: follow-up desktools-v2/33 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskboot: follow-up desktools-v2/33 (command tree, declared configuration, contract tests)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/README.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/references/desk-shell.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-review-desk/SKILL.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/contracts.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/worker-desk/SKILL.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/pr-shepherd/SKILL.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/scripts/inbound-monitor.sh: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/worker-desk/references/dispatch-runbook.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/desk/internal/loopengine/README.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/codex-smoke-runs/2026-09-12-codex-0.154.0.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/codex-smoke-runs/2026-09-13-codex-0.154.0-step5-rerun.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/the-desk/SKILL.md: follow-up desktools-v2/33 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/33 (owned rows flip to migrated)"
gate-why: >-
  These commands mint credentials and hold claims. Human sign-off confirms custody, token
  scope and claim semantics are unchanged, with no credential in argv, help or errors.
decision-trigger: "spec"
---

# Brief 33 — Migrate the token and claim custody verbs to Cobra and Viper

## Context

files:
- `tools/desk/cmd/desktoken` — `desktoken`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 2479 LOC, ~2 verbs, ~8 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskclaim-ref` — `deskclaim-ref`; module tools/desk; parser: manual os.Args parsing; 1265 LOC, ~6 verbs, ~0 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskclaim` — `deskclaim`; module tools/desk; parser: Go flag: one FlagSet per verb with manual verb dispatch; 686 LOC, ~4 verbs, ~8 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskboot` — `deskboot`; module tools/desk; parser: Go flag: FlagSet; 1116 LOC, ~0 verbs, ~4 flags at 3ad1ad83c871; release: desk-tools release archive (release.yml cmd/* loop) and Makefile DESK_CMDS.
- `tools/desk/cmd/deskboot/cli_test.go` (planned), `tools/desk/cmd/deskclaim-ref/cli_test.go` (planned), `tools/desk/cmd/deskclaim/cli_test.go` (planned), `tools/desk/cmd/desktoken/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms, TestCLIAdmissionBoundary.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-33.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-custody.md` (planned).

facts:
- desktoken prints a token file path, never the token; that output contract stays.
- deskclaim-ref speaks six verbs (acquire, progress, release, steal, show, list) over a ref protocol; the verbs and their exit codes stay.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
single-point-of-failure: credential-source and claim validation — it stays below the adapter; TestCLIAdmissionBoundary requests a disallowed role or claim through Cobra and directly.
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

1. Build a Cobra command tree for `desktoken`, `deskclaim-ref`, `deskclaim`, `deskboot`: root, every current verb and sub-verb, hidden internal entrypoints where they exist. Cobra/pflag does the parsing, argument validation and help. Remove the old parser once equivalent coverage exists; never run two parsers.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: TestCLIAdmissionBoundary drives refused inputs through the Cobra tree AND straight into the domain check with the adapter bypassed; both refuse, and no effect occurs on parse failure.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/desktoken ./cmd/deskclaim-ref ./cmd/deskclaim ./cmd/deskboot -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 4` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/desktoken ./cmd/deskclaim-ref ./cmd/deskclaim ./cmd/deskboot -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 4` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/desktoken ./cmd/deskclaim-ref ./cmd/deskclaim ./cmd/deskboot -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 4` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/desktoken ./cmd/deskclaim-ref ./cmd/deskclaim ./cmd/deskboot -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIAdmissionBoundary " "$f")" -eq 4` | exit 0; TestCLIAdmissionBoundary PASS in every package; refused inputs stay refused through Cobra and with the adapter bypassed |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-33.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, and a refusal moved out of the domain check into the adapter; each reddens the owner tests |
| 6 | check | `cd tools/desk && go test -count=1 ./cmd/desktoken/... ./cmd/deskclaim-ref/... ./cmd/deskclaim/... ./cmd/deskboot/...` | exit 0; every pre-existing test in the owned packages still passes |
| 7 | check | `cd tools/desk && GOOS=windows go vet ./cmd/desktoken ./cmd/deskclaim-ref ./cmd/deskclaim ./cmd/deskboot` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 8 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/33 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
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
