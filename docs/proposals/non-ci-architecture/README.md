# Unified non-CI architecture for Assay

**Status: draft proposal, 10 October 2026.** This directory is the public home of
the proposed product architecture. Approval must name its reviewed revision and
scope. Publishing or merging these documents does not activate a runtime, grant
credentials, amend existing briefs or implement an authoring interface.

Assay's operator CLI (`cellctl`), governed work verbs (desk-tools), definition and
evidence engine (`statusgen`), quality tools, service and cockpit share meanings
and authority boundaries. The proposed refactor gives each meaning one owner,
then migrates consumers through shared contracts and complete flow cutovers.
Verification is the first end-to-end test.

| Document | Purpose |
|---|---|
| [Target architecture](11-target-architecture.md) | Domain ownership, authority, public service/cockpit and disposition of existing tools |
| [Future-state model](12-future-state-model.md) | Observed, approved, transition and proposal views; shared discovery and authoring handoff |
| [Migration and decisions](13-migration-and-decisions.md) | Supported-path fixes, verifier transaction, cutover, rollback and unresolved decisions |
| [Storage, forge mirror and graph](14-storage-forge-graph.md) | PostgreSQL, reconstruction, authority after data loss, event/poll ingestion and queries |
| [Regression plan](15-regression-from-day-one.md) | Existing public test sources and new runtime/authoring qualification cases |
| [Delivery sequence](16-delivery-sequence.md) | Focused implementation packages, dependencies and retirement obligations |
| [Shared context graph](17-context-substrate.md) | Cell classification, composition, source ownership, snapshot/query and visibility contracts |
| [Authoring/context delivery](18-authoring-context-delivery.md) | Existing public skill homes, proposed resolver/skill packages and independent context slices |
| [Synthetic change set](examples/architecture-change-v0.yaml) | Illustrative records with no approval or implementation claim; not a supported schema |

## Requested direction and proposed mechanisms

The design starts with one operator-managed host running multiple isolated cells.
Command compatibility can break when that simplifies the result. Approved
architecture changes can reconcile unfinished plans together while preserving
completed evidence and historical source accounts. Public Assay owns the reusable
service, cockpit, operator tools, schemas, libraries and skills.

The canonical [author-spec](../../../plugins/assay/skills/author-spec/SKILL.md) and
[author-brief](../../../plugins/assay/skills/author-brief/SKILL.md) are already public.
The proposed context-graph/resolver and skill improvements also belong here; this
PR specifies their delivery but changes no skill body or executable interface.

PostgreSQL coordinates in-flight work and holds query projections; reviewed Git
records retain canonical intent and published evidence for this phase. Recovery
should reconstruct useful state, allowing declared transient loss and safe repeated
work. Forge records are mirrored with attributed coverage and freshness. Active
verification publishes through a governed role-bound path.

The detailed mechanisms remain proposals: one mutating cell per external
repository, separate effect executors, fresh operator-controlled authority on
reconstruction, whole-repository verifier cutover and a bounded discovery/receipt
interface for both author skills. They need approval and qualification at the
appropriate phase. A graph explains relationships; it never grants runtime authority.

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

First approve the target and explicit contract deltas. Then deliver the shared
context-packet/resolver and author-skill changes, initially bootstrapped from a
manually inspected packet. Follow with focused runtime and migration PRs.
Independent runtime implementation can proceed against approved contracts while
authoring automation is completed.

The [delivery sequence](16-delivery-sequence.md) is a proposed decomposition.
Dispatchable briefs, named implementers, estimates and applied amendments are
still required. This PR neither closes existing work nor claims quantified savings.
Runtime proof gates activation; it must not become a circular prerequisite to
authorizing the implementation that produces that proof.
