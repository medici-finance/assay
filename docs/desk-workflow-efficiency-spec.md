# Decision-oriented desk reads and dispatch efficiency

**Status:** routed — 17 October 2026. This PR records approval and authors its citing briefs.
**Routes-to:** docs/streams/desk-tools/ (with existing measurement and service amendments).
**Approval:** approved — 17 October 2026. The maintainer confirmed that merging
[PR #3322](https://github.com/medici-finance/assay/pull/3322) was approval, and explicitly
authorized brief authoring on 17 October 2026. Approved source head:
`7504e6dbbaaba3faab6bf64b1c987fb756b4a980`; merge:
`067c7f02bd39997d719d0f64d8576efbc6f55bcc`. Approval authorizes the design and its
decomposition; caller switches, retirement and collection retain their delivery gates.
**Requested amendments:** §2.1 and A11–A12 are outside that earlier approval.
**Approved delivery routing:** existing desk-tools, desktools-v2, forge-neutral and
performance-collection owners, with adopter cutover and retirement owners. The
[change map](desk-workflow-efficiency-change-map.md) distinguishes reuse, amendments and gaps.
**Source pin:** public Assay `fed652fcd0094e42c52d6586ced409623b9a3b15` (17 October 2026).

## 1. Outcome and scope

The review desk must obtain the information needed for one complete review cycle without
joining independent full-board reads to recover omitted fields. Shared observation and
evaluation must also reduce repeated work in the worker, intake, coordinator and reporting
paths. Dispatch must reuse applicable preparation facts rather than collect them again for
its reading packet. Measure the complete supported workflow before and after each change.

Optimize the vector **CPU service time, elapsed wall time, provider requests and provider
quota cost**. Keep components separate: no monetary conversion or weighted scalar is
selected. Improve required work first: remove duplicate collection, narrow typed queries,
share evaluation, then tune bounded concurrency. Additional caching requires a measured
remaining bottleneck and explicit validity/permission rules. Preserve existing qualified
token reuse, conditional requests and immutable-fact reuse during comparisons.

Initial delivery is review-first. It covers queue reads, offline planning, review dispatch
preparation, their watchers, and shared collection seams used by other desks. Worker issue
discovery and intake escalation receive distinct projections over shared issue observations.
It does not introduce another scheduler, daemon, credential broker, store, mandatory upload,
automatic merge or authority to publish. It does not redesign every desk command at once.

## 2. Brownfield evidence and inherited obligations

At the source pin:

- `plugins/assay/skills/pr-review-desk/SKILL.md` runs `deskboard actions`, then `deskboard prs`,
  then `reviewloop plan --actions ... --prs ...`. `actionRow` omits the head SHA already in
  the collected PR; the second read supplies it. `reviewloop/plan.go` explains the join.
- `deskboard/throughput.go` calls the full actions report for review depth. `main.go` applies
  delta/quiet processing after collection. Delta output is not incremental collection.
- `deskboard/board.go` already uses `ReviewQueueSnapshot` and per-PR fact reuse. `readcost.go`
  installs conditional GETs and a per-process meter/floor. `sweep.go` bounds nested pools by
  their product, not by a cross-process installation budget.
- `issueboard/board.go:computeIssueBoard` walks repositories and issues serially; escalation
  and addressed-item classification read comment histories. Worker discovery uses this same
  broad issues path. Trust, overflow and positive closure checks have distinct purposes.
- `deskdispatch/reviewroundread.go` and `packet_review.go` independently read change/review
  facts. Packet file reads have a 60-second budget; verifier history has a ten-second budget
  and a 2,000-commit ceiling. These are existing limits, not newly selected acceptance targets.
- `deskboard` registers flags rather than subcommands with `PerfRun`; `issueboard` supplies
  no verb vocabulary; ordinary dispatches become `other`. `perf-v1` CPU is own-process and
  child/request counters cover only instrumented paths. Audit joins can help an investigation
  but are not a supported attribution contract.

Quality evidence: a Git-history inspection over 19 July–17 October 2026 UTC, through the
source pin, found 32 commits touching `deskboard/board.go` (15 subjects containing `fix` or
`revert`) and 66 touching `deskdispatch/dispatch.go` (34 such subjects). These are churn and
subject proxies, not defect counts or a brittle designation. The files contain 3,475 and
2,343 lines respectively. This nominates their collection/evaluation seams for bounded
extraction; it does not justify rewriting unrelated controls. No new complexity ranking or
controlled fleet cost study was obtained for this document.

Preserve the existing [library boundaries](library-first.md),
[future architecture](proposals/non-ci-architecture/11-target-architecture.md),
[source-query and serving boundaries](proposals/non-ci-architecture/22-forge-service-and-source-queries.md),
[regression floor](proposals/non-ci-architecture/15-regression-from-day-one.md),
[telemetry promise](telemetry.md) and
[refactor-oracle procedure](streams/build-less-brittle/spec.md). Missing data cannot become
empty, green, idle or acceptance. An observation never grants a claim or permission to write.
Keep independent review, exact-subject verdicts, ready ownership and human merge unchanged.

## 2.1 Candidate-plus-priority planner — requested amendment, 23 October 2026

This bounded planning amendment extends the routed baseline at the operator's request;
it does not extend the earlier revision-bound approval, activate a desk, or rebind an
in-flight assignment. Source inspected on App-fetched main `9c4307a19`; the call-path audit
used release v1.0.57. Recheck versions, active work and affected-context applicability at pickup.

Both a prose/skill desk and its discrete client must consume the **same role-qualified
planner implementation**. There are two explicit inputs: (1) the complete authorized
candidate observation and (2) current approved priority/selection policy, with explicit time
and capacity. Composition delegates to the existing role/domain evaluators; it is not a
universal replacement score or another scheduler. The role's pure planner returns the same
ordered proposals and held/unknown/watch dispositions for identical inputs on both paths.

- Observation includes qualified identity, brief/issue/PR aliases, exact subject revision,
  all source lanes, provenance, freshness and coverage. Union and deduplicate the relevant
  lanes before scheduling selection. A display/span cap is not a complete candidate universe;
  retain held-back rows/counts and distinguish unknown coverage from an empty queue.
- Policy input names its approving owner and revision and carries existing lane precedence,
  drive priority, scores, dependency and reservation/cap semantics. Unmapped work follows an
  explicit existing default or is surfaced; an inner join must not silently drop it. A priority
  entry with no observed candidate remains not-observed/coverage-gap. Portfolio shares remain
  advisory until their separately approved policy integration; no filler work or new floors.
- The plan binds role, observation and policy/evaluator revisions, time/capacity, order,
  selected lane/head and explanatory reasons. Review's operation-key coalescing order must not
  accidentally become reviewer-slot priority. Keep access/trust, hard holds and eligibility
  independent of score; a high rank grants no claim, write or acceptance authority.
- Planning reads no provider, filesystem, process, credential or mutable global state. Priority
  changes and slot completions rerun this function on applicable observations. Collection has
  its own due/invalidation contract; a SelectQueue/refill call does not inherently require a
  fleet refresh. Source change, access change, expiry or incomplete coverage triggers the
  existing bounded refresh/hold behavior, never an invented global TTL.
- Prose explains and executes this plan instead of hand-ranking a second queue; discrete
  clients adapt the same result to the existing Loop/reactor interfaces. Claim, admission,
  exact-head/current-state and effect rechecks remain at their existing boundaries. Coordinator
  and intake judgment remain bounded model work; review remains reactive rather than a
  drain-once queue. No seventh Loop method, new daemon, policy register or credential owner.

Current reference plans are not complete prose replacements: worker's shown board is already
capped and its orphan source can be unwired; verify also owes an Evidence-PR repair sweep;
intake events do not include all persistent issue/raw-intake debt. Migration must demonstrate
source coverage as well as identical ranking. Missing facts hold the affected scope.

Delivery reuses desk-tools/39–40 for review observation/pure planning, /42 for issue projections,
/43 for actual caller adoption and /44 for cost/parity/retirement qualification; /45 and /51 own
verification queue and skill integration. graph-execution/27 owns the existing five-role
composition/driver adapter, consuming those contracts through legal module boundaries.
statusgen/evaluation and the declared domain owners retain eligibility/coverage meanings.
The direct-reader bridge can ship before R6. Cross-window reuse belongs to forge-neutral/37–43;
desktools-v2/09's verified typed snapshots and existing batches/caches are reused, not redone.

## 3. Better review operations

Names below are proposed contracts and spellings, not available commands. The current bridge
can ship independently of the future runtime. The target follows the tiny-verb/role-executor
boundary; neither adding a command nor importing a library moves credentials into an agent.

| Logical operation | Current-path bridge | Target adapter | Responsibility |
|---|---|---|---|
| `ReviewQueue` | `deskboard review-queue --out <file>` | `desk-review-queue` | One declared-scope collection and review projection, including exact heads, ordering inputs, all dispositions and coverage. |
| `PlanReview` | `reviewloop plan --snapshot <file>` | Library call from the queue/planning adapter; no extra binary required | Pure planning over that observation and explicit policy/time/capacity inputs; no provider read or outward effect. |
| `PrepareReview` | `deskdispatch <key> --kit review ... --observation <file>` | `desk-review-prepare` through the role executor | Prepare one exact-subject review using existing claim, workspace, attestation and packet responsibilities. Revalidate consequential current inputs at their existing boundaries. |

### Queue and plan

`ReviewQueue` emits a versioned `review-observation-v1` envelope. It contains:

- Requested and admitted provider/host/repository scope, collection start/end, source
  provenance, supported schema, evaluator and policy versions, and explicit freshness and
  completeness per input group. Provider pages may be observed at different instants: this
  is an observation set, not a promise of a cross-repository atomic snapshot.
- For each change: qualified identity; full head SHA; observed base SHA when used (otherwise
  unavailable); body-edit/review/check revisions needed for applicability; action, required
  lanes, rank inputs, hold/surface reasons and source coverage. Preserve WAIT, SURFACE and
  NO-OP populations alongside actionable rows so filtering cannot manufacture an idle verdict.
- Queue depths and alarm inputs derived from the same observation, with their population and
  cutoff stated. Never substitute visible/capped rows for the uncapped eligible population.
- The cost of collection when measurement is enabled, with basis and coverage. This does not
  turn ordinary authorized output into permission to export repository details as telemetry.

The adapter freezes a validated envelope before atomic output. A partial enumeration follows
the current fail-closed command contract; an explicitly degraded fact remains visibly unknown
under the current classifier contract. A future mirror's coverage envelope must be mapped
without upgrading stale/partial data to a complete legacy board. Consumers refuse unsupported
schemas or missing mandatory head/coverage fields. Stable action/ordering semantics remain
the existing owners' meanings, not a new classifier in the skill.

`PlanReview` consumes one envelope, not independently observed actions and PR lists. It
retains `reviewloop`'s action-table exhaustiveness, exact-head coalescing and idle rules.
Changes to those meanings require their normal review; adding omitted facts does not imply
a new ready predicate. Planning uses explicit time so time-based conditions are reproducible.
An observation that aged beyond the applicable freshness contract requires a refresh before
an idle claim. That contract must be selected by the existing owner; no new universal TTL is
invented here. Conflicting mandatory fields fail visibly rather than selecting a convenient
head from another file. Legacy input forms remain supported for their compatibility window.

### Prepare one review

An observation file is untrusted advisory input, not a bearer capability. Validate schema,
scope, subject, evaluator/policy binding and applicability. The existing dispatcher still
acquires the canonical claim and validates role/model/workspace requirements. Before a
consequential operation, re-read the mutable facts required by that operation. A moved head
produces the existing conflict/refusal/replan outcome; do not relabel the assignment to the
new head while retaining old facts. Body, review, check or policy changes can invalidate
applicability without moving the head and must be covered too.

Introduce a shared `ReviewContext` library result for round preparation and packet assembly.
Reuse an already-collected fact only when its complete validity key matches. Immutable
content uses provider/host/repository identity, exact commit SHA, path and interpretation
version; comparisons also bind both endpoint SHAs. A mutable base branch name is not an
immutable comparison key. Read from the common Git store only after confirming the exact
object is available: the allocated worktree's default-branch HEAD is not the PR head.
Keep the terminal applicability recheck; deduplication cannot remove a race check.

Preserve packet caps, untrusted-content escaping, omission reporting and nonfatal optional
packet behavior. Measure reads that produce omitted/unused content. Preparation budgets
must be bounded before launching remote reads, with cancellation/deadline propagation and
explicit omissions; a check between files alone cannot stop an already-blocked request.
Judge packet value across dispatcher plus reviewer, including repeated reads after launch.
Context production gets no additional standalone verb until a real caller requires one.

## 4. One implementation, several adapters

Separate observation collection, deterministic evaluation/planning, and presentation:

| Layer | Owner and reuse boundary |
|---|---|
| Provider reads and custody | Existing typed `Forge`/`ReviewQueueSnapshot` and authenticated composition in `tools/desk/internal/deskkit`; provider queries stay behind the seam. |
| Shared observation contracts | Extend narrow `forgeread` contracts through forge-neutral/36-class ownership when an external consumer needs them. Its root remains offline/credential-free; online adapters do not enter statusgen. |
| Review evaluation and planning | Extract the existing deskboard classification and reviewloop action-table/coalescing implementations into pure packages inside the desk module first. These `internal` packages can be shared by deskboard, reviewloop and deskdispatch, but cannot be imported by statusgen or a separate module. No `cmd` imports another `cmd`. |
| Canonical domain evaluation | Keep `statusgen/evaluation`'s eligibility/coverage owner. Promotion of review predicates into a public pure API belongs to the existing desk-simplification policy/executor work; do not create a second implementation in `deskcore`, `forgeread` or the skill. |
| Effects | Existing claim, dispatch and review/ready publishers; later the qualified role-bound executor. Read reuse never inherits its capability. |
| Persistent observations | Future R6 mirror, admitted store and refresh owners under forge-neutral/37–38; later typed serving under /39–40. No command-specific daemon or second database. |
| Measurement | Existing `perfrec` writer/desk instrumentation, performance-collection contracts/adapters/reporting and operational accounting. Optional measurements do not replace audit or enforcement. |

The first extraction moves the existing implementation and its oracle together. Register
the actual semantic owner in `docs/contracts.md` when implementation lands; this document
does not create a competing ownership register. Use the existing oracle/characterization
procedure to distinguish intended behavior from accidents. Export or move a package only
when an actual module consumer requires it; qualify `GOWORK=off`, build/release closure and
external module consumption before promising an importable SDK.

The same observation may feed actions, heads, review depth and alarms within a cycle without
another provider enumeration. Direct collectors are request/cycle-scoped, with explicit
scope and applicability keys; they do not become a hidden shared daemon. Across independent
desk windows, use the qualified R6 ingestion/refresh owner when available. One consumer
outage must not start a full poller in every caller. Targeted consequential revalidation
remains separate from shared bulk observation.

For issueboard, share open-issue facts while selecting the required projection before
expensive enrichment: worker work-ready discovery, intake classification, or addressed inbox.
Each projection declares its complete population and required trust/history/closure facts.
An empty worker projection says nothing about intake backlog. Batching must preserve comment
pagination, trust identity/edit checks, addressed-response and escalation semantics, and
positive closure evidence. Unsupported provider batching takes the honest bounded fallback.

Bound provider work by the admitted provider principal/installation as well as local pools.
Do not add more pool width without testing concurrent callers. Before managed coordination
exists, retain and report local bounds and the inability to coordinate independent processes;
do not claim a fleet quota guarantee or build another coordinator for this optimization.

## 5. Measurement sufficient for the next investigation

First correct the fixed command/kit vocabulary and missing instrumentation paths in the
existing local measurement owner. Keep `perf-v1` and its opt-in/field promises compatible.
The [perf-v3 contract](performance-attribution-spec.md) now owns the requested local
attribution/request-cost extension through perfrec and desktools-v2/63–65. Richer collection
still extends performance-collection's existing aliased event contract; do not silently append
workflow identifiers to legacy records or treat an existing tool opt-in as workflow consent.
The [collection design](streams/performance-collection/README.md) owns local retention,
bounded loss, profiles, optional ingestion and upload. Database/API deployment is not a
prerequisite for a local diagnostic or a reproducible offline comparison.

Required diagnostic distinctions, where observable:

- Actual operation, dispatch kit, caller role, build/platform and workload dimensions.
  Missing dimensions have coverage, not guessed values or unbounded argv labels.
- Monotonic elapsed time for collection, evaluation, claim, workspace, context/packet,
  attestation and output. Record phase outcome and bounded reason class, including refusals,
  unknowns, interrupted runs and optional-section omissions.
- Own-process CPU separately from measured child CPU. An inclusive total requires known
  nonoverlapping process coverage; otherwise report a lower bound or unavailable value.
  Overlapping child/phase durations are not summed as elapsed workflow time.
- Actual provider attempts at the transport, endpoint class, retry/backoff, conditional
  responses, bytes and in-flight concurrency. Separate client-to-service requests from
  service-to-provider requests. A 304 remains a request even if its primary quota charge is
  avoided. Record GraphQL cost and quota resource when observed; do not call
  requests-minus-304 the universal charged-cost metric. Missing point data stays unknown.
- Optional, explicitly selected workflow-profile correlation for parent/child invocations,
  batches and launched reviewers, using local opaque aliases and the existing identity/epoch
  policy. Do not double-count a parent's aggregate and the child events it summarizes.
- Collection/telemetry overhead and coverage. Instrumentation errors, disk limits or consent
  withdrawal cannot change operational results, claims, budgets or required audit.

Use existing profiling facilities or an explicit local diagnostic run for CPU/heap stacks
when phase evidence nominates an in-process hotspot. Do not install a permanent profiler
listener or upload stacks by default. Profiles may contain sensitive data and are distinct
from fixed-vocabulary performance events.

Reports show sample counts, distributions, total wall/CPU/provider work and result classes,
split by build, workload and coverage. Pool samples/mergeable distributions, not percentiles.
Keep successful, refused and unavailable runs; avoid survivor bias. Required views include
per invocation, per complete desk cycle, per dispatch batch, and dispatcher-plus-agent cost.
Cadence reports state scope, caller count and observation window. Savings from moving work
between processes or returning less required coverage do not qualify.

## 6. Qualification and acceptance

No fleet-wide numeric SLO is inferred from a small local sample. Before implementation
comparison, freeze matched baseline/subject revisions, platform, provider capabilities,
scope/policy, existing cache state, query versions, workload and the chosen tradeoff limits.
The approving owner selects any allowed CPU/latency/quota regression before results are
scored. Until selected, unqualified cost regressions block a claim of an efficiency win.

Compare cold and steady state, sparse and skewed repositories, growing independent counts
of callers/PRs/reviews/issues/comments/files/bytes, overflowed histories, concurrent windows,
unchanged cycles, one changed input, rate limits, permission loss and provider failures.
Use disposable fake providers and pinned Git fixtures for deterministic request counts and
correctness. Actual provider cost requires an authorized observation, never a synthetic
number relabeled as measured. Synthetic scale is declared, not claimed as fleet scale.

| ID | Acceptance example and independent evidence |
|---|---|
| A1 | One review queue produces the same intended action, exact head, lanes, ordering and coalesced plan as the frozen oracle for complete inputs, without a second open-PR sweep. Include WAIT/SURFACE, unknown action and missing-head controls. |
| A2 | Partial enumeration, empty admission, access loss, incompatible schema and stale observations cannot produce a complete empty/idle result. Exercise the lower adapter as well as the CLI. |
| A3 | For an unchanged complete input, requesting actions, heads and review depth in one cycle adds no duplicate provider enumeration. Overflow fallback is separately counted. Changing the head or only body/review/check/policy inputs invalidates exactly the required facts. |
| A4 | Preparation and packet assembly share eligible reads; exact-SHA local and provider file content agree. An injected moved head or mutable-base change cannot reuse the old assignment/diff. Required rechecks still occur. |
| A5 | Worker discovery and intake/inbox projections preserve their independent populations and legacy trust/escalation/closure outcomes. A long or unavailable thread retains the correct row/sweep degradation. |
| A6 | Known synthetic phase/process/request work reconstructs without double-counting. Retries, 304s and GraphQL points remain distinct; uninstrumented descendants, killed runs and absent CPU stay visibly incomplete. |
| A7 | Consent-off, optional sink failure and malformed diagnostic input preserve operational output/effects. Secret-, repository-, title-, path- and argv-shaped data cannot escape the selected measurement profile. |
| A8 | Full matched-cycle and dispatch-plus-agent comparisons report all objective components and field coverage. A faster warm-only inner loop, dropped population or work shifted into a child fails the claimed saving. |
| A9 | Old and new supported command/skill versions pass the compatibility matrix; unsupported combinations fail clearly before effects. Reader rollback restores a qualified path without replaying submitted writes or changing claims. |
| A10 | A retirement rehearsal finds a planted caller in a skill, watcher, installer or supported pin and refuses deletion; a control cannot retire solely because its first caller migrated. |

Implementation briefs must bind these examples to real entrypoints and named non-vacuous
tests, with a planted failure relevant to each claim. Preserve existing regression/mutation
coverage and dated Evidence. This spec has not run or passed those future checks.

## 7. Caller migration and retirement

1. Inventory at implementation pickup: current skills, reviewloop, monitor scripts, inbox
   launcher, coordinator reports, other desks, tests, CI, release/install manifests, adopted
   versions and documented operator recipes. Check actual callers, not just string matches.
2. Ship shared implementation plus new schema/adapters with legacy forms intact. Shadow
   comparisons consume the same observations; do not double production provider load by
   running independent old/new collectors indefinitely. A bounded fresh-observation parity
   run is explicitly measured and authorized.
3. Release/pin the compatible producer before updating a skill or monitor to require it.
   The review skill moves from actions+prs+plan to one queue observation+pure plan, then
   passes the observation to review preparation. Watchers and coordinator review-depth
   consumers migrate too; retaining an old full sweep there would erase much of the saving.
4. Reuse the same collectors for worker/intake projections in separately reviewable changes.
   Move persistent reads to R6 only after its actual contract, scope, coverage and release
   qualification. The early direct-read lane does not wait for PostgreSQL or executor cutover.
5. Record supported old/new producer, skill, schema, provider and deployment combinations,
   rollback path and the last supported release shipping each retiring entrypoint. Read-only
   adapter migration is distinct from the fenced publisher/credential switch.
6. Remove redundant caller invocations first, duplicate implementations next, command
   subcommands after their last supported consumer, and binaries only when no surviving
   subcommand needs them. Compatibility wrappers delegate to one implementation and carry
   an owner/removal condition; they must not become parallel collectors or predicates.

The review migration alone cannot retire `deskboard`, `issueboard` or `deskdispatch`.
Other desks and reports use them. A zero-consumer record covers skills, workflows, cells,
supported adopter pins and operator recipes; an uninspected consumer remains unresolved.
Adopters retain their existing retirement and control-equivalence procedures. No local
refusal, scan, exact-head check or publication layer disappears merely to meet a cost target.

## 8. Alternatives, decisions and handoff

An additive head field alone is a useful bridge but leaves duplicated collection and missing
phase attribution. A new all-in-one privileged daemon would merge authorities and duplicate
R6/runtime work. A persistent result cache first would hide excess reads while adding
invalidation duties. Unbounded batching/concurrency could trade lower latency for higher
GraphQL or secondary-limit cost. The selected proposal is bounded library extraction and
complete decision-oriented observations, compatible with later R6 ingestion.

Approval is revision-bound and does not activate anything. The maintainer approved the
contract, logical adapter names and direct-read-first scope by merging #3322, explicitly
confirmed on 17 October 2026. Implementers must declare the actual compatibility window,
release/pin versions, selected measurement-profile fields and workload-specific cost
tradeoffs before qualification; existing privacy and effect gates still apply.

The [change map](desk-workflow-efficiency-change-map.md#authored-delivery-routing) now
routes the implementation to desk-tools/37–44 and bounded amendments of the existing
performance-collection and forge-neutral owners. The authoring PR proposes these briefs;
its merge completes routing, not implementation. Preserve completed briefs and Evidence.
The active CLI ports and pending supervision/improvement proposals have explicit pickup
checkpoints. No implementation is declared complete by these amendments.

## Proposed verification extension — 20 October 2026

[Verification planning performance and diagnostics](verifyloop-performance-spec.md) proposes
applying this observation/evaluation and matched-cost method to verification. It reuses the
existing recorder/provider adapters and separates local read reuse from R5/R6 service work.
It is a draft extension, not an enlargement of this document's revision-bound approval or
an amendment to the review-only briefs. The accompanying verifyloop diagnostic integration
is explicitly requested; behavioral optimizations retain their own approval and qualification.

## Ranked provider-call reductions — planning estimates, 23 October 2026

Rank by expected fleet request savings for repeated mostly unchanged work. These alternatives
overlap; percentages must not be added. R=repositories, P=open PRs, I=open issues,
T=selection passes, C=full discovery cost, g=required targeted rechecks, M=complete producer
reconciliations during the same interval. Count actual HTTP attempts, including 304, separately
from GraphQL points, quota, CPU and wall time. Git transport and local I/O are separate.

| Rank | Existing delivery owner and change | Before -> after requests | O() and qualification |
|---|---|---|---|
| 1 | forge-neutral/37–43: shared ingestion/mirror, one authorized scope owner | Sum of independent caller scans -> one producer refresh stream + changed facts + g | O(T*C) -> O(M*C + changed facts + g); M normally smaller than T. Producer and full repair inventory are never free; routine client reads add no GH calls. |
| 2 | graph-execution/27 with desk-tools/39–43: separate collection from refill/priority selection | T*C -> one applicable observation + invalidated fact refresh + g | In one-full-refresh window O(T*(R+P+I)) -> O(R+P+I+total changed items+g), restoring page/history factors. Mere timer/completion events do not require provider enumeration. |
| 3 | Existing typed snapshot plus graph-execution/27 worker binding: share orphan/disposition/stalled observations | PR component 6R+drafts+2*blocked-drafts+stale-drafts -> 4R+2*blocked-drafts+stale-drafts | Saves 2R+drafts in ordinary one-page conditions; usually still O(R+P). Keep commit/comment/supersession/claim guards and separate issue/recent-merged discovery. |
| 4 | desk-tools/42 and forge-neutral/37–38: projection-before-enrichment and qualified issue batching | L+U+E+H+A -> L+b(U)+b(E)+b(H)+b(A)+overflow | b(X)=sum per repo ceil(X/32), proposed batch size for comparison, not an installed contract. Fixed batching remains O(R+I+pages). Preserve complete histories/trust/closure and provider point budgets. |
| 5 | desk-tools/39–40,43: complete review observation and watcher handoff | Foreground 6R+2*changed-PRs -> 5R+2*changed-PRs; reuse a same-cutoff watcher observation instead of another foreground scan | Saves R for the head inventory; removing a coincident second classification saves its actual measured collection. Cold O(R+P) unchanged, repeated pure selection makes zero GH requests. |
| 6 | desk-tools/45,47,51: one complete multi-root verify plan and shared repair observations | Common-blocker reads sum per-root Q -> distinct Q; planner V + repair Y -> V+Y-proven overlap | Repeated common blockers O(B*Q) -> O(Q) across B separate root plans. Existing 32-target PR/receipt batches remain. Name the repair reader before assigning a numeric overlap coefficient; retain final held-head checks. |
| 7 | Existing zero-CI and exact-source owners through graph-execution/27 and forge-neutral/42 | Probe reaching both CI reads: about 4Z -> 2Z; immutable workflow reads reused by exact SHA | Fresh CI remains O(T*Z); repeated immutable workflow bytes can fall O(T*Z*W) -> O(Z*W). Share equivalent observations without caching mutable green/ready conclusions. |

L is actual listing pages; U=selected untrusted issues; E=response-clock issues; H=label-history
placeholder candidates; A=absent unfinished placeholders needing positive closure evidence.
For current REST issue enumeration, open PRs also occupy pages before filtering: ordinary
complete pages cost sum(floor((I_r+P_r)/100)+1), including the terminal exact-boundary request.
The compatibility monitor can use a different query/paging shape. Histories, snapshots hitting
caps, zero-CI probes, auth, retries and overflow fallback require separately recorded counts.

Synthetic examples, not live measurements: four repos/20 ordinary trusted PRs give review
foreground 64->60 cold requests; four repos with 20 drafts, five blocked and three stale give
worker PR component 57->29; 64 short-history issues evenly split across four repos into 32
response-clock and 32 placeholder candidates give 68->12 under the proposed batching. Four
root plans sharing ten blocker issues give 40->10 blocker reads with one multi-root plan.
A warm review/coordinator/watcher slice can fall 68->20 only when one qualified shared
classification replaces the duplicate readers and the retained producer is charged once.
None of these excludes legitimate final g or proves the benefit of an unbuilt mirror.

A11: same observation/policy/time/capacity must yield identical coverage, ordered decisions,
selected lane/head and hold reasons in prose and discrete production adapters for all five
roles. Negative controls include second-root-only work, capped/hidden candidates, unmapped
priorities, a priority change with unchanged source, key-sort versus policy order, shared
blockers, partial/denied inputs and moved heads at admission. Missing actual role adapters
is could-not-check, not parity via a mocked second planner.

A12: qualify call growth at R/P/I/T sizes 1, 4 and a larger bounded cohort under cold, warm,
changed and error states. Count provider attempts at the transport; repeated pure planning
must issue zero GH calls, collector and consequential reads remain visible, and no savings
may come from omitted sources or weaker gates. The existing /37 oracle and /44 qualification
own the review/issue cohort and common cost format. graph-execution/27 and /51 append actual
role/repair coverage without becoming prerequisites to the early review bridge. These are
added implementation criteria, not new executed Evidence or consent.

### Per-desk request accounting for the ranked changes

These are conditional successful one-page estimates, not additive fleet totals. Restore
history/file pages, zero-CI/trust probes, auth/retries and coverage caps; P means admitted
open PRs for changed-file enrichment, while I is the issue population before projections.

| Desk | Before -> after | Scaling and retained work |
|---|---|---|
| Coordinator | Independent review depth about 5R+2*changed-PRs plus issue-register reads -> zero duplicate bulk reads when an applicable already-produced scope observation exists, plus g | Role bulk O(R+P+issue pages) can move to shared producer; producer still pays once. A standalone coordinator cannot claim this saving without another required collector. |
| Intake | L+U+E+H+A -> L+b(U)+b(E)+b(H)+b(A)+overflow; separate poll remains until qualified shared ingestion | Fixed batches remain O(R+I+pages). Changed-event author/trust reads and positive closure evidence remain; no-event does not mean no backlog. |
| Worker | 6R+drafts+2*blocked-drafts+stale-drafts+issue/recent-merged discovery -> 4R+2*blocked-drafts+stale-drafts+the same other discovery | Usually O(R+P+I+recent-merged pages); saving 2R+drafts is not removal of source coverage or final claims. |
| Review | Foreground 6R+2*changed-PRs -> 5R+2*changed-PRs; compatible watcher handoff avoids a second full classification | Cold O(R+P) remains; T pure plan/refill calls add zero GH attempts. Head/body/review/check applicability still invalidates as required. |
| Verify | V+Y -> V+Y-proven overlap; per-root shared blockers sum(Q_root) -> distinct Q in one multi-root pass | V is approximately 2R_t+2b(P_e)+4b(A_receipts)+Q+F+J on the supported batch path; Y is the separate owned-Evidence repair sweep. Existing batches/cache are baseline; keep final J and name Y before claiming a coefficient. |

For verify, P_e is all listed verifier-authored PR candidates before detailed admission;
A_receipts is the distinct eagerly prepared attestation references; F is exact-head GitHub
content reads; J is proposed holds requiring final head checks. File/label overflow can
repeat the full native walk, not only its missing tail. Separate root plans can multiply
shared blocker reads O(BQ); one full-root pass keeps O(Q). Mirror adoption then removes
routine client bulk GH calls while the shared reconciliation/refresh and consequential g
remain counted at their actual owners.
