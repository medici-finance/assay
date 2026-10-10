# Migration: shared contracts, verifier first, then other flows

**Draft proposal, 10 October 2026.** Reads with the [target architecture](11-target-architecture.md) and [future-state model](12-future-state-model.md). This is a program-level migration proposal, not a dispatchable set of briefs. The [delivery sequence](16-delivery-sequence.md) names focused PR packages and the remaining execution-planning work. Existing authority and evidence requirements remain in effect until their specific amendments are approved.

For each slice, map open and recently closed issues to their failure mechanism, owning contract and old/new regression disposition. Urgent supported-path defects receive a repair; closed fixes become preservation obligations. Decision and rollout debt remain separately owned. An architectural mapping does not close an issue or require fixing every issue before the verifier pilot.

## Use contract replacement and flow cutover together

Use contract substitution and flow cutover together. Replace a shared contract when that safely removes duplicate semantics; switch a complete flow when its authority and state must move together. A verifier-only vertical slice exercises enough of the whole architecture to expose missed crossings: source resolution, admission, claims, execution, independent evidence, publication, recovery and human visibility.

Do not first rewrite all libraries and discover integration at the end. Do not make a parser migration or universal CLI a prerequisite. Deliver the smallest new system that can complete and recover one real verifier obligation, with the same contract visible through its API and cockpit.

| Work arriving during migration | Proposed policy |
|---|---|
| Urgent security, correctness or availability defect in a supported flow | Repair the old flow immediately; record applicability to the new owner and its regression case. Fix the shared implementation once where possible; otherwise implement the corresponding protection in both. A structurally absent defect gets a reason and proof, not a pointless duplicate patch. |
| New product feature | Design and implement against the approved target. If it needs to appear through an old entry point, use a narrow adapter. Do not create another full legacy implementation. |
| Existing unfinished feature brief | Reconcile against the target: retain useful work, amend, replace, split or retire. Recalculate its dependencies and Verify obligations. “Already planned” is not a reason to build an obsolete architecture. |
| Active implementation affected by a new architecture decision | Identify its pinned assumptions and owner; choose compatible continuation, explicit handback or cancellation/replan. Preserve work and evidence; do not silently edit underneath it. |
| Critical legacy-only exception | Record why waiting or using the new seam is insufficient, the bounded change, a new-system applicability obligation and retirement condition. Exceptions are visible, not an indefinite second product roadmap. |

“Both” means **both supported behaviors are protected**, not two permanent codebases or mechanically identical patches. A per-defect applicability record should name old/new affected versions, the semantic regression and where it runs. That obligation closes when each supported path is fixed or shown unaffected.

## Migration stages and their exit evidence

| Stage | Concrete deliverable | Evidence needed before proceeding |
|---|---|---|
| 0. Reconcile intent | One draft target, semantic owner/retirement map, approved decisions and affected-plan dispositions; a manually reviewed bootstrap handoff while authoring tools are built | The verifier packet declares source/query coverage, current and future consumers, actual maturity, protected history and active-work obligations. Architecture approval does not require an implemented resolver. |
| 1. Establish the thin spine | Existing regression floor, shared identity/fact/evaluation contracts, isolated runtime, PostgreSQL coordination/reconstruction and a public API slice; bounded planning schema, resolver and author-skill integration in the planning lane | Source-to-result and authoring handoff fixtures pass; missing/partial facts, stale generations and wrong revisions remain distinct; complete-loss reconstruction and safe reruns are exercised. Qualification gates the corresponding capability, while independent implementation may overlap. |
| 2. Substitute safe contracts | Pure evaluation and wire contracts used by selected old callers and the new verifier | Same frozen inputs; every difference classified as intended change, legacy defect or insufficient evidence. Preserve approved semantics, not accidental bugs. |
| 3. Shadow and rehearse | Read-only queue/acceptance comparison, disposable verification runs, recovery and handover rehearsal | Shadow has no authoritative claims or publication credentials. Fault cases prove recovery, isolation and applicability; dry-run output alone is insufficient. |
| 4. Cut over verification for one repository | Pause all verification publication for the selected repository; revoke old publication paths and move the whole repository verifier flow. The old verifier may serve other repositories only through separately scoped authority. | Proven revocation/expiry and process containment, no unresolved conflicting old effects, fresh authority incarnation, observed evidence PR, cockpit visibility and demonstrated recovery. |
| 5. Expand by flow | Review, worker, intake/coordinator and later continuing-operation consumers, ordered by readiness | Each flow reuses admission/identity/effect/evidence contracts and has its own qualification and retirement evidence. There is no mandatory wait for all workflow-graph features. |
| 6. Retire old paths | Delete unused commands, duplicate reducers/stores and shims; migrate remaining supported consumers | Last-consumer inventory, retained history readability, tested state migration, release/pin updates and closed recovery window. |

