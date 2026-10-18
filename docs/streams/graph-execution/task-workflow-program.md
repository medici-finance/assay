# Task workflow program — routed execution extension

**Routes-to:** docs/streams/graph-execution/

**Status:** routed to briefs 19–27, 2026-10-02. Existing briefs 04/05/06/09/14/16/18 carry compatible consumer amendments. All new implementation remains todo; no live activation or new merge authority is conferred.

## Scope and ownership

A thin deterministic program advances canonical workflow instances through scoped specialist model stages. Durable work, decisions and evidence survive process/session loss. Existing standing desks can use the shared launch supervisor independently; a desk can also delegate selected workflow nodes to the graph controller while continuing other work. The program replaces scheduling only for an explicitly transferred cohort; it is not an additional claimant for the same node.

This amendment permits the executable host excluded by the original stream's “new orchestration platform” non-goal, narrowly: reuse the existing instance, eligibility, admission, claim, recovery and evidence contracts. It permits two local embedded stores and no other: a single-controller, embedded SQLite store of the existing instance/node/attempt record family (/19), and each loop-admin supervisor's own execution journal (/26), a per-supervisor, process-facts-only record of invocations, receipts, lifecycle, limits and usage on the supervisor's own filesystem, whose embedded engine is qualified in /26 (SQLite is the expected choice). Neither is a shared or distributed store, and the journal never holds work, decisions or acceptance. It still excludes a graph database or graph-store service, a distributed or shared store or lease service, a message bus, a credential broker, a new rule engine, a second scheduler authority and a duplicate evaluator. Role executors remain separate; model choice never grants capabilities. The existing interface remains compatible for non-cohort clients.

