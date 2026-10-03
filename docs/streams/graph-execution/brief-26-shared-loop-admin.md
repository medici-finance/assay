---
brief: assay:assay:graph-execution:26
title: Shared loop-admin launch supervision for desks and workflows
why: A reusable launch supervisor lets standing desks and graph stages share provider adapters, cancellation, recovery
  and cost controls without sharing scheduling authority.
wave: 1
depends:
- graph-execution/20
unblocks:
- graph-execution/22
- graph-execution/27
- graph-execution/21
effort: M
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Adds an authenticated launch endpoint and moves the process runner the process-launch audit scans; the owner confirms callers cannot change role, profile or limits and that the audit keeps sight of every exec site.
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
- 'workflow/controller: follow-up graph-execution/21'
- 'loopadmin/adapters: follow-up graph-execution/22 (consumes the supervisor and the moved process runner)'
- 'tools/desk/internal/loopengine: follow-up graph-execution/27'
- 'tools/desk/internal/cellprocess: follow-up graph-execution/26 (this brief; moved into loopadmin/process; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/26 (this brief; cadence path calls the moved runner; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/internal/forgeban: follow-up graph-execution/26 (this brief; process-launch audit also scans loopadmin; flips to fixed-here when the implementation edits the path)'
- 'operator lifecycle and cockpit: out-of-scope (adopter qualification)'
version: 1
id: bcbee0b0-8f12-4e7a-b028-0637b2bab0c3
---

# Brief 26 — Shared loop-admin launch supervision for desks and workflows

## Context

files: `loopadmin/supervisor/` (planned), `loopadmin/process/` (planned), `tools/desk/internal/cellprocess/` (moved into loopadmin/process by this brief), `tools/desk/cmd/cellctl/cadence.go`, `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/internal/forgeban/allowlist.go`, `tools/desk/internal/deskkit/forge_surface_test.go`, `docs/cellctl-cadence.md`, `loopadmin/journal/` (planned), `loopadmin/control/` (planned), `loopadmin/cmd/loop-admin/` (planned), `loopadmin/testdata/supervisor/` (planned), `loopadmin/README.md` (planned), `docs/loop-admin.md` (planned), `.github/workflows/loopadmin.yml` (planned), `changelog/graph-execution-26-loop-admin.md` (planned).

facts: The new loopadmin module is bootstrapped by /20. At main 307fe1699 `tools/desk/internal/cellprocess` already runs one harness invocation and reaps its process group or job, and cellctl's cadence path (`tools/desk/cmd/cellctl/cadence.go`) calls it; the process-launch audit (`TestNoForgeCLIShellout` over the `tools/desk` tree, with the runner's exec site in `tools/desk/internal/forgeban/allowlist.go`) covers it there. `tools/desk/internal/loopengine` owns in-session queue behavior for standing desks; it launches no process. Workflow instance ownership remains outside the supervisor; the shared module has no dependency on the instance store (19). Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the caller-scoped launch endpoint (caller namespace, approved profile, role, generation, shared capacity) — behind it, the per-role cadence lease (27) and the role executors' own credentials, which a direct lower-boundary call still cannot obtain.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The supervisor runs offline fixtures only, enrolls no live desk, and holds no effect credential or operator administration, but it adds an authenticated launch endpoint and moves the process runner the process-launch audit scans: once it is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

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

