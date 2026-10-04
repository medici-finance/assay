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

## Review
Gate: model. Check full inventory coverage, bounded child scope and unchanged domain ownership.
