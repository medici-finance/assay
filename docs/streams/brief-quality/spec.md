# Brief quality: assessment, acceptance review, and outcomes

Version: 0.1-design · Status: proposed, not an implemented control
Home: medici-finance/assay
Provenance: operator request, 2026-09-26, to codify six task dimensions, assess authoring quality, and consider independently authored or challenged verification.
Freshness: symbols-resolved 2026-09-26 @ 7aa3835d7; freshness-checked 2026-09-26 @ 7aa3835d7.
Existing paths below were inspected at that revision. Paths marked NEW are planned outputs.

## 1. Purpose and boundary

Measure whether a brief supplies an executable, testable contract, and whether its original assessment predicts the work subsequently observed. Keep authoring quality distinct from implementation quality and from routing economics. A capable worker can repair a bad brief; a good brief can receive a bad implementation.

This is a design for extending the existing brief schema and tools, not evidence that authoring quality is already measured. MUST and SHOULD below describe the proposed implementation. No routing setting, review requirement, or fleet configuration changes when this design merges. No performance advantage for a model/effort pair is claimed.

The implementation MUST retain three separately addressable products: task assessment, dispatch policy/result, and observed outcome. No composite score averages away a capability minimum, risk answer, or human gate. Existing `domain`, `budget`, and requirement-linked `outcome` keep their meanings; observed results MUST NOT reuse `outcome` for a new meaning.

## 2. Decisions and authority

| ID | Question | Status | Recommendation / authority |
|---|---|---|---|
| D1 | Codify six task dimensions and measure outcomes / brief authoring | Requested | Operator request of 2026-09-26 authorizes this design and implementation stream, not deployment |
| D2 | Who writes the first DoD? | Proposed; human ruling required | Author writes intent, constraints, initial DoD; independent reviewer derives failure cases first, then challenges/amends DoD before dispatch |
| D3 | Which briefs require acceptance review? | Proposed; human ruling required | Opt-in pilot for all newly authored briefs in named pilot streams; no retroactive gate on existing work; enforcement only after explicit rollout ruling |
| D4 | Must the reviewer use a different model? | Proposed; human ruling required | Separate non-author run/context mandatory; model-family diversity recorded and preferred where available, not a substitute for independence |
| D5 | Retention, cost disclosure and pilot promotion | Proposed; human ruling required | Local/private structured events, no prompts or raw sessions; operator chooses retention, pilot cohorts, spend ceiling and evaluation window before collection |

A model recommendation is not a human ruling. A ruling record MUST carry decision ID, selected option, actor/authority, source locator, time and affected spec revision. Superseding rulings link the previous one. An absent answer is pending. The stream's decision brief is a typed prerequisite for policy implementation; a README sentence alone cannot enforce ordering. D1 does not imply D2–D5 were accepted.

## 3. Six dimensions

Each category requires rationale and evidence references. `unknown` is permitted only with a reason and an explicit resolution owner; it is not the lowest category. The profile is an assessment at a revision, not an intrinsic numeric score.

| Dimension | Anchors | Operational meaning |
|---|---|---|
| Work size | Existing `effort: S / M / L` | S: one bounded change; M: several related steps; L: largest coherent reviewable unit, split beyond L. Repository sizing examples refine these anchors. Not model reasoning effort. |
| Specification completeness | `ready / open-choices / missing-facts / mixed / unknown` | Ready: necessary facts and binding choices supplied. Open choices: facts available but delegated design choices remain. Missing facts: an investigation must establish named facts. Mixed: both. |
| Reasoning difficulty | `pattern / constrained-design / novel-reasoning / unknown` | Pattern: verified precedent applies. Constrained design: choose among known approaches. Novel reasoning: establish an unfamiliar mechanism or reconcile interacting constraints. |
| Coupling | `local / shared-contract / cross-boundary / unknown` | Local: isolated behavior. Shared contract: multiple consumers rely on one meaning. Cross boundary: correctness spans independently failing components/artifacts/repos. File count is not a category. |
| Verification strength | `decisive / partial / judgment / unknown` | Decisive: named critical claims have executable discriminating checks plus negative controls. Partial: important claims remain outside those checks. Judgment: acceptance rests primarily on expert/human assessment. Never a claim to prove all possible correctness. |
| Failure consequence | `contained / recoverable / consequential / unknown` | Contained: bounded local rework. Recoverable: wider disruption with demonstrated recovery. Consequential: funds, access, sensitive information, regulatory obligations, irreversible damage or comparable impact. Record affected surface and recovery evidence. |

