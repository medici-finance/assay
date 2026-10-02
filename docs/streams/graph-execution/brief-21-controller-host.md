---
brief: assay:assay:graph-execution:21
title: Deterministic task controller with durable dispatch and waits
why: Queue mechanics should survive sessions and invoke a model only when a work item becomes actionable.
wave: 4
depends:
- graph-execution/19
- graph-execution/20
- graph-execution/04
- graph-execution/14
- graph-execution/26
unblocks:
- graph-execution/05
- graph-execution/24
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
- 'workflow/internalreview: follow-up graph-execution/24'
- 'statusgen/assuranceexperiment.go: follow-up graph-execution/18'
- 'operator clients: out-of-scope (consume the published protocol through their own adoption gates)'
version: 2
id: 008dd7b9-6421-42bd-b6c6-d3c495837fa8
---

# Brief 21 — Deterministic task controller with durable dispatch and waits

## Context

files: `workflow/controller/` (planned), `workflow/cmd/assay-workflow/` (planned), `workflow/testdata/controller/` (planned), `workflow/README.md` (planned), `docs/enforcement-model.md`, `changelog/graph-execution-21-controller-host.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the controller's current-owner and acceptance recheck at each transition — behind it, the existing effect boundaries (04 receipts, 14 assignment recheck), which reject a stale generation when the controller is bypassed.

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

1. Implement the executable host over 19 storage, 20 runner protocol and existing eligibility/admission/reservation/effect interfaces. Input events trigger reconciliation; periodic reconciliation recovers missed notifications. Reuse the callable admission boundary, never a copied router or a second claim service.
2. Durably record dispatch reason/fingerprint and launch intent; dedupe unchanged events at the authoritative owner boundary. Confirmed failed launch may retry with a new attempt; unknown launch/effect reconciles first. Persist typed human/external waits without a live model session.
3. Implement pause-admission, drain, cancellation intent and passive restore. Recheck subject/acceptance/policy/generation at result acceptance. Every accepted transition is atomic or fenced. A lease expiration does not prove an old provider request stopped.
4. Expose read-only versioned snapshot/export and narrow operator commands through authenticated local control, with no generic arbitrary-command or caller-selected-role endpoint. Effect credentials stay in separate role executors. The first host profile is single controller on durable local storage; no active-active mode.
5. Exercise the complete connected fake-runner/effect corpus, including direct boundary bypass. CLI default is offline/dry-run and prints no secret values. Document limits and CI cross-module coverage; live adoption is separately gated.

## Interface contract

Pinned instance/attempt/subject and actor capability enter; typed state, evidence and receipts leave with the same identity. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Shared loop-admin amendment — 2026-10-02

Use /26 loop-admin as the sole model/session launch supervisor; this controller is its workflow-stage client. Map canonical graph instance/node/attempt and claim/reservation references to the /20 envelope and persist their correlation. The controller owns node readiness, pattern progression, decisions and output acceptance; loop-admin owns only process/session lifecycle. Reconcile both journals after a split crash; never launch directly through /22 or duplicate its restart logic. A standing desk can run through the same supervisor without this controller.

Additional implementation files: `workflow/controller/loopadmin_test.go` (planned).

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestControllerDuplicateAndRetry$" ./... > "$routing_out" && grep -q -- "--- PASS: TestControllerDuplicateAndRetry " "$routing_out")` | exit 0; named PASS; same-state notifications launch once and a definite failed launch retries; mutation: drop the actionable-fingerprint check before launch — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestControllerCrashUnknownAndPause$" ./... > "$routing_out" && grep -q -- "--- PASS: TestControllerCrashUnknownAndPause " "$routing_out")` | exit 0; named PASS; crash recovery retains pause/spend and reconciles unknown effects before resuming; mutation: resume after a crash without reconciling unknown effects — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestControllerStaleAcceptanceAndBypass$" ./... > "$routing_out" && grep -q -- "--- PASS: TestControllerStaleAcceptanceAndBypass " "$routing_out")` | exit 0; named PASS; head movement or a direct call without a current owner cannot advance; mutation: skip the current-owner check on the direct transition call — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd workflow && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestControllerUsesSharedLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestControllerUsesSharedLoopAdmin " "$routing_out")` | exit 0; named PASS; graph node progression uses the common supervisor; a lost cross-journal receipt does not double-launch; a process exit cannot advance an unaccepted node; mutation: advance a node on process exit without acceptance — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
