---
brief: assay:assay:test-fixture-isolation:09
id: 93dbc6c1-8131-4864-9522-e2b7b2f64a29
version: 2
title: Nonmutating child fixture environment
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 1
depends: ["test-fixture-isolation/01"]
unblocks: ["test-fixture-isolation/10", "test-fixture-isolation/11", "test-fixture-isolation/08", "test-fixture-isolation/04"]
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
  - "tools/desk/cmd/cellctl/policy_binary_test.go: follow-up test-fixture-isolation/10"
  - "tools/desk/cmd/cellctl/hotfix_test.go: follow-up test-fixture-isolation/10"
  - "tools/desk/internal/deskkit/originslug_test.go: follow-up test-fixture-isolation/11"
---

# Brief 09 — Nonmutating child fixture environment

## Context

files: gitcore/testenv/environment.go, gitcore/testenv/environment_test.go, changelog/child-fixture-environment.md

facts:
- Install owns process scope. Compiled CLI and generated-shim builders need the same explicit child environment without resetting deliberate parent test state. Add New in the same test-support owner, retaining Install as its scoped process adapter.
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

1. Add `func New(t testing.TB, opts Options) *Fixture` to the existing owner. It creates the same owned directories/tools/frozen environment but never changes process variables; Install delegates construction to New and retains its existing restoration semantics.
2. Retain all executable validation, reserved HOME/PATH ownership, Windows launch handling, Git quiet templates, explicit overlays and immutable Env behavior. No new production API or dependency.
3. Prove New leaves parent environment and independently constructed config/repository victims unchanged, supports parallel child fixtures, and can launch a real child with explicit inputs. Re-run existing Install restoration and intentional-hostility controls.
4. Do not import test support into production tools. Consumers10/11 use the real constructor commit, not an invented API. Nested parent fixture state remains untouched.
5. Keep Unix Install/cleanup lifetimes from accumulating environment-cache tombstones. The repeated-lifetime cost must stay bounded while every live key/value/presence remains identical; retain testing's concurrency guard before any manual compaction. New itself never mutates or compacts parent state. This shared lifecycle fix replaces the temporary08 wrapper workaround without changing production allocation caps.

## Verify

Run from repository root with prepared offline Go dependencies and synthetic/local fixtures.

| # | Class | Command | Expect |
|---|---|---|---|
|1|check:ci +mutation|`cd gitcore && GOWORK=off go test -count=1 -race -run 'TestNewFixtureEnvironment\|TestFixtureEnvironment\|TestFixtureVictimUnchanged' ./testenv`|exit0; parent unchanged, parallel child environments independent, existing restoration controls retained|
|2|check:ci|`cd gitcore && GOWORK=off go test -count=1 ./testenv`|exit0|
|3|check:ci|`cd gitcore && GOWORK=off go vet ./testenv`|exit0|

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate:model. Four risks are no: test-support and fixture-only changes. No production auth, metadata/expiry assertions, protected tables, operational identity or activation changes. Reviews and independent post-merge qualification stay with their desks.
