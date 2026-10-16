# Product intent, portfolio and drift — contract and delivery amendment

**Planning amendment, 13 October 2026.** Implements the planning follow-through for the
[future architecture](11-target-architecture.md#portfolio-directed-improvement-and-drift),
merged in [PR #2756](https://github.com/medici-finance/assay/pull/2756). The destination is
selected; the versioned extensions below still need their owning implementation, schema,
consumer and independent-verification work. No field, command, assessment or deployment
is claimed to exist merely because this document describes it.

## Source and authority boundary

Public source inspected at `d484dd5df24f2aa8d557dafce41687b38173f227`: REQUIREMENTS parsing
in `statusgen/requirements.go`, outcome validation in `statusgen/brieffile.go`, the work
export in `statusgen/graph.go`, and runner attribution in `loopadmin/runner/`. These are
reusable foundations; none proves a product-target lifecycle or portfolio ledger.
No fresh deployed-health or complexity baseline was collected. Proceed with additive,
offline contracts and synthetic tests; refresh quality evidence before runtime integration.
The statusgen/19, R2, R6 and R7-owner statements below were refreshed against main after
PRs #2763, #2750, #2743 and #2890 merged.

The existing context handoff contract has merged under statusgen/19
([PR #2763](https://github.com/medici-finance/assay/pull/2763); board status `implemented`,
independent verification pending). This amendment does not change that brief, its Evidence
or its contract. Statusgen/23 extends the reviewed result after statusgen/20, with explicit
version negotiation and migration. R2 storage amendments merged in
[PR #2750](https://github.com/medici-finance/assay/pull/2750), and R6 mirror briefs merged in
[PR #2743](https://github.com/medici-finance/assay/pull/2743); do not duplicate them. A merged
planning amendment is not delivered runtime behavior.

PostgreSQL remains the sole online coordination backend. These offline evaluators use
explicit snapshots and have no database dependency. They neither introduce a SQLite
runtime nor require a second store. Reviewed Git retains intent and durable evidence;
any future runtime projection uses the existing PostgreSQL and effect boundaries.

## Proposed contract delta and ownership

| Owner | Extension to implement | Compatibility and authority |
|---|---|---|
| registers-v1 / REQUIREMENTS | Optional product-intent extension: kind, accountable owner, direction revision, target revision and evidence-backed observed condition | Keep REQ IDs, mandatory base fields, satisfaction claims and tombstones. Existing records remain valid and explicitly unbound. A feature or journey kind does not create a second operational registry. |
| brief-v1 / brief-v2 readers | Optional `outcome-revision`, qualified by the existing `outcome` REQ reference | Keep `outcome: REQ-<slug>` and cross-repo grammar unchanged. `REQ@revision` is not introduced. Missing binding is not malformed legacy data; incompatible mandatory extension versions refuse explicitly. |
| context-handoff / amendment-manifest | Separate product and architecture bindings; mismatch assessment and append-only continue/adapt/stop disposition | Reuse discovery receipts, approval evidence, membership checks and active-work checkpoints. No second invalidation engine; unapproved targets cannot stale an approved binding. |
| continuing-operations | Qualified operational subject references and one impact/observation obligation lineage | The REQ owns intent; operational subjects own their definitions and assessments. Several requirements can reference one journey. Transfer obligations rather than copying deadlines or assessments. |
| work/policy measurement | Versioned portfolio allocation and independent agent/human capacity ledgers | Read-only advisory evaluation initially. No new dispatch authority, default weights, hard allocation floors or automatic target changes. |
| projection / interaction | Separate product-condition, architecture, binding, allocation and observation views | Same typed evaluations for CLI/API/cockpit. Visibility filtering precedes joins/counts; unknown coverage never becomes zero or a complete total. |

### Product-intent records

The optional extension retains one REQ identity and revisioned direction, target and
observation records rather than overwriting prior intent. A direction records rationale,
owner, unique revision and deciding evidence. A target records condition, measure/unit,
window, confidence/basis, unique revision and its direction revision. `observed` references
an attributed assessment with source, date/window, confidence and limitations; it is a
summary/reference, not an independent assessment ledger. Approval is a resolved decision
binding, not a self-declared boolean. Withdrawals, renames and target replacement retain
history, historical bindings and outstanding observation references.

A product target is not an architecture baseline. Changing a measure alone leaves the
architecture binding intact. Bindings are repository-qualified and preserve the original
selection revision. A declared mismatch produces an assessment record; the authorized
owner decides continue, adapt or stop at a useful checkpoint, earlier for urgent evidence.
A continued compatible item may retain its original binding with rationale. An unbound
item or missing secondary relationship stays a coverage gap, not a claim of alignment.

### Allocation and observations

Allocation policy names buckets and subject references, scope/window, policy revision,
separate desired agent and human shares, and any explicitly approved limits. The initial
ledger requires an explicit weight policy; it ships no universal numeric weights. Agent
segments carry stable `(caller, id)` invocation identity, profile revision, actual model,
source, stable segment identity/revision and measured active interval. Retries are distinct invocations, duplicate reports
are not. Multiple segments within one invocation retain their identities; overlapping or
conflicting intervals require explicit reconciliation rather than double-counting. Elapsed-time estimates are a separately labeled proxy and cannot silently join
active-time totals. Human decisions have a stable dedupe identity, declared unit/window,
one primary bucket and source; unattributed decisions stay visible. Counts are not time.
Unknown duration/weight/cost affects only that measure: expose known subtotal and coverage,
not a false total. Zero total means undefined shares. A 5% target and 2% consumption
produce a divergence; neither catch-up work nor a rewritten target follows automatically.

The observation obligation has one stable ID, subject and original brief/impact revisions,
owner, signal/unit, metric revision, baseline or baseline gap, cohort, guardrails,
evaluation rule/band, exposure trigger/window, source and retained
history. It is distinct from evidence and from an assessment. Missing exposure leaves the
trigger unresolved. `done` does not resolve it. Manual-to-practice transfer changes custody
with a receipt and expected prior revision, preserving the same obligation and assessments;
replay cannot create a second deadline. Supporting, adverse, mixed, inconclusive and missing
evidence remain distinguishable. A no-direct-impact disposition preserves its reason and
cannot waive enabling or learning obligations. `observe:` remains candidate authoring
syntax until reconciled with the existing expected-impact extension; no universal mandate.

## Concrete work and dependency waves

| Brief | Deliverable | Wave | Dependencies | Effort |
|---|---|---|---|---|
| [statusgen/22](../../streams/statusgen/brief-22-product-target-records-and-compatible-requirement-parsing.md) | Product-target extension, schema/parser parity and pure record validation | 1 | None | M |
| [statusgen/24](../../streams/statusgen/brief-24-portfolio-allocation-and-capacity-accounting.md) | Versioned allocation and read-only capacity ledger | 2 | statusgen/22 | M |
| [continuing-operations/01](../../streams/continuing-operations/brief-01-operational-subjects-linked-to-product-intent.md) | Operational subject identities and REQ links | 2 | statusgen/22 | M |
| [statusgen/23](../../streams/statusgen/brief-23-product-target-binding-and-context-drift-assessment.md) | Product binding and existing context-reconciliation extension | 3 | statusgen/22, statusgen/20 | M |
| [continuing-operations/02](../../streams/continuing-operations/brief-02-observation-obligations-through-delivery-closure.md) | Offline observation obligation, assessment references and custody handoff | 3 | continuing-operations/01 | M |
| statusgen/21 (amended) | Skills consume the shipped binding contract and retain manual bootstrap | 4 | statusgen/20, statusgen/23 | M |
| [statusgen/25](../../streams/statusgen/brief-25-read-only-portfolio-and-drift-projections.md) | Shared read-only projections, CLI adapter and synthetic journey | 4 | statusgen/23, statusgen/24, continuing-operations/02 | M |

There are two binding paths to the projection: `19 → 20 → 23 → 25` and
`22 → continuing-operations/01 → 02 → 25`, with `22 → 24 → 25` also required.
The first path's real head is statusgen/20 (statusgen/19's contract has merged; its
independent verification is separate), not the unbuilt portfolio dashboard. Statusgen/22 and /24 can proceed independently. The operations
stream remains parked: its admission is an explicit additional prerequisite for /01,
/02 and hence /25. This planning change does not silently activate it. Approving the
bounded contracts and admitting those two briefs does not approve the remaining outlines.

All new tests/commands named in briefs are future deliverables. No runtime or parser is
implemented in this planning PR. Statusgen/19's scope and historical Evidence are unchanged;
/20 retains architecture discovery, /23 adds product semantics, /21 consumes the resulting
supported versions. An implementation already started against /20 or /21 must checkpoint
its owner before accepting this amendment; never silently rebind it.

## Deferred work and preserved boundaries

R7 remains responsible for the authorized public API/cockpit. The amendment manifest names
the `cell-api` stream as its owner (ratified with PR #2890); that stream's scaffolding and
serving brief are still owed. R7 will consume statusgen/25 rather than recreate the ledger in a renderer.
Exposure ingestion, live collection, recurring observation scheduling, automatic dispatch
policy changes, numerical default weights, universal observation enforcement and expanded
autonomy require separately scoped decisions/work. O02–O14 remain outlines except for the
bounded portions mapped in continuing-operations' outline disposition table.

Keep the verifier R1–R8 critical path independent of this portfolio lane. R2 and R6 stay
with their existing PR owners; neither needs portfolio automation to complete. Existing
legacy selection, claims, gates, observation privacy and independent verification remain
controlling until their own qualified migration. Completed Evidence is never rewritten.
