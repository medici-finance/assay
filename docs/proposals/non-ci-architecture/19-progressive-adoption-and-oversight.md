# Progressive adoption and scoped human oversight

**Draft architectural amendment, 12 October 2026.** Extends the [target](11-target-architecture.md), [future-state model](12-future-state-model.md) and [delivery sequence](16-delivery-sequence.md). It proposes product behavior and contract changes; it does not implement profiles, change current acceptance predicates, delegate authority or activate a runtime.

## Outcome and architectural delta

Assay should be useful on one existing task before an adopter operates a fleet. An organization should be able to vary human involvement by project and work class, with explicit evidence and authority requirements. A small installation is a supported destination, not a temporary exception to a compulsory full deployment.

Keep the future architecture's shared semantic owners, isolated role effects, PostgreSQL coordination and thin clients. Add independently installable, effect-free entry paths and an explainable oversight contract to those owners. The running cell remains the deployment unit for managed execution; an offline library/CLI consumer need not instantiate that deployment unit.

| Existing architectural owner | Amendment | Consumer / preservation obligation |
|---|---|---|
| Definitions and planning | Minimal views/imports of canonical work records; adoption and oversight bindings in target/context receipts | Author skills and issue/PR adapters share identities and meaning; no second brief dialect |
| Work and policy | Scoped, approved human-checkpoint and assurance policy; one pure evaluation result | Installer, controller, effect admission and clients consume the same decision; no client-only eligibility rule |
| Evidence and decisions | Explicit self-check, observation, independence and acceptance provenance; bounded approval units | A successful self-check cannot become independent verification; decision subjects/revisions remain immutable |
| Cell operation | Capability/dependency installation plan, selected prerequisites and qualified upgrades | Reuse configuration, pins, custody and recovery owners; no new profile registry or universal broker |
| Runtime coordination and provider effects | Consume admitted oversight policy; retain current authority, fencing and uncertain-effect reconciliation | Fewer interruptions cannot admit another writer or replay a possibly submitted effect |
| Projection and interaction | Minimal first-use output, effective guarantees and actionable decision queue | CLI/IDE/cockpit agree; notification preferences never answer a pending gate |
| Continuing operations and quality | Measure adoption effort and oversight outcomes with AssayScore and counter-metrics | Measurements recommend adjustments; they do not enact grants or prove per-change safety |

The required outcome is lower setup and attention burden **with the selected guarantees preserved**. Shorter installation, fewer prompts and higher throughput are proxies. Useful repeat use without maintainer repair is the product acceptance.

## Four independent choices, one effective configuration

An entry screen asks for the useful behavior first. Advanced configuration reveals four different axes rather than treating them as one autonomy slider:

1. **Adoption journey:** Inspect, Assist, Enforce or Scale selects behavior and its dependency closure.
2. **Oversight policy:** Coached, Supervised or Delegated is a suggested preset for human checkpoints, review and escalation inside an approved envelope.
3. **Credential topology:** the existing credential owner determines principal separation and blast radius. Fewer Apps do not prove independent verification.
4. **Deployment qualification:** the existing runtime/custody owner determines which execution and effect guarantees have actually been demonstrated.

These are selections over existing semantic owners, not four new services or editable registries. Compile presets to explicit versioned requirements; show their resolved provenance. A workflow label, fewer principals, a score or a capability description grants no authority. A partially provisioned combination is held or unavailable, never silently replaced by another combination.

### Progressive entry paths

| Journey | Required behavior | Initially unnecessary machinery | Result meaning |
|---|---|---|---|
| Inspect | Read an explicit export/snapshot and show evidence with coverage; optional provider reads use an authorized adapter and disclosed target | Standing desks, App creation, CI writer, database service, cockpit, repository mutation | Observed or unavailable evidence, not proof of delivery correctness |
| Assist | Review a proposed scope/check set, capture checks for one task and inspect its result in an existing work environment | Standing loops, automatic publication, portfolio graph, extra report packs, database service | Self-checks, observed review/CI and human acceptance retain their distinct provenance |
| Enforce | Evaluate mandatory requirements and admit only qualified managed effects | Unused roles, unrelated modules, richer hosted/team UI | Policy satisfaction and independent verification require their own actual predicates |
| Scale | Add parallel attempts, standing loops, shared observation and team/portfolio clients | Features unrelated to the selected operational needs | Same contracts and authority, broader operation |

