---
brief: assay:assay:graph-execution:30
title: Derived evaluation context and bounded validity
why: >-
  A cached eligible result can outlive a lease or permission even when no source file
  changes. Runtime consumers need the complete inputs and validity bound of an
  evaluation so display reuse cannot become stale admission.
wave: 3
depends: ["graph-execution/28"]
unblocks: ["graph-execution/14", "graph-execution/19"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v2
version: 1
id: e9baecf8-fffd-47a9-9fe2-6a28c4f3f2dd
authored: 2026-10-14 by worker-desk (author-brief)
sources:
  - docs/proposals/non-ci-architecture/22-runtime-contract-alignment.md
  - docs/streams/graph-execution/work-input-amendment.md — WI-2 applicability
  - statusgen/evaluation/coverage_api.go — DependencyManifest and explicit CoverageInput
  - statusgen/evaluation/identity.go — canonical evaluator Identity
  - freshness-checked 2026-10-14 @ 9a8df91dc; no statusgen/domain or runtime context contract on main
exec-tier: strong
exec-tier-why: Time, dependency coverage and revision identity must agree across pure evaluation, storage and admission.
domain: complicated
consumers:
  - "spec/evaluation-context-v1.md, schemas/evaluation-context-v1.json, statusgen/domain, statusgen/testdata/domain/context: follow-up graph-execution/30"
  - "tools/desk/internal/deskkit/graphadmission.go: follow-up graph-execution/14"
  - "workflow/store and workflow/bindings: follow-up graph-execution/19"
  - "workflow/controller: follow-up graph-execution/21 (through admission /14)"
  - "forgeread source/index caches: out-of-scope (forge-neutral/42–43 own those caches; this contract grants no source access)"
---

# Brief 30 — Derived evaluation context and bounded validity

## Context

files: `spec/evaluation-context-v1.md` (planned), `schemas/evaluation-context-v1.json` (planned), `statusgen/domain/context.go` (planned), `statusgen/domain/context_test.go` (planned), `statusgen/testdata/domain/context/` (planned), `docs/contracts.md`,
`docs/lifecycle.md`, `changelog/graph-execution-30-evaluation-context.md` (planned).

facts (9a8df91dc, 2026-10-14):
- /28 creates the domain records; no domain package exists on this main baseline.
- Existing `evaluation.EvaluateCoverage` takes a revision and dependency manifest;
  WI-2 source/policy/build/environment reuse remains its owner. Do not fork that algorithm.
- `evaluation.EvaluateEligibility` consumes explicit graph facts. It has no live clock,
  credentials or runtime lease authority. The caller must establish current runtime facts.
- /14 rechecks authoritative admission; /19 persists bindings. Those consumers are amended
  with dependency/Verify rows in this authoring change, not implemented here.

layering: extend /28's pure domain package with data and applicability validation. Time is
supplied explicitly; filesystem, clock reads, network, SQL and authorization remain adapters.
There is no new cache service, evaluator or scheduler.

single-point-of-failure: the context validator could omit an input; the admission boundary
independently obtains current lease/access observations and refuses an expired or unavailable
one. A cache hit cannot supply that authority. Row 3 walks both checks with synthetic facts.

design-fit:
  owner: statusgen/domain records; existing eligibility and coverage owners keep their algorithms
  contract: S-verification-domain (planned by /28), extended by evaluation-context-v1
  retires: [implicit assumption that unchanged hashes mean a still-current runtime decision]
  weight: "verbs 0, flags 0, refusals 0; additive pure record validation"
  why-add: n/a; extend the existing domain owner rather than introduce a cache subsystem

## Read first

- [Runtime alignment](../../proposals/non-ci-architecture/22-runtime-contract-alignment.md).
- /28 domain contract and `statusgen/evaluation/coverage_api.go`, `identity.go`.
- /14 admission and /19 store, including their dated context amendments.

## Ground rules

- Isolated branch and draft PR; no deployment, provider contact, merge or credential changes.
- Stop at implemented; independent verification owns later states.
- Synthetic fixtures only. Keep source/coverage algorithms and current human gates.
- No bearer token, credential or raw identity secret in fingerprints or diagnostics.

## Task

1. Define a versioned context using /28 identities: qualified cell/repository/work,
   candidate/acceptance digest, exact source/fact frontiers, evaluator identity,
   query/options, dependency-set digest and coverage, policy/environment versions,
   access-policy epoch, reevaluation generation, observed cutoff and applicable expiry
   bounds. Distinguish immutable derivation identity from a current-use validity check.
   Unknown or incomplete inputs cannot claim a reusable complete context.
2. Add canonical fingerprinting and pure applicability validation to `statusgen/domain`.
   Accept explicit trusted observation time/current context; do not read the clock or
   resolve access yourself. Reject use at/after the earliest required expiry, stale
   epochs/generation, changed membership, missing inputs or mismatched subject/version.
   Caller authorization is a separate mandatory check, never implied by a fingerprint.
3. Supply fixed expected results and schema fixtures. A graph member's removal, a formerly
   absent dependency becoming present, time passing without a new commit, and a policy or
   evaluator update invalidate affected reuse. Incomplete dependency coverage requires
   conservative reevaluation; the implementation must not invent selective applicability.
4. Drive the real /28 Assess and existing evaluator through one fixture flow. Reusing
   computation never retargets an old exact-revision PASS to a new candidate. Compare
   reuse with fresh evaluation, including an authorized immutable hit and an expired
   runtime decision over the same immutable data. No production gate cache is implemented.
5. Update the semantic-owner row, lifecycle explanation, schema and changelog. Document
   /14 and /19 consumption and offline cutoff semantics. Keep environment identity explicit;
   a recorded model/tool digest proves attribution, not reproducibility of nondeterminism.

## Interface contract

Normalized explicit observations -> immutable context/fingerprint plus typed applicability
result -> existing Assess/evaluation. The result is advisory until current admission checks
succeed. Future adapters refuse unsupported mandatory fields and retain invalidation reasons.

## Verify

Named tests and files below are implementation deliverables; no PASS is asserted at authoring.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextInputs$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextInputs " "$out")` | exit 0; named PASS; each declared input changes the fingerprint or applicability; missing membership/coverage is held; mutation: omit evaluator identity |
| 2 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextExpiry$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextExpiry " "$out")` | exit 0; named PASS; unchanged source hashes at and after lease/access expiry are unusable; unavailable time is held; mutation: skip expiry comparison |
| 3 | check:ci +flow +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextAdmissionFlow$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextAdmissionFlow " "$out")` | exit 0; named PASS; real Assess/evaluator flow preserves candidate binding; a synthetic current-authority check refuses an expired grant even with the upper context validator bypassed; mutation: treat a hit as authority |
| 4 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextMembership$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextMembership " "$out")` | exit 0; named PASS; removed and newly discovered dependencies force reevaluation; incomplete coverage never selectively releases; mutation: compare only surviving dependencies |
| 5 | check:ci +dereference +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestContextSchema$" ./domain/ > "$out" && grep -F -- "--- PASS: TestContextSchema " "$out")` | exit 0; named PASS; positive/negative fixtures agree with schema and spec field inventory; no secret material is serialized; mutation: remove a mandatory schema field |

## Definition of Done

All Verify rows discriminate against their mutations; /28 records and the existing evaluator
are reused. Declared docs/schema and changelog match code. No managed runtime is activated.
The contract's adequacy and choice of expiry sources remain independent review judgments.

## Pre-mortem

Missing input (row 1), unchanged hashes hiding expiry (row 2), cache grants authority (row 3),
incomplete membership (row 4), and a schema that omits fields (row 5) have named failing controls.

## Evidence

<!-- Independent verifier records each row, exact subject/environment, command, exit and observed result. -->

## Review

Gate: model; no live authority or data is changed. Confirm no second evaluator/cache owner.
