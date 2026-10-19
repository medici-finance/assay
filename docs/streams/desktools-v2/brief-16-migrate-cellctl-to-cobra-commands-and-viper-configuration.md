---
brief: "assay:assay:desktools-v2:16"
title: "Migrate cellctl to Cobra commands and Viper configuration"
why: >-
  Cellctl subcommand help currently errors instead of explaining the command, and
  parsing/configuration rules are duplicated manually. Make it the reference migration while
  keeping launch plans, model selection and credential boundaries compatible.
wave: 2
depends: ["desktools-v2/15"]
unblocks: ["desktools-v2/17", "desktools-v2/18", "desktools-v2/19", "desktools-v2/20", "desktools-v2/21", "desktools-v2/22", "desktools-v2/23", "desktools-v2/24", "desktools-v2/25", "desktools-v2/26", "desktools-v2/27", "desktools-v2/28", "desktools-v2/29", "desktools-v2/30", "desktools-v2/31", "desktools-v2/32", "desktools-v2/33", "desktools-v2/34", "desktools-v2/35", "desktools-v2/36", "desktools-v2/37", "desktools-v2/38", "desktools-v2/39", "desktools-v2/40", "desktools-v2/41", "desktools-v2/42", "desktools-v2/43", "desktools-v2/44", "desktools-v2/45", "desktools-v2/46", "desktools-v2/47", "desktools-v2/48", "desktools-v2/49", "desktools-v2/50", "desktools-v2/51", "desktools-v2/52", "desktools-v2/53", "desktools-v2/54", "desktools-v2/55", "desktools-v2/56", "desktools-v2/57", "comms-core/04"]
effort: "L"
gate: "human"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "yes"}
design: DR-cellctl-cobra
issues: [2111]
schema: "brief-v2"
outcome: none
version: 2
id: "962d7195-764d-47e8-b1d3-83d289fb061c"
authored: "2026-10-03 by coordinator"
sources:
  - "https://github.com/medici-finance/assay/issues/2111"
  - "docs/streams/desktools-v2/spec.md §9"
  - "freshness-checked 2026-10-03 @ b165003865f6; cellctl main.go/desk.go/cell.go and release.yml read after authenticated origin/main fetch"
  - "https://github.com/spf13/cobra"
  - "https://github.com/spf13/viper"
exec-tier: "strong"
exec-tier-why: >-
  Design and cross-component configuration/CLI compatibility require a strong executor; a
  happy-path parser test cannot establish unchanged authority or downstream behavior.
domain: "complicated"
consumers:
  - "tools/desk/cmd/cellctl: follow-up desktools-v2/16 (all command constructors and config binding)"
  - "tools/cellctl/tests and tools/cellctl/testdata: follow-up desktools-v2/16 (explicit help-delta and retained behavior fixtures)"
  - "docs/cellctl.md, docs/cellctl-cadence.md, docs/cellctl-model-policy.md, docs/cellctl-windows.md: follow-up desktools-v2/16 (generated CLI reference and usage)"
  - "tools/desk/internal/cli: follow-up desktools-v2/15 (shared adapter; consume it)"
  - "remaining maintained CLI migrations: follow-up desktools-v2/15 (authors bounded children after inventory)"
gate-why: >-
  Cellctl configuration selects executable paths, role identity and credential locations.
  Human sign-off confirms the parser migration preserves existing custody, source
  restrictions and execution admission; choosing Cobra and Viper was already requested in
  issue 2111.
decision-trigger: "spec"
---

# Brief 16 — Migrate cellctl to Cobra commands and Viper configuration

## Context

files:
- `tools/desk/cmd/cellctl/` — main/command construction, per-verb argument loops, `cell.go`, `help.go`, `usage.go`, `usage.txt`, tests and fixtures.
- `tools/cellctl/tests/`, `tools/cellctl/testdata/cellctl-shell-oracle.sh` — retained behavior comparisons and explicit help differences.
- `tools/desk/internal/clicontract/testdata/mutations/desktools-v2-16.json` (planned) — muhar mutation spec for cellctl.
- `tools/desk/internal/clicontract/` — cellctl black-box cases and migration state in `docs/streams/desktools-v2/cli-migration.json`.
- `docs/cellctl.md`, `docs/cellctl-cadence.md`, `docs/cellctl-model-policy.md`, `docs/cellctl-windows.md`.
- `changelog/cellctl-cobra-viper.md` (planned).