Stages can overlap in independent areas, but authority handover is sequential. The public service/cockpit refactor is a delivery lane of this program. Its first useful slice supports the verifier; broader UI packaging and hosted features can follow without defining a different state model.

## Authoring-support delivery order

This architecture includes discovery coverage, reproducible handoff, historical preservation and phase-specific authority requirements. An earlier implementation PR is not a prerequisite for approving them. Once the owning specification/decisions are approved, deliver the neutral contract and synthetic fixtures, bounded resolver/receipt checks, then public author-spec/author-brief integration through small reviewed PRs. Skill changes consume an available versioned interface; they must not instruct users to call unshipped commands. The private corpus supplies additional qualification without becoming a public dependency.

Use the [bounded bootstrap path](12-future-state-model.md#phase-specific-gates-and-the-bootstrap-path) to author those first changes from a manually inspected packet, recording approval scope, coverage limits and the affected revision. Recheck source membership and active claims before dispatch. Build the verifier runtime in parallel where its approved contracts permit; require fencing/recovery proofs before activation, not before implementation begins. An unapproved rival proposal cannot freeze authorized work.

The automated planning path becomes usable only after the actual resolver and both author skills complete one reviewed verifier change: discover consumers, emit a structured handoff, amend the appropriate unfinished plans, preserve history, and correctly recheck changes before dispatch. Until then the manual route remains explicitly recorded. Preserve separately scoped offline artifact work; do not silently add these deliverables to it. Estimate the planning extension explicitly and review its cost alongside the first verifier slice.

## The verifier transaction, end to end

1. **Resolve the obligation.** Identify the work in its tracking repository, the landed candidate in its delivery repository, the approved acceptance-contract revision and relevant policy/fact snapshot. Resolve applicability by the declared artifact/content rule; do not assume the newest main SHA is automatically the only valid subject.
2. **Admit and reserve.** Check prerequisite evidence and decision gates, budget policy, stop state and source completeness. Reserve the scoped obligation with an owner and generation; persist the attempt before launch. Distinct work definitions and repeated attempts are not aliases.
3. **Select and execute independently.** A trusted selector fixes the contract independently of the candidate. Run in a disposable sandbox with explicit inputs and permitted capabilities. The runner has no publisher, human-gate or operator credential. Separate recorded operational permission from an agent's self-description.
4. **Collect and assess.** Persist logs/artifacts, execution identity, actual input digests and completion state. An independently owned evaluator assesses the result against the selected obligations. Timeout, unavailable observation, malformed output and failed checks remain distinct; a zero exit status alone does not constitute a passing assessment.
5. **Publish through the authorized path.** Recheck the candidate/acceptance binding and relevant current authority, record a stable effect intent, and publish the attributed result. Use a governed evidence-PR path; preserve independent review/merge and human gates. Where an adopter currently permits direct evidence writes, retire or restrict every such verifier path before the cutover and record the necessary policy amendment. A submitted evidence PR is not yet accepted completion.
6. **Reconcile and project.** Record provider acknowledgments or unknown outcomes, reconcile lost acknowledgments, and derive work lifecycle from accepted evidence. Release only the matching owner/generation. Refresh board/API/cockpit from the same semantic record. Appending bookkeeping must not change the acceptance subject and cause a verification loop.

The [regression plan](15-regression-from-day-one.md) reuses current fixtures and adds transactional intent recording, full database deletion/reconstruction and safe-retry cases from the first implementation. The [storage contract](14-storage-forge-graph.md) uses PostgreSQL for operational coordination and allows some transient loss/repeated work after database loss. Canonical intent and published evidence remain recoverable; reconstruction reconciles effects before conflicting publication.

A workflow engine, standing verify desk or manual invocation can request this use case, but none implements a second acceptance algorithm. Work completion can remain pending after an attempt finishes. Reverification follows a meaningful input/obligation change or an explicit request, not a broad file-hash accident.

## Qualification cases drawn from the current fault patterns

| Case | Required result |
|---|---|
| Tracking and delivery repositories differ; invocation starts in an unrelated cwd | Correct coordinates and source revisions reach runner, receipt and publication. |
| Forge pagination is capped, read scope is denied, or a cached approval is stale | Visible incomplete/unavailable/needs-refresh state; no authorization derived from absence. |
| Acceptance changes after selection or the candidate differs from the evidence subject | Hold/re-evaluate applicability; retain historical evidence without relabeling it current. |
| Worker finishes after its lease was replaced; old release arrives last | New holder remains intact; stale attempt cannot publish under the new generation. |
| Provider accepts an effect and the connection drops before acknowledgment | Reconcile the same intent; do not blindly duplicate publication or restart the obligation. |
| Controller/runner dies at each durable transition; the host restarts | Recover owned attempts and outstanding effects; terminal reconnection does not launch work. |
| A failing witness exists, a success marker appears only in quoted text, or output is malformed | Evaluate the typed applicable result; do not accept witness presence or string matching. |
| Evidence recording alters a brief but not its acceptance-relevant content | No self-triggered verification loop. |
| Candidate attempts to use another cell's socket, publisher key or host administration | The qualified deployment boundary refuses the attempt. A source-level purity test alone is insufficient. |
| DB/artifact backup restores to a prior point while provider effects exist | Restore in paused mode, reconcile external effects and missing artifacts, then reopen admission. Never replay stale outbox entries blindly. |
| Cockpit reads stale projection data or reconnects its terminal | Show freshness and durable attempt identity; no duplicate mutation or inferred acceptance. |

These cases turn reported mechanisms into a qualification plan; this research has not reproduced every incident or executed the new architecture. Each implementation brief must pin the incident or independent fixture that demonstrates its claimed failure mechanism. Tests should observe behavior and authority at the actual boundary, rather than assert that expected strings or package names exist.

## Handover and rollback are state migrations

**First cutover unit: one repository's complete verifier flow.** The first pilot does not require a credential to distinguish individual work cohorts inside that repository. Pause legacy verifier admission and publication for the repository. Other legacy roles may continue only if their credentials cannot take the retired verifier publication path; a broad worker credential that can still perform that effect is part of the revocation inventory.

1. Inventory every old verifier publisher, direct evidence write path, token/minting source and automatic/manual caller, including writers outside the selected host. Capture existing attempts and submitted effects before removing their authority.
2. Stop/drain the old path, revoke its relevant access and terminate its publication processes. If a credential is shared across repositories, replace it with separately scoped access or pause all affected publishing until restriction is proven. No “old path serves the rest” exception retains a repository-wide bypass.
3. Test the old direct publication path using its previously valid capability in the qualified disposable environment: it must fail. Provider-issued credentials that cannot be revoked remain a hold until expiry or an independently enforced boundary denies them. Disabling the queue or changing SQL generation is not this proof.
4. Reconcile already-submitted provider requests. Unknown outcomes hold conflicting publication even after token expiry; revocation cannot undo an accepted request. Import necessary state, preserve evidence identities and assign a fresh authority incarnation.
5. Grant the new role-bound publisher and open repository verifier admission. Observe a full result through the evidence-PR/review path, API and cockpit. The runtime cannot grant itself operator authority.

**Later partial cohorts are conditional.** They may be introduced only after every old/new publication caller uses the same role-bound admission boundary, with no independently usable legacy credential. Qualification must show transferred work refused on the old path and retained work still admitted under the chosen partition. This extra migration is not a prerequisite for the first repository-wide cutover.

For cross-cell transfer use the same sequence plus the exclusive repository assignment update; independent cell-local generations cannot authorize the move. Rollback also freezes publication, reconciles new effects, revokes the new authority and grants a fresh incarnation to a compatible old path. If old state cannot represent the new result, hold and repair forward. Copying an older database never restores publication permission.

During contract substitution, old callers may import pure new libraries or call an isolated API. They must not bypass its custody or become a second writer. Adapters have a named consumer and removal condition. Permission to break commands lets us omit unnecessary shims, but installed adopters, skills, scripts and pinned releases still need an explicit migration notice or unsupported-version failure.

## Reconcile these plans as one program

Use semantic ownership as the consolidation boundary. Existing stream names may remain useful reporting groups. The following are proposed treatments, not amendments already made:

| Public capability / source | Treatment |
|---|---|
| desktools-v2 and forge-neutral adapters | Preserve custody, scoped reads and typed effects; drop obsolete parser/command work only after identifying its consumers. |
| statusgen evaluation and board generation | Extract pure meaning once; keep existing board single-writer ownership until its own approved cutover. |
| graph-execution, loopadmin, drainloop and desk execution engines | Inspect implemented contracts first; converge runner ownership/recovery while keeping optional workflow progression separate. |
| Verification, claims and review publication | One applicability and authority contract, with per-provider reconciliation and a shared regression suite. |
| Context, graph and authoring | One versioned target/transition/discovery model. Preserve useful bounded discovery work and separately scope the larger resolver. |
| Public service, forge mirror and cockpit | One API/evaluation model; event/poll ingestion and PostgreSQL projections; thin verifier UI before broader packaging. |
| Continuing operations and quality | Keep delivery completion distinct from enduring operational benefit; add actual consumers without blocking the first verifier on the whole model. |
| Distribution, skills, configuration, communications and installed consumers | Treat these as real consumers; release/pin and migration notices accompany contract cutovers. |

Public product briefs belong here. Each adopter maintains its own deployment overlay and local plan amendments. Public implementation must not require access to an adopter's private corpus. Reconcile public contract changes and their applicable local overlays together through exact revision references, without publishing private identifiers or counts.

## Decisions already available and still needed

The requested design inputs are one host with isolated cells, command breaks where helpful, coordinated reconciliation of unfinished plans, and a public service/cockpit. PostgreSQL coordinates in-flight work; useful reconstruction may lose explicitly classified transient data and repeat safe work. Git retains canonical intent and published evidence for this phase. The active verifier may publish through its governed role-bound path.

The proposed mechanisms requiring architecture approval include exclusive repository write assignment, a repository-wide first verifier cutover and operator-controlled fresh authority on reconstruction. None is a claim about deployed enforcement.

Before dispatch, settle the first-slice owners, exact implementation and affected-plan revisions, remaining interface details, and a bounded estimate/investment ceiling. Before activation, settle the deployment/custody profile, recovery and freshness bounds, transient-loss classes, backup policy, forge retention/receiver scope and measured success thresholds. The detailed activation evidence must be produced by implementation; it is not a prerequisite to starting authorized implementation.

Measure the legacy flow before cutover using comparable work classes. Prioritize operator repair effort; report holds, repeated runs, latency and cost. Stop the pilot on incorrect acceptance, unauthorized publication or loss of required evidence. Select a real-obligation sample and observation window before activation; do not infer reliability or savings from an arbitrary sample or exclude failures to improve the denominator.

This proposal sets no numerical budget, promises no net effort saving and closes no issues or briefs. Complete the affected-plan amendment bundle and executable verifier briefs through the [delivery sequence](16-delivery-sequence.md), using a bounded manual context packet until the automated path is qualified.
