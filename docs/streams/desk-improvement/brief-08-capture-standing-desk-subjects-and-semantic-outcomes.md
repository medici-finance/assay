---
brief: "assay:assay:desk-improvement:08"
title: "Capture standing desk subjects and semantic outcomes"
why: "A long-running desk process may handle many items or none; role decisions and semantic outcomes require explicit receipts rather than a successful invocation inferred as delivery."
wave: 3
depends: ["desk-improvement/05", "desk-improvement/06"]
unblocks: ["desk-improvement/10"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "0622d0f4-7752-4a0c-8f56-f1afb1f6e44f"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["loopadmin/clients/desk/evidence.go: follow-up desk-improvement/08", "loopadmin/clients/desk/evidence_test.go: follow-up desk-improvement/08", "tools/desk/internal/loopengine/evidence.go: follow-up desk-improvement/08", "tools/desk/internal/loopengine/evidence_test.go: follow-up desk-improvement/08", "loopadmin/testdata/desk-evidence/: follow-up desk-improvement/08", "plugins/assay/references/desk-improvement.md: follow-up desk-improvement/08"]
gates: [{"on": "graph-execution/27", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}, {"on": "brief-quality/07", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}]
---

# Brief 08 — Capture standing desk subjects and semantic outcomes

## Context

files:
- `loopadmin/clients/desk/evidence.go (planned adapter in upstream client)`
- `loopadmin/clients/desk/evidence_test.go (planned)`
- `tools/desk/internal/loopengine/evidence.go (planned permitted-result adapter)`
- `tools/desk/internal/loopengine/evidence_test.go (planned)`
- `loopadmin/testdata/desk-evidence/ (planned synthetic/disposable fixtures)`
- `plugins/assay/references/desk-improvement.md (04 reference; add supported capture binding)`
- `changelog/desk-improvement-08.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: standing desk client and existing role-result/brief-quality adapters
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

1. Extend the actual graph-execution/27 standing client with explicit invocation-to-subject associations and existing role receipts; consume brief-quality/07 outcomes instead of cloning its join/attribution logic. Validate source identity/authority before interpreting an outcome. An agent-produced statement alone is not an independent verdict.
2. Bind intake admission/disposition, worker attempts/no-PR outcomes, review exact subject/finding/disposition, verification subject/contract/witness and coordinator arrival/capacity/eligibility/decision opportunity. Unsupported role sources stay unknown; do not guess associations from a checkout, session name or successful process exit. Preserve non-item/idle/held invocations separately.
3. Reconcile multiple original items, shared/unallocated usage, repeated/failed/abandoned attempts and per-item scope corrections. No single invocation ID is repurposed as a work UUID. Existing source owners retain publication authority, human receipts and privacy.
4. Test native client/result path through binding and reducer for all five reference profiles plus one additional configured role. Include no-item invocation, two-item allocation, missing decision-opportunity snapshot, unjudged finding, stale witness and conflicting receipt. Document actual supported modes; no silent five-role assumption or automatic cadence change.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd loopadmin && go test -v ./clients/desk/... -run '^TestDeskNativeEvidence$' > "${TMPDIR:-/tmp}/di-08-TestDeskNativeEvidence.out" && grep -F -- '--- PASS: TestDeskNativeEvidence' "${TMPDIR:-/tmp}/di-08-TestDeskNativeEvidence.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd loopadmin && go test -v ./clients/desk/... -run '^TestDeskEvidenceMissing$' > "${TMPDIR:-/tmp}/di-08-TestDeskEvidenceMissing.out" && grep -F -- '--- PASS: TestDeskEvidenceMissing' "${TMPDIR:-/tmp}/di-08-TestDeskEvidenceMissing.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd loopadmin && go test -v ./clients/desk/... -run '^TestDeskEvidenceRoles$' > "${TMPDIR:-/tmp}/di-08-TestDeskEvidenceRoles.out" && grep -F -- '--- PASS: TestDeskEvidenceRoles' "${TMPDIR:-/tmp}/di-08-TestDeskEvidenceRoles.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | check +mutation | `cd tools/desk && go test -v ./internal/loopengine/... -run '^TestRoleEvidenceReceipt$' > "${TMPDIR:-/tmp}/di-role.out" && grep -F -- '--- PASS: TestRoleEvidenceReceipt' "${TMPDIR:-/tmp}/di-role.out"` | Existing role result source→permitted receipt adapter actually runs; no successful process is counted as independent acceptance |
| 5 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/08` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
