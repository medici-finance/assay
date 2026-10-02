---
brief: assay:assay:iso-9001:08
title: Versioned source obligations and project applicability
why: Projects need to know which source revision a requirement came from and who decided it applies. A citation in prose cannot reliably reopen the right reviews when the source or project scope changes.
wave: 0
depends: []
unblocks:
- iso-9001/09
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
- 'statusgen: fixed-here'
- 'spec: fixed-here'
- 'schemas: fixed-here'
- 'docs/iso9001-mapping.md: fixed-here'
- 'docs/evidence-bundle.md: out-of-scope (this brief defines upstream source records)'
version: 1
id: 7722e206-2451-45b4-b077-5c6f17ecf540
---

# Brief 08 — Versioned source obligations and project applicability

## Context

files: `spec/project-obligations-v1.md` (planned), `schemas/project-obligations-v1.json` (planned), `statusgen/projectobligations.go` (planned), `statusgen/projectobligations_test.go` (planned), `statusgen/testdata/projectobligations/` (planned), `statusgen/decisiongateanchor.go` (existing offline corroboration seam; reuse), `statusgen/decisionruling.go` (existing ruling provenance; reuse), `spec/registers-v1.md` (existing contract; extend), `docs/iso9001-mapping.md` (existing contract; extend), `changelog/iso-9001-08-source-applicability.md` (planned)

facts: At the source baseline, statusgen/requirements.go defines ID, acceptance and satisfied-by fields; there is no source-revision or project-applicability decision contract. Existing requirements remain the integration seam. This brief can run offline with synthetic sources independently of graph-execution/15.

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

1. Implement the source-revision, obligation-mapping and applicability-decision contract in specification sections 3–4. Keep the existing REQ lifecycle and parser authoritative; the new mapping references REQ IDs and dereferences them. Extend the existing register specification with the link contract, without adding a second requirement state machine.
2. Accept only local, explicitly supplied sources. Record access/permitted-use references before content becomes eligible for model input; unknown or denied use permits authorized metadata only. Keep licensed text out of fixtures and public records. An external citation and human-authored interpretation are valid inputs where AI processing is not permitted.
3. Separate candidate analysis from authorized applicability. Reuse DECISIONS records and the offline corroboration seam in `statusgen/decisiongateanchor.go`, with provenance rules documented by `statusgen/decisionruling.go`; accept only an explicitly approved disposition whose interpretation has been reviewed. Corroboration establishes who/where, not whether prose approves or rejects. With no trusted corroboration receipt, applicability remains unresolved; this loader must not fetch the forge. Bind the accepted decision bound to source, mapping and project/profile revisions. Specify the mapping's acceptance link as the existing decision ID plus subject digest, explicit approved disposition, trusted corroboration reference and qualified-review reference; bind source/mapping/profile revisions in that subject digest. These are references to existing decision/review records, not new approvals. A supplied JSON object is not itself trusted corroboration. A typed name, model output or copied approval text is insufficient. Unresolved or conflicting applicability cannot be silently excluded.
4. Preserve prior revisions and supersession, including not-applicable decisions and reasons. Validate mappings against existing requirement entries and reject unknown references for this new review contract without changing unrelated legacy lint behavior.
5. Add named tests below through the production loader/validator. Prove the lower decision-validation boundary rejects forged acceptance even when the candidate-input validator is bypassed. Document the distinction between source identity, permitted use and semantic correctness.

## Interface contract

Permitted source revisions + authenticated project applicability decisions → mappings referencing existing REQ IDs; unresolved or unauthorized inputs cannot produce accepted mappings.

## Verify

These commands are future implementation obligations. No execution evidence is asserted by authoring. The named test must exist and execute; a zero-test exit is not evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceSourceIdentityAndPermissions$" .` | exit 0; named TestProjectAssuranceSourceIdentityAndPermissions executes, with no [no tests to run]; A1: stable revisions and metadata-only input; denied AI use never enters model payload |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceApplicabilityAuthority$" .` | exit 0; named TestProjectAssuranceApplicabilityAuthority executes, with no [no tests to run]; A2: forged identity, stale decision and bypass of candidate validation are rejected |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceRequirementDereference$" .` | exit 0; named TestProjectAssuranceRequirementDereference executes, with no [no tests to run]; A3: a real fixture REQ resolves; dangling REQ fails through the production parser |
| 4 | check:ci +neighbour | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestRequirementValidEntryIsClean$" .` | exit 0; existing requirements without new mappings still parse unchanged |

## Pre-mortem and detection

- Plausible but unsupported outcome: rows 1–2 exercise production inputs and independent expected records.
- Silent omission or stale identity: row 3 exercises the named refusal/qualification boundary.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
