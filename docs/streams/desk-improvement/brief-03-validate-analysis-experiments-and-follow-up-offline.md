---
brief: "assay:assay:desk-improvement:03"
title: "Validate analysis experiments and follow-up offline"
why: "Experiments must distinguish actual benefit from missing attempts, leaked answers or a favorable window, and follow-up must remain bound to the original decision."
wave: 1
depends: ["desk-improvement/01"]
unblocks: ["desk-improvement/04", "desk-improvement/09"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "7e965aa7-804f-4e35-9f03-5549e8e9a77d"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["qualgen/deskquality/analysis.go: follow-up desk-improvement/03", "qualgen/deskquality/experiment.go: follow-up desk-improvement/03", "qualgen/deskquality/corpus.go: follow-up desk-improvement/03", "qualgen/deskquality/followup.go: follow-up desk-improvement/03", "qualgen/deskquality/experiment_test.go: follow-up desk-improvement/03", "qualgen/deskquality/testdata/experiments/: follow-up desk-improvement/03", "qualgen/desks.go: follow-up desk-improvement/03", "docs/desk-improvement.md: follow-up desk-improvement/03", "schemas/desk-validation-v1.json: follow-up desk-improvement/03"]
gates: [{"on": "continuing-operations/02", "type": "build-dep", "reason": "Reuse the canonical operational subject or observation-obligation contract; do not invent another identity or follow-up ledger."}]
---

# Brief 03 — Validate analysis experiments and follow-up offline

## Context

files:
- `schemas/desk-validation-v1.json` (planned)
- `qualgen/deskquality/analysis.go (planned)`
- `qualgen/deskquality/experiment.go (planned)`
- `qualgen/deskquality/corpus.go (planned)`
- `qualgen/deskquality/followup.go (planned)`
- `qualgen/deskquality/experiment_test.go (planned)`
- `qualgen/deskquality/testdata/experiments/ (planned synthetic cases)`
- `qualgen/desks.go (01/02 CLI adapter; add validation operation)`
- `docs/desk-improvement.md (planned usage/reference protocol)`
- `changelog/desk-improvement-03.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: continuing operations assessment protocol, implemented as pure qualgen validation
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

1. Define the validation response schema (valid, readiness qualified/held/partial, reasons and subject/input/version bindings). Qualified means structurally ready, never authorized or independently true. Implement reusable validation and templates for diagnosis, frozen corpus/source hashes, protocol, run manifest, independent assessment, decision binding and follow-up. Reference canonical event/work IDs; no new trial queue or independent record authority. Pure validation consumes explicit data, never fetches private sources or invokes a model.
2. A protocol fixes hypothesis/competing explanation, decision, eligibility/assignment, original context, development/holdout family split, conditions, evaluator, positive/clean-control labels, practical benefit/quality margins, uncertainty/sample rationale, scope/retention, budgets, stops and horizon before outcomes. Unknown required limits hold execution; structural validation cannot establish permission or label truth.
3. Account for every assigned attempt, timeout, interruption, partial usage and deviation; show assignment and completed-pair views. Reject selective reruns, shared condition context, later answers, post-outcome thresholds and a premature follow-up. Keep supported/adverse/insufficient/invalid conclusions distinct. A no-change decision may complete a cycle without fabricated adoption exposure.
4. Expose `qualgen desks validate --kind analysis|corpus|experiment|followup --input FILE` (planned) through the existing desks adapter and document the reference delta-versus-full rereview protocol. Replay measures detection/resource use, not live convergence or avoided incidents. An adopter owns real cases, provider calls and the independently assessed outcome; no reference corpus or runtime completion is claimed by this brief.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -v ./deskquality/... -run '^TestExperimentScope$' > "${TMPDIR:-/tmp}/di-03-TestExperimentScope.out" && grep -F -- '--- PASS: TestExperimentScope' "${TMPDIR:-/tmp}/di-03-TestExperimentScope.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestExperimentAttempts$' > "${TMPDIR:-/tmp}/di-03-TestExperimentAttempts.out" && grep -F -- '--- PASS: TestExperimentAttempts' "${TMPDIR:-/tmp}/di-03-TestExperimentAttempts.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestFollowupBinding$' > "${TMPDIR:-/tmp}/di-03-TestFollowupBinding.out" && grep -F -- '--- PASS: TestFollowupBinding' "${TMPDIR:-/tmp}/di-03-TestFollowupBinding.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/03` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