1. Move `tools/desk/internal/cellprocess` into `loopadmin/process` as the module's one process runner, without copying it: keep its process-tree containment and uncertain-cleanup result unchanged, delete the old package, and re-point cellctl's cadence path at the moved package through a `require` plus relative `replace` of the loopadmin module in `tools/desk/go.mod` (the pattern `tools/loopresolve/go.mod` uses for `cellconfig`); cellctl's existing cadence tests stay green unchanged, and the tools/desk release build must resolve the relative `replace`. Extend the process-launch audit so it also scans `loopadmin/` and re-key the runner's exec-site entry to the moved path; the audit must not lose sight of that exec site. Implement one shared process/session supervisor over /20 and that runner. Accept only authenticated caller-scoped, operator-approved launch profiles and validated assignment/reservation references. Support standing-desk and workflow-stage modes with identical adapter and lifecycle semantics. An operator may run separate per-role processes of the same binary; reuse does not imply a privileged all-role daemon.
2. Persist launch intent before starting a child in an execution journal keyed by caller namespace, invocation ID and owner generation. Keep adapter receipts, lifecycle state, cancellation, limits, usage and checkpoints. The journal is the second local embedded store the program's scope exception permits (a per-supervisor record on the supervisor's own filesystem; this brief selects and qualifies its embedded engine, with SQLite the expected choice, and loopadmin takes no dependency on workflow/); it is not a shared or distributed store. This journal owns process facts only; desk queues and the instance store (19) retain authoritative work/acceptance state. Caller intent and journal receipt reconcile by stable request ID, including a crash between their separate commits; do not claim a transaction across stores.
3. Implement start/observe/cancel/reconcile and idle session retention only when the adapter declares support. Duplicate input cannot launch twice; unknown launch/stop holds until reconciled; bounded restarts never create another unknown request. Recheck role/profile/generation and budget at every child launch and receipt delivery. Shared concurrency reservations bound all callers using this profile; preserve consumption after crash/restore.
4. Keep runtime launch authority separate from operator profile/install/credential administration. A desk client may request an allowed child invocation through a bounded, capability-scoped endpoint, but cannot choose arbitrary argv, escalate role, raise limits or recursively create unconstrained agents. Effect credentials and operator admin are absent; any inference credential stays confined to its approved adapter boundary and is excluded from packets, artifacts and logs. Fail closed: an unrecognised caller, an unreadable or missing profile or role-binding configuration, and an absent or unreadable budget each refuse the launch; none falls back to a default profile, role or limit.
5. Export sanitized process/session status with mode, caller binding and optional canonical workflow references. Viewer attach/detach never starts a process. The default command uses offline fixtures; live profile activation is separate. Provide backup/restore of the execution journal with referenced artifacts, preserving pause/spend/unknowns; restore is passive. Document state ownership and refusal behavior.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminBothModesNoGraph$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminBothModesNoGraph " "$routing_out")` | exit 0; named PASS; both caller modes launch via one fake adapter; standing mode starts with the instance store absent; mutation: import the workflow module from the supervisor — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminUnknownRestartAndAccounting$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminUnknownRestartAndAccounting " "$routing_out")` | exit 0; named PASS; crash between caller intent and launch receipt reconciles once; unknown cancel blocks relaunch; usage and limits survive restart; mutation: relaunch on restart when the launch receipt is missing — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminCallerIsolationAndDirectBypass$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminCallerIsolationAndDirectBypass " "$routing_out")` | exit 0; named PASS; a direct lower-boundary call cannot change role/profile, reuse another caller receipt or exceed shared capacity; operator actions are unreachable; mutation: skip the caller-namespace check on receipt lookup — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestProcessLaunchAuditCoversLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestProcessLaunchAuditCoversLoopAdmin " "$routing_out")` | exit 0; named PASS; the process-launch audit scans `loopadmin/` as well as `tools/desk`, finds the moved runner's exec site in its register, and fails on an unregistered exec site planted in a `loopadmin/` fixture; mutation: drop the `loopadmin/` root from the audit's scan — the named test must fail |
| 5 | check:ci +flow +mutation | `(cd loopadmin && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestLoopAdminFailClosed$" ./... > "$routing_out" && grep -q -- "--- PASS: TestLoopAdminFailClosed " "$routing_out")` | exit 0; named PASS; an unrecognised caller, an unreadable profile or role-binding configuration and an absent budget each refuse the launch and start no child; mutation: treat an absent budget as unlimited — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