Inspect and Assist use pure definitions/policy/evidence libraries and bounded local adapters over explicit inputs. Persist important task records and observed results as canonical, versioned artifacts; this is not another in-flight coordination database. They do not own leases, publish governed evidence or mint effect capabilities. Live reads still require the existing provider adapter and explicit read authority.

Local check execution is opt-in, with a preview of commands and target. An imported repository, issue or command is untrusted input; inspection does not execute it. If an assisted action enters a managed mutation domain, that domain's approved policy and effect path apply. Selecting Assist cannot bypass repository assignment, acquire another role's credentials or turn ordinary local execution into a qualified independent witness.

The minimal view asks for outcome, scope and acceptance checks. Stable identity, source links and routine metadata are derived. Scheduling waves, broad registers, taxonomy, recurring assessments and optional packs appear when the chosen behavior needs them. Required schema and semantics are still validated; a smaller UI cannot make a mandatory fact optional. Any conditional field requirement is an explicit versioned contract amendment, not a skipped lint rule.

The larger running-cell design is unchanged: PostgreSQL owns transactional coordination, pending decisions, leases, effect intents and projections. This amendment selects an existing offline boundary for smaller use; it introduces no SQLite runtime, parallel scheduler, all-role broker or compulsory per-operation Git journal.

## Acceptance and assurance are separate facts

Every result identifies the work/candidate revision, acceptance contract, evidence applicability/outcome, runner attribution, independence, policy revision, coverage and freshness. A display may say **self-checks passed**, **human accepted**, **requirements satisfied** or **independently verified** only when the corresponding predicate holds. These are facts/provenance, not a replacement lifecycle enumeration.

Existing governed `verified` and `done` conditions remain controlling. The proposed light Assist path is advisory; it does not assert that it has satisfied those conditions. Approval of a different completion predicate would require a separate versioned lifecycle/consumer migration. A user can keep advisory operation permanently without receiving a misleading stronger badge.

