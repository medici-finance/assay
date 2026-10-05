---
brief: "assay:assay:desktools-v2:15"
title: "Cobra and Viper foundation and complete CLI migration routing"
why: >-
  Operators and automated callers encounter inconsistent parsing, help and configuration
  across the tool suite. Establish one tested command contract and an exhaustive rollout map
  so every maintained tool receives the migration without a single oversized rewrite.
wave: 1
depends: []
unblocks: ["desktools-v2/16"]
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "64a4258c-4332-4e66-8c5e-34ed26566ee6"
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
  - "tools/desk/internal/cli: follow-up desktools-v2/15 (new small adapter package and tests)"
  - "tools/desk/internal/clicontract: follow-up desktools-v2/15 (new discovery and compatibility harness)"
  - "tools/desk/go.mod and tools/desk/go.sum: follow-up desktools-v2/15 (pin Cobra and Viper)"
  - "tools/desk/cmd/cellctl: follow-up desktools-v2/16 (reference consumer)"
  - "all remaining maintained command entrypoints: follow-up desktools-v2/15 (inventory and bounded implementation-brief authoring; migration is owed to those children)"
  - "suite completion checks: follow-up desktools-v2/17 (all inventory rows must migrate or name a delivered retirement)"
---

# Brief 15 — Cobra and Viper foundation and complete CLI migration routing

## Context

files:
- NEW `tools/desk/internal/cli/` — Cobra constructors, command-local Viper binding and test fixtures.
- NEW `tools/desk/internal/clicontract/` — executable discovery, migration-registry validation and reusable black-box checks.
- `tools/desk/go.mod`, `tools/desk/go.sum` — pinned upstream dependencies.
- NEW `docs/streams/desktools-v2/cli-contract.md` (planned) and `cli-migration.json`.
- `docs/streams/desktools-v2/README.md`, `brief-16-*.md`, `brief-17-*.md` and NEW bounded migration briefs starting at the next free number.
- `changelog/cli-foundation.md` (planned).

facts:
- At the freshness SHA, `tools/desk/cmd/cellctl/main.go` dispatches `os.Args` manually; `desk.go` loops over literal flag names; `usage.go` embeds the shell oracle's header.
- `tools/desk/cmd/cellctl/cell.go` distinguishes unset from explicitly empty values and overlays cell.env on inherited values. Generic Viper precedence is not automatically compatible.
- `Makefile` DESK_CMDS and `.github/workflows/release.yml` package `tools/desk/cmd/*`; statusgen and qualgen are separate Go modules and released binaries. Module-local maintenance CLIs also exist under `tools/`.
- The current desk module has neither Cobra nor Viper in go.mod. Re-establish the inventory from tracked Go package/main declarations, release/build declarations and documented operator entrypoints, not only files literally named main.go.

layering: a small CLI adapter owns parsing, help and allowed configuration binding; typed options cross into existing handlers. Domain admission and external effects remain in their current owners. Independent modules consume upstream libraries directly, not internal deskkit or a new cross-module policy framework.
design-fit:
  owner: command adapters in each owning Go module
  contract: none — this brief defines the CLI contract
  retires: [handwritten flag loops and embedded-help copies as each consumer migrates, duplicate untyped configuration precedence]
  weight: verbs +0; flags +0 in the foundation; refusals +0 in production; rule-text lines positive for one shared contract
  why-add: use upstream Cobra/Viper with the smallest adapter; retire per-command parsing rather than create another parser or a global mutable registry

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve the existing domain checks, custody decisions and exit
codes; a CLI library is not a new authority source. Independent verification owns Evidence
and lifecycle advancement. Proposed tests and support files below are deliverables, not
claims that they already exist.

## Task

