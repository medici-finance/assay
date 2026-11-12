---
brief: assay:assay:graph-execution:27
title: Standing desk clients for the shared loop-admin
why: The existing specialist desks should gain the shared launcher before workflow adoption, while keeping their
  queue rules and human gates.
wave: 8
depends:
- graph-execution/26
- graph-execution/22
- desk-supervision/34
- desk-tools/39
- desk-tools/42
- desk-tools/45
- desk-tools/43
- desk-tools/51
unblocks:
- agent-execution/05
- "agent-execution/09"
- "desk-containers/33"
effort: L
gate: human
risk:
  regulatory: 'no'
  customer: 'no'
  irreversible: 'yes'
  sensitive-data: 'no'
gate-why: Transfers launch of the standing desk roles and edits shipped launch, lease, CI and role-skill files; the owner confirms one launcher per role, that existing stop controls still stop an enrolled role, and that credential custody does not change.
issues: []
schema: brief-v2
authored: 2026-10-02 by task-workflow authoring session
sources:
- "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
- "medici-finance/assay#3353 — cell startup scratch scan; measurements on v1.0.48"
- "medici-finance/assay#3099 — remaining parent launch telemetry coverage"
- "freshness-checked 2026-10-20 @ eb82d5b62 (merged main, installed v1.0.51); the 2026-10-18 v1.0.49 install recorded source 3cb57291e"
- docs/streams/graph-execution/task-workflow-program.md
- freshness-checked 2026-10-02 @ 307fe16992caef53fa46c52622753dd400c7b42a
exec-tier: strong
exec-tier-why: Durable state, authority and cross-component failure cases require design judgment.
domain: complicated
consumers:
- 'configured standing desks: follow-up graph-execution/27 (this brief; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/cmd/cellctl: follow-up graph-execution/27 (this brief; enrolled roles launch through loopadmin; legacy launch path kept for non-enrolled roles; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/internal/cellcadence: follow-up graph-execution/27 (this brief; enrolled roles hold the same per-role lease; flips to fixed-here when the implementation edits the path)'
- 'operator adoption: out-of-scope (per-role profile and custody qualification)'
- 'workflow/controller: out-of-scope (separate caller delivered by graph-execution/21)'
- 'plugins/assay/skills/worker-desk/SKILL.md desk-supervision/34 usage-record step: follow-up graph-execution/27 (this brief; for an enrolled worker role the step is skipped because the /26 journal records that run''s usage; flips to fixed-here when the implementation edits the path)'
- 'tools/desk/cmd/deskusage/ record path (desk-supervision/34''s verb; the enrolled-worker skip): follow-up graph-execution/27 (this brief; `deskusage record` writes nothing for an enrolled worker role, Verify row 5; flips to fixed-here when the implementation edits the path)'
version: 11
id: af7e88a8-b411-40f9-b205-6b93f04ebed8
---

# Brief 27 — Standing desk clients for the shared loop-admin