### 3.1 Proposed representation and compatibility

Keep `effort` as the only stored size category. Add a versioned `assessment` mapping to brief-v2; its `size` child stores rationale/evidence only, referring to `effort`, never a duplicate size value. Each other dimension has `level`, `rationale`, `evidence` and, if unknown, `resolution`. The four existing `risk` answers remain the authority inputs for `gate`; consequence is an additional description and cannot relax them. Contradictions are review findings. `exec-tier: strong` remains a hard lower bound. Existing complexity questions continue to apply until their replacement is explicitly ratified.

```yaml
# PROPOSED extension, not accepted/enforced by current tooling
assessment:
  schema: brief-assessment-v1
  size: {rationale: "One parser and its existing fixture suite", evidence: ["Context"]}
  specification: {level: ready, rationale: "Wire contract and malformed inputs specified", evidence: ["Task"]}
  reasoning: {level: pattern, rationale: "Existing parser pattern applies", evidence: ["Read first"]}
  coupling: {level: shared-contract, rationale: "Generator and dispatcher consume the field", evidence: ["consumers"]}
  verification: {level: partial, rationale: "Fixture checks cannot establish classification judgment", evidence: ["Verify"]}
  consequence: {level: recoverable, rationale: "Wrong classification delays work; revert documented", evidence: ["Review"]}
```

Linter validates shape/enums/references, not the truth of a judgment. Unknown required facts block implementation until supplied or explicitly scoped into a discovery brief. An open design choice may be delegated when constraints and acceptance are sufficient. Unknown consequence requires assessment before dispatch; unknown verification requires an explicit review route. Missing legacy assessments render `not-assessed`; never impute ready/low. Rollout is additive parsing → advisory capture → pilot enforcement → explicitly approved expansion. An older reader that ignores an optional field does not qualify as enforcement; dispatch requires a proven compatible tool version before arming the gate. No forced schema renumbering or historical backfill of guessed assessments.

## 4. Acceptance review before implementation

The author remains responsible for intent, scope, non-goals and initial acceptance criteria. A fresh non-author reviewer first receives those plus permitted domain evidence, WITHOUT the author's proposed Verify table or implementation. It records candidate failure modes. It then receives the draft Verify table and maps those modes to checks, amendments or explicit review-only limits. This two-step input sequence reduces anchoring and is recorded; it does not prove cognitive independence.

Review records MUST contain author/reviewer run identities, actual model identifiers when available, input digests, exposure sequence, requirement-to-check mapping, accepted/rejected findings with reasons, residual unchecked claims, result (`accepted / changes-requested / could-not-review`), and reviewed contract digest. Reviewer may amend criteria only within the existing intent. Scope/value changes go back to the decision authority. No quota of extra tests; a justified no-change review is valid.

Admission binds to the exact approved contract: normalized frontmatter excluding mutable evidence/lifecycle fields, Context, Task, constraints, requirement references, and Verify. The implementation MUST publish canonicalization test vectors; content digests cannot depend on host line endings or YAML key order. Any semantic change to those inputs invalidates the approval. Git revision remains recorded separately. Documentation-only formatting changes may preserve the canonical digest only when the canonicalization contract proves equivalence.

Dispatcher MUST verify non-author identity, digest match, successful review, applicable policy version and resolved human decisions. Missing input is `could-not-check`, not authorization. Reviewer attestations come from the review role's existing identity/control path; a worker-authored frontmatter flag cannot authorize its own dispatch. Different model name alone is insufficient. Self-review cannot satisfy this lane. Post-merge verification remains a fresh non-implementer execution of the checks; it is not replaced by acceptance review.

After dispatch, contract amendments record before/after digests, reason and actor, supersede approval and require re-review before further dependent work. The worker may collect evidence without changing the contract. Evaluation-only held-out checks must derive from the frozen intent and be versioned separately; a new requirement discovered after implementation cannot be relabeled a pre-existing defect.

## 5. Durable records and ownership

Use existing statusgen provenance/witnesses and qualgen telemetry/attribution seams. Add a portable `brief-quality-event-v1` envelope and a pure reducer under NEW `qualgen/briefquality/`; do not create a second history miner, scheduler or global database. Local spool and retained evidence live outside public source by operator configuration. Source repo fixtures are synthetic.

