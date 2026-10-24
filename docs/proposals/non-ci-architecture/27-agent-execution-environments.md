# Portable agent execution environments

**Status:** approved — bounded implementation scope and agent-execution admission approved
by the maintainer on 24 October 2026; effective on the maintainer's merge of the
[approval and reconciliation change](https://github.com/medici-finance/assay/issues/4347),
the ruling-bearing PR under lifecycle §8.4.

**Routes-to:** docs/streams/agent-execution/

This extends the existing R2/R3/R4/R7 owners through the already-authored
[agent-execution stream](../../streams/agent-execution/README.md). The maintainer explicitly
approved amendment 27 for implementation and requested that stream be unparked.
This is a new bounded implementation approval, not an expansion of the earlier
revision-bound architecture decision. Deployment, credential migration, remote backend
selection, concrete profile adoption and live activation retain their separate approvals.
Existing decisions, Task/Verify obligations and historical Evidence stay intact.

## Outcome and controlling design

An admitted invocation can use a qualified harness and execution environment without
changing work identity, scheduling, effect authority or acceptance. Operators can tell
whether preparation, execution, recovery or acceptance needs attention. The same contract
serves standing desks without requiring a workflow instance.

Use the [whole-system target](11-target-architecture.md),
[runtime alignment](23-runtime-contract-alignment.md),
[storage/recovery contract](14-storage-forge-graph.md) and
[operator lifecycle](21-cell-operator-surface.md). The
[celld session-host proposal](../../celld-session-host-spec.md) explores packaging and
vendor-specific session controls; its S1–S6 experiments retain their scope and authorization.
This amendment adds no operator lifecycle transition, Cell API, scheduler or database.

## Evidence and actual gap

Source inspection: Assay main `ec8d77e56627a7c2269f3d143ef6a31f3e41bd02`,
16 October 2026. `loopadmin/runner/request.go` already has strict decoding,
`ext`/`require` negotiation, pinned profiles, workspace references and session references.
`runner/usage.go` declares resume/snapshot booleans and unknown-preserving usage.
`runner/conformance` exercises the real client against hostile fake adapters; its existing
suite passed during authoring. This proves that boundary, not a real harness or deployment.
At this revision `loopadmin/supervisor/`, `loopadmin/adapters/localharness/` and `workflow/`
are absent; graph-execution/22, /25–27 remain planned implementation owners.

The missing increment is a qualified harness/backend/model tuple, explicit preservation
semantics, a prepared-environment receipt, and production-consumer qualification of those
records. Existing graph-execution/20 is consumed unchanged, with no rewrite of its Evidence.
Existing maintenance and runtime-resource contracts are reused only within their real scope:
`runtime-resource-v1` does not currently classify Git attempt workspaces.

AX supplies useful design evidence at
[`8d2ccf36f194f6f58c57b90753bd3986639b562f`](https://github.com/google/ax/tree/8d2ccf36f194f6f58c57b90753bd3986639b562f):
its [concepts](https://github.com/google/ax/blob/8d2ccf36f194f6f58c57b90753bd3986639b562f/docs/concepts.md)
separate tasks and reusable workspace descriptions;
the [runner contract](https://github.com/google/ax/blob/8d2ccf36f194f6f58c57b90753bd3986639b562f/docs/runner.md)
distinguishes preparation from execution and describes workspace restoration into a fresh
process tree. Its [security policy](https://github.com/google/ax/blob/8d2ccf36f194f6f58c57b90753bd3986639b562f/SECURITY.md)
documents missing control-API authentication/authorization and task-visible model keys.
Its [design](https://github.com/google/ax/blob/8d2ccf36f194f6f58c57b90753bd3986639b562f/DESIGN.md)
uses Redis and Agent Substrate. These observations select no Assay backend or storage change.
They provide no measured Assay performance or savings claim.

## Ownership and configuration

| Concern | Owner | Boundary |
|---|---|---|
| Runnable work, human waits and acceptance | Existing desk queues/workflow controller and domain evaluators | Backend task status cannot select work or close obligations |
| Invocations and process supervision | loop-admin, graph-execution/26 | One launch owner, stable invocation identity, same cell PostgreSQL journal |
| Preparation and retained environment | loop-admin workspace port; existing Git/resource/retirement adapters | No second Git implementation, maintenance loop or deletion authority |
| Candidate and independent review input | graph-execution/23 and /28 | Preparation receipt is not a candidate or review verdict |
| External effects and unknown outcomes | R4 role executors and existing intent/receipt ledger | No credential-bearing generic execution proxy |
| Operator configuration and human presentation | Existing cell-operation and R7 Cell API/client owners | Observation/attachment cannot start work or change admission |

Resolve three independent configuration axes: **harness** (implementation and protocol
version), **model** (requested profile and actual reported identity), and **execution
backend** (qualified host/container/VM profile, later possibly AX). Independence does not
promise every combination works. Bind the supported tuple, profile digest, qualification
evidence, tool/network policy revision and capability requirements before launch.
No task-supplied executable, endpoint, mount or credential can widen an approved profile.
Any added process needs a custody, isolation or lifecycle reason; these rows are logical
boundaries within the modular Go product.

## Capability and receipt contracts

Extend v1 through a documented, schema-backed extension negotiated by `ext` plus
`require: [ext:<name>]`. Its final names and source/schema coverage tests belong to
agent-execution/01. Existing v1 requests remain compatible. Legacy resume/snapshot booleans
keep their existing meanings; a consumer requiring stronger preservation must require the
new extension and refuse an old adapter. Unknown mandatory semantics never downgrade.

The qualified capability record states separately whether files, conversation history,
process memory, descendant processes and pending interactions survive a particular operation.
It declares cancellation confirmation, budget scope, telemetry coverage and relevant
version/platform limits. Unsupported and could-not-check stay distinct. Conversation
rebind, filesystem restore and process continuation are different observations; none renews
authority or lifts a pause. Retain doc 21's operator vocabulary.

A **prepared-environment receipt** binds caller/invocation, externally validated owner
incarnation/generation, workspace allocation, exact repository revisions, environment recipe
and tool/skill versions, image digest where applicable, approved writable paths and effective
tool/network policy references. It includes readiness outcome, validation provenance and
artifact references. Secret values and credential-bearing URLs are excluded. Canonical
candidate/acceptance identities remain references owned by R1; loopadmin imports no workflow
store or evaluator. The port accepts opaque qualified references and callers validate them.

Preparation returns ready only after required checks and independent profile validation.
The receipt describes effective observed configuration; it is not proof that an untrusted
agent enforced isolation. A cached receipt must be rebound/revalidated for its new invocation,
current authority and policy before use. Unknown allocation or partial preparation holds;
reconciliation inspects the existing resource before retrying. Restoring files must never
re-clone over unpublished work. TTL or snapshot availability alone cannot authorize removal.

## Recovery and enforcement

Environment preservation and work continuation are separate checks. Before restored execution
can act, existing owners recheck incarnation/generation, assignment, stop/decision state,
budgets, receipt applicability and unresolved effects. A provider write with a lost reply
is reconciled using its stable logical effect identity; a new process or delivery attempt
does not create a new operation or fresh budget. Unknown stop cannot authorize a replacement.
Schema rollback/restore must preserve these obligations and return held if it cannot.

Candidate code and its runner cannot read operator or publisher credentials. Network
enforcement/credential injection may be supplied by a qualified deployment profile; it
does not replace the role executor's operation/repository/revision/current-owner checks.
Test those independently with the upper caller bypassed. AX's current documented trust
boundary is not sufficient evidence to qualify this deployment. No new vault, universal
gateway or production network contact is commissioned here.

## Lifecycle projection and quality scenarios

Use one sanitized snapshot from the existing supervisor/cell-operation API, with independent
dimensions for preparation, process execution, environment preservation/recovery, external
effect reconciliation, and work acceptance. Translate onto existing state vocabularies;
do not create a second durable status writer. Every dimension names its owner, subject,
timestamp/coverage and explicit unknown or unsupported result. R7 consumes that snapshot
through its existing ingress; this stream supplies no alternative endpoint or cockpit.

| Scenario | Required observable result | Evidence owner |
|---|---|---|
| Required conversation continuity, backend restores files only | Refuse before launch; no fresh-session fallback | /01 contract; /03 production gate |
| Wrong revision or changed policy with a plausible ready receipt | Hold before launch; no reused readiness or changed candidate | /02 receipt; /03 supervisor flow |
| Container alive after command failure | Show process failure separately from environment liveness and acceptance | /05 lifecycle projection |
| Publish succeeded, reply lost, process restored | One logical effect; reconcile or hold; spend and pause persist | /04 integrated recovery |
| Stale restored owner bypasses upper caller | Existing executor rejects the effect independently | /04 integrated negative control |
| Partial preparation, retained unpublished files, expired allocation | Preserve files and hold uncertain release; no second pruner | /02 and /03 workspace flows |
| AX cannot supply a required boundary | Compatibility report records no-go/unknown; local route remains usable | /06 offline investigation |

Optimize only after measuring cold/warm preparation, readiness/recovery duration, active
versus waiting time, bytes/storage and full-attempt cost with coverage. Optional performance
collection reuses its existing owner; required budgets and audit do not depend on it.
No numerical speed, density or savings target is approved by this amendment. A live backend
trial needs an owner-approved workload, comparator and thresholds in its separate trial plan.

## Harness improvement as reviewed profile proposals

Harness² is complementary research, not an AX runtime or a replacement controller.
Source inspected 16 October 2026 at
[`1cfe254e8d25b15f37048c3730134686abe60335`](https://github.com/google-research/harness2/tree/1cfe254e8d25b15f37048c3730134686abe60335).
Its [README](https://github.com/google-research/harness2/blob/1cfe254e8d25b15f37048c3730134686abe60335/README.md)
describes task-level propose–probe–compose and retained lessons across tasks using fixed
model weights and existing executors. It also permits editing guardrails among its harness
components. This is research software, not a declaration that Google's production harness
or an AX integration has been released.

The [experiment guide](https://github.com/google-research/harness2/blob/1cfe254e8d25b15f37048c3730134686abe60335/docs/EXPERIMENTS.md)
documents unrestricted executor networking and observed access to a public benchmark rubric.
That makes independent holdouts and enforced evaluation-data isolation material to any
Assay trial. Reported benchmark gains are upstream observations, not expected Assay gains.

Treat model-proposed prompt/skill/tool-guidance changes as untrusted candidate profile
bundles, pinned by digest and provenance alongside the harness binary/model/backend tuple.
Probe them in isolated scratch executions with a cumulative experiment budget. Keep proposal,
selection evidence and independent assessment separate. A learned memory entry cannot become
policy or acceptance evidence by being retained. No proposal may edit operator policy,
authorization, acceptance criteria, enforced guardrails, credentials or a live attempt's
pinned bundle. The existing reviewed change/profile-adoption path remains the promotion owner.

The new /07 investigation uses deterministic offline fixtures to qualify the proposal and
evaluation protocol first. It must include an unchanged baseline, an equal-budget retry
comparator, independent held-out tasks, protected control edits, hidden-rubric access and a
regressing composition. Count all reference runs, proposals, probes, composition, reflection,
failed attempts and human review. Fixture success establishes the protocol only; measuring
model-backed benefit requires a separate authorized trial. The existing improvement-proposal
owners retain scheduling/promotion; no self-modifying production loop is selected here.

## Delivery, compatibility and decisions

[Seven M-sized briefs](../../streams/agent-execution/README.md) deliver the extension,
workspace receipts, first local consumer, integrated recovery, lifecycle projection and
independent AX feasibility and Harness² evaluation reports. Core delivery uses local fixtures and existing owners.
AX feasibility neither gates that path nor qualifies remote execution. A production AX adapter,
remote registration, cluster operations, credential migration and live promotion require
separately authored work after measured compatibility and existing activation decisions.

No existing unfinished Task or Verify obligation is weakened or silently reassigned.
Only reciprocal dependency declarations and a dated routing note are added to their owners.
Refresh claims and current PRs at pickup. Finish active work under its pinned contract or
coordinate an explicit amendment; never reinterpret historical Evidence as coverage of this delta.
Release/pin consumers together; a mixed-version consumer requiring the extension holds.
Rollback drains/reconciles before profile handback and never turns unknown effects into retries.

Decisions retained: operator approval of a concrete launcher/profile boundary, inherited
R3/R4 qualification and live activation. Whether AX merits a production adapter is unresolved;
the default is no remote activation. The useful first move is /01 over implemented /20,
whose conformance suite passed offline during authoring. Full integration also depends on
existing graph-execution/02 independent verification and /28–29, R2/R3, publication /25;
an AX installation cannot unblock those prerequisites.
