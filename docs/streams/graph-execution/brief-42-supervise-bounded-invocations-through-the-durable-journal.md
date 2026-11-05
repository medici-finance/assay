---
brief: assay:assay:graph-execution:42
title: Supervise bounded invocations through the durable journal
why: "One durable lifecycle owner lets desks and workflow stages reconcile bounded work without duplicate launches, model waiters or regained capacity after restart."
wave: 6
depends: ["graph-execution/41", "graph-execution/16", "graph-execution/28", "graph-execution/29"]
unblocks: ["graph-execution/43"]
split-from: graph-execution/26
effort: M
gate: human
risk: {regulatory: no, customer: no, irreversible: yes, sensitive-data: no}
gate-why: "Composes durable process state with role/profile/generation and launch brakes. Human acceptance confirms every new launch and receipt delivery rechecks current authority, including direct lower-boundary calls."
decision-trigger: spec
issues: [5492]
gates: [{"on": "#5492", "type": "human-gate", "reason": "Decomposition review: dispatch waits for a driver comment on #5492, posted under the driver's own login (never a role App), that accepts this proposed split and its source/acceptance routing. Only a follow-up PR citing that comment removes this edge."}]
schema: brief-v2
version: 1
id: 4665c5e6-0de1-49c5-a39f-4dd335bac0ed
authored: 2026-11-05 by statusgen newbrief
sources:
- docs/streams/graph-execution/brief-26-shared-loop-admin.md
- docs/streams/graph-execution/task-workflow-program.md
- docs/proposals/non-ci-architecture/20-amendment-manifest.md
- docs/go-engineering.md
- "freshness-checked 2026-11-05 @ 7f61cff94b188d46b4e38e6259798d85bb5d4510"
- "PR #4157: delivered current-owner startup preparation; reused, not reimplemented"
exec-tier: strong
exec-tier-why: "Cross-module custody, durable failure recovery and independent lower-boundary authority checks require design judgment."
domain: complicated
outcome: none
consumers:
- "loopadmin/supervisor/: follow-up graph-execution/42 (this child; only implementation may change to fixed-here)"
- "loopadmin/runner/: follow-up graph-execution/42 (this child; only implementation may change to fixed-here)"
- "loopadmin/README.md: follow-up graph-execution/42 (this child; only implementation may change to fixed-here)"
- "docs/loop-admin.md: follow-up graph-execution/42 (this child; only implementation may change to fixed-here)"
- "docs/contracts.md: follow-up graph-execution/42 (this child; only implementation may change to fixed-here)"
---

# Brief 42 — Supervise bounded invocations through the durable journal

## Context

files:
- `loopadmin/supervisor/`
- `loopadmin/testdata/supervisor/`
- `loopadmin/runner/` (only contract-compatible composition seams if needed)
- `loopadmin/README.md`
- `docs/loop-admin.md` (planned)
- `docs/contracts.md`
- `changelog/graph-execution-42-supervisor.md` (planned)

facts: At pickup runner.Adapter already exposes Capabilities, Start, Observe, Cancel and Reconcile, and runner.Client checks extensions, model/profile identity and generation when accepting results. Its invocation map is in-memory and does not replace the planned durable process journal. No supervisor exists. The caller owns work/acceptance and its claim/assignment generation; the cell/group generation is distinct. /16 is implemented on the board, while /20,/28,/29 are verified; fresh independent producer qualification is still required before any live activation.

layering: Compose the existing runner contract, moved process owner and durable journal in one loopadmin supervisor. It owns launch lifecycle and deterministic waits, never queue selection, graph progression, credentials or operator profile administration. Keep real admission at the supervisor boundary and the adapter fence at the lower execution boundary; an earlier caller check cannot substitute for either.

design-fit:
- owner: loopadmin/supervisor for invocation lifecycle; runner for adapter protocol; journal for durable process facts
- contract: spec/loop-admin-runner-v1.md, spec/loop-admin-execution-profile-v1.md and spec/operator-incarnation-v1.md
- retires: no earlier delivered control; this is the remaining parent scope
- weight: no new command/flag/refusal counts are ratified by authoring; the implementation must measure its exact delta, keep additions in the canonical owner and explain each positive dimension. No new scheduler, work store, alternate engine or credential broker is permitted.
- why-add: One durable lifecycle owner lets desks and workflow stages reconcile bounded work without duplicate launches, model waiters or regained capacity after restart.

