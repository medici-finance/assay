---
brief: assay:assay:ci-tooling:06
title: "Relocate an existing public CI cohort"
why: "The public tree already contains reusable checkers. Migrating a bounded cohort prevents a second copy being imported from an adopter and validates the pattern beyond the pilot."
wave: 6
depends: ["ci-tooling/05"]
unblocks: ["ci-tooling/07"]
effort: "L"
gate: "human"
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
issues: []
schema: "brief-v2"
outcome: none
version: 1
id: "0e25d867-24b0-44e2-a8a4-395b17fcd501"
authored: "2026-10-13 by architecture authoring session"
sources: ["docs/streams/ci-tooling/spec.md", "docs/proposals/non-ci-architecture/25-ci-tooling.md", "docs/streams/desktools-v2/brief-49-migrate-the-harness-and-version-guard-clis-to-cobra-and-viper.md (migrates tools/harnesslint in place; lands first)", ".github/workflows/ci.yml (live suite selection by module path and the real-tree harnesslint job)"]
exec-tier: "strong"
exec-tier-why: "Shared contracts, release paths and independent consumers require cross-component reasoning."
domain: "complicated"
consumers: ["tools/claimguard/: follow-up ci-tooling/06", "tools/harnesslint/: follow-up ci-tooling/06", "tools/plugindrift/: follow-up ci-tooling/06", "ci/cmd/: follow-up ci-tooling/06", "ci/checks/: follow-up ci-tooling/06", "ci/tests/cohort_parity.py: follow-up ci-tooling/06", "ci/tests/live_coverage.py: follow-up ci-tooling/06", "ci/migration/inventory.json: follow-up ci-tooling/06", "Makefile: follow-up ci-tooling/06", "scripts/build-windows.ps1: follow-up ci-tooling/06", "ci/staged-workflows/ci.yml: follow-up ci-tooling/06", "ci/staged-workflows/release.yml: follow-up ci-tooling/06"]
gate-why: "This changes publication, workflow execution or private-data handling boundaries; review the concrete pins, scope and negative controls before activation."
decision-trigger: "start"
---

# Brief 06 — Relocate an existing public CI cohort

## Context

files: `tools/claimguard/`, `tools/harnesslint/`, `tools/plugindrift/`, `ci/cmd/` (planned), `ci/checks/` (planned), `ci/tests/cohort_parity.py` (planned), `ci/tests/live_coverage.py` (planned), `.github/workflows/ci.yml` (read, not edited), `ci/migration/inventory.json` (planned), `Makefile`, `scripts/build-windows.ps1`, `ci/staged-workflows/ci.yml` (planned), `ci/staged-workflows/release.yml`, `changelog/ci-tooling-06.md` (planned)

facts:
- Source inspection: 2026-10-13; public Assay base 4696d598e. Refresh inventory and consumer pins at pickup.
- The stream spec defines the two library layers, private namespace and staged/released/adopted distinction.
- This is an authored migration obligation; the named new tests and paths are deliverables, not existing capabilities.
- Live wiring at `a1c0a9c1e`: `.github/workflows/ci.yml` runs `go test ./...` only for module dirs matching `tools/harnesslint` or `tools/plugindrift` (every other module gets build and vet), and its gating three-state `harnesslint` job runs from `tools/harnesslint` over the real skill tree. A module under `ci/` matches neither, so moving the cohort without a promoted workflow would leave its suites dark or the job red.
- desktools-v2/49 (`todo`) migrates the `tools/harnesslint` CLI in place. It lands first; this brief relocates the migrated tree.

layering: Keep domain checks explicit-input and shared mechanisms narrow; process/provider and CI presentation adapters stay outside pure evaluators. Task and Verify below exercise the complete consumer flow.

design-fit:
  owner: CI tool family and the existing domain owners named in the spec
  contract: none — this draft proposes the packaging/check-result seam; existing S-eligibility and S-evidence-coverage remain unchanged
  retires: [duplicated adapters or obsolete paths named in this brief after consumer proof]
  weight: verbs +0, flags +0, refusals +0 intended externally; rule-text lines added for the bounded migration contract
  why-add: a source move alone cannot preserve pin, publication and workflow authority; this contract assigns the retirement proof to its owner

