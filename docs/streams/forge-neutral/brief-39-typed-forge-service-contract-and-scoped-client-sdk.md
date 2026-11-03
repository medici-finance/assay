---
brief: assay:assay:forge-neutral:39
title: "Typed forge service contract and scoped client SDK"
why: "The mirror and the desk write paths lack a shared caller contract for scoped queries, operation receipts and read-after-write visibility. A small typed contract lets both clients use the same meanings without importing credentials or the entire desk module."
wave: 3
depends: ["desk-tools/39"]
unblocks: ["forge-neutral/40", "forge-neutral/42", "cell-api/01"]
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Confirm the demonstrated wire allowlist and privacy boundary. The contract does not grant a caller a role or a credential; detailed issuer and executor configuration remains separately controlled."
decision-trigger: spec
issues: []
outcome: none
schema: brief-v2
version: 4
id: a2814dbf-4d0c-4cef-bfc6-aceaf50dd255
authored: 2026-10-14 by forge-service authoring
sources:
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
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
  - "forgeread/service/: follow-up forge-neutral/39"
  - "forgeread/client/: follow-up forge-neutral/39"
  - "forgeread/testdata/service/: follow-up forge-neutral/39"
  - "spec/forge-service-v1.md: follow-up forge-neutral/39"
  - "schemas/forge-service-v1.json: follow-up forge-neutral/39"
  - "docs/forge-service.md: follow-up forge-neutral/39"
  - "docs/library-first.md: follow-up forge-neutral/39"
  - "docs/contracts.md: follow-up forge-neutral/39"
  - "changelog/forge-service-contract.md: follow-up forge-neutral/39"
  - "statusgen/evaluation/: out-of-scope (existing domain evaluation meaning preserved)"
  - "plugins/assay/skills/: out-of-scope (no caller wired to an unshipped service)"
  - "forgeread/service/reviewmapping_test.go: follow-up forge-neutral/39"
---

# Brief 39 — Typed forge service contract and scoped client SDK

## Context

files: `forgeread/service/` (planned), `forgeread/client/` (planned), `forgeread/testdata/service/` (planned), `spec/forge-service-v1.md` (planned), `schemas/forge-service-v1.json` (planned), `docs/forge-service.md` (planned), `docs/library-first.md`, `docs/contracts.md`, `changelog/forge-service-contract.md` (planned), `forgeread/service/reviewmapping_test.go` (planned) — Paths not present at the source pin are planned deliverables; existing files are amended only at the named seam. No user-home or out-of-repo configuration is edited.

facts:
- forgeread is a dependency-free module. Its root OpenIssues/Result and OfflineReader remain the supported current surface; service packages are new deliverables.
- The mirror/event briefs are pending draft contracts. Mirror identity is provider, host, repository path and object kind/number; a PR and MR are not interchangeable.
- deskkit is internal to tools/desk. Exporting a giant interface or importing it from a separate module is not a reuse solution.

layering: forgeread service envelopes; domain identities and effect authority remain with their existing owners. Pure typed requests and source facts stay independent of
transport/storage/custody; effectful composition remains inside the desk module and R2 store.
Task and served/negative Verify cases below enforce that split.

single-point-of-failure: the binding from wire references to admitted source identities; the served scope gate and the mirror store enforce admission independently, and root dependency tests prevent credential imports.

design-fit:
  owner: forgeread service envelopes; domain identities and effect authority remain with their existing owners
  contract: none — extend existing read/effect owners and register only the new transport or derived-index responsibility in the named docs
  retires: []
  weight: verbs 0; flags 0; refusals +4 typed protocol errors; rule-text +0; two narrow packages
  why-add: Reuse existing mirror, effect, Git and custody boundaries instead of adding another authority. The named capability fills a concrete missing consumer seam; measure the implemented delta and do not widen it speculatively.

## Read first

