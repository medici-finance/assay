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
