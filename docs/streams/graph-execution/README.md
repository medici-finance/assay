---
stream: graph-execution
repo: medici-finance/assay
serves: assay
status: active
priority: P1
track: platform
issues: [1213]
board: generated
spec: docs/streams/graph-execution/spec.md
---

# Graph-Execution Stream — reviewed workflow patterns over one eligibility evaluator

A brief already declares its graph — `depends:`/`unblocks:`, and since brief-v2 the reserved
`gates:`/`feathers:` edges — and `statusgen` scores a frontier from it. The graph is
**rendered, not executed**: the reserved edges are parsed and then explicitly not gating
(`statusgen/briefv2.go`); which lane an item takes lives in each desk's routing code; no
deterministic rule says which claims must carry evidence at the item's revision before
downstream work is released; and `drainloop`'s journal is best-effort, so an effect whose
acknowledgment was lost can be repeated. The scoping document
[`spec.md`](./spec.md) states the design: **a small bank of reviewed, versioned workflow
patterns, selected per task, over one deterministic eligibility evaluator and the existing
executor interfaces.** The graph carries obligations, dependencies, permitted effects and
required evidence; the agent keeps its freedom inside that contract.

**First correctness milestone:** editing a declared dependency, gate or risk class changes
dispatch with **zero diff to role-specific routing code**. Brief 05's fixture is that proof.

## End state — what "done" means

`statusgen --eligibility` answers, for every brief, *eligible / held by `<ref>` because
`<reason>` / eligible with feather `<ref>` unavailable*, and Next-up reads that answer instead
of `depends:` alone. Three reviewed patterns (implementation, research, signal-triggered)
live as versioned files under `spec/`, each node an execution contract that points at — and
can never exceed — the roster's role bindings. A coverage rule refuses to release a downstream
node while any mandatory claim is `missing`, `error`, `could-not-check` or `wrong-revision`.
An effect-bearing node cannot run without a run/attempt identity and cannot retry without
reconciling its receipt. The offline experiment has run the eight cases against today's fixed
procedure and recorded the comparison; one incident-derived pattern revision has been replayed
over its motivating cases and unchanged holdouts. Flow instruments report eligible-to-start
delay, active time, external wait, verification time, CI-slot saturation and gate
catch/override rate with environment identity stamped on every number.

## Scope — the eight units, and what each owns

1. **Eligibility evaluator (brief 01).** `gates:`/`feathers:` become gating with an
   explainable reason per brief; Next-up consumes the evaluator. An unresolvable cross-repo
   ref is could-not-check: a hard gate HOLDS, a feather NOTICEs. Head of one branch.
2. **Pattern schema + node contract + two patterns (brief 02).** `spec/workflow-pattern-v1.md` (planned)
   and `spec/workflow-patterns/{implementation,research}-v1.yaml` (planned); the lint that refuses a
   node whose effects exceed its role binding, and a risk class as a declared input. Head of
   the critical path.
3. **Evidence coverage rule + `observe` kind (brief 03).** The deterministic aggregation over
   Evidence rows and witnesses, revision-bound; the join node's integration check; the
   duration evidence kind, declared only where a deploy exists.
4. **Recovery contract in `drainloop` (brief 04).** Run/attempt identity, effect receipts and
   idempotency keys, journal mandatory for effect nodes, reconcile-before-retry; the
   lost-acknowledgment test.
5. **Offline two-pattern experiment (brief 05).** Frozen fixtures, the eight cases, the
   baseline comparison, the zero-routing-diff proof, the written report.
6. **Run records + replay/learning loop (brief 06).** The run-record schema, a replay fixture
   from a failed run, one versioned pattern revision, regression over motivating cases and
   holdouts, outcome and permitted-method scored separately.
7. **Flow instruments (brief 07).** Transition latencies from the historian split into
   service and wait, CI-slot saturation, gate catch/override rate, environment identity on
   every comparison.