Changing journey or oversight policy does not rewrite history. New stronger requirements may need new evidence against the current subject; old self-checks never become independent. A weaker policy cannot silently bless an already-admitted attempt. Compare policy revisions, identify affected obligations and reconcile active checkpoints through the [future-state contract](12-future-state-model.md#discovery-and-handoff-contract).

## One explainable oversight evaluator

Extend the work/policy domain with a pure operation over explicit facts, an approved policy binding and relevant subject revisions. Its result contains applicability, required human acts, required evidence, eligible delegated actions, limits, hold reasons and evaluation provenance. Domain evaluation neither loads a token nor prompts a person nor starts a process.

The controller consumes that result for managed admission. The effect executor independently checks current role/target authority, policy validity, subject revision, authority incarnation and generation at the effect boundary. A UI or agent cannot supply an approved result by setting its own role/preset string. A standalone advisory evaluation can explain requirements but cannot issue effect authority.

Organization requirements bound project/work-class preferences. Composition may narrow permitted actions and increase obligations; conflicting requirements or unknown mandatory facts produce an explainable hold. Any authorized exception binds scope, reason, owner, revision and expiry. An exception cannot overrule a boundary outside its approver's authority. Agents can request an exception or a smaller limit, not approve their own wider envelope.

The evaluator and its input/result contracts follow the [three-part contract pattern](../../contracts.md): versioned artifact, source coverage/negative control, and consumer conformance. The implementation brief must inspect actual module/import boundaries before promising an SDK. A package boundary alone is not custody or a sandbox.

### Human-involvement levers

| Lever | Organizational choice | Owner and bound |
|---|---|---|
| Action authority | Draft, publish a branch/PR, or use an explicitly approved automatic lane | Approved policy plus role-bound effects/forge controls; existing merge/release gates remain until specifically amended |
| Risk routing | Protected scope, sensitivity, reversibility, novelty and external effect | Work/policy evaluator; protected and unknown mandatory conditions cannot be downgraded by a convenience preset |
| Checkpoint placement | Human acceptance of scope, plan, publication, merge or release | Evidence/decision requirements; remove redundant informational prompts without removing required authorization |
| Review sampling | All eligible work, a selected fraction or periodic audit | Approved assurance policy; forced review of excluded/high-risk classes, independent sample selection and visible denominator |
| Review depth and independence | Self-check, model review, non-author review, independent execution or specialist review | Acceptance/evidence owner; a configured label never manufactures a different runner or authority |
| Change and resource limits | Scope, affected repositories, cost, duration or concurrency before escalation | Operator ceilings through existing config/admission owners; increasing capacity does not grant new effects |
| Escalation sensitivity | Ask on material ambiguity, scope drift, missing evidence or repeated failures | Existing policy/guard/escalation owners; model confidence is not a required-check bypass |
| Approval batching/window | Individual subjects or bounded, time-limited delegation | Revision-bound human decision; currentness, consumption and revocation checked at admission/effect |
| Attention cadence | Immediate critical alerts, digest, quiet hours or notification threshold | Projection/notification preferences; the underlying required gate remains pending |
| Attention budget | Desired decision effort and capacity reserved for critical work | Backpressure: batch, reduce work in progress or defer low-priority tasks; budget exhaustion never approves |
| Recovery tolerance | Bounded retries, pause threshold and escalation deadline | Existing recovery/effect owner; unknown submitted effects are reconciled before replay; silence is not consent |
| AssayScore targets | Outcome/component trends worth investigating | Continuing-operations measurement; target/routing rule is approved separately and cannot grant authority |

Presets are templates with explicit expansions, not opaque safety scores. A team can choose a lower-touch policy for routine documentation and stronger checkpoints for sensitive migrations. The initial interface exposes the small preset choice and its consequences; advanced levers are discoverable without twelve day-one questions. No universal numeric thresholds or sampling fraction are selected here.

Numerical and presentation preferences use the existing config resolver. Settings changing allowed actions, required checks, review independence or approval obligations belong to approved policy. Do not hide authority in an environment override or create a second resolver inside the UI. An effective-setting query shows the source, value, applicability and unmet prerequisite without revealing private policy contents to unauthorized clients.

### Sampling and batching need precise subjects

Sampling is permitted only inside an approved eligible class. Selection uses stable approval-unit identities and policy/epoch inputs controlled independently of the implementer, with a retained selection decision; merely publishing an easy-to-reroll seed does not prevent evasion. Repeated runs retain the decision. The implementer cannot choose the sample, reroll identity, split the same approval unit or change its classification to avoid review. Record eligible counts, sampled counts, exclusions and unreadable coverage. Mandatory independent execution or protected-path review is not waived by a human sampling setting.

A batch decision binds explicit subject/revision/action/policy tuples, the authorized actor, expiry, revocation and consumption semantics. It cannot mean approve any future version of this branch. Revalidate before each applicable effect: an expired grant, affected changed subject or revoked policy holds the affected operation. Unchanged subjects may proceed only where their grant still permits it. Quiet hours, elapsed time, absent replies and exhausted attention budgets never renew approval. Already-submitted unknown effects retain their reconciliation obligation.

Changing oversight on active work is a revisioned plan/authority change. Use the existing checkpoint/acknowledgment or cancel/reconcile path; no silent rebinding to a weaker preset. Tightening policy, revocation and authority reconstruction remain effective before the next governed effect; preserving a historical evaluation does not preserve a revoked grant.

## AssayScore evaluates tuning; it does not enact it

The current [score implementation](../../../statusgen/assayscore.go) combines Speed, Flow, Quality and Value with published component inputs, a self-relative baseline window and incomplete-data reporting. It is an outcome instrument, not an enacted HITL policy or per-change probability of safety. Keep its formula/normalization owner; adding oversight does not invent another score.

The useful control is an organization-approved target or component floor that triggers investigation/recommendation. Measure human minutes including review, repair and support, plus independently sampled escaped defects, rework, required-check coverage, exceptions and unavailable data. Reduced review can reduce detection and inflate first-pass yield; a higher aggregate score alone does not establish improved delivery. Low-volume/incomplete data cannot bootstrap greater authority.

Tune one lever on a bounded eligible work class, recording policy revision, workload, coverage, observation window and stop/rollback criteria before observing results. A human or authorized organizational policy process adopts the proposed policy revision; a metric does not mutate grants. No performance, causal effect, cross-organization benchmark or reliability threshold is established by this specification.

## Installation, storage and upgrade behavior

Extend the existing installation/pairing/configuration owner with versioned selected-behavior and dependency information. Compute the closure before demanding prerequisites. Fixture-based inspection needs no forge identity; advisory local records need no standing-role configuration. Qualified managed effects require the existing custody/runtime/forge proofs. Optional packs are absent by default unless selected; a disabled capability is distinct from a required capability that could not be checked.

Preview exact files, pins, permission requirements and any target contacted. Install only the selected closure, verify paired artifacts, preserve existing authored files and provide explicit resume/repair. Removing optional machinery preserves work/evidence and does not revoke a separately granted credential by guess. Supported lighter paths remain usable when an upgrade is held. Version mismatch is visible rather than silently loading an unqualified fallback.

For managed execution, persist effective policy bindings, approval units, sampling decisions and pending gates transactionally through the existing coordination/outbox owner. Canonical approvals and required audit evidence follow the established retained-record policy. After restart preserve pending decisions and sampling identity; after reconstruction, unknown sampling/consumption/budget history cannot become unsampled work or a fresh grant. Use retained records or an explicit authorized reconciliation while effects remain held. Fresh authority incarnation and old publisher revocation still apply.

Notification delivery preserves deduplication/delivery-unknown behavior under the [storage contract](14-storage-forge-graph.md#notification-deduplication-is-a-carried-contract). Notification batching may coalesce presentation, not erase distinct decisions or their authorization requirements. A quieter projection cannot turn pending acceptance into completed work.

Target and authoring receipts bind selected behavior/oversight contract revision, approved scope, capability evidence and producer/consumer versions where relevant. Discovery includes installer, author skills, CLI/API/cockpit, evidence publication and deployed configuration consumers. A preset change affecting a required approval is a material contract change; pure notification layout changes need not invalidate unrelated acceptance. No private policy values, counts or target existence may escape through impact/diagnostic output.

## Adoption acceptance and architecture qualification

The following are future executable obligations. Passing document checks in this PR does not execute or satisfy them. Implementer-independent fixtures must exercise the public APIs and actual served/effect boundaries, not only a preset table or schema.

| Case | Required discriminator |
|---|---|
| Clean-host inspection | Explicit fixture/export produces a useful report with no network, credentials, home roster, source checkout, database service or repository mutation |
| First and second assisted task | Canonical records/checks/results work in the existing environment without standing roles; required facts remain valid and repeat use needs no maintainer repair |
| Self-check presented as independent | Consumer refuses the stronger claim although the command succeeded; policy satisfaction and verification provenance remain distinct |
| Interrupted installation / bad pin / existing files | Resume changes only unmet artifacts; bad/missing digest holds installation; user work is preserved |
| Protected scope with low-touch preset | Display and admission explain the same required act; permissive fake provider does not defeat the lower authority/effect control |
| Worker edits preset, authority or required checks | Workspace/env changes cannot widen the approved contract; current operator-approved policy remains the only authority source |
| Expired or changed batch subject | Relevant effect holds; unrelated valid subjects retain only their actual grant; silence does not authorize |
| Sampling reroll, split or unreadable coverage | Independent selection and stable approval unit prevent evasion; unknown coverage never means complete unsampled success |
| Full attention budget / quiet hours / timeout | Backpressure or deferred delivery preserves the gate and decision subject; no automatic acceptance |
| Restart and database reconstruction | Pending/sampled/consumed/unknown states do not become fresh grants; old capabilities still fail at the effect boundary |
| Higher AssayScore with less detection | Outcome assessment retains coverage and sampled defect counter-metrics; recommendation cannot enable greater authority |
| Upgrade, downgrade and existing governed work | Preserve IDs, historical Evidence and active checkpoints; stronger claims require applicable evidence; mandatory organization requirements remain binding |
| Same source through offline and cell consumers | Canonical domain meaning agrees for equivalent authorized inputs; fresh-effect requirements and differing visibility/coverage remain explicit |

Measure hands-on setup, elapsed time, user decisions/concepts, artifact/dependency/permission counts, first useful result, total support effort and independent repeat use. Agree workload-specific acceptance targets before qualification; no arbitrary under-ten-minute promise is made. An exploratory usability sample locates friction but does not prove broad demand or security. A read-only assessment with no finding is a legitimate useful result.

## Delivery changes to the existing program

1. **Approve the bounded adoption/oversight contract delta.** Name advisory acceptance behavior, material policy decisions, supported first scope and unresolved advanced levers. Preserve existing authority until its explicit migration.
2. **Deliver the small pure contract and fixture slice.** Extend R1's domain/evidence boundaries and existing install-contract pattern; inspect package-main extraction/import closure. Start from a bounded manual context packet. No PostgreSQL or full resolver is required to implement this slice.
3. **Ship read-only inspection, then assisted records/results and selected installation.** These are independent product slices with their own qualification and release/pin acceptance. Reuse current supported harness and source adapters; do not require a new IDE plugin, cockpit or full registers merely to demonstrate one task.
4. **Integrate oversight into R2–R5/R7 managed flows.** R1 owns meanings, R2 retains decisions, R3/R4 enforce authority and R5/R7 expose the same holds/results. Preserve R8's whole-repository handover and separate activation gates. Missing runtime qualification holds Enforce, not an independently qualified Inspect release.
5. **Add advanced sampling/batch delegation and scale only as qualified owner deltas.** Notification batching can ship first because it does not alter acceptance. Sampling/approval changes need explicit policy contracts, negative cases and relevant deployment evidence.

This is an amendment to the existing package sequence, not ten new dispatchable briefs or another control-plane rewrite. Reconcile installer/identity work, optional report packs, operator-attention, quality/score, current lifecycle, context and effect owners before allocating implementation. Preserve completed work and source accounts; apply exact consumer/unfinished-brief amendments through the current handoff.

### Decisions before implementation or activation

| Decision | Recommendation | Alternative / consequence |
|---|---|---|
| Light completion meaning | Permanent advisory Assist; governed predicates unchanged | A lighter governed predicate needs its own versioned lifecycle migration |
| Initial surface | Shared offline contracts and existing CLI/harness, optional running cell | Hosted-first makes service operation and connectivity entry prerequisites |
| Initial oversight slice | Scoped risk/checkpoints plus explainable presets and notification preferences | Presentation-only can ship while authority-changing controls remain unapproved |
| Sampling/batch delegation | Explicit scoped policy and effect-boundary qualification | Defer these controls rather than expose an unqualified setting |
| Score response | Recommendation with component/coverage/counter-metrics | Automatic authority relaxation from a composite is outside this proposal |

Architecture approval names the reviewed target and contract scope; implementation authorization produces the proofs, and activation remains a separate authorized operator act. No numerical thresholds, live scope, credentials or local deployment configuration are chosen here.

## Source baseline and quality disposition

The current architecture is on main after [PR #2501](https://github.com/medici-finance/assay/pull/2501); its draft status remains explicit. This amendment inspects public source at [`7908965abe8d43ff9c21d1fbf099f42f33aba102`](https://github.com/medici-finance/assay/tree/7908965abe8d43ff9c21d1fbf099f42f33aba102): [initializer](../../../statusgen/init.go), [approved policy data](../../../deskcore/policy/policy.go), [configuration resolver](../../../deskcore/config/resolve.go), [score](../../../statusgen/assayscore.go), existing installation and proposal contracts.

The initializer currently scaffolds the full stream/register/rules/CI shape. Policy data/structural validation explicitly precedes runtime predicates; statusgen remains a main package. These facts nominate bounded extraction, package selection and contract extensions, not proof of a delivered small-use SDK or a mandate for a full rewrite. Preserve no-clobber, evidence uncertainty, policy-digest and configuration-boundary regression cases.

No fresh adoption conversion, customer timing, churn/hotspot comparison or oversight outcome measurement was collected for this amendment. They remain implementation/evaluation gaps. Broader refactoring retains the existing quality investigation and preservation requirements; touching an owner does not authorize unrelated cleanup. All added behavior above is proposed until independently qualified and deliberately adopted by consumers.
