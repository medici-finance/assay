---
brief: assay:assay:test-fixture-isolation:02
id: 1f02f6c6-e9f2-4f88-b9ee-0313841bc091
version: 1
title: Statusgen Git fixture migration
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 1
depends: ["test-fixture-isolation/01"]
unblocks: ["test-fixture-isolation/04", "test-fixture-isolation/08"]
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
  - "statusgen/: follow-up test-fixture-isolation/02 (declared test files and test-support wrapper)"
  - "statusgen/gitcore_read.go: out-of-scope (production native-profile refusal stays unchanged)"
  - "docs/streams/statusgen/brief-40-chain-approved-acceptance-transitions.md: out-of-scope (protected table and Evidence remain unchanged)"
---

# Brief 02 — Statusgen Git fixture migration

## Context

files: statusgen/acceptancetransition_test.go, statusgen/transitionchain_test.go, statusgen/transitioncontext_test.go, statusgen/transitionhistory_test.go, statusgen/transitionroundtrip_test.go, statusgen/expecttransition_test.go, statusgen/protectedrecord_test.go, statusgen/flow_52_test.go, statusgen/coverage_crossrepo_test.go, statusgen/coverage_session_test.go, statusgen/readset_adapter_test.go, statusgen/readset_time_test.go, statusgen/transitionbracket_test.go, statusgen/runnerform_test.go, statusgen/mixedtransitionflow_test.go, statusgen/fixture_isolation_test.go (new), statusgen/autonomy_gather_test.go, statusgen/gitinfo_test.go, statusgen/gitquiet_test.go, changelog/statusgen-fixture-environment.md

facts:
- At the pinned source, gitCoreFor checks process GIT_CONFIG_COUNT presence before opening the cached reader; cleaning only child commands cannot fix that path.
- contextHistoryFixture, chainFixtureWith, vfFixture, mustGitInit and coverage fixture entry points must be inspected for overlapping construction and deliberate hostile-input tests.
- statusgen/40's existing failed qualification and all protected Verify tables remain unchanged by this repair.

Read first: README.md Shared interface contract, brief 01, statusgen/gitcore_read.go, statusgen/acceptancetransition_test.go, statusgen/gitinfo_test.go.

single-point-of-failure: constructor adoption can omit in-process callers; an outer hostile subprocess exercises real transition, coverage and flow tests without using their internal environment builder.

design-fit:
  owner: statusgen test fixture constructors
  contract: none — fixture construction only; the README owns the test-support API and production authority contracts remain unchanged
  retires: duplicated incidental Git-variable unsets in migrated fixture entry points
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0
  why-add: fixture construction and production in-process readers must observe the same intended environment

## Ground rules

Use an owned isolated worktree and synthetic fixtures. Branch publication follows normal worker guards; no main push, merge, workflow dispatch, review, Evidence or lifecycle advance is authorized. Stop at an implementation handoff. Production security controls and protected acceptance tables are out of scope.

## Task

1. Install brief 01 at selected top-level transition/context, coverage, flow and autonomous-fixture tests before their constructors run; constructor reuse must not erase deliberate later injections. Retain prepared Go cache paths and explicitly approved Git/Go/shell dependencies; never replace the caller PATH with an open system-directory fallback.
2. Give child Git calls explicit fixture environments, including intentional author dates/identities and the owned quiet template. Migrate direct child uses in the declared files; route further sibling sites with exact paths.
3. Migrate autonomyGHFixture in autonomy_gather_test.go to declared fake gh and explicit synthetic role-token state, preserving stored-login refusals. Preserve tests that intentionally reject injected configuration. Install the fixture first and plant rejected inputs afterward; ensure nested constructors do not erase deliberate controls. Do not alter gitCoreFor, NativeDiscoveryReason, authority readers or their verdicts.
4. Add an outer subprocess regression covering transition-chain, coverage and FlowCard under admitted-style Git variables and a disposable global-config victim. The child uses actual fixture consumers; assert named tests execute, no skip substitutes for execution, and the victim is unchanged.

## Verify

The new regression names are implementation obligations, not existing coverage claims.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -run '^(TestTransitionChain\|TestAcceptance\|TestCoverage\|TestFlowCard\|TestFixtureGitQuiet\|TestQuietEnvPreserved\|TestAutonomy)' .` | exit 0; existing behavioral assertions retained |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -run '^TestStatusgenFixtureHostileParent$' .` | exit 0; named real tests run under hostile parent and fixture victim unchanged; inherited-environment control fails |
| 3 | check:ci | `cd statusgen && GOWORK=off go test -count=1 -run '^TestFixtureAuthorityControls$' .` | exit 0; explicitly injected unsupported profiles remain rejected after fixture installation |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate: model. All risks are no because only test fixtures change. Production Git refusal logic and admitted row environment stay unchanged. Post-merge statusgen/40 qualification remains the verify desk's work, including its separate row-10 environment limitation.