8. **Signal-triggered pattern (brief 08).** The incident/regression pattern with its four
   obligations and two exits, over the schema from 02; signal liveness stated as its
   precondition, not delivered here.

**Out of scope:** a graph database, a trained router, a new orchestration platform, a
PID-tuned controller (the orchestration-platform exclusion is narrowed 2026-10-02 for the
routed execution host only — see "Task workflow execution routing" below); any real-work trial
(an adopter's own brief, in the adopter's tree);
admission-control policy values (an adopter's ruling from its own measured demand); moving
or removing any human gate (spec §5 states the divergence).

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Eligibility evaluator — gates and feathers become gating, with a reason](brief-01-eligibility-evaluator.md) | 0 | L | done | 2026-09-17 sonnet-5-verifier (9/12 clean PASS; rows 6/12 literal-fail-shaped but independently proven pre-existing board-drift, not regressions; risk-values DERIVED) | 2026-09-19 assay-reviewer-app[bot] (approved PR #1298 @ d3b1f88dd9a6941e7c8449a460513e4638b8ae38) |
| 02 | [Workflow-pattern schema, node contract, and the implementation and research patterns](brief-02-pattern-schema-and-node-contract.md) | 0 | L | verified | 2026-10-25 assay-verifier-app[bot] @ 4aeecf71626f (forge-identity) | — |
| 03 | [Evidence coverage rule and the observe evidence kind](brief-03-evidence-coverage-rule.md) | 1 | L | implemented | — | — |
| 04 | [Recovery contract for effect-bearing nodes in drainloop](brief-04-recovery-contract.md) | 4 | L | verified | 2026-10-30 assay-verifier-app[bot] @ 40925e8233c8 (forge-identity) | — |
| 05 | [Offline two-pattern experiment on frozen fixtures](brief-05-offline-experiment.md) | 8 | L | todo | — | — |
| 06 | [Run records and the replay/learning loop](brief-06-run-records-and-replay.md) | 9 | L | todo | — | — |
| 07 | [Flow instruments — service/wait split, CI-slot saturation, gate catch/override](brief-07-flow-instruments.md) | 1 | M | implemented | — | — |
| 08 | [Signal-triggered pattern — incident and regression](brief-08-signal-triggered-pattern.md) | 1 | M | todo | — | — |
| 09 | [Versioned workflow instances and shared identity](brief-09-instance-contract.md) | 1 | M | verified | 2026-10-19 assay-verifier-app[bot] @ 4b15482af5c6 (claude-opus-5-5) | — |
| 10 | [Typed advice and separate deterministic policy records](brief-10-decision-contract.md) | 0 | M | done | 2026-09-30 assay-verifier-app[bot] @ b0088804294b (claude-opus-5-5) | 2026-09-30 assay-reviewer-app[bot] (approved PR #1499 @ f4c4e5222b775331f2d255daf981984b029023b3) |
| 11 | [Optional pinned Laya provider with explicit CPU and GPU profiles](brief-11-laya-local-provider.md) | 1 | M | todo | — | — |
| 12 | [Reproducible decision evaluation and calibration manifests](brief-12-decision-evaluation.md) | 2 | M | todo | — | — |
| 13 | [Deterministic admission over facts and bounded probabilistic advice](brief-13-agentic-admission.md) | 1 | M | implemented | — | — |
| 14 | [Bind admission and graph eligibility at the dispatch boundary](brief-14-admission-dispatch-binding.md) | 6 | M | todo | — | — |
| 15 | [Control profiles and complete scoped evidence exports](brief-15-control-evidence.md) | 10 | L | todo | — | — |
| 16 | [Cell ownership, cumulative budgets and restoration fencing](brief-16-cell-ownership-budgets.md) | 5 | L | todo | — | — |
| 17 | [Graph-linked release and outcome records without new authority](brief-17-lifecycle-links.md) | 10 | M | todo | — | — |
| 18 | [Offline graph, advice and assurance integration proof](brief-18-assurance-experiment.md) | 12 | M | todo | — | — |
| 19 | [Durable instance store and production coverage binding](brief-19-durable-instance-store.md) | 6 | L | todo | — | — |
| 20 | [Shared loop-admin runner protocol and offline conformance kit](brief-20-runner-contract.md) | 0 | M | verified | 2026-10-19 assay-verifier-app[bot] @ 3d29dd923ab7 (claude-opus-5-5) | — |
| 21 | [Deterministic task controller with durable dispatch and waits](brief-21-controller-host.md) | 7 | M | todo | — | — |
| 22 | [Shared local harness adapter with Codex and Claude qualification](brief-22-local-harness-adapter.md) | 7 | L | todo | — | — |
| 23 | [Immutable candidate workspaces and independent check inputs](brief-23-candidate-workspaces.md) | 7 | M | todo | — | — |
| 24 | [Versioned author implement review pattern with bounded repair](brief-24-internal-specialist-pattern.md) | 10 | M | todo | — | — |
| 25 | [Exact-candidate publication and independent review evidence bridge](brief-25-publication-review-bridge.md) | 11 | M | todo | — | — |
| 26 | [Shared loop-admin launch supervision for desks and workflows](brief-26-shared-loop-admin.md) | 6 | L | todo | — | — |
| 27 | [Standing desk clients for the shared loop-admin](brief-27-standing-desk-loop-admin.md) | 8 | L | todo | — | — |
| 28 | [Verifier domain contracts with qualified identities and candidate-bound acceptance](brief-28-verifier-domain-contracts.md) | 2 | M | verified | 2026-10-28 assay-verifier-app[bot] @ 8db7d356334e (forge-identity) | — |
| 29 | [Old and new regression floor for the verifier domain contracts](brief-29-verifier-regression-floor.md) | 3 | M | verified | 2026-10-28 assay-verifier-app[bot] @ 9363e5df55fa (forge-identity) | — |
| 30 | [Derived evaluation context and bounded validity](brief-30-derived-evaluation-context-and-bounded-validity.md) | 3 | M | verified | 2026-10-29 assay-verifier-app[bot] @ 5729430698d4 (forge-identity) | — |
| 31 | [Accountable principal and assurance provenance](brief-31-accountable-principal-and-assurance-provenance.md) | 3 | M | implemented | — | — |
| 32 | [Investigate criterion-level evidence freshness](brief-32-investigate-criterion-level-evidence-freshness.md) | 0 | M | verified | 2026-10-27 assay-verifier-app[bot] @ 47ef38f961aa (forge-identity) | — |
| 33 | [Merge as a human decision node, and a derived risk-input mapping](brief-33-merge-decision-node-and-risk-input.md) | 0 | L | todo | — | — |
<!-- statusgen:briefs:end -->

## Structured-work amendment (2026-09-30)

[The amendment](work-input-amendment.md) assigns packet views to 09, applicability to 03,
dispatch/coalescing to 14, cumulative accounting to 16, and connected replay/measurement to
18. Existing waves and activation gates were unchanged by that amendment; waves have since
been recalculated by the 2026-10-02 routing below, and the authoring table is current. 02 and 07's implemented delivery
records are retained; their consumers own compatible extensions. The first runnable proof
uses offline fixtures; real provider savings need a separately authorized adopting pilot.

## Standing-desk token priority — 2026-10-18

The [single delivery path](task-workflow-program.md#standing-desk-efficiency-delivery--2026-10-18)
reconciles desk drain policy, process supervision, Codex/Claude qualification and skill-loop
retirement. Its dependency waves and current blockers supersede abbreviated execution
chains below for the standing-desk milestone. Existing brief identities, completed Evidence,
PostgreSQL routing and safety gates remain authoritative. /22 is now L for two qualified
profiles; /27 is L for deterministic five-role integration and measured retirement.
No parallel drain engine, model timer or telemetry store is commissioned.

## Critical path

```
[No external-environment head. Everything is source, spec files, fixtures and CI in this
 repo; the experiment is offline by design.]
                 |
  01 eligibility evaluator ──┬──► 07 flow instruments
                             │
  02 pattern schema ─────────┼──► 03 coverage rule ──┐
                             ├──► 04 recovery contract ┼──► 05 offline experiment ──► 06 run records + replay
                             └──► 08 signal-triggered  │
                                   (01 also feeds 05) ─┘
```

**In-stream head: 02**, with 01 alongside it. Longest chain is `02 → 03 → 05 → 06` (equal
length via 04). 02 is at the head because 03 needs the evidence kinds a node declares, 04
needs the effect vocabulary, 08 is an instance of the schema, and 05 runs the two patterns
02 ships. **Head verification (2026-09-16, `statusgen/briefv2.go` and `statusgen/nextup.go`
on this repo's main):** the reserved keys are parsed and lint-validated with an explicit
"reserved, not gating" NOTICE, Next-up scores from `depends:` only, and `topology.yaml`
carries the role names (`desk`, `reviewer`, `verifier`, `worker`) a node contract can point
at — nothing upstream of 01 or 02 is missing, so neither is a dead end. The tempting wrong
first step is 05: an experiment written before the evaluator, the schema and the coverage
rule exist would compare two hand-scripted procedures and prove nothing about declarations.

## Shared conventions

- **Public tree.** Every fixture, test name and doc example uses `example-org/*` placeholders
  and the illustrative `topology.yaml` roles; nothing names a private repo, a private
  measurement, or an adopter's numbers.
- **Vocabulary is fixed in 02 and reused verbatim.** Node kinds `artifact | check | decision |
  effect`; evidence kinds `command | review | witness | observe`; evidence results `pass |
  fail | missing | error | could-not-check | wrong-revision`; eligibility verdicts
  `eligible | held | eligible-with-notice`. A brief that needs a new value adds it to
  `spec/workflow-pattern-v1.md` (planned) in the same change.
- **Three-state everywhere.** Could-not-check is reported, never collapsed into pass or fail
  (`docs/three-state-instrument-rule.md`).
- **Docs ride the brief.** A brief that changes public behaviour updates
  `docs/dependency-graph-design.md`, `docs/lifecycle.md` or `docs/enforcement-model.md` in
  the same change and lists the file in its `files:` line; there is no separate docs brief.
- **Changelog fragment** per PR (`changelog/<slug>.md`), listed in every brief's `files:`.

## Proposed admission and assurance extension (2026-09-18)

Read [the amendment](admission-assurance-spec.md). This extends the existing graph;
it does not commission another runtime. 01 is now done; 02 retains its implemented status. The
starting-state prose above is historical; source presence is not deployed verification.
Existing 03–06 gain shared-contract amendments; their statuses remain todo.

Waves as of that amendment (historical — the authoring table above is current, after the
2026-10-02 routing moved 05/06/15/17/18): 0 = 10; 1 = 09, 11, 13 alongside existing 03/04/07/08;
2 = 12, 16 and existing 05; 3 = 14 and existing 06;
4 = 15, 17; 5 = 18. Existing 05 additionally depends on 09.

```
02 -> 09 ----+-> 05 -> 06 -> 15/17 --------+-> 18
02 -> 03/04 -+                            |
04 + 09 -> 16 -> 14 ----------------------+
10 -> 13 --------^                       |
10 -> 11 -> 12 --------------------------+
01 -> 14; 07 -> 18
```

The longest new integration chain runs through existing coverage/recovery, 05 and 06,
then 15/17 and 18. The decision-provider branch can start immediately at 10; 09 can
start from the implemented pattern schema, subject to independent verification before
release. The real initial seams were inspected at 951ca784d on 2026-09-18: instantiation
is explicitly absent from pattern v1; Decide has no assessment distribution; no Laya
adapter or admission module exists at the planned paths. The original graph milestone
is not blocked on model evaluation or a GPU. Source implementation is not an operational
receipt. Do not add both the old migration package estimate and its graph equivalent.

Source refresh 2026-09-19: public Assay `e4109205751a219330b954f75855c05b4be2a5c8`;
01 verification is now recorded. No changes to the proposed 09–18 target seams were found.

## Task workflow execution routing — 2026-10-02

[The execution extension](task-workflow-program.md) is routed into **19–27**, all todo.
It permits one thin controller host over existing contracts, and two local embedded stores
and no other: the single-controller embedded SQLite store of the existing instance/node/attempt
record family (19), and each loop-admin supervisor's own execution journal (26), a per-supervisor,
process-facts-only record of invocations, receipts, lifecycle, limits and usage on the
supervisor's own filesystem, whose embedded engine is qualified in 26 (SQLite is the expected
choice). Neither is a shared or distributed store, and the journal never holds work, decisions
or acceptance. The earlier platform exclusion still prohibits a graph database or graph-store service, a
distributed or shared store or lease service, a message bus, a credential broker, a new rule
engine, a second scheduler authority and a duplicate evaluator.
Existing 04/05/06/09/14/16/18 carry consumer/flow amendments. Implemented and completed records
01/02/03/07/10/13 remain intact. New source scopes/tests are deliverables, not executed evidence.

The current execution path supersedes the historical critical-path prose above for this extension:

```
02 -> 09 + 03 -> 19 store/binding
20 shared runner contract -> 26 shared supervisor
20 -> 26 -> 22 first adapter over the supervisor and its one process runner; 26 + 22 -> 27 standing desk clients
04 + 09 -> 16
10 -> 13 -> 14 (also 01,09,16)
19 + 20 + 26 + 04 + 14 -> 21 workflow controller -> 05 experiment -> 06 records
19 + 20 -> 23 immutable workspace
21 + 23 + 06 -> 24 internal loop -> 25 publication -> 18 integrated proof
```

The offline host does not wait for live adoption. The verified public experiment/record/flow
and assurance contracts remain prerequisites for an adopter's live trial as declared there.
The runner contract /20 has no graph prerequisite, but the execution delivery does: /26 also requires /28–29 and /16, and /16 requires /04 and /09. Follow the standing-desk priority path above rather than treating /20 as the sole remaining blocker.
The workflow branch starts at 09 and 04 with pure admission 13 alongside them. Model invocation
alone does not supply recovery or ownership. 22 is qualified without graph storage (it needs 20 and 26, not 19); no production
provider calls, automatic fallback or live authority changes are commissioned by these briefs.

Dependency waves (including the existing graph/advice branches) are the authoring table above;
`wave` is recalculated from in-repo `depends`, while adopter gates remain outside this graph.

## Library-first work — 2026-10-08

Library-first update: /19 now depends on `statusgen/15` for the existing evaluator API.
The instance store no longer owns an independent evaluator extraction.
See [the component contract](../../library-first.md). Existing completed history is unchanged.

## Non-CI architecture reconciliation — 2026-10-13

The maintainer ratified the non-CI verifier target (medici-finance/assay#2500, revision
23f655f5d); approval activates nothing and each package's activation stays gated by the
delivery sequence. Package R1 (domain contracts and regression floor) routes here:
`28` defines qualified identities, fact completeness, candidate/acceptance binding and
reevaluation generations as one pure contract over the existing evaluator (statusgen/15);
`29` runs the pinned baseline cases against old and new adapters. Path: 02 verification ->
09 -> 28 -> 29, and 28 -> 23 (23 now consumes the candidate record instead of defining one).
The 2026-10-02 routing above stays as the record of that decision. The ratified target places
runtime coordination in PostgreSQL and keeps SQLite to offline read-only projection, so the
embedded store of 19 and the journal engine of 26 conflict with it. The R2 follow-up amends
19, 26, 16 and 04 and this routing; until it lands, starting the store or journal engine
choice in 19 or 26 needs that amendment first. Owner rows for R1–R8: the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).
Existing completed history is unchanged.

## Non-CI R2 routing — PostgreSQL coordination — 2026-10-13

This section **replaces the storage routing** of the 2026-10-02 section above for this stream;
that section stays as the record of its decision. Authority: the maintainer's ratification of
the non-CI target (medici-finance/assay#2500, revision 23f655f5d); approval activates nothing,
and activation stays gated by the delivery sequence. Owner row: package R2 in the
[amendment manifest](../../proposals/non-ci-architecture/20-amendment-manifest.md).

- **One runtime coordination store: the cell's PostgreSQL database.** Attempts, leases and
  claim generations, schedules, effect intents and receipts, pending decisions and feedback,
  and retirement progress live there (target doc 11 line 92; doc 14 lines 7 and 24). One
  database and restricted role per cell on a shared server where the profile permits
  (doc 11 line 100). The 2026-10-02 permission for an embedded SQLite instance store (19) and
  a per-supervisor embedded journal engine (26) is withdrawn: there is no SQLite runtime
  (doc 19 line 49), and SQLite may remain only an offline read-only projection format.
- **The exclusion of a shared store is rewritten.** The cell's PostgreSQL coordination store
  is the permitted shared store. Still excluded: a graph database or graph-store service, a
  second online database backend, a message bus, a credential broker, a new rule engine, a
  second scheduler authority, a duplicate evaluator and a compulsory per-operation Git journal
  (doc 14 line 36).
- **Owners.** 19 owns the store, the `coordination-store-v1` contract, migrations,
  reconstruction, the carried claim-test intent and the R2 operating obligations. 16 owns the
  operator incarnation (`operator-incarnation-v1`), held outside PostgreSQL. 04 owns the
  store-neutral transactional intent/outbox handoff in drainloop. 26 keeps process facts only,
  now in its own schema in the same cell database, with no Go dependency on `workflow/`.
- **Claim store applicability.** forge-neutral/22–32 (the file/served claim store) remain the
  supported claim path of the existing desk tools. They are not the coordination owner for the
  non-CI runtime: no `workflow/` or `loopadmin/` code binds to them as lease authority. Nothing
  there is retired; see the forge-neutral README's dated note.

Path after R2 (R1 first):

```
02 verification -> 09 -> 28 -> 29
28 + 29 + 02 -> 04 transactional handoff
04 + 09 + 28 + 29 -> 16 operator incarnation
09 + 03 + statusgen/15 + 28 + 29 + 04 + 16 -> 19 PostgreSQL coordination store
20 + 28 + 29 + 16 -> 26 supervisor journal on the cell database -> 22 -> 27
19 + 20 + 04 + 14 + 26 -> 21 -> 05 -> 06;  19 + 20 + 28 -> 23
```

Waves are recalculated from `depends:`: 04→4, 16→5, 19→6, 26→6, 14→6, 21→7, 22→7, 23→7,
05→8, 27→8, 06→9, 15→10, 17→10, 24→10, 25→11, 18→12; iso-9001/09–11 move to 11–13 through
their edge to 15. Only wave numbers changed in briefs outside R2.

**R3 follow-up, not in this routing:** R3 later amends 16 (exclusive repository assignment),
26 (runner guard events, budgets, resource capacity and custody) and 22 (first real adapter).
R4 later amends 04 and 25. Those sections are marked in the briefs and are not authored here.


## Public runtime alignment and gap routing — 2026-10-14

The [runtime contract](../../proposals/non-ci-architecture/23-runtime-contract-alignment.md)
selects one public Assay implementation. Existing PostgreSQL, recovery, candidate, runner,
evidence and controller briefs remain owners; Scion source/cache work is forge-neutral/39–45.
Only /30 (context/validity) and /31 (principal/proof provenance) are newly authored here.
Generated lifecycle cells remain authoritative; no implementation is claimed.

Critical path and waves for this addition:

```text
02 independent verification -> 09 -> 28
                                  ├-> 30 -> 19 and 14
                                  └-> 31 -> 14 and 25
existing R2/R3/R4 dependencies -> managed consumers -> R5/R7/R8 proof -> operator activation
```

/30 and /31 are wave 3 and can develop independently after /28. Existing consumer waves
remain /19=6, /14=6 and /25=11 because their other prerequisites are later. The smallest
unblocking move remains independent /02 verification; a cache daemon or new SCM does not
unblock it. Missing `workflow/` and `statusgen/domain/` at inspected main 9a8df91dc are
planned deliverables, not a reason to claim the tests already pass. /31's trust-profile
activation requires its human decision; no automatic waiver or stronger assurance is added.

## Criterion evidence freshness investigation — 2026-10-16

[Brief 32](brief-32-investigate-criterion-level-evidence-freshness.md) is an independent
Wave 0 research item: trace the existing requirement-to-witness path and test acceptance
edits against actual controls before proposing another mapping or identity mechanism.
Its critical path is source inventory → fixed cases → offline replay → decision report;
there is no implementation dependency on the proposed domain work in 28–31. Fresh main
`d0580c5e5` was inspected on 2026-10-16: criteria are strings in the requirement register,
while Command/Expect and dependency applicability already have baseline guards. The first
unblocking move is reproducing their actual coverage, not assigning new criterion IDs.

A no-change or guidance-only conclusion is valid. This research neither changes the main
implementation dependency waves above nor authorizes a production freshness gate.

## Execution-environment follow-ons — 2026-10-16

The [agent-execution stream](../agent-execution/README.md) extends existing owners under
[architecture amendment 27](../../proposals/non-ci-architecture/27-agent-execution-environments.md).
Its /01 consumes implemented /20; /03 follows /22 and /26; /04 qualifies /21 and /25 under
restoration; /05 follows /27. Reciprocal edges are declared in the brief frontmatter.
Original Task/Verify obligations and Evidence remain unchanged. The new stream owns no
second controller, claim store, publisher, process runner or lifecycle API. Its AX feasibility
branch is offline and independent of this stream's existing critical path.

## Proposed desk-improvement evidence consumers — 2026-10-17

[Public desk-improvement](../desk-improvement/README.md) consumes this stream's native
records instead of introducing another execution journal. Its /05 binds /06 run records;
/06 captures the /26 supervisor and /22 adapter; /07 extends /21 controller evidence; /08
consumes /27 standing clients; /10 qualifies native cross-mode source→card→assessment flow.
These consumer prerequisites point toward this stream, with no reverse gating cycle. Existing
source scope and activation remain controlling; supported native measurement is a separate
qualified capability from offline cards or an implemented schema.

## Verifier migration sequence — 19 October 2026

[Execution-first migration](verifier-migration.md) maps the six visible verifier responsibilities
to the existing owners and keeps evidence production separate from authority handover.
The dependency amendment removes real-model calibration /12 from the model-disabled /18
integration prerequisite; calibrated-provider activation still requires /12. Kubernetes is
the separately gated agent-execution/08 profile, not an interim runner prerequisite.

The historical critical-path sketch retains the former /12 → /18 edge for history; it is superseded by the 19 October verifier migration sequence above.

## Merge decision amendment — 2026-10-26

[Brief 33](brief-33-merge-decision-node-and-risk-input.md) amends the shipped implementation
pattern, per the driver's ruling:
https://github.com/medici-finance/assay/issues/1933#issuecomment-6419943765
Its `merge` node becomes a `human` decision closed by an external authenticated act, `human`
is allowed as an owner only on effect-free decision nodes, the pattern and its testdata twins
are re-pinned, and the `risk-input` mapping is derived rather than named. The role-effect table
is not widened. graph-execution/02 stays held until /33 lands, read as /33 reaching `verified`;
the hold note is in brief 02's `## Review` section. /33 is a Wave 0 item with no open
dependency, because /02 and /09's deliverables are already on main; it is a new prerequisite of
/14, which binds the re-pinned pattern at dispatch.
