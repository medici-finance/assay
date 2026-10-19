---
brief: "assay:assay:desktools-v2:54"
title: "Port the security control scripts"
why: >-
  Five security-control scripts (the forge-ban counter, the automerge refusal decision, the
  image layer secret scan, the container entrypoint and GitLab token renewal) each carry their
  own parser. Moving them behind Go commands keeps each control's decision while removing the
  shell parsers.
wave: 4
depends: ["desktools-v2/16", "desktools-v2/30"]
unblocks: ["desktools-v2/17"]
effort: "M"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "4a6d27b1-797f-4763-96f8-b5969608bd8b"
authored: "2026-10-04 by desktools-v2/15"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "docs/streams/desktools-v2/cli-contract.md"
  - "docs/streams/desktools-v2/cli-migration.json (rows owned by desktools-v2/54)"
  - "freshness-checked 2026-10-04 @ 3ad1ad83c871; entrypoint sources, release.yml and Makefile read after origin/main fetch"
exec-tier: "strong"
exec-tier-why: >-
  Cross-component CLI and configuration compatibility: every legacy caller listed below must
  keep working, and a happy-path parser test cannot show that.
domain: "complicated"
consumers:
  - "tools/desk/scripts/forge-ban.sh: follow-up desktools-v2/54 (command tree, declared configuration, contract tests)"
  - "tools/evidence-automerge/automerge-refusal.sh: follow-up desktools-v2/54 (command tree, declared configuration, contract tests)"
  - "containers/scripts/layer-secret-scan.sh: follow-up desktools-v2/54 (command tree, declared configuration, contract tests)"
  - "containers/entrypoint.sh: follow-up desktools-v2/54 (command tree, declared configuration, contract tests)"
  - "tools/renew-fleet-gitlab-tokens.sh: follow-up desktools-v2/54 (command tree, declared configuration, contract tests)"
  - "tools/desk/cmd/deskcontrols: follow-up desktools-v2/54 (new Go entrypoint and its registry row)"
  - "tools/desk/cmd/deskentry: follow-up desktools-v2/54 (new Go entrypoint and its registry row)"
  - "tools/desk/cmd/deskgitlab: follow-up desktools-v2/54 (new Go entrypoint and its registry row)"
  - ".github/workflows/forge-surface-control.yml: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/evidence-automerge.yml: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "ci/staged-workflows/evidence-automerge.yml: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "tools/ci-load/activation/evidence-automerge.yml: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - ".github/workflows/docker-publish.yml: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/iso9001-mapping.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/docker.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/quality/QUALITY.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay-gitlab.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/adopting-assay.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "plugins/assay/skills/install/SKILL.md: follow-up desktools-v2/54 (legacy invocation forms kept; quoted help reconciled)"
  - "docs/streams/desktools-v2/cli-migration.json: follow-up desktools-v2/54 (owned rows flip to migrated)"
gate-why: >-
  These scripts are security controls: a secret scan, a seam-ban counter, an automerge
  refusal, a credential preflight and token rotation. Human sign-off confirms each decision,
  exit code and fail-closed default survives the port.
decision-trigger: "spec"
---

# Brief 54 — Port the security control scripts

## Context

