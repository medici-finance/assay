---
brief: assay:assay:graph-execution:26
title: Shared loop-admin launch supervision for desks and workflows
why: A reusable launch supervisor lets standing desks and graph stages share provider adapters, cancellation, recovery
  and cost controls without sharing scheduling authority.
wave: 1
depends:
- graph-execution/20
unblocks:
- graph-execution/27
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
- 'tools/desk/internal/loopengine: follow-up graph-execution/27'
- 'operator lifecycle and cockpit: out-of-scope (adopter qualification)'
version: 1
id: bcbee0b0-8f12-4e7a-b028-0637b2bab0c3
---

# Brief 26 — Shared loop-admin launch supervision for desks and workflows

## Context

files: `loopadmin/supervisor/` (planned), `loopadmin/journal/` (planned), `loopadmin/control/` (planned), `loopadmin/cmd/loop-admin/` (planned), `loopadmin/testdata/supervisor/` (planned), `loopadmin/README.md` (planned), `docs/loop-admin.md` (planned), `.github/workflows/loopadmin.yml` (planned), `changelog/graph-execution-26-loop-admin.md` (planned).

facts: The new loopadmin module is bootstrapped by /20. Current tools/desk/internal/loopengine owns standing desk work. Workflow instance ownership remains outside the supervisor; the shared module has no graph-store dependency. Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

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

1. Implement one shared process/session supervisor over /20. Accept only authenticated caller-scoped, operator-approved launch profiles and validated assignment/reservation references. Support standing-desk and workflow-stage modes with identical adapter and lifecycle semantics. An operator may run separate per-role processes of the same binary; reuse does not imply a privileged all-role daemon.
2. Persist launch intent before starting a child in an execution journal keyed by caller namespace, invocation ID and owner generation. Keep adapter receipts, lifecycle state, cancellation, limits, usage and checkpoints. This journal owns process facts only; desk queues and graph stores retain authoritative work/acceptance state. Caller intent and journal receipt reconcile by stable request ID, including a crash between their separate commits; do not claim a transaction across stores.
3. Implement start/observe/cancel/reconcile and idle session retention only when the adapter declares support. Duplicate input cannot launch twice; unknown launch/stop holds until reconciled; bounded restarts never create another unknown request. Recheck role/profile/generation and budget at every child launch and receipt delivery. Shared concurrency reservations bound all callers using this profile; preserve consumption after crash/restore.
4. Keep runtime launch authority separate from operator profile/install/credential administration. A desk client may request an allowed child invocation through a bounded, capability-scoped endpoint, but cannot choose arbitrary argv, escalate role, raise limits or recursively create unconstrained agents. Effect credentials and operator admin are absent; any inference credential stays confined to its approved adapter boundary and is excluded from packets, artifacts and logs.
5. Export sanitized process/session status with mode, caller binding and optional canonical workflow references. Viewer attach/detach never starts a process. The default command uses offline fixtures; live profile activation is separate. Provide backup/restore of the execution journal with referenced artifacts, preserving pause/spend/unknowns; restore is passive. Document state ownership and refusal behavior.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminBothModesNoGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminBothModesNoGraph " "$routing_out")` | exit 0; named PASS; both caller modes launch via one fake adapter; standing mode starts with graph storage absent |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminUnknownRestartAndAccounting$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminUnknownRestartAndAccounting " "$routing_out")` | exit 0; named PASS; crash between caller intent and launch receipt reconciles once; unknown cancel blocks relaunch; usage and limits survive restart |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminCallerIsolationAndDirectBypass$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminCallerIsolationAndDirectBypass " "$routing_out")` | exit 0; named PASS; a direct lower-boundary call cannot change role/profile, reuse another caller receipt or exceed shared capacity; operator actions are unreachable |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
