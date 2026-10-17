# Desk improvement: measured outcomes and bounded learning

**Status:** draft — 2026-10-17. Proposed capability and implementation plan; no activation.
**Owner:** continuing operations and quality owns assessment meaning; existing work, runtime,
evidence/decision and projection owners retain their authority.
**Routes-to:** [public implementation briefs](README.md). Source baseline: public main
`2f56cf2892afb4ce78aab1636344aa400bf3e93c`, inspected 2026-10-17.
**Provenance:** the request to make desk improvement a reusable product capability, and
[Anthropic's review-process article](https://claude.dev/blog/solving-code-review-at-anthropic/),
published 2026-10-15 and read 2026-10-17. Vendor results do not establish adopter benefit.

## 1. Outcome and ownership

Make a desk process an enduring operational subject that can be measured, diagnosed,
experimented on and assessed after a change. Reuse [continuing operations](../continuing-operations/spec.md)
subject identity and impact/assessment lineage. A desk profile is not a new task lifecycle.
The five reference profiles below cover the supplied desk roles; role bindings and additional
profiles are configurable. The implementation must not assume every deployment has five desks.

Each cycle produces a scorecard with coverage, a supported diagnosis, a bounded experiment,
an independent assessment, an authorized decision and a later keep/revert/hold judgment.
Rejecting a change or finding insufficient evidence is a valid result. Quality, total effort
and human burden matter together. Do not rank individuals or treat more PRs as useful output.

This is an assessment capability over existing facts. It adds no scheduler, authority to
merge or adopt, competing work ID, credential broker or second history store. Pure tools
remain usable with operator-supplied offline inputs. Actual collection, provider use,
scheduling and policy adoption each retain their applicable operator/maintainer authority.

## 2. Existing components and boundaries

| Meaning | Owner to extend | Required boundary |
|---|---|---|
| Work, attempts and execution records | graph-execution/06 and /09; canonical brief-flow schema | Preserve original work UUID, invocation, attempt, contract and exact candidate; PRs are contributions |
| Assessment, defects and attribution | brief-quality/05 and /07; qualgen telemetry/attribution | Independent labels, introduced versus caught stage, failures and unknowns; do not fork their event lifecycle |
| Process invocation and observed usage | loopadmin runner, graph-execution/26 | Native lifecycle/usage with completeness; process success is not accepted delivery |
| Workflow decisions and stage outcomes | graph-execution/19 and /21 | Durable subject-bound decisions/attempts/effects remain in their original owners |
| Standing desk decisions/outcomes | graph-execution/27 and existing role receipts | One invocation can handle many items or none; require explicit associations, never infer work from a directory |
| Assessment reduction | NEW exported qualgen/deskquality package | Pure normalized-input validation/reduction, versioned artifacts; no authentication, network or execution imports |
| Analysis, experiments and follow-up | Existing continuing-operations assessment lineage, extended by this stream | Protocol/results are interpretations of frozen evidence, not acceptance or a new work queue |
| Proposal and adoption view | Existing tools/desk/internal/improve | Read-only; preserve human commit route and one adoption per cadence |
| UI/API and retained history | Existing projection/API and coordination owners | Same contracts for clients, authorized drill-down; no new reporting database or private dependency |

The [future architecture](../../proposals/non-ci-architecture/11-target-architecture.md) and
its [storage amendment](../../proposals/non-ci-architecture/14-storage-forge-graph.md) control
service-mode storage: runtime attempts/journal facts use the existing PostgreSQL coordination
owner. Offline exports and synthetic fixtures are portable inputs, not fallback execution
journals. A collector failure may leave a measurement gap; it cannot repeat an external effect
or reinterpret an unknown launch. Existing native durability/fencing rules remain binding.

Code inspection establishes a gap, not a blank slate: loopadmin/runner/usage.go already
preserves unknown token/cost fields; runner results bind caller/invocation/generation, but
these alone cannot establish five-role outcomes or active human time. qualgen/telemetry is
PR-keyed. Supervisor/workflow/standing bindings remain partly planned. The runtime briefs
below extend those seams rather than claiming source schemas prove deployed emission.

The exported assessment package must build for independent consumers. Improve is internal
to tools/desk and cannot be imported from another module. Use a qualified public module
version or validated typed artifact adapter; never ship a sibling filesystem replacement as
proof of compatibility. Public source, fixtures and examples contain synthetic data only.

## 3. Evidence and scorecard contract

### 3.1 Cohorts and attribution

System throughput uses the existing stable brief UUID and independently accepted outcome.
Desk diagnostics have their own natural admission unit: intake item, admitted worker work,
reviewed exact candidate, verification subject, or coordinator decision opportunity. Keep
these units explicit; never sum their denominators or substitute PR count for accepted work.
Issue-only work without a UUID stays a separately identified stratum under existing identity
rules; this stream does not mint a second work ID. Retain no-PR, failed, abandoned, held and
unresolved work. Attempts and repairs attach to the original admitted unit.

Freeze admission interval [start,end), as-of receipt time, eligible population, inclusion
rules, repository/cell scope, work-purpose and size/risk strata, mode and configuration
revisions. Follow the same cohort to its declared observation horizon. Open work remains
right-censored, not failed or silently dropped; historical work without a known start is
left-censored. Completion-cohort latency may be shown separately with its selection bias.
Use explicit UTC boundaries; any local label includes its zone and exact UTC conversion.

A defect's introducing stage and each stage with a documented opportunity to catch it are
separate judgments. One stable defect may have several catch opportunities but is counted
once in the system defect total. A root-cause class is not an extra incident. Findings are
suspicions until independently dispositioned; a model's agreement or an agent's repair is
not truth. Frozen requirement changes are distinguished from pre-existing omissions.
Use existing attribution vocabulary; disputes and unresolved labels remain visible.

### 3.2 Required output for every metric

| Field | Required interpretation |
|---|---|
| Identity | Definition/profile version, desk role, metric ID, unit and desirable direction; no combined desk score |
| Population | Admission and follow-up windows, eligible count E, observed count O, excluded count/reasons, unresolved/censored count |
| Value | Numerator N and denominator D with exact eligibility rule; quantiles include sample size and method; zero denominator is unknown |
| Coverage | Observed-zero, measured value, or could-not-measure with reason; source permissions/completeness, age and missing strata |
| Provenance | Restricted input digests, source revisions and evidence pointers; tool/profile/model/skill/runner identities where observed |
| Uncertainty | Counts and sampling method; uncertainty method where justified; no small-sample precision or causal claim from a trend |
| Comparison | Like-for-like baseline, cohort mix and configuration changes; observed association distinct from experimental result |
| Decision | Proposed next question/action and accountable owner, or explicit no-action/insufficient-evidence |

E/O/D are distinct: O/E measures observation coverage, and D is the subset qualified for a
particular rate. A rate uses adjudicated, eligible observations; unadjudicated records are
reported alongside it, not counted as negatives. Complete sampling metadata is required
before generalizing a sample rate. Unknown human minutes or provider spend cannot become 0.
Tokens, currency, subscription quota, local CPU, wall time and human effort remain separate
units; any conversion names its source/version. Partial observed cost is a lower bound.

Human active effort needs explicit measured spans/receipts. Waiting on a human, comment count,
interruption count and approvals are separate burden proxies; none estimates human minutes.
Cost per accepted brief includes attributable authoring, implementation, review, verification,
repair, failure and abandonment from the fixed cohort. If acceptance count is zero, show cost
and count separately, not a finite per-outcome ratio. Do not allocate shared cost to every unit.

### 3.3 Storage, correction and publication

Consume existing event IDs, run/attempt records, witnesses and rulings. New finding dispositions,
analysis, protocol, assessment and follow-up records reference those IDs and their revision;
they are interpretations, not new execution events. Duplicate ID/digest is idempotent;
conflicting content is quarantined; corrections/supersession preserve the previous report
identity and recompute affected cohorts. Deletion/permission withdrawal invalidates dependent
claims rather than leaving a reassuring stale number.

Raw human-act/source receipts remain in operator-authorized evidence storage.
No prompts, transcripts, credentials, identities, act IDs, input manifests or source pointers
enter committed scorecards through this feature. The committed projection inherits the
publication allowlist: approved
scope keys, windows, versions, numeric aggregates and coverage summaries only. Evidence
links/drill-down live in the authorized restricted report. Repository read access is not
publication permission. Missing publication authorization holds the affected projection.
No new retention period is invented: the concrete scope names its policy and deletion rules.

The pure reducer is deterministic and offline. A proposed `qualgen desks` CLI emits JSON and
Markdown from the same result and never follows remote links. Preserve existing CLI exit
codes: 0 means a valid report was emitted, including explicit measurement gaps; 1 IO/internal
failure; 2 invalid/unsupported input. JSON report completeness is separate from process exit.
Invalid/conflicting input does not replace a previous complete report. A missing/stale source
may produce a partial report, but never silently suppress a desk or reuse a stale card as fresh.

The proposed CLI interfaces are `qualgen desks --input FILE --output DIR --as-of RFC3339`
and `qualgen desks validate --kind KIND --input FILE`, where KIND is analysis, corpus,
experiment or followup. Validation emits JSON with valid, readiness (qualified/held/partial),
reasons and exact subject/input/version bindings. Qualified is structural readiness; it is
not permission, independent truth or an adoption grant. CLI commands remain planned until
their implementation and consumer version are qualified.

## 4. Five reference desk scorecards

These profiles define minimum questions, not quotas or adoption thresholds. Brief 01 freezes
machine-readable definitions, source mappings and test vectors. Sample judgment outcomes are
kept separate from the full eligible-population counts. Each card includes its leading
coverage gap and at most one prioritized improvement question, with restricted drill-down.

### 4.1 Intake

Responsibility: turn eligible inbound into a justified tracked disposition with enough intent
and dependencies for its next owner. Denominator: distinct eligible inbound admitted during
the window; show unresolved age, not only disposed items.

| Metric | Numerator / denominator or distribution | Interpretation and guardrail |
|---|---|---|
| Disposition validity | Independently upheld sampled dispositions / adjudicated sampled dispositions | Split spec/brief, bug, finding, decision and rejected/watching; agreement alone is insufficient |
| Late readiness gaps | Dispatched work with a confirmed pre-existing intake omission / observed dispatched work traceable to intake | Separate authoring omissions and later requirements; show unlinked downstream work |
| Duplicates and bad rejection | Confirmed avoidable duplicate work and overturned rejections / adjudicated eligible cases in each class | Retain rejection controls; faster triage cannot hide lost valid work |
| Flow and burden | Admission→disposition p50/p90, age of unresolved items, measured clarification effort and proxy counts | Include waiting causes; no inference of effort from elapsed time |

Candidate experiment: compare structured disposition packets with the existing procedure on
frozen historical inbound, blinded to later disposition; assess validity and missing facts.

### 4.2 Worker

Responsibility: deliver the admitted acceptance contract with reproducible implementation
evidence. Denominator: admitted original work units; failed/no-PR attempts stay in the cohort.

| Metric | Definition | Interpretation and guardrail |
|---|---|---|
| Accepted delivery | Independently accepted units / admitted units with adequate follow-up; unresolved shown separately | Report frozen-cohort counts; merged is not verified, and partial PRs are not whole units |
| Implementation defects | Units with an adjudicated implementation error / observed eligible units | By severity and detection stage; do not blame worker for changed requirements |
| Repair and abandonment | Attempts and avoidable repair per original unit; confirmed terminal abandonments / admitted units | Planned investigation outcomes are not automatically failure |
| Time, cost and human help | Admission→accepted latency, measured active/queue intervals, all-attempt cost and interventions | Incomplete cost is a lower bound; count unsuccessful attempts and recovery |

Candidate experiment: alternative context packets with task/model/checks held constant;
quality and all-attempt cost are assessed independently.

### 4.3 PR review

Responsibility: detect material problems on the exact candidate and converge without harmful
repairs. Denominators: reviewed candidates for workload, defect cases for recall, adjudicated
findings for precision; these are distinct series.

| Metric | Definition | Interpretation and guardrail |
|---|---|---|
| Confirmed finding precision | Valid actionable findings / adjudicated findings | Classify wrong, intentional, immaterial, pre-existing and harmful-to-fix; unknowns retained |
| Confirmed bug recall | Known in-scope defects detected / independently labelled defect cases | Requires frozen historical oracle; sampled rereview agreement is not recall |
| Harmful suggestions | Suggestions independently shown to induce a regression / adjudicated suggestions; also clean cases affected / clean cases | Merely fixing a suggestion does not validate it |
| Convergence | Rounds-to-pass p50/p90, held/abandoned PRs, repeated findings and late findings in unchanged code | Deduplicate stable IDs; log class-level repairs; never reduce rounds by skipping required checks |
| Escapes, cost and burden | Confirmed missed defects with review opportunity, repair effort, cost and human arbitration | Observation horizon and severity matter; zero reverts alone is insufficient |

Reference experiment: eligible delta re-review versus full re-review, preserving the first full
review, security obligations and exact-head validity. Detailed protocol is in §6.

### 4.4 Verify

Responsibility: independently establish the merged subject meets the frozen acceptance
contract; not just produce an Evidence row. Denominator: exact subject/contract verification
episodes, linked to original brief UUID; reruns are attempts, not more delivered work.

| Metric | Definition | Interpretation and guardrail |
|---|---|---|
| Incorrect pass / incorrect failure | Independently adjudicated erroneous decisions / adjudicated decisions of each eligible class | Keep these separate; report unjudged decisions |
| Discriminating checks | In-scope planted wrong implementations rejected / planted failure cases | Synthetic detection is conformance, not real-world recall; never plant into production |
| Reproducibility | Same pinned subject/inputs/conditions reproducing the verdict / attempted independent reproductions | Record environment variance and stale evidence, not just rerun count |
| Flow and burden | Merge eligibility→qualified verification, reruns, environment holds and human recovery | Missing prerequisites are could-not-check, not a pass or a failed product |

Candidate experiment: independently derived/challenged checks versus existing acceptance
checks on frozen correct and plausibly wrong subjects, without weakening actual acceptance.

### 4.5 Coordinator

Responsibility: preserve priority and authority, unblock eligible work, and finish improvement
cycles without turning the driver into a relay. Denominator: retained scheduling/decision
opportunities under a versioned policy, with arrival/capacity/dependency snapshots.

| Metric | Definition | Interpretation and guardrail |
|---|---|---|
| Priority fidelity / starvation | Adjudicated avoidable priority inversions and eligible idle intervals / observed eligible decision opportunities | Requires what was knowable then; no hindsight blame for later arrivals |
| Escalation quality | Necessary and avoidable escalations / independently judged escalations | A proper human gate is not avoidable friction; missing a required escalation is a separate defect |
| Handoffs and decisions | Handoff delay, unresolved-decision age, avoidable relay/interruption proxies and measured human effort | Unknown arrivals/capacity mean unknown scheduling quality |
| Improvement completion | Due cycles with assessed result, decision and follow-up / cycles due under the declared cadence | No reward for experiment count or positive results; deferred and inconclusive remain visible |
| Shared delivery | Accepted outcomes, defects and total cohort cost from the system view | Local utilization or PR count cannot stand in for useful output |

Candidate experiment: replay alternate dispatch decisions on identical arrival/capacity/
dependency snapshots; then a separately scoped prospective test. Replay estimates policy
behavior under retained facts, not live throughput improvement.

## 5. Shared system view and diagnosis loop

Show unique accepted original work, confirmed escaped defects by severity/opportunity/horizon,
all-attempt cost by unit, end-to-end latency, active human effort or its explicit absence,
and backlog/coverage. Segment modes, work size/risk/purpose and versions. Do not average desk
rates or publish a universal composite score. A rise in throughput accompanied by poorer
quality or missing coverage is a question, never automatically an improvement.

Proposed operating cadence: daily evidence refresh/coverage check, weekly diagnosis of the
largest supported problem, assessment at experiment completion, and follow-up at the
protocol's declared horizon. These are operating requirements for an approved pilot, not
a scheduler created by this draft. Reuse the deployment's existing operational reporting and Improve once authorized. A manual
cycle proves the workflow first; schedule changes are separate reviewed deployment work.

The coordinator owns cycle completion; each desk explains its process evidence. An independent
review/verify actor assesses labels, experiments and coordinator changes. Self-assessment
can nominate a problem but cannot settle its own benefit claim. No mandatory new role.

A diagnosis record must contain: scorecard revision/window, observation and coverage,
materiality, at least two plausible explanations (including workload/measurement change
where relevant), evidence for and against each, the next discriminating observation,
which decision it could change, proposed mechanism owner, and no-change/removal alternatives.
If evidence cannot separate explanations, choose investigate or insufficient-evidence.
A graph alone is not an analysis; correlation is not a root-cause finding.

Every cycle ends in one explicit disposition: no-action, investigate, propose-experiment,
reject, inconclusive, propose-adoption, or follow-up keep/revert/hold. These are analysis
outcomes, not a new brief/task lifecycle. Follow existing tracker/brief/Improve records;
no parallel work queue. Repeated defects may nominate a build-less-brittle investigation;
reuse its design-fit/retirement process rather than adding a default guard or rule.

## 6. Experiment protocol and reference comparison

Before any run, pin the hypothesis, competing explanation, decision to change, treatment and
control versions, eligible population, exclusions, assignment unit, frozen inputs, primary
outcome, quality safeguards, analysis method, numeric benefit threshold and acceptable quality
loss, sample-size/uncertainty rationale, full budgets, stopping rules, independent adjudicator,
source/provider permissions, retention and observation horizon. Missing values hold actual
model evaluation; they do not block synthetic preparation. Thresholds are justified before
outcomes are inspected; this spec invents no universal sample count or acceptable bug loss.

The reference comparison asks whether delta re-review reduces total review+repair effort on
eligible successive candidates without materially lowering detection or increasing harmful
fixes. Use the same initial full review, immutable commit sequence, model/tool/skill profile,
context access and mandatory security checks. Only re-review scope differs. Include cases
with a new bug, an unchanged old bug, a changed dependency/interface, an already resolved
finding, and a clean repair. Cases unsuitable for delta review must be classified before
assignment; compare coverage separately instead of quietly excluding failed treatment runs.

Use paired offline case runs with order randomized and isolated contexts. Keep original
base/head/contract context available at each decision point; later fixes, labels and other
condition outputs stay outside reviewer inputs. Split development/holdout by defect family;
freeze before tuning, record seed and protocol hash, and keep evaluation labels restricted.
Adjudicators judge findings without seeing condition labels where feasible; record limits.
A fixed historical commit sequence evaluates detection/cost under replay, not live convergence
or counterfactual repair behavior. Live rounds-to-pass needs a later prospective scope.

Every attempt, timeout, partial output, retry and interruption remains in the assigned arm.
Unknown interrupted usage is not free. Analyze by assignment as well as completed pairs,
report missingness/deviations, and never rerun only failed arms to manufacture a balanced
result. Stop for permission loss, invalid isolation, exhausted budget, unsafe behavior or the
protocol's severity trigger; retain the reason and invalidate affected comparisons. Any
post-outcome protocol amendment requires a new version and clearly exploratory result.

Results distinguish supported benefit, adverse result, insufficient evidence and invalid run.
Independent assessment checks original inputs, labels, all attempts, uncertainty and scope.
Recommend adopt/revise/reject/inconclusive; no result changes production policy automatically.
A scoped prospective advisory trial, if needed, consumes the existing approved trial owner and
requires separate source, provider, eligibility, spend, wall, expiry and rollback approval.

## 7. Adoption, accountability and follow-up

Analysis may recommend removing a rule, simplifying tools, correcting an upstream contract,
changing review depth, or keeping the current process. Require evidence about quality and
human effort, including adverse and inconclusive outcomes. A new control is not the default.

The Improve projection stays read-only and preserves its human-commit adoption route and
one-adoption-per-cadence invariant. It links the protocol, assessment and permitted summary;
it cannot write a ruling, run an experiment, edit skills, deploy, change merge policy or count
an agent recommendation as human assent. The adopted configuration/version, human authority,
applicable scope, release pins, rollback and review date must be recorded in the existing
adoption record. Any unresolved security or human gate remains binding.

Follow-up compares the original declared outcomes and quality safeguards at its predeclared
horizon. Record exposure (eligible runs actually using the version), workload changes,
missing coverage, regressions and operator recovery. Delivery, adoption exposure and realized
benefit are separate. Return keep/revert/hold with reasons; policy reversal uses existing
human authority. For reject/inconclusive, follow up the recorded hold/no-change decision and
next evidence need instead of inventing exposure or a successful adoption.

## 8. Runtime evidence responsibilities

A record schema is not evidence capture. Producers must emit facts at their existing source
boundaries; exports and the reducer may validate/join them but cannot reconstruct missing
semantics. The binding manifest introduced by 05 maps existing record fields and source IDs
to assessment inputs; it is not another envelope or ledger of work state.

| Producer | Facts required for its supported mode | New brief / prerequisite |
|---|---|---|
| Shared runner/supervisor | caller, mode, invocation/generation, request/profile digests, requested/resolved/observed model, lifecycle, time precision, usage and completeness, stop/recovery; source watermark/gaps | desk-improvement/06 after /05 and graph-execution/26 + /22 |
| Workflow controller | work/instance/node/attempt, frozen intent/candidate, admission reason, waits, cancellation, effect/acceptance receipts, findings/dispositions and all terminal/nonterminal outcomes | desk-improvement/07 after /06 and graph-execution/21 |
| Standing desk client | role/subject and invocation associations, source decision/receipt, per-item attempt, disposition, review/verification subject and result, coordinator opportunity facts | desk-improvement/08 after /06 and graph-execution/27 + brief-quality/07 |
| Human-action source | authorized distinct act identity, scope, occurrence precision and explicit measured spans where available | Existing event/decision owner; /05 binds the permitted fields, not a transcript miner |
| Offline adapter | native-to-normalized mapping, source/reducer versions, input digests, censoring and missingness | /02 and /03; absent runtime sources remain explicit gaps |

Record observations after native operations, referencing authoritative receipts. Retry of an
export deduplicates by native fact ID; same ID/different content is a conflict. Do not claim
an atomic transaction across journals without an actual one. Use existing transaction/outbox
or reconciliation as its owner specifies. Capture/export failure does not undo an operation,
create another attempt or relax admission. Existing authority-critical journal failure still
holds execution according to its own contract.

A process interval can contain multiple work units. Allocation must be explicit, sums must
reconcile, and unallocated usage remains separate rather than copied to every item. A terminal
process result cannot imply a review disposition, intake exit or verified brief. A cancelled
provider with unknown usage/stop state stays unknown. Requested and observed models remain
separate; active human minutes are unavailable without a qualified direct source.

The reference conformance corpus covers no-PR failure, mid-run crash/restart, unknown launch,
duplicate receipt, late correction, held human wait, changed acceptance, clean repair,
unsupported provider telemetry and two work units inside one desk invocation. Produce the
native records through actual producer code with disposable/fake external boundaries; a
handwritten normalized JSON file only proves reducer behavior. Mode qualification binds the
producer/build/profile/version and exact permitted output scope.

## 9. Future state and staged adoption

This adds an operational feedback cycle to the future architecture's existing continuing-
operations and portfolio domains: measure → diagnose → experiment → independently assess →
authorized decision → observed follow-up. The coordinator profile owns cycle completion;
independent assessors challenge its own proposals. A changed metric cannot rewrite a product
target, dispatch work, change a risk gate or adopt policy.

| Stage | Capability / completion condition |
|---|---|
| Offline assessment | /01–03 after continuing-operations/01–02 contracts, with permitted retained inputs; useful partial cards do not require the runtime or full cockpit |
| Operating procedure | /04 supplies configurable role profiles, diagnosis and experimental discipline; invocation is manual until a deployment separately authorizes cadence |
| Qualified native evidence | /05–08 implement and test actual process, workflow and standing desk producers against the existing record owners |
| Read-only improvement view | /09 consumes the same versioned evidence/protocol/result/follow-up; existing human route stays authoritative |
| Mode qualification | /10 proves native source→card→assessment→view and replay/rollback, including gaps; adopter enrollment and a real trial remain separate |

A deployment can use offline assistance before adopting managed runtime components. Native
measurement claims require the corresponding producer closure; a partial offline card never
certifies runtime emission or end-to-end process quality. Service-mode storage and migration
follow the already selected target; no second database or all-role process is introduced.

The [architecture change model](../../proposals/non-ci-architecture/12-future-state-model.md)
distinguishes observed, approved target, transition and proposal. This amendment remains a
proposal and its briefs parked until reviewed. It does not alter historical approvals or
silently enlarge earlier ratification. Current code, released tools and enrolled deployments
are three separate observations. No future-state claim is marked implemented by this design.

Generic plans have no dependency on a particular adopter's corpus, credentials or trial.
Adopters depend on versioned public outputs through qualified build-dependency gates. They
keep their source inventory, labels, budgets, permission choices, trial results and adoption
acts in their own tracking boundary. Public plans use no private source locators or issue IDs.

## 10. Quality requirements and proof

| Failure scenario | Acceptance / verifier |
|---|---|
| Duplicate, late correction or deleted evidence | Native ID replay counts once; conflicts quarantine; retraction withdraws affected claims; /02 and /05–08 independently tested |
| Missing labels, zero denominator or partial usage | Per-metric eligible/observed/qualified counts and unknown reasons; no favorable zero; /01–03 |
| Forbidden publication fields | Explicit output allowlist independently rejects raw act IDs, identities and source receipts; /01–03 and /10 |
| Faster review misses defects or induces repairs | Frozen defect/clean-control labels, quality margins, all attempts and independent assessment; /03 |
| Interrupted run or missing usage | Preserve assignment and partial attempt; no selective reruns or free timeout; /03 and native producers |
| Unsupported/mixed schema version | Compatible adapter or explicit unsupported input; independent module build, /01–02 and /09 |
| Runtime output exists only in a fixture | Native producer flow and recovery tests fail without emitter; /06–08 and /10 |
| A favorable result bypasses adoption | Existing read-only/cadence/authority tests and explicit follow-up; /09–10 |
| Record count scales | One indexed normalization/reduction pass plus deterministic sorting; benchmark 10k/100k synthetic records, build/hardware/results recorded in /02 |

Numerical runtime, storage, cadence and experiment budgets must be chosen from a deployment's
actual workload before activation. Benchmark results inform the choice; no universal SLO or
acceptable defect-loss threshold is invented here. A protocol's sample size, practical benefit,
quality margins and uncertainty method are fixed before results. Missing binding limits hold
the applicable evaluation, not development of synthetic/offline conformance.

Independent review must examine wrong-but-plausible claims, including a finished-only cohort
that seems cheaper, model agreement mislabelled bug truth, a runtime success mislabelled
acceptance, and a deployment claim derived only from a schema test. Definitions/labels and
causal benefit require independent judgment; keyword/presence checks cannot establish them.

## 11. Decisions, compatibility and retirement

Confirm the exported qualgen package and consumer version boundary in /01 before dependent
implementation; its pure artifact adapter is valid if a compatible library release is not
available. Existing record owners resolve required additions in /05, never fork their IDs.
A new profile, algorithm, model or measurement definition starts a versioned comparison;
original results remain reproducible under their versions.

Adopters name data/provider scope, retention/audience, independent evaluator, observation
horizon, budget and adoption authority. Shipped role-procedure changes receive the applicable
maintainer review; no actor may self-approve its own independent judgment. An unapproved
experiment cannot relax security or human gates. Negative/inconclusive outcomes are retained.

Rollback restores the previous policy/reader without deleting prior assessments or results.
Keep portable evidence until its authorized retention/deletion policy requires otherwise.
Retire duplicate summaries only after their last consumer and replacement parity are proved;
no blanket rule, daemon or second event miner is justified by this proposal.

## 12. Acceptance trace

| Brief | Independent acceptance |
|---|---|
| 01 | Configurable profiles, definitions, source/visibility contract and module boundary with discriminating vectors |
| 02 | Hand-calculated cohort/coverage/cost arithmetic and deterministic replay across five reference profiles |
| 03 | Frozen original-input protocol, independent labels, all assignments, assessment and follow-up validation, including valid rejection/inconclusive outcomes |
| 04 | Roles can identify their evidence obligations and complete diagnosis/decision/follow-up without policy self-adoption |
| 05 | Binding to canonical source records without parallel identities, mixed-version silent success or private payload widening |
| 06 | Real supervisor/adapter code emits lifecycle/usage and recovery facts with native completeness, not synthetic summaries alone |
| 07 | Workflow source joins candidate/waits/attempt/acceptance facts and preserves failed/no-PR/cancelled work |
| 08 | Standing client emits explicit per-item semantic outcomes and coordinator opportunities without guessing from process success |
| 09 | Independent consumer builds; evidence views preserve human commit/cadence and forbid effects |
| 10 | Native input→calculation→analysis→view parity plus recovery/rollback and partial-mode limits |

Implementation acceptance and a real improvement claim remain distinct. Synthetic conformance
proves boundaries/calculation, while an adopter's scoped, independently assessed experiment
and follow-up establish what its evidence supports about outcomes.