**Storage routing superseded 2026-10-13:** the embedded-store permissions and shared-store exclusion in this historical paragraph are replaced by [Non-CI R2 routing — PostgreSQL coordination — 2026-10-13](README.md#non-ci-r2-routing--postgresql-coordination--2026-10-13). The cell's PostgreSQL database is the runtime coordination store; the other exclusions and human gates stand.

## Shared loop-admin: two clients, one execution component

The reusable **loop-admin** component launches and supervises agents/models for two independent callers:

| Mode | Work owner | What loop-admin runs | Required durable references |
|---|---|---|---|
| Standing desk | Existing coordinator, intake, worker, review or verify desk/queue | The role session and its permitted specialist children, with configured model profiles | Desk/caller binding, invocation, claim/owner generation; no graph instance required |
| Workflow stage | Deterministic workflow controller | A bounded author, implementer, reviewer or other approved stage invocation | Canonical work/instance/node/attempt plus the same launch envelope |

One implementation of start/observe/cancel/reconcile, provider adapters, process supervision, usage accounting and bounded recovery serves both. It may run as separate scoped instances per role; this is not a credential-holding all-role broker. `cellctl` installs approved profiles and performs operator lifecycle actions. "One implementation" includes the launch path main already ships; see "Existing launch path on main" below for what is moved, reused and wrapped. The runtime name loop-admin does **not** grant credential administration: child agents cannot install profiles, elevate roles or change limits.

A new independent `loopadmin/` Go module contains runner contracts (/20), the first adapter (/22), the supervisor and execution journal (/26), and desk client bindings (/27). It has no dependency on the instance store. `workflow/` contains the instance store (/19), the controller (/21), candidate/review/publication logic (/23–25); /21 consumes /26. A desk-only installation must start with `workflow/` absent. The first useful increment is therefore shared execution under existing desks, before workflow adoption.

The supervisor journal records invocation and process facts; desk queues and the instance store (/19) remain authoritative for work, decisions and acceptance. Caller intent is persisted before the launch request, and the returned receipt is correlated by stable caller/invocation/generation. The journal is the second permitted local store above, never a shared one. Separate stores cannot share an assumed atomic commit: a crash between them requires reconciliation, not another launch. Unknown cancellation remains unknown in either mode. Process success never means work accepted.

**Journal storage superseded 2026-10-13:** the local-only journal choice above is historical; see [Non-CI R2 routing — PostgreSQL coordination — 2026-10-13](README.md#non-ci-r2-routing--postgresql-coordination--2026-10-13) and brief 26's R2 amendment. The journal now uses its own schema in the cell's PostgreSQL database. Process-fact ownership and reconciliation across separate caller/journal commits still apply.

Standing role context can remain available when the adapter supports it; idle polling and terminal reconnect create no model request. Model choice and session retention are profile settings, not scheduling authority. A desk may request scoped child work, but recursion, concurrency, spend and role are bounded by the approved profile. A migrated binding has one launch owner; transfer from the legacy launcher (cellctl's existing launch path, below) requires drain/reconcile/fence.

Both modes expose the same sanitized lifecycle fields to the cockpit, including mode, caller, actual model, invocation/child IDs, usage completeness and held state. Graph references enrich workflow views; they are not mandatory to display or operate a desk. `deskd` and the viewer do not start/restart execution on attach.

## Existing launch path on main

Freshness-checked 2026-10-02 at main `307fe16992caef53fa46c52622753dd400c7b42a`. Main already ships a Go-owned launch path for host desks:

- `tools/desk/internal/cellprocess` runs one harness invocation, interactive or bounded, and reaps its owned process group or job before another turn; it reports an uncertain exit when cleanup cannot be confirmed.
- `tools/desk/internal/cellcadence` is the per-role clock: an OS file-lock lease, a checkpoint, and a refusal to replace a role owner that did not finish.
- `tools/desk/cmd/cellctl` (`launch.go`, `cadence.go`, `launch_tick.go`, `cursor.go`) builds the operator-approved argv/env for the Claude, Codex and Cursor harnesses and runs them interactively or on a cadence through those two packages ([cadence reference](../../cellctl-cadence.md)).

This program reuses that path; it does not add a second one beside it:

- **Process supervision is moved, not duplicated (/26).** `cellprocess` moves into the loopadmin module as its one process runner, and cellctl's cadence path calls it there through a `require` plus relative `replace` of the loopadmin module in `tools/desk/go.mod`, the pattern `tools/loopresolve/go.mod` already uses for `cellconfig`. The process-launch audit that scans `tools/desk` today must also scan the moved runner. Dropping it from the scan would weaken a control.
- **The per-role cadence lease is reused (/27).** `cellcadence` stays cellctl's host-desk clock. An enrolled role holds the same per-role lease while loop-admin supervises it, so the legacy and shared launchers cannot both own one role. An unfinished checkpoint maps to a held unknown stop, not a relaunch.
- **Harness argv is wrapped (/22).** The shared adapter takes cellctl's resolved, operator-approved argv/env profiles and adds no second argv builder. /22 qualifies bounded Codex first and Claude through the same implementation; each profile stays legacy until independently qualified and enrolled. Cursor remains legacy until separately qualified.
- **The legacy launcher (/27)** is cellctl's `deskLaunch` → `runInteractiveHarness`/`runCadencedHarness` path. A role binding moves off it only through the drain/reconcile/fence rule above. Non-enrolled roles keep it unchanged.
- `tools/desk/internal/loopengine` stays the in-session queue engine of a standing desk, not a process launcher. /27 binds it as a caller.

## Standing-desk efficiency delivery — 2026-10-18

**Outcome:** existing desk policy selects work; deterministic code owns admission,
claim/slot bookkeeping, process observation, CI/external waiting and result collection;
models receive bounded implementation, review or judgment assignments. No actionable
input means no new model request. Both supervisor and specialist consumption count:
moving a polling loop into a child is not a saving.

This is one delivery path, not another engine or stream. Apply the approved
[future architecture](../../proposals/non-ci-architecture/11-target-architecture.md),
[library-first ownership](../../library-first.md) and
[workflow-efficiency ownership map](../../desk-workflow-efficiency-change-map.md).
The first independent-verifier contracts remain the safety foundation; the review desk
is the first standing-desk adoption candidate because it exercises independent lanes
and substantial supervision. That order does not replace the first verifier product
milestone or authorize activation.

| Owner | Deliverable | Boundary / what is not repeated |
|---|---|---|
| Existing desk/queue policy and loopengine adapters | Eligible work, claims, next-item policy, role-specific judgment requests and result disposition | Use the existing drain contract. A planner CLI is not a running conductor. No second queue or evaluator in loop-admin. |
| /20 | Runner contract and conformance kit | Implemented delivery and historical Evidence stay intact; refresh its Linux witness through the existing local-container path, not another protocol. |
| /28–29, /04, /16 | Verification contracts/regression floor, unknown-effect recovery and ownership/budget fencing | Retain dependency and human gates. Idle efficiency does not justify weakening these controls. |
| /26 | One process runner, PostgreSQL process journal, deterministic observation/waits and launch brakes | Move/reuse cellprocess; no model heartbeat, alternate local store, all-role credential broker or second scheduler authority. Preserve the startup preparation/telemetry amendments. |
| /22 | Shared local-harness adapter; independently qualified Codex and Claude profiles | Codex is first in the bounded-invocation qualification; reuse cellctl's existing argv/env builders. Cursor remains legacy until separately qualified. No provider-specific copy of the supervisor. |
| desk-tools/39 and /42 | Importable review policy and role-specific issue projections | Existing extraction owners; /27 consumes them from tools/desk-local bindings, never imports cmd/main or duplicates classifiers. |
| /27 | Deterministic standing-desk clients, bounded jobs and one-owner transfer | Existing queues/claims remain work owners. Remove model-managed mechanical loops only when the replacement behavior is proven. No workflow store/controller required. |
| desk-supervision/34 | Usage compatibility for non-enrolled workers | /27 preserves the existing skip for enrolled workers; do not count one invocation in two stores. |
| desk-tools/37, /43–44 | Matched baseline, caller migration and complete-workflow qualification | Reuse its oracle/cohorts and qualification record. Minimal local counters are sufficient initially; parked performance collection is not an activation prerequisite or a second measurement pipeline. |

### Priority, blockers and dependency waves

Operational containment can use the **already shipped** explicit cellctl cadence mode
under the [existing clean-handoff procedure](../../cellctl-cadence.md#switching-an-existing-interactive-house).
This requires operator selection and ownership checks, not a new launcher or automatic
Goal manipulation. It is temporary containment: scheduled empty ticks still invoke a
model and therefore do not meet the target's zero-idle-request criterion. No live desk
is switched by this planning amendment.

The implementation path retains its declared safety dependencies:

| Wave | Existing work | Exit / next consumer |
|---|---|---|
| 0 | /20 independent witness refresh; /02 and /09 inherited contract verification; desk-supervision/34 and desk-tools/37 baseline in parallel | Preserve implemented work; resolve actual witness/definition holds, not status-table cosmetics. Local Docker availability is a route to a witness, not proof it exists. |
| 1 | /28 after /09; desk-tools/39 and /38 after /37 in parallel | Qualified candidate/acceptance identities and fact completeness; shared review policy and attribution. |
| 2 | /29 after /28; desk-tools/42 after /38 in parallel | Independent old/new regression floor; role-specific issue projections. |
| 3 | /04 after /02, /28–29 | Unknown outcomes reconcile before replacement. |
| 4 | /16 after /04, /09, /28–29 | Ownership, cumulative budgets, transfer and restoration fencing. |
| 5 | /26 after /20, /28–29, /16 | Shared execution with bounded startup and code-owned waits. |
| 6 | /22 after /20 and /26 | Codex-first and Claude profile qualification over that execution component. |
| 7 | /27 after /26, /22, desk-supervision/34 and desk-tools/39, /42 | Offline five-role proof; operator-approved review cohort first, then worker/verify/intake/coordinator only after each role's policy and retirement checks pass. |

Critical implementation path: **/09 → /28 → /29 → /04 → /16 → /26 → /22 → /27**;
/02 and /20 verification plus desk-supervision/34 join at their existing edges.
The parallel policy branch is **desk-tools/37 → /39** and **/37 → /38 → /42**,
both joining /27; these are missing reusable APIs, not optional tool optimizations.
The frontmatter edges and generated authoring table remain authoritative; these waves
express delivery order rather than introducing duplicate brief identities. The immediate
head is qualification of existing contracts plus the unimplemented /28, not /27.
At authoring, runner Client/conformance, cellprocess/cellcadence and the planner CLIs
exist; supervisor/process/adapters do not yet implement the proposed shared launch path.
Do not treat `reviewloop plan` or native Goal continuation as that implementation.

### Acceptance and retirement

The /27 production-path fixture must prove zero model starts for unchanged queues,
pending CI, repeated notifications, viewer attach and idle timer checks; a changed
admitted item starts exactly once, and a newly completed slot is refilled without a
model polling for it. Unknown stop, missing facts, exhausted budgets and stale generations
hold. An error needing judgment can produce one bounded assignment through the existing
role policy; an unchanged repeated error cannot create an endless model loop.

The same fixture must attribute supervisor versus child calls and uncached/cached input,
output and reported reasoning to invocation, role, lane, exact subject and profile.
Missing telemetry stays unknown. Compare matched completed-item cohorts using the
existing /37–44 qualification machinery; report review/verification coverage, rework
and completed outcomes as well as cost. There is no invented percentage saving target.
Zero idle launches is a deterministic criterion; useful-work efficiency requires the
independent matched comparison. No cost claim may omit children or count lost coverage
as a saving.

Remove duplicate calls before duplicate code. /27 owns removal of the adopted mechanical
skill loop at the same transfer boundary; independent review/verification judgments,
prior findings, source reads, acceptance tests and human authority remain. Legacy paths
stay for non-enrolled profiles until the last supported consumer is qualified and fenced.
Legacy skill-simplification work owns only remaining prose/kit cleanup and its own
historical close-out; it must not author another launch supervisor. Broad workflow/store,
publication and cockpit adoption follow their existing owners after this desk milestone.

## Canonical records and execution

The new workflow module stores the existing instance/node/attempt family, not another work ID. Records bind accepted intent, pattern/policy/acceptance versions, input/dependency fingerprints, assignment backend/generation, runner/model/tool profile, candidate/artifact hashes, findings/evidence, decisions, outstanding effects, usage and stop state. Large immutable artifacts are referenced. Export/restore validates all referenced artifacts; restored stores are passive until the existing authority fences the previous owner.

A single controller on durable local transactional storage is the first profile. SQLite is the initial store choice, with a pinned maintained driver and documented filesystem/durability assumptions qualified in 19. This is not a distributed lease service. Existing cross-machine ownership remains authoritative. An expected-version/generation transition is atomic; acknowledgment follows durability.

**Engine choice superseded 2026-10-13:** the SQLite choice above is historical; see [Non-CI R2 routing — PostgreSQL coordination — 2026-10-13](README.md#non-ci-r2-routing--postgresql-coordination--2026-10-13) and brief 19's R2 amendment for PostgreSQL coordination. The durability and authority-fencing obligations still apply.

Events wake reconciliation. A runnable node requires existing eligibility/admission, available reservations, current input/acceptance/policy and a valid role binding. Record actionable fingerprint and caller launch intent before asking loop-admin to invoke the runner. Duplicate notifications do not launch twice. Definite failed launches may retry; unknown launches/effects reconcile first. No relevant change means no model invocation. Human/external waits hold durable state without an active model session; productive context can continue inside a node.

The runner protocol is start/observe/cancel/reconcile with a stable request identity. Adapters declare supported resume, cancellation acknowledgment, request versus launch control and usage completeness. Missing data is unknown, not zero. No silent model/provider fallback. Actual model/profile versions are recorded. Operator-configured stage mappings may select different models; automatic routing optimization is outside this extension.

The controller/agent domain holds no role-executor credentials or general operator capabilities. Independent effect boundaries reject stale generations even if the scheduler is bypassed. A provider without enforceable dedupe or confirmed stop must yield a held unknown outcome, not an exactly-once assertion.

## Candidates, internal review and publication

One implementation writer owns a mutable workspace. A candidate pins repository, base, exact commit/tree, acceptance and artifact manifest; reviewers/checks consume immutable snapshots. Repair creates a new candidate, leaves prior evidence historical and invalidates affected verdicts. Parallel read checks are allowed; concurrent writers and integration of shards are deferred.

The opt-in prepublication pattern is author/plan review → intent decision where required → implement → freeze → independent checks/review → bounded repair → publication request. Existing implementation-v1 is retained. Stable findings, dispositions, failed rounds and all costs remain inspectable. Attempts, wall time and cumulative budget are bounded; exhaustion holds. Model diversity is not evidence of independence: actor, context, capability and immutable subject must be distinct where the role requires it.

Publication uses a narrow worker port and the exact reviewed candidate. The evidence bridge defaults to report-only and ordinary forge review. A separately activated adapter/profile can request an authorized reviewer executor to publish independent evidence at the current head, after checking provenance, completeness, acceptance/policy and atomic/fenced subject binding. The worker/controller never approves itself or marks ready. Human merge and independent merged-result verification remain unchanged. Stale or unavailable evidence requires fresh review/hold.

## Coexistence and adoption

1. Offline shared runner/supervisor and standing-desk bindings; qualify one existing desk at a time under the same responsibilities. This path requires no workflow controller.
2. Offline controller and connected fault corpus with fake runners/effects.
3. Read-only shadow over the same approved inputs: no model calls, claims or effects.
4. Existing dispatcher delegates a small worker cohort; normal draft-PR review path.
5. Separately qualified internal specialist-loop cohort and evidence bridge.
6. Expand only from measured accepted outcomes and last-consumer retirement evidence.

An adopter owns live profile, provider/data authorization, operator controls and migration gates. A queue exclusion cannot constrain an old direct credential. Before transfer, remove/restrict that path or migrate the credential/role boundary to the enforcing executor while retaining desks as clients. Stop admission, drain/reconcile, fence old generation, retain unknown reservations, then assign the replacement. Rollback preserves artifacts, decisions, spend and stop state; old binary pins alone are insufficient.

The host exports versioned read-only snapshots for existing clients. UI presence, terminal connection or a viewer refresh never starts/restarts an attempt. Operator lifecycle stays outside model and viewer domains. Typed human decisions use an authenticated existing surface, bound to exact immutable content and applicability; a terminal message cannot substitute for authorization.

## Routed units and tests

| Brief | Owns |
|---|---|
| 19 | Durable store, artifact integrity and actual pattern coverage binding |
| 20 | Shared runner contract/fake conformance and independent loopadmin module foundation |
| 21 | Workflow-stage client/controller over shared /26; graph progression and waits |
| 22 | One pinned local harness adapter; each additional adapter needs its own qualification |
| 23 | Candidate workspaces and frozen independent check inputs |
| 24 | Versioned internal specialist pattern, findings and bounded repair |
| 25 | Exact publication and report-only independent-review bridge |
| 26 | Shared loop-admin supervisor, execution journal and scoped runtime API |
| 27 | Standing desk/child bindings with no workflow-controller dependency |

Every brief is M/strong. 23 and 24 split the former candidate-workspace/internal-review unit. Existing 05 now consumes the production controller rather than inventing another experiment scheduler; 06 owns the one run-record family; 18 connects all components with its existing advice/control coverage. Completed/implemented 01/02/03/07/10/13 records are not reopened. New instance binding belongs to 19.

Required connected cases include duplicate events, definite/unknown launch, competing backends, stale writer after transfer, lost acknowledgment, journal failure, missing restore artifact, retained spend/pause, policy change outside edited files, head movement at acceptance, unauthorized actor/tool, cancellation with outstanding effect, mutable-review input, superseded decision, omitted finding, repair exhaustion and partial publication. Bypass the scheduler to test a separate authority boundary. Deliberate omission of a failed attempt must change the accounting result. Tests must exercise production contracts, not isolated serializers.

Measure original accepted-and-subsequently-verified work, full attempt/preparation/review/CI/human cost, arrival-to-acceptance median/p95, recovery, escaped defects and maintenance surface. Keep models fixed when comparing orchestration; model changes are a separate experiment. Later PR creation is not itself a speedup. A finite clean corpus is evidence of those cases, not universal safety. This stream's offline results do not authorize a live cohort.

## Contract ownership and documentation

Public work depends only on public contracts and fake-provider conformance. Adopter-only data, cohort decisions, installed secrets, operator acts and private measurements stay in adopting repositories. All consumers share canonical IDs and versions; unsupported mandatory fields refuse rather than silently downgrading. Each implementation updates its declared lifecycle/enforcement/runner documentation and cross-module CI path registry. No public brief depends on an adopter's private brief.

## Proposed native improvement evidence — 17 October 2026

[The desk-improvement contract](../desk-improvement/spec.md#8-runtime-evidence-responsibilities)
adds actual source emission to the future execution path. After the current owners land,
desk-improvement/05 binds canonical run/outcome records, /06 captures supervisor lifecycle
and usage, /07 workflow attempt/decision facts, and /08 standing per-item semantic outcomes.
Source operation receipts, role/subject bindings, failed/no-PR attempts and unknown usage
must survive source→assessment export. A successful invocation does not imply accepted work.

These are measurement consumers of the existing PostgreSQL, recovery and authority owners;
there is no reverse prerequisite from foundations to their new consumers. /10 qualifies
actual native emission and cross-mode replay. Offline retained-input cards can precede the
runtime; a claim of native measurement requires its producer/build/profile evidence. The
[future-state amendment](../../proposals/non-ci-architecture/22-desk-improvement.md) keeps
policy adoption and real enrollment under their existing separate gates.
