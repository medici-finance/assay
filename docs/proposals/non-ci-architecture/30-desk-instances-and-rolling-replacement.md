# Automatic desk instances and rolling replacement

**Requested architecture amendment, 25 October 2026.** This records the operator's
requested direction and amends unfinished work. It implements or activates nothing;
revision-bound architecture, custody, implementation and deployment gates remain.
Source pickup: public Assay main `57196d5da`, fetched 25 October 2026. Current cellctl
still keys house worktrees, leases and cadence checkpoints by role. Existing item
claims and model-policy resolution are reuse inputs, not proof of multi-instance safety.

## Outcome and scope

A cell can run several worker and review desk instances, independently configured
with qualified harness/model/provider/backend profiles. They compete for the same
eligible work and divide it through the existing claim authority. Coordinator, intake
and verification remain singleton roles in this first delivery. A desk instance is
an enduring operational subject; it is not a workflow instance, work attempt, model
invocation, permission role, terminal or Kubernetes Pod.

Pause an instance without disturbing its peers. Replacement stops admission on the
old instance and starts a new one promptly while the old one drains its existing
assignments. The desired population comes from operator-owned `cell.env`; runtime
state is coordinated by the existing R2/R3 owners, not by a second cellctl scheduler.
Standing desks continue to require no workflow instance tables or controller.

## Identity, names and configuration

R2 records immutable cell-qualified desk instance IDs, execution generations, invocation
IDs and replacement links. A role binding supplies authority; extra instances do not
create additional permissions, Apps or independent repository assignments. The existing
one-mutating-cell-per-repository restriction is unchanged.

Assign a human name automatically from role, harness, provider and one word in the
versioned [200-word dictionary](desk-instance-words.txt), each word 3–4 lowercase ASCII
letters. For example `example-worker-claude-anthropic-fern`. Include the cell in the
fully qualified display/Kubernetes name; cell-scoped commands may use an unambiguous
short name. A name is a locator, never proof of identity or permission.

Allocate names atomically within the cell and retain them for paused, draining and
unreconciled instances. Retry collisions; exhausted names return a clear hold without
reusing a live name or falling back to random letters. This finite dictionary is not a
population recommendation. Restarting the same instance keeps its word/ID and advances
its execution generation. A replacement gets a new word/ID and an explicit predecessor
link; its descriptive prefix reflects its selected profile. Never reconstruct an ID
from that prefix, and do not rotate identity on every `up` or attach.

`cell.env` is the current operator configuration entrypoint. The configuration owner
publishes one versioned desired-population representation with stable desired-entry
keys, role, qualified profile references, desired running/paused/stopped state and an
optional per-instance task ceiling. Initial generated names/IDs are assigned once and
persisted through the trusted configuration/runtime binding; entries never require the
operator to invent names. The implementation must specify the exact encoding and its
atomic update/revision checks in its public config contract, docs and fixtures. Do not
add executable shell evaluation or a competing source of desired population.

Desired-entry identity, desk identity and process identity are distinct: a replacement
updates the desired-entry binding to the successor while retaining its predecessor as
draining runtime history. A concurrent/retried request creates at most one successor.
`up` reuses those bindings, starts missing desired-running instances, preserves desired
paused/stopped entries and persistent instance stops, and leaves draining predecessors
alone. Removal/config edits never silently kill work or lift stops. Adding an instance
is an explicit desired-population operation, not a side effect of repeated `up`.

Legacy `ROLES` configuration maps to one implicit desired entry per enabled role.
The config adapter may support legacy and instance forms individually but rejects
ambiguous mixed declarations. An old reader must refuse a required instance capability;
it cannot collapse several entries to one role. Negotiate new contract fields using the
existing extensible-contract rules. Wire/result changes belong to desk-containers/31
and cell-api/01, not a rewrite of completed desk-containers/15 Evidence.

## Work assignment and capacity

Keep one claim authority for each eligible work item. Instances observe the shared
queue; claim acquisition, not independent queue scans, decides the winner. Worker
claims retain canonical work identity; review selection deduplicates `(repository,
PR, lane, head)`. Preserve existing review claim keys and explicit current-head checks;
if those keys exclude a newer head while an older review drains, report the hold rather
than inventing a new claim namespace. Independent correctness/security lanes remain
independent, while a second review instance is not an implicit second opinion.