Every event carries: schema, event_id, repo identity + stable brief UUID + display ID, brief version, observed_at, recorded_at, source locator/digest, actor/role/run identity, event kind, and typed payload. Every attempt has an attempt_id and role (author, acceptance-review, implementation, PR-review, verification, repair). Provider/model/reasoning-effort/requested-versus-actual values, tool environment digest and routing-policy revision live on attempts, not in the task profile. Unavailable fields retain a reason.

Required event families:

- `snapshot`: authored, dispatch-approved, completed; exact content digest/revision and assessment. Capture authored when submitted for first review, approved at admission, completed at independently verified closure; abandoned/cancelled briefs retain an explicit terminal outcome without inventing completed verification. All attempts and amendments remain linked.
- `acceptance-review`: first-pass findings, their dispositions, approval digest, unchecked claims and measured review cost.
- `dispatch / attempt-finished`: selected policy, actual execution settings, completion/failure/cancellation, usage source and unit; replayed reads are idempotent.
- `discovery / contract-amended`: missing fact, unresolved choice, bad reference, dependency, consumer, scope split, environment issue or changed requirement; evidence, severity and disposition recorded separately from observer's initial suspicion.
- `verification / defect`: consume witnesses and independently adjudicated attribution; track detected_at and the stage escaped. A stage introduced and a stage that failed to catch it are separate fields.
- `cost / interval / ruling`: meter provenance and completeness, role-specific active/queue/human-wait intervals, authoritative decision links. Estimated currency carries price-table version; tokens never silently convert to currency.

Writes use unique per-event files with atomic rename (NEW reference file adapter), avoiding concurrent JSONL appends. A reducer may emit deterministic JSONL exports. Duplicate event ID + identical digest deduplicates; same ID + different content is an error. Corrections append a superseding event; original records persist. Late events revise aggregates while retaining report cutoff, input set digest and prior report identity. Snapshot content must remain retrievable under retention policy, not merely named by an eventually unreachable git SHA. A hash proves content identity, not completeness or author authority.

Adapters collect only approved fields, never full prompts, credentials or raw sessions by default. Redaction/deletion obligations win over append-only retention; tombstones record unavailable evidence and reports withdraw dependent claims. No public telemetry upload, automatic issue filing, routing change or CI schedule in the pilot.

## 6. What the analysis reports

Report every metric with numerator, denominator, eligible count, measured count, missing reasons, time window, cohort, evidence links and method version. Separate assessment changes before dispatch from deficiencies discovered afterwards. A missing observation is not zero; a zero denominator produces could-not-measure. Findings are not defects until dispositioned; multiple findings about one omission count once by stable finding identity.

| Measure | Definition / limit |
|---|---|
| Readiness omission rate | Dispatched briefs with at least one independently accepted pre-existing missing fact/choice/reference/dependency / dispatched briefs with observation coverage; also report uncovered eligible briefs |
| Initial contract gaps | Briefs with accepted acceptance-review coverage findings / briefs actually independently reviewed; rejected/redundant findings shown separately |
| Residual contract gaps | Approved briefs with a subsequently confirmed gap in the frozen contract / observed approved briefs; distinguish changed requirements |
| Assessment calibration | Per-dimension authored→approved→observed transition counts, unknowns and disagreements; observed judgments carry evidence and adjudicator; no invented numeric ground truth |
| Scope stability | Dispatched briefs requiring unplanned split or scope amendment, by reason; planned discovery outcome is not automatically a defect |
| Review yield | Accepted actionable findings, rejection rate, review cost and delay; do not call caught issues avoided defects without counterfactual evidence |
| Verified delivery cost | Sum attributable measured cost of ALL cohort attempts, including author/review/repair/failed/abandoned attempts, divided by independently verified accepted briefs in that fixed cohort; incomplete coverage yields partial-cost label, never complete-spend ranking |
| Escapes | Confirmed defects linked to frozen contract and stage, at a declared observation horizon; younger cohorts are right-censored and kept separate |

Attribution vocabulary: `brief-omission / implementation-error / environment / changed-requirement / unresolved`. Attribution records observer, adjudicator, evidence and dispute status. Extend existing qualgen stage attribution rather than converting a tracing heuristic into a causal verdict. Stage and cause can be multi-valued with allocation explicitly unknown; do not double-count aggregate defect IDs.

