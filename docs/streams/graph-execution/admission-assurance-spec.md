# Graph execution: admission, decisions and assurance

Status: **proposed amendment, routed to briefs; not implemented or operationally approved**.
Authored 2026-09-19; source inspection 2026-09-18. Extends [the graph stream spec](spec.md); the existing human gates stand.

## 1. One execution foundation

Assay's graph evaluator and reviewed pattern bank are the execution core. Keep Cells,
desk-tools, statusgen and replaceable reasoning agents. The graph declares obligations;
the Cell owns bounded execution; probabilistic providers advise; deterministic policy
controls admission and effects. Operator clients project those same records. Do not build
a second scheduler, pattern catalogue, eligibility evaluator or run-record family.

Keep the structural graph (reviewed dependencies), workflow instance (nodes and attempts)
and evidence/provenance relationships distinguishable, joined by stable identity. An
inferred relationship never silently becomes an authorized dependency. Retain canonical
brief IDs; a node instance additionally binds run ID, pattern revision and node ID.

At the refreshed 2026-09-19 source revision, graph 01 is done, 02 is implemented and 03–08 are planned.
The pattern spec remains a draft and explicitly leaves instantiation unspecified. Existing
Decide returns bounded advice but has no calibrated distribution contract. No benchmark,
installed-release reconciliation or live deployment was performed to author this amendment.

## 2. Instantiation and authority

GEA-01. A reviewed pattern MUST instantiate into a versioned contract binding work identity,
Cell, pattern digest/version, node instance IDs, immutable input references, acceptance
version, policy/control-profile revisions, permitted roles/effects and budget reference.
Changes create a superseding instance or invalidate affected decisions/evidence; they do
not rewrite history. Implement this in graph 09, building on 02, with compatibility fixtures.

GEA-02. One eligibility evaluator MUST decide readiness. Admission precedes dispatch and
is rechecked at consequential transitions. Graph eligibility, capacity admission and
agentic suitability are distinct predicates; all applicable predicates must permit work.
A missing mandatory prerequisite holds the affected work. Unavailable informational
feathers remain notices. Claims and budget reservation precede execution.

GEA-03. Graph declarations MUST NOT mint capability. Executor-side subject/role checks
and actual provider permissions independently constrain effects. Keep human merge and
human release/production authorization. The current effect vocabulary is forge-oriented;
release adapters require separately reviewed permissions. Cross-pattern composition is
not supported by the present schema: graph 17 defines explicit lifecycle links, not an
unbounded graph language or automatic deployment.

## 3. Local decision provider

GEA-04. Extend `deskkit.Decide`; retain `Advice` compatibility, defaults, kill switch,
closed vocabularies and mandatory journalling of non-default advice. The decider remains
outside dispatchable worker tiers. Define request/response and policy-result records in
10. A provider gets no write credential, tool callback or ability to select policy.

GEA-05. **Laya is the preferred initial OSS candidate**, behind this provider-neutral
contract. This is a candidate preference, not a quality endorsement or runtime enablement.
Use the upstream `NandhaKishorM/laya` runtime and `convaiinnovations/laya` checkpoint only
after exact revisions, transitive artifacts and licenses are pinned and verified. Use a
separate optional Python adapter process; do not make Python/PyTorch mandatory for core
Go/CLI adopters. CPU is the default pilot backend. CUDA GPU is optional; MPS is a separate
configuration if tested. These are device paths, not assumed different trained models.

GEA-06. Supply a complete local artifact bundle, including tokenizer and encoder config.
Runtime must be offline with no automatic Hub download or remote-provider fallback.
Loading must not silently mutate the approved artifact. Reject missing or tampered parts.
Record requested AND actual device, precision, package/model hashes, input/question digest,
latency, memory and fallback reason. An unapproved device fallback causes abstention;
approved CPU fallback still needs its own measured latency and calibration applicability.
The upstream loader's convenience fallback is not an authorization policy.

GEA-07. Detect token/option limits before inference. Silent input truncation is forbidden:
reject/abstain or use an approved versioned context-selection procedure retaining source
references and omissions. Treat context as untrusted data. Upstream `confidence` and
`action` fields are annotations, never a calibrated probability of safe execution.
Question schemas, option order, batching and numeric rounding need adapter tests.

GEA-08. Evaluate Laya against rules and a small classical classifier before adoption.
Independent labels, related-change/repository grouping and temporal holdouts prevent
training leakage. Report class imbalance, per-class confusion, Brier/log loss, reliability,
selective risk/coverage with uncertainty, abstention, downstream interventions and total
cost per verified outcome. Calibrate on a disjoint split. No universal confidence threshold.
A model/precision/schema/domain change invalidates calibration unless equivalence is
proved. CPU/GPU comparisons use the same cases and report cold/warm p50/p95, concurrency,
peak host memory and VRAM separately. Model execution is optional for ordinary CI; recorded
fixtures must never be reported as fresh model measurements.

## 4. Mixed agentic admission

GEA-09. `AgenticAssessment` MUST separate deterministic facts from probabilistic assessments.
Hard checks cover authority, category opt-in, acceptance availability, executor capability,
data handling, independent verifier, budget and effect recoverability. Advisory dimensions
include ambiguity, verification adequacy, task/model fit and semantic risk. Each dimension
carries evidence, scope, freshness and uncertainty. Do not average a failed hard check away.

