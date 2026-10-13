---
brief: assay:assay:statusgen:24
title: Portfolio allocation and capacity accounting
why: "Allocation targets cannot be compared with agent usage or human attention reliably until their units, identities and missing data are explicit."
wave: 2
depends: ["statusgen/22"]
unblocks: ["statusgen/25"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: 5762abad-67ee-467a-9773-619f25a0dd06
consumers:
  - "spec/portfolio-measurement-v1.md, schemas/portfolio-measurement-v1.json, statusgen/portfolio: follow-up statusgen/24"
  - "statusgen portfolio CLI, projection consumers: follow-up statusgen/25"
  - "loopadmin/runner telemetry production: out-of-scope (live active-time collector is deferred; consume explicit attributed intervals)"
---

# Brief 24 — Portfolio allocation and capacity accounting

## Context

files: `spec/portfolio-measurement-v1.md` (planned), `schemas/portfolio-measurement-v1.json` (planned), `statusgen/portfolio/` (planned), `statusgen/testdata/portfolio/` (planned), `docs/contracts.md`; `changelog/statusgen-24-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- loopadmin/runner carries caller/id, profile, actual_model and usage; it does not promise active-time intervals.
- No current opmetrics aggregate proves a per-bucket decision identity or duration.
- Adopter policy supplies numeric weights and allocation targets; this brief introduces no default or dispatch effect.

layering: pure typed records/evaluators in `statusgen/portfolio`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/portfolio
  contract: portfolio-measurement-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/21-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Define portfolio policy records with scope/window/revision, qualified REQ bucket membership, independent agent/human targets, explicit weight policy and optional already-authorized limits. Validate units, overlapping/ambiguous assignment and source coverage without deciding policy.
2. Implement a pure ledger over explicit invocation segments and decision records. Deduplicate source reports by stable identity; distinct retries count separately. Preserve profile and actual model. Label elapsed-time proxies separately from active time and retain unknown measures, known subtotals, unattributed decisions and undefined zero-denominator shares.
3. Compare measured allocation with policy without modifying the target or dispatching filler. Provide a documented adapter example from the existing runner observation fixture, explicitly requiring a supplied interval source; preserve the runner contract and existing Evidence. Live collectors and default policies are out of scope.

## Interface contract

Input: versioned allocation policy, invocation/interval observations, decision records and visibility-filtered REQ references. Output: independent capacity ledgers, coverage, attributed divergence and explicit undefined/unknown states; never an admission decision.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioLedgerIdentityAndUnits$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioLedgerIdentityAndUnits " "$check_output")` | exit 0; named PASS; duplicate observations do not double count, retries count once each and human counts never join agent durations; mutation: count every report |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioUnknownAndZero$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioUnknownAndZero " "$check_output")` | exit 0; named PASS; unknown weight/time/cost only taints its measure; subtotal/coverage remain explicit; all-zero shares undefined; mutation: coerce missing to zero |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioRunnerAdapterFlow$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioRunnerAdapterFlow " "$check_output")` | exit 0; named PASS; actual runner-shaped fixture plus attributed interval reaches the ledger with profile/model preserved; missing interval cannot masquerade as active time; mutation: use elapsed duration silently |
| 4 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestPortfolioTargetNotConsumption$" ./portfolio/ > "$check_output" && grep -F -- "--- PASS: TestPortfolioTargetNotConsumption " "$check_output")` | exit 0; named PASS; 5 percent target with 2 percent observed remains divergent without a policy rewrite or effect; mutation: normalize target to consumption |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.