1. Publish the contract defined by spec §9: Cobra command trees and flag definitions are the help source; root and nested help/version are parsed before cell/config/credential reads, guard initialization or effects; preserve non-help stdout/stderr, flag spellings, positional forms and exit semantics. Define required args, unknown flags, `--`, repeated flags, `--name=value`, dash-leading values and global-flag placement explicitly. Document deliberate help improvements separately from legacy compatibility.
2. Add a small desk-local adapter with fresh Cobra/Viper instances per invocation and injected streams/effects. Viper must actually resolve explicitly declared settings into typed handler options. Use an allowlist of environment/config bindings, preserve unset versus empty and per-key source restrictions. Do not use blanket AutomaticEnv, automatic config search, remote config, credential values on argv, or Viper's global singleton. Existing roster/custody parsers remain authoritative validators; Viper cannot make a rejected source admissible. Tools without configuration bind only declared flags/defaults; do not invent config files or options merely to use Viper.
3. Build the exhaustive tracked-entrypoint inventory. Each row names executable, owning module/package, release/build source, CLI/config consumers, current parser, migration owner and state. Include all maintained desk binaries, statusgen, qualgen, maintenance/lint/generation CLIs and operator-facing script launchers. Test fixtures/vendor/demo code may be excluded only by an explicit reason and source classification; a shipped launcher is not a fixture. Scripts that expose their own CLI must be ported behind Go Cobra/Viper entrypoints or reduced to transparent adapters with a named implementation owner. Never silently exempt a tool because it is small or outside tools/desk.
4. In the SAME deliverable, author concrete implementation briefs for every remaining inventory row (cellctl is already owned by /16). Allocate at most five simple binaries per child; one complex launcher/large CLI such as statusgen per child; each <= L, preferably M. State exact paths, legacy consumers, risk-derived gate, compatibility cases and meaningful Verify commands. Each child owns its CLI platform cases with executable Verify rows covering Windows/POSIX option forms and paths, explicit-empty versus unset keys, per-key precedence and effect-free help; it cannot delegate those cases to /12. Use assay:author-brief and statusgen newbrief. Children depend on /16's reference adapter; unrelated children may then proceed independently. Make /17 depend on EVERY child, update inverse edges and waves, and route each inventory row to an existing brief. This brief is not done with an inventory or promise to author later. If scope exceeds L, split the foundation/routing work before dispatch.
5. Add tests proving inventory coverage against discovered sources and proving every row has an owning brief. Negative controls must insert a new command, remove an inventory row, orphan an owner and remove one child from /17's dependencies; each must fail. Adding a new maintained command after the inventory must likewise fail until routed. Validate test names and command cases are nonempty and select real tests.
6. Own reusable Windows/POSIX CLI fixtures for option forms, paths, per-key precedence, explicit-empty/unset values and effect-free help in the new harness; these do not wait for /12. Demonstrate the adapter with fixture commands: flag/config/env/default resolution reaches a typed fake handler, help/version reaches zero effects even with absent or malformed config, and execution still reaches the existing admission layer. Repeated in-process invocations cannot leak flags or config into the next run. Enumerate each existing caller in the inventory; do not enforce the final all-migrated gate before its owners land.
7. Keep the established regression floor; update module-local testing/CI path coverage for new cross-module readers. Workflow mutations use the project's reviewed promotion path; a staged workflow is not claimed active.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd tools/desk && go test -count=1 -v ./internal/cli` | exit 0; named tests TestCLIHelpNoEffects, TestCLIConfigFlow and TestCLIReentrant PASS; each executes a command through the adapter into instrumented effects or a typed handler; Windows/POSIX fixture cases exercise option forms, paths, per-key precedence and empty/unset values |
| 2 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIInventory$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIInventory " "$f"` | exit 0 with TestCLIInventory PASS; discovered maintained CLI set equals inventoried/routed set, including standalone modules and operator scripts |
| 3 | check +mutation | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIRoutingMutations$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIRoutingMutations " "$f"` | exit 0 with TestCLIRoutingMutations PASS; added command, omitted row, orphan owner and missing final-gate dependency each cause the inner validator to fail for the planted reason |
| 4 | check | `statusgen --root . --lint --diff-base refs/remotes/origin/main` | exit 0; every child resolves, inverse edges/waves agree, no newly introduced brief defect |
| 5 | check +flow | `bash tools/desk/internal/regression/check-floor.sh` | exit 0 with every registered seed run; no removed or weakened inherited behavior |
| 6 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

