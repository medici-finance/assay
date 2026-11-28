---
brief: assay:assay:test-fixture-isolation:11
id: f33fba46-70e2-4110-be74-cf1ae6161f8d
version: 2
title: Deskkit origin fixture environment
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 2
depends: ["test-fixture-isolation/09"]
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
  - "tools/test-fixture-environment.sh: follow-up test-fixture-isolation/04 (joined full-suite caller)"
---

# Brief 11 — Deskkit origin fixture environment

## Context

files: tools/desk/internal/deskkit/originslug_test.go, tools/desk/internal/deskkit/worksource_ext_test.go, tools/desk/internal/deskkit/worksource_test.go, tools/desk/internal/deskkit/fixture_siblings_env_test.go (new), changelog/deskkit-fixture-environment.md

facts:
- The six origin/work-source controls pass plain and fail exact admitted markers on unchanged72ca source. The expired-shim finding actually belongs to cmd/cellctl/hotfix_test.go and is routed to10.
- Fresh integrated census and plain/admitted discriminators are bound to source72ca976d3f1986910bd421b1eab01e74f4357821. Recheck affected current source at pickup; do not redo a delivered fix.

Read first: README.md shared interface, briefs01/04, actual gitcore/testenv owner, named fixture sources and production callers (read only), docs/test-policy.md.

single-point-of-failure: construction-only assertions could miss a child that inherits caller state; actual consumer execution and independent parent/victim/decoy observations check the boundary separately. Deliberate rejected inputs remain rejection controls.

layering: one test-support environment owner; process-scoped Install and child-only New share construction. Production authority, auth, metadata and expiry policies remain unchanged.

design-fit:
  owner: existing gitcore/testenv and named test fixture adapters
  contract: existing README test-support interface plus nonmutating New from09
  retires: accidental caller environment inheritance at the declared fixture seam
  weight: production verbs +0, flags +0, refusals +0, resident rule-text lines +0;09 adds one test-support constructor
  why-add: compiled children cannot reset deliberate parent state; reuse the owner instead of a second per-file environment implementation

## Task

1. Install the existing shared process fixture environment at relevant top-level origin/work-source boundaries. Preserve later deliberate worktree/include/injected-origin refusal inputs; never reset inside nested repository constructors.
2. Keep cmd/cellctl expired-shim fixture work in10; there is no internal/deskkit expired-shim case in this scope. Use the actual09 shared owner and preserve the CI requirement indicator for origin fixtures.
3. Add a parent/victim and decoy control using the actual broken origin consumers, with a failing inherited control and retained intentional authority rejection. Preserve no-token negative cases explicitly.
4. Run the full deskkit package and targeted hostile-profile controls; remaining unrelated failures are reported without assertion weakening or package omission.

## Verify

Run from repository root with prepared offline Go dependencies and synthetic/local fixtures.

| # | Class | Command | Expect |
|---|---|---|---|
|1|check:ci|`cd tools/desk && GOWORK=off go test -count=1 ./internal/deskkit`|exit0; full package executes|
|2|check:ci +mutation|`o=$(mktemp) && (cd tools/desk && GOWORK=off go test -v -count=1 -run '^TestDeskkitFixtureParentEnvironment$' ./internal/deskkit) > "$o" 2>&1 && grep -F -- '--- PASS: TestDeskkitFixtureParentEnvironment' "$o"`|exit0; real origin consumers isolated, decoy/victims protected, intentional refusal preserved|
|3|check:ci|`cd tools/desk && GOWORK=off go vet ./internal/deskkit`|exit0|

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate:model. Four risks are no: test-support and fixture-only changes. No production auth, metadata/expiry assertions, protected tables, operational identity or activation changes. Reviews and independent post-merge qualification stay with their desks.
