---
brief: "assay:assay:desk-improvement:07"
title: "Capture workflow attempt and decision evidence"
why: "Workflow review, verification and repair outcomes need exact subject-bound evidence, including waits, failed attempts and cancellation, beyond the runner process result."
wave: 3
depends: ["desk-improvement/05", "desk-improvement/06"]
unblocks: ["desk-improvement/10"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "c31496ce-3434-4797-bad7-09290805946d"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["workflow/controller/evidence.go: follow-up desk-improvement/07", "workflow/controller/evidence_test.go: follow-up desk-improvement/07", "workflow/testdata/evidence/: follow-up desk-improvement/07", "workflow/README.md: follow-up desk-improvement/07"]
gates: [{"on": "graph-execution/21", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}]
---

# Brief 07 — Capture workflow attempt and decision evidence

## Context

files:
- `workflow/controller/evidence.go (planned adapter in upstream controller)`
- `workflow/controller/evidence_test.go (planned)`
- `workflow/testdata/evidence/ (planned synthetic/disposable fixtures)`
- `workflow/README.md (upstream planned documentation)`
- `changelog/desk-improvement-07.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: workflow controller and canonical work/attempt/decision store
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

1. Bind the actual /21 controller's work/instance/node/attempt, intent/acceptance/candidate revisions, admission reason, waits, findings/dispositions, effect/decision receipts and terminal/nonterminal outcomes to /05 and /06 evidence. Preserve one original unit through repair and replacement PRs. Process success cannot create acceptance.
2. Export after authoritative native transitions with stable source IDs, explicit occurrence/observation precision and reconciliation checkpoints. No assumed atomic commit across journals. Measurement delivery failures must not repeat the transition/effect or change the controller's scheduling/acceptance policy.
3. Test mid-transition restart, stale acceptance, unresolved human/external wait, failed/no-PR/abandoned attempts, duplicate receipt and late adjudication. Use real producer code over disposable/fake external boundaries; verify normalized counts and retained uncertainty against native records.
4. Document mode/profile/reducer compatibility and coverage; versioned report/replay/rollback preserves prior evidence. Keep privacy filtering separate from source validation; no prompts, credentials or unrestricted human-act pointers enter publication.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd workflow && go test -v ./controller/... -run '^TestWorkEvidenceFlow$' > "${TMPDIR:-/tmp}/di-07-TestWorkEvidenceFlow.out" && grep -F -- '--- PASS: TestWorkEvidenceFlow' "${TMPDIR:-/tmp}/di-07-TestWorkEvidenceFlow.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd workflow && go test -v ./controller/... -run '^TestWorkEvidenceReplay$' > "${TMPDIR:-/tmp}/di-07-TestWorkEvidenceReplay.out" && grep -F -- '--- PASS: TestWorkEvidenceReplay' "${TMPDIR:-/tmp}/di-07-TestWorkEvidenceReplay.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd workflow && go test -v ./controller/... -run '^TestWorkEvidencePrivacy$' > "${TMPDIR:-/tmp}/di-07-TestWorkEvidencePrivacy.out" && grep -F -- '--- PASS: TestWorkEvidencePrivacy' "${TMPDIR:-/tmp}/di-07-TestWorkEvidencePrivacy.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/07` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
