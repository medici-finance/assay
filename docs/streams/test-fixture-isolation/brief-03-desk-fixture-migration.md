---
brief: assay:assay:test-fixture-isolation:03
id: 380fd9d7-3682-4492-ac41-d56a067d975d
version: 1
title: Desk fixture migration
why: "Caller Git configuration, installed desk binaries and credential state can redirect test fixtures, delaying unrelated qualification and risking writes outside temporary HOME. Explicit fixture ownership makes failures describe the code under test."
wave: 1
depends: ["test-fixture-isolation/01"]
unblocks: ["test-fixture-isolation/04", "test-fixture-isolation/06"]
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
  - "tools/desk/cmd/deskdispatch/: follow-up test-fixture-isolation/03 (fake-child selection controls)"
  - "tools/desk/cmd/cellctl/helper_custody_test.go: follow-up test-fixture-isolation/03"
  - "tools/desk/cmd/deskevidence/landed_test.go: follow-up test-fixture-isolation/03"
  - "tools/desk/cmd/deskwt/: follow-up test-fixture-isolation/03 (maintenance fixture ownership)"
  - "statusgen/autonomy_gather_test.go: follow-up test-fixture-isolation/02 (different module and worker)"
---

# Brief 03 — Desk fixture migration

## Context

files: tools/desk/cmd/deskdispatch/decisionauth_test.go, tools/desk/cmd/deskdispatch/claimauth_test.go, tools/desk/cmd/deskdispatch/deskdispatch_test.go, tools/desk/cmd/cellctl/helper_custody_test.go, tools/desk/cmd/deskevidence/landed_test.go, tools/desk/cmd/deskevidence/flip_test.go, tools/desk/cmd/deskwt/deskwt_test.go, tools/desk/cmd/deskwt/maintenance_test.go, tools/desk/cmd/deskwt/maintenancehooks_test.go, changelog/desk-fixture-environment.md

facts:
- Decision-child tests install a fake claim binary but can resolve an installed deskdecision first.
- Cell-origin and landed-origin tests call real production authority readers in-process and must clean fixture process state, not only Git subprocesses.
- The maintenance global-config case must write only the fixture's explicit global file; temporary HOME alone is insufficient.
- #5014 autonomy fake-forge tests live in statusgen/autonomy_gather_test.go and are routed to brief 02; this desk slice does not own them.

Read first: README.md Shared interface contract, brief 01, the named test files and their production callers (read only).

single-point-of-failure: fake binary presence alone does not prove selection; record which fake executed, and retain an unrelated caller decoy plus a separate global-config victim digest.

design-fit:
  owner: existing desk fixture setup helpers
  contract: none — fixture construction only; the README owns the shared test-support API, with production credential/authority contracts unchanged
  retires: inherited PATH concatenation and HOME-only assumptions at the named sites
  weight: verbs +0, flags +0, production refusals +0, resident rule-text lines +0; test changes only
  why-add: separate process and child consumers must both observe the fixture's intended authority

## Ground rules

Use an owned isolated worktree and synthetic fixtures. Branch publication follows normal worker guards; no main push, merge, workflow dispatch, review, Evidence or lifecycle advance is authorized. Stop at an implementation handoff. Production security controls and protected acceptance tables are out of scope.

## Task

1. Use the shared fixture environment for decision/claim children, explicitly mapping the fake decision binary or legacy script as appropriate. Keep production resolution order and refusal behavior unchanged. Assert a fake-child receipt, not only an exit code.
2. Migrate cell-origin and landed-origin setup before repository creation and in-process authority reads. Preserve custody restoration and negative origin tests, including deliberate injected-config refusals planted after installation.
3. Migrate worktree maintenance fixtures to private Git config and declared tools. Build the hostile global-config victim independently and prove its bytes, mode and repository state remain unchanged; never baseline against the operator's real global file.
4. Inventory sibling test entry points and route those beyond these named groups with exact paths. Keep statusgen autonomy credential fixtures in brief 02. The hostile desk regression must prove intentional injected-config refusal survives fixture installation and must not contact a live forge.

Sibling disposition: deskperf, deskpushguard, deskreply and verifyloop fixture families are follow-up test-fixture-isolation/05. The wider deskwt newRepo/withEnv consumers are follow-up test-fixture-isolation/06; this preserves the maintenance-only bound of this delivery.

## Verify

New test names below are obligations to implement. No command contacts a live forge or cluster.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -run 'TestDecisionChild' ./cmd/deskdispatch` | exit 0; fake-child receipts show intended tool and credential selection |
| 2 | check:ci | `cd tools/desk && GOWORK=off go test -count=1 -run 'TestDeskdCoordinateUsesCellOriginNotCallerDirectory\|TestLandedApprovalContext' ./cmd/cellctl ./cmd/deskevidence` | exit 0; both real authority-reader fixture paths execute |
| 3 | check:ci +mutation | `cd tools/desk && GOWORK=off go test -count=1 -run '^TestMaintenanceFixtureGlobalVictim$' ./cmd/deskwt` | exit 0; inherited control modifies only disposable victim; isolated fixture leaves victim unchanged |
| 4 | check:ci +mutation | `cd tools/desk && GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=grep.patternType GIT_CONFIG_VALUE_0=default GIT_CONFIG_KEY_1=grep.extendedRegexp GIT_CONFIG_VALUE_1=false GIT_NO_REPLACE_OBJECTS=1 GOWORK=off go test -count=1 -run 'TestDeskdCoordinateUsesCellOriginNotCallerDirectory\|TestLandedApprovalContext\|TestDecisionChild\|TestMaintenanceHookPolicy\|TestMaintenanceFixtureGlobalVictim' ./cmd/cellctl ./cmd/deskevidence ./cmd/deskdispatch ./cmd/deskwt` | exit 0; real named suites run under admitted-style Git inputs, fake-child receipts retained and explicit origin refusal still rejects |

## Evidence

Independent desk to record after implementation; intentionally empty.

## Review

Gate: model. Four risks are no: only test setup and regression controls, no authority/credential implementation changes. Existing production security tests remain mandatory and use explicit hostile input after isolation. Reviewer judges the sibling routing; verifier owns subsequent qualification.
