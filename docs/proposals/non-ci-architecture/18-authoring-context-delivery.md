# Public delivery scope: author skills and context graph

**Draft execution scope, 10 October 2026.** These packages belong in public Assay.
They are not implemented by this proposal and are not allocated, dispatchable
briefs. Approve applicable contracts, inspect existing work, then amend or author
the owning public briefs with exact paths, dependencies and runnable checks. Do not
create a duplicate stream merely because a proposal names a package.

## Existing skills and proposed additions

The canonical [author-spec](../../../plugins/assay/skills/author-spec/SKILL.md) and
[author-brief](../../../plugins/assay/skills/author-brief/SKILL.md) already live here.
Their existing consumer inspection, provenance and contract-ownership requirements
are the baseline. No skill body is changed by this architecture PR. An older
installed version or house copy is a release/adoption concern, not another
implementation home.

The addition is one reproducible context operation and typed spec-to-brief handoff.
That requires schema, discovery, receipt and amendment checks as well as skill
instructions. Prose telling both skills to consider the future cannot deliver it.

## Focused public packages

| Package | Concrete deliverable and routing | Dependency and proof |
|---|---|---|
| Context/plan contracts | Versioned machine-readable target, discovery receipt, impact and amendment-manifest contracts; neutral examples and negative fixtures. Extend `schemas/`, `spec/` and existing semantic owners. | Approved scope; use the [three-part contract pattern](../../contracts.md). Exact versions/parser ownership belong in implementation briefs. The proposal YAML is not a production schema. |
| Offline future-state resolver | Importable, effect-free operation over explicit authorized snapshots; qualified IDs, declared relationships, independent directory enumeration, deterministic prose-candidate discovery, limits and coverage. Source adapters remain separate. | Contract fixtures; exact membership, query versions and positive/negative results. No service, PostgreSQL or private corpus required. Reuse graph/parser ownership within import boundaries. |
| Receipt and amendment validation | Canonicalization; membership/relevance rechecks; base/patch binding; protected Evidence and dated sources; active-work checkpoints; compatible target promotion. | Resolver and contracts; distinguish unrelated edits, new consumers outside the read set, missing inputs and conflicting approved deltas. Schema validity alone is insufficient. |
| Canonical skill integration | Update both public skill bodies to consume the same shipped operation and handoff. Specs emit actionable obligations; briefs produce candidate amendments and their manifest. Preserve dependency/consumer checks. | Available interface and supported-version behavior; qualify spec → handoff → amendment. Keep the bounded manual bootstrap explicit while automation is unavailable. |
| Release and adoption support | Public release notes, compatibility documentation and neutral migration examples. | Qualified schema/resolver/skills combination; adopters separately update pins, overlays and active work. Publishing source does not update installed skills. |

The first three packages may share a PR if reviewable; otherwise split in dependency
order. Integrate skills when their required interface is available. This follows
architecture approval and precedes broad automated reconciliation. Independent
runtime implementation can use a bounded manual packet against approved contracts.

## Context work that retains its own scope

The [shared context design](17-context-substrate.md) also covers these independently
bounded capabilities. Reconcile existing commitments before assigning them; do not
silently absorb them into the future-state resolver:

- **Offline artifact index:** explicit roots, initially `go.mod` and tool pins,
  declared dependency semantics, bounded inverse/forward queries, exact source pins
  and synthetic two-repository fixtures. Refuse dirty/changing/malformed inputs,
  root escapes and duplicate identities. Retain declared replacements without
  following arbitrary replacement paths. Publish deterministic, owner-readable
  local output atomically; failed builds preserve prior snapshots. No network,
  hooks, automatic fleet discovery, real committed indexes or implied resolved
  build list.
- **Optional cell catalog/classification:** versioned profile and compatibility
  migration from current topology/cell configuration, synthetic multi-cell profiles,
  public/private composition and grouping. No new authority, mandatory corpus
  rewrite or taxonomy prerequisite for the first verifier authoring operation.
- **Service and cockpit consumers:** public adapters and views after producer
  contracts ship. Preserve client/evaluation ownership and release dependencies;
  expose completeness and authorization without moving execution into the graph.

Their implementation and neutral regression fixtures belong here too. Adopter-only
workflow promotion, real source catalogs and private validation overlays do not.

## Acceptance of the authoring slice

Use the [authoring qualification cases](15-regression-from-day-one.md#authoring-and-plan-evolution-qualification),
including withheld synthetic consumers, new consumers without README/typed edges,
competing proposals, unavailable sources, active workers, delivered implementations
and historical-source corruption. Apply candidate amendments to fixture bases and
check task, dependency, delivery and Verify deltas plus preserved history. Bind
results to actual parser/resolver/skill revisions; manual scenarios are not executed
implementation gates.

The public suite runs without private repositories, credentials or operational
data. Private diagnostics can supply additional adopter evidence without becoming
a dependency or a general recall claim. Report the fixture suite's coverage limits.

No product implementation is assigned to an adopter repository by this design.
Moving an existing plan requires a source-identity/successor mapping, active-work
reconciliation and reviewed lifecycle changes. This table does not claim that a
transfer, retirement or implementation has occurred.
