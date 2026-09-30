# Structured work inputs, freshness and execution cost

**Status:** routed — proposed amendment to existing unfinished briefs; no runtime activation.
**Routes-to:** docs/streams/graph-execution/
**Authored:** 2026-09-30. Source review at `8485778515c041fc87966902a14eb9d195492be3`.
**Scope:** extends [the graph spec](spec.md) and [admission/assurance](admission-assurance-spec.md).

## One execution foundation

Use the existing node/instance contract, eligibility evaluator, attempt journal and Cell
reservation interfaces. A desk packet is a role-specific **view** of these records, not a
second work identity, scheduler, pattern bank or record family. Model agents perform task
work and judgment; deterministic code observes changes, waits, claims, dispatches and
reconciles. No relevant change means no model invocation. Polling still has API and compute
costs. Keep the per-role authority boundary and independent review/verification.

The parent lifetime does not set a worker's conversation lifetime. Local exploration stays
inside a node; checkpoint at an artifact, independent check, decision or external effect.
Neither a universal small prefix nor an arbitrary turn count is a correctness constraint.
Context limits, model choice and account admission are separate experimental dimensions.

## WI-1 — A packet view with sufficient evidence (09; consumed by 14)

Project from canonical work, instance, node and attempt IDs:

- intent, scope/exclusions, required acceptance claims and unresolved inputs;
- immutable source, code, acceptance and policy references; separate design base, review
  head and merged execution tree where applicable;
- a dependency manifest linking source/API/schema/build/policy/environment fingerprints to
  the conclusions or acceptance rows they support; missing coverage is explicitly unknown;
- prior artifact/receipt references, completed scope, remaining scope, and next safe action;
- a computed delta since the prior attempt, including affected dependencies and assumptions;
- required role/control profile, tools, actual runner/model and budget reservation reference.

Keep large diffs/logs as immutable, hashed artifacts with a coverage index. Include the
material needed for the current judgment and access to the rest. Source pointers are not a
substitute for reading required evidence. Record omissions; reject silent truncation.
Selected tool/policy bundles must retain their required enforcement. Stable prompt text
alone does not prove stable runner serialization or provider cache reuse.

GEA-14 also binds role packets and WI-4 usage records: carry identifiers and authorized
references, never keys, tokens or authentication headers. Preserve each reused source's
origin, trust-gate disposition and restrictions. Untrusted or quarantined reporter text
remains source data; copying it into a packet never makes it an instruction or grants
authority. A missing disposition goes through the existing trust gate before use.

The returned handoff binds result scope, evidence, dependency updates, artifacts, unresolved
questions, next action and usage to that same attempt. Mechanical fields are assembled in
code. An agent resolves ambiguous intent; count its preparation cost. Ordinary work uses
its existing brief/acceptance contract; the full refactor oracle remains for redesign only.

## WI-2 — Freshness and conditional reuse (03, 09, 14)

Keep historical evidence immutable. A cached derivation may be reused only for the same
complete input fingerprint, evaluator version and policy. That is not permission to label
old evidence PASS at a new revision. Final review remains tied to the exact change head;
verification remains tied to the required merged subject and acceptance version.

Check bindings before dispatch **and** before accepting a result or advancing work. The
acceptance boundary compares its expected version/head to the current one atomically where
supported; otherwise hold automatic advancement until a supported atomic operation or
fenced boundary establishes the required binding. A pre-write read alone is not a fence.
On movement, keep artifacts, compute the delta and requeue affected work. Do not silently
retarget results, replay writes, or interrupt an in-progress effect without reconciliation.

Start with conservative invalidation. Selective reuse is optional and must produce an
explicit applicability derivation for the current subject and claim; old receipts keep
their original revision. Incomplete dependency knowledge requires broader revalidation.
File non-overlap alone is insufficient: shared APIs, generated inputs, build configuration,
authority rules and transitive callers may invalidate an assumption outside the edited files.

Example fixture: a design at A depends on a writer's authorization policy; another change at
B alters that policy while the reader file is untouched. The affected acceptance row holds
until revalidated at B. An unrelated edit fixture can preserve source analysis, but cannot
copy an exact-head review approval onto B. A target moving again during execution must be
caught at acceptance. Test these as connected paths, not only serializer examples.

## WI-3 — Actionable states and recovery (14, 16)

Coalesce notifications by work/node, instance/acceptance version and relevant input state.
A model launch must record why work is now actionable: new item, changed dependency,
missing evidence or retry after failure. Repeated ticks do not manufacture new attempts.
Claim/reservation and launch deduplication must be atomic at the existing authority; failed
or lost launches can retry under a new attempt without permanently suppressing the work.
Keep intake incident deduplication distinct from execution-attempt identity.

