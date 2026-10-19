---
brief: "assay:assay:desk-improvement:05"
title: "Bind desk assessments to canonical evidence records"
why: "A PR-keyed telemetry interface cannot bind every desk admission, attempt and outcome, so missing native facts must be mapped explicitly before emitters can claim coverage."
wave: 1
depends: ["desk-improvement/01"]
unblocks: ["desk-improvement/06", "desk-improvement/07", "desk-improvement/08"]
effort: "M"
gate: "model"
risk: {"regulatory": "no", "customer": "no", "irreversible": "no", "sensitive-data": "no"}
issues: []
schema: "brief-v2"
version: 1
id: "7d352a74-2dd8-409a-9a55-0dc47aa3b8d0"
authored: "2026-10-17 by desk-improvement authoring"
sources: ["docs/streams/desk-improvement/spec.md", "docs/proposals/non-ci-architecture/11-target-architecture.md", "freshness-checked 2026-10-17 @ 2f56cf289; reconcile implementation claims and current PRs at pickup"]
exec-tier: "strong"
exec-tier-why: "Cross-record correctness, independent evidence and module/authority boundaries require design judgment."
domain: "complicated"
outcome: "none"
consumers: ["docs/streams/desk-improvement/evidence-bindings.md: follow-up desk-improvement/05", "schemas/desk-evidence-binding-v1.json: follow-up desk-improvement/05", "qualgen/deskquality/evidence.go: follow-up desk-improvement/05", "qualgen/deskquality/evidence_test.go: follow-up desk-improvement/05", "qualgen/deskquality/testdata/bindings/: follow-up desk-improvement/05", "schemas/run-record-v1.json: follow-up desk-improvement/05", "qualgen/briefquality/events.go: follow-up desk-improvement/05", "docs/contracts.md: follow-up desk-improvement/05"]
gates: [{"on": "graph-execution/06", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}, {"on": "brief-quality/05", "type": "build-dep", "reason": "Consume the implemented and independently checked canonical owner; a proposed schema or board label alone is insufficient."}]
---

# Brief 05 — Bind desk assessments to canonical evidence records

## Context

files:
- `docs/streams/desk-improvement/evidence-bindings.md (planned mapping contract)`
- `schemas/desk-evidence-binding-v1.json (planned binding manifest, not an event lifecycle)`
- `qualgen/deskquality/evidence.go (planned pure mapping validation)`
- `qualgen/deskquality/evidence_test.go (planned)`
- `qualgen/deskquality/testdata/bindings/ (planned synthetic mappings)`
- `schemas/run-record-v1.json (upstream planned; compatible optional binding extension only if needed)`
- `qualgen/briefquality/events.go (upstream planned; compatible optional associations only if needed)`
- `docs/contracts.md (existing owner mapping)`
- `changelog/desk-improvement-05.md` (planned) — use the actual unique implementation branch slug

facts (source-checked 2026-10-17 @ 2f56cf289):
- Canonical intent is spec.md; upstream gates above name actual owners, not a deployed assertion.
- runner Usage preserves unknown token/cost values and invocation identity already exists; semantic outcome capture remains partly planned.
- Continuing operations owns enduring assessment subjects; runtime/work/evidence owners retain their existing IDs, custody, journals and acceptance.

layering: pure assessment/binding functions with narrow source/view adapters; current runtime owns effects and PostgreSQL coordination. No new service, work lifecycle or credential boundary.

design-fit:
  owner: existing execution/brief-quality record owners; deskquality binding projection
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

1. Dereference the actual outputs of graph-execution/06 and brief-quality/05. Publish a versioned mapping from native records to 01's normalized evidence: work/instance/node/attempt and role subject, caller/invocation/generation, exact candidate/acceptance/configuration, findings/dispositions, decisions, interval precision, usage units, visibility and completeness.
2. Reuse native identities, corrections, authority and record lifecycle. A binding manifest describes source fields and permitted joins; it is not another event envelope, authoritative ledger or new attempt ID. Make only compatible optional extensions in the owning schema/module where genuinely missing, document every consumer and preserve older readers' behavior or explicit unsupported result.
3. Keep workload allocation explicit for multi-item invocations, no-PR work and unallocated usage. Requested/resolved/observed model and elapsed/active/human time are separate. Publication permits only its declared field profile; source authority and original human receipts stay in their existing restricted owners.
4. Conformance vectors reject duplicate-counted effects, same-ID conflicts, guessed subject associations, unsupported required semantics, silent missing fields and forbidden publication payloads. Supply the source-binding interface for 06–08 and independent 02/03 consumers, with no network or execution imports.

## Interface contract

Canonical source/contract revisions → validated evidence/assessment projections. Source identity, unknowns and authority are preserved. This interface cannot adopt policy, synthesize acceptance or repeat an effect.

## Verify

Rows run from this repository root. Named tests must actually execute; future implementation supplies them. Independent review assesses label/permission/benefit claims beyond structural validity.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow | `cd qualgen && go test -v ./deskquality/... -run '^TestEvidenceBinding$' > "${TMPDIR:-/tmp}/di-05-TestEvidenceBinding.out" && grep -F -- '--- PASS: TestEvidenceBinding' "${TMPDIR:-/tmp}/di-05-TestEvidenceBinding.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 2 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestBindingNoFork$' > "${TMPDIR:-/tmp}/di-05-TestBindingNoFork.out" && grep -F -- '--- PASS: TestBindingNoFork' "${TMPDIR:-/tmp}/di-05-TestBindingNoFork.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 3 | check +mutation | `cd qualgen && go test -v ./deskquality/... -run '^TestBindingPrivacy$' > "${TMPDIR:-/tmp}/di-05-TestBindingPrivacy.out" && grep -F -- '--- PASS: TestBindingPrivacy' "${TMPDIR:-/tmp}/di-05-TestBindingPrivacy.out"` | exit 0; named positive and wrong-but-plausible negative/source-flow cases actually execute |
| 4 | gate:model +dereference | check:manual — independent reviewer dereferences the original source/contract and output claims, including upstream prerequisites | Record exact evidence, supported scope and unresolved limits; shape/test presence is not truth or deployment |
| 90 | check | `statusgen --root . --consumers --brief desk-improvement/05` | exit 0 after implementation routing matches the actual diff; omitted consumers still require review |

## Definition of done

Task outputs satisfy the canonical spec acceptance trace for this brief, with actual discriminating row evidence, supported modes/versions and independent assessment of remaining limits.

## Premortem

The plausible false result is a favorable summary from missing/guessed data or a schema claimed as native emission. Source-flow row 1 and negative cases reject it; the independent row owns truth, authority and adequacy. No measurement result changes an execution/human gate.

## Evidence

Not implemented by this authoring change. A non-implementer appends subject, versions, command, exit/output and date after execution.

## Review

Gate: model for bounded offline implementation; no verified/done assertion is made by this plan.