files:
- `tools/desk/scripts/forge-ban.sh` — `forge-ban.sh`; module bash; parser: bash positional arguments; 223 lines at 3ad1ad83c871; release: repo tooling; run by forge-surface-control.yml. Target: `tools/desk/cmd/deskcontrols`.
- `tools/evidence-automerge/automerge-refusal.sh` — `automerge-refusal.sh`; module bash; parser: no arguments (wrapper); 165 lines at 3ad1ad83c871; release: repo tooling; run by evidence-automerge.yml. Target: `tools/desk/cmd/deskcontrols`.
- `containers/scripts/layer-secret-scan.sh` — `layer-secret-scan.sh`; module bash; parser: bash manual flag loop; 256 lines at 3ad1ad83c871; release: repo tooling; run by docker-publish.yml. Target: `tools/desk/cmd/deskcontrols`.
- `containers/entrypoint.sh` — `entrypoint.sh`; module bash; parser: bash positional arguments; 133 lines at 3ad1ad83c871; release: repo tooling; run by docker-publish.yml. Target: `tools/desk/cmd/deskentry`.
- `tools/renew-fleet-gitlab-tokens.sh` — `renew-fleet-gitlab-tokens.sh`; module bash; parser: bash manual flag loop; 714 lines at 3ad1ad83c871; release: repo tooling; operator-invoked. Target: `tools/desk/cmd/deskgitlab`.
- `tools/desk/cmd/deskcontrols/cli_test.go` (planned), `tools/desk/cmd/deskentry/cli_test.go` (planned), `tools/desk/cmd/deskgitlab/cli_test.go` (planned) — TestCLIHelpOffline, TestCLIConfigFlow, TestCLILegacyForms, TestCLIAdmissionBoundary.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-54.json` (planned) — muhar mutation spec for the owned commands.
- `docs/streams/desktools-v2/cli-migration.json` — owned rows flip to migrated.
- `changelog/cli-security.md` (planned).

facts:
- Token renewal joins the `tools/desk/cmd/deskgitlab` command from desktools-v2/30, so this brief depends on /30.
- forge-ban, automerge-refusal and layer-secret-scan move to `tools/desk/cmd/deskcontrols`; the container entrypoint moves to `tools/desk/cmd/deskentry`.
- Contract: `docs/streams/desktools-v2/cli-contract.md` (from /15). Desk-module commands build on `tools/desk/internal/cli`; standalone modules use Cobra and Viper directly. Legacy consumers named in the registry are the files that mention each entrypoint most at 3ad1ad83c871 (up to six per row); re-enumerate every caller at pickup.
- A migrated script row names its Go entrypoint in `migrated_to`, and the script itself must name that entrypoint (it delegates); a new Go entrypoint gets its own registry row, owned by this brief, state migrated.

layering: Cobra constructs the command tree and passes validated typed options into the existing handlers; a fresh command-local Viper resolves only declared keys. Domain checks, custody and external effects keep their current owners.
single-point-of-failure: each control's decision — it moves into a Go handler unchanged; TestCLIAdmissionBoundary plants a secret, a banned call and a refused automerge through Cobra and directly.
design-fit:
  owner: each command's own package; domain owners unchanged
  contract: none — consumes cli-contract.md delivered by /15
  retires: [hand-rolled flag loops and usage strings, shell argument parsers in the ported scripts]
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

1. Port each script's behaviour into its Go target (`tools/desk/scripts/forge-ban.sh` → `tools/desk/cmd/deskcontrols`; `tools/evidence-automerge/automerge-refusal.sh` → `tools/desk/cmd/deskcontrols`; `containers/scripts/layer-secret-scan.sh` → `tools/desk/cmd/deskcontrols`; `containers/entrypoint.sh` → `tools/desk/cmd/deskentry`; `tools/renew-fleet-gitlab-tokens.sh` → `tools/desk/cmd/deskgitlab`). Reduce the script to a transparent adapter that execs the target with equivalent arguments, or retire it in the same change that removes every consumer. Add a registry row for any new Go entrypoint.
2. Root and nested `-h`, `--help`, `help <verb>` and version exit 0 before any config, roster, credential or network read, lock, worktree or child process, even with absent or malformed configuration. Required-argument checks apply to execution, not to help. Disable Cobra's completion command unless deliberately added with tests and docs.
3. Write the golden precedence matrix BEFORE porting: for every setting, flag, environment, config file and default, with explicit-empty versus unset. Bind only declared keys through a fresh Viper instance per invocation; no AutomaticEnv, config search, remote config, global Viper or credential values on argv. A tool without configuration binds only its declared flags and defaults.
4. Inventory every legacy form the consumers use and keep it: `--f=v`, `--f v`, single-dash Go-flag spellings where callers use them, `--` termination, repeated flags, dash-leading values and global-flag placement. Unknown flags and bad values fail before any effect, with the exit code the contract states. Paths behave natively on Windows and POSIX.
5. Prove the lower layer still holds: TestCLIAdmissionBoundary drives refused inputs through the Cobra tree AND straight into the domain check with the adapter bypassed; both refuse, and no effect occurs on parse failure.
6. Declare TestCLIHelpOffline, TestCLIConfigFlow and TestCLILegacyForms in each migrated package, using the fixture cases listed in cli-contract.md. Flip this brief's registry rows to migrated only after they pass, then reconcile any consumer doc that quotes old help text.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskcontrols ./cmd/deskentry ./cmd/deskgitlab -run '^TestCLIHelpOffline$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIHelpOffline " "$f")" -eq 3` | exit 0; TestCLIHelpOffline PASS in every migrated package; root and nested help and version under absent and malformed config with zero effects and zero config reads |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskcontrols ./cmd/deskentry ./cmd/deskgitlab -run '^TestCLIConfigFlow$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIConfigFlow " "$f")" -eq 3` | exit 0; TestCLIConfigFlow PASS in every package; each precedence-matrix row and explicit-empty versus unset case reaches the typed handler with the expected value |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskcontrols ./cmd/deskentry ./cmd/deskgitlab -run '^TestCLILegacyForms$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLILegacyForms " "$f")" -eq 3` | exit 0; TestCLILegacyForms PASS in every package; every consumer's legacy form, Windows and POSIX path cases and unknown-flag exits behave as documented |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/deskcontrols ./cmd/deskentry ./cmd/deskgitlab -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1; test "$(grep -c -F -e "--- PASS: TestCLIAdmissionBoundary " "$f")" -eq 3` | exit 0; TestCLIAdmissionBoundary PASS in every package; refused inputs stay refused through Cobra and with the adapter bypassed |
| 5 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-54.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then for each owned command plants: help running after a config read, an undeclared config key binding, one legacy flag form dropped, and a refusal moved out of the domain check into the adapter; each reddens the owner tests |
| 6 | check | `cd tools/desk && go test -count=1 ./cmd/deskcontrols/... ./cmd/deskentry/... ./cmd/deskgitlab/...` | exit 0; every pre-existing test in the owned packages still passes |
| 7 | check +flow | `bash tools/evidence-automerge/automerge-refusal_test.sh && bash containers/scripts/layer-secret-scan.test.sh && bash tools/renew-fleet-gitlab-tokens_test.sh` | exit 0; the scripts' existing offline suites pass against the adapters |
| 8 | check | `cd tools/desk && GOOS=windows go vet ./cmd/deskcontrols ./cmd/deskentry ./cmd/deskgitlab` | exit 0; the owned packages compile for Windows (native Windows behaviour runs on the CI lane, not here) |
| 9 | check +dereference | `f=$(mktemp) && cd tools/desk && CLI_OWNER=desktools-v2/54 go test -count=1 -v ./internal/clicontract -run '^TestCLIOwnerMigrated$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIOwnerMigrated " "$f"` | exit 0; TestCLIOwnerMigrated PASS; every row this brief owns is migrated, the registry validates and the contract tests pass in each migrated package |
| 10 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 11 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

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
