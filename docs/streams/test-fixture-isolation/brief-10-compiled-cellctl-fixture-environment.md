---
brief: assay:assay:test-fixture-isolation:10
id: 73eb770a-08ee-45ff-85ea-eb132e746917
version: 2
title: Compiled cellctl and shim fixture environment
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

# Brief 10 — Compiled cellctl and shim fixture environment

## Context

files: tools/desk/cmd/cellctl/hotfix_test.go, tools/desk/cmd/cellctl/policy_binary_test.go, tools/desk/cmd/cellctl/cli_nonhelp_parity_test.go, tools/desk/cmd/cellctl/compiled_fixture_env_test.go (new), changelog/cellctl-compiled-fixture-environment.md

facts:
- On unchanged integrated72ca source, both CLI parity cases pass with ordinary readonly Go flags in plain and exact admitted profiles, but fail with caller buildvcs=false even in a plain profile. cellctlBinary builds with a nil child environment; the assertions correctly require the intended native build metadata.
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

1. Use the actual09 child-only constructor for the compiler environment. Capture approved Go/Git and prepared Go paths before HOME changes; explicitly declare offline module/toolchain and build-metadata settings. Avoid process mutation in the sync.Once builder and retain the binary for its existing package lifetime.
2. Preserve existing version/golden/output assertions. Do not remove the hostile build flag from the CI parent or weaken expected dev-build metadata. The fixture compiler must own its inputs.
3. Add a real compiled-consumer control under hostile parent GOFLAGS, Git markers/selectors and credentials; prove parent/victims unchanged and the binary preserves its expected version/output contract. Run full cellctl suites and keep unrelated custody/origin controls intact.
4. Repair the ACTUAL expired-shim fixture in cmd/cellctl/hotfix_test.go. Supply its existing typed CELLCTL_GH_AMBIENT synthetic input and an owned child environment so a caller gh cannot answer the auth-token fallback. Keep production template/auth/expiry behavior and no-token rejection controls unchanged. All worker full runs with the old fixture use a private HOME and gh decoy, never a real stored login. The final full-run decoy post-check must remain empty.

## Verify

Run from repository root with prepared offline Go dependencies and synthetic/local fixtures.

| # | Class | Command | Expect |
|---|---|---|---|
|1|check:ci|`cd tools/desk && GOWORK=off go test -count=1 ./cmd/cellctl`|exit0; full package executes|
|2|check:ci +mutation|`o=$(mktemp) && (cd tools/desk && GOWORK=off go test -v -count=1 -run '^TestCompiledFixtureParentEnvironment$' ./cmd/cellctl) > "$o" 2>&1 && grep -F -- '--- PASS: TestCompiledFixtureParentEnvironment' "$o"`|exit0; real compiled consumer ignores parent build controls without changing its assertions or parent state|
|3|check:ci|`cd tools/desk && GOWORK=off go test -v -count=1 -run 'TestHotfixShim' ./cmd/cellctl`|exit0; existing shim cases execute with fixture-owned synthetic inputs and no parent forge CLI invocation|
|4|check:ci|`cd tools/desk && GOWORK=off go vet ./cmd/cellctl`|exit0|

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate:model. Four risks are no: test-support and fixture-only changes. No production auth, metadata/expiry assertions, protected tables, operational identity or activation changes. Reviews and independent post-merge qualification stay with their desks.
