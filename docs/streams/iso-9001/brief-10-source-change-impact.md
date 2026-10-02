---
brief: assay:assay:iso-9001:10
title: Reassess affected project reviews after source changes
why: An old review can remain apparently current after its governing source or scope changes. Projects need a bounded impact list and a new applicability decision without erasing what the earlier evidence actually proved.
wave: 6
depends:
- iso-9001/09
unblocks:
- iso-9001/11
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
- 'docs/evidence-bundle.md: fixed-here'
version: 1
id: bb775839-aa6d-4957-a872-3ae70dfef0ff
---

# Brief 10 — Reassess affected project reviews after source changes

## Context

files: `statusgen/projectimpact.go` (planned), `statusgen/projectimpact_test.go` (planned), `statusgen/testdata/projectimpact/` (planned), `statusgen/main.go` (existing contract; extend), `spec/project-obligations-v1.md` (planned), `spec/project-assurance-review-v1.md` (planned), `docs/evidence-bundle.md` (existing contract; extend), `changelog/iso-9001-10-source-change-impact.md` (planned)

facts: The source/applicability and packet contracts are the outputs of 08–09, not files already shipped at authoring. Generic graph coverage owns technical revision checks; this brief adds source-change reverse links and scoped reassessment.

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

1. Add the offline entry point `statusgen --assurance-impact <change.json> --root <root> --out <directory>`. It accepts an explicitly supplied old/new source or profile revision, including discovery and effective dates. No polling, web fetch or automatic subscription is introduced.
2. Walk mapping → REQ/control → review references and emit stable impact identities with current-applicability holds and proposed actions. Unknown or unreadable links produce could-not-check with bounded coverage, never zero impact. Reuse canonical revision coverage for evidence; this code contributes source applicability, not a competing evidence freshness evaluator.
3. Preserve historical packets, their source revisions, decisions and evidence. Scope propagation to the affected project/profile and effective period; a matching title is insufficient. Retrospective effective dates trigger explicit review questions, not silent rewriting of history.
4. An authorized exact-subject disposition can accept no-impact or require reassessment. Enforce decision binding and supersession; a decision for old source bytes or another project cannot clear the hold. Materiality is human judgment, not a string-diff or confidence threshold.
5. Deduplicate repeated change input and support restart from the exported impact record. Emit proposed work references only; the current intake/decision owner performs any external write. Test replay, late decisions and conflicting source interpretations through the production reader.

## Interface contract

Explicit source/profile change + existing mapping graph → scoped impact record and current-applicability holds; historical evidence stays immutable.

## Verify

These commands are future implementation obligations. No execution evidence is asserted by authoring. The named test must exist and execute; a zero-test exit is not evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceSelectiveImpact$" .` | exit 0; named TestProjectAssuranceSelectiveImpact executes, with no [no tests to run]; A6: two affected project mappings hold, unrelated mapping and historical packet bytes unchanged |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceImpactReplay$" .` | exit 0; named TestProjectAssuranceImpactReplay executes, with no [no tests to run]; A7: duplicate source events keep one identity; late and wrong-subject approvals cannot clear new holds |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceUnknownImpact$" .` | exit 0; named TestProjectAssuranceUnknownImpact executes, with no [no tests to run]; unreadable source links and unknown reverse references return could-not-check, never a clean empty set |

## Pre-mortem and detection

- Plausible but unsupported outcome: rows 1–2 exercise production inputs and independent expected records.
- Silent omission or stale identity: row 3 exercises the named refusal/qualification boundary.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
