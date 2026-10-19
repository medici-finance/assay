---
brief: "assay:assay:desk-improvement:01"
title: "Define the reusable scorecard and experiment contracts"
why: "Different adapters must calculate the same cohort and expose the same unknowns before a desk comparison can support a decision."
wave: 0
depends: []
unblocks: ["desk-improvement/02", "desk-improvement/03", "desk-improvement/05"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 2
id: "769bec2d-8a6b-4c87-a355-d5f78f669d29"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["docs/streams/desk-improvement/spec.md: follow-up desk-improvement/01", "schemas/desk-scorecard-v1.json: follow-up desk-improvement/01", "schemas/desk-experiment-v1.json: follow-up desk-improvement/01", "qualgen/deskquality/contract.go: follow-up desk-improvement/01", "qualgen/deskquality/contract_test.go: follow-up desk-improvement/01", "qualgen/deskquality/testdata/contracts/: follow-up desk-improvement/01", "docs/contracts.md: follow-up desk-improvement/01"]
gates: [{"on": "continuing-operations/01", "type": "build-dep", "reason": "Reuse the canonical operational subject or observation-obligation contract; do not invent another identity or follow-up ledger."}]
---

# Brief 01 — Define the reusable scorecard and experiment contracts

## Context

files:
- `docs/streams/desk-improvement/spec.md (existing canonical design; refine versioned contract)`
- `schemas/desk-scorecard-v1.json (planned)`
- `schemas/desk-experiment-v1.json (planned)`
- `qualgen/deskquality/contract.go (planned)`
- `qualgen/deskquality/contract_test.go (planned)`
- `qualgen/deskquality/testdata/contracts/ (planned synthetic vectors)`
- `docs/contracts.md (existing semantic-owner index)`
- `changelog/desk-improvement-01.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: continuing operations/quality assessment contracts
  contract: none — desk-improvement/01 adds the assessment row to the existing semantic-owner index
  retires: []
  weight: "CLI modes +0; policy refusals +0; rule-text +0 shipped desk method"
  why-add: "Existing source/assessment gaps need one owned extension; a duplicate collector, rule set or execution ledger was rejected."

Single-point-of-failure note: validation cannot establish truth or authority; independent native-source/label assessment backs structural checks. Neither layer may grant acceptance or policy adoption.

## Read first

[Canonical specification](spec.md), [plan and prerequisites](README.md), and the exact upstream owner contracts named in gates. Their required boundaries are restated in Task. Planned paths/tests below are deliverables, not claims that implementation exists.

## Ground rules

- Isolated branch and draft PR; no main push, merge, workflow dispatch, production endpoint or cluster contact.
- Offline synthetic/disposable fixtures; no model spend, real-data export or enrollment is granted here.
- Preserve completed Evidence and current authority; stop at implemented for independent verification.

## Task

1. Freeze configurable role/profile definitions, five reference profiles and system view from spec §§3–4. Define eligible/observed/qualified counts, fixed admission, censoring, all-attempt cost, defect introduction/catch opportunity, zero/unknown/partial states and version compatibility. Preserve original work/attempt/subject IDs and the continuing-operations subject lineage.
2. Define analysis, protocol, assessment, authorized decision and follow-up bindings, including visibility profiles. Public source uses synthetic examples; restricted source pointers/act identities never enter the minimal publication projection. A report cannot grant permission, acceptance or adoption.
3. Register the assessment semantic owner in docs/contracts.md. Reconcile existing brief-quality and flow contract fields, and qualify the exported qualgen package versus typed-artifact consumer route. No tools/desk internal import, custody import or shipping sibling replace. This is the single canonical design; do not maintain a second manually copied specification.
4. Supply discriminating contract vectors: wrong rate denominator, incomplete cohort, zero accepted outcomes, unjudged finding, unknown usage, conflicting identity, forbidden publication field and stale protocol. An operator chooses real permissions and numeric experiment limits before execution; no adopter-specific trial is required for these definitions.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -v ./deskquality/... -run '^TestCardContract$' > "${TMPDIR:-/tmp}/di-01-TestCardContract.out" && grep -F -- '--- PASS: TestCardContract' "${TMPDIR:-/tmp}/di-01-TestCardContract.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestProtocolBinding$' > "${TMPDIR:-/tmp}/di-01-TestProtocolBinding.out" && grep -F -- '--- PASS: TestProtocolBinding' "${TMPDIR:-/tmp}/di-01-TestProtocolBinding.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestPublishBoundary$' > "${TMPDIR:-/tmp}/di-01-TestPublishBoundary.out" && grep -F -- '--- PASS: TestPublishBoundary' "${TMPDIR:-/tmp}/di-01-TestPublishBoundary.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/01` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
