# Task workflow program — routed execution extension

**Routes-to:** docs/streams/graph-execution/

**Status:** routed to briefs 19–27, 2026-10-02. Existing briefs 04/05/06/09/14/16/18 carry compatible consumer amendments. All new implementation remains todo; no live activation or new merge authority is conferred.

## Scope and ownership

A thin deterministic program advances canonical workflow instances through scoped specialist model stages. Durable work, decisions and evidence survive process/session loss. Existing standing desks can use the shared launch supervisor independently; a desk can also delegate selected workflow nodes to the graph controller while continuing other work. The program replaces scheduling only for an explicitly transferred cohort; it is not an additional claimant for the same node.

This amendment permits the executable host excluded by the original stream's “new orchestration platform” non-goal, narrowly: reuse the existing instance, eligibility, admission, claim, recovery and evidence contracts. It does not commission a graph database, bus, credential broker, new rule engine or second scheduler authority. Role executors remain separate; model choice never grants capabilities. The existing interface remains compatible for non-cohort clients.

## Shared loop-admin: two clients, one execution component

The reusable **loop-admin** component launches and supervises agents/models for two independent callers:

| Mode | Work owner | What loop-admin runs | Required durable references |
|---|---|---|---|
| Standing desk | Existing coordinator, intake, worker, review or verify desk/queue | The role session and its permitted specialist children, with configured model profiles | Desk/caller binding, invocation, claim/owner generation; no graph instance required |
| Workflow stage | Deterministic workflow controller | A bounded author, implementer, reviewer or other approved stage invocation | Canonical work/instance/node/attempt plus the same launch envelope |

One implementation of start/observe/cancel/reconcile, provider adapters, process supervision, usage accounting and bounded recovery serves both. It may run as separate scoped instances per role; this is not a credential-holding all-role broker. `cellctl` installs approved profiles and performs operator lifecycle actions. The runtime name loop-admin does **not** grant credential administration: child agents cannot install profiles, elevate roles or change limits.

A new independent `loopadmin/` Go module contains runner contracts (/20), the first adapter (/22), the supervisor and execution journal (/26), and desk client bindings (/27). It has no graph-store dependency. `workflow/` contains graph storage (/19), the controller (/21), candidate/review/publication logic (/23–25); /21 consumes /26. A desk-only installation must start with `workflow/` absent. The first useful increment is therefore shared execution under existing desks, before workflow adoption.

The supervisor journal records invocation and process facts; desk queues and graph stores remain authoritative for work, decisions and acceptance. Caller intent is persisted before the launch request, and the returned receipt is correlated by stable caller/invocation/generation. Separate stores cannot share an assumed atomic commit: a crash between them requires reconciliation, not another launch. Unknown cancellation remains unknown in either mode. Process success never means work accepted.

Standing role context can remain available when the adapter supports it; idle polling and terminal reconnect create no model request. Model choice and session retention are profile settings, not scheduling authority. A desk may request scoped child work, but recursion, concurrency, spend and role are bounded by the approved profile. A migrated binding has one launch owner; transfer from the legacy launcher requires drain/reconcile/fence.

Both modes expose the same sanitized lifecycle fields to the cockpit, including mode, caller, actual model, invocation/child IDs, usage completeness and held state. Graph references enrich workflow views; they are not mandatory to display or operate a desk. `deskd` and the viewer do not start/restart execution on attach.

## Canonical records and execution

The new workflow module stores the existing instance/node/attempt family, not another work ID. Records bind accepted intent, pattern/policy/acceptance versions, input/dependency fingerprints, assignment backend/generation, runner/model/tool profile, candidate/artifact hashes, findings/evidence, decisions, outstanding effects, usage and stop state. Large immutable artifacts are referenced. Export/restore validates all referenced artifacts; restored stores are passive until the existing authority fences the previous owner.

A single controller on durable local transactional storage is the first profile. SQLite is the initial store choice, with a pinned maintained driver and documented filesystem/durability assumptions qualified in 19. This is not a distributed lease service. Existing cross-machine ownership remains authoritative. An expected-version/generation transition is atomic; acknowledgment follows durability.

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

Every brief is M/strong. 23 and 24 split the former candidate-workspace/internal-review unit. Existing 05 now consumes the production controller rather than inventing another experiment scheduler; 06 owns the one run-record family; 18 connects all components with its existing advice/control coverage. Completed/implemented 01/02/03/07/10 records are not reopened. New instance binding belongs to 19.

Required connected cases include duplicate events, definite/unknown launch, competing backends, stale writer after transfer, lost acknowledgment, journal failure, missing restore artifact, retained spend/pause, policy change outside edited files, head movement at acceptance, unauthorized actor/tool, cancellation with outstanding effect, mutable-review input, superseded decision, omitted finding, repair exhaustion and partial publication. Bypass the scheduler to test a separate authority boundary. Deliberate omission of a failed attempt must change the accounting result. Tests must exercise production contracts, not isolated serializers.

Measure original accepted-and-subsequently-verified work, full attempt/preparation/review/CI/human cost, arrival-to-acceptance median/p95, recovery, escaped defects and maintenance surface. Keep models fixed when comparing orchestration; model changes are a separate experiment. Later PR creation is not itself a speedup. A finite clean corpus is evidence of those cases, not universal safety. This stream's offline results do not authorize a live cohort.

## Contract ownership and documentation

Public work depends only on public contracts and fake-provider conformance. Adopter-only data, cohort decisions, installed secrets, operator acts and private measurements stay in adopting repositories. All consumers share canonical IDs and versions; unsupported mandatory fields refuse rather than silently downgrading. Each implementation updates its declared lifecycle/enforcement/runner documentation and cross-module CI path registry. No public brief depends on an adopter's private brief.
