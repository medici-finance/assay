---
brief: assay:assay:iso-9001:11
title: Qualify project assurance preparation on an offline corpus
why: A well-formed review packet can still make unsupported claims or miss the useful next action. A frozen task corpus and independently reviewed rubric make those limits visible before a project relies on the procedure.
wave: 10
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
- 'statusgen: follow-up iso-9001/11'
- 'docs/assurance-review-qualification.md: follow-up iso-9001/11'
- 'spec: out-of-scope (qualification consumes the approved contracts)'
version: 1
id: 57bfe550-f444-44d6-bc93-4f31cd8cbae3
---

# Brief 11 — Qualify project assurance preparation on an offline corpus

## Context

files: `statusgen/projectreview_qualification_test.go` (planned), `statusgen/testdata/projectreview/qualification/` (planned), `docs/assurance-review-qualification.md` (planned), `docs/streams/iso-9001/qualification-report.md` (planned), `changelog/iso-9001-11-assurance-review-qualification.md` (planned)

facts:
- corpus: the public task corpus is synthetic and offline
- provider: a stub candidate-analysis provider only; no vendor or live infrastructure call
- claim-scope: fixture success establishes contract boundaries, not vendor performance or a compliance assessment
- motivation: published work-product evaluations of legal AI agents motivate the protocol; no benchmark score or unrun vendor comparison counts as qualification here

design-fit:
  owner: the production paths of 08–10; this brief adds only a corpus, a test and a report
  contract: none — qualification consumes the approved contracts and defines no shared meaning
  retires: []
  weight: verbs 0, flags 0, refusals 0, rule-text lines 0
  why-add: n/a (no ratcheted growth)

single-point-of-failure: the independent expected-population check, the one place a dropped case is caught. Behind it, each case's expected outcome is stored apart from generated packets (row 1), and the unsupported-claims check fails on fabricated citations whatever the summary says (row 2).

## Read first

- [Project assurance specification](project-assurance-spec.md), especially sections 3–6.
- [Control exports](../graph-execution/brief-15-control-evidence.md) and [existing evidence contracts](../../evidence-bundle.md).
- [Requirements specification](../../../spec/registers-v1.md).

## Human decision

Decision-trigger: spec. At pickup, prepare the concrete contract and negative-path evidence, then record the owner decision before activating the behavior for an adopting project. No response authorizes activation. Synthetic implementation and review may proceed within this brief.

## Ground rules

- Never git push, trigger workflows or run mutating infrastructure commands unless explicitly instructed. Feature branch and draft PR only; no merge, deployment, external provider or live infrastructure access.
- Keep the stream's parked state; prioritization is a separate owner decision.
- Public examples and fixtures are synthetic. No licensed normative text or adopter records.
- Stop at implemented; independent verification and normal review own later states.
- Unknown or missing evidence never becomes a pass. Required upstream behavior must be independently verified before operational reliance.
- If an instruction is unclear or contradicts the repository state, report NEEDS_CONTEXT rather than guess.

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
| 1 | check:ci +flow +dereference | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceQualCorpus$" .` | exit 0; named TestAssuranceQualCorpus executes, with no [no tests to run]; A1–A12 enumerated and exercised by production paths; a dropped case fails the independent expected-population check |
| 2 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceUnsupported$" .` | exit 0; named TestAssuranceUnsupported executes, with no [no tests to run]; A10: fabricated citations and missing organizational acts stay unsupported despite plausible summaries |
| 3 | check:ci +mutation | `cd statusgen && GOWORK=off go test -count=1 -v -run "^TestAssuranceProviderChange$" .` | exit 0; named TestAssuranceProviderChange executes, with no [no tests to run]; A11–A12: results preserve baseline/assisted identity and do not inherit qualification after provider change |

## Pre-mortem and detection

- A dropped or silently skipped case: row 1 fails the independent expected-population check.
- A plausible conclusion with a fabricated citation or missing organizational act: row 2.
- Qualification inherited after a provider change: row 3.
- Semantic adequacy of an interpretation: review-only; human adjudication recorded as a real act.

## Evidence

<!-- No implementation or independent verification is claimed. Append actual runs at implementation and verification. -->

## Review

Gate: human. Confirm source rights, applicability/authority binding, consumer compatibility and honest completeness claims. Review semantic adequacy separately from mechanical evidence.
