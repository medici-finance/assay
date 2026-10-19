# Delivery sequence and remaining execution planning

**Draft decomposition, 10 October 2026.** These are focused PR packages, not newly
allocated brief IDs, assigned work or a fixed PR-count promise. Reuse or amend an
existing brief when its inspected scope fits. Do not create a parallel owner just
because the target introduces a new package name.

## Architecture, then authoring support

1. **Approve the product target and explicit contract deltas.** Record the exact
   revision, decision scope and unresolved details. Existing approval procedures
   remain authoritative. Approval does not activate the pilot.
2. **Implement the shared context packet and authoring support in public Assay.**
   The [public delivery scope](18-authoring-context-delivery.md) identifies canonical
   skill homes and the schema, resolver, validation and release work.
   Keep these in one PR if the qualified slice is reviewable; otherwise split into
   schema/synthetic fixtures, offline resolver/receipt checks, then skill integration.
   Both skills consume the same target and discovery binding. They must not call an
   interface that has not shipped. Qualify a real spec-to-amendment journey before
   claiming automated dispatch use.
3. **Reconcile the first affected work set.** Produce actual patches to public
   briefs and a revision-bound amendment manifest. Adopters make their corresponding
   local amendments. Review active-work checkpoints and protect populated Evidence
   and dated source accounts. An inventory row saying “amend” does not fulfill this.

Use a manually assembled packet with explicit coverage and bounded authorization
to bootstrap the first implementation. The future resolver is not a prerequisite
to building itself. Nor does an unapproved competing proposal freeze approved work.

## Independently useful adoption lane