facts:
- v1.0.31 prints root help with exit 0; `cellctl desk --help` exits 1 with argument usage; `cellctl desk house the-desk --help` exits 3 with unknown flag --help. Reproduced 2026-10-03 without starting a role.
- `main.go` echoes roster configuration before dispatch; `cmdDesk` loads a cell before its flag loop. Help must move ahead of those operations.
- The shell oracle preserves launch/output behavior; it is a test fixture, not the new source for generated help.
- `cell.go` Env tracks key presence separately from value and parses shell-style assignments without executing a shell. `model.go`, `provider_defaults.go` and policy files carry precedence rules that a generic Viper merge must not replace accidentally.

layering: Cobra constructs the command tree and passes validated typed inputs into existing handlers; command-local Viper resolves only declared inputs through compatibility adapters. Launch planning, custody, policy and external execution retain their current owners.
single-point-of-failure: parser-selected options must not grant authority; domain admission remains an independent check below the adapter, and fixture launch/custody tests exercise it with the parser bypassed.
design-fit:
  owner: cellctl command/configuration adapter; existing launch and custody owners stay authoritative
  contract: none — consumes cli-contract.md delivered by /15
  retires: [manual os.Args verb/flag dispatch, embedded help copied from the shell oracle, duplicate configuration merge paths]
  weight: verbs +0 except Cobra's standard help; flags +0 except conventional help aliases/spellings; refusals net nonpositive; rule-text lines decrease by retiring duplicate help
  why-add: standard upstream help behavior replaces bespoke parsing; keep only documented compatibility normalization that existing callers need

## Human decision

At pickup, record the concrete compatibility report and any proposed configuration-format
transition before requesting approval. Library choice is already settled. Human acceptance
covers preserved credential-source restrictions, execution admission and any explicitly
listed command/configuration compatibility changes; silence never authorizes a weaker gate.

Default if no answer: none — blocks until answered.

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve the existing domain checks, custody decisions and exit
codes; a CLI library is not a new authority source. Independent verification owns Evidence
and lifecycle advancement. Proposed tests and support files below are deliverables, not
claims that they already exist.

## Library-first amendment — 2026-10-08

Read `docs/library-first.md`. Keep this a command/configuration adapter migration. Call
existing application libraries or the typed client for an independent service; do not move
policy, custody or admission into Cobra handlers. Preserve help/version offline behavior and
underlying admission tests. No dependency on the SDK extraction or cell registry is added;
coordinate overlapping files with the extraction owner when both changes are in flight.

## Task

