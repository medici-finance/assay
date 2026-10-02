---
brief: assay:assay:graph-execution:24
title: Versioned author implement review pattern with bounded repair
why: A program should converge on a reviewable candidate without hiding failed attempts or letting implementation
  approve itself.
wave: 7
depends:
- graph-execution/21
- graph-execution/23
- graph-execution/06
unblocks:
- graph-execution/25
effort: M
gate: model
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'no'
  sensitive-data: 'no'
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/publication: follow-up graph-execution/25'
- 'statusgen/assuranceexperiment.go: follow-up graph-execution/18'
version: 1
id: 3cb66385-8999-438e-8ac6-d0709f92a0b9
---

# Brief 24 — Versioned author implement review pattern with bounded repair

## Context

files: `workflow/internalreview/` (planned), `workflow/testdata/internalreview/` (planned), `spec/workflow-patterns/prepublication-v1.yaml` (planned), `spec/workflow-pattern-v1.md`, `workflow/README.md` (planned), `changelog/graph-execution-24-internal-specialist-pattern.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the distinct-actor check on internal review verdicts — behind it, 25's bridge, which re-validates independent actor and exact subject before any reviewer effect, and behind that the unchanged forge review.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Task

1. Add an opt-in prepublication pattern: author/plan review, required intent decision, implement, freeze, independent review/checks, bounded repair and publication request. Keep the existing implementation-v1 unchanged. Pattern/model mapping is configuration, not a trained router; role effects cannot exceed approved bindings.
2. Give each review its own context and capability/actor identity; independent roles cannot be impersonated by the implementer. Preserve stable finding IDs, severity, dispositions, all rounds and cumulative cost. Different model names alone do not establish independence.
3. Configure per-work attempt, wall-time and cumulative budgets. Exhaustion holds; it cannot convert failed or unknown checks into approval. Persist waits/resumption using canonical decision references; changed intent supersedes affected acceptance.
4. Test plan supersession, repair race, omitted findings, self-approval, budget exhaustion, authenticated but stale human answers and no silent model fallback. Reuse the existing run-record schema and recovery layer. Document a publication request as distinct from forge approval; no real forge effect in this brief.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInternalReviewFindingAndBudget$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInternalReviewFindingAndBudget " "$routing_out")` | exit 0; named PASS; all findings/attempt costs survive; exhaustion holds; mutation: reset the attempt budget on each repair round — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInternalReviewSelfApprovalDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInternalReviewSelfApprovalDenied " "$routing_out")` | exit 0; named PASS; implementer-produced verdict cannot satisfy independent review; mutation: drop the distinct-actor check on the review verdict — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestInternalReviewSupersededDecision$" ./... > "$routing_out" && grep -q -- "--- PASS: TestInternalReviewSupersededDecision " "$routing_out")` | exit 0; named PASS; changed question or candidate invalidates prior answer/verdict; mutation: keep a prior verdict valid after the candidate changes — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