Extend the [same owners](19-progressive-adoption-and-oversight.md#delivery-changes-to-the-existing-program)
with a smaller product lane. These are scoped amendments to existing packages,
not new allocated briefs or another implementation home:

| Slice | Existing owner/package | Prerequisite and acceptance |
|---|---|---|
| Pure adoption/oversight contracts | R1, definitions/policy/evidence; existing install-contract owner | Approved bounded meanings and actual module/import closure; neutral positive/negative fixtures |
| Inspect and advisory Assist | Offline definition/evidence adapters, existing authoring/CLI surfaces | Selected pure contracts and supported pins; cold-host first value and second task without full runtime |
| Selected installation and actionable output | Cell-operation/config and projection owners | Versioned dependency closure, preview/resume/no-clobber, identical assurance/hold meanings across consumers |
| Managed oversight | R1 policy; R2 pending decisions; R3/R4 authority; R5/R7 result flow | Full current runtime/custody/effect qualification; presets cannot bypass organization requirements |
| Sampling/batch delegation and scale | Existing decision, runtime, effects and operations owners | Separately approved scoped contract, anti-evasion/expiry/recovery cases and required capability evidence |

Read-only and advisory slices need neither R2–R8 nor the complete future-state resolver
merely to be implemented and qualified. Use the bounded manual context bootstrap.
Their libraries do not acquire provider effects, standing ownership or SQL runtime
state. Managed writer integration retains the R1–R8 dependency and activation sequence.
Notification preferences can precede sampling/authority changes because they preserve
the pending gate. Never wire an installer, skill or cockpit to an unshipped interface.

## First verifier implementation packages

| Package | Delivers | Dependencies | Proof required for its claim |
|---|---|---|---|
| R1: domain contracts and regression floor | Qualified identities; fact completeness; candidate/acceptance binding; pure eligibility/evidence interfaces; old/new fixture adapters | Approved interface scope and manually or automatically reconciled work packet | Selected existing semantic cases run against named implementations; intended differences reviewed; missing facts and wrong subjects cannot pass |
| R2: PostgreSQL coordination and reconstruction | Attempt/lease/effect-intent transactions; migration schema; reconstruction inputs; operator incarnation binding | R1 | Real database restart, deletion and backup-rollback tests; canonical obligations recovered; unknown usage/effects remain visible |
| R3: isolated runner and cell authority | Existing runner protocol adapted to real disposable execution; role separation; exclusive repository assignment and process containment | R1; can develop beside R2 | Actual runner and capability positive/negative cases; candidate cannot reach publisher or operator credentials; duplicate cell assignment refused |
| R4: role-bound publication and reconciliation | Stable effect identity; current authority checks; evidence-PR submission; lost-acknowledgment handling | R1–R3 | Stale capability refused at effect boundary; already-submitted ambiguous effects reconciled; publication cannot grant acceptance or a human decision |
| R5: complete verifier use case | Resolve → admit → run → assess → publish/reconcile → project, with cancellation and recovery | R1–R4 | Complete disposable journey, crashes at durable transitions, independently selected acceptance and no bookkeeping-triggered rerun loop |
| R6: forge mirror and shared observations | Scoped PR/MR/issue reads; persisted freshness/coverage; conditional refresh; polling repair and authenticated event adapter where enabled | R1–R2; can develop beside R3–R5 | Pagination, lost/reordered events, permission loss and partial scans preserve uncertainty; client outage cannot cause a poll storm |
| R7: public API and thin cockpit | Same work/evidence/hold semantics for human UI and clients; freshness and recovery visibility | R5–R6; interface work may begin earlier | Actual mounted-route authorization; cross-cell/private isolation including counts; reconnect cannot launch work or imply acceptance |
| R8: shadow, migration and retirement tooling | Read-only comparisons; bounded recovery/handover rehearsals; legacy publisher inventory; migration notices and adapters with removal conditions | R5–R7 | Old capability refusal, repository-wide handover and rollback in disposable environment; classified old/new differences; required history remains readable |

### Operating obligations within these packages

| Package | Added obligation |
|---|---|
| R1 | Resource/retirement identities, reevaluation generations, guard events and distinct execution/acceptance/retirement states; contract fixtures for the new boundaries. |
| R2 | Durable pending feedback and retirement progress, transaction/outbox handoff and safe reconstruction of unknown resources. |
| R3 | Runner guard events/budgets, resource capacity and custody; preserve the actual isolation and network-control guarantees of the chosen profile. |
| R4–R5 | Keep duplicate-effect protection at the role-bound effect boundary; integrate resumable retirement, reevaluation and the sustained-work qualification into the complete flow. |
| R6 | Translate scoped refreshed feedback into durable reevaluation; preserve distinct source records and incomplete coverage. |
| R7 | Show execution, acceptance, pending feedback, resource holds and maintenance health independently through the same authorized API. |
| R8 | Preserve and map the recent hygiene regressions in the migration inventory; qualify one maintenance owner per common Git store, bounded tasks and release/pin migration. |

Basic safe resource release, capacity and recovery apply from the first runner; R8 is the migration/retirement gate, not permission to postpone those safeguards. Object-pack scheduling is a separately qualified extension of the same cell-operation owner, required before activating a profile that relies on it. Learned-context curation follows the context-adapter delivery lane and does not block R1–R8. No new scheduler, workflow engine or Git host is selected by these additions.


The runtime integration path is R1 → R2/R3 → R4 → R5 → R7 → R8, with R6 feeding
R7. This is a dependency sequence, not an elapsed-time estimate. Later feature
work does not have to wait for every optional graph or UI capability.

After R8, **activation is a separate operator act** under the approved deployment
and custody profile. Pause the entire selected repository's legacy verifier
publication, reconcile old effects, revoke/restrict credentials and minting access,
rotate authority, then admit the new verifier. Retain review/merge and human gates.
Observe evidence publication and cockpit state; apply the agreed stop criteria.
Live infrastructure is never needed for the development regression suite.

## Continue by flow and retire by consumer

Move review, worker and intake/coordinator flows through the same contracts in
separate reviewed slices, ordered by readiness and actual dependency. Add continuing
operations and richer quality consumers without redefining delivery acceptance.
Keep urgent fixes on every supported path; route new features to the approved target.

An early continuing-operations slice is **workspace lifecycle completion**: teardown at owning-attempt completion in dispatch; allocation verbs that stamp class and retention for every fleet-created resource, including scratch; class-aware retirement and forge-state eligibility in the sweep; lock-lifecycle retirement; consolidation of repository-local sweepers behind the single maintenance owner; and the supervised maintenance cadence with its per-OS keep-alive profiles. It depends on the approved retirement and maintenance contracts, ships behind explicit flags on current tools where possible, and proves itself through the sustained-work workload retiring disposable resources while held reasons stay explainable.

Each slice carries the old entry points, new contract versions, supported version
combination, state/evidence mapping, cutover action, rollback limit and removal
condition. Old callers may use pure new libraries or isolated APIs when useful.
Delete an adapter only after the installed-consumer inventory, release/pin migration
and recovery window support that removal. Command breaks need clear failure or
migration instructions rather than a silent fallback to a different state store.

## Work required before dispatch and activation

For each package, inspect the latest implementation and existing unfinished briefs;
select the owning brief(s), delivery paths, contract version, dependencies, test
commands, active-work treatment and effort estimate. Apply the declared brief
amendments rather than merely linking an impact report. Completed work remains
history and potential reusable implementation.

Before activation, name the repository/profile, credential revocation inventory,
accepted loss classes, recovery/freshness objectives, backup/retention policy,
observation window and success/stop thresholds. Measure the old flow first.
Repository selection and private operational details belong to the adopter's plan;
the public qualification and migration mechanisms belong here.

The architecture reduces duplicate semantic and runtime owners by design, but adds
database operation, migration, discovery and qualification work. Net savings remain
a hypothesis until the remaining implementation is sized and measured. A brief
retirement count is not an engineering-effort estimate.

## Portfolio and drift delivery — 13 October 2026

The [product-intent and portfolio amendment](26-product-intent-and-portfolio.md) supplies a parallel offline lane with six authored briefs:
statusgen/22–25 and continuing-operations/01–02, plus amendments to statusgen/20–21.
It leaves statusgen/19 (merged in PR #2763) and its Evidence unchanged. The dependencies and remaining parked-stream
admission are explicit there. This lane does not gate R1–R8, choose an adopter pilot,
activate a scheduler or add a second database backend. R7 consumes its qualified view API.

## Scion-derived forge-service extension — 14 October 2026

The [forge-service amendment](22-forge-service-and-source-queries.md) extends R6 alongside
R4 and the read portion of R7; it is not a second mirror or authority program. Reuse the
forge-neutral/37–38 briefs and the forge-neutral/39–45 briefs merged in PR 2743 (merge commit
`6b479a573`). Those briefs cover typed service contracts,
authenticated serving and role-executor custody, write-through receipts, exact-commit source
queries, incremental derived indexes, a shared-Git reuse trial, and served/recovery/cutover
qualification. Their source contracts can be tested offline; the managed writer still needs
R1–R4 and the mirror store before activation. The public cockpit and remote OIDC ingress
remain R7 work, consuming the same authorization and observation contract.

Do not make a shared-object daemon or broad source checkout redesign a prerequisite for the
first mirror/service. The reuse trial is a bounded go/no-go comparison, while request coalescing
and bounded upstream fallback are already mirror obligations. Necessary PR branches and the
existing candidate/retirement owners stay. Cross-provider qualification must cover operation
unknowns, admission-bound private data and exact candidate currency, not only read speed.


## Public runtime contract gaps — 14 October 2026

The [runtime alignment](23-runtime-contract-alignment.md) maps prior requirements onto
existing R1–R8 owners. Add graph-execution/30 (complete evaluation context/time validity)
and /31 (accountable-principal and proof provenance) after /28, independently at wave 3.
They add no second runtime or signer. /14 consumes both, /19 consumes /30 and /25 consumes
/31 through dated Task/Verify amendments; existing human activation gates remain.

The real head is still independent verification of /02 before /09 -> /28. /30–31 contract
fixtures do not need a deployed PostgreSQL controller; their managed consumers still require
their existing R2–R5 prerequisites and separate activation proof. R3–R8 package amendments
remain with their existing owners; this revision does not mark their implementation complete.

## Communications extraction and existing-work amendments (proposed; pending approval)

Proposal against the ratified revision `23f655f5d`. The R2, R7 and R8 edges below are not in the amendment manifest's rows or dependency path until the proposal is approved and the manifest is reconciled.

The [communications target](24-communications.md) and [comms-core briefs](../../streams/comms-core/README.md)
add a concrete extraction path: 01 contract/policy → 02 server/file backend → 03 client;
04 shared configuration follows 02 and desktools-v2/16. Existing desktools-v2/42 now
depends on 02/03 and remains the sole owner of Cobra migration for the comms commands.
This is actual brief amendment work, not an inventory-only promise. Adopters reconcile
their own unfinished mode/activation plans against these public interfaces.

R1 consumes the versioned message/receipt contract only where its flow sends messages.
R2 owns transactional SQL outbox and business inbox deduplication; it does not implement
a second durable broker. R7's comms host integration follows 02 and a qualified public
service/custody seam; its unrelated UI work stays independent. R8's messaging cutover
requires selected-broker proof and an accepted/held/acked state reconciliation manifest.
Broker qualification, adapter, host composition and cutover need subsequent bounded
briefs; the four extraction briefs neither select a broker nor enable native firing.

## CI and directory migration lane

The [CI extension](25-ci-tooling.md) adds a separately deliverable lane: directory/staging
contract → public shared libraries → in-place consumer cutover → qualified command moves
and action adoption → source retirement. Build the first reusable actions before or beside
the first workflow cutover. The [ci-tooling stream](../../streams/ci-tooling/README.md)
owns this decomposition; it does not wait for the runtime controller. R8 retains the
desk/operator/service relocation work routed by the layout inventory and must qualify
module visibility, authority and lifecycle before retiring their old paths.

## Execution-environment delivery — 16 October 2026

[Agent-execution](../../streams/agent-execution/README.md) extends the R3 runner boundary:
/01 profile contract then /02 preparation receipt can proceed over implemented runner v1;
/03 follows the existing supervisor/first adapter, /04 follows actual controller/publication,
and /05 follows their recovery proof and standing clients. The independent /06 AX feasibility
report neither gates the local path nor activates a remote service. Use its typed dependency
waves and existing R1–R8 owners; do not duplicate their implementations to accelerate a demo.

## Verifier pilot sequencing proposal — 19 October 2026

Consume [the verifier migration amendment](../../streams/graph-execution/verifier-migration.md).
An adopter may move evidence production through its existing qualified owner before R2–R8,
and evaluate shared assessment in shadow after R1. This grants neither a second standing
claim authority nor a publisher. Managed completion retains R1–R8 and repository-wide
handover. The proposed Kubernetes profile is agent-execution/08; its stream remains parked.
