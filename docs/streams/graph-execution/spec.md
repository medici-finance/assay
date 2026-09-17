# graph-execution — scoping document

**Status:** approved — commissioned 2026-09-16; the approval is stamped by the merge of the
pull request that lands this document, per `spec/lifecycle-v1.md` §8.4.
**Routes-to:** docs/streams/graph-execution/
**Source:** the maintainers' research assessment of graph execution (2026-09-14, amended
2026-09-16), which reconciled a July 2026 lifecycle-handoff design with the code as built and
with a corpus of conference talks. The assessment is an internal document; every claim this
spec makes is grounded below in public code paths or public talk recordings.
**Authored:** 2026-09-16, strong-tier authoring session (author-brief).
**Owner:** methodology track.

## 1. The problem, stated from the code

A brief already declares the graph it belongs to: `depends:`/`unblocks:` for structure, and
since brief-v2 the reserved `gates:` and `feathers:` keys for behavioural and cross-repo
edges. `statusgen` computes a scored frontier from those declarations and the desks
dispatch from it. What the graph does **not** do is execute:

- `gates:` and `feathers:` are parsed, type-checked and lint-validated, and then explicitly
  **not gating** (`statusgen/briefv2.go`, header comment: "GATING behaviour is deferred to
  the graph stream"). A declared hard prerequisite changes nothing about what Next-up offers.
- Which review lane, which verifier and which sign-off an item needs is decided inside each
  desk's routing code, not read from a declaration. Changing the policy means editing tools.
- Evidence rows and witnesses are recorded, but no deterministic rule says which claims
  MUST have applicable evidence at the item's revision before downstream work is released.
  `evidenceactor.go` emits NOTICEs; `autoflip.go` promotes `verified` to `done` on a
  reviewer-App approval and, by its own header comment, does not re-check the stamp it
  promotes.
- `drainloop` has claim, retry, evidence and an optional journal, and its journal is
  best-effort by contract (`drainloop/journal.go`: "a returned error is logged but does not
  abort the drain"). Nothing requires an effect-bearing step to reconcile an authoritative
  receipt before a retry, so a worker that dies after an effect succeeds but before recording
  it can cause the effect twice.
- The historian (`statusgen/history.go`) records observed status changes at regeneration
  time; the factory-floor score (`statusgen/bottleneck.go`) is WIP × current stage age. Neither
  separates active service from waiting, so a "bottleneck" reading cannot tell scheduling
  delay from capacity shortage.

An earlier internal design (July 2026) proposed faster lifecycle handoffs: queue-time
instrumentation, an evented verification feed, WIP limits, SHA-keyed gates, event propagation
and atomic claims. Parts of that landed (the human sign-off pair
`.github/workflows/verify-gate-open.yml` / `verify-gate-close.yml`; slug-identified
registers; forge claims). Faster handoffs remain useful, but they do not by themselves make
the workflow adapt to the task or prove the execution was reliable. This stream takes the
second problem.

## 2. The design — reviewed patterns over one evaluator

Build a **small library of reviewed, versioned, executable workflow patterns**, selected and
adapted per task, over **one deterministic eligibility evaluator** and the **existing
executor interfaces** (`drainloop`, the desk verbs). The graph expresses obligations,
dependencies, permitted effects and required evidence; an agent keeps the freedom to choose
any valid route inside that contract.

**First correctness milestone:** editing a declared dependency, gate or risk class changes
dispatch (readiness, lane, route) **without editing a desk's routing code**. The proof is a
fixture where the only diff between two runs is one declaration line.

The five parts, each with the public talk that motivated it. A talk is evidence that the
idea works somewhere; none of them is a measured result for Assay.

### 2.1 A pattern bank, selected then adapted

Precompute a bank of workflows and adapt one per task, between "one fixed topology for
everything" and "a fresh topology per query" (Furong Huang, bank-and-reuse at
[10:03–10:31](https://youtu.be/GfmK-v8CARk?t=603)). Begin with **two** patterns —
**implementation** (preserves review, merge and independent verification) and **research**
(collect sources, resolve gaps, synthesise, review the artifact) — and a **third,
signal-triggered** pattern (incident/regression: contain → confirm and de-duplicate →
root-cause with a mandatory replay fixture → propose, with two exits: a draft fix PR into
the ordinary review lane, or a mitigation left for a human to execute). Recurring reports
can compose collection, freshness, delta detection, synthesis and delivery acknowledgment
later. The bank stays small and versioned; select a known pattern first and introduce
learned selection only if simple routing proves insufficient. The definitions live with the
methodology spec, not in a second catalogue maintained in parallel with the desk skills.

### 2.2 A node is an execution contract

Nodes declare what they do, what they require and whom they may call (Daniel Fink, Neuro
SAN, [01:20–01:33](https://youtu.be/6V6BAb3qG6U?t=80); data-driven interaction testing at
[03:05–03:20](https://youtu.be/6V6BAb3qG6U?t=185)). An Assay node contract references its
input artifacts **and their revisions**, expected outputs, required evidence, the existing
actor/capability policy it operates under, budgets, and its wait/failure outcomes. **Risk
class is a declared input**: the classifier's verdict selects which gates are mandatory on
an instance (model review; model + security lane; model + human), so lane selection is a
declaration change, not a change to four tools. The contract **points at** existing
authority definitions; **a generated graph must not grant itself permissions**, and a lint
refuses a node whose declared effects exceed what its role is bound to.

Nodes sit at durable boundaries only — a meaningful artifact, an independent check, a human
decision, an external effect. Local exploration stays inside a node, so the implementation
remains replaceable as model capabilities change.

### 2.3 Verification governs transitions and informs replanning

Separate requirement extraction from execution; the expectation comes from the requirement
and the pre-change context, and execution examines the candidate artifact (Baz:
planner/verifier split at [09:26](https://youtu.be/aWrGSM5vVyc?t=566), per-requirement
dispatch at [11:46](https://youtu.be/aWrGSM5vVyc?t=706), pre-change grounding at
[14:32](https://youtu.be/aWrGSM5vVyc?t=872)). Assay's adaptation is a **deterministic
coverage rule**: every mandatory claim needs applicable evidence at the item's revision;
`missing`, `error`, `could-not-check` and `wrong-revision` results **cannot release
downstream work**. Individually passing requirement checks do not prove the whole; the join
node of a pattern carries an **integration check**. Evidence kinds are point-in-time today;
add a duration kind, `observe: {signal, band, window, source}`, filled by the verifier from
telemetry and unfillable (hence blocking) where the source is unreadable — declared only
where a deploy exists, omitted elsewhere.

Verifier results may feed the planner as a **proposed, versioned revision** to the remaining
work. Prior attempts and evidence are preserved, and a failing worker never deletes its own
acceptance conditions.

### 2.4 Recovery semantics, beyond an audit log

Deterministic orchestration over nondeterministic activities, with retries (Temporal
workshop, [05:06–05:44](https://youtu.be/QjVpE6-G18U?t=306) and
[20:09–22:22](https://youtu.be/QjVpE6-G18U?t=1209)). The discriminating case for Assay:
**an effect succeeded, the worker disappeared before recording success, another worker
resumes.** It must reconcile an authoritative receipt or idempotency key before repeating
the effect. Record stable run/attempt identity, the relevant inputs, effect receipts, and why
a run is waiting; keep reconciliation for events lost or delivered twice. For effect-bearing
nodes the journal stops being best-effort: a record failure aborts before the effect, not
after.

### 2.5 Improvements are learned by replay and review

Mine anomalous runs into small reviewed changes, with a human deciding what ships (Replit,
trace-to-PR at [04:20](https://youtu.be/J8XxVnqUjYE?t=260), ship/wait/drop at
[04:52](https://youtu.be/J8XxVnqUjYE?t=292)). The loop: run record → recurring failure →
replay fixture → small pattern/skill change → regression over the motivating cases **and
unchanged holdouts** → reviewed next version. Run records capture externally meaningful
facts — selected pattern, eligibility reason, blocked edge, tool outcome, intervention,
evidence references — never private reasoning as a source of truth. **Outcome and permitted
method are evaluated independently**: task success alone is an unsafe learning objective,
because it reinforces constraint violations.

### 2.6 Measurement

Test valid outcomes and constraints, not obedience to one sequence (Rajamohan,
[07:14–07:58](https://youtu.be/BIBDhLDgMdE?t=434)). Record eligible-to-start delay, active
work time, external wait, verification time, successful recovery, repeated intervention, and
cost per verified outcome, with environment/model/tool versions on every comparison. Add
**CI-slot saturation** and **gate catch/override rate** before dispatch width rises: offered
load is what an agent fleet manufactures for free, and these two distinguish throughput from
churn. Final-result evaluation (correctness, completeness, usefulness) stays; trajectory
checks add efficiency, recovery and permitted-method information.

## 3. Starting points already in the tree

| Seam | Where | State |
|---|---|---|
| Reserved graph keys | `statusgen/briefv2.go` | parsed, validated, not gating |
| Frontier scoring | `statusgen/nextup.go` | structural `depends:` only |
| Structural/lifecycle split | `docs/dependency-graph-design.md` | design, adopted |
| Evidence attribution | `statusgen/evidenceactor.go` | advisory NOTICE |
| Verified→done flip | `statusgen/autoflip.go` | approval at head; stamp not re-checked |
| Drain engine | `drainloop/` | claim/retry/evidence; journal best-effort |
| Human sign-off handoff | `.github/workflows/verify-gate-open.yml`, `verify-gate-close.yml` | evented, human lane only |
| Stage-age heuristic | `statusgen/bottleneck.go`, `statusgen/history.go` | WIP × age; no service/wait split |

## 4. The experiment that decides the direction

Offline, on frozen fixtures, with the implementation and research patterns over the same
evaluator and executor adapters. Cases: fan-out/join; an unmet hard prerequisite; an
unavailable informational feather; missing or wrong-revision verification evidence; a denied
effect; duplicate delivery; a reviewed revision to pending work; and the lost-acknowledgment
recovery (effect succeeded, no local success recorded, reconciliation required before any
retry). Compare against today's fixed procedure on the same cases.

Pass means: changing a declaration changes readiness or route with **zero** diff to
role-specific routing code; no case enlarges authority or loses evidence; the interrupted
effect is reconciled, not repeated. Then one incident-derived improvement is proposed and
replayed over its motivating cases and unchanged holdouts. A later real-work trial, owned by
the adopter running it, measures whether interventions and time/cost per verified outcome
fall. The conference evidence motivates the experiment; it does not pre-approve its result.

## 5. Non-goals and the divergence stated

Not prerequisites, and not in scope: a graph database, a trained router, a new orchestration
platform, a PID-tuned controller, or a full rollout of any private platform design. A derived
index over reviewed files and execution facts is enough for the first experiment. Inferred
knowledge relationships are not authorized dependencies: hard gates hold work when
unavailable; informational feathers stay visible without blocking.

**Divergence stated.** Vendor accounts remove the human at merge for opt-in low-risk areas.
This design keeps the human at merge and at `gate: human`; it moves no gate later and removes
none. Any automatic lane is opened by category (an owner-opted-in path set) and kept open by
a score recomputed at each gate whose only authority is to eject. A future revision that
changes this must say so against this line.

## 6. Scope split

This stream carries what any adopter can run: the evaluator, the pattern schema and the
three reviewed patterns, the coverage rule, the recovery contract, the offline experiment,
the run-record/replay loop, and the flow instruments — all in this repository. An adopter's
real-work trial, its admission-control policy, its console integration and its public-facing
description of the direction are the adopter's own briefs, in the adopter's tree.