Reuse the existing shared observation/watch owner and current role-policy evaluator.
Additional desk instances must not each recreate upstream polling, queue collection or
model-driven idle checks; explicit independent final review reads remain.

The role's capacity is an aggregate task limit across its instances in the cell and
across that cell's repositories. A lower optional instance ceiling narrows it; idle
instance capacity is borrowable up to the role limit. Desk processes, child task slots
and provider requests are separate counts. Adding a desk cannot multiply slots. Paused
and draining instances continue to consume capacity for active children/assignments;
unknown outcomes retain reservations until reconciled. Admission atomically checks and
reserves against current occupancy before launching; crash, retry, restart and restore
cannot reset consumption. No static per-instance partition or fairness scheduler is
required initially; expose occupancy and holds and reuse existing priority policy.

Shared provider/credential brakes still apply across roles and cells that use the same
budget scope. A new model/provider cannot bypass the role limit, and a new instance
cannot bypass a provider brake. Today's optional per-repository repair-admission gate
is not evidence of complete cell-wide capacity. A partially upgraded or legacy dispatcher
in the same work domain must participate in the same limit or stay excluded from managed
admission; an unknown legacy writer holds activation.

## Lifecycle and overlapping replacement

Use the existing running, pausing/paused, draining/drained and stopped transitions per
instance. Role/cell commands explicitly expand to their applicable instances; instance
commands never affect a peer. Existing machine, cell and role stops remain stronger
than instance-level resume. `up` never resumes an intentionally paused desk. Pause keeps
an instance available for later resumption; replacement retires its predecessor after drain.

For an eligible worker/review replacement:

1. Resolve and validate the successor's qualified profile, readiness and current operator
   authority. Record an idempotent replacement intent with predecessor and desired revision.
2. Stop new assignments on the predecessor and durably acknowledge that admission barrier.
3. Start the successor without waiting for the predecessor's tasks to finish. It claims
   only unowned eligible work and only within remaining shared task/provider capacity.
4. Let the predecessor finish its existing assignments and reconcile effects. Stop and
   retire that predecessor only after verified drain/stop; preserve its evidence and usage.

This permits distinct instance IDs to overlap. It does not permit two current owners
of one instance/generation, transfer a live claim, launch a second copy of an unknown
attempt or treat an uncertain old effect as cancelled. A healthy successor may do
unrelated unclaimed work while an uncertain predecessor holds its affected items and
reservations; no conflicting work or authority handback is admitted. Instance restart
and Pod recovery still require the same-instance old execution to be stopped/fenced.

Successor startup failure leaves the predecessor paused/draining and reports partial
progress; never resume it or create another successor automatically. Drain timeout
leaves the predecessor visible and held, with the successor's status independent.
Operator recovery uses current intent/claim/effect observations. Forced abandonment,
foreign-stop lift and plan apply retain the existing confirmation/authority contract.
Sequential restart remains the fallback for singleton roles and profiles that cannot
safely overlap. Shared-binary upgrades still refuse unsafe swaps under running desks.
No separate takeover controller or mandatory new takeover verb is introduced.

Profile changes do not promise conversation or child-process migration. Preserve files,
conversation, pending interactions and process continuity only as independently declared
and qualified by amendment 27. A fresh session reconstructs durable work state; a restored
workspace grants no lease, budget, effect retry or human approval.

## Kubernetes, projections and communications

The managed Kubernetes profile provides a separate workload and retained writable
workspace for each configured desk instance. Role-specific images/permissions can be
reused. Singleton compatibility manifests and their historical Evidence remain intact;
the managed desired-population renderer is delivered by desk-containers/28 consuming /31.

Display cell/role/harness/provider/word names in Kubernetes within validated platform
name limits, using a deterministic short locator if the full prefix does not fit; retain
the full display name in metadata. Bind cell ID, role, instance ID and generation in
labels/receipts, and verify namespace, Pod UID and container independently. Neither a
same-named Pod nor `replicas > 1` against one shared role workspace establishes isolation.
Installation pre-provisions declared workloads; ordinary desk launch cannot gain arbitrary
Pod, service-account, mount or credential creation rights. Replacement preparation uses
that same operator-owned provisioning path before successor start.

