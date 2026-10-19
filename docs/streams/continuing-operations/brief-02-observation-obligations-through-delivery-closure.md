---
brief: continuing-operations/02
title: Observation obligations through delivery closure
why: "Delivery closure must not erase the obligation to observe whether the change helped. Preserve one due record and assessment lineage through manual and practice ownership."
wave: 3
depends: ["continuing-operations/01"]
unblocks: ["statusgen/25"]
effort: M
gate: model
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: no}
issues: []
schema: brief-v1
authored: 2026-10-13 by statusgen newbrief
sources:
  - docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - freshness-checked 2026-10-13 @ d484dd5df24f2aa8d557dafce41687b38173f227
exec-tier: strong
exec-tier-why: Cross-contract identity, compatibility and evidence semantics require design and consumer reasoning.
domain: complicated
outcome: none
id: d00ff0e4-84a3-4670-a15f-04ba085ca8b7
consumers:
  - "spec/change-impact-v1.md, schemas/change-impact-v1.json, statusgen/operations: follow-up continuing-operations/02"
  - "statusgen/portfolio read projection: follow-up statusgen/25"
  - "recurring observation scheduler and exposure collector: out-of-scope (deferred outlines O04/O06/O11; no live integration in this slice)"
version: 2
---

# Brief 02 — Observation obligations through delivery closure

## Context

files: `spec/change-impact-v1.md` (planned), `schemas/change-impact-v1.json` (planned), `statusgen/operations/`, `docs/operations/observation-handoff.md` (planned), `docs/streams/continuing-operations/spec.md`, `docs/contracts.md`; `changelog/continuing-operations-02-portfolio.md` (planned).

facts (d484dd5df, 13 October 2026):
- O03/O05/O06/O10 contain impact and practice intentions; their runtime adapters remain unimplemented.
- Delivery lifecycle and independent verification are not replaced by exposure, usage or observed benefit.
- This offline contract slice uses explicit evidence snapshots and a manual due list, with no scheduler.

layering: pure typed records/evaluators in `statusgen/operations`; filesystem and CLI adapters remain outside the package. No network, credentials, scheduler or runtime database. The flow and boundary fixtures below enforce this separation.

single-point-of-failure: an incorrect record interpretation could create a false alignment or completion claim; schema/record validation and independently supplied source/approval/coverage fixtures exercise different failure inputs, while existing admission and publication controls remain outside this read-only capability.

design-fit:
  owner: statusgen/operations
  contract: change-impact-v1 (planned extension; register its semantic owner in docs/contracts.md)
  retires: []
  weight: "one bounded contract/evaluation capability; no new service, scheduler or dispatch authority"
  why-add: "Existing records do not carry this meaning; extend their owning contracts rather than adding an independent product-state system."

## Read first

- `docs/proposals/non-ci-architecture/26-product-intent-and-portfolio.md` — exact record semantics, boundaries and owner mapping.
- `spec/registers-v1.md`, `spec/brief-v1.md` and the declared dependency briefs.
- `docs/streams/continuing-operations/spec.md` for operational identity and impact ownership.

## Ground rules

- Isolated branch and draft PR; no merge, release or activation.
- Synthetic offline inputs only; no collection from real transcripts or production systems.
- Preserve existing records and populated Evidence; stop at implemented for independent verification.
- Named tests and planned interfaces below must be delivered; they are not current commands claimed to pass.

## Task

1. Define one versioned impact/obligation record with stable identity, subject and brief/impact revisions, owner, signal/unit, metric revision, baseline or baseline gap, cohort, guardrails, rule/band, exposure trigger/window, evidence source and history. Map candidate observe syntax to this owner; document supported syntax only when schema/parser agree. Preserve no-direct-impact rationale without waiving enabling/learning duties.
2. Implement pure due/trigger/assessment evaluation at an explicit as-of time: missing exposure unresolved; delivered/done does not satisfy the obligation; supporting, adverse, mixed and inconclusive assessments retain source references and contradictions.
3. Implement an offline custody-transfer record with expected prior revision, receipt and idempotent replay from manual due list to practice reference. One obligation and assessment lineage survive closure, rename and transfer; conflicting transfer or missing referenced evidence remains held/unknown. Publish manual handoff instructions; recurring runtime operation remains deferred.

## Interface contract

Input: subject, impact, exposure/evidence and custody snapshots plus as-of time. Output: stable obligation/assessment references, unresolved/due/overdue status with coverage and transfer receipts. Historical verification is preserved; this does not advance delivery lifecycle.

## Verify

Run from the repository root after implementation. Each named test asserts the stated
behavior and includes the specified failing mutation. A missing test/package or absent
named PASS fails the row; do not substitute a skipped or zero-test run.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObservationSurvivesDone$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestObservationSurvivesDone " "$check_output")` | exit 0; named PASS; done brief with unresolved exposure retains one due obligation; mutation: close it on delivery completion |
| 2 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObservationTransferOnce$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestObservationTransferOnce " "$check_output")` | exit 0; named PASS; replayed transfer preserves ID, trigger/history and one deadline; stale concurrent transfer refuses; mutation: create a new obligation on transfer |
| 3 | check:ci +flow +mutation | `(cd statusgen && check_output=$(mktemp) && trap 'rm -f "$check_output"' 0 && GOWORK=off go test -count=1 -v -run "^TestObservationAssessmentFlow$" ./operations/ > "$check_output" && grep -F -- "--- PASS: TestObservationAssessmentFlow " "$check_output")` | exit 0; named PASS; subject and impact snapshot through explicit evidence to due view distinguishes adverse/mixed/missing and unresolved trigger; mutation: missing evidence becomes zero benefit |

## Evidence

<!-- Independent verifier records subject revision, command, exit, named result and environment. No implementation or execution evidence is asserted here. -->

## Review

Gate: model; all risk answers no because this slice processes synthetic/explicit offline
snapshots and makes no live policy, publication or deployment change. Review semantic
compatibility and the real consumer flow; later live integration derives its own risk gate.

## Proposed desk assessment consumer — 2026-10-17

desk-improvement/03 declares its build-dependency gate on continuing-operations/02. It reuses operational identity/observation
lineage rather than defining a duplicate. This is a consumer note, not a reverse prerequisite;
original Task/Verify/Evidence and activation conditions remain unchanged.
