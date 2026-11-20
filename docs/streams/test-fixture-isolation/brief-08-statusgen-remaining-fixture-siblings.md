---
brief: assay:assay:test-fixture-isolation:08
id: 2a56be74-dc7a-4012-be20-5ec37808fc55
version: 3
title: Statusgen remaining fixture siblings
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 2
depends: ["test-fixture-isolation/02", "test-fixture-isolation/09"]
unblocks: ["test-fixture-isolation/04"]
effort: L
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

# Brief08 — Statusgen remaining fixture siblings

## Context

files: statusgen/*_test.go, changelog/statusgen-remaining-fixture-isolation.md

facts:
- The worker full hostile module run at1e889512f reached additional inherited-environment failures beyond the transition/coverage/flow/autonomy groups: admitted-row credential checks, briefinfo fail-first, claim-decay forge fixtures, derived/container history, decision-ruling custody and evidence-repair fixtures.
- The root package exhausted its25minute diagnostic bound while processing RepairApprovalScope; this is a partial census, not proof of a deadlock or of unvisited tests passing.
- Existing02 scoped hostile-parent, intentional authority refusal, quiet-template and focused race controls pass. Existing allocation budgets also pass unchanged without race instrumentation; do not weaken their thresholds.
- This delivery consumes the actual02 implementation and its fixture wrapper rather than duplicating the shared environment contract. Fresh source d7a10023 changes none of these directories relative to the examined initial main.

Read first: README.md Shared interface contract, briefs01/02, statusgen/fixture_isolation_test.go from the actual02 commit, docs/test-policy.md, named failing tests and their production callers (read only).

single-point-of-failure: broad fixture setup could accidentally remove the malicious input tested by an authority assertion; isolate at the top-level boundary, then explicitly plant negative controls. Independent hostile-parent subprocesses and victim snapshots detect incomplete fixture adoption.

layering: retain gitcore/testenv as the only explicit-environment constructor, the existing statusgen wrapper for synthetic roster/caches, and unchanged production authority/credential controls. No new production policy.

design-fit:
  owner: existing statusgen fixture entry points and shared02 wrapper
  contract: shared test-only Install/Options/Fixture/Env API in README
  retires: remaining ambient fixture Git/config/credential/PATH inheritance exposed by full module execution
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0
  why-add: the full-module acceptance surface exceeds the original bounded transition-chain repair

Sizing: L is justified by the shared test-package environment owner and its single synthetic roster/cache wrapper. The package-wide migration consists primarily of scoped top-level entry calls plus independent hostile-parent controls; splitting it alphabetically would leave the same environment owner and full-suite acceptance distributed across disconnected fixture variants. No production implementation is added.

## Task

1. Reproduce each newly named family under plain/admitted profiles with disposable canonical HOME/global config and prepared offline caches. Preserve exact source and compiled-binary attribution. Route every unvisited or timed-out family truthfully; do not infer that a partial alphabetical census is complete.
2. Reuse the actual02 scoped wrapper at the relevant top-level fixture boundary. Sweep sibling direct/native/in-process repository and fake-forge consumers, including claim-decay, history and evidence-repair families. Declare executable and synthetic credential inputs deliberately.
3. Retain explicit production input-rejection tests. Install before deliberate hostile plants, never clear those inside nested fixture constructors; preserve replacement-ref, custody, origin and stored-login assertions. No production admitted-row or --consumers behavior changes.
4. Add TestFixtureSiblingParentIsolation08 using a genuinely failing inherited control and actual previously failing consumer families. Assert the independent parent's config/repository remains unchanged and intentionally injected authority input remains rejected after setup.
5. Run focused meaningful race controls and full non-race module suites. Do not run allocation-budget assertions under race and call their instrumentation cost a regression, or relax the existing non-race limits. New source failures are worker work; environmental timeouts are reported with their measured scope, never turned into a green result.

The shared09 lifecycle fix is the required producer for final allocation acceptance: Unix environment tombstones caused os.Environ cost to grow after repeated fixture lifetimes. Retire the temporary wrapper-only compaction after consuming the real shared fix; retain the1,000-lifetime actual-export regression and unchanged allocation limits.

## Verify

Run from the repository root with prepared offline Go dependencies and synthetic fixtures.

| # | Class | Command | Expect |
|---|---|---|---|
|1|check:ci|`cd statusgen && GOWORK=off go test -count=1 ./...`|exit0; all module packages execute without omission|
|2|check:ci|`cd statusgen && GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=grep.patternType GIT_CONFIG_VALUE_0=default GIT_CONFIG_KEY_1=grep.extendedRegexp GIT_CONFIG_VALUE_1=false GIT_NO_REPLACE_OBJECTS=1 GOWORK=off go test -count=1 ./...`|exit0; full non-race module suite executes under admitted-style inputs|
|3|check:ci +mutation|`o=$(mktemp) && (cd statusgen && GOWORK=off go test -v -count=1 -run '^TestFixtureSiblingParentIsolation08$' .) > "$o" 2>&1 && grep -F -- '--- PASS: TestFixtureSiblingParentIsolation08' "$o"`|exit0; named consumer controls execute, victims unchanged, intentional authority rejection retained|
|4|check:ci|`cd statusgen && GOWORK=off go vet ./...`|exit0|

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate:model. Four risks are no: fixture/test setup only, no production authority or credential changes. Reviews and post-merge verification remain separate. No readiness, merge, Evidence or operational activation is authorized.
