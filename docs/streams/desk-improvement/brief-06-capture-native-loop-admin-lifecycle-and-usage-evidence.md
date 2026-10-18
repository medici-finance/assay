---
brief: "assay:assay:desk-improvement:06"
title: "Capture native loop-admin lifecycle and usage evidence"
why: "Native process, recovery and usage facts must be emitted by their runtime owner; a reducer cannot infer them from PRs or handwritten summaries."
wave: 2
depends: ["desk-improvement/02", "desk-improvement/05"]
unblocks: ["desk-improvement/07", "desk-improvement/08"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 2
id: "bd47632e-b4ad-4205-aef6-6ce7672eb5b1"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["loopadmin/supervisor/evidence.go: follow-up desk-improvement/06", "loopadmin/supervisor/evidence_test.go: follow-up desk-improvement/06", "loopadmin/journal/evidence.go: follow-up desk-improvement/06", "loopadmin/journal/migrations/: follow-up desk-improvement/06", "loopadmin/testdata/evidence/: follow-up desk-improvement/06", "loopadmin/README.md: follow-up desk-improvement/06", "docs/loop-admin.md: follow-up desk-improvement/06"]
gates: [{"on": "graph-execution/26", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}, {"on": "graph-execution/22", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}]
---

# Brief 06 — Capture native loop-admin lifecycle and usage evidence

## Context

files:
- `loopadmin/supervisor/evidence.go (planned adapter in upstream supervisor)`
- `loopadmin/supervisor/evidence_test.go (planned)`
- `loopadmin/journal/evidence.go (planned export/reconciliation hook)`
- `loopadmin/journal/migrations/ (upstream owner; compatible addition only if required)`
- `loopadmin/testdata/evidence/ (planned synthetic/disposable fixtures)`
- `loopadmin/README.md (existing)`
- `docs/loop-admin.md (upstream planned documentation)`
- `changelog/desk-improvement-06.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: loopadmin supervisor/journal and existing qualified runner adapter
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

1. Extend the actual graph-execution/26 supervisor/journal and /22 qualified adapter with 05's mapping. Persist/export native invocation/caller/mode/generation, profile/input digests, requested/resolved/observed model, lifecycle/time precision, usage completeness and stop/recovery facts. Preserve nullable/unknown usage; no unsupported cache/provider units or active time are inferred.
2. Use existing PostgreSQL journal transactions and source revision/watermark semantics. Observation export failure creates a coverage gap and reconciliation obligation, never a repeated launch/effect or a second execution ledger. Existing authority-critical journal failure still holds native execution under its current contract.
3. Exercise failed-before-PR, lost launch acknowledgment, mid-run crash/restart, cancel-with-unknown-stop, zero/unknown usage, two work units in one invocation and duplicate export. Native producer code must create records; hand-authored normalized fixtures cannot satisfy the producer claim. An allocation-less invocation retains unallocated usage, not a charge per associated item.
4. Run source→binding→reducer integration through the exported package or validated artifact boundary; respect module and authority separation. Use upstream disposable PostgreSQL/adapter fixtures where already qualified, explicit opt-in for any local test service, and never contact a configured live endpoint. Record supported provider/profile limits and the real-adapter integration evidence.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd loopadmin && go test -v ./supervisor/... -run '^TestEvidenceNativeFlow$' > "${TMPDIR:-/tmp}/di-06-TestEvidenceNativeFlow.out" && grep -F -- '--- PASS: TestEvidenceNativeFlow' "${TMPDIR:-/tmp}/di-06-TestEvidenceNativeFlow.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd loopadmin && go test -v ./supervisor/... -run '^TestEvidenceRecovery$' > "${TMPDIR:-/tmp}/di-06-TestEvidenceRecovery.out" && grep -F -- '--- PASS: TestEvidenceRecovery' "${TMPDIR:-/tmp}/di-06-TestEvidenceRecovery.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd loopadmin && go test -v ./supervisor/... -run '^TestEvidenceUsageGap$' > "${TMPDIR:-/tmp}/di-06-TestEvidenceUsageGap.out" && grep -F -- '--- PASS: TestEvidenceUsageGap' "${TMPDIR:-/tmp}/di-06-TestEvidenceUsageGap.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/06` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
