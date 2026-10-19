---
brief: assay:assay:graph-execution:21
title: Deterministic task controller with durable dispatch and waits
why: Queue mechanics should survive sessions and invoke a model only when a work item becomes actionable.
wave: 7
depends:
- graph-execution/19
- graph-execution/20
- graph-execution/04
- graph-execution/14
- graph-execution/26
unblocks:
- graph-execution/05
- graph-execution/24
- agent-execution/04
- "agent-execution/08"
effort: M
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Dispatches attempts through the admission boundary and holds operator pause, drain and cancellation; the owner confirms a bypassed or stale controller cannot advance work or widen dispatch authority.
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
version: 5
id: 008dd7b9-6421-42bd-b6c6-d3c495837fa8
---

# Brief 21 — Deterministic task controller with durable dispatch and waits

## Context

files: `workflow/controller/` (planned), `workflow/cmd/assay-workflow/` (planned), `workflow/testdata/controller/` (planned), `workflow/README.md` (planned), `docs/enforcement-model.md`, `changelog/graph-execution-21-controller-host.md` (planned).

facts: The graph instance, admission and recovery contracts are the canonical source. The workflow module is new at the inspected revision. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the controller's current-owner and acceptance recheck at each transition — behind it, the existing effect boundaries (04 receipts, 14 assignment recheck), which reject a stale generation when the controller is bypassed.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The implementation runs offline fixtures only and holds no effect credential, but the controller dispatches through the admission boundary and holds pause, drain and cancellation: once such a control is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

## Read first

- [Task workflow specification](task-workflow-program.md).
- [Stream dependencies and rollout](README.md).
- [Structured input contract](work-input-amendment.md).

## Ground rules

- Work in an isolated branch; no merge, deployment or live infrastructure contact.
- Offline fixtures/fake providers only in this brief; a concrete adapter does not authorize provider calls.
- Stop at implemented; independent verification owns verified/done.
- Preserve one canonical work identity and one claim authority; no credentials in packets or results.

## Library-first amendment — 2026-10-08

Read `docs/library-first.md`. Reuse the declared component library where it shares authority,
and a typed SDK client where a process boundary enforces authority or isolation. Workflow
progression, process supervision and effect execution remain distinct; a shared binary or
library grants no additional role. Preserve the existing independent fence, no-store standing
mode, process-launch audit and cross-journal recovery tests. No cell taxonomy dependency is added.

## Verifier migration amendment — 2026-10-19

Implement [the migration state/queue contract](verifier-migration.md) through the existing
/19 transactions and /26 supervisor. Expose each component, current owner, reason, wait age
and release condition in the existing snapshot/export; queues remain derived views of the
same records. Deterministic transitions consume versioned recorded facts, including model
outputs and time observations. No inference on replay and no provider is required in the
model-disabled profile. Generative judgment and System-1 classification are bounded
activities selected by policy, never controller agents or publication credentials.

Unknown launch or external effect enters reconciliation, not a fresh dispatch. Assertion
failure is repair-required, not retry-until-green. A relevant event re-reads current facts
before resumption; supersession/cancellation fences work while preserving reconciliation
and resource-retirement obligations. Complete requires the authorized publication receipt;
accepted/publication-pending remains visible. Snapshot consumers must not dispatch.

Add `TestVerifierStagesReplay` (planned) in `workflow/controller/` (already declared). This is a
checkpoint within /21, not independent activation or a new runtime implementation.


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
| 5 | check:ci +flow +mutation | `(cd workflow && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestVerifierStagesReplay$" ./controller/ > "$out" && grep -Fq -- "--- PASS: TestVerifierStagesReplay " "$out")` | exit 0; named PASS; recorded facts replay to identical transitions without inference; unchanged held inputs do not dispatch; accepted/publication-pending is not complete; mutation: advance on process success or infer on replay — the test fails |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Execution-environment follow-on routing — 2026-10-16

Reciprocal unblocks only: agent-execution/04. The [execution amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md) adds a follow-on obligation over this brief's existing output. Its Task, Verify, gate, version and historical Evidence are unchanged; this note grants no delivery, reassignment or activation.

## Proposed measurement consumer — 2026-10-17

desk-improvement/07 consumes this brief for native controller decision/wait/attempt/outcome evidence after this controller exists.
This is a forward consumer-routing amendment only: the new consumer waits on this owner,
not the reverse. Original Task/Verify and completed Evidence are unchanged. Reconcile any
active implementation claim before adding capture hooks. Native emission qualification is
delivered by the named follow-up; this source contract alone does not claim it exists.
