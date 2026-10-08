---
brief: assay:assay:iso-9001:09
title: Prepare project assurance reviews from canonical evidence
why: Evidence exports still leave a reviewer to reconstruct applicability, omissions and required acts. A repeatable preparation procedure can reduce that effort while keeping machine evidence, model assessments and human decisions distinct.
wave: 8
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

facts:
- auditpack: `statusgen/auditpack.go` exists at the source baseline and its tests detect omitted backing evidence (row 4)
- control-export: graph-execution/15 names `spec/control-assurance-v1.md` (planned) and `statusgen/controlassurance.go` (planned) as its deliverables; no substitute is built here while that dependency is open
- source-links: source and applicability records come from iso-9001/08
- authority: the packet grants no merge, release or vendor-upload authority and sends no notifications

layering: `domain-core`. Packet assembly and completeness rules are pure over explicit inputs (request, canonical exports, decisions, optional candidate analysis); file I/O and the CLI flag are the adapter, and the independent packet reader is a separate check over the emitted packet. Task 2, 4 and 5 name the boundary; rows 1–2 check it.

design-fit:
  owner: graph-execution/15's control export owns evidence meaning and `statusgen/auditpack.go` owns release collection; this brief owns only the review-request and packet contract
  contract: S-decision-acceptance for the source decisions it resolves; none for the packet itself — the semantic-owner index has no review-preparation row, and this brief adds no second owner of an indexed meaning
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0 in the ratcheted `tools/desk` set; statusgen gains one CLI flag (`--prepare-assurance-review`) outside it
  why-add: the flag is the offline entry point for packet preparation. auditpack and the control export each collect one population and neither joins source applicability to it; folding the join into either would make it a second owner of review semantics. A separate reader over their unchanged outputs was chosen over adding a mode to the existing `--export-audit-pack` flag

single-point-of-failure: the permission filter in front of candidate analysis. Behind it, the canonical population/provenance reader re-checks the emitted packet on its own evidence (row 2 offers an unauthorized payload directly to that reader with the filter bypassed), and the completeness result stays non-success on a missing population or wrong revision (row 1).

## Read first

- [Project assurance specification](project-assurance-spec.md), especially sections 3–6.
- [Control exports](../graph-execution/brief-15-control-evidence.md) and [existing evidence contracts](../../evidence-bundle.md).
- [Requirements specification](../../../spec/registers-v1.md).

## Human decision

Decision-trigger: spec. At pickup, prepare the concrete contract and negative-path evidence, then record the owner decision before activating the behavior for an adopting project. No response authorizes activation. Synthetic implementation and review may proceed within this brief.

## Ground rules

- Never git push, trigger workflows or run mutating infrastructure commands unless explicitly instructed. Feature branch and draft PR only; no merge, deployment, external provider or live infrastructure access.
- Preserve the owner-set stream status and priority; reprioritization is a separate owner decision.
- Public examples and fixtures are synthetic. No licensed normative text or adopter records.
- Stop at implemented; independent verification and normal review own later states.
- Unknown or missing evidence never becomes a pass. Required upstream behavior must be independently verified before operational reliance.
- If an instruction is unclear or contradicts the repository state, report NEEDS_CONTEXT rather than guess.

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
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssurancePacketFlow$" .` | exit 0; named TestAssurancePacketFlow executes, with no [no tests to run]; A4: request → canonical exports → independently read packet; missing population and wrong revision remain incomplete |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssurancePermissionBoundary$" .` | exit 0; named TestAssurancePermissionBoundary executes, with no [no tests to run]; A5: denied payload and embedded instructions cannot cross the export/decision boundary, including direct reader bypass |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceEffectNoModel$" .` | exit 0; named TestAssuranceEffectNoModel executes, with no [no tests to run]; A8–A9: correction remains open without effectiveness; no-model factual packet works |
| 4 | check:ci +neighbour | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAuditPackCoverageAgreementCatchesDroppedBacking$" .` | exit 0; existing independent audit-pack omission detection still executes |

## Pre-mortem and detection

- Plausible but incomplete packet: row 1 compares it with an independently enumerated population and fails on a missing record or wrong revision.
- Unauthorized content or embedded instructions crossing into analysis: row 2, including a direct bypass of the filter.
- Correction reported closed without effectiveness, or no packet when the model is absent: row 3.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