> **Requested amendment — 25 October 2026:** the [desk-instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and the Task/Verify additions below amend this unfinished delivery. Earlier singleton/sequential source accounts remain history; unchanged acceptance and gates remain.


## Context

files: `tools/desk/internal/loopengine/loopadmin.go` (planned), `tools/desk/internal/loopengine/loopadmin_test.go` (planned), `tools/desk/internal/deskkit/loopadminprofile.go` (planned), `tools/desk/internal/deskkit/loopadminprofile_test.go` (planned), `tools/desk/go.mod`, `tools/desk/go.sum`, `tools/desk/cmd/cellctl/launch.go`, `tools/desk/cmd/cellctl/cadence.go`, `tools/desk/internal/cellcadence/`, `loopadmin/clients/desk/` (planned), `plugins/assay/references/loop-admin.md` (planned), `plugins/assay/skills/{the-desk,intake-desk,worker-desk,pr-review-desk,verify-desk}/SKILL.md` (qualified enrolled-mode integration and mechanical-loop retirement only; judgments, STOP/guard-refusal/trust-gate prose and non-enrolled behavior preserved), `tools/desk/cmd/deskusage/record.go` (planned) (desk-supervision/34 creates it; this brief adds the enrolled-worker skip only), `loopadmin/testdata/desk/` (planned), `docs/loop-admin.md` (planned), `.github/workflows/ci.yml`, `changelog/graph-execution-27-standing-desks.md` (planned), `tools/desk/internal/deskclient/`, `tools/desk/internal/reviewflow/`, `tools/desk/internal/issueflow/`, `tools/desk/cmd/cellctl/{up.go,down.go,prepare_test.go}` — Additional role-policy adapter scope: `tools/desk/internal/deskclient/` (planned, tools/desk-local binding only); consume `tools/desk/internal/reviewflow/` from desk-tools/39 and `tools/desk/internal/issueflow/` from desk-tools/42 without copying their semantics. Additional startup scope: `tools/desk/cmd/cellctl/{up.go,down.go,prepare_test.go}` (integration with /26; no new preparation engine).

facts: The new loopadmin module is bootstrapped by /20. At main 307fe1699 a standing desk is launched by cellctl's launch path — `deskLaunch` in `tools/desk/cmd/cellctl/launch.go`, then `runInteractiveHarness` or `runCadencedHarness` in `tools/desk/cmd/cellctl/cadence.go` — under the per-role lease and checkpoint of `tools/desk/internal/cellcadence`, which refuses to replace an owner that did not finish. That cellctl path is the legacy desk launch path this brief transfers enrolled roles from. `tools/desk/internal/loopengine` owns in-session queue behavior for engine-backed desks. Workflow instance ownership remains outside the supervisor; the shared module has no dependency on the instance store (19). Existing role capabilities and human merge/verification gates remain binding. All named commands/tests below are implementation deliverables, not tests already run.

single-point-of-failure: the per-role cadence lease that admits one launcher per role binding — behind it, 26's caller-scoped endpoint refusing role or profile mismatch, and the unchanged review/verify actors and effect executors.

risk-answers: `irreversible: yes`, so `gate: human`, as for 14 and 16 in this stream. The five-role run is an offline fixture, no live desk is enrolled, enrollment is per-role opt-in and reversible through the drain/reconcile/fence rollback, and credential custody does not change, but the brief edits shipped launch and lease code, CI and five role skills: once it is implemented and verified, a human check skipped at verification cannot be recovered by a revert. Lowering the gate is a maintainer ruling, not an authoring choice.

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

## Candidate and priority amendment — 2026-10-23

Bind one role-qualified candidate-plus-priority planning entrypoint in the existing
internal/deskclient composition surface for both prose/skill and discrete clients. Compose
reviewflow (/39), issueflow (/42), canonical verify queue (/45), existing worker lanes and
coordinator policy; do not promote their meanings into a new universal score or clone them
inside loopadmin. Both paths consume identical candidate/policy/evaluator/time/capacity
inputs and return the same ordered proposals, held/unknown dispositions and reasons.

Separate observation invalidation/due collection from SelectQueue/refill. A completion or
priority change can rerun pure selection with zero GH calls. Source membership, permission,
expiry and changed facts use the existing bounded refresh/hold contract; a full pool is not
permission to skip due liveness/STOP checks. Preserve Loop and review reactor interfaces,
claim authority, bounded judgment and live admission/effect rechecks. Keep unqualified roles
explicitly unavailable instead of parsing planner prose or claiming five-role parity from
mocks. /45 is a real prerequisite and its reciprocal edge is added with this amendment.
Add the named case in the already-declared internal/deskclient test scope. Use /44's ranked
cost cohorts; no new scheduler, service, credential store or graph-instance prerequisite.

## Desk-instance scope — 25 October 2026

Read the [instance contract](../../proposals/non-ci-architecture/30-desk-instances-and-rolling-replacement.md) and refresh affected active claims/PRs before implementation. This is a source-pinned **adapt** handoff from public main `57196d5da`, not evidence of deployment. The Task and Verify additions below carry this owner's delta; unchanged obligations remain.

## Task

1. Bind the five configured desk roles (coordinator, intake, worker, review and verify) as independent standing-desk clients of the shared /26 supervisor. Preserve each existing queue, claim, prompt/skill, model profile, review boundary and next-item policy. The existing desk skills and queues remain work owners. For engine-backed consumers, loopengine/engine.go, journal.go and recover.go retain that ownership; add a runner bridge rather than a second queue engine. The frozen loopengine.Loop interface (SelectQueue/TierPolicy/Dispatch/Land/OnIdle) is unchanged: adapt Dispatch/Handle, never add hooks. Skill-driven coordinator/intake/review desks use the same client without pretending they are engine-backed. No workflow store, pattern instance or workflow controller is required.
2. Run mechanical role progression as deterministic code, using bounded model invocations only for work/judgment selected by the existing role policy. A retained role session is optional adapter-supported context, never an always-running model conductor. Support bounded child invocations where the approved profile permits them. Retain useful context only through declared adapter session capability; durable role/queue checkpoints survive loss of that session. Idle/no actionable input performs no new model request. A terminal is an optional attachment, not the identity or life-support of the desk. Reconnect and cockpit refresh never launch a duplicate.
3. For an enrolled role, use one launch path — cellctl hands the role's resolved approved profile to the shared /26 supervisor through the /22 adapter — for both the standing role process and its delegated specialist children. Map existing desk claim/reservation/stop controls into the common envelope; reject role/profile mismatch and unsupported capability. Carry canonical work references for child tasks when available without fabricating graph instances. Per-run usage for an enrolled role is the /26 journal's `runner.Usage` record: where the worker-desk skill carries desk-supervision/34's `deskusage record` step (amended 2026-10-13), the record is skipped for an enrolled worker role and kept for a non-enrolled one, so one run never has two usage records. The skip is code, not skill prose: `deskusage record` reads the worker role's enrollment from the binding that `loopadminprofile.go` resolves and, for an enrolled role, writes nothing to `usage.jsonl`, prints `skipped: worker role enrolled on loop-admin` and exits 0, so the skill step carries no condition and the refill is not blocked; an unreadable binding refuses the record (exit 5, nothing written) rather than guessing either way. Verify row 5 needs desk-supervision/34's `tools/desk/cmd/deskusage`, so this brief depends on desk-supervision/34: while that verb is absent there is no record step to skip, and the row fails rather than passing vacuously. Preserve independent review/verification actors and existing effect executors; launch completion never declares work done. Fail closed: an unrecognised role binding, an unreadable profile or binding configuration and an absent budget refuse the launch, and an existing desk stop control set for an enrolled role holds every launch for that role.
4. Implement explicit profile-selected opt-in with the legacy desk launch path (cellctl's `deskLaunch` → `runInteractiveHarness`/`runCadencedHarness`) retained unchanged for non-enrolled roles. For a given desk binding, only one launcher owns each generation: an enrolled role holds the same `cellcadence` per-role lease while the shared supervisor runs it, so a legacy launch of that role is refused, and an unfinished checkpoint maps to a held unknown stop rather than a relaunch. Transfer and rollback stop admission, reconcile old attempts/effects and fence before replacement; unknown stop cannot be treated as successful handback. Migrating the launcher does not itself authorize a credential-custody change.
5. Run an offline five-role fixture through the actual loopengine bridge and common supervisor/adapter, including child work, next-item progress, idle behavior, cancellation and restart. Enroll no live desk. Document source/consumer CI paths and the separately qualified per-role adoption procedure.

- **Instance amendment:** Bind worker/review clients per instance rather than one client per role; other role cardinalities stay singleton. Preserve one canonical shared queue/claim owner: workers divide items and review clients divide repository/PR/lane/head selections with existing claim keys/current-head checks. Map instance stops plus stronger role/cell stops and existing enrollment through the shared owner. Legacy and managed launchers cannot independently own one instance; refill uses a role-wide best-effort estimate, not a central slot allocator. Instruction/docs changes must retire the one-window-per-role assumption without copying queue policy.

## Token containment and role migration amendment — 2026-10-18

Implement the [single delivery contract](task-workflow-program.md#standing-desk-efficiency-delivery--2026-10-18)
through existing queues and policy APIs. Add role integration and fixtures in the already
declared `loopadmin/clients/desk/`, loopengine bridge and `loopadmin/testdata/desk/`;
update the existing-owner `docs/loop-admin.md` (planned) adoption/rollback contract. No new daemon,
coordinator role, claim authority or generic workflow scheduler. Role-policy bindings
live under tools/desk/internal/deskclient, where reviewflow, issueflow and loopengine
are legally importable. The loopadmin client exposes the typed launch protocol only;
it cannot import tools/desk/internal or cmd/main packages, own role credentials or
reimplement the review classifier. desk-tools/39 and desk-tools/42 are real prerequisites: if their
importable policy contract is absent, the role integration holds instead of parsing
human planner text or copying a predicate. Coordinator judgments remain bounded model
work through the existing coordinator policy; no generic model supervisor is added.

1. Code performs the ordinary queue sweep, claim/slot bookkeeping, CI/status observation,
   retry/backoff and result collection. The existing policy owner decides applicability,
   due reevaluation and when an unresolved judgment needs a bounded model assignment.
   Hold missing/incomplete observations; an unchanged blocked state does not dispatch
   a model to restate it. Event duplication must not create duplicate work. The frozen
   Loop interface and existing independent role policies remain intact.
2. Give each new item/lane a bounded assignment containing its exact subject, approved
   role/profile, required contract/packet paths and prior unresolved findings. Do not
   inherit unrelated supervisor history or erase required reads/checks to reduce tokens.
   Same-item continuity is capability-qualified; it does not make an agent a permanent
   polling slot. Keep one routine watch owner; independent verdict/readiness reads remain.
3. Prove the full five-role flow offline with qualified Codex and Claude profiles and
   workflow/controller storage absent. Each role must select, dispatch, collect and
   reach its next-item/hold outcome through production adapters; neither a mocked
   role policy nor successful process exit can stand in for accepted work.
4. Publish per-role adoption and rollback using the existing cellcadence lease and
   drain/reconcile/fence contract. The first proposed live desk cohort is review,
   then worker, then the remaining roles after each independent proof and operator
   approval; no live enrollment happens in this implementation brief. Remove the
   adopted mechanical skill loop in that role's own enrollment mode, retaining legacy
   instructions for non-enrolled consumers and all review/verification judgments.
   "Mechanical" never covers STOP, guard-refusal, trust-gate or escalation prose: each
   such line stays in the enrolled-mode skill text, or moves into code only together with
   a named test that fails when the code stops enforcing it.
5. Reuse desk-tools/37's cohorts and desk-tools/44's qualification record for a matched local comparison:
   standing/child model calls and tokens, completed items, coverage and rework. Reuse
   /26's invocation records plus desk-supervision/34's non-enrolled-worker compatibility path; count
   each run once, and keep unavailable usage unknown. Record qualified image, harness,
   launcher/plugin, model/profile, exact subject and cache state. Initial proof needs
   no performance-collection DB/API, new consent or live model run. A later adopting pilot
   measures real cost; offline tests establish deterministic launch counts, not dollars.

Already-shipped explicit cadence is the optional containment path described in the
program, not this milestone's zero-idle-launch completion claim. Mere prompt shortening,
`wait_agent` guidance or planner use cannot satisfy the tests below.

## Interface contract

Caller/invocation/subject and validated actor capability enter; typed state, evidence and receipts leave with that identity. Workflow mode additionally binds canonical instance/node/attempt; standing mode binds its desk/role and does not require graph fields. Unknown outcomes remain unknown. Consumers must refuse unsupported mandatory fields and stale generations.

### Cell startup integration — 2026-10-18

Consume /26's shared preparation and bounded storage admission; this brief does not create a
second maintenance owner. Include `up` for a five-role cell, direct `desk`, cadence restart,
concurrent starts and interrupted `down` in the existing offline five-role fixture. Exercise
both enrolled and non-enrolled roles with workflow instance storage absent. Task 4's legacy
path preservation means behavior and authority compatibility, including the /26 startup repair;
it does not freeze the expensive historical sweep. Attaching to an existing role starts nothing.

Keep one compatible cell preparation across roles and fresh per-role admission. Reuse never
bypasses stop flags, role leases, approved profiles, budget or owner-generation fences. Add
1x/10x protected history and a maintenance scan held mid-traversal: role launch must not walk
that history or wait for its full scan. Check protected bytes after restart/rollback. Record
first/all confirmed starts, incomplete requests and shared versus role costs using /26's
existing opt-in producer; no optional collector is needed. A missing phase is unknown.

The new row is an implementation deliverable, with the mutation as its pre-mortem control.
Existing custody, rollout, legacy rollback and human gates remain unchanged.

## Verify

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskFiveRolesThroughLoopAdmin$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskFiveRolesThroughLoopAdmin " "$routing_out")` | exit 0; named PASS; all five role profiles and permitted child launches use the same shared supervisor with workflow controller/storage absent; mutation: route one enrolled role through the legacy cellctl launch path — the named test must fail |
| 2 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskIdleAttachAndNextItem$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskIdleAttachAndNextItem " "$routing_out")` | exit 0; named PASS; idle/reconnect launches nothing; completed receipt reaches existing desk logic and only a new actionable item invokes again; mutation: launch a model request on viewer attach — the named test must fail |
| 3 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskLauncherTransferUnknown$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskLauncherTransferUnknown " "$routing_out")` | exit 0; named PASS; legacy and shared launchers cannot own the same binding generation — with an enrolled role running, a cellctl legacy launch of that role is refused by the cellcadence per-role lease; unknown stop holds rollback and direct role escalation is denied; mutation: skip the cellcadence per-role lease when the shared launcher starts an enrolled role — the named test must fail |
| 4 | check:ci +flow +mutation | `(cd tools/desk && routing_out=$(mktemp) && trap 'rm -f "$routing_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskStopHolds$" ./... > "$routing_out" && grep -q -- "--- PASS: TestStandingDeskStopHolds " "$routing_out")` | exit 0; named PASS; with the existing desk stop control set for an enrolled role, the shared supervisor launches nothing for that role, standing or child, until the stop clears; mutation: drop the stop-control mapping into the common envelope — the named test must fail |
| 5 | check:ci +flow +mutation +dereference | `(cd tools/desk && test -d cmd/deskusage && usage_out=$(mktemp) && trap 'rm -f "$usage_out"' 0 && GOWORK=off go test -count=1 -v -run "^TestStandingDeskEnrolledWorkerOneUsageRecord$" ./... > "$usage_out" && grep -q -- "--- PASS: TestStandingDeskEnrolledWorkerOneUsageRecord " "$usage_out")` | exit 0; named PASS; with `tools/desk/cmd/deskusage` absent the row exits 1. The test completes one worker run per arm through the offline fixture, with `ASSAY_USAGE_DIR` and the /26 journal in temp dirs, then calls the real `deskusage record --claim-key <key>` exactly as desk-supervision/34's Task 5 skill step does, and counts usage records for that run's `dispatch_ref` in BOTH stores: enrolled worker → journal 1, `usage.jsonl` 0; non-enrolled worker → journal 0, `usage.jsonl` 1; the total is exactly 1 in each arm; an unreadable binding → exit 5 and `usage.jsonl` byte-identical; mutation: keep the record for an enrolled worker role (the enrollment check in `deskusage record` reports not-enrolled) — `usage.jsonl` gains a second record for the run, the total is 2, and the named test must fail |
| 6 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStandingCellPrep$' ./... > "$o" && grep -F -- '--- PASS: TestStandingCellPrep ' "$o")` | exit 0; named PASS. Five-role mixed enrollment, direct starts and restart/rollback consume one preparation contract; 1x/10x history and in-flight maintenance cause no recursive admission walks, duplicate owner or data loss. Mutation: one role falls back to synchronous Sweep, or stale preparation bypasses a stop; test fails. |
| 7 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStandingDeskUnchangedWaitNoModel$' ./... > "$o" && grep -F -- '--- PASS: TestStandingDeskUnchangedWaitNoModel ' "$o")` | exit 0; named PASS. All five production role adapters with Codex and Claude fixtures perform due non-model checks but start zero model jobs for unchanged queues, pending CI, repeated errors/events and viewer attach. A changed eligible item starts exactly once and a freed slot refills. Mutation: invoke a model on an unchanged queue/CI tick; the named test fails. |
| 8 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStandingDeskBoundedJobsAndUsage$' ./... > "$o" && grep -F -- '--- PASS: TestStandingDeskBoundedJobsAndUsage ' "$o")` | exit 0; named PASS. Actual role/adapter flow keeps unrelated item histories separate, preserves prior findings and independent lanes, attributes supervisor plus child usage once, and keeps missing telemetry unknown. Mutation: hide child usage or carry an unrelated completed-item conversation into the next assignment; the named test fails. |
| 9 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStandingDeskQualifiedLoopRetirement$' ./... > "$o" && grep -F -- '--- PASS: TestStandingDeskQualifiedLoopRetirement ' "$o")` | exit 0; named PASS. Enrolled review mode uses code-owned progression and exactly one routine watcher while non-enrolled mode remains compatible; independent final reads and held unknown-stop rollback remain. Mutation: leave the legacy model conductor active beside the enrolled client; the named test fails. |
| 10 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestDeskPlannerParity$' ./... > "$o" && grep -F -- '--- PASS: TestDeskPlannerParity ' "$o")` | exit 0; named PASS. all five actual role bindings give equal ranked/selected/held results to prose and discrete clients on identical observations and policy; pure refill/reprioritization adds zero GH attempts. Second-root, capped, unmapped, duplicate, key-order and denied fixtures plus moved-head admission discriminate a copied ranker, repeated collection or omitted source. |
| I1 | check +flow +mutation | `(o=$(mktemp) && trap 'rm -f "$o"' 0 && cd tools/desk && GOWORK=off go test -count=1 -v ./cmd/cellctl/... -run '^TestStandingDeskInstances$' > "$o" 2>&1 && grep -F -- '--- PASS: TestStandingDeskInstances ' "$o")` | Named test executes (zero matching tests is not a pass); Two workers and three reviewers with mixed qualified profiles divide tasks/lanes, preserve a paused peer and produce one usage record per invocation; singleton duplicate role still refuses. Bypass upper selection: conflicting claim cannot launch. Mutation: prefix work claims with instance or retain a legacy conductor makes the test fail. |

## Pre-mortem and dispatch checks

The plausible wrong implementations are the negative cases named in Verify: stale acceptance, missing durable data, or a bypass that still returns success. An enrolled worker that still writes desk-supervision/34's `usage.jsonl` record gives one run two usage records; row 5 counts both stores. Each row must exercise production code and an independent expected outcome, not only serialization. Bypass the upper controller in at least one authority test. Each +mutation row names its mutation, and that mutation must turn the row's named test red. Facts, declared files, risks, consumers and sizing were re-checked against main at 307fe16992caef53fa46c52622753dd400c7b42a on 2026-10-02, after merging main; final implementation design adequacy remains review-only.

## Evidence

<!-- Independent verifier records command, exit, named result, subject/environment and date. -->

## Review

Gate: human. Confirm scope, exact-subject evidence, independent failure controls and cross-component flow. No test result changes merge authority.

## Execution-environment follow-on routing — 2026-10-16

Reciprocal unblocks only: agent-execution/05. The [execution amendment](../../proposals/non-ci-architecture/27-agent-execution-environments.md) adds a follow-on obligation over this brief's existing output. Its Task, Verify, gate, version and historical Evidence are unchanged; this note grants no delivery, reassignment or activation.

## Proposed measurement consumer — 2026-10-17

desk-improvement/08 consumes this brief for explicit per-item subject and semantic receipts after this standing client exists.
This is a forward consumer-routing amendment only: the new consumer waits on this owner,
not the reverse. Original Task/Verify and completed Evidence are unchanged. Reconcile any
active implementation claim before adding capture hooks. Native emission qualification is
delivered by the named follow-up; this source contract alone does not claim it exists.

## Interactive Kubernetes consumer — 20 October 2026

[The in-cluster operator/interactive target](../../proposals/non-ci-architecture/28-interactive-kubernetes-desks.md)
consumes this owner through agent-execution/09. Preserve current preparation, journal,
role-policy, credential and zero-idle-launch acceptance. The standing Kubernetes profile
requires no workflow instance/controller and creates no separate launch owner. A role Pod
or terminal may stay available while bounded model invocations start only for admitted
work/judgment. Pod replacement cannot treat an unconfirmed old execution group as stopped.
Carry current perf-v3 context across the transport and retain unknown usage. Existing
negative-path, independent-control and startup work/count checks apply to this consumer;
/09 and desk-containers/29 own its adapter and effective-profile qualification.