Read [the bounded architecture amendment](https://github.com/medici-finance/assay/blob/b59a1dc90ce9f383ba350eca8398fe2f6d934152/docs/proposals/non-ci-architecture/21-forge-service-and-source-queries.md), the stream README's dated forge-service
section, and each dependency brief's current contract. All load-bearing requirements for this
brief are restated here. The architecture draft is not activation approval; refresh its revision,
source paths, active claims and existing decisions before dispatch. Missing required contracts
mean NEEDS_CONTEXT, not an invented substitute or a narrowed acceptance test.

## Human decision

Confirm the demonstrated wire allowlist and privacy boundary. The contract does not grant a caller a role or a credential; detailed issuer and executor configuration remains separately controlled.

Options:
1. **Accept the demonstrated bounded design** — accept only the proven scope and failure behavior; keep live credential provisioning and activation as separate operator acts.
2. **Hold for correction** — identify the unproven boundary or budget and revise before acceptance.

Default if no answer: none — blocks until answered.

Acceptance remains pending. This implementation-evidence decision
does not prevent authoring or independent offline fixture work. Existing mirror retention and
webhook payload decisions are not answered by this section.

## Ground rules

- Isolate changes; never push main, merge, ready-flip, release, dispatch workflows or contact live infrastructure.
- Use synthetic data, disposable processes/repositories and explicit local database fixtures; no production tokens or data.
- Preserve role, candidate, unknown-outcome, required audit/budget and canonical acceptance meanings.
- Stop at implemented; independent verification and acceptance remain separate.
- Do not widen an unsupported provider capability or duplicate another store/evaluator to make a test pass.

## Task

1. Publish forge-service-v1 and independent literal fixtures for bounded mirror queries, FileAt/Diff/MergeBase source queries, issue creation, change comment creation, operation lookup and observation-token reads. Restrict fields/count/bytes, reject unknown required schema/operations and keep provider-specific facts. Translate existing qualified work/effect identities at application composition; do not invent a competing work namespace.

2. Define caller/source admission, access epoch, observed source revision, completeness/freshness, request identity and minimum observation token. Observation tokens are opaque and scope-bound; no global SQL sequence/LSN or inaccessible activity count is exported. Tokens express observation progress, not authorization or a total provider version. Define older, confirmed, newer, projection-pending, unavailable and permission outcomes, without turning unknown into empty/success.

3. Define effect-result transport over R2/R4 operation identities: provider outcome succeeded/failed/unknown, projection status current/pending/unavailable, confirmed provider object identity, payload digest and bounded provenance. A retry of one logical operation retrieves/reconciles it; a different payload conflicts. No universal exactly-once or conditional-write claim.

4. Implement pure service envelope validation and a separately imported stdlib HTTP client accepting an already-admitted transport and endpoint from trusted composition. No environment, credential discovery, arbitrary URL passthrough, issuer or role override. Preserve the root/offline import closure and zero external module dependencies. Relative response links cannot redirect credentials to another provider.

5. Publish the local fixture example and mapping from existing reader/desk outputs. Retain existing schema fixtures unchanged. Register transport ownership only in docs/contracts.md; policy, evidence, candidate and effect meanings stay with their current owners. Remote login, managed serving and write activation are later consumers.


### Approved workflow-efficiency amendment — 17 October 2026

Carry review-observation-v1 and role issue projection envelopes through the public typed service
contract, preserving scope, provenance, per-input completeness/freshness, exact SHA/applicability,
all action rows and same-population depth. Reuse the semantic contract from desk-tools/39 through an
explicit legal extraction/mapping; tools/desk/internal cannot be imported from forgeread or
statusgen. Keep forgeread root pure/credential-free and eligibility/coverage meanings with
statusgen/evaluation. A transport DTO is not a second evaluator. Version matrix covers old
OpenIssues consumers and unknown mandatory schema. The direct bridge may ship before this service;
no private consumer is a prerequisite.

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
| 1 | check:ci +flow +mutation | `(cd forgeread && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeServiceContractCompatibility$' ./service/... > "$r" && grep -F -- '--- PASS: TestForgeServiceContractCompatibility ' "$r")` | exit 0; named PASS. Literal schema/example fixtures traverse the production encoder/decoder and retain provider provenance, null/unknown states, old OpenIssues behavior and exact candidate references. A schema-valid wrong observation token fails the independent expected fixture. |
| 2 | check:ci +flow +mutation | `(cd forgeread && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestForgeServiceClientFlow$' ./client/... > "$r" && grep -F -- '--- PASS: TestForgeServiceClientFlow ' "$r")` | exit 0; named PASS. Real fixture HTTP server -> client -> typed reader exposes confirmed/newer/pending correctly; unauthorized object existence is not exposed. No arbitrary host, redirect or role can widen the admitted transport. |
| 3 | check:ci +flow +mutation | `(cd forgeread && r=$(mktemp) && trap 'rm -f "$r"' 0 && GOWORK=off go test -count=1 -v -run '^TestServicePackageBoundaries$' ./... > "$r" && grep -F -- '--- PASS: TestServicePackageBoundaries ' "$r")` | exit 0; named PASS. Dependency closure keeps root/offline free of service HTTP/database/custody and forbids environment discovery in the new packages; positive stdlib-only closure passes and the planted transitive ambient-credential variant fails. |
| 4 | check:ci +dereference | `statusgen --root . --consumers --brief forge-neutral/39` | exit 0; implementation diff corroborates routing claims. Out-of-scope claims remain reviewer judgments, not gate-certified passes. |
| 5 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestReviewServiceMapping$' ./service/... > "$o" && grep -F -- '--- PASS: TestReviewServiceMapping ' "$o")` | exit 0; named PASS. Independent review-observation/projection fixtures roundtrip through public encoder/client with exact scope/head/depth and null semantics; malformed schema or cmd/internal import is rejected. |

## Pre-mortem and detection

For each critical Verify case, retain the valid control and a deliberately wrong subject
variant: removed permission/generation check, unknown converted to success, wrong source key,
omitted invalidation or unsafe replay, as applicable. The independent expected result must
reject that variant; record the failing control with the restored production pass. Do not
satisfy a flow row with a route registry, a prose legend or zero selected tests. Test fixtures
qualify their named local boundary; provider permission inventories, deployment workload and
operator cutover remain explicit evidence/acceptance obligations.

## Perf-v3 delivery amendment — 19 October 2026

For this brief's existing performance observations, reuse the [perf-v3 contract](../../performance-attribution-spec.md) and /63–65's
shared typed context and request-cost seam when those adapters are available. Each process
records its own attempts, timing and observed GraphQL points with units/basis/coverage;
client-to-service and service-to-provider work are distinct. Carry admitted parent invocation
links when known, preserve missing context and never turn it into operational authority.
No extra provider request or expanded business query for monitoring; qualified bounded cost
metadata may join the same request. No copied descendant call total,
and no v2-only replacement instrument. Keep this brief's scope/Verify/Evidence and independent
service delivery; record unsupported boundaries until producer integration. /45's existing
matched service qualification must include own-request and parent/child no-double-count cases.

## Query/cache measurement amendment — 19 October 2026

Apply docs/performance-attribution-spec.md, including its query/cache boundary contract.

Freeze typed logical-call versus transport-attempt versus server-handler observations and optional bounded opaque diagnostic links. Propagation is separately admitted at the existing transport/protocol and requires workflow consent at both ends; no automatic headers, raw work/session/agent locators, new endpoint or credential discovery. Untrusted links confer no authority. Client/server reports of one call are not two logical queries.

Extend TestForgeServiceClientFlow and the existing corresponding Verify flow with these negative cases; require its exact PASS line.
No service activation or extra dependency on a live collector is introduced.

## Evidence

<!-- Independent verifier records source/subject revision, environment, literal command,
exit, named result, positive and failing control, and date. No result is asserted at authoring. -->

## Review

Gate: human. Inspect actual dependency/import and credential reachability, private data and
count isolation, current authority and unknown outcomes. Tests cannot authorize provisioning
credentials, switching a managed writer, answering a human decision or changing a merge policy.
