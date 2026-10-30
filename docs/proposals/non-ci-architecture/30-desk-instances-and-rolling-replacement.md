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
optional per-instance refill target. Initial generated names/IDs are assigned once and
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
existing extensible-contract rules. Wire/result changes belong to desk-containers/33
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

**Capacity clarification — 26 October 2026.** The shared work count is an
approximate role-wide concurrency gauge and refill target across the cell's instances
and repositories. It is pacing, not a hard admission budget or ownership boundary.
Temporary undercount, overcount and concurrency above/below the target are accepted;
desks converge toward the configured level as observations refresh and tasks finish.
An optional per-instance target shapes local refill; it does not partition global slots.
Adding an instance does not intentionally multiply the configured role target.

Each desk independently estimates active role work from available shared observations
(for example existing claims, roster/heartbeat summaries and supervisor process facts),
uses the estimate to decide a bounded next refill, and refreshes/reconciles periodically.
A cached/shared counter may be an optimization; no centralized capacity mutex, global
semaphore, serialized count-and-reserve transaction, leased slot, or per-launch
coordination round-trip is required. A cache update failure cannot block unrelated
otherwise-authorized work. No exact last-slot winner, zero-overshoot promise or invented
numeric drift bound is part of this contract.

When the estimate is above target, stop refilling and let completions bring it down;
do not kill work to enforce the target. When an overestimate ages out or fresher facts
show a gap, refill again. Concurrent desks may read the same gap and both launch distinct
claimed items. Estimate freshness/coverage and uncertainty stay visible; missing data
is not presented as a measured zero. Retained recent observations and best-effort refresh
can guide continued bounded refill through unavailable/stale telemetry. The pacing
counter alone does not globally hold the role, require a leader/mutex for reconstruction,
or create a fail-closed slot ledger. Include busy children on paused/draining predecessors
in the estimate when observed; uncertainty affects the estimate rather than inventing
an exact capacity reservation. Stale observations must be correctable so the role does
not remain permanently starved or deliberately refill the whole target per instance.

For example, with target eight and observed active work seven, two independent desks
may each start one different item, giving actual work nine. Both can subsequently observe
the excess and stop refilling until completions bring it toward eight. Conversely, an
estimate of ten while only six remain must recover after observation refresh/expiry.
Tests qualify correction in both directions, not a globally serialized last slot.

This is an instantaneous occupancy gauge, not a cumulative operation count. Existing
window/reset accounting for cumulative pacing counters is not turned into a new durable
role-slot ledger. Reuse the existing observation/runtime owners and pacing classification;
there is no requirement for an exact global re-derive before every desk dispatch.

**Exactness remains where it protects work or authority.** Per-item claims still prevent
two instances from owning the same task; review claim keys/current-head checks remain.
Instance generations, role/scope authorization, uncertain external effects, durable effect
identity and genuine fail-closed brakes (recursive child-launch/time/cost/resource budgets,
write ceilings, circuit breakers and provider-enforced quotas) retain their contracts.
Their reservations protect those operations/budgets, not this aggregate work gauge.
A pacing estimate grants no authority. Classify provider request pacing separately from
provider-enforced quotas or explicit safety brakes. Legacy writers must participate in
canonical work ownership/authority before managed activation; their work should be
observed best-effort, without requiring a capacity-slot protocol or identical counters.

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
   only unowned eligible work, using approximate role refill pacing and the existing
   independent provider/safety controls.
4. Let the predecessor finish its existing assignments and reconcile effects. Stop and
   retire that predecessor only after verified drain/stop; preserve its evidence and usage.

This permits distinct instance IDs to overlap. It does not permit two current owners
of one instance/generation, transfer a live claim, launch a second copy of an unknown
attempt or treat an uncertain old effect as cancelled. A healthy successor may do
unrelated unclaimed work while an uncertain predecessor holds its affected items and
operation/authority reservations; its observed active work remains in the pacing estimate; no conflicting work or authority handback is admitted. Instance restart
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
the managed desired-population renderer is delivered by desk-containers/28 consuming /33.

Display cell/role/harness/provider/word names in Kubernetes within validated platform
name limits, using a deterministic short locator if the full prefix does not fit; retain
the full display name in metadata. Bind cell ID, role, instance ID and generation in
labels/receipts, and verify namespace, Pod UID and container independently. Neither a
same-named Pod nor `replicas > 1` against one shared role workspace establishes isolation.
Installation pre-provisions declared workloads; ordinary desk launch cannot gain arbitrary
Pod, service-account, mount or credential creation rights. Replacement preparation uses
that same operator-owned provisioning path before successor start.

