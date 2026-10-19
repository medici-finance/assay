---
brief: assay:assay:forge-neutral:43
title: "Incremental forge indexes and dependency-aware invalidation"
why: "A shared mirror saves provider reads but can still recompute every query after every event. Dependency-aware derived indexes allow selective work while proving that new objects, time expiry and permission changes cannot leave a stale answer looking current."
wave: 8
depends: ["forge-neutral/37", "forge-neutral/42"]
unblocks: ["forge-neutral/45"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm selective invalidation only for the demonstrated complete dependencies and workload. Broader or unknown dependencies require a bounded fresh evaluation; an optimization cannot relax eligibility, coverage or access requirements."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 2
id: dd9e0da4-b348-4311-a31c-83605c8a6522
authored: 2026-10-14 by forge-service authoring
sources:
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/forge-neutral/README.md
  - freshness-checked 2026-10-14 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
  - R6 proposal inspected at c5e8a3f8da3c715b72efa8d8d6b5e74e659ebd52; main merged into this authoring branch without editing historical Evidence
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: Cross-component identity, privacy, concurrency and failure semantics require design judgment and negative controls.
domain: complicated
consumers:
  - "forgeread/mirror/derive/: follow-up forge-neutral/43"
  - "forgeread/testdata/derive/: follow-up forge-neutral/43"
  - "workflow/store/forgederived.go: follow-up forge-neutral/43"
  - "workflow/store/forgederived_test.go: follow-up forge-neutral/43"
  - "tools/desk/internal/forgeservice/query/: follow-up forge-neutral/43"
  - "docs/forge-service.md: follow-up forge-neutral/43"
  - "docs/contracts.md: follow-up forge-neutral/43"
  - "changelog/forge-incremental-indexes.md: follow-up forge-neutral/43"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
  - "forgeread/mirror/derive/reviewcontext_test.go: follow-up forge-neutral/43"
---

# Brief 43 — Incremental forge indexes and dependency-aware invalidation

## Context

files: `forgeread/mirror/derive/` (planned), `forgeread/testdata/derive/` (planned), `workflow/store/forgederived.go` (planned), `workflow/store/forgederived_test.go` (planned), `tools/desk/internal/forgeservice/query/` (planned), `docs/forge-service.md` (planned), `docs/contracts.md`, `changelog/forge-incremental-indexes.md` (planned), `forgeread/mirror/derive/reviewcontext_test.go` (planned). Paths not present at the source pin are planned deliverables; existing
files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- 37 owns base observations, admissions and refresh. This brief adds rebuildable derived indexes, not base forge records or a second mirror.
- statusgen/evaluation remains an existing semantic owner; query caching cannot introduce another readiness/acceptance rule or imports that widen forgeread root.
- 42 supplies exact-source inputs; mutable forge snapshots, time and access epochs remain separate dependencies.

layering: forgeread mirror-derived indexes; domain evaluation stays with existing application/evaluator owners. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: an incomplete reverse dependency list falsely declares a cached result current; full recomputation comparisons, scope-membership invalidation and access/expiry gates independently detect or constrain it.

design-fit:
  owner: forgeread mirror-derived indexes; domain evaluation stays with existing application/evaluator owners
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags 0; refusals +2 unknown dependency/version classes; rule-text +0
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm selective invalidation only for the demonstrated complete dependencies and workload. Broader or unknown dependencies require a bounded fresh evaluation; an optimization cannot relax eligibility, coverage or access requirements.

Options:
1. **Accept the demonstrated bounded design** — accept only the proven scope and failure behavior; keep live credential provisioning and activation as separate operator acts.
2. **Hold for correction** — identify the unproven boundary or budget and revise before acceptance.

Default if no answer: none — acceptance remains pending. This implementation-evidence decision
does not prevent authoring or independent offline fixture work. Existing mirror retention and
webhook payload decisions are not answered by this section.

## Ground rules

- Isolate changes; never push main, merge, ready-flip, release, dispatch workflows or contact live infrastructure.
- Use synthetic data, disposable processes/repositories and explicit local database fixtures; no production tokens or data.
- Preserve role, candidate, unknown-outcome, required audit/budget and canonical acceptance meanings.
- Stop at implemented; independent verification and acceptance remain separate.
- Do not widen an unsupported provider capability or duplicate another store/evaluator to make a test pass.

## Task

1. Implement versioned rebuildable reverse indexes from source observations to bounded query results (repository inventory, change/comment/check references and affected source query keys). Declare every input revision/admission, derivation version and coverage requirement. Persist adapters in the existing workflow store and keep pure dependency planning in forgeread.

2. Invalidate known affected results on confirmed writes and refreshes. Membership additions/deletions, changed query definitions, missing dependencies, time/expiry and access/policy epochs cause the required broader invalidation. A cache hit cannot change an unavailable input into complete data. Coalesce recomputation without erasing distinct decisions or reevaluation generations.

3. Reuse the existing domain evaluator through application composition when a query needs eligibility/review currency. Store references/derived indexes rather than authoritative approval or accepted-work booleans. Keep forgeread dependency-free and root/offline linking unchanged.

4. Compare incrementally maintained results with fresh full queries against an independently selected fixture sequence covering review head changes, missed/reordered hints, new issues and access expiry. Use 37 source refresh and 42 exact-source keys rather than duplicating either. A crash/rebuild discards derived caches safely and reconstitutes their complete inputs.

5. Measure recomputed query count, provider calls, CPU and cold/warm latency at declared fixture scale. An unrelated known observation should preserve unaffected immutable results; when dependency completeness cannot be established, prefer bounded full recomputation. No new universal performance threshold or readiness shortcut.


### Approved workflow-efficiency amendment — 17 October 2026

Consider incremental reuse after duplicate collection and narrow projections are measured (desk-
tools/37–44), not as the first optimization. Include head/base/body/review/check/source-
admission/schema-version and population membership dependencies for shared review contexts/queues.
Retain exact immutable source keys and mandatory mutable rechecks. Never cache authoritative
approved/ready booleans or mark incomplete dependency knowledge complete. Compare against the shared
semantic oracle and full recomputation; report CPU/wall/attempt/points components and coverage
rather than a single hit ratio.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

## Interface contract

Use forge-service-v1 from 39, mirror admission/freshness from 37, event hints/demand from 38,
R2/R4 logical effect identity and current R2/R3 authority. Cache/observation tokens never grant
permission. Preserve requester, represented actor, provider executor and exact source inputs.
Provider outcome and local projection availability are separate. Unsupported, stale, incomplete
and permission-denied are explicit; loss does not become success, an empty queue or fresh authority.

## Verify

Named tests below are implementation deliverables, not tests already run. Each command requires
the named PASS line, so a missing test or skipped database case fails. Database tests must use
the inherited graph-execution/19 explicitly disposable PostgreSQL harness and fail when it is
unavailable; no live context or silent skip. Faulty controls exercise the actual production
boundary and independent expectation, not another implementation of the same checker.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd forgeread && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestIncrementalDerivationEquivalence$' ./mirror/derive/... > "$r" && grep -F -- '--- PASS: TestIncrementalDerivationEquivalence ' "$r")` | exit 0; named PASS. A literal expected sequence and fresh full oracle agree with incremental results after known changes, new members, missing observations, candidate changes and deletions; an omitted dependency plant fails the comparison. |
| 2 | check:ci +flow +mutation | `(cd workflow && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestDerivedIndexPostgresRecovery$' ./store/... > "$r" && grep -F -- '--- PASS: TestDerivedIndexPostgresRecovery ' "$r")` | exit 0; named PASS. Real disposable PostgreSQL persists input/version dependencies; restart and discarded derived rows rebuild from base observations. An incomplete base snapshot never gains complete coverage. |
| 3 | check:ci +flow +mutation | `(cd tools/desk && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestDerivedAccessTimeAndWriteFlow$' ./internal/forgeservice/... > "$r" && grep -F -- '--- PASS: TestDerivedAccessTimeAndWriteFlow ' "$r")` | exit 0; named PASS. Actual served write/refresh -> derived query -> expiry/revocation. Known unaffected result reused; affected result changes; private joins/counts and expired reviews do not survive an access/time transition. |
| 4 | check:ci +flow +mutation | `(cd forgeread && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestDerivationCostAndFallback$' ./mirror/derive/... > "$r" && grep -F -- '--- PASS: TestDerivationCostAndFallback ' "$r")` | exit 0; named PASS. Declared fan-out fixture records selective/full recomputation costs; unknown membership forces bounded full recomputation. Faster but stale results fail the independent oracle. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/43` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |
| 6 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewIndexInvalidation$' ./mirror/derive/... > "$o" && grep -F -- '--- PASS: TestReviewIndexInvalidation ' "$o")` | exit 0; named PASS. All review applicability/scope/access changes match fresh oracle; omitted dependency, cached ready or unknown-as-empty mutation fails. |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Performance reuse amendment — 2026-10-16

Use canonical parsing/evaluation owners on exact admitted source snapshots, retaining
the reverse dependency and full-recompute oracle already specified. Incorporate the
manifest/membership invalidation lessons from desktools-v2/60 without importing that
desk-specific activation algorithm. A new source/consumer outside the previous read set,
changed derivation version, dirty snapshot or access epoch must invalidate the necessary
result even if all previously read hashes match. Report file reads/parses, recomputation,
cache hit/miss and request costs separately; faster stale results fail. Statusgen/26 is the
credential-free offline measurement consumer; serving indexes do not become its authority.


## Evidence

<!-- Independent verifier records source/subject revision, environment, literal command,
exit, named result, positive and failing control, and date. No result is asserted at authoring. -->

## Review

Gate: human. Inspect actual dependency/import and credential reachability, private data and
count isolation, current authority and unknown outcomes. Tests cannot authorize provisioning
credentials, switching a managed writer, answering a human decision or changing a merge policy.
