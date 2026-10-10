# Shared context graph and cell classification

**Draft product design, 10 October 2026.** Public Assay owns the reusable context
schema, resolver, source adapters, bounded queries and neutral examples. Adopters
own their source catalogs, private overlays and operational configuration. These
are proposed capabilities, not implemented interfaces.

This chapter supplies the common substrate for the [future-state model](12-future-state-model.md).
That model owns target/transition semantics and the authoring handoff; this chapter
owns composition, classification and snapshot/query requirements. They form one
design, with no separate identity or relationship registries.

## Boundaries and existing sources

Keep two connected work packages: cell definitions/classification and derived
indexes/queries. Continuing operations, execution, evaluation and client delivery
retain their own semantic owners. An artifact query can be useful before a rich
taxonomy, a database service or a cockpit exists.

| Source or owner | Reuse and boundary |
|---|---|
| [Cell configuration](../../../cellconfig/config/schema/cells.go) | The strict reader owns operational configuration. New classification fields require a versioned parser rollout or a separately referenced catalog. |
| [Topology](../../../tools/desk/internal/topology/topology.go) and [graph aliases](../../streams/graph-repos.yaml) | Resolve existing composition and qualified identities. Declarations cannot widen permitted repositories or credentials. |
| [Dependency graph](../../dependency-graph-design.md), [statusgen graph](../../../statusgen/graph.go) and [stream-view contract](../../../statusgen/streamview) | Reuse work identities, dependency meanings and evaluation/projection ownership. No second eligibility or blocker evaluator. |
| Component contracts and semantic owners | Index provided/consumed contracts, allowed dependencies and conformance references at their existing source; no rival editable owner list. |
| Runtime and effect owners | Relate attempts, claims, leases and effect receipts; never acquire custody, schedule work or publish an effect through a query. |
| [Continuing operations](../../streams/continuing-operations/spec.md) | Consume subject/assessment contracts as they become available. A journey, operational workflow or practice differs from an execution definition or run. |
| Manifests, locks, pins and evidence | Preserve declaring sources and distinguish declared, resolved, observed and verified facts. |

Evaluation/projection stays with its semantic owner, publishing versioned snapshots
for authorized reads through the public service to cockpit and agent clients. The
context index joins permitted projections and artifact facts. Shared code respects
import and authority boundaries: reuse of a wire contract may be appropriate where
importing its producer is not. Import checks alone cannot prove runtime isolation.

The offline resolver remains usable without the service. PostgreSQL can project
the same model under the [storage design](14-storage-forge-graph.md); neither that
projection nor GraphQL becomes the canonical definition. Extend the existing
serving surface rather than introduce another MCP server or require every author
to start the runtime.

## Cell classification

An ontology defines kinds and relationships; a taxonomy names categories within
those kinds. A cell selects a versioned profile and can add local nodes. Multiple
membership is normal; a universal category tree is unnecessary.

| Dimension | Meaning | Neutral examples |
|---|---|---|
| Theme | Why invest? | Evidence quality, adoption, service continuity |
| Plane | What responsibility? | Control, execution, evidence, experience, knowledge/context |
| Architectural layer | Where does implementation sit? | Interface, service, domain, persistence, linked to applicable dependency contracts |
| Substrate | Which shared system? | Identity, execution, knowledge, evidence, data; concrete instances have distinct IDs |
| Scope | Which kind of work? | Product, cell operations, shared platform |

Plane, layer and substrate are different dimensions. A cockpit can serve the
experience plane while using a context substrate and consuming an evidence read
contract. Classification grants no imports, authority or eligibility. Define
inclusion/exclusion examples and equivalent/broader/narrower mappings; identical
labels in different profiles do not establish equal meaning.

Categories need stable namespaced IDs, kind, label, definition, steward, revision,
active/deprecated lifecycle and explicit replacement/mapping references. Same-kind
parents may form a DAG; parent cycles, unknown references, duplicate IDs and
conflicting definitions fail validation. Deprecated IDs remain resolvable. Count
distinct work identities when categories overlap.

Source metadata may reference an optional classification catalog. Pin its profile
revision/digest at composition level. Incompatible profiles require an explicit
migration; missing classification means unclassified. Do not overload the existing
`theme` presentation setting. Briefs may inherit stream grouping without a mandatory
rewrite. Operational impact links remain owned by their subject schema.

## Composition and migration

A cell needs one logical home for descriptive composition, not necessarily a new
repository. Inventory existing topology, source catalogs, fields and readers before
extending them. A catalog must not become another editable repository/owner roster.