Author quality is cohort evidence, not a leaderboard. Stratify by the six dimensions, authoring skill/prompt version, model, repository/domain and review policy; show small sample sizes and selection bias. Work size is not realized wall time: derive execution time only from measured intervals. Historian in-progress dwell is a labeled proxy and MUST NOT be renamed active time. Separate queueing, human waits, model execution and tool execution where measurable; otherwise state missing.

## 7. Offline tool contract and operator loop

Proposed entry point: `qualgen briefs --events <dir> --as-of <timestamp> --out <dir>`, with JSON plus self-contained HTML and Markdown projections from the same reducer. No network by default. Tool exits: 0 complete measured report, 2 invalid/conflicting input with no new report, 3 report produced with gaps, 1 IO/internal failure. Preserve raw denominators and evidence drill-down when filtering. Mixed units cannot form a spend total. A synthetic fixture report is visibly labeled throughout; never mix it with an operator cohort.

The initial report answers: which facts were missing after dispatch; which critical claims lacked detection; where assessments changed; what the review added; where cost data are absent. It proposes a concrete authoring/template correction with evidence references for human/intake triage, but does not file or apply it automatically. Periodic collection is operator-invoked initially; an on-demand repeat produces byte-identical data for identical normalized inputs. A scheduled monitor is a separately approved deployment, not created by this spec.

## 8. Pilot and evaluation

Before collecting, record policy, eligible cohort, task strata, allocation method, budget/retention, observation horizon, metrics and stopping rules. D2–D5 must be ruled; no silent default. No universal minimum sample size or improvement threshold is invented here: the pilot proposal must justify them from expected volume, effect size and acceptable uncertainty before outcomes are inspected.

Compare author-only initial contracts with the independently reviewed revisions; do not deliberately remove mandatory safety gates to create a control arm. Measure accepted gaps and additional cost first. A shadow pilot can show corrections and feasibility, but cannot establish avoided defects or causal ROI. Any prospective experiment comparing model/effort pairs must respect capability minima, gate parity and equivalent tools/context; task-profile matching alone does not eliminate confounding.

Report unsuccessful and abandoned work as well as completed work. Cost per verified brief uses a fixed entry cohort and reports follow-up completeness; a completed-only sample hides failures. Pilot results may recommend revise/continue/stop, never automatically promote. Fresh independent review of the pilot report precedes a human rollout ruling. New default dispatch enforcement requires version pins and rollback procedure; preserving evidence during rollback is mandatory.

## 9. Existing implementation seams

| Existing path | Reuse / boundary |
|---|---|
| `spec/brief-v1.md`, `docs/brief-template.md`, `statusgen/schemas/brief-v2.json` | Reconcile documented v1 contract with implemented v2 extension; keep one dimension vocabulary |
| `statusgen/brieffile.go`, `briefv2.go`, `newbrief.go` | Parse/lint/scaffold profiles, preserve versions and UUIDs |
| `statusgen/history.go`, `verifyoutcomes.go`, `briefefficiency.go`, `costoutcome.go` | Reuse witnesses and history, preserve proxy labels and requirement-linked outcome meaning |
| `tools/desk/internal/deskkit/briefschemagate.go`, `tools/desk/cmd/deskdispatch/` | Tool compatibility and pre-dispatch policy admission |
| `plugins/assay/skills/{author-brief,pr-review-desk,worker-desk,verify-desk}/SKILL.md` | Author assessment, acceptance review, amendments, independent verification |
| `qualgen/telemetry/source.go`, `qualgen/attribution/`, `qualgen/reflex/` | Portable events, attribution, findings and gate-yield; no duplicate miner |
| `tools/metrics-harvest/cost.go` | Existing cost-per-row stays distinct from cost-per-verified-brief; do not optimize by adding rows |

## 10. Acceptance of this design and implementation

The stream carries executable checks for its eventual implementation. Document presence checks prove structure only; independent review owns adequacy of the categories, attribution rules and pilot protocol. Seeded contradictions MUST exercise stale approval, self-review, missing cost, duplicate/conflicting events, missing snapshot, changed requirement, concurrent write, late correction, abandoned work, censored defects and unresolved attribution. Every unsupported input must render a gap, never a favorable result. No risk or human gate can be weakened by a profile, an approval or a report.
