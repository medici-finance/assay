# Verification migration: execution first, authority last

**Planning amendment, 19 October 2026.** Source inspected at public main
`2b2383f8e`. This proposal refines the existing R1–R8 sequence; it neither certifies
implementation nor activates a deployment. Consumer cohort identities belong in the
adopter's plan. No second controller, publisher, queue taxonomy or acceptance evaluator.

## Boundaries and sequence

1. **Evidence production:** an existing authorized owner invokes a qualified runner for
   selected work. Retain the current independent verifier and publisher. Historical or
   shadow output cannot acquire current acceptance authority.
2. **Shared assessment in shadow:** graph-execution/28 consumes the existing pure evaluator;
   /29 compares legacy and new adapters against independently specified expected results.
   A legacy/new disagreement is investigated, never decided by majority or equality alone.
3. **Durable verifier:** /19 store, /16 ownership, /26 supervisor, /04 effect identity and
   /14 admission feed /21. Six observable responsibilities are bind, admit, execute,
   assess, decide, publish/reconcile. They are logical components, not six services.
4. **Integrated proof:** /18 qualifies the connected production path using disposable
   infrastructure and synthetic providers. Model providers may be disabled; fake/recorded
   assessments test policy without establishing model quality.
5. **Authoritative handover:** R4–R8 proof and a separately authorized adopter action pause
   the selected repository's old verifier publisher, reconcile submitted effects and
   fence old capabilities before the new publisher is admitted. Rollback performs the
   same ownership/effect reconciliation. Sampling execution does not permit two writers.
6. **Kubernetes profile:** agent-execution/08 adapts the existing runner/supervisor contract.
   It remains in the parked execution stream pending approval of that stream's controlling
   amendment and this profile. It is not on the interim evidence-production critical path.

## State and queue contract

One immutable candidate/acceptance instance may contain several bounded attempts. Persist
attempt outcomes separately from the acceptance decision and publication receipt.
The lifecycle is Received → Bound → Admitted → Executing → Assessing → Decided →
Publishing/Reconcile → Complete. Held is a resumable superstate containing a typed reason,
responsible owner, required fact/event and observation cutoff. Cancellation and supersession
stop new admission, retain evidence, and reconcile already-submitted effects before release.

| Ingress/event | Guard and transition | Egress/work obligation |
|---|---|---|
| Candidate or accepted-contract revision | Validate identity/contract; new instance on changed candidate | Admission work or contract repair |
| Admission eligibility/capacity | Current authority, independence, budget and complete prerequisites | Execution activity or typed hold |
| Execution result | Match attempt, candidate, contract, generation and environment | Evidence assessment; stale results retained but cannot advance |
| Assessment complete | All mandatory claims supported, policy and human acts satisfied | Accepted; otherwise repair-required or held |
| Publication response or timeout | Read stable effect intent; reconcile unknown before retry | Confirmed receipt or reconciliation work |
| Relevant fact/event or bounded reconciliation timer | Re-read authoritative facts; fingerprint complete; unchanged inputs stay held | Reevaluate only affected obligations |
| Cancellation/supersession | Fence further work; reconcile external effects | Terminal record plus resource-retirement obligation |

Queues are views/activities over the existing coordination records, not new authoritative
stores. A code-defect returns to the existing worker repair owner and creates a new candidate;
a contract defect goes to its author/review owner; environment failures go to operations;
human waits go to the existing human-decision owner. Resource retirement remains separate
from acceptance. Accepted/publication-pending is visible and cannot be called Complete.

## Non-deterministic activities

Deterministic code owns transitions, applicability and authority. Generative activities may
investigate and produce structured, cited assessments under an admitted rubric; they cannot
rewrite the rubric, certify unexecuted checks, grant independence or publish. System-1/CLM
activities may classify ambiguous failures or recommend a permitted route. Both are observations
with versioned inputs and recorded outputs, explicit abstention and bounded cost/time.
Replay uses those recorded observations, not another inference call. Classifiers start in
shadow; /11 and /12 own provider and calibration qualification. Missing calibration cannot
become authority and must not prevent the model-disabled verifier from operating.

No checkpoint, probability threshold or new model is selected by this amendment.
A CLM here means a constrained classification/decision model, not a guarantee of truth.

## Existing owners and migration effect

| Owner | Migration effect | Required proof |
|---|---|---|
| desk-tools/45 and current adoption owners | One canonical queue meaning reaches callers | Unknown coverage never counts as zero; do not recreate active work |
| graph-execution/02 → /09 → /28 → /29 | Bind exact work and assess evidence consistently | Candidate mismatch, missing facts and independently judged discrepancy corpus |
| graph-execution/04, /16, /19 | Durable intent, ownership, waits and recovery | Real database recovery; stale authority and unknown effects remain held |
| graph-execution/20 → /26 → /22 | Protocol → supervision → qualified local execution | Real adapter conformance and independent custody checks |
| graph-execution/14, /21 | One deterministic controller, six visible responsibilities | Queue/result provenance, replay, budgets, cancellation and typed resumption |
| graph-execution/25 | Governed publication | Lost acknowledgment cannot create a second effect; no approval inferred from publication |
| graph-execution/18 | Connected acceptance proof with models disabled | Crash at every durable boundary; failed/held outcomes retained |
| R6/R7/R8 existing owners | Authorized observations, thin view and handover | Partial coverage, capability refusal and repository-wide handover |
| agent-execution/08 | Kubernetes transport under the same owners | Offline hostile API fixtures, then separately authorized disposable-cluster qualification |
| graph-execution/11–12 | Optional System-1/CLM provider and calibration | Rules baseline, independent labels, abstention and relevant-workload error/cost |

## Dependency amendment

Remove /12 as a hard prerequisite of /18: the latter's existing fake/recorded advice proof
must work with real providers absent. The /12 calibration report remains mandatory before
any real classifier is activated; no calibration requirement is waived. /18 retains its
other dependencies and full controller/publication/control cases. This is a reviewed
sequencing change, not a claim that the runtime can skip R1–R8 qualification.

## Readiness and measurement

The /02 check-definition repair merged in PR #3166 on 19 October; fresh independent
verification remains the head before /09 and /28. Parser repair PR #3429 merged the same day;
consumer installation/pins and failed-check repairs still need separate evidence. PR #3376
owns desk-tools/45; do not amend its active implementation here. The remaining verification-flow consumer work belongs to desk-tools/46–52, authored by
PR #3341 (merged 18 October), not to that closed planning branch.

Record eligible-to-evidence and evidence-to-recorded-completion separately, retries without
changed inputs, active versus held time, operator interventions, incorrect acceptance and
rework. A small pilot proves operation, not a defect-rate or speedup claim. No release,
installation, dispatch reallocation or provider write is performed by this plan.

## Deferred provider activation follow-up

Real provider activation is outside this migration. Before proposing it, the graph-execution
owner must author a separately human-gated activation brief with a hard dependency on
[graph-execution/12](brief-12-decision-evaluation.md) and the applicable admission policy.
That brief must consume exact model/schema/domain/backend calibration and enforce expiry,
revocation, abstention and shadow-to-active qualification. The dependency edge is not yet
authored: this is an explicit outstanding follow-up, not a claim of an enforced activation
gate or authorization to activate. The model-disabled /18 proof remains independent.
