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
