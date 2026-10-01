---
brief: assay:assay:graph-execution:22
title: First pinned local harness adapter and qualification fixtures
why: A fake runner proves contracts but cannot execute useful model work; a narrow adapter connects one harness
  without multiplying orchestration implementations.
wave: 1
depends:
- graph-execution/20
unblocks:
- graph-execution/27
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
- 'loopadmin/supervisor: follow-up graph-execution/26'
- 'standing desk bindings: follow-up graph-execution/27'
- 'workflow/controller: follow-up graph-execution/21'
- 'adopting cohorts: out-of-scope (live qualification and model choice require owner authorization)'
version: 2
id: 97c5ef2b-7f1c-4d7b-9484-dcfc042926d1
---

# Brief 22 — First pinned local harness adapter and qualification fixtures

## Context

files: `loopadmin/adapters/localharness/` (planned), `loopadmin/testdata/localharness/` (planned), `loopadmin/README.md` (planned), `docs/workflow-runner-qualification.md` (planned), `changelog/graph-execution-22-local-harness-adapter.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The shared loopadmin module is bootstrapped by /20 and must remain usable without graph storage. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

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

1. Implement the first adapter for a pinned Claude Code harness profile using the existing public harness reference and cellctl model-policy conventions as inputs. Record exact executable/version, request/session ID, model actually used, tool restrictions and serialization version. Determine supported invocation/output from the pinned installed client at implementation pickup; commit scrubbed fixture transcripts and the capability manifest before claiming support. No hard-coded commercial price or assumed cancellation guarantee.
2. Expose the adapter through the shared loop-admin supervisor /26; do not add a caller-specific process launcher. Use an isolated supervised process with an operator-approved argv/profile, sanitized environment and constrained workspace/tool access. Never execute an argv supplied by task text. The controller store and role credentials are outside the child domain. Inherit existing credential custody; do not implement another vault.
3. Map start/observe/cancel/reconcile to protocol states. When the harness cannot prove an unknown request stopped, retain unknown and require reconciliation/operator resolution. Do not simulate resume by launching a second unknown request. Charge all reported usage to the original attempt/work; missing usage remains unknown.
4. Run the 20 conformance suite against a process fixture plus captured protocol outputs. Publish an offline qualification report; live model calls and additional adapters require an adopting cohort decision. Record control gaps rather than weakening them. Update docs and changelog.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessConformance$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessConformance " "$routing_out")` | exit 0; named PASS; one fixture process executes a scoped attempt and preserves actual runner/model identity |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUnknownCancel$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUnknownCancel " "$routing_out")` | exit 0; named PASS; lost acknowledgment or unconfirmed cancellation holds instead of relaunching |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessUntrustedArgvAndSecrets$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessUntrustedArgvAndSecrets " "$routing_out")` | exit 0; named PASS; task-supplied argv and secret leakage are refused at the launcher boundary |
| 4 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLocalHarnessBothCallers$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLocalHarnessBothCallers " "$routing_out")` | exit 0; named PASS; desk and workflow clients use the same adapter and pinned model controls; session continuity cannot cross roles and unsupported resume holds in both modes |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
