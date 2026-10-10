# A traversable model of present, future and transition

**Draft proposal, 10 October 2026.** Extend existing graph and consumer-routing capabilities with a common architecture-change model. Approved changes may reconcile affected unfinished plans together, while completed history remains intact. This document changes no live brief or skill; its interface and schema are proposed.

## The missing capability is an architecture change model

Yes, we need a graph, but adding links between today's briefs is insufficient. Authors need a **versioned architecture model with approved targets, explicit transition states and proposed change sets**. A query must answer both “what consumes this today?” and “what is already committed to consuming its replacement?” It must distinguish a declared plan from implemented or deployed evidence.

There is already useful foundation:

- [statusgen's graph export](https://github.com/medici-finance/assay/blob/f9327a5954d57e786464e244a8e9c06f695d980d/statusgen/graph.go) derives stream/brief/finding/intake/issue relationships, including depends, unblocks, affects and sources. Its current node records are ID/type/label; it is not a versioned future architecture model. It is a per-root export, so cross-repo composition needs qualified resolution. Dangling edges are currently omitted rather than expressed as unresolved architectural obligations.
- Component/contract relationships, provenance, snapshots, bounded queries and serving integration should share the existing graph and planning ownership. A cell classification describes topology; it does not grant a capability. Reconcile existing scoped graph work before extending it.
- `author-spec` already requires inspecting affected consumers and unfinished work. `author-brief` already carries provenance, dependencies, design-fit and consumer routing; its consumers check enforces parts of declared routing. The gap is **complete-enough discovery, a selected future baseline, impact closure and selective invalidation**, not an absence of any dependency discipline.

The existing semantic-owner register remains the home of ownership declarations. Move its authoritative fields into structured records or structured blocks there, with a generated readable view. Do not maintain a manually edited YAML owner list alongside contradictory Markdown. Target overlays describe intended changes to those same identities; the context index is derived from these sources.

## Four views, not one ambiguous status field

| View | What it means | Evidence required |
|---|---|---|
| Observed | What source implements, releases contain, or a particular cell has deployed | Separate source SHA, build/release provenance and deployment observation. Code on main is not a deployed assertion. Unknown remains explicit. |
| Approved target | The chosen architecture under a named immutable revision | Approved contracts, ownership, allowed dependencies, invariants and decision references. Approval is not implementation. |
| Transition | A supported intermediate combination for a cohort/profile | Version compatibility, owner for each migrated resource, entry/exit conditions, migration/recovery steps and retirement obligations. |
| Proposal | A possible delta from a selected baseline | Rationale, alternatives, assumptions, impacted plans and unresolved decisions. It cannot silently become the target. |

Several proposals can coexist. A cell/adopter can target a particular approved revision rather than “latest.” Milestones select coherent subsets of the target. Calendar dates alone do not make a transition valid. Compare **observed ↔ target**, **current target ↔ proposed target**, and **transition ↔ producer/consumer versions** separately.

The target is not a UML diagram pretending to be executable code. It is a small set of consequential contracts and relationships, with narrative design attached. Begin with the verifier slice; expand as changed contracts require it. Do not annotate every function or backfill the entire brief corpus before the first useful query.

## Records and relationships

| Record | Minimum useful content |
|---|---|
| Requirement / capability / invariant | Desired behavior or outcome, accountable owner, scope, source decision and contracts that satisfy or enforce it |
| Decision | Chosen option, deciding authority, applicable scope/revision, rationale and explicit supersession links |
| Component | Stable identity, purpose, owner, source locator, provided/consumed contract references, allowed dependencies and authority boundary |
| Contract version | Schema/API and behavioral invariants, semantic owner, compatibility rule, source revision, conformance cases and forbidden shortcuts |
| Architecture target | Immutable revision, exact imported contract/component revisions, accepted decisions, supported profiles and predecessor |
| Change set | Base target, desired delta, rationale, affected invariants, plan dispositions, dependencies, activation/retirement conditions and approval reference |
| Plan reference | Existing qualified spec/brief identity and revision, what contract it changes/consumes, assumed target and delivery/activation conditions |
| Transition | External mutation domain, assigned cell, old/new contract versions, cohort granularity permitted by the actual publication fence, authority incarnation, state migration, recovery and last-consumer condition |
| Observation/conformance | Subject revision, evidence reference, scope, timestamp/window and outcome including unavailable/incomplete |
| Incident / regression obligation | Repository-qualified issue identity, observed state/closure reason, canonical or superseding target, failure mechanism, contract version, owning plan, old/new test mapping and separate delivery/release/activation/verification evidence |
| Operational subject | Existing continuing-operations identity and its assessment/impact links, once those contracts land |

A new requirement starts from the capability or operational subject it changes, then follows `satisfies/enforces` relationships to contracts and planned implementations. If those relationships are absent, source inspection supplies candidate links and an explicit coverage gap; an empty contract query is not proof that the requirement is independent.

Useful typed edges are `provides`, `consumes`, `owns`, `enforces`, `persists`, `changes`, `assumes`, `realizes`, `replaces`, `migrates`, `requires` and `checked-by`. Reuse existing equivalent edge definitions rather than inventing synonyms. Each edge cites its declaring or deriving source and revision. Mark it declared, mechanically observed or inferred; an inferred edge is a discovery candidate, not an approved constraint.

Existing brief identities stay repository-qualified. Components/contracts receive stable IDs only through the shared schema; a rename or repository move has an explicit mapping. Contract version and architecture-target revision are separate. A name like `verification-result/v2` says nothing about whether an installed controller supports it.

A conformance link and a passing result are different records. A target's single semantic owner can have several deployment instances or explicitly compatible implementations. Validation rejects conflicting effective owners for the same target/version/scope, not legitimate deployment replication. Cycle rules apply by edge type: a prerequisites cycle can be invalid even though an ordinary relationship graph is cyclic.

## Storage and a concrete shape

Keep canonical definitions in reviewed Git sources. Start with YAML plus a schema and deterministic resolver; PostgreSQL holds the running service's derived traversal index, with JSONL useful for portable/offline snapshots. The [storage amendment](14-storage-forge-graph.md) distinguishes canonical Git intent/evidence from PostgreSQL working state, permits transient loss during reconstruction, and defines forge-mirror integration. A graph database, vector store and natural-language query planner are unnecessary for the first impact operation. Search/LLM assistance can nominate missing relationships, while typed queries provide the reproducible result.

The companion [machine-readable sketch](examples/architecture-change-v0.yaml) demonstrates a public source pin, a draft target, contract changes, synthetic plan obligations and migration conditions. Its `v0-proposal` format is **illustrative, not an implemented Assay schema**. The example plan identities do not resolve to real briefs. It deliberately declares a partial footprint and unresolved approvals; no tool should accept it as an executable plan. The exact names belong in the shared planning schema implementation.

A proposed authoring operation would look like this; these are interface sketches, not commands that exist today:

```text
assay context impact contract:verification-result \
  --observed <snapshot-id> --target <approved-target-revision> \
  --proposal <change-set> --include-transitions --include-plans

assay plan reconcile <change-set> --check
```

The coverage input must include the complete stream roster, with reasons for excluded non-stream directories, plus consumer paths outside those repositories. Stream labels such as education or marketing cannot remove a cockpit, installer or API consumer from discovery. Retain-behavior and retain-delivery are separate judgments: a destination, persistence or authority change requires an amendment even when the UX requirement survives. Record external-consumer and in-flight-work coverage limits explicitly.

Enumerate source directories independently of README presence and classify streams versus record registers. Records excluded from brief-row counts can still carry requirements, findings or evidence needed for impact analysis. Include relevant **closed** incidents as regression obligations; follow supersession and delivery links rather than reading `completed` as deployed/accepted or `not_planned` as unfixed. A curated issue inventory is useful input, but is not an implemented resolver or proof of comprehensive coverage.

The first produces a bounded **impact packet** with current callers, future callers, invariants, related issues, affected unfinished briefs, relevant completed evidence, unresolved references and material decisions. The second checks that every reported obligation has a disposition and that those dispositions actually exist at the cited revisions. Neither gives an agent permission to approve a policy or publish a change.

## Discovery and handoff contract

Both author skills must consume the same bounded discovery operation and structured handoff. Extra prose and a longer impact list cannot establish affected-plan closure. Qualify this with withheld consumer identities and actual candidate amendments, as specified in the [authoring cases](15-regression-from-day-one.md#authoring-and-plan-evolution-qualification). A known test corpus is not a general recall estimate.

Keep source enumeration, relationship discovery and materiality assessment separate. Enumerate the explicitly authorized repository/path universe at pinned revisions independently of README files or existing graph edges. Record included and excluded paths, their reasons, supported formats, access failures and truncation. Discover declared references and mechanically observed consumers; deterministic source-search rules also nominate prose-only candidates for review. Retain the exact query/rule versions, parameters, positive and negative results, and unresolved candidates. An unsupported format, inaccessible source or query limit is a coverage gap. No search rule can prove the absence of every undeclared relationship.

The neutral schema and resolver belong in public Assay's existing context/planning ownership. Reuse stable qualified identities, graph aliases and consumer routing. Keep Git/filesystem enumeration and authorized forge reads at adapters; the resolver works on explicit snapshots and remains usable offline. A PostgreSQL projection can accelerate the same query later without becoming its source of truth or requiring a separate graph service. Keep bounded offline artifact/consumer discovery independently useful; target/transition discovery is separately scoped extension work, not an implicit expansion of an existing brief.

The shared schema must define the following handoff, with exact field names and canonical serialization settled in its implementation specification:

| Record | Required content |
|---|---|
| Target and intent | Selected target revision and approval reference where required, proposed contract revisions, transition and independently controlled acceptance source; distinguish draft changes from approved baseline |
| Discovery receipt | Source identities/revisions, authorized source universe and membership, inclusion/exclusion reasons, parser/query versions and parameters, results including negative searches, relevant dependency slice and explicit coverage/read failures |
| Impact obligation | Stable repository-qualified existing identity, inspected lifecycle/implementation evidence, declaring or inferred relationship and source locator, proposed disposition and reason; retain unresolved candidates visibly |
| Amendment obligation | Before/after task, dependency, consumer/delivery and Verify changes; replacement links; pinned base revision and exact candidate patch/result digest when produced |
| Preserved history | Locators and digests for historical Evidence, source/decision accounts and source identities; explicit correction/supersession lineage where a historical correction is proposed |
| Active-work binding | Observed claim and brief revision, observation freshness, owner/checkpoint and acknowledged compatible adjustment or cancellation/reconciliation; a recorded in-progress row alone is insufficient |
| Decisions and readiness | Unresolved obligations, authorized deciding role, permitted next act and phase-specific prerequisites; source availability, proposal approval and deployment qualification remain different facts |

`author-spec` produces intent and actionable amendment obligations. `author-brief` consumes that exact target/contract/discovery binding, constructs candidate amendments and returns a manifest binding each obligation to its patch or justified disposition. A row saying “amend” is not a completed amendment. Reuse the existing consumer/dependency checks, then verify the proposed source revisions actually carry the declared deltas. Both skills must expose partial coverage rather than turn a valid schema into a claim of completeness.

## Authoring becomes a change-set workflow

1. **Choose the intended delivery target.** An urgent legacy repair selects its supported baseline and records new-system applicability. New product work selects the approved future target and usable transition. A draft alternative remains explicitly proposed.
2. **Resolve and discover context.** Run the shared operation over the declared source universe; retain its receipt, coverage gaps and inferred candidates. Access-filter traversal, counts and explanations before returning results.
3. **Reconcile existing obligations.** Inspect current implementations and populated history before choosing reuse or new work. For each affected unfinished brief choose retain, amend, split, replace, retire or explicitly defer with an owner and reason. Retention of behavior does not imply retention of a private destination or obsolete state contract.
4. **Produce a reviewable amendment bundle.** Update task, dependencies, consumer routing, Verify and scope consistently at pinned bases. Link carried obligations and replacements to original identities. Validate historical preservation as well as patch application; keep unresolved obligations explicit.
5. **Review the target and amendments together.** Bind the deciding authority to the actual revision and act. Active work receives an observed checkpoint and acknowledged compatible adjustment, or explicit cancellation/reconciliation before rebinding. Publishing a new document does not change an executing worker's contract.
6. **Recheck at publication and dispatch.** Refresh discovery membership, relevant semantics and active-work bindings. Reuse unaffected conclusions; hold only the affected scope when required evidence is missing or a material conflict remains.

Preserve dated source/decision accounts as well as completed Evidence. Moving a destination changes current delivery fields and adds a versioned amendment; it does not change what an earlier instruction said. Record a genuine historical correction separately with the old claim, correction rationale/source and authorized review, retaining the original record. A historical locator move carries an identity/locator mapping. Compare protected source regions as well as Evidence: a broad title/frontmatter replacement can preserve Evidence hashes while corrupting a dated `sources:` statement. Neither automatic replacement nor a digest alone proves semantic preservation.

The method remains usable across adopters. A project may supply a small declared contract set and reviewed consumer inventory with explicit limits. Missing rich architecture metadata is a coverage gap, not permission for a skill to invent dependencies. The neutral schema and skills must not require private house references.

## Context receipts and changes that invalidate work

Version the receipt schema, discovery/parser/query definitions, digest algorithm and canonical serialization. Bind the selected target and contract revisions, query inputs, authorized source membership and relevant dependency slice. Keep byte provenance distinct from semantic applicability: a digest proves which inputs were used, not that discovery was complete or that a difference matters.

At recheck, enumerate the current authorized source universe again and compare additions, deletions and membership/access changes, including paths absent from the old read set. Re-run affected discovery rules so a new prose consumer without README or typed edges is considered. A repository revision or membership change triggers assessment; it does not automatically invalidate all conclusions. An unrelated edit can retain the prior result with a recorded non-intersection assessment. A producer, acceptance, ownership, custody, migration or activation change requires reconciliation of its affected obligations. Missing required input yields scoped could-not-check; source removal is never an empty-success result.

Concurrent proposals can conflict despite a clean Git merge. At promotion, compare their ownership, contract and migration deltas against the current approved target. Refuse incompatible combined promotion and route the conflict to its deciding authority. An unapproved rival receives no pause or veto authority over already-approved work; target-aligned review and independent implementation continue unless an authorized decision changes their scope. Re-evaluate if that rival is subsequently approved.

Cross-repository changes use a manifest of exact required revisions, approvals and reconciliation results. Approval may precede rollout; activation requires the compatible participants and operational proofs. Public definitions cannot require private source contents, and private overlays must not leak through public results or coverage diagnostics.

## Phase-specific gates and the bootstrap path

| Act | Prerequisite and scope |
|---|---|
| Explore, draft and review | Explicit observed/proposed baseline, source coverage and unresolved decisions; proposing an alternative grants no new authority |
| Approve a target or material plan change | Authorized decision bound to the reviewed revision and affected obligations; implementation proof is required only when the decision specifically depends on it |
| Publish an approved brief or superseding amendment | Approved target binding, reconciled amendment manifest, historical-preservation review and current receipt or a recorded bounded bootstrap decision; opening a draft PR does not confer this status |
| Dispatch and implement | Approved work/acceptance scope, current affected-context assessment and valid work ownership; an active revision change requires its checkpoint/acknowledgement or cancellation/reconciliation |
| Activate the new verifier | Qualified custody, fencing, publication, recovery and handover proofs from the migration plan; these are outputs of implementation, not prerequisites to writing that implementation |

This architecture PR carries the design contract, qualification cases and migration order. It can be reviewed and approved before the resolver or skill integration exists. Bootstrap implementation is commissioned from a manually assembled, source-pinned packet using the same fields, with an independent review and an explicit scope/time-or-revision-bounded bootstrap decision under existing approval rules. Recheck material source changes and active work manually until the qualified operation replaces that step. No blanket fleet exception or synthetic test approval is carried forward.

After the owning specification/decisions are approved, deliver small implementation PRs for the shared schema/fixtures, bounded discovery/receipt validation and public author-skill integration. Independent runtime work can proceed against its approved contracts. Qualify an end-to-end verifier authoring/amendment journey before claiming the automated path usable for dispatch or expanding it across streams. Portable schemas/offline checks can land before the cell service; full graph coverage, PostgreSQL indexing and later workflow features are not bootstrap prerequisites. The [migration sequence](13-migration-and-decisions.md#authoring-support-delivery-order) places this work inside the planning delivery lane; scope and effort require an explicit reviewed estimate.

## The verifier is also the first test of this planning method

Take the proposed shift to an applicable, immutable verification receipt. The impact packet should find more than a statusgen file:

| Relationship | Plans/capabilities that need examination |
|---|---|
| Receipt subject and applicability | Public statusgen eligibility/coverage extraction and graph-execution coverage bindings |
| Runner and publisher authority | Executor/effect separation, witness publication, verifier integrity and verdict publication |
| Work completion and wake conditions | Verification reset/repair, board derivation, supervisor progression and bookkeeping-only re-wakes |
| Durable attempt identity | Shared runner protocol, claim generation, recovery and workflow-instance bindings |
| Human presentation | Public console move, current/history/outcome projections and explanation of stale versus accepted evidence |
| Downstream measurement | Quality/continuing-operations consumers that interpret verification or outcome records; route only actual consumers, not every analytics brief |

The packet must return the **current lifecycle and inspected implementation evidence**, not merely the existence of a plan. If a contract already exists, it should propose reuse or migration; if a brief is completed, it should preserve that result and create the necessary new change obligation. An open issue whose behavior is already fixed should not generate a duplicate implementation brief.

The [authoring qualification cases](15-regression-from-day-one.md#authoring-and-plan-evolution-qualification) require executable resolver/handoff and amendment checks with positive and failing controls. Repeat source-pinned authoring journeys with held-out cases before broad adoption; preserve discovered extra consumers and scoped unknowns as well as known probes. Retrospective cases must disclose their cutoff and hindsight. A manual diagnostic is not proof of implemented gates or prevention in production.

Measure missed affected consumers, unjustified impact hits, author/reviewer effort, unnecessary brief churn and stale-at-dispatch discoveries. The success claim is fewer unanticipated overlaps at tolerable authoring cost, not maximum graph density. Undeclared relationships remain possible; the graph makes coverage and uncertainty inspectable rather than promising omniscience.
