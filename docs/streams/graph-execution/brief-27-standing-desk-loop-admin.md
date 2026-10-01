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
- freshness-checked 2026-10-02 @ a944ad1103aadaba919c11fe425089057f5c2f4e
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'configured standing desks: fixed-here'
- 'operator adoption: out-of-scope (per-role profile and custody qualification)'
- 'workflow/controller: out-of-scope (separate caller delivered by graph-execution/21)'
version: 1
id: af7e88a8-b411-40f9-b205-6b93f04ebed8
---

# Brief 27 — Standing desk clients for the shared loop-admin

## Context

files: `tools/desk/internal/loopengine/loopadmin.go` (planned), `tools/desk/internal/loopengine/loopadmin_test.go` (planned), `tools/desk/internal/deskkit/loopadminprofile.go` (planned), `tools/desk/internal/deskkit/loopadminprofile_test.go` (planned), `tools/desk/go.mod` (planned), `tools/desk/go.sum` (planned), `loopadmin/clients/desk/` (planned), `plugins/assay/references/loop-admin.md` (planned), `plugins/assay/skills/{the-desk,intake-desk,worker-desk,pr-review-desk,verify-desk}/SKILL.md` (mode-selection integration only), `loopadmin/testdata/desk/` (planned), `docs/loop-admin.md` (planned), `.github/workflows/ci.yml` (planned), `changelog/graph-execution-27-standing-desks.md` (planned).

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

1. Bind the five configured desk roles (coordinator, intake, worker, review and verify) as independent standing-desk clients of the shared /26 supervisor. Preserve each existing queue, claim, prompt/skill, model profile, review boundary and next-item policy. The existing desk skills and queues remain work owners. For engine-backed consumers, loopengine/engine.go, journal.go and recover.go retain that ownership; add a runner bridge rather than a second queue engine. The frozen loopengine.Loop interface (SelectQueue/TierPolicy/Dispatch/Land/OnIdle) is unchanged: adapt Dispatch/Handle, never add hooks. Skill-driven coordinator/intake/review desks use the same client without pretending they are engine-backed. No workflow store, pattern instance or workflow controller is required.
2. Support a standing role session plus bounded child invocations where its profile permits them. Retain useful context only through declared adapter session capability; durable role/queue checkpoints survive loss of that session. Idle/no actionable input performs no new model request. A terminal is an optional attachment, not the identity or life-support of the desk. Reconnect and cockpit refresh never launch a duplicate.
3. Use one launch path for both the standing role process and its delegated specialist children. Map existing desk claim/reservation/stop controls into the common envelope; reject role/profile mismatch and unsupported capability. Carry canonical work references for child tasks when available without fabricating graph instances. Preserve independent review/verification actors and existing effect executors; launch completion never declares work done.
4. Implement explicit profile-selected opt-in with the legacy desk launch path retained for non-enrolled roles. For a given desk binding, only one launcher owns each generation. Transfer and rollback stop admission, reconcile old attempts/effects and fence before replacement; unknown stop cannot be treated as successful handback. Migrating the launcher does not itself authorize a credential-custody change.
5. Run an offline five-role fixture through the actual loopengine bridge and common supervisor/adapter, including child work, next-item progress, idle behavior, cancellation and restart. Enroll no live desk. Document source/consumer CI paths and the separately qualified per-role adoption procedure.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskFiveRolesThroughLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskFiveRolesThroughLoopAdmin " "$routing_out")` | exit 0; named PASS; all five role profiles and permitted child launches use the same shared supervisor with workflow controller/storage absent |
| 2 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskIdleAttachAndNextItem$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskIdleAttachAndNextItem " "$routing_out")` | exit 0; named PASS; idle/reconnect launches nothing; completed receipt reaches existing desk logic and only a new actionable item invokes again |
| 3 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskLauncherTransferUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskLauncherTransferUnknown " "$routing_out")` | exit 0; named PASS; legacy and shared launchers cannot own the same binding generation; unknown stop holds rollback and direct role escalation is denied |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Fail a deliberate mutation of the named control. Facts/source revisions, declared files, risks, consumers and sizing were checked at authoring; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: model. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.
