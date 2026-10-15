# Performance and architecture assessment profiles

**Status: draft, 15 October 2026.** Proposed assessment design and pilot contract;
no new lane, blocker, model spend, scheduled job or repair authority is activated.
Source inspection: public main `7d52be61b66cb81f5ad2ab7e1d250747a6f125de`.

## Purpose and lead questions

Architecture asks **“Does this PR move us toward our future state?”** It gives an
explained direction: advances, preserves, moves away, or cannot assess. Preserving a
sound boundary is a valid result. A necessary transitional compromise records its
owner, removal condition and accepted decision; it need not recreate the destination
inside one small PR. Review against the approved revision and applicable migration
stage, distinguishing proposed future changes from adopted obligations.

Performance asks **“Does this design improve efficiency, avoid undue contention, and
could a better algorithm achieve equivalent results?”** Start from the tool's goals,
workload and required behaviour. Examine design before demanding measurements:
repeated work, data structures, complexity, batching, incrementality, caching, lock
scope, I/O, scheduling and concurrency. Suggest a concrete alternative with a cost
model, assumptions and validation plan. Measurements establish impact; their absence
does not prevent a useful design hypothesis, and a hypothesis is not a measured defect.

The aim is additional important findings at acceptable total delivery cost. These are
two selectively applied assessment profiles, not two unconditional model approvals.
An architecture assessment can run during spec/brief design and again for PR conformance;
algorithm choices with high consequence should be challenged before implementation.

## Current foundation and scope

`tools/desk/internal/deskkit/reviewlanes.go` already defines correctness, security,
fact-check and fail-first lanes. `tools/desk/cmd/deskdispatch/references/review-prompt.md`
contains advisory design-fit review; `reviewscope.go` has BasisDesignFit. The existing
`tools/desk/internal/arch/arch_test.go` checks dependency direction, hub imports,
registered semantic ownership and pure evaluation. These are useful fitness functions,
not proof of all system architecture. Extend them where repeatable findings justify it.
Preserve [build-less-brittle](../build-less-brittle/spec.md), its historical Evidence,
[review scope](../desk-supervision/brief-20-review-scope-and-first-pass.md),
[review finding identity](../desk-supervision/review-finding-v1.md), and existing round limits.

Correctness continues to ask whether behaviour fulfils the applicable contract,
including edge cases, concurrency, errors, recovery and compatibility. Security asks
whether the change preserves trust boundaries, authorization, data custody and safe
handling of untrusted input. Neither profile is removed or weakened by this proposal.
A performance observation that proves wrong behaviour still follows correctness scope;
an architectural concern that proves a concrete safety defect still follows security
scope. Deduplicate the underlying finding across profiles rather than counting it twice.
See [the four-review guide](overview.html) for the reader-facing comparison.

## CI and operational assessment

| Context | Architecture | Performance | Result |
|---|---|---|---|
| Spec/brief design | Does the proposed change advance the applicable future state? Which owner, boundary or temporary mechanism changes? | Can a simpler algorithm or less duplicated work meet the same goals? What workload and tradeoff distinguish alternatives? | Design recommendation or bounded spike before expensive implementation |
| PR/CI | Direction of this delta; conformance with adopted decisions, owners and migration stage | Efficiency of the changed design, scaling/cost model, contention and controlled base/head evidence where needed | Explainable advisory assessment; separately required tests still enforce existing obligations |
| Daily operations | Record notable ownership/temporary-path changes for later assessment | Actual tool runs, CPU, wall time, tokens, forge/GH calls, frequency × cost, queue delay and cost per verified brief | Prioritized observed opportunities, coverage gaps and post-change outcomes |
| Periodic architecture review | Weekly/monthly, owner-selected: accumulated drift, coupling, expired transitional mechanisms, recovery and coordination ownership; reconsider the target through its decision owner | Cross-tool/global contention and workload changes that isolated PRs miss | No-change, investigate, propose change or escalate; no autonomous policy rewrite |

Use the same versioned profile definition and record envelope across contexts, with
different subject, inputs, freshness, applicability and decision authority. A CI finding
is not proof of production impact. Operational findings nominate future regression cases;
a claimed efficiency gain needs later exposure and outcome evidence. Cadence is an
operator-selected proposal, not a new scheduler or an activated schedule.

## Architecture profile

Input: exact base/head, stated intent and applicable requirements, approved architecture
revision, semantic-owner index, relevant decisions, migration stage, dependency diff,
consumer inventory and retirement obligations. The context manifest records source SHA,
access scope, freshness and unavailable inputs; inferred links never become decisions.

Check and explain:

- Movement toward the target: consolidate meaning in its owner, simplify boundaries,
  remove obsolete mechanisms, or create a justified migration seam. Say what moves and why.
- One semantic owner per meaning; no rival eligibility predicate, acceptance decision,
  coordination writer, scheduler, lease authority or finding ledger.