single-point-of-failure: the journal-admitted invocation boundary; protected operator incarnation and the adapter/caller authority fence remain independent and are exercised with direct bypass fixtures.

## Read first

- [Parent contract](brief-26-shared-loop-admin.md), version 10, including all amendments and its unchanged acceptance table.
- [Task workflow program](task-workflow-program.md), [structured input](work-input-amendment.md) and [stream routing](README.md).
- [Library ownership](../../library-first.md), [Go engineering](../../go-engineering.md), [semantic owners and controls](../../contracts.md).
- [R2 target and amendment ownership](../../proposals/non-ci-architecture/20-amendment-manifest.md), [operator incarnation](../../../spec/operator-incarnation-v1.md).
- [Implementation pickup](../../../plugins/assay/references/implementation-pickup.md); re-establish current source and active claims before effects.

## Ground rules

- This is a proposed split of graph-execution/26. Its parent remains the complete acceptance contract; this authoring does not deliver it or change its prerequisites, gate, Task, Verify or Evidence.
- Work offline in an owned branch. No operational database, provider/model call, image launch, service activation, live desk migration, release, workflow dispatch, merge or ready flip.
- Real PostgreSQL checks use only an explicitly owned disposable fixture. Missing fixture configuration fails its row; no self-skip, ambient database discovery or local embedded substitute.
- Preserve one canonical work identity and claim authority. No effect credential or operator administration in launch code. No credentials in packets, errors, artifacts or logs.
- Retain every existing independent guard and mutation. A control relocation must carry its consumer flow and negative control; any security weakening stops for the human.
- Stop at implemented behind a draft PR; independent verification and the human gate own acceptance. Record before/after weight and fail-first evidence as required by the worker kit.

## Task

1. Implement one shared lifecycle over runner.Adapter and the journal. Standing mode binds caller/desk/role without workflow fields or an instance store. Workflow mode additionally binds canonical instance/node/attempt, with the same adapter, lifecycle and refusal semantics. No workflow Go import, alternate work store or provider-specific supervisor.
2. Start only a journal-admitted invocation with an approved immutable profile, validated assignment/reservation reference, current owner/claim generations, current protected incarnation and bounded budget. Recheck role, profile, stop state, generations, expiry and budget at every child launch and receipt delivery. Share capacity/consumption through the journal across callers and supervisor instances; no process-local mutex as the durable capacity proof.
3. Persist/recover stable identity around caller intent, journal admission receipt and provider acknowledgment. Duplicate requests return/reconcile the same invocation; unknown launch or stop holds until independently reconciled, including missing receipt/database data and crash windows. Bounded restart must not invent another unknown request. Preserve usage unknownness, limits, spent allowance and cancellation/checkpoints through reopen and passive restore.
4. Implement observe/cancel/reconcile and idle retention only for declared supported adapter capabilities. Timers, unchanged observations, repeated receipts and timeout/cancel reconciliation run in code and never call Start as a progress heartbeat or invoke a model waiter. Completion delivery does not count as work acceptance. Viewer attach/detach has no launch path.
5. Expose only sanitized typed lifecycle state for the caller binding, mode, optional workflow identity, model/profile, usage completeness and held state. Refuse unsupported mandatory fields and stale generation. Errors and status exclude argv payloads, credentials and adapter errors that can echo them. Preserve all runner conformance, generation-fence and independent-control fixtures.
6. Deliver both-mode, duplicate/crash/unknown-stop/accounting, fail-closed and zero-wait-launch production-path tests using stateful fake adapters. Link real journal qualification from /41, preserving its environment limits; do not substitute in-memory restart evidence for PostgreSQL. Add each required fail-first mutation and update owner docs/changelog.

## Interface contract

Compose the existing runner contract, moved process owner and durable journal in one loopadmin supervisor. It owns launch lifecycle and deterministic waits, never queue selection, graph progression, credentials or operator profile administration. Keep real admission at the supervisor boundary and the adapter fence at the lower execution boundary; an earlier caller check cannot substitute for either. Caller identity, invocation and current generations remain attached to every typed state, receipt and evidence item. Unknown stays unknown; process success is never work acceptance.

