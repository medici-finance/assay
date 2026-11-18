---
brief: assay:assay:test-fixture-isolation:04
id: 7eec7169-7e1b-492e-a10c-6584ece39c71
version: 2
title: Hostile environment CI coverage
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 3
depends: ["test-fixture-isolation/02", "test-fixture-isolation/03", "test-fixture-isolation/05", "test-fixture-isolation/06", "test-fixture-isolation/07", "test-fixture-isolation/08"]
unblocks: []
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
outcome: none
authored: "2026-11-17 by worker-desk, design deliverable for issue #6300"
exec-tier: strong
exec-tier-why: "Cross-module fixture semantics and negative controls must agree without weakening production authority."
domain: complicated
sources:
  - "docs/streams/test-fixture-isolation/README.md — adopted class outcomes and shared interface contract"
  - "https://github.com/medici-finance/assay/issues/6300 — test environment class and design-owed deliverable"
  - "https://github.com/medici-finance/assay/issues/2715 — admitted Git fixture failures"
  - "freshness-checked 2026-11-17 @ dabede22b0be3abd3320a0a7016132a917192e86 (origin/main)"
consumers:
  - ".github/workflows/ci.yml: follow-up test-fixture-isolation/04"
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up test-fixture-isolation/04"
  - "docs/test-policy.md: follow-up test-fixture-isolation/04"
---

# Brief 04 — Hostile environment CI coverage

## Context

files: .github/workflows/ci.yml, tools/test-fixture-environment.sh (new), tools/test-fixture-environment_test.sh (new), tools/desk/internal/deskkit/citrigger_test.go, tools/desk/internal/deskkit/fixture_environment_ci_test.go (new), docs/test-policy.md, changelog/fixture-hostile-ci.md

facts:
- ci.yml runs Go build/test without Docker on medici-builder-public; the fixture lane requires no service or container daemon.
- Existing required build-test aggregates its shard dependencies; a new optional green job alone would not satisfy #6300's required-lane criterion.
- Tests reading workflow, runner or another module need an explicit cross-module registry row and matching workflow trigger coverage.

Read first: README.md acceptance and limits, briefs 01–03, .github/workflows/ci.yml, tools/desk/internal/deskkit/citrigger_test.go, docs/test-policy.md.

single-point-of-failure: a correct lane that is not required can be ignored; structural aggregation tests plant an omitted dependency and a failure/skipped-lane case to prove the existing required aggregate fails closed.

design-fit:
  owner: existing CI test aggregate and cross-module trigger registry
  contract: none — CI execution plumbing; existing docs/test-policy.md and workflow aggregate remain the acceptance owners
  retires: reliance on a friendly runner environment to catch fixture isolation failures
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0; one CI runner and aggregate dependency
  why-add: helper tests cannot detect a future caller that never adopts the helper

## Ground rules

Use an owned isolated worktree and synthetic fixtures. Branch publication follows normal worker guards; no main push, merge, workflow dispatch, review, Evidence or lifecycle advance is authorized. Stop at an implementation handoff. Production security controls and protected acceptance tables are out of scope.

layering: a flat local test runner supplies hostile inputs to real module suites; the existing CI aggregate owns required-success semantics. Task 3 and Verify row 3 check aggregate failure independently of runner behavior.

## Task

1. Add an offline-capable runner that creates disposable Git/config victims and caller decoy binaries, sets admitted-style Git variables and explicitly withholds forge credentials, then runs full statusgen and tools/desk module suites. Preserve prepared Go cache paths; no downloaded fixture tools or live endpoints. Print target module/test invocation, never credential values. Exit nonzero on any failed/absent required execution, and restore/remove only owned scratch.
2. Exercise both absence and synthetic-presence credential scenarios where the test contract requires them. Demonstrate a deliberately leaking fixture, installed-binary selection and global-config escape are detected by controls; the runner must not erase hostile variables before starting suites.
3. Wire the lane through the already-required build-test aggregate, including failure, cancellation and skipped dependency cases. Do not change required-check administration or weaken any existing shard/control to make the lane pass. If those settings cannot be established with source wiring, retain the operator act as an explicit unresolved closure condition.
4. Register all cross-module readers and widen affected workflow paths. Document the repeatable local command and the distinction between worker CI results and independent post-merge qualification. Route additional module-suite failures as findings; do not omit failing packages or relabel a partial lane as full coverage.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `bash tools/test-fixture-environment_test.sh` | exit 0; deliberately nonisolated Git/PATH/credential/global-config controls are detected, cleanup preserves victims |
| 2 | check:ci | `bash tools/test-fixture-environment.sh` | exit 0; full statusgen and desk suites executed in hostile parent, no selected-package substitute |
| 3 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -run '^TestFixtureEnvironmentCIAggregate$' ./internal/deskkit` | exit 0; healthy lane required; missing/failed/skipped/cancelled lane cannot yield aggregate success |
| 4 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -run 'TestCI' ./internal/deskkit` | exit 0; cross-module readers registered and workflow paths cover their inputs |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate: model. Four risks are no: source CI/test plumbing only. No branch-protection mutation or workflow dispatch authorized. Review must inspect fail-closed aggregate wiring, platform applicability and full-suite execution. Production admitted-row consumers and independent verifier lifecycle remain outside this fixture repair.
