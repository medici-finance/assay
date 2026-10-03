---
brief: "assay:assay:desktools-v2:17"
title: "Enforce complete Cobra and Viper adoption across the tool suite"
why: >-
  A cellctl-only migration would leave inconsistent behavior in the rest of the suite. Close
  the rollout only when every maintained entrypoint is migrated or deliberately retired and
  executable compatibility checks prevent new custom parsers from returning.
wave: 3
depends: ["desktools-v2/16"]
unblocks: []
effort: "L"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: [2111]
schema: "brief-v2"
outcome: none
version: 1
id: "4dcb4036-a55c-4a56-af87-2efe09225307"
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
  - "tools/desk/internal/clicontract: follow-up desktools-v2/17 (completion and behavior conformance)"
  - "docs/streams/desktools-v2/cli-migration.json and cli-contract.md: follow-up desktools-v2/17 (complete coverage and generated documentation)"
  - ".github/workflows/ci.yml and owning module test jobs: follow-up desktools-v2/17 (wire the actual completion gate through the existing workflow-change process)"
  - "all maintained command entrypoints: follow-up desktools-v2/15 (authors their bounded implementation owners; /17 does not port the suite itself)"
---

# Brief 17 — Enforce complete Cobra and Viper adoption across the tool suite

## Context

files:
- `tools/desk/internal/clicontract/` — coverage, behavior conformance and negative controls.
- `docs/streams/desktools-v2/cli-migration.json`, `cli-contract.md`, `README.md`.
- Owning module test registries and `.github/workflows/ci.yml` (or the project's staged workflow-promotion artifact when direct workflow scope is unavailable).
- `changelog/cli-suite-conformance.md` (planned).

facts:
- /15 must land exhaustive source discovery plus a concrete migration brief for every remaining entrypoint and add those children to THIS brief's depends before /15 completes. The current /16 edge is the pilot, not permission to close a partial rollout.
- `tools/desk/cmd/*` alone misses statusgen, qualgen and module-local operator/maintenance CLIs. The registry must be checked against independent discovery and release/build declarations.
- Existing tools use different output/exit contracts; uniform libraries do not authorize replacing those contracts with Cobra defaults.

risk rationale: workflow scope is additive offline test wiring only; no permission, credential, environment or existing gate change is authorized. If implementation requires one, re-derive risk and route that change before proceeding.

layering: independent black-box conformance executes built commands with fixtures; source coverage detects missing owners and remaining manual entry parsing. Production authorization remains outside this test harness.
design-fit:
  owner: CLI conformance in the desk module; runtime semantics remain in each command's module
  contract: none — cli-contract.md and the migration registry supplied by /15
  retires: [temporary pending migration entries, copied help fixtures replaced by generated references]
  weight: verbs +0; flags +0; refusals +0 in production; test assertions positive
  why-add: strengthen the existing regression/test lane with complete CLI coverage rather than add another runtime gate

## Ground rules

Feature branch and draft PR only. No workflow dispatch, live infrastructure contact,
ready flip or merge. Preserve the existing domain checks, custody decisions and exit
codes; a CLI library is not a new authority source. Independent verification owns Evidence
and lifecycle advancement. Proposed tests and support files below are deliverables, not
claims that they already exist.

## Task

1. Refuse completion until every maintained CLI discovered from tracked source/build/release declarations is either migrated by its named child brief or retired by a merged change that also removes its consumers. No pending row, arbitrary exclusion or source-only import counts as adopted. All children authored by /15 must be actual dependency edges here; recompute waves and README critical path when those edges land. Do not migrate dozens of commands in this final brief.
2. Run a shared black-box matrix against every migrated command tree: root and nested help/version without config/auth/effects, conventional flag/value/-- forms, required args and invalid inputs, approved env/config/default precedence, explicit-empty behavior, machine output and stable execution exits. Each configuration-capable tool must demonstrate Viper's resolved value reaching its handler. A tool without config demonstrates declared flags/defaults through its Viper instance, without invented config surfaces. Test hidden entrypoints' parsing separately while preserving their documented machine protocol.
3. Add independent source-level checks, with narrow documented compatibility adapters, detecting reintroduced manual os.Args dispatch, flag.FlagSet production parsing, duplicated handwritten help or global Viper use. Do not confuse legitimate raw argv forwarding AFTER `--`, test fixtures, shell data-format decoders or unrelated libraries' flag use with a second parser. Compile the supported Go targets and run native Windows behavior cases through the existing CI lane; a cross-build is not a native behavior pass.
4. Exercise negative controls: a planted maintained command omitted from the registry, a false migrated row whose handler ignores Viper, a help handler that reads config or triggers an effect, a new manual parser and a removed config-source restriction must each fail the corresponding check. Existing lower-level authorization/custody regression tests must stay independent of this harness.
5. Wire the coverage and conformance checks into PR CI with path triggers covering every module/registry they read. Use the existing workflow-change process. If a workflow patch is staged rather than live, record the exact promotion owner and evidence gap; this brief and the CLI completion claim remain unfinished until the actual trigger/gate is independently evidenced. Never weaken current gates to green this one.
6. Update generated CLI references and the README with measured completion, not a promise. Keep package/module boundaries: statusgen still reaches forge reads through deskread; it does not import deskkit merely to share CLI plumbing.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +dereference | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIComplete$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIComplete " "$f"` | exit 0 with TestCLIComplete PASS; exact discovered coverage, zero pending/unowned rows, every dependency satisfied by implementation/evidence rather than an authoring-only PR |
| 2 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIBinaries$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIBinaries " "$f"` | exit 0 with TestCLIBinaries PASS; every registered command/case built and executed, nonempty per-tool case counts, zero credential/config reads or effects on help/version |
| 3 | check +mutation | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLIConformanceMutations$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLIConformanceMutations " "$f"` | exit 0 with TestCLIConformanceMutations PASS; each planted omission, imports-only adoption, effectful help, custom parser and source-precedence regression is detected |
| 4 | check +flow | `f=$(mktemp) && cd tools/desk && go test -count=1 -v ./internal/clicontract -run '^TestCLICIGates$' > "$f" 2>&1 && grep -F -e "--- PASS: TestCLICIGates " "$f"` | exit 0 with TestCLICIGates PASS; the actual workflow and cross-module trigger registry include the checks; a staged-only patch fails this row |
| 5 | check | `bash tools/desk/internal/regression/check-floor.sh` | exit 0; inherited behavior floor preserved |
| 6 | check | `statusgen --root . --consumers` | exit 0; no declared consumer routing disproved by the change |

## Pre-mortem

A suite can look complete by excluding forgotten tools: row 1 discovers the set independently.
Imports can hide a retained custom parser or unused Viper: rows 2–3 exercise values and plant defects.
A workflow exists but never fires on another module: row 4 checks the trigger paths as well as the command.
Live native Windows evidence is a review requirement; missing runner access is could-not-check,
never substituted by cross-compilation or a Linux path fixture.

## Evidence
<!-- Independent verifier records actual command, exit, output, date and runner. -->

## Review
Gate: model. Reject completion based only on cellctl, imports or a staged CI patch.
