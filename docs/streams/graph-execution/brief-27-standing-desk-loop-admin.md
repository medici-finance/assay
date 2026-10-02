---
brief: assay:assay:graph-execution:27
title: Standing desk clients for the shared loop-admin
why: The existing specialist desks should gain the shared launcher before workflow adoption, while keeping their
  queue rules and human gates.
wave: 2
depends:
- graph-execution/26
- graph-execution/22
unblocks: []
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
- 'configured standing desks: follow-up graph-execution/27'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/27 (enrolled roles launch through loopadmin; legacy launch path kept for non-enrolled roles)'
- 'tools/desk/internal/cellcadence: follow-up graph-execution/27 (enrolled roles hold the same per-role lease)'
- 'operator adoption: out-of-scope (per-role profile and custody qualification)'
- 'workflow/controller: out-of-scope (separate caller delivered by graph-execution/21)'
version: 1
id: af7e88a8-b411-40f9-b205-6b93f04ebed8
---

# Brief 27 — Standing desk clients for the shared loop-admin

## Context

files: `tools/desk/internal/loopengine/loopadmin.go` (planned), `tools/desk/internal/loopengine/loopadmin_test.go` (planned), `tools/desk/internal/deskkit/loopadminprofile.go` (planned), `tools/desk/internal/deskkit/loopadminprofile_test.go` (planned), `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/cellctl/cadence.go`, `tools/desk/internal/cellcadence/`, `loopadmin/clients/desk/` (planned), `plugins/assay/references/loop-admin.md` (planned), `plugins/assay/skills/{the-desk,intake-desk,worker-desk,pr-review-desk,verify-desk}/SKILL.md` (mode-selection integration only), `loopadmin/testdata/desk/` (planned), `docs/loop-admin.md` (planned), `.github/workflows/ci.yml`, `changelog/graph-execution-27-standing-desks.md` (planned).

facts: The new loopadmin module is bootstrapped by /20. At main 307fe1699 a standing desk is launched by cellctl's launch path — `deskLaunch` in `tools/desk/cmd/cellctl/launch.go`, then `runInteractiveHarness` or `runCadencedHarness` in `tools/desk/cmd/cellctl/cadence.go` — under the per-role lease and checkpoint of `tools/desk/internal/cellcadence`, which refuses to replace an owner that did not finish. That cellctl path is the legacy desk launch path this brief transfers enrolled roles from. `tools/desk/internal/loopengine` owns in-session queue behavior for engine-backed desks. Workflow instance ownership remains outside the supervisor; the shared module has no dependency on the instance store (19). Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the per-role cadence lease that admits one launcher per role binding — behind it, 26's caller-scoped endpoint refusing role or profile mismatch, and the unchanged review/verify actors and effect executors.

risk-answers: all `no` because the five-role run is an offline fixture, no live desk is enrolled, enrollment is per-role opt-in and reversible through the drain/reconcile/fence rollback, and credential custody does not change. It edits tools/desk launch code, CI and five role skills, so the implementing change needs an independent security review before it is marked ready.

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

1. Bind the five configured desk roles (coordinator, intake, worker, review and verify) as independent standing-desk clients of the shared /26 supervisor. Preserve each existing queue, claim, prompt/skill, model profile, review boundary and next-item policy. The existing desk skills and queues remain work owners. For engine-backed consumers, loopengine/engine.go, journal.go and recover.go retain that ownership; add a runner bridge rather than a second queue engine. The frozen loopengine.Loop interface (SelectQueue/TierPolicy/Dispatch/Land/OnIdle) is unchanged: adapt Dispatch/Handle, never add hooks. Skill-driven coordinator/intake/review desks use the same client without pretending they are engine-backed. No workflow store, pattern instance or workflow controller is required.
2. Support a standing role session plus bounded child invocations where its profile permits them. Retain useful context only through declared adapter session capability; durable role/queue checkpoints survive loss of that session. Idle/no actionable input performs no new model request. A terminal is an optional attachment, not the identity or life-support of the desk. Reconnect and cockpit refresh never launch a duplicate.
3. For an enrolled role, use one launch path — cellctl hands the role's resolved approved profile to the shared /26 supervisor through the /22 adapter — for both the standing role process and its delegated specialist children. Map existing desk claim/reservation/stop controls into the common envelope; reject role/profile mismatch and unsupported capability. Carry canonical work references for child tasks when available without fabricating graph instances. Preserve independent review/verification actors and existing effect executors; launch completion never declares work done.
4. Implement explicit profile-selected opt-in with the legacy desk launch path (cellctl's `deskLaunch` → `runInteractiveHarness`/`runCadencedHarness`) retained unchanged for non-enrolled roles. For a given desk binding, only one launcher owns each generation: an enrolled role holds the same `cellcadence` per-role lease while the shared supervisor runs it, so a legacy launch of that role is refused, and an unfinished checkpoint maps to a held unknown stop rather than a relaunch. Transfer and rollback stop admission, reconcile old attempts/effects and fence before replacement; unknown stop cannot be treated as successful handback. Migrating the launcher does not itself authorize a credential-custody change.
5. Run an offline five-role fixture through the actual loopengine bridge and common supervisor/adapter, including child work, next-item progress, idle behavior, cancellation and restart. Enroll no live desk. Document source/consumer CI paths and the separately qualified per-role adoption procedure.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskFiveRolesThroughLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskFiveRolesThroughLoopAdmin " "$routing_out")` | exit 0; named PASS; all five role profiles and permitted child launches use the same shared supervisor with workflow controller/storage absent; mutation: route one enrolled role through the legacy cellctl launch path — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskIdleAttachAndNextItem$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskIdleAttachAndNextItem " "$routing_out")` | exit 0; named PASS; idle/reconnect launches nothing; completed receipt reaches existing desk logic and only a new actionable item invokes again; mutation: launch a model request on viewer attach — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskLauncherTransferUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskLauncherTransferUnknown " "$routing_out")` | exit 0; named PASS; legacy and shared launchers cannot own the same binding generation — with an enrolled role running, a cellctl legacy launch of that role is refused by the cellcadence per-role lease; unknown stop holds rollback and direct role escalation is denied; mutation: skip the cellcadence per-role lease when the shared launcher starts an enrolled role — the named test must fail |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