- Dependency direction and pure evaluation; side effects and credential custody at the
  intended adapters/executors. Independent enforcement at distinct trust boundaries can
  be necessary and is not semantic duplication merely because checks look similar.
- Intent, admission, execution, evidence, acceptance and observation remain distinct;
  a model report or projection cannot approve its own candidate.
- Consumer, compatibility, migration, restart/recovery and retirement coverage. A helpful
  local shortcut can still move away from the target if it creates another state owner.

Each concern names the target passage and status, changed mechanism, consequence,
feasible alternative and retirement or decision need. Current obligations can support a
finding under existing scope rules; a proposed destination alone cannot create a blocker.
Do not require unrelated whole-system cleanup. “Cannot assess” names the missing input.

## Performance profile

1. Restate the tool's required result and constraints: correctness, security, determinism,
   ordering, freshness, failure/recovery, memory and available operational budgets.
2. Identify workload dimensions (items, history, files, calls, concurrent workers), expected
   distribution and cold/warm paths. Derive work/IO/call/space complexity and contention.
3. Compare at least the current approach and any justified alternative: e.g. per-item full
   scans versus an index; repeated forge reads versus an exact-SHA shared packet; serial
   requests versus bounded batching; full replay versus an incremental reducer. “No better
   justified alternative” is valid. Do not invent an optimization to fill a quota.
4. State equivalence conditions and tradeoffs. Cache keys need revision/scope and mutable
   freshness; concurrency needs budgets, ordering and backpressure; an index needs update
   and recovery semantics. Faster by skipping required checks or weakening custody fails.
5. Where consequence warrants measurement, specify a discriminating controlled experiment.
   Compare exact base/head on the same environment and workload; separate cold/warm and
   realistic size/concurrency strata. Use trusted workload definitions and fixtures, repeated
   samples, a predeclared sample plan, practical absolute/relative materiality and uncertainty.
   Do not rerun until green or call a sparse tail estimate a reliable P95.

Report separately: measured regression/improvement, violation of an existing explicit
budget, reasoned design opportunity, and inconclusive/missing evidence. Complexity alone
is not a measured bottleneck. Small-input tradeoffs can favour the simpler implementation.
An optional new optimization becomes a bounded follow-up unless it meets existing blocking
scope; an explicit acceptance obligation remains enforceable through its existing owner.

Required CI benchmarks and correctness/security checks produce canonical evidence even
when optional telemetry is off or broken. Independent budget accounting also continues.
Read-only model review does not execute PR code. Candidate execution occurs in isolated,
credential-free test workers; trusted/base-controlled workloads and assertions cannot be
silently rewritten by the candidate. Report changed benchmark definitions separately.

## Shared assessment and integration contract

Brief 01 defines a versioned assessment envelope, referencing rather than replacing
existing findings and reviews. Carry profile ID/version, assessment ID, context and
source manifest digest, exact subject/base/head/contract revisions, trigger/context,
applicability and routing reason, direction or efficiency conclusion, evidence references,
coverage/unknowns, alternatives/tradeoffs, finding identities and disposition authority.
Each run also pins model/provider/version/effort, harness, prompt, policy, router,
workload/benchmark and pricing revisions. Changed bindings make prior evidence inapplicable
where required; a same-head prose edit never fabricates a fresh independent review.

Reuse review-finding-v1 identities, class continuity, first-pass inventory, deduplication,
three-round cap and current-head ready aggregation. A new profile must flow through dispatch,
read-only packet, verdict publication, re-review, counters, reports and readiness tests;
a new prompt alone is insufficient. Existing tier/security routing remains controlling.
An unsupported profile must be explicit; never turn a missing result into approval.
Initial integration is opt-in advisory: no new mandatory approval or fail-closed review
policy is introduced without a separate calibrated policy decision.

## Calibration and cost

Proposed adopter pilot: 30–50 historical PRs, known defects plus clean controls and
legitimate architectural deviations. Freeze an eligible population and selection seed;
stratify performance/architecture, size and local/non-local defects. Split development
and held-out cases by defect family. Review the original base/head without later fixes,
issue diagnoses, review comments or answer labels. An evaluator can use those separately
for adjudication. Protect non-public source and permitted provider/data scope.

Compare frozen correctness/security baseline against baseline plus each new profile
and the combined profiles on identical packets and equal declared effort caps. Retain
baseline other lanes when applicable. Seal outputs, randomize condition-hidden adjudication
where practical, and use a non-implementing evaluator. Human disagreements and incomplete
labels stay visible. Address rate is a burden/usefulness proxy, not proof of correctness.
Tune only on development cases; after holdout tuning a new holdout is required.

Before runs, record numeric per-case and total tokens/spend/CPU/wall/forge-call caps,
minimum positive/negative cases per stratum, practical improvement thresholds, acceptable
false-positive burden and uncertainty method, stop rules and the decision owner. These
are unresolved pilot-specific choices; neither this draft nor external benchmark scores
supply universal promotion thresholds. Missing consent, budgets or sample support holds
the relevant evaluation or yields inconclusive, never a retrospective favourable threshold.