| Material | Treatment |
|---|---|
| Identity, purpose, roles, source catalogs and repository relationships | Reference existing authoritative definitions and versions |
| Profile, categories and mappings | Versioned descriptive catalog |
| Journeys, practices, metrics, policies and budgets | References to their owning contracts; appropriately disclosed metadata |
| Cross-stream grouping and relationships | Derived projections |
| Checkout bindings, permitted actors, credentials, leases and current spend | Existing machine/operator/runtime configuration and stores; never taxonomy authority |

Keep a stream's tracking home while composing views across repositories. Membership
and upstream relationships confer neither ownership nor writes. Preserve current
ownership constraints and the target's explicit mutation-domain rules. Coordinate
strict-reader versions before adding configuration fields. Test two synthetic cells
with distinct profiles, renamed sources, duplicate IDs and mixed visibility before
authoring adopter migrations.

## Identity and relationship contract

Reuse repository-qualified work IDs and aliases. Names, paths and stream homes are
locators. Rename/repository-move continuity needs an explicit mapping; the existing
repository-plus-slug stream key cannot silently become a new identity scheme.
Reject ambiguous aliases and source-import cycles. Other cycle rules depend on
relationship type.

Relate work records; operations/assessments; artifacts, evidence and explicitly
admitted knowledge; and classifications. Add targets/transitions under the
future-state model. Personal memory is not implicitly admitted. Briefs and PRs
remain distinct, including multiple PRs delivering one brief.

Nodes/edges carry kind/schema version, stable identity, declaring source and
revision/digest, derivation version, access scope and declared/observed/inferred
basis. Inferred links are discovery candidates. Conformance references are not
passing results, review routing need not identify operational accountability, and
activity counts establish neither expertise nor competence. Membership implies no
dependency, authority, deployment, completion or causation.

## Snapshot and query contract

Record exact source membership, revisions/digests, adapter/schema versions,
completeness and observation cutoff. Sort and content-address the deterministic
payload; keep wall-clock receipts separate. Clean-source mode rejects dirty or
changing inputs. Later working-tree modes must label and digest actual bytes. A
multi-repository snapshot names the revision set read, not an atomic fleet commit.

Build privately, validate, then publish atomically with one output owner. Failed or
partial refreshes cannot replace the last complete snapshot or disguise its age.
Queries return snapshot identity, coverage, provenance and freshness, distinguishing
empty results from unavailable, stale, unsupported and denied inputs. Missing
retained history cannot prove an absence of historical change.

Start with typed membership, dependency impact/inverse and source-explanation
queries. Bound rows, depth and time; specify stable ordering and pagination.
Artifact adapters distinguish ecosystem/coordinate, declared constraint, resolved
version, direct/transitive dependency and observed deployment. A manifest match is
a candidate impact, not proof of a deployed vulnerable workload. Bounded artifact
indexing alone cannot satisfy chapter 12's discovery receipt and plan reconciliation.

## Visibility and trust

Private overlays may add records referencing public IDs, but cannot redefine them
or relax access constraints. Derive edge visibility from endpoints and evidence.
Build public views from authorized inputs, not a scrubbed private graph. Actual
private indexes belong in access-controlled storage; committed fixtures are synthetic.

Constrain traversal, joins, counts, explanations and caches before returning a
response. Adding inaccessible records must leave public results and diagnostics
unchanged. Do not expose withheld existence unless an authorized source explicitly
declares an unpublished reference. Cache keys/invalidation honor the selected read
contract's authorization epoch and revocation bounds; no invented instant-revocation
promise. Possession of a local snapshot is itself access to its contents.

Visibility and trust differ. Parse allowlisted data without executing hooks,
fetching untrusted heads or following paths outside configured roots. Forge text is
attributed source material, never instructions.

## Delivery and qualification

The [public delivery scope](18-authoring-context-delivery.md) separates an offline
artifact slice, optional classification, future-state discovery and skill integration.
Existing consumer owners add views after relevant contracts ship. Operational
records survive archival of a change stream that improved them; the graph neither
reschedules practices nor evaluates assessments.

Require synthetic evidence for identity migration/discontinuity, rejected unknown
schemas and duplicate IDs, distinct totals, unchanged eligibility under taxonomy
edits, visibility invariance, coverage gaps, deterministic snapshots and preservation
after failed refresh. Integration traverses source → resolver → query → consumer
view. Schema validation alone proves neither isolation nor discovery completeness.