GEA-10. Policy outputs one of `blocked`, `discovery-only`, `human-led`, `supervised-agent`,
`bounded-agent-work`, plus reason codes, permitted operations and mandatory gates. These
are proposed admission dispositions, not replacements for graph risk classes or desk roles.
Define an explicit mapping to the pattern's existing `risk-input` requirements; preserve
mandatory nodes by union, never subtraction. A score may restrict an authorized lane; it
cannot open one. Unknown mandatory implementation readiness permits at most separately
authorized discovery. Discovery itself needs scoped read/data authority. All-hard-pass is
necessary, not sufficient, for bounded agent work.

GEA-11. Preserve three separate products: **AssayScore** (its existing deterministic
formula and incompleteness), **AgenticAssessment** (suitability and admitted scope), and
**ControlAssurance** (which controls have what evidence over which scope/period).
Neither a delivery score nor a model score establishes compliance or correctness.

## 5. Evidence and regulated operation

GEA-12. Reuse graph 03's evidence vocabulary and join coverage. Bind evidence to exact
subject, acceptance digest, producer identity, environment and observation time/window.
Keep pass, fail, missing, error, could-not-check and wrong-revision distinct. Model advice
may challenge adequacy but cannot manufacture an executed witness or replace required
independent verification. Protected acceptance and executor identity remain separate controls.

GEA-13. A control profile names its owner, applicability, standard/criteria edition,
mandatory checks, permitted exceptions, retention/access policy and external organizational
obligations. ISO 9001 process support and SOC 2 examination support are use cases, not
product certifications. Do not hard-code one edition or claim that software supplies an
organization's quality policy, internal audit, management review or assessor opinion.
Reuse the ISO stream's validation pack, retention policy, authorizer and effectiveness work.

GEA-14. Export a scoped evidence packet with expected population, included records,
omissions, exceptions, provenance, digests and offline verification instructions. Distinguish
claimed, observed and independently verified facts, and enforced versus advisory controls.
Missing expected records cannot be hidden by an all-green percentage. Secrets and sensitive
payloads stay behind authorized references. Retention/holds and deletion authority apply;
a hash chain alone is not immutable storage. Required record persistence fails closed for
privileged effects; diagnostic logging remains separable.

GEA-15. Corrective action links the original failure, correction, preventive/control change,
owner and later independent effectiveness observation. A code merge alone does not prove
effectiveness. Periodic organizational controls can reference multiple runs and external
records; not every obligation is a graph node on every software change.

## 6. Runtime, lifecycle and operator contract

GEA-16. Extend graph 04/06 for Cell ownership generations, cumulative run budgets,
cancellation, durable stop state and effect reconciliation. Fence stale workers at the
actual effect boundary, including legacy credentials. Restore must retain pauses and
unresolved effects. Unknown external outcomes remain held; no exactly-once claim without
provider support. Keep Go and the existing storage model until measured needs justify a
change. Shared policy libraries or CLI JSON come before copying logic into services.

GEA-17. Link objective/change → instance/attempt → evidence → release → observation.
Keep verified-but-unreleased, released-but-unobserved and inconclusive outcome distinct.
A delivery adapter remains the authority on its actual external state. Lifecycle links
must carry artifact/environment identity and observed provenance; product outcome is not
inferred from merge count. Public contracts have no dependency on a private consumer.

GEA-18. Operator projections show source age, pending owner, blocked edge, scope, evidence,
model/calibration identity and budget. Clients cannot recalculate a more permissive verdict.
No new generic execute/approve endpoint. The cockpit's visual treatment and human identity
custody belong to its consumer contract; a runtime record does not widen that contract.

## 7. Delivery, tests and boundaries

The existing 01–08 milestone remains independently deliverable with deterministic routing
and fake/offline providers. New 09–18 extend it; Laya is not a dependency of baseline graph
correctness. Prototype with synthetic/public fixtures, then an adopter-owned labeled trial.
Adopter rollout requires source/release/receipt reconciliation, approved scope and explicit
credentials/environment authority. No infrastructure is contacted by these authoring briefs.

The combined proof (18) includes: failed hard check despite high model confidence; unknown
prerequisite; stale assessment; incomplete acceptance; wrong-subject evidence; cross-Cell
replay; denied effect; lost acknowledgment; stale owner; unavailable required ledger;
restored pause; budget exhaustion; unauthorized provider fallback; overlong input; missing
control population; and a successful offline path through release/observation references.
Baseline graph tests remain unchanged and the report distinguishes fixture results from
measured model behavior and real deployment.

No duplicate funding: graph 03/04/06 own coverage/recovery/run records; 09–18 own the
incremental contracts and consumers below. Organizational QMS, unrestricted autonomous
production, model training and a graph database are not prerequisites. Product discovery,
causal product experiments, retirement and general data migrations remain future scope.

## Sources and freshness

- Public Assay source inspected 2026-09-18 at `951ca784d100a7d201a28a34033da6709ec2ec8f`:
  `spec/workflow-pattern-v1.md`, `statusgen/eligibility.go`,
  `tools/desk/internal/deskkit/decide.go`, `tools/desk/internal/runnertable/decider.go`,
  `drainloop/journal.go`, `docs/streams/iso-9001/README.md`.
- Laya runtime source pinned for inspection:
  https://github.com/NandhaKishorM/laya/blob/1161ff639204388b1e576a7c5d56a0f6df470455/laya/agent.py
  (device resolution, fallback, local loading and input handling; no benchmark reproduced).
- Candidate model card: https://huggingface.co/convaiinnovations/laya . Exact weight revision,
  transitive artifacts and license provenance are a deliverable of 11, not asserted here.

Source refresh 2026-09-19: public Assay `e4109205751a219330b954f75855c05b4be2a5c8`;
01 verification is now recorded. No changes to the proposed 09–18 target seams were found.