Checkpoints preserve artifacts, unresolved effects, lease generation, cancellation intent
and cumulative spend. Resume checks them before any repeated effect. Charge failed attempts,
summaries, fallback and independent verification to the original work. Unknown outcomes
remain unknown until reconciled. Do not claim exactly-once where the provider cannot enforce it.

## WI-4 — Resource accounting without new authority (16)

Reservation records carry account/provider, actual model, raw input/cache-write/cache-read/
output usage, exact timestamps, request identity, attempt lineage and pricing version.
Deduplicate across retries/resumes where identity permits; missing telemetry is unknown.
Keep each provider's units separate and distinguish reported tokens, price proxies and
observed account depletion. Account for rolling and longer-period limits, in-flight spend,
concurrent non-cohort use and reserved operator headroom. A deterministic threshold alarm
is not a substitute for reservations.

The core owns the durable budget seam and fake-provider conformance tests. Concrete model
meter adapters/calibration belong to adopting pilots; no provider SDK, live billing probe
or hard-coded rate is introduced by these briefs. Launch admission does not imply a hard
per-request stop. Record the supported control boundary and uncertainty for each runner.
Do not silently change provider or weaken controls to fit a reservation.

## WI-5 — Matched measurement and fault corpus (18)

Use the existing run records and flow instruments. Trace arrival, eligibility, dispatch,
active work, waits, review, verification and acceptance with task/attempt/version identity.
Measure packet preparation and indexing as well as model calls. Report:

| Objective | Denominator / measurement |
|---|---|
| Speed | Arrival-to-independent-acceptance median/p95; queue, blocked, active and review time separately |
| Cost | Cohort total spend including failed/abandoned attempts divided by accepted outcomes; also matched-task distribution |
| Repetition | Duplicate concurrent launches for unchanged actionable state; rediscovery calls per accepted task, with sampled label validation |
| Survival | Stale acceptance, relevant changes detected, duplicated/lost effects, resume time and tokens |
| Selectivity | Unnecessary invalidations on seeded unrelated edits; reusable scope, with dependency coverage stated |
| Throughput | Accepted original work units per account window at comparable scope and fixed human-review capacity |

First compare structured/current inputs at the same model, tools and session profile. Then
cross context-profile changes with model choice. Counterbalance order/cache warmth, retain
failures and human effort, and report sample sizes and uncertainty. Do not count extra jobs
created by splitting a brief as extra completed work.

Required offline replay cases: duplicate event/unchanged state; failed launch then retry;
unrelated edit; relevant source edit; policy/API change outside touched files; target movement
during execution; crash after effect before acknowledgment; overlapping reservations and
restore with spend/stop state intact. The fixture oracle declares affected claims in advance.
No stale acceptance or duplicated effects are permitted in the exercised corpus; report
zero misses as a corpus result, not proof of general reliability.

Actual model/quota claims require an explicitly authorized adopting pilot. Candidate pilot
margins: 20% lower median completion time and matched-task quota cost from structured inputs,
p95 within 1.2× control; broader profile/model experiments can separately target 2× cost
reduction. Margins are hypotheses, not default admission policy or changes to existing stream
gates. A meter too coarse to resolve the difference yields a proxy-only result.

## Routing and compatibility

| Owner | Deliverable | Check that discriminates a plausible wrong implementation |
|---|---|---|
| graph-execution/09 | Packet/dependency view and round-trip provenance | Changed source or omitted required evidence cannot round-trip as the old current view |
| graph-execution/03 | Applicable evidence and stale acceptance | A policy-only dependency edit holds the affected claim; old PASS cannot release it |
| graph-execution/14 | Dispatch reasons, coalescing and both freshness boundaries | Duplicate events launch once; failed launch retries; head movement blocks acceptance |
| graph-execution/16 | Durable reservations and attempt accounting | Concurrent reservations cannot double-spend; restart retains spend and stop state |
| graph-execution/18 | Connected offline replay and measurement export | Injected wrong revision and omitted failed-attempt cost change the verdict/report |

`build-less-brittle/04,09,12` structure intake, investigation and oracle records in their own
existing artifacts. They can work without graph tooling and introduce no scheduler dependency.
Implemented schema/flow briefs (02/07) are not retroactively marked incomplete: pending 09
owns compatible instance extensions, 18 owns experiment integration. Scope remains bounded;
semantic dependency inference and automatic cross-provider routing are not deliverables.
