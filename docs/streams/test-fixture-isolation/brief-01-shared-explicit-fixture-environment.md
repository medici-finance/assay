---
brief: assay:assay:test-fixture-isolation:01
id: a6566a59-40e2-479a-84b5-eb9ab6b53d32
version: 1
title: Shared explicit fixture environment
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 0
depends: []
unblocks: ["test-fixture-isolation/02", "test-fixture-isolation/03"]
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
  - "gitcore/testenv/: follow-up test-fixture-isolation/01 (new helper and boundary tests)"
  - ".github/workflows/ci.yml: follow-up test-fixture-isolation/01 (existing gitcore job executes new helper tests)"
  - "tools/desk/internal/deskkit/citrigger_test.go: follow-up test-fixture-isolation/01 (new cross-module helper-reader coverage)"
  - "statusgen/: follow-up test-fixture-isolation/02 (named fixture consumers only)"
  - "tools/desk/cmd/: follow-up test-fixture-isolation/03 (named fixture consumers only)"
  - "tools/desk/internal/regression/fixtureenv.go: follow-up test-fixture-isolation/01 (compatibility rationale; existing wrappers kept deliberately)"
---

# Brief 01 — Shared explicit fixture environment

## Context

files: gitcore/testenv/environment.go (new), gitcore/testenv/environment_test.go (new), gitcore/testenv/imports_test.go (new), tools/desk/internal/regression/fixtureenv.go, tools/desk/internal/regression/gitenv_test.go, .github/workflows/ci.yml, tools/desk/internal/deskkit/citrigger_test.go, changelog/explicit-fixture-environment.md

facts:
- At the pinned source, regression.FixtureEnv and IsolateGit are desk-internal and cannot serve statusgen.
- Both statusgen/go.mod and tools/desk/go.mod already replace the shared gitcore module locally; no new module or dependency release is required.
- gitquiet.TemplateDir(t) supplies the owned no-maintenance template; dropping it revives detached maintenance races.

Read first: README.md Shared interface contract, gitquiet/gitquiet.go, tools/desk/internal/regression/fixtureenv.go, docs/test-policy.md.

single-point-of-failure: a wrongly constructed base could redirect every consumer; independent hostile victim-directory and source-import controls detect that failure outside the construction assertions.

design-fit:
  owner: gitcore/testenv (test support only)
  contract: none — test-support environment ownership; the README specifies the new API and no production semantic-owner rule changes
  retires: duplicate fixture-local environment construction in migrated consumers; existing regression wrappers retained until each caller owns an explicit migration
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0; one test-support package
  why-add: cross-module consumers cannot import desk-internal helpers; gittest's global flag is unrelated baggage

## Ground rules

Use an owned isolated worktree and synthetic fixtures. Branch publication follows normal worker guards; no main push, merge, workflow dispatch, review, Evidence or lifecycle advance is authorized. Stop at an implementation handoff. Production security controls and protected acceptance tables are out of scope.

layering: a flat test-support package owns environment construction and scoped process effects; production tools cannot depend on it. Task 1 and 4 and Verify rows 2–3 check behavior and import direction independently.

## Task

1. Implement the README interface with no production imports. Explicitly handle runtime Windows variables and case-insensitive environment keys. Reject missing/nonabsolute tool mappings; never extend the caller PATH.
2. Keep each fixture's global Git config writable and private. Clear inherited Git selectors, command-priority config and credentials before applying explicit overlays. Restore absence as well as values. Preserve owned Git maintenance templates.
3. Test parent GIT_DIR/WORK_TREE/INDEX_FILE, injected count/key/value and global config against a separately constructed victim repository and config file; both stay byte-unchanged. Prove explicit fixture overlays and later hostile controls still reach in-process and child readers.
4. Add ./testenv to the existing gitcore CI test invocation and register its import-boundary readers of gitcore/statusgen/tools/desk with matching trigger coverage; the new hostile lane remains brief 04. Delegate regression helpers where behavior is equivalent; retain intentionally different wrappers with an explicit reason. Add an import-boundary test that fails when a non-test production file imports testenv, allowing only declared test-support adapters.

## Verify

Tests are planned names to implement, not existing coverage claims. Run from the repository root.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd gitcore && GOWORK=off go test -count=1 -run '^TestFixtureEnvironment$' ./testenv` | exit 0; all environment, restoration and overlay subtests pass |
| 2 | check:ci +mutation | `cd gitcore && GOWORK=off go test -count=1 -run '^TestFixtureVictimUnchanged$' ./testenv` | exit 0; a deliberately unsanitized control reaches the disposable victim while the helper does not; victim content and config unchanged after helper use |
| 3 | check:ci | `cd gitcore && GOWORK=off go test -count=1 -run '^TestFixtureImportBoundary$' ./testenv` | exit 0; production import plant is rejected and actual imports are test-only |
| 4 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -run '^TestFloorExecEnv$' ./internal/regression` | exit 0; compatibility floor retains its intentionally different existing semantics |
| 5 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -run 'TestCI' ./internal/deskkit` | exit 0; gitcore testenv execution and all cross-module reads covered by existing CI |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate: model. All four risks are no: test support only; production admission and credential custody unchanged. Review the real process/child flow and Windows restoration. Do not grant a class closure from helper tests alone.
