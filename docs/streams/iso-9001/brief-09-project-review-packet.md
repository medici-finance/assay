---
brief: assay:assay:iso-9001:09
title: Prepare project assurance reviews from canonical evidence
why: Evidence exports still leave a reviewer to reconstruct applicability, omissions and required acts. A repeatable preparation procedure can reduce that effort while keeping machine evidence, model assessments and human decisions distinct.
wave: 5
depends:
- iso-9001/08
- iso-9001/03
- iso-9001/04
- graph-execution/02
- graph-execution/03
- graph-execution/15
unblocks:
- iso-9001/10
effort: M
gate: human
risk:
  regulatory: 'yes'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
gate-why: The accountable owner reviews applicability, evidence boundaries and permitted source use; an automated preparation result must not become a conformity or authorization claim.
decision-trigger: spec
issues: []
schema: brief-v2
authored: 2026-10-03 by Codex (author-brief)
sources:
- docs/streams/iso-9001/project-assurance-spec.md
- freshness-checked 2026-10-03 @ cf31c32418ba49f93c679913813768542db1c072
exec-tier: strong
exec-tier-why: Cross-artifact authority, version applicability and evidence completeness must agree across independent readers.
domain: complicated
consumers:
- 'statusgen: follow-up iso-9001/09'
- 'spec: follow-up iso-9001/09'
- 'docs/evidence-bundle.md: follow-up iso-9001/09'
version: 2
id: 8e2dce79-e393-4c6c-88fb-f31aaa6b4cbe
---

# Brief 09 — Prepare project assurance reviews from canonical evidence

## Context

files: `spec/project-assurance-review-v1.md` (planned), `spec/workflow-patterns/project-assurance-v1.yaml` (planned), `statusgen/projectreview.go` (planned), `statusgen/projectreview_test.go` (planned), `statusgen/testdata/projectreview/` (planned), `statusgen/main.go` (existing contract; extend), `docs/evidence-bundle.md` (existing contract; extend), `spec/control-assurance-v1.md` (planned), `changelog/iso-9001-09-project-review-packet.md` (planned)

facts: At the source baseline, statusgen/auditpack.go exists and its tests check omitted backing evidence. graph-execution/15 names spec/control-assurance-v1.md and statusgen/controlassurance.go as planned; do not implement their substitutes while that dependency remains open.

single-point-of-failure: trusting the candidate analysis would allow a plausible summary to impersonate evidence. The input permission/authority boundary and an independent packet/fixture reader must fail on different evidence, with negative tests of each.

## Read first

- [Project assurance specification](project-assurance-spec.md), especially sections 3–6.
- [Control exports](../graph-execution/brief-15-control-evidence.md) and [existing evidence contracts](../../evidence-bundle.md).
- [Requirements specification](../../../spec/registers-v1.md).

## Human decision

Decision-trigger: spec. At pickup, prepare the concrete contract and negative-path evidence, then record the owner decision before activating the behavior for an adopting project. No response authorizes activation. Synthetic implementation and review may proceed within this brief.

## Ground rules

- Isolated branch and draft PR; no merge, deployment, external provider or live infrastructure access.
- Keep the stream's parked state; prioritization is a separate owner decision.
- Public examples and fixtures are synthetic. No licensed normative text or adopter records.
- Stop at implemented; independent verification and normal review own later states.
- Unknown or missing evidence never becomes a pass. Required upstream behavior must be independently verified before operational reliance.

## Task

1. Add the offline review preparation entry point `statusgen --prepare-assurance-review <request.json> --root <root> --out <directory>` and its documented request/packet contract. Reuse the actual control profile/export delivered by graph-execution/15 and auditpack collector output; do not clone either collector. Resolve section 3's source/applicability links against those canonical records.
2. Emit a factual packet without a model. Optional bounded analysis is a supplied candidate artifact with provider/template/input-manifest provenance, never authority. Output separate facts, assessments, conflicts, omissions and required acts. Unknown population, a missing mandatory record or wrong revision must remain incomplete. Define and test explicit exit/result semantics in the contract.
3. Add a declarative pattern using the existing artifact/check/decision kinds and an independent integration join. Validate with the production pattern parser. The pattern has no network or deployment effects and adds no scheduler. No observe claim is invented for a document review.
4. Bind the reader's packet-completeness result to the request and packet digests and to authorized source decisions. Keep a separately attributed semantic review assessment; neither result changes control verdicts, determines conformity or grants authority. Preserve release-authorizer identity and effectiveness references from existing mechanisms. A merged correction with no required effectiveness record remains unresolved; a model summary cannot supply an organizational act.
5. Enforce permission filtering before candidate analysis and independently recheck the resulting packet with the canonical population/provenance reader. Test an unauthorized payload offered directly to the lower reader, malicious instructions in a document, and forged citations. Missing metadata that policy forbids revealing is reported in an authorized aggregate form.
6. Emit proposed action references only. Do not send notifications, create issues, clear release gates or persist alternative control verdicts. Preserve historical exports and include concrete offline reader instructions.

## Interface contract

Versioned request + accepted mappings + canonical exports → independently readable scoped packet; incomplete facts remain incomplete even with a persuasive analysis.

## Verify

These commands are future implementation obligations. No execution evidence is asserted by authoring. The named test must exist and execute; a zero-test exit is not evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssurancePacketFlow$" .` | exit 0; named TestProjectAssurancePacketFlow executes, with no [no tests to run]; A4: request → canonical exports → independently read packet; missing population and wrong revision remain incomplete |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssurancePermissionBoundary$" .` | exit 0; named TestProjectAssurancePermissionBoundary executes, with no [no tests to run]; A5: denied payload and embedded instructions cannot cross the export/decision boundary, including direct reader bypass |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceEffectivenessAndNoModel$" .` | exit 0; named TestProjectAssuranceEffectivenessAndNoModel executes, with no [no tests to run]; A8–A9: correction remains open without effectiveness; no-model factual packet works |
| 4 | check:ci +neighbour | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAuditPackCoverageAgreementCatchesDroppedBacking$" .` | exit 0; existing independent audit-pack omission detection still executes |

## Pre-mortem and detection

- Plausible but unsupported outcome: rows 1–2 exercise production inputs and independent expected records.
- Silent omission or stale identity: row 3 exercises the named refusal/qualification boundary.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