## Read first

- [Stream spec](spec.md) and [dependency map](README.md).
- Public Assay `docs/contracts.md` and `docs/library-first.md`; do not introduce a rival semantic owner.

## Human decision

The proposed migration changes a boundary used to run checks or keep private material local.
Approve the reviewed scope only after the implementation identifies actual versions,
consumer coverage and negative-control results. Approval does not expand credentials.

Options:
1. **Approve the qualified cutover** — retain current authority and use the tested rollback combination.
2. **Hold the cutover** — keep the supported implementation until the missing proof is supplied.

Default if no answer: none — activation blocks until answered.

## Ground rules

- Author staged workflows only; no workflow activation, credential expansion or live infrastructure probes.
- Preserve historical Evidence and independent controls. Stop at implementation; do not assert verified/done.
- Missing publication, release or consumer evidence is could-not-check, never permission to delete source.

## Task

1. Relocate only the existing public cohort: claimguard (unless brief 05 already moved it as the fallback pilot), plugindrift, and harnesslint once desktools-v2/49 is `done` (if it is not, harnesslint stays routed to a later bounded brief and this brief proceeds without it). Map each old module/import and command to its new owner; retain a supported compatibility module only with a named external consumer and expiry.
2. Adopt shared mechanisms only where behavior matches. Preserve each tool's domain-specific semantics, side-effect boundaries and declared exclusions. Keep other inventory rows explicitly routed; this cohort is not an all-tools rewrite.
3. Update build/release roots, workflow-change candidates (through the landing path in force at pickup), scripts, tests and documentation together. Cross-module readers must trigger on both their code and every external fixture/policy path they read. Each old `tools/<x>` tree stays authoritative, unchanged in behavior and selected by the live `.github/workflows/` legs (unit suite and real-tree job) until a promoted workflow is observed running the relocated suite; the new location is a parity-checked copy until then. Implement live_coverage.py over the inventory and the live workflow files: it fails when any inventoried control's suite or real-tree job is not selected at its authoritative path.
4. Compare old/new builds with recorded compatible toolchains and synthetic inputs, including unavailable upstream observations and malformed metadata. Do not delete any old tree here: retirement is brief 07's act, only after the promoted workflow is observed running the relocated suite and real-tree job, every declared caller is updated and the replacement release is qualified.

## Verify

| # | Command | Expect | Class |
|---|---------|--------|-------|
| 1 | `python3 ci/tests/moved_suites.py --brief ci-tooling/06` | exit 0; every cohort package this brief relocates exists under `ci/` and `go test -json` (GOWORK=off) reports at least the recorded old-tree test count; an empty set is refused. Fail-first: the same command at brief 05's merged head exits non-zero (cohort packages absent), and that red output is quoted in Evidence | check +mutation |
| 2 | `python3 ci/tests/cohort_parity.py` | exit 0; COHORT PARITY PASS; missing fixture and missing trigger controls fail | check +flow +dereference |
| 3 | `python3 ci/tests/live_coverage.py` | exit 0; LIVE COVERAGE PASS; the live `.github/workflows/` wiring selects every inventoried cohort control's unit suite and real-tree job at its authoritative (old) path | check +neighbour |
| 4 | `python3 ci/tests/live_coverage.py --assume-relocated` | exit non-zero (fail-first): with the cohort marked `ci/`-located and the current live `ci.yml` unchanged, the check names the unselected harnesslint and plugindrift suites and the harnesslint real-tree job; that red output is quoted in Evidence | check +mutation |
| 5 | `statusgen --root . --consumers` | exit 0; consumer routing corroborated against the implementation diff | check |

## Evidence

Implementation and independent verification have not run. Append observed commands,
exit codes, outputs or hashes, date and runner here; never infer activation from staging.

## Review

Gate: human. The pre-mortem is a green local tool with a stale or unsafe consumer.
Rows 1 and 4 are fail-first: row 1 cannot pass on brief 03's module alone, and row 4 shows
the live-coverage check red against today's `ci.yml` for a relocated cohort. Row 2 is the
parity and negative-control row; row 3 binds live coverage; row 5 covers consumer routing.
The reviewer confirms no old tree was deleted and the live legs still select it. Publication adequacy and retained authority also require review.
