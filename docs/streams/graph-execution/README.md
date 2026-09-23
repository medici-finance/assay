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
PID-tuned controller; any real-work trial (an adopter's own brief, in the adopter's tree);
admission-control policy values (an adopter's ruling from its own measured demand); moving
or removing any human gate (spec §5 states the divergence).

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Eligibility evaluator — gates and feathers become gating, with a reason](brief-01-eligibility-evaluator.md) | 0 | L | done | 2026-09-17 sonnet-5-verifier (9/12 clean PASS; rows 6/12 literal-fail-shaped but independently proven pre-existing board-drift, not regressions; risk-values DERIVED) | 2026-09-19 assay-reviewer-app[bot] (approved PR #1298 @ d3b1f88dd9a6941e7c8449a460513e4638b8ae38) |
| 02 | [Workflow-pattern schema, node contract, and the implementation and research patterns](brief-02-pattern-schema-and-node-contract.md) | 0 | L | implemented | — | — |
| 03 | [Evidence coverage rule and the observe evidence kind](brief-03-evidence-coverage-rule.md) | 1 | L | todo | — | — |
| 04 | [Recovery contract for effect-bearing nodes in drainloop](brief-04-recovery-contract.md) | 1 | L | todo | — | — |
| 05 | [Offline two-pattern experiment on frozen fixtures](brief-05-offline-experiment.md) | 2 | L | todo | — | — |
| 06 | [Run records and the replay/learning loop](brief-06-run-records-and-replay.md) | 3 | L | todo | — | — |
| 07 | [Flow instruments — service/wait split, CI-slot saturation, gate catch/override](brief-07-flow-instruments.md) | 1 | M | implemented | — | — |
| 08 | [Signal-triggered pattern — incident and regression](brief-08-signal-triggered-pattern.md) | 1 | M | todo | — | — |
| 09 | [Versioned workflow instances and shared identity](brief-09-instance-contract.md) | 1 | M | todo | — | — |
| 10 | [Typed advice and separate deterministic policy records](brief-10-decision-contract.md) | 0 | M | implemented | — | — |
| 11 | [Optional pinned Laya provider with explicit CPU and GPU profiles](brief-11-laya-local-provider.md) | 1 | M | todo | — | — |
| 12 | [Reproducible decision evaluation and calibration manifests](brief-12-decision-evaluation.md) | 2 | M | todo | — | — |
| 13 | [Deterministic admission over facts and bounded probabilistic advice](brief-13-agentic-admission.md) | 1 | M | todo | — | — |
| 14 | [Bind admission and graph eligibility at the dispatch boundary](brief-14-admission-dispatch-binding.md) | 3 | M | todo | — | — |
| 15 | [Control profiles and complete scoped evidence exports](brief-15-control-evidence.md) | 4 | L | todo | — | — |
| 16 | [Cell ownership, cumulative budgets and restoration fencing](brief-16-cell-ownership-budgets.md) | 2 | L | todo | — | — |
| 17 | [Graph-linked release and outcome records without new authority](brief-17-lifecycle-links.md) | 4 | M | todo | — | — |
| 18 | [Offline graph, advice and assurance integration proof](brief-18-assurance-experiment.md) | 5 | M | todo | — | — |
<!-- statusgen:briefs:end -->

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

Waves: 0 = 10; 1 = 09, 11, 13 alongside existing 03/04/07/08;
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