Status, plans, logs and attach expose individual instances, predecessor/successor state,
desired versus observed state, shared role usage and instance ceilings. Aggregates never
hide paused, draining, unknown or failed members. Short selectors must refuse ambiguity.
Sanitized projections preserve profile/actor and per-invocation usage; terminal attachment
remains optional presentation and causes zero model calls or ownership changes.

Communications retain role-based authorization. A work request addressed to a role is
claimed by one eligible instance; a targeted request binds cell/instance and any required
generation. Observation fanout is explicit and cannot duplicate work delivery. Retry,
acknowledgment and stale-target handling retain the existing broker's owner; no per-instance
credential broker, competing inbox or new external ingress is introduced. Until that
transport is qualified, the existing supported transport remains explicit.

## Delivery and acceptance

| Owner | Applied brief amendment |
|---|---|
| R2 ownership/capacity, coordination records | graph-execution/16, /19; shared supervisor journal /26 |
| Standing clients and claim/usage policy | graph-execution/27 |
| Config, naming, desired-population adapter and cell-op extension | new desk-containers/31; consumes /27 and completed /15 |
| Stops, lifecycle and replacement | desk-containers/16, /18, /19 |
| Status, diagnostics, logs, upgrade, GC and repair consumers | desk-containers/07, /20–25 |
| Qualified profile, readiness, recovery and projection | agent-execution/01–05, /09 |
| Managed Kubernetes/operator and independent journey | desk-containers/28–29; compatibility /06 preserved |
| Served contracts, authorization, thin clients and cutover | cell-api/01–04, /06–07 |
| Role/group transport routing | amendment 24 retains its existing owners; no transport implementation in this amendment |

Qualification must use the production admission/runner/adapter paths with disposable
PostgreSQL, fixture harnesses and fake providers/Kubernetes APIs. Prove two mixed-profile
workers and three reviewers divide items/lanes, preserve a paused peer, and respect one
aggregate role limit across two repositories. Race admission for the last slot and repeat
it across crashes. Replace one instance while a child is busy: successor starts before
old child completion, old instance takes zero new work, active reservations remain counted,
and no duplicate item/effect occurs. Cover failure to start, drain timeout, lost acknowledgments,
concurrent `up`/replace, name collision/exhaustion, ambiguous selector, Pod UID replacement,
backup rollback, legacy writer exclusion, old-reader refusal and cross-cell isolation.
Run named negative/mutation controls with caller/UI bypassed where appropriate. Fake
transport success is not effective Kubernetes custody proof; desk-containers/29 retains
its separate independently witnessed activation gate. Scale from one to the declared mixed population on an unchanged queue: zero extra model
launches and no duplicated routine upstream collector; retain existing due refresh and
independent final-read obligations. No cluster/model calls in authoring.

## Pickup, history and approval

This amendment supersedes singleton role-process assumptions only for worker/review
instances and sequential replacement assumptions only for their qualified overlap mode.
Unchanged scope, existing risk gates, independent lanes, lower-boundary authority and
completed Evidence remain controlling. Dated source accounts remain history. Amended
unfinished Task/Verify versions require fresh pickup of the selected contract, active
claims and code; they do not invalidate already delivered singleton behavior.

Open-PR overlap was checked on 25 October 2026: public #4349 (execution amendment admission),
#4223 (store acceptance/consumer holds) and #4420 (cell-op consumer verification) touch
related documents. Preserve their distinct approval, acceptance and delivery-evidence
changes when reconciling; this request does not silently adopt or cancel their work.
The open-PR listing is bounded, and dispatch refs were not inventoried: implementation
pickup must refresh both. This is a manual source-pinned context assessment: **adapt**
future singleton assumptions within existing R2/R3/R4/R7 owners; **hold activation** until
instance capacity, ownership and effective profile qualification pass. No process-wide
complexity measurement was taken; known coupling is the role-keyed launch/cadence/config
seam and optional per-repository admission gate. Remove that singleton coupling through
the existing owner rather than add a second lease/store. This draft requires the existing
revision-bound review/ratification process; the operator's request authorizes authoring.