An imports-only migration passes a source grep: row 1 requires configuration-to-handler flow.
A hardcoded inventory misses a new command: rows 2–3 discover and mutate the source set.
A future migration is unowned or omitted from the finish gate: row 3 breaks that edge.
A global Viper instance contaminates a second run: row 1 repeats invocations with disjoint inputs.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

### Non-implementer verifier — VERIFY: PASS — 2026-10-04 claude-opus-5-5-verifier

Brief desktools-v2/15 (Cobra and Viper foundation and complete CLI migration routing), implemented by medici-finance/assay#2147, verified against merged main 5f5072d89 (pre-merge parent 4aac3271b used for supplementary diff-based runs). Expectations were derived from the brief Task/DoD/Verify and spec section 9 before the diff was read.

| # | Command | Expected | Observed | Date / Runner |
|---|---------|----------|----------|---------------|
| 1 | cd tools/desk && go test -count=1 -v ./internal/cli | exit 0; TestCLIHelpNoEffects, TestCLIConfigFlow, TestCLIReentrant PASS through the adapter, Windows/POSIX option forms, per-key precedence, empty vs unset | exit 0; all three named tests PASS (164 PASS lines): help forms over absent / malformed / unterminated-quote config, precedence matrix, unset-vs-empty, long-equals / long-space / go-equals / go-space x posix and win paths, secret from env only, dash-leading and double-dash, 3 reentrant rounds with global viper holding no keys | 2026-10-04 claude-opus-5-5-verifier |
| 2 | Verify row 2 (TestCLIInventory, captured log, grep for the top-level PASS line) | exit 0; discovered CLI set equals the routed set | exit 0; "--- PASS: TestCLIInventory (0.35s)"; log "discovered 181 entrypoints: 114 routed rows (84 go, 30 script) across 38 owners, 1 excluded rows, 66 excluded by rule"; my independent discovery of package-main dirs and script launchers matched with no missing and no extra rows | 2026-10-04 claude-opus-5-5-verifier |
| 3 | Verify row 3 (TestCLIRoutingMutations, captured log, grep for the top-level PASS line) | exit 0; added command, omitted row, orphan owner, missing final-gate dependency each fail for the planted reason | exit 0; "--- PASS: TestCLIRoutingMutations (11.60s)", 21 subtests PASS (added desk command / module main / script / extensionless launcher, omitted row, orphan owner, missing final-gate dependency, child skips reference, budget, shipped-excluded, empty test selector) | 2026-10-04 claude-opus-5-5-verifier |
| 4 | statusgen --root . --lint --diff-base refs/remotes/origin/main | exit 0; children resolve, inverse edges and waves agree, no new brief defect | exit 0, "LINT: PASS" (at merged main the base resolves to HEAD so it ran the full lint, 0 problems); supplementary run with diff base 4aac3271b: exit 0, "0 diff-introduced problem(s)", LINT: PASS | 2026-10-04 claude-opus-5-5-verifier |
| 5 | bash tools/desk/internal/regression/check-floor.sh | exit 0, every registered seed runs | exit 0, "seed passes=26" (equals the 26 manifest rows above Dropped); see observation O1 for two environment-caused reds that are not attributable to #2147 | 2026-10-04 claude-opus-5-5-verifier |
| 6 | statusgen --root . --consumers | exit 0; no consumer routing disproved | exit 0. Witnessed at #2147's PR head 13ae74e8e3bd in a clean throwaway clone; the brief file there is byte-identical to main's. The verbatim command's default base resolves to the merge-base 556aa2a914e3: 39 briefs, "summary: 474 corroborated, 0 disproved, 9 unchecked, 0 brief(s) claiming nothing". The 9 unchecked are claims unchanged since the merge-base (follow-ups owned by /15, /16, /17). At merged main the same command finds no brief diff ("nothing to corroborate"), which is could-not-check, so that run is not the witness | 2026-10-04 claude-opus-5-5-verifier |