Report with counts, denominators, intervals and coverage by profile/condition:

- Validated incremental important findings beyond baseline; precision/false positives,
  known-corpus recall, missed known cases, duplicates/unique findings and severity.
  Corpus recall is not a claim to know all defects in live changes.
- Additional findings actually repaired and verified, realized efficiency/directional
  improvement preserving guardrails, reversals, repair rounds and worker rework.
- Runs/model requests, input/cache-read/cache-write/output tokens where available,
  CPU, tool/benchmark/reviewer wall time, queue time, memory and forge/GH calls; retries,
  failed/aborted runs, packet preparation, collection and adjudication overhead included.
- Mean and P50/P95 distributions over time/cohort with sample counts and quantile method;
  never average daily P95s. Wall intervals may overlap; do not sum them into lead time.
  CPU, elapsed time and waiting are distinct; provider-unavailable CPU stays unknown.
- Human hands-on minutes versus elapsed decision latency; attributable/shared/unattributed
  overhead; estimated money with currency/pricing revision and missing usage categories.
- Cost per validated incremental important finding, total delivery cost and lead time per
  independently verified brief, paired with preserved correctness/security outcomes.
  Zero findings means an undefined cost-per-finding ratio, not zero cost or infinity as data.

After offline evaluation, a separately authorized bounded live shadow can assess actual
noise and burden. Keep model/policy/workload versions fixed within cohorts; report a version
change as a new cohort. Repeat calibration after material model/prompt/router changes.
Promotion, scope expansion, blocking status and cadence activation are separate decisions.

## Future architecture and repair loop

Map to [the target architecture](../../proposals/non-ci-architecture/11-target-architecture.md),
[context substrate](../../proposals/non-ci-architecture/17-context-substrate.md),
[progressive adoption](../../proposals/non-ci-architecture/19-progressive-adoption-and-oversight.md)
and [forge/source queries](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md):

| Responsibility | Existing/future owner | Boundary |
|---|---|---|
| Profile/architecture/workload definitions | Reviewed Git definitions/planning and versioned quality definitions | No profile grants itself policy authority |
| Routing and eligibility | Existing pure review/policy evaluation | Profile application is explicit and explainable |
| Attempts, budgets, cadence and recovery | Current supervised execution; future single cell controller/runtime coordination | No profile-specific scheduler, journal or budget store |
| Controlled runs | Isolated workers; role-bound effect executors for permitted publication | Read-only reviewer has no candidate execution or publication credential |
| Findings and acceptance | Existing evidence/decision and review owners | Exact candidate and contract; independent acceptance preserved |
| Operational assessment | Continuing operations/quality practices | May recommend work, never redefine acceptance from optional telemetry |
| Performance history | Local-first optional performance-collection, separate performance database later | Not cell coordination or canonical evidence; consent and coverage preserved |
| Display | Existing projections/CLI/cockpit | Direction/efficiency recommendations confer no action authority |

Local replay, manual assessment and imported evidence work before managed cells, the
collector or the graph context service. Future source packets use authorized exact-SHA
Diff/FileAt caching and context receipts; mutable check/review state has explicit freshness.
No new general graph service or alternate source of architectural decisions is needed.

Later cadence: observed opportunity → independent confirmation/applicability → deduplicated
issue → admitted, bounded repair attempt → draft PR → existing correctness/security plus
applicable new assessment → independent verification → exposure-window reassessment.
Reuse stable occurrence/effect identities, ownership/claims, cumulative budgets and restart
recovery. Stop on stale evidence, recurring same-class failure, exhausted budget or lost
authorization; escalate through existing intake. No autonomous merge, release, deployment,
policy promotion or architecture-target rewrite. A negative outcome may close an experiment
as disconfirmed; it does not authorize another unbounded attempt. Promote repeatedly useful
checks into deterministic fitness functions after review.

## Delivery and source evidence

[Briefs and dependencies](README.md). Existing collection adapters and continuing-operations
outlines are amended to consume these obligations, not duplicate the review implementation.
No production measurements or performance savings are claimed by these planning changes.

Primary research motivates selective verification and local calibration, not universal ROI:
[Uber uReview](https://www.uber.com/us/en/blog/ureview/),
[ReviewBench](https://github.blog/ai-and-ml/reviewbench-an-open-benchmark-for-ai-code-review/),
[CRBugBench](https://arxiv.org/abs/2610.13668),
[benchstat methodology](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat),
[ArchUnit constraints and freezing](https://www.archunit.org/userguide/html/000_Index.html),
and [Meta's operational performance work](https://engineering.fb.com/2026/04/16/developer-tools/capacity-efficiency-at-meta-how-unified-ai-agents-optimize-performance-at-hyperscale/).
Their reported results are source-specific; none establishes Assay's incremental benefit.