Status, plans, logs and attach expose individual instances, predecessor/successor state,
desired versus observed state, estimated role work/target, observation freshness and optional instance targets. Aggregates never
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
| R2 exact ownership and runtime facts; approximate role pacing | graph-execution/16, /19; shared supervisor journal /26 |
| Standing clients and claim/usage policy | graph-execution/27 |
| Config, naming, desired-population adapter and cell-op extension | new desk-containers/33; consumes /27 and completed /15 |
| Stops, lifecycle and replacement | desk-containers/16, /18, /19 |
| Status, diagnostics, logs, upgrade, GC and repair consumers | desk-containers/07, /20–25 |
| Qualified profile, readiness, recovery and projection | delivered agent-execution/01 consumed; extension in /03, with /02, /04–05, /09 |
| Managed Kubernetes/operator and independent journey | desk-containers/28–29; compatibility /06 preserved |
| Served contracts, authorization, thin clients and cutover | cell-api/01–04, /06–07 |
| Role/group transport routing | amendment 24 retains its existing owners; no transport implementation in this amendment |

Qualification must use production selection/runner/adapter paths with disposable
PostgreSQL where required by exact work/authority records, fixture harnesses and fake
providers/Kubernetes APIs. Prove two mixed-profile workers and three reviewers divide
items/lanes, preserve a paused peer and use one configured role target across two
repositories. Feed lagging observations to competing desks: permit both to fill an
observed last gap with different items, then show refill stops above target and resumes
after completions/refresh. Feed an overestimate and show its correction prevents permanent
starvation. Counter-write failure, delayed/missing heartbeat, crash and restart do not
require a global capacity lock or block unrelated authorized work solely on pacing data.
Required mutation controls: replace pacing with a central slot mutex/leased semaphore,
count only each instance's own work, never refresh a stale overestimate, or make counter
maintenance failure fatal; each must fail the named production-path qualification.

Replace one instance while a child is busy: successor starts before old child completion,
old instance takes zero new work, observed predecessor work informs the estimate, and no
duplicate item/effect occurs. Cover failure to start, drain timeout, lost acknowledgments,
concurrent up/replace, name collision/exhaustion, ambiguous selector, Pod UID replacement,
backup rollback, legacy ownership exclusion, old-reader refusal and cross-cell isolation.
Exact ownership/authority and lower-boundary safety tests remain independent of pacing.
Fake transport success is not effective Kubernetes custody proof; desk-containers/29
retains its separate independently witnessed activation gate. Scale from one to the
declared mixed population on an unchanged queue: zero extra model launches and no
duplicated routine upstream collector, preserving due refresh/independent final reads.
No cluster/model calls in authoring.

## Pickup, history and approval

This amendment supersedes singleton role-process assumptions only for worker/review
instances and sequential replacement assumptions only for their qualified overlap mode.
Unchanged scope, existing risk gates, independent lanes, lower-boundary authority and
completed Evidence remain controlling. Dated source accounts remain history. Amended
unfinished Task/Verify versions require fresh pickup of the selected contract, active
claims and code; they do not invalidate already delivered singleton behavior.

Historical open-PR overlap was checked on 25 October 2026: public #4349 (execution amendment admission),
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

## Clarification pickup — 26 October 2026

Merged public main `125611dfb` into this draft, preserving subsequent profile implementation,
execution admission and store consumer-integration changes. The operator clarified that the
shared work count is approximate and must not require centralized slot coordination.
This explicitly supersedes this draft's earlier atomic aggregate-capacity requirement,
not per-item claims or independent safety brakes. Related PRs #4223 and #4349 have since
landed; their acceptance/admission history remains intact. The implemented profile contract
is consumed with follow-on instance integration in agent-execution/03. Future qualification
must show pacing convergence/no capacity lock as well as exact ownership safety.

## Collision correction pickup — 30 October 2026

This successor carries the public proposal from #4429 at `d0f46083` onto current
main `d5bfe27e`. Main has since assigned desk-containers/31 and /32 to the separate
compatibility Kubernetes operation and qualification briefs. Those identities,
contracts and historical Evidence remain intact; the automatic-instance adapter
is now desk-containers/33 throughout this proposal and its unfinished consumers.
The correction is tracked by #5030. The original proposal remains open for independent
supersession review. Its approximate pacing clarification and exact ownership/safety
requirements are retained, and no implementation or activation is claimed.