## Verify

The parent-row number in Expect identifies the unchanged obligation this child implements. These tests are prospective deliverables, not observed results. Preserve the original named selectors even where their existing contract names exceed the worker naming preference. Use targeted bounded package runs in the worker; CI owns the full matrix.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminBothModesNoGraph$' ./supervisor/... > "$o" && grep -F -- '--- PASS: TestLoopAdminBothModesNoGraph ' "$o")` | Parent row 1: exit 0; named PASS. Both modes cross one production supervisor/fake adapter; standing composition works with workflow absent. Importing workflow into the supervisor fails the dependency guard exercised by this test. |
| 2 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminUnknownRestartAndAccounting$' ./supervisor/... > "$o" && grep -F -- '--- PASS: TestLoopAdminUnknownRestartAndAccounting ' "$o")` | Parent row 2: exit 0; named PASS. Crash between caller intent and journal receipt reconciles once; unknown cancel holds relaunch; usage and limits survive reopen. Relaunch-on-missing-receipt fails. |
| 3 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminFailClosed$' ./supervisor/... > "$o" && grep -F -- '--- PASS: TestLoopAdminFailClosed ' "$o")` | Parent row 5: exit 0; named PASS. Unrecognized caller, unreadable/missing profile or role binding and absent/unreadable budget each start no child. Treating absent budget as unlimited fails. |
| 4 | check +flow +mutation | `(cd loopadmin && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -timeout 90s -v -run '^TestLoopAdminWaitDoesNotInvokeModel$' ./supervisor/... > "$o" && grep -F -- '--- PASS: TestLoopAdminWaitDoesNotInvokeModel ' "$o")` | Parent row 13: exit 0; named PASS. Production timers, unchanged observations and repeated receipts start zero additional invocations; one admitted job starts once and delivers completion in code. Observe-timeout Start mutation fails. |
| 5 | check:ci | `(consumer_out=$(mktemp) && trap 'rm -f "$consumer_out"' 0 && statusgen --root . --consumers --brief graph-execution/42 > "$consumer_out" && cat "$consumer_out" && grep -Eqx -- 'summary: [1-9][0-9]* corroborated, 0 disproved, 0 unchecked, 0 brief\(s\) claiming nothing' "$consumer_out")` | Exit 0 at the implementation delivery: each `consumers:` entry flips to fixed-here where the diff edits it and corroborates; none disproved or unchecked. With `--brief` and no `--base`/`--delivery`, statusgen pins the run to the delivery its committed Verification-Table marker records; if that marker is absent the run is could-not-check and the verifier pins `--delivery <implementation merge> --base <its first parent>` explicitly. A routing claim the diff contradicts fails. |

## Pre-mortem and dispatch checks

A retry creates a second unknown invocation: row 2. Role/cell generation is mistaken for a work claim: rows 1–3 use stale and mismatched bindings independently. Observation launches a heartbeat model: row 4 counts actual adapter Start. Standing mode quietly needs graph data: row 1 excludes workflow. Unknown usage becomes zero during reopen: row 2 preserves incomplete readings.

Rows discriminate through the named production mutations; every claimed fact is source-pinned above. Final design adequacy, the mandatory custody API and source/fixture fidelity are independent review obligations, not presence checks. All four parent risk flags and its human gate are conserved. This split authors no runtime profile or operational permission.

## Evidence

<!-- A non-implementer records actual command, exit, named result, immutable subject/environment and date. No prospective row is claimed passed by authoring. -->

## Review

Gate: human. Parent decision issue #5492 remains the decision surface for the complete /26 promise. No child approval is inferred from an empty issue or from authoring this split. At design pickup, present the concrete child acceptance candidate under the existing decision process before activation or final acceptance. The proposed decomposition must be reviewed before dispatch. "Decomposition review" is one event: a driver comment on #5492, under the driver's own login, accepting this split and its routing. Until then the `gates:` edge on #5492 holds this child off Next-up (a forge-backed gate is could-not-check, so held); a follow-up PR citing that comment removes the edge.
