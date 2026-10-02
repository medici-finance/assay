---
brief: assay:assay:iso-9001:11
title: Qualify project assurance preparation on an offline corpus
why: A well-formed review packet can still make unsupported claims or miss the useful next action. A frozen task corpus and independently reviewed rubric make those limits visible before a project relies on the procedure.
wave: 7
depends:
- iso-9001/10
unblocks: []
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
- 'docs/assurance-review-qualification.md: fixed-here'
- 'spec: out-of-scope (qualification consumes the approved contracts)'
version: 1
id: 57bfe550-f444-44d6-bc93-4f31cd8cbae3
---

# Brief 11 — Qualify project assurance preparation on an offline corpus

## Context

files: `statusgen/projectreview_qualification_test.go` (planned), `statusgen/testdata/projectreview/qualification/` (planned), `docs/assurance-review-qualification.md` (planned), `docs/streams/iso-9001/qualification-report.md` (planned), `changelog/iso-9001-11-assurance-review-qualification.md` (planned)

facts: The public task corpus is synthetic. LAB-style work-product evaluation motivates the protocol, but no legal benchmark score or unrun vendor comparison counts as qualification for this procedure.

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

1. Freeze synthetic source, project, control-export and organizational-record fixtures covering specification A1–A12, including positive controls. Store expected outcomes separately from generated packets; enumerate the expected case population and reject a missing case.
2. Execute the actual source loader, canonical export reader, review preparation and impact paths together. Use a stub candidate-analysis provider to inject fabricated citations, claimed meetings, source instructions and overconfident conclusions. No vendor or live infrastructure call is permitted.
3. An independent rubric separates mechanical refusal from semantic adequacy. Mechanically detect missing/incorrect provenance and fabricated source references; human reviewers assess whether an interpretation or citation actually supports its claim. Record any manual adjudication as a real act, never a generated signature.
4. Produce a report with case manifest/digests, required versus included population, observed verdicts, known-answer misses and limits. Document an adopter evaluation protocol comparing the same cases under manual, existing-agent and optional approved-provider conditions. Measure completion, abstention, missed obligations, unsupported conclusions, correction effort, analyst minutes and full cost; include failures in denominators.
5. Require owners to predeclare semantic acceptance thresholds before a real trial. A new model/provider or material playbook change requires explicit requalification scope. Public deterministic fixture success establishes contract boundaries only; it is not vendor performance, a compliance assessment, or approval to process project data.

## Interface contract

Frozen expected corpus + actual production review paths → reproducible boundary results and an adopter evaluation protocol; no fabricated semantic approval.

## Verify

These commands are future implementation obligations. No execution evidence is asserted by authoring. The named test must exist and execute; a zero-test exit is not evidence.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceQualificationCorpus$" .` | exit 0; named TestProjectAssuranceQualificationCorpus executes, with no [no tests to run]; A1–A12 enumerated and exercised by production paths; a dropped case fails the independent expected-population check |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceUnsupportedClaims$" .` | exit 0; named TestProjectAssuranceUnsupportedClaims executes, with no [no tests to run]; A10: fabricated citations and missing organizational acts stay unsupported despite plausible summaries |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestProjectAssuranceProviderChange$" .` | exit 0; named TestProjectAssuranceProviderChange executes, with no [no tests to run]; A11–A12: results preserve baseline/assisted identity and do not inherit qualification after provider change |

## Pre-mortem and detection

- Plausible but unsupported outcome: rows 1–2 exercise production inputs and independent expected records.
- Silent omission or stale identity: row 3 exercises the named refusal/qualification boundary.
- Semantically wrong but correctly cited interpretation: review-only; the qualified reviewer must inspect the source and record disagreement. A presence check cannot settle it.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
