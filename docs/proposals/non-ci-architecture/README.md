# Unified non-CI architecture for Assay

**Status: future architecture we are moving towards. Updated 14 October 2026; includes portfolio, drift and the write-through forge-service amendment.** This directory is the canonical public home of Assay's future product architecture. It states the direction of travel, not what is already implemented or deployed. Detailed contract changes still require revision-bound review; implementation and activation require their own evidence and authorization. Publishing or merging this documentation does not activate a runtime, grant credentials or amend existing briefs.

Read the [implemented foundations and remaining work](11-target-architecture.md#implemented-foundations-and-work-still-to-build) alongside the target. Portfolio allocation, target/context drift and continuing observation are included in the destination; their integrated automation remains work to build and qualify.

The maintainer ratified the whole target at revision `23f655f5d`, packages R1–R8, on [medici-finance/assay#2500](https://github.com/medici-finance/assay/issues/2500). That approval remains revision-bound: it does not imply that the portfolio/drift additions below were included in that earlier revision, or that any runtime has been activated.

Assay's operator CLI (`cellctl`), governed work verbs (desk-tools), definition and
evidence engine (`statusgen`), quality tools, service and cockpit share meanings
and authority boundaries. The target refactor gives each meaning one owner,
then migrates consumers through shared contracts and complete flow cutovers.
Verification is the first end-to-end test.

| Document | Purpose |
|---|---|
| [Target architecture](11-target-architecture.md) | Domain ownership, portfolio/drift, implementation boundaries, public service/cockpit and disposition of existing tools |
| [Future-state model](12-future-state-model.md) | Observed, approved, transition and proposal views; shared discovery and authoring handoff |
| [Migration and decisions](13-migration-and-decisions.md) | Supported-path fixes, verifier transaction, cutover, rollback and unresolved decisions |
| [Storage, forge mirror and graph](14-storage-forge-graph.md) | PostgreSQL, reconstruction, authority after data loss, event/poll ingestion and queries |
| [Regression plan](15-regression-from-day-one.md) | Existing public test sources and new runtime/authoring qualification cases |
| [Delivery sequence](16-delivery-sequence.md) | Focused implementation packages, dependencies and retirement obligations |
| [Shared context graph](17-context-substrate.md) | Cell classification, composition, source ownership, snapshot/query and visibility contracts |
| [Authoring/context delivery](18-authoring-context-delivery.md) | Existing public skill homes, proposed resolver/skill packages and independent context slices |
| [Progressive adoption and oversight](19-progressive-adoption-and-oversight.md) | Smaller independently useful entry paths, scoped human involvement, assurance and package/consumer changes |
| [Amendment manifest](20-amendment-manifest.md) | Owners, paths, dependencies and active-work treatment for the ratified packages; brief patches for R1 and step 2 |
| [Forge service and source queries](21-forge-service-and-source-queries.md) | Write-through visibility, caller/executor authorization, immutable source caches and Scion-inspired qualification |
| [Synthetic change set](examples/architecture-change-v0.yaml) | Illustrative records with no approval or implementation claim; not a supported schema |

## Future direction and mechanisms to qualify

The design starts with one operator-managed host running multiple isolated cells.
Command compatibility can break when that simplifies the result. Approved
architecture changes can reconcile unfinished plans together while preserving
completed evidence and historical source accounts. Public Assay owns the reusable
service, cockpit, operator tools, schemas, libraries and skills.

The canonical [author-spec](../../../plugins/assay/skills/author-spec/SKILL.md) and
[author-brief](../../../plugins/assay/skills/author-brief/SKILL.md) are already public.
The proposed context-graph/resolver and skill improvements also belong here; this
architecture specifies their delivery but changes no skill body or executable interface.

PostgreSQL coordinates in-flight work and holds query projections; reviewed Git
records retain canonical intent and published evidence for this phase. Recovery
should reconstruct useful state, allowing declared transient loss and safe repeated
work. Forge records are mirrored with attributed coverage and freshness. The
[forge-service amendment](21-forge-service-and-source-queries.md) routes authorized writes
through existing role-bound executors and projects confirmed results immediately; it also
provides SHA-bound source queries and scoped cache reuse. Active
verification publishes through a governed role-bound path.

The ratified target includes one mutating cell per external repository, separate
effect executors, fresh operator-controlled authority on reconstruction, whole-repository
verifier cutover and a bounded discovery/receipt interface for both author skills.
Implementation and activation still require the corresponding contract and runtime
qualification. A graph explains relationships; it never grants runtime authority.

## Scope and public boundary

This covers non-CI product tooling, including installation/configuration,
communications, monitors, role clients and support tools that share the affected
contracts. Existing CI is touched later only where needed to run the selected
regression suite; redesigning CI is not this program's objective.

Adopters retain deployment configuration, secrets references, private policy
overlays, incident evidence and local brief reconciliation. They may reference
this public design. Public contracts and tests must stand alone without those
private sources. Private content, identifiers and counts must not leak through
queries, coverage diagnostics or generated public artifacts.

## Evidence and currentness

The source inventory is pinned to public Assay
[`f9327a5954d57e786464e244a8e9c06f695d980d`](https://github.com/medici-finance/assay/tree/f9327a5954d57e786464e244a8e9c06f695d980d).
The [public vision proposal](https://github.com/medici-finance/assay/blob/466c36a794bad4e837c61fe1a3335572bd458e15/docs/vision/README.md)
is design input, not proof of delivered capability. Existing
[contracts](../../contracts.md), [library-first rules](../../library-first.md),
[spec lifecycle](../../../spec/lifecycle-v1.md#8-spec-and-scoping-doc-lifecycle)
and role authority remain controlling until their specific amendments are approved.

This proposal incorporates review concerns about overlapping cell ownership,
legacy publication credentials, recovery from rollback, incomplete consumer
discovery, historical provenance and confusing approval with activation.
The concrete responses and qualification cases are included here; no inaccessible
review record is required to understand them.

The source snapshot is not a claim about current deployments. Refresh affected
code, plans, active claims and releases when authoring each delivery brief. Tests
linked here are reuse candidates; a mapped issue, passing legacy test or drafted
target is not evidence that the replacement is implemented or its consumers migrated.

## Approval and execution

Build from the ratified target and review subsequent contract deltas against their exact
revisions. Deliver the shared
context-packet/resolver and author-skill changes, initially bootstrapped from a
manually inspected packet. Follow with focused runtime and migration PRs.
Independent runtime implementation can proceed against approved contracts while
authoring automation is completed.

The [delivery sequence](16-delivery-sequence.md) and [amendment manifest](20-amendment-manifest.md)
record the ratified packages and their reconciliation. The manifest distinguishes
applied R1/authoring-support brief amendments from follow-up owner rows. Further
delivery still needs dispatchable briefs, named implementers, estimates and applied amendments. This architecture update neither closes existing work nor claims quantified savings.
Runtime proof gates activation; it must not become a circular prerequisite to
authorizing the implementation that produces that proof.

## Progressive adoption and oversight amendment

The [12 October amendment](19-progressive-adoption-and-oversight.md) extends the same
semantic owners with independently usable Inspect/Assist entry paths and scoped human
involvement. Offline contracts do not require the running PostgreSQL cell. Managed
execution keeps its custody, transaction, fencing and activation boundaries.

Adoption journey, oversight policy, credential topology and deployment qualification
are independent choices. Presets expand into explicit requirements; evidence depth,
action authority and notification preferences remain distinct. AssayScore measures
outcomes and can inform tuning recommendations; it never enacts a grant.

The delivery sequence now includes the smaller product lane beside R1–R8. This is a
draft architectural delta, not released settings, an implemented installer or an
approved change to current lifecycle/authority. Existing records and source accounts
remain preserved. Architecture, implementation and activation approval stay separate.

## Operating research incorporated in this revision

The proposal now includes resumable resource retirement, bounded repository maintenance, durable feedback reevaluation, observable execution guards, governed learned context and separate cockpit states. These extend existing domain owners and R1–R8; implementation and activation remain separate. A follow-up operating revision adds allocation records at creation, class-aware detached-review retirement, forge-state branch eligibility, an explicit lock lifecycle, a supervised maintenance cadence with per-OS keep-alive profiles, lifecycle policy hooks, and the matching preservation obligations and qualification cases.

Public research inputs, inspected 10 October 2026:

- [langgraph-harness at 95368b4](https://github.com/vrajpal-jhala/langgraph-harness/tree/95368b4ef964b6fff82d0d54572e558d98d693c9): tool-call guards, pending reruns, memory curation and worktree lifecycle are useful implementation examples. Its framework, model-assessed ready transition and process-local ownership assumptions are not adopted. Source inspection is not a reproduced runtime test.
- [Cursor: Git at any scale](https://cursor.com/blog/git-at-any-scale): storage layout, maintenance cost and reconstruction motivate qualification. Its hosting architecture and vendor throughput claims are not adopted or independently validated here.
- Recent public Assay fixes are mapped in the [migration obligations](13-migration-and-decisions.md#preserve-recent-repository-hygiene-behavior), with new [continuous-work qualification](15-regression-from-day-one.md#continuous-work-and-resource-qualification).

PostgreSQL remains the operational transaction owner; reviewed Git records retain canonical intent and published evidence. These lessons introduce neither a mandatory Git append per operation nor a replacement authoritative store.


## One public runtime — 14 October 2026

The [runtime alignment](22-runtime-contract-alignment.md) reconciles earlier refs-first
ideas into the existing R1–R8 implementation in public Assay. PostgreSQL coordinates
runtime effects; Git retains definitions/evidence; the forge service owns scoped provider
observations and effects. There is no second runtime, native SCM prerequisite or new signer
deployment. The coverage table maps existing owners and routes only two new contract gaps
(graph-execution/30–31), with amendments to their existing admission/store/publication consumers.