**Verifier mutations on the real tree (each reverted; restored tree re-PASSes TestCLIInventory / TestCLIConfigFlow):**

| Mutation | Result |
|----------|--------|
| new Go main under tools/desk cmd (zzverifier) | TestCLIInventory FAIL: "unrouted entrypoint tools/desk/cmd/zzverifier (go)" |
| new script launcher under plugins/assay scripts | TestCLIInventory FAIL: "unrouted entrypoint ... (script)" |
| removed the deskwt registry row | TestCLIInventory FAIL: "unrouted entrypoint tools/desk/cmd/deskwt" |
| row owner set to desktools-v2/98 | TestCLIInventory FAIL: "orphan owner desktools-v2/98: no brief file carries it" |
| dropped desktools-v2/40 from /17 depends | TestCLIInventory FAIL: "final gate desktools-v2/17 does not depend on desktools-v2/40" |
| env lookup in bind.go changed to treat empty as unset | TestCLIConfigFlow FAIL (precedence empty-env-beats-config; unset-vs-empty) |

**DoD checks:** contract doc covers option forms, dash-leading values, double-dash, unknown flags, precedence matrix and banned list; changelog fragment present; every child brief /18–/54 carries Windows, empty-vs-unset, precedence and effect-free-help rows plus a CLI_OWNER TestCLIOwnerMigrated completion row; /17 depends on /16 and /18–/54 at wave 5; CI runs the inventory test on every push (tools/desk go test with no path filter). Invented scope: none (the deskkit citrigger_test edit maps to Task 7's cross-module reader entry).

**Risk-bearing values** (enumerated from the diff, ranked by irreversibility; dependency pins and the 15-minute owner-test timeout are reversible and ranked last):

- RISK-VALUE: DERIVED — final_gate = "desktools-v2/17" and reference = "desktools-v2/16" @ cli-migration.json:5-6 — spec section 9 Rollout: /16 is the reference every child depends on, /17 must depend on all children and may not complete with pending rows; mutation proves the dependency is enforced.
- RISK-VALUE: DERIVED — MaxSimple = 5, complex row alone @ clicontract validate.go:17 — spec section 9: "at most five simple binaries or one complex CLI"; TestCLIRoutingMutations budget cases fail at 6 and at complex+simple.
- RISK-VALUE: DERIVED — Secret binding may not declare a flag @ cli bind.go:100 — spec section 9 bans "credential values on argv"; the panic makes the ban structural.
- RISK-VALUE: DERIVED — default source order Flag > Env > Config > Default with per-key override @ cli bind.go:141 — spec section 9 "preserve the existing per-key precedence"; contract section 2 Order row; per-key override exists for file-only authorities (cellctl cell.env), so the generic order is not imposed.
- RISK-VALUE: DERIVED — ExitUsage = 2 @ cli cli.go:17 — preserves Go's flag package parse-error exit (ExitOnError exits 2), the deployed tools' existing usage-error code.
- RISK-VALUE: DERIVED — viper KeyDelimiter "::" @ cli bind.go:262 with keyRE ^[a-z][a-z0-9-]*$ @ bind.go:87 — keyRE forbids ':' so no declared key can be split into a nested path; the default "." delimiter would not be safe for dotted names.
- RISK-VALUE: NAMED, NOT DERIVED — brief-20 gate "model" with all-"no" risk answers @ brief-20 line 12 while its deskboard owner path matches a security-path trigger (lint [risk-files-crossread], new in #2147) — whether deskboard's trust-filter reads keep the risk answers correct needs a reviewer/human call; it does not affect /15's own Verify rows.
- Dependency pins cobra v1.10.2 / viper v1.20.1 / pflag v1.0.9 / cast v1.7.1 @ tools/desk go.mod:10-13 — reversible, ranked last.

Risk-bearing Verify-row count: 0 (risk answers all "no"; rows check structure), so isolated sub-verifications were not required; the risk-bearing trigger fires fail-safe on the deskkit path.

**Observations (not DoD failures):**

- O1 (row 5, environment, pre-existing): check-floor went red twice for reasons outside #2147. (a) The first direct run hit the 60s deadline in TestReg786FleetHardening under host load ~42, with all assertions ok; that test alone then passed in 42.06s and the full rerun passed. (b) Under the cell shim (the statusgen on PATH is a cell shim that exports CELLCTL_GH_AMBIENT and swaps HOME), the witness run failed TestReg1145ShimCredential (gen-shims-gh-token.test.sh: 2 FAILED: "verb's gh subprocess carried the ambient token resolved under the REAL home", and the nested equivalent). I reproduced it outside verifyrun with only CELLCTL_GH_AMBIENT set to a dummy value; plain env passes. #2147 touches neither tools/cellctl nor the regression package. This is a test-hermeticity defect: the shim test inherits an ambient CELLCTL_GH_AMBIENT. The witness below was produced with the real statusgen binary (v1.0.31, the same one the shim execs) and that variable unset. Suggested follow-up for the desk: the test should unset CELLCTL_GH_AMBIENT in its fixture env.
- O2: lint NOTICE [gotest-run-vacuous] x161 on child briefs is a false positive, because those rows assert a PASS count with grep -c. There are also 2 ordering-gate prose notices (/29 line 53, /54 line 76).
- O3: clicontract discovery walks the working-tree filesystem rather than git-tracked files. That is the stricter direction: untracked mains also fail the inventory.
- O4: row 6 is vacuous at merged main: the base resolves to HEAD and the command finds no brief diff. Its execution witness was therefore taken at #2147's PR head 13ae74e8e3bd (see row 6 and the note under the witness table). Row 4 is not vacuous: when the base resolves to HEAD, lint falls back to a full-strength run, which reported 0 problems. The supplementary run of row 4 against 4aac3271b reported 0 diff-introduced problems.

**verifyrun --check summary:** brief-15: 6 pass, 0 fail, 0 could-not-run/missing (of 6 Verify rows)

**Execution witness (statusgen verifyrun v1.0.31):**

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `cd tools/desk && go test -count=1 -v ./internal/cli` | pass exit=0 | sha256:f2ea507c7f39 | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 2 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIInventory$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIInventory " "$f"` | pass exit=0 | sha256:9fb67eed0abd | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 3 | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIRoutingMutations$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIRoutingMutations " "$f"` | pass exit=0 | sha256:207482de067a | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 4 | `statusgen --root . --lint --diff-base refs/remotes/origin/main` | pass exit=0 | sha256:7f69e778fc84 | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 5 | `bash tools/desk/internal/regression/check-floor.sh` | pass exit=0 | sha256:990299e2303e | 2026-10-04 | assay-verifier-app[bot] @ 5f5072d89b11+dirty (on-behalf-of human:ian) (forge-identity) |
| 6 | `statusgen --root . --consumers` | pass exit=0 | sha256:1c4a643cffeb | 2026-10-04 | assay-verifier-app[bot] @ 13ae74e8e3bd (on-behalf-of human:ian) (forge-identity) |

Rows 1–5 were witnessed at merged main 5f5072d89b11. Row 6 was witnessed by `statusgen verifyrun --dry-run` at #2147's PR head 13ae74e8e3bd in a clean throwaway clone, because the Verify row has no fixed base and is vacuous at merged main. Its line above is copied from that run.

## Review
Gate: model. Check full inventory coverage, bounded child scope and unchanged domain ownership.
