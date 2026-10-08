---
brief: assay:assay:iso-9001:10
title: Reassess affected project reviews after source changes
why: An old review can remain apparently current after its governing source or scope changes. Projects need a bounded impact list and a new applicability decision without erasing what the earlier evidence actually proved.
wave: 9
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
- 'statusgen: follow-up iso-9001/10'
- 'spec: follow-up iso-9001/10'
- 'docs/evidence-bundle.md: follow-up iso-9001/10'
version: 2
id: bb775839-aa6d-4957-a872-3ae70dfef0ff
---

# Brief 10 — Reassess affected project reviews after source changes

## Context

files: `statusgen/projectimpact.go` (planned), `statusgen/projectimpact_test.go` (planned), `statusgen/testdata/projectimpact/` (planned), `statusgen/main.go` (existing contract; extend), `spec/project-obligations-v1.md` (planned), `spec/project-assurance-review-v1.md` (planned), `docs/evidence-bundle.md` (existing contract; extend), `changelog/iso-9001-10-source-change-impact.md` (planned)

facts:
- inputs: the source/applicability and packet contracts are outputs of 08–09, not files shipped at authoring
- coverage-owner: generic graph coverage (graph-execution/03) owns technical revision checks; this brief adds source-change reverse links only
- triggering: change records are supplied explicitly; no polling, web fetch or subscription
- history: historical packets, decisions and evidence are never rewritten

layering: `domain-core`. The impact walk, deduplication key and hold derivation are pure over supplied change, mapping and review records; reading files and writing the impact record are the adapter. Task 2–5 name the boundary; rows 1–3 check it.

design-fit:
  owner: graph-execution/03 owns evidence revision coverage and iso-9001/08's records own the source-to-mapping link; this brief owns only the reverse-link impact record
  contract: S-decision-acceptance — no-impact and reassess dispositions reuse decision acceptance; no new acceptance check
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0 in the ratcheted `tools/desk` set; statusgen gains one CLI flag (`--assurance-impact`) outside it
  why-add: an impact walk needs the reverse of the 08 mapping, which neither the coverage rule (evidence freshness) nor the packet reader (one subject) computes. Making the coverage rule source-aware was considered and rejected because it would give coverage a second meaning

single-point-of-failure: exact-subject binding of a disposition, the one check that stops a decision for old source bytes or another project from clearing a hold. Behind it, the walk reports could-not-check on unknown reverse links instead of an empty set (row 3), and replayed changes keep one impact identity (row 2).

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
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceSelectiveImpact$" .` | exit 0; named TestAssuranceSelectiveImpact executes, with no [no tests to run]; A6: two affected project mappings hold, unrelated mapping and historical packet bytes unchanged |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceImpactReplay$" .` | exit 0; named TestAssuranceImpactReplay executes, with no [no tests to run]; A7: duplicate source events keep one identity; late and wrong-subject approvals cannot clear new holds |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceUnknownImpact$" .` | exit 0; named TestAssuranceUnknownImpact executes, with no [no tests to run]; unreadable source links and unknown reverse references return could-not-check, never a clean empty set |

## Pre-mortem and detection

- Over- or under-propagation: row 1 holds exactly the two affected mappings and leaves the unrelated mapping and historical packet unchanged.
- Duplicate events, or a late or wrong-subject approval clearing a hold: row 2.
- A silent empty impact set on unreadable links: row 3.
- Materiality misjudged: review-only; materiality is human judgment, not a diff or confidence threshold.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
