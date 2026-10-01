---
brief: assay:assay:graph-execution:20
title: Role runner protocol and offline conformance kit
why: Models and harnesses can vary by stage only if interruption, identity, tool scope and usage have a common checked
  contract.
wave: 3
depends:
- graph-execution/09
- graph-execution/16
- graph-execution/19
unblocks:
- graph-execution/22
- graph-execution/23
- graph-execution/21
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
- freshness-checked 2026-10-02 @ a944ad1103aadaba919c11fe425089057f5c2f4e
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'workflow/controller: follow-up graph-execution/21'
- 'workflow/adapters: follow-up graph-execution/22'
version: 1
id: 64e79118-7a61-43d3-b9a8-5ec8aed75ea7
---

# Brief 20 — Role runner protocol and offline conformance kit

## Context

files: `workflow/runner/` (planned), `workflow/testdata/runner/` (planned), `spec/workflow-runner-v1.md` (planned), `schemas/workflow-runner-v1.json` (planned), `workflow/README.md` (planned), `changelog/graph-execution-20-runner-contract.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: controller correctness alone is insufficient — independently enforcing effect/role boundaries and fixture oracles must still reject a bypass.

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

1. Define start/observe/cancel/reconcile with stable launch request identity, role packet, pinned model/skill/tool profile, workspace reference, claim generation and budget reservation. Define typed results/artifacts/usage and unknown states; preserve source trust and credential exclusion.
2. Declare adapter capabilities for resume, cancellation acknowledgment, launch versus per-request budgets, telemetry and snapshot access. Never interpret missing telemetry as zero, process exit as accepted output, or cancel request as confirmed stop. No silent provider/model fallback.
3. Ship a fake runner and adapter conformance kit covering lost launch acknowledgment, late output from a fenced attempt, malformed result, unauthorized tool request, missing cost, cancelled-but-still-running and unsupported resume. Authentication/role enforcement is independent of model text.
4. Document the protocol and compatibility rules; register the workflow module test job/path triggers and changelog. No real provider client or provider call in this unit.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceUnknownLaunch$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceUnknownLaunch " "$routing_out")` | exit 0; named PASS; unknown launch requires reconcile and cannot create a concurrent replacement |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceLateResultDenied$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceLateResultDenied " "$routing_out")` | exit 0; named PASS; fenced generation cannot be accepted even when its result is well formed |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRunnerConformanceCredentialAndUsage$" ./... > "$routing_out" && grep -q -- "--- PASS: TestRunnerConformanceCredentialAndUsage " "$routing_out")` | exit 0; named PASS; secrets are rejected and missing usage remains unknown |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