1. Build the complete Cobra command tree: root, all current visible verbs/subverbs, and internal entrypoints such as model-policy/container-run (hidden where appropriate). Use Cobra/pflag for actual parsing, argument validation and help generation. Model --cells-root as a real persistent option. Remove the old top-level/command-specific parsers after equivalent coverage exists; do not run two parsers in parallel.
2. Make `cellctl -h`, `cellctl --help`, `cellctl help desk`, `cellctl desk --help` and help at nested commands succeed with exit 0 without a real cell, role, credentials or config. Version remains the stamped release version. Help/version must not echo roster values, mint tokens, create locks/worktrees, probe Docker/network or launch anything. Required-argument checks apply to execution, not to help. Disable unrequested auxiliary Cobra commands (for example completion) unless deliberately added with tests/docs.
3. Bind allowed settings through a fresh Viper instance into typed options. Establish a golden precedence matrix BEFORE the port for flags, model override env, cell.env pins, shared/per-cell providers.json, model policy, defaults, explicit empty and absent keys. Preserve per-setting precedence, source restrictions and case behavior on POSIX/Windows; do not silently substitute Viper's generic env-over-file rule. Keep existing cell.env syntax readable through a non-executing compatibility decoder as necessary; migrate persistence only through reviewed, atomic, round-trip-compatible behavior. Do not read arbitrary config locations or expose credential material in help/errors/show.
4. Prove flags such as --model=value, --model value, --cells-root placement and -- termination behave as documented. Inventory and preserve legacy forms used by scripts, generated shims, cadence supervisors and model-policy hooks. Required flags, positional counts, unknown flags and bad types must fail predictably before execution. Retain operational exit codes and machine output; define only explicit help/parse improvements.
5. Retain model namespace/provider resolution, pinned models, --set persistence, native path semantics, launcher argv boundaries, custody, lock and cadence behavior. Test typed options through generated launch argv/env into a fake harness, and invoke the underlying domain admission independently with disallowed inputs to prove moving checks into Cobra did not remove the lower layer.
6. Generate reference help from command definitions and reconcile the operator docs. Retire the embedded shell-header help parity assertion while retaining behavior parity for non-help commands. Capture non-help transcripts from the pre-migration Go implementation at its recorded SHA using the existing fake/DRY_RUN fixture harness, then compare the migrated binary to them in TestCLINonHelpParity. The shell parity script defaults BOTH sides to the oracle, so a bare invocation is not migration evidence. Existing Go-versus-shell differences are a separately recorded baseline, not new regressions to hide or an instruction to expand this brief. Record deliberate differences in a reviewed compatibility table; never suppress unrelated failing parity tests. Mark only cellctl's inventory row migrated after its black-box suite passes.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIHelpOffline$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIHelpOffline " "$f"` | exit 0 with TestCLIHelpOffline PASS; built binary root/nested help and version work under missing/malformed cell and credential state; all instrumented effect counts and config reads are zero |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIConfigLaunch$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIConfigLaunch " "$f"` | exit 0 with TestCLIConfigLaunch PASS; precedence/empty-value fixtures reach exact fake-harness argv/env, including separate sequential invocations, --set round trips and Windows source semantics |
| 3 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIAdmissionBoundary " "$f"` | exit 0 with TestCLIAdmissionBoundary PASS; rejected execution remains rejected both through Cobra and when the adapter is bypassed; no effect occurs on parse failure |
| 4 | check | `cd tools/desk && go test -count=1 ./cmd/cellctl/... ./internal/cli/...` | exit 0; all existing launcher, custody, hook, cadence and model-policy regressions retained; no empty package/test selection |
| 5 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLINonHelpParity$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLINonHelpParity " "$f"` | exit 0 with TestCLINonHelpParity PASS; current Go binary matches the pre-migration Go fixture transcripts for non-help commands through the existing fake/DRY_RUN harness, with no live launch or oracle-versus-itself comparison |
| 6 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; every inherited floor seed still runs and passes |
| 7 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |
| 8 | check +mutation | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-16.json` | exit 0 and the last line reads `Totals: N caught, 0 NOT CAUGHT, 0 could-not-mutate.` with N equal to the spec's mutation count; the harness proves the baseline green and its positive control caught, then plants: help running after the roster echo or cell load, an undeclared config key binding, one legacy flag form dropped, and a refusal moved out of the domain check into the adapter; each reddens the cellctl owner tests |

## Pre-mortem

Help loads credentials before printing: row 1 uses hostile/missing config and effect counters.
Viper changes an empty pin or credential source: rows 2–3 compare before/after fixtures.
Required flags intercept help or a child suppresses the parent's guard: rows 1 and 3 cover both.
Only imports change while manual loops remain: source/command-tree inspection is a review obligation,
backed by /17's independent migration coverage check.
The owner tests pass on the happy path but miss a broken guard: row 8 plants each break and requires it to redden.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->
### Non-implementer verifier run — 2026-10-14 claude-opus-5-5 verifier at 946fd67330e5 (8/8 rows exit 0; mutation 37/37 caught; consumers row vacuous at merged main)

Verification-Attestation: medici-finance/assay#2938 run=fc91868f1cc6b6710e8a103efa310876bdd61264f5426a154017414d611d979e source=946fd67330e5bbc6acc0fe3e0bbe9521d7786caa brief=docs/streams/desktools-v2/brief-16-migrate-cellctl-to-cobra-commands-and-viper-configuration.md model=claude-opus-5-5 tier=strong

Fresh non-implementer run on merged main 946fd67330e5 (delivery: PR #2391, e7b14f343), detached home, offline (KUBECONFIG=/dev/null), short private TMPDIR, go1.27.2 darwin/arm64, statusgen v1.0.42. Admission check passed before the first row and again before writing this section. Row 8 mutates files in place and restores each one; the home was clean afterwards. This brief is gate: human (decision issue #2240): this run records Evidence only, and the sign-off belongs to the human.

| # | Command | Result | Output | Date | Runner |
|---|---|---|---|---|---|
| 1 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIHelpOffline$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIHelpOffline " "$f"` | PASS | exit 0; `--- PASS: TestCLI Help Offline (4.57s)`; 6/6 subtests PASS (absent-registry, cell-env-absent, cell-env-not-key-value, cell-env-unterminated-quote, unreadable-credentials, roster-echo-count) | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIConfigLaunch$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIConfigLaunch " "$f"` | PASS | exit 0; `--- PASS: TestCLI Config Launch (1.84s)`; 3/3 subtests PASS (fake-harness-argv, sequential-invocations-and-set-round-trip, nothing-to-persist-is-refused). The Windows case-folding branch runs only on Windows; this darwin run covers the POSIX side | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLIAdmissionBoundary$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIAdmissionBoundary " "$f"` | PASS | exit 0; `--- PASS: TestCLI Admission Boundary (1.58s)`; 8/8 subtests PASS (the-desk-opus, unknown-harness, unknown-cockpit, persist-nothing, persist-with-policy, override-from-environment-is-checked-too, parse-failure-has-no-effect, hook-parse-failures-exit-2) | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 4 | `cd tools/desk && go test -count=1 ./cmd/cellctl/... ./internal/cli/...` | PASS | exit 0; `ok .../tools/desk/cmd/cellctl 108.224s`, `ok .../tools/desk/internal/cli 0.140s`; both packages selected, neither empty | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 5 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./cmd/cellctl -run '^TestCLINonHelpParity$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLINonHelpParity " "$f"` | PASS | exit 0; `--- PASS: TestCLI Non Help Parity (9.03s)`; compares the migrated binary against the pre-migration Go golden transcripts | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 6 | `bash tools/desk/internal/regression/check-floor.sh` | PASS | exit 0; `seed passes=26`; 26 top-level PASS lines | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 7 | `statusgen --root . --consumers` | PASS (vacuous) | exit 0; `consumers: no brief files in the diff against 946fd67330e5... — nothing to corroborate`. At merged main the diff is empty (known shape, #1915). Extra read-only run with `--base 423232013 --brief desktools-v2/16`: exit 0, 0 corroborated, 0 disproved, 5 claims not judged (unchanged since the merge-base) | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |
| 8 | `cd tools/desk && go run ./cmd/muhar -j 1 -spec internal/clicontract/testdata/mutations/desktools-v2-16.json` | PASS | exit 0; `Harness healthy: baseline GREEN, positive control CAUGHT.`; last line `Totals: 37 caught, 0 NOT CAUGHT, 0 could-not-mutate.`; the spec has 37 mutations, so N = 37 | 2026-10-14 | assay-verifier-app[bot] @ 946fd67330e5 (on-behalf-of human:ian) (git-config) |

Additional check: a binary built from 946fd67330e5, run with an empty HOME and a missing cells root, printed help and exited 0 for `-h`, `--help`, `help desk`, `desk --help`, `desk house the-desk --help` and `scratch --help`. `completion` was refused with exit 3, and `--version` printed `dev-946fd67330e5`. Review note for the human: the consumer claim on tools/cellctl/tests and tools/cellctl/testdata has no matching change in the delivery commit. The help-parity assertion that was retired lived in the tools/desk usage test, which the delivery deleted.

RISK-VALUE: DERIVED — UsageExit = 3 @ tools/desk/cmd/cellctl/cobra.go:193 — the pre-migration die() exited 3 (main.go:38 at e7b14f343^1), and the brief requires exit codes to stay the same. Row 8's control mutation (3 to 4) was caught.
RISK-VALUE: DERIVED — legacy-shape refusal return 3 @ tools/desk/cmd/cellctl/cobra.go:154 — this is the same refusal code as legacy die(), so the usage-error class is unchanged.
RISK-VALUE: DERIVED — hookBlockExit = 2 @ tools/desk/cmd/cellctl/policy_enforce.go:44 (the new fail-closed rewrite at cobra.go:138 applies it) — under the Claude Code hook protocol, exit 2 is the only status that blocks. Any other non-zero exit, including usage 3, lets the action through, so a hook-parse failure has to map to 2.
RISK-VALUE: DERIVED — env source bindings DESK_MODEL_OVERRIDE @ cobra.go:581, CELL_CADENCE @ :587, CELL_TICK_BUDGET @ :588, CELL_PROVIDER @ :804, DESK_APP_ID @ :843, DESK_SESSION @ :961, ASSAY_SCRATCH_ID @ :967 (all tools/desk/cmd/cellctl/cobra.go) — each is the name the pre-migration cellctl already read (scratch.go:38 and :45 at e7b14f343^1 for session and id). No binding is marked Secret. Row 8 planted an undeclared env name and an undeclared config source, and both were caught.

**VERIFY: PASS** — 8/8 rows exit 0 (row 7 vacuous at merged main, #1915); Evidence only: gate: human, sign-off stays with the human (#2240)

## Review
Gate: human. No weakening of existing authority/custody checks is included in this migration.
