---
brief: assay:assay:test-fixture-isolation:07
id: e5644982-4b2f-4dfc-bb6a-1837fbf99ec9
version: 3
title: Dispatch and fleet fixture siblings
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 1
depends: ["test-fixture-isolation/01"]
unblocks: ["test-fixture-isolation/04"]
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
  - "freshness-checked 2026-11-17 @ d7a10023e92ace806ab6d11e670025aa4c6b800a (origin/main)"
consumers:
  - "tools/test-fixture-environment.sh: follow-up test-fixture-isolation/04 (full-suite caller)"
---

# Brief 07 — Dispatch and fleet fixture siblings

## Context

files: tools/desk/cmd/deskdispatch/resumealloc_test.go, tools/desk/cmd/deskdispatch/reworkhome_test.go, tools/desk/cmd/deskdispatch/verifiercrossrepo_test.go, tools/desk/cmd/deskdispatch/*fixture*_test.go, tools/desk/internal/fleetharness/fleetharness_test.go, changelog/dispatch-fleet-fixture-isolation.md

facts:
- The complete worker census at dabede22b records real allocator origin refusals in TestResumeRealAllocator and four TestReworkAdopts/Refuses cases; newRealResume builds real deskwt before the child inherits caller Git settings.
- Three fleet deskreply workflow scenarios reach an authority refusal instead of their expected synthetic remote seam; newFleetEnv and scenario/Git child environments append os.Environ.
- Fresh origin/main d7a10023e92ace806ab6d11e670025aa4c6b800a changes none of these affected source directories. The scoped refusal mechanism remains current.
- A subsequent unchanged-source discriminator confirms TestCrossRepoDispatchExecution: plain passes, admitted fails both public/private claim-root tracking resolution. Its top-level fixture is included; unrelated module timeouts remain outside this delivery until diagnosed.

Read first: README.md Shared interface contract, brief01, the named fixture sources and production origin reader (read only), docs/test-policy.md.

single-point-of-failure: a helper-only green test cannot show a real child used the fixture environment; actual compiled allocator/fleet scenarios and independently constructed hostile-parent victims check that seam. Deliberate rejected input is planted after setup and still fails production admission.

layering: reuse the shared test environment for fixture inputs; retain production origin/stored-login enforcement and the fleet's expected remote seam. No new production wrapper or policy.

design-fit:
  owner: existing allocator and fleet scenario fixture boundaries
  contract: shared test-only Install/Options/Fixture/Env API in this stream README
  retires: ambient Git/config/credential/PATH inheritance in the named real-child fixtures
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0
  why-add: whole-module execution exposed real-child consumers beyond the original decision receipt slice

## Task

1. Reproduce each named real-child failure in plain and admitted-style environments using owned canonical temporary paths, explicit global/system Git files and pre-resolved Go caches. Retain failure attribution to the exact source/binary.
2. Install the shared environment once at the scenario/top-level fixture boundary before compilation and repository creation. Declare required real/synthetic executables, cache variables and synthetic roster state. Never reset deliberate hostile state inside a nested repository constructor.
3. Make compiled children receive the explicit fixture environment and final owned HOME/config paths. Preserve offline cache use; do not enable downloads or invoke real desk Apps to repair a test.
4. Retain all current real allocator/fleet assertions and the credential/authority refusals. Add a hostile parent regression proving the inherited control fails, the migrated real consumer reaches its expected seam, and independent config/repository victims stay unchanged. Replant intentional rejected config after installation and show it still refuses.
5. Run full named packages under the same admitted settings. Route remaining distinct failures by exact source; never omit failing packages from required full-module CI or declare this class closed from this bounded slice.

## Verify

Run from the repository root. Prepared Go dependencies and synthetic/local remotes suffice.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskdispatch ./internal/fleetharness` | exit0; both full packages execute |
| 2 | check:ci | `cd tools/desk && GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=grep.patternType GIT_CONFIG_VALUE_0=default GIT_CONFIG_KEY_1=grep.extendedRegexp GIT_CONFIG_VALUE_1=false GIT_NO_REPLACE_OBJECTS=1 GOWORK=off go test -count=1 ./cmd/deskdispatch ./internal/fleetharness` | exit0; full real-child suites run under admitted-style inputs |
| 3 | check:ci +mutation | `o=$(mktemp) && (cd tools/desk && GOWORK=off go test -v -count=1 -run '^TestFixtureSiblingParentIsolation07$' ./cmd/deskdispatch ./internal/fleetharness) > "$o" 2>&1 && grep -F -- '--- PASS: TestFixtureSiblingParentIsolation07' "$o"` | exit0; named real-consumer control executes, preserves victims and retains intentional refusal |
| 4 | check:ci | `cd tools/desk && GOWORK=off go vet ./cmd/deskdispatch ./internal/fleetharness` | exit0 |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate:model. Four risks are no: test setup and regression controls only. No production authority, token custody, workflow permissions, protected tables, readiness or operational activation changes. Reviewer judges the child-environment flow; independent verifier owns qualification.
