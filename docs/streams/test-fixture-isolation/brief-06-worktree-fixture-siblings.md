---
brief: assay:assay:test-fixture-isolation:06
id: cee8c90d-6a8c-4323-bc9b-4e39dfe39765
version: 2
title: Worktree fixture siblings
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 2
depends: ["test-fixture-isolation/03"]
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

# Brief 06 — Worktree fixture siblings

## Context

files: tools/desk/cmd/deskwt/*_test.go, changelog/worktree-fixture-isolation.md

facts:
- newRepo and withEnv are shared by the worktree activity, admission, adoption, branch and cache-pressure fixtures. They create repositories and read origin authority in-process under inherited command-priority configuration. Maintenance ownership is already bounded in brief03; this sibling sweep consumes that real implementation without reopening its completed source claims.
- The complete hostile desk worker census at source dabede22b records these siblings in the existing class; no production behavior change is proposed.
- A refreshed origin/main at d7a10023e92ace806ab6d11e670025aa4c6b800a changes none of the affected source directories; the reported fixture sites remain current.

Read first: README.md Shared interface contract, briefs01 and03, docs/test-policy.md, the named fixture sources and production authority callers (read only).

single-point-of-failure: fixture setup could erase the hostile input a guard test needs; install at the top-level test boundary and then deliberately plant rejected input. An independently constructed parent-config/repository victim catches fixture writes outside that boundary.

layering: reuse gitcore/testenv for test setup and the unchanged production authority guard for intentional hostile input; no production adapter or policy is added.

design-fit:
  owner: existing command test fixture entry points
  contract: the shared test-only Install/Options/Fixture/Env interface in this stream README
  retires: ambient process configuration and undeclared tool/cache inheritance at these sibling sites
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0; test migration only
  why-add: whole-suite execution exposed consumers beyond the original narrow desk slice

## Task

1. Reproduce the listed family under admitted Git settings in a disposable HOME/global config and preserve resolved Go caches. Inventory each matching fixture site as fixed here, already isolated, or a named follow-up; never omit a failing package from the full hostile lane.
2. Install the shared explicit environment once at each relevant top-level fixture boundary. Declare real and synthetic executables and required synthetic roster/cache variables. Do not reset the environment inside nested repository constructors after a test has planted intentional hostile state.
3. Keep HOME, USERPROFILE, XDG configuration and explicit global Git paths aligned when an existing helper installs its final owned roster. Preserve explicitly resolved GOCACHE/GOMODCACHE/GOPATH/GOROOT before any HOME change; never enable dependency downloads to repair an empty-cache fixture.
4. Retain every production injected-config/stored-login refusal and its assertion. Add a hostile parent regression with a genuinely failing inherited control and a post-install deliberate authority rejection. Prove an independent parent configuration/repository victim stays unchanged.
5. Run the full named command packages under the same admitted Git settings. Record remaining failures truthfully for the class and required CI lane; local worker tests do not supply independent verification, readiness or class closure.

## Verify

Run from the repository root. Prepared dependencies are local; no live forge or cluster is required.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 ./cmd/deskwt` | exit 0; all named packages execute without test omission |
| 2 | check:ci | `cd tools/desk && GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=grep.patternType GIT_CONFIG_VALUE_0=default GIT_CONFIG_KEY_1=grep.extendedRegexp GIT_CONFIG_VALUE_1=false GIT_NO_REPLACE_OBJECTS=1 GOWORK=off go test -count=1 ./cmd/deskwt` | exit 0; full named package suites run under admitted-style configuration |
| 3 | check:ci +mutation | `o=$(mktemp) && (cd tools/desk && GOWORK=off go test -v -count=1 -run '^TestFixtureSiblingParentIsolation06$' ./cmd/deskwt) > "$o" 2>&1 && grep -F -- '--- PASS: TestFixtureSiblingParentIsolation06' "$o"` | exit 0; inherited control fails, isolated fixture preserves victim, and deliberate rejected input remains rejected; named test must execute |
| 4 | check:ci | `cd tools/desk && GOWORK=off go vet ./cmd/deskwt` | exit 0 |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate: model. Four risks are no because changes are confined to test setup and regression controls. Production admission, credentials, protected tables, operational actions and independent verification remain unchanged. Review must inspect the sibling dispositions and negative-control ordering.
