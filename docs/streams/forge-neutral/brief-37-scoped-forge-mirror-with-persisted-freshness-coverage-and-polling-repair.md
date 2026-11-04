---
brief: assay:assay:forge-neutral:37
title: Scoped forge mirror with persisted freshness, coverage and polling repair
why: "Every desk, status run and planning read asks the forge the same questions again, one verb at a time, and a throttled or partial read can look like an empty queue. One scoped mirror that records what it read, when, and how completely lets those readers share one answer and see when it is stale or incomplete, without each of them polling the forge."
wave: 7
depends: ["graph-execution/28", "graph-execution/29", "graph-execution/19", "forge-neutral/36", "forge-neutral/33", "desktools-v2/09", "forge-neutral/46", "forge-neutral/47", "forge-neutral/48", "forge-neutral/50", "forge-neutral/54"]
unblocks: ["forge-neutral/38", "forge-neutral/41", "forge-neutral/43", "cell-api/02"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "sensitive-data is yes because the mirror copies private pull request, merge request, issue and review content into a database that outlives a single read. The human confirms that rows are stored only for an admitted scope, that losing permission makes coverage visibly unavailable instead of silently empty, and what happens to already-mirrored content when that permission is lost (the decision below)."
issues: []
outcome: none
schema: brief-v2
version: 19
decision-issue: 5380
id: 06dc3c03-bfb0-49cf-b171-8f7d192b7aea
authored: 2026-10-13 by statusgen newbrief (non-CI R6 authoring follow-up)
sources:
  - "freshness-checked 2026-11-04 @ b45a44e4b0e962d52d7d18d1b6aaea9379a71503; genuine main merge preserves read-through and mapper ownership contracts"
  - "freshness-checked 2026-11-03 @ 438310c23194b20a5cb1ceba998fb6b42b30bf0f; F-mapper-owner routing repair from PR #5397 and implementation PR #5406"
  - "Requested shared read-through and failure-cache behavior, 2026-11-03; no activation grant"
  - "Planning amendment 2026-10-23; source checked on App-fetched main 9c4307a19a190964db084104ecc1f7bd62cba433; candidate-plus-priority parity and GH-call audit"
  - "docs/performance-attribution-spec.md — requested perf-v3 amendment, 2026-10-19 @ 2e02111ed"
  - "2026-10-16 performance authoring amendment, inspected origin/main eba941721; docs/streams/desktools-v2/README.md performance priority"
  - docs/proposals/non-ci-architecture/20-amendment-manifest.md — R6 owner row and the "R2–R8 amendment scope" R6 paragraph
  - docs/proposals/non-ci-architecture/16-delivery-sequence.md — package R6 (pagination, lost/reordered events, permission loss and partial scans preserve uncertainty; a client outage cannot cause a poll storm)
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md — "Reuse and expand the existing forge-mirror work" (pipeline, one ingestion owner per scope, overlap windows, stale-not-deleted, centralised refresh)
  - docs/proposals/non-ci-architecture/15-regression-from-day-one.md — the forge-mirror rows (failed reads keep stale rows; partial pagination and permission removal stay incomplete)
  - "medici-finance/assay#2500 — maintainer ratification of the whole non-CI target @ 23f655f5d (approval activates nothing)"
  - "medici-finance/assay#2739 — the step-3 amendment manifest (merge 025afaf6b)"
  - docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md — the module layout and the three-state envelope this brief extends
  - docs/streams/forge-neutral/brief-33-forge-reads-for-statusgen-s-remaining-sites.md — the all-state issue and change reads the inventory pass reuses
  - docs/streams/desktools-v2/brief-09-access-pattern-queries.md — `ReviewQueueSnapshot`, the tuned one-query change and review snapshot this brief reuses
  - freshness-checked 2026-10-13 @ 4696d598e (origin/main)
  - "docs/desk-workflow-efficiency-spec.md"
  - "freshness-checked 2026-10-17 @ fb85924c039c27e7d2aa80e055bfce118e46125d; approved spec and consolidation amendment"
exec-tier: strong
exec-tier-why: "Question (c), silent failure toward a pass: an unfinished scan that advances a watermark, an absence read as deletion, or a refused read cached as an empty list all pass a happy-path test and hand every consumer a confident wrong answer."
domain: complicated
decision-trigger: start
consumers:
  - "forgeread/mirror/, forgeread/mirror/mirrortest/: follow-up forge-neutral/46 (pure kernel and reusable reference conformance creation)"
  - "forgeread/mirror/, forgeread/mirror/mirrortest/: follow-up forge-neutral/37 (consume the kernel; add only joined ingestion, coalescing, chosen retention and PostgreSQL conformance extensions)"
  - "forgeread/adapters/, forgeread/adapters/github/, forgeread/adapters/gitlab/: follow-up forge-neutral/47 (the two incremental list reads and explicit conditional-unsupported baseline)"
  - "forgeread/adapters/access.go, forgeread/adapters/github/, forgeread/adapters/gitlab/, tools/desk/internal/deskkit/sharedread.go: follow-up forge-neutral/37 (optional conditional page access and actual mirror integration; no second incremental walker)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go, forge_surface_deskread_test.go: follow-up forge-neutral/47 (create exactly the two enumerated methods, shared-adapter delegation and frozen surface entries)"
  - "tools/desk/internal/deskkit/mirrorsource.go and its base tests: follow-up forge-neutral/47 (admitted MirrorReadSource consuming the two incremental operations)"
  - "tools/desk/internal/deskkit/mirroringest.go and source-only tests: follow-up forge-neutral/50 (admitted MirrorReadSource to mirror.Source.Scan observation mapper)"
  - "trusted desk-tools composition and joined PostgreSQL tests: follow-up forge-neutral/37 (consume /50 source observations; supply authenticated admission and current authorizer to the joined source/store ports without a second mapper)"
  - "tools/desk/internal/deskkit/forge_response_bounds.go and forge_response_bounds_test.go: follow-up forge-neutral/48 (one native mirror-response bound and real-backend conformance; shared-library relocation reuses this owner)"
  - "tools/desk/internal/deskkit/forge_github.go, forge_gitlab.go, sharedread.go and mirrorsource.go: follow-up forge-neutral/48 (native bound composition before buffering/decoding; admitted custody and existing operation semantics)"
  - "tools/desk/internal/deskkit/forge_response_bounds.go, forge_github.go, forge_gitlab.go, sharedread.go and mirrorsource.go: follow-up forge-neutral/37 (consume /48's native bound in joined mapper/store failure flow; no second implementation)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/47 (the two incremental-operation inventory rows)"
  - "workflow/store/: follow-up forge-neutral/52 (canonical mirror tables, numbered migration and mirror.Store adapter; this parent consumes that owner)"
  - "workflow/store/: follow-up forge-neutral/54 (canonical refresh Gate and AttemptAuthority; this parent consumes them at trusted deskkit composition)"
  - "graph-execution/28 fact-completeness mapping at trusted composition: follow-up forge-neutral/37 (full-parent fact integration remains; no duplicate Store or Gate implementation)"
  - "statusgen/testdata/regressionfloor/ (as planned by graph-execution/29): follow-up forge-neutral/37 (one frozen mirror-coverage case under 29's inventory rules)"
  - "statusgen/: out-of-scope (forge-neutral/18 owns statusgen's forge reads; this brief adds no statusgen link and no deskread kind)"
  - "comments, check and status observations, event delivery and reevaluation demand: follow-up forge-neutral/38"
  - "tools/forge-import-check/: fixed-here (source-only verification helper relocation; not a runtime mirror implementation)"
  - "forgeread/mirror/reviewcoverage_test.go: follow-up forge-neutral/37"
---

# Brief 37 — Scoped forge mirror with persisted freshness, coverage and polling repair

## Context

files:
- `tools/forge-import-check/check_env_imports.go`
- `tools/forge-import-check/check_env_imports_test.go`
- `forgeread/mirror/`
- `forgeread/mirror/mirrortest/`
- `forgeread/adapters/access.go`
- `forgeread/mirror/reviewcoverage_test.go` (planned)
- `forgeread/adapters/github/`
- `forgeread/adapters/gitlab/`
- `forgeread/README.md`
- `tools/desk/internal/deskkit/forge.go`
- `tools/desk/internal/deskkit/forge_github.go`
- `tools/desk/internal/deskkit/forge_gitlab.go`
- `tools/desk/internal/deskkit/sharedread.go`
- `tools/desk/internal/deskkit/mirrorsource.go`
- `tools/desk/internal/deskkit/mirroringest.go` (planned)
- `tools/desk/internal/deskkit/forge_response_bounds.go` (planned)
- `tools/desk/internal/deskkit/forge_response_bounds_test.go` (planned)
- `tools/desk/internal/deskkit/forge_surface_deskread_test.go`
- `docs/streams/forge-gitlab/inventory.md`
- `workflow/store/` (planned)
- `statusgen/testdata/regressionfloor/`
- `changelog/forge-mirror-polling.md` (planned)

facts (4696d598e, 2026-10-13):
- Ownership check (2026-11-03): main `438310c23194b20a5cb1ceba998fb6b42b30bf0f` still assigns the mapper to /37, while authoring #5397 at `be2e142fd` and implementation #5406 at `2677323c9` assign the source-only mapper to /50. This split routes that owner explicitly: the `mirroringest.go` path above is a consumed composition input, not a second mapper deliverable; /37 retains joined PostgreSQL assembly and integration evidence.
- `forgeread/` exists (forge-neutral/36, implemented). Its root package is offline and frozen only;
  authenticated adapters live under `forgeread/adapters/` and read through `adapters.Access`, which
  the desk-tools composition supplies after resolving the credential and checking the repository.
  statusgen links the root package only (forge-neutral/36 rows 10 and 11; forge-neutral/18 row 21).
- `ReviewQueueSnapshot` (desktools-v2/09, verified) reads a repository's open changes and each
  change's reviews in one query; `ReviewsComplete=false` means "read per item", never "no reviews".
- `ListIssues(repo, {State: all})` and `ListChanges(repo, states)` (forge-neutral/33) walk to a page
  ceiling and report `Incomplete`; neither takes an updated-since bound or a validator.
- No conditional request (ETag or equivalent) exists in either backend's transport.
- `workflow/` does not exist; graph-execution/19 plans `workflow/store/`, and the R2 follow-up amends
  it to PostgreSQL coordination. This brief adds tables to that store; it does not create a store.
- No repository root `go.work` (its presence fails `loopadmin/runner/request_test.go`); every module
  test runs with `GOWORK=off`.
- `forgeread/go.mod` has no `require` and no `replace`; `forgeread/README.md` says "The module has no
  dependencies". statusgen requires forgeread through a relative `replace`, and graph-execution/19
  plans a `workflow` module that consumes statusgen packages. A forgeread import of a statusgen
  package would therefore be a module cycle.
- graph-execution/28 plans its Go types in `statusgen/domain`: qualified work identity, candidate,
  fact completeness and reevaluation generation. It defines no pull or merge request identity.

layering: `forgeread/mirror` is a library with no network client, no process launch and no credential.
It plans refreshes, applies observations and answers queries against a `Store` interface. The source of
observations is supplied by trusted composition in desk-tools: `tools/desk/internal/deskkit/mirrorsource.go` from /47 owns the
admitted incremental reader, while forge-neutral/50 owns `tools/desk/internal/deskkit/mirroringest.go` (planned), which maps its observations into /46's
kernel/source/store contracts. Reuse those owners rather than creating competing walkers or models.
PostgreSQL persistence implements `Store` inside the R2 store. Rows 2, 10, 11 and 18 hold the split.
The mirror defines its own repository and change identities. The graph-execution/28 mapping lives
in the `workflow` module, which links both `forgeread` and `statusgen`, never in `forgeread`
(rows 17 and 19).

design-fit:
  owner: forgeread/mirror (scoped mirror semantics); workflow/store (persistence)
  contract: "forge mirror v1 — scope, observation, coverage and freshness records; versioned with the store schema"
  retires: [per-verb repeated full forge sweeps once consumers move in later briefs; nothing retired in this brief]
  weight: "verbs 0, flags 0, refusals 0; two read operations, one library package, one store migration"
  why-add: "Readers need one shared, honest answer about forge state with its freshness and coverage. Extending each verb's own sweep would multiply provider load and keep every reader blind to partial reads."

single-point-of-failure: the completeness-watermark rule in `forgeread/mirror` (a scope's
last-complete time advances only in the same store transaction as a fully walked scan). Behind it:
the store's own constraint that refuses a watermark advance whose scan record is not marked complete
(row 7, with the library check bypassed), and every query answer carrying its coverage state, so a
consumer that ignores the watermark still sees `partial` or `unavailable` (rows 4 and 6).
On the read side, the one control is the query-time admission check in `forgeread/mirror`. Behind it,
the store independently enforces Task 1's same admission relation and returns an explicit
permission refusal, not a successful empty result, even with the library check bypassed (row 15).

Read first: `docs/proposals/non-ci-architecture/14-storage-forge-graph.md` §"Reuse and expand the
existing forge-mirror work", `forgeread/README.md`, `docs/library-first.md`,
`docs/streams/forge-neutral/brief-36-importable-fact-reader-sdk-and-first-shared-read.md`.

## Relationship to forge-neutral/18 — 2026-10-13

forge-neutral/18 is in progress. It moves statusgen's remaining forge reads onto the `deskread` verb
and keeps statusgen credential-free. This brief does **not** rebind it: /18's Task, Verify, sites,
dependencies and completion test are unchanged. This brief adds no `deskread` kind and no statusgen
import. statusgen cannot link `forgeread/mirror`, because it is a subpackage (rows 10 and 11 hold
that). Any later move of a statusgen read onto the mirror is a separate brief with its own ruling. It
must keep /18's and /36's credential limits; it is not implied by this brief or by /18's completion.
If /18 lands first, nothing here changes; if this brief lands first, /18's row counts are unaffected
because no statusgen file is touched.

## Toward R7 — 2026-10-13

R6 feeds R7 (public API and thin cockpit), and R7 has no owner yet. The manifest leaves the
derived-board-or-new-stream choice to the R7 follow-up, so no R7 brief exists to name. This brief's
query answers (rows, coverage, last complete refresh, observed revision, retained-history window) are
the read contract R7 consumes. When the R7 briefs are authored, each that reads the mirror declares
`depends: ["forge-neutral/37"]`, and `statusgen newbrief` writes the inverse `unblocks:` edge here.
Until then `unblocks:` names only forge-neutral/38, because the lint refuses an edge to a brief that
does not exist.

## Early read-only contributions — 3 November 2026

The first useful supported outcome is a fixture consumer that applies source observations and
queries an admitted mirror scope with explicit coverage and freshness. Two independent capability
contributions precede this joined delivery: [46](brief-46-pure-read-only-mirror-kernel-and-conformance.md)
owns the pure mirror kernel and its reusable conformance contract;
[47](brief-47-admitted-incremental-forge-readers-for-mirror-ingestion.md) owns the bounded provider
update readers and their concrete source composition. Both reuse the already shipped admitted-read
capability from 36; neither requires a new durable store or the write executor. Inspect source
availability and the dependency's actual capability at pickup; lifecycle labels alone are not proof
that an API is absent or independently accepted.

This brief consumes those owners rather than implementing competing models or provider walkers.
Every original Task and Verify criterion remains an obligation of the joined delivery. In
particular, the two early contributions do not supply PostgreSQL persistence/transaction constraints,
restart recovery, production permission-loss retention policy, event/controller mapping, full
review/check coverage, conditional refresh transport, cross-process refresh coalescing, actual reader
cutover or operational savings. Conditional requests in row 8 and coalescing in row 6 remain here;
47's explicit unsupported/full-read baseline does not satisfy them. Passing early fixture tests does
not satisfy rows 7, 9, 12, 13 or 15 against the assembled production adapters.

The Human decision below remains unratified; no retention default is selected, no gate is lowered,
and this brief remains held for its gate record and missing durable-store prerequisites.
Early implementation does not claim this brief implemented or activate a cache. The current
[DR-forge-library](../decisions/DR-forge-library.md) supersedes the older process-only integration
boundary: trusted transport composition follows its shared-library owner, without moving credentials
into the pure mirror or inventing another service. Historical source accounts remain unchanged.

## Human decision

The forge mirror copies private pull request, merge request, issue and review content for each
repository it is allowed to read into a database that keeps it between reads. When the reading
credential later loses access to a repository, the mirror stops being able to refresh it and must
show that repository as unavailable. The open question is what happens to the content already
copied.

Options:
1. **Keep it as stale, access-checked history**: rows stay, marked stale and permission-lost.
   They are served only with that coverage state, and only to callers whose own authorization
   still covers the repository. Retention follows the configured history window. Reconstruction
   and audit keep what was observed, and the private-data footprint stays larger.
2. **Purge the content and keep only identity tombstones**: on confirmed loss of access, private
   bodies and fields are deleted. Identities, the coverage gap and the loss time remain. The
   footprint is smaller, and the earlier observations are gone if access returns. The mirror then
   re-reads from scratch.

Default if no answer: none — blocks until answered.

The mirror stays unactivated and the brief stays held until its human gate is satisfied.

### Implementation direction — 3 November 2026

The operator requested option 2 (purge copied private content; retain identity tombstones)
in the working conversation. Before the recorded ruling, prepare only option-neutral
synthetic fixtures and test scaffolding usable under either policy; do not implement any
retention action before the human decision is recorded, as row 16 requires.
This bot-authored record does not ratify the human gate: the canonical decision
record is medici-finance/assay#5380, and only the ratifying human's own recorded ruling
satisfies it. Preserve both options in the decision section and row 16 until that ruling
is recorded; then select the purge criterion and remove the unchosen criterion as row 16
requires. No storage activation or exposure follows from implementation direction.

### Next contributions and critical path — 3 November 2026

Native response bounds are independently extracted into
[48](brief-48-bound-native-mirror-response-allocation-before-decoding.md). Consume that
single implementation here. This contribution does not satisfy this brief's joined row 23,
which still requires the actual mapper, durable Store and preserved prior watermark.

The freshly inspected main 678d7ea5 has no workflow module. The persistence path remains
graph-execution/16 → graph-execution/19 → this brief → forge-neutral/38, /41, /43 and /45.
graph-execution/19 also requires its other declared prerequisites, including /03; /16
alone is insufficient. The current move is /16's store-neutral ownership/reservation
implementation, in parallel with /48. Do not introduce another mirror database, credential
service or per-tool polling fallback. The shared-library cutover remains desktools-v2/75
and /77 under DR-forge-library. Independent /46 and /47 re-derivation is tracked by #5377;
GitLab case identity qualification before activation remains #5376.

## Source-only mapper contribution — 3 November 2026

forge-neutral/50 owns the admitted incremental-source to mirror.Source.Scan adapter,
including conservative field presence and incomplete-scan mapping. It may prepare
synthetic native-to-reference-Store evidence independently of PostgreSQL, execution
and serving. It does not discharge this brief's persisted joined row 23, choose
retention, qualify shared refresh ownership or authorize private ingestion/activation.
All 23 existing acceptance rows, joined composition obligations and gates remain owed here. Task 8 consumes /50's mapper and adds the PostgreSQL assembly and joined evidence; /50 does not satisfy those remaining obligations.

## Bounded durable refresh contributions — 3 November 2026

[53](brief-53-qualified-native-mirror-attempt-accounting-and-retry-metadata.md) supplies
safe typed native attempt/retry evidence and the pre-request authority seam;
[54](brief-54-canonical-postgresql-refresh-gate-and-admitted-read-through-integration.md)
implements the canonical SQL Gate/attempt authority and real admitted source/coordinator/
Store fixture. The forge-neutral/53 and forge-neutral/54 contributions are bounded precursors to this complete joined delivery, not a new
retention, service, policy or activation owner. Offline pickup uses their explicitly
qualified immutable source contracts, without waiting for workflow execution features.
The forge-neutral/51 Requirements/checked-Outcome repair must be genuinely consumed; owner-only
fulfilment or post-hoc request counts cannot qualify the durable join. Unknown state
holds. Every existing Task, dependency, risk/gate and all23 Class/Command pairs here
remain; these contributions do not satisfy the rest of full joined row23 by inference.

## Precursor ownership and pickup ordering

The typed dependency on forge-neutral/54 orders full37 after the bounded50–54
source/coordinator/persistence contributions. Existing Context paths remain later
integration touchpoints, not permission to implement their already-owned source mapper,
Gate or Store concurrently. Consume50/53's admitted mapper/native boundary,51's
coordinator contract,52's Store and54's canonical SQL Gate before extending the
remaining retention/scheduler/fact-mapping/complete-acceptance obligations. Preserve every
original Task, Verify row, gate and Evidence entry. This dependency adds ordering; it
neither accepts a precursor nor removes full37's other prerequisites.

## Ground rules

- Offline fixtures and fake provider transports only; real PostgreSQL only in the disposable test
  harness graph-execution/19 provides after its R2 amendment. No production credential, query,
  deployment or activation. Approval of this brief activates nothing.
- No generic query, URL or exec port. Two new read operations only, landing with their consuming
  call site (the mirror source), on both backends, with a could-not-check refusal where a provider
  cannot answer. They join the frozen inventory. No other change to the operation set.
- Credential composition stays in desk-tools. Nothing in `forgeread/` resolves a credential, reads
  the environment for one or names a host. No shipped file in `forgeread/mirror` imports `os` or
  `syscall`; configuration and secrets arrive as parameters from composition (row 18).
- `forgeread` stays a module with no dependencies: no `require` or `replace` in `forgeread/go.mod`,
  and no import of a statusgen or `workflow` package. Mappings onto graph-execution/28 records belong
  to the `workflow` module (rows 17 and 19).
- The mirror store is per cell, partitioned by admission. Desks share rows only within the same
  composition-derived admission; distinct credential installations do not share an admission just
  because both cover a repository. Each has its own scope, rows, coverage and refresh ownership.
  No caller outside a scope's bound admission reads its rows or receives a complete-empty answer
  for that scope (row 15). Sharing a mirror across cells needs matching access scopes and explicit
  ownership, and is a separate brief.
- Full /37 retains graph-execution/19's R2 prerequisites. Its bounded PostgreSQL Store child
  forge-neutral/52 may instead consume an actual tested canonical migration and container
  harness revision from graph-execution/36 before full /19 execution delivery, including
  through a genuine owned feature-branch merge for offline work. If that actual foundation is
  absent, stop and report NEEDS_CONTEXT. Do not build a second store. This exception grants
  no parent acceptance, shared-refresh fencing, retention action or activation.
- Stop at `implemented`; an independent verifier owns verified and done.

## Read-through and failure-cache amendment — 3 November 2026

Consume the read-through/failure-cache contract in the [architecture amendment](../../proposals/non-ci-architecture/22-forge-service-and-source-queries.md). /37 owns the shared mirror refresh, not client fallback or a second poller. A cold or insufficient kernel query can be fulfilled through an injected refresh port and the admitted source/store ports; a coverage-sufficient source walk supplies the original query and cache. Serving and client-fallback assertions belong to /40 and /45; caller accounting, fairness and request-identity gates belong to composition outside the pure kernel. Cache failed refreshes with bounded retry eligibility under the same scope/access epoch, preserving prior rows and LastComplete plus explicit unavailable/stale coverage. Expiry permits one bounded retry, not watermark advancement or permission-hold recovery. Keep the pure kernel network-free and all 23 existing Class/Command pairs unchanged. Extend row 6 with injected-port cold-query fulfillment and kernel failure cooldown/expiry/recovery, with no second source path; row 9 retains independent scope refusal. The selected retention gate is unchanged.

## Candidate and priority amendment — 2026-10-23

Serve the existing planner's complete scoped candidate observations through this mirror
owner, as specified in efficiency spec §2.1 and A11–A12. One ingestion/refresh owner supplies
all compatible role clients; policy/time/capacity-only reselection does not enumerate GH.
Keep full initial/periodic inventory, event/poll repair, removals, permission and freshness
coverage. No outage fallback poller per caller and no zero-cost producer assumption.
Retain all held/unknown and priority-unmapped source records; projections do not become
policy or dispatch authority. Reuse the existing reviewcoverage_test fixture surface to
measure repeated consumers and changed facts. The direct-reader bridge remains independent;
existing completed batching/conditional requests are baseline, not this brief's new saving.

## Task

1. **Mirror model (`forgeread/mirror`).** Reuse /46's typed kernel records rather than defining
   another model. The joined delivery must retain and extend that owner where needed for scope
   (provider, host, repository, record kind: change or issue); change and issue observations with
   PR versus MR provenance, state,
   closure reason, head revision, labels, author, requested reviewers, provider updated time and
   observation time; and review/approval observations kept provider-specific beside the common
   fields (a GitHub review and a GitLab approval are never reduced to one boolean). The mirror
   defines its own provider-qualified identities and never keys on a file path or a display name: a
   repository is (provider, host, full repository path); a change is (repository, change kind
   `pull-request` or `merge-request`, number); an issue is (repository, number). graph-execution/28
   defines work and candidate identity, not change identity, and `forgeread` does not import it.
   Each scope is also bound to the admission it was read under: an opaque admission ID that
   trusted composition derives from the authenticated caller's cell and credential installation.
   The stored scope key includes this admission ID as well as the repository and record kind.
   Query admission is derived by that same trusted composition, never accepted from request input;
   an admission ID is a binding, not a bearer credential.
   Both the library and the store enforce the same fail-closed relation: the caller has a valid
   composition-derived admission, its ID equals the scope's bound admission ID, and the supplied
   authorizer confirms current repository coverage at query time. Missing, invalid, nonmatching
   or no-longer-authorized admission answers `unavailable: permission` with no rows, even if the
   stored scope is complete. Repository coverage alone never substitutes for identity equality.
   The library refuses before querying scope data; the store independently checks the relation
   using the trusted query context and authorizer, includes admission equality in its data
   predicate, and returns a typed permission refusal when the relation fails. It must not turn a
   filtered-out scope into a successful empty result or return that scope's complete coverage.
   A complete-empty answer is valid only after admission succeeds and the admitted scope itself
   contains no matching records. Row 15 exercises each layer with the other bypassed.
2. **Coverage and freshness.** Per scope, persist: coverage state (`never-read`, `complete`,
   `partial`, `unavailable` with a reason class: `permission`, `transport`, `rate-limited`,
   `truncated`), last attempted time, last complete time (the completeness watermark), provider
   observed-up-to time, and a retained-history watermark kept separate from freshness. Each state
   maps to exactly one graph-execution/28 fact-completeness value, and only `complete` within the
   configured freshness window maps to present. That mapping is a function in `workflow/store/`,
   the module that links both `forgeread` and `statusgen`; `forgeread/mirror` exports the coverage
   states and imports nothing from statusgen. Every query answer carries its scope's coverage,
   last complete time, observed revision and stale flag. A query over a scope that is not complete
   never answers a bare empty list. Task 1's admission refusal overrides stored completeness;
   only an admitted query can expose the stored coverage.
3. **Reads.** Consume exactly the two operations created by /47, `ChangesUpdatedSince` and
   `IssuesUpdatedSince`, for both providers in `forgeread/adapters/{github,gitlab}` and their admitted
   deskkit delegation. Do not recreate them or add duplicate surface/inventory entries. The joined
   delivery still must prove each lists records updated at or after a bound, ordered by update time,
   walks every required page up to a ceiling and reports `Incomplete` at the ceiling, never a short
   complete list. Extend that existing owner with optional `ConditionalAccess` beside `adapters.Access`:
   one page with a supplied validator, answering not-modified or a page with its new validator.
   Where the provider or endpoint gives no validator, read in full and record `conditional: unsupported`.
   /47's unsupported/full-read baseline does not satisfy this conditional transport/scan acceptance.
   Keep its inventory rows and the frozen surface's want list at exactly the same two operation names.
4. **Refresh planner and polling repair.** Reuse /46's deterministic refresh/scan planning
   semantics; compose the actual ingestion and polling owner here, one per (admission ID, repository,
   record kind) scope. The first read enumerates every page. Each incremental poll reads from the last observed-up-to time minus a
   declared overlap window. A periodic bounded full inventory reconciles closures, transfers and
   removals. Absence from a partial or failed scan is never deletion. Absence from a complete
   inventory marks the record `not-in-inventory`, which is distinct from closed. A failed or partial
   scan keeps prior rows as stale and never advances the watermark. Open changes and their reviews
   come from `ReviewQueueSnapshot`; a change with `ReviewsComplete=false` keeps its review coverage
   `partial` until a per-item read completes.
5. **Centralised refresh.** Refresh requests (`scope`, optional `subject`, `reason`) are coalesced
   per scope and rate-limited with backoff. N concurrent requests produce one provider read. A mirror
   or provider outage answers stale and unavailable to every caller and never makes clients start
   their own scans.
6. **Permission loss.** A scope-level refusal (provider 401, 403 or repository-level 404 for an
   admitted scope) sets coverage `unavailable: permission`. It never deletes records, never reports
   "no open changes" and never reports "no reviews". The handling of already-mirrored content
   follows the recorded human decision; implement only the chosen option. Option 1's "only to
   callers whose own authorization still covers the repository" is Task 1's query-time admission
   check. Row 16 pins whichever option is chosen. Recovery also preserves /46’s full-inventory
   and fresh-completeness requirement; an incremental/304 result or stale Store update cannot
   clear the permission hold. This changes no recorded retention choice.
7. **Persistence.** Implement `mirror.Store` in `workflow/store/` with a versioned migration. The
   watermark advance and the complete scan's rows commit in one transaction, and the store refuses
   an advance whose scan record is not complete. Reuse /46's `mirrortest` reference conformance
   suite and extend it with this brief's durable-store requirements; run the same suite against
   the in-memory reference store and the PostgreSQL store. Restart keeps rows and
   watermarks. Deleting the mirror tables and rebuilding starts every scope at `never-read` and
   claims no earlier completeness. The durable Store owns RetainedFrom: populate it only from
   retained-history coverage qualified under the selected policy, validate its ordering against
   the other coverage stamps, and leave it unknown when that history has not been qualified.
   The pure kernel’s optional zero value is not a retained-history guarantee.
8. **Composition and floor.** Reuse /47's `tools/desk/internal/deskkit/mirrorsource.go` MirrorReadSource
   and its two provider operations, including the lower-layer refusal of a repository outside the
   credential's scope before any request. Consume /50's observation mapper in
   `tools/desk/internal/deskkit/mirroringest.go` (planned), connecting that admitted source to /46's
   mirror contracts and this brief's PostgreSQL store. Add only the joined PostgreSQL assembly
   and integration tests here; do not reimplement /50's source-only mapping. Canonicalize GitHub owner/repository case
   before constructing kernel scopes; use provider-qualified identities rather than aliases. The assembly supplies the composition-derived
   admission ID with each access and the query-time authorizer over the same admitted repository set
   to both library and store. It derives query admission from the authenticated caller; a
   request-supplied ID cannot select another admission (row 9). The earlier base-source proof does not
   replace the joined mapper/admission/store evidence.

   **Native response allocation bound, before activation (row 23).** This full-mirror owner closes
   /47's explicitly deferred single-response allocation gap for both existing GitHub and GitLab
   backends. At the admitted native mirror-page transport/read boundary, enforce at most 8 MiB of
   response JSON bytes per response, no wider than /47's aggregate operation budget. Count the bytes
   actually presented to JSON consumers after any supported content decoding, before unbounded
   full-body buffering or SDK decoding; account for oversized non-2xx/error bodies too. A Content-Length
   check alone is insufficient: absent/incorrect lengths, chunked bodies and supported compressed
   expansion cannot evade the bound. Oversize must return a typed could-not-check/limit failure with
   no successful population and no nonzero ObservedUpTo, never a valid truncated JSON prefix or
   complete-empty success. Preserve the prior complete mirror watermark and unavailable/partial
   coverage through the joined mapper; it cannot advance on this failure.
   Implement the bound once in the existing trusted deskkit response composition and apply that
   same implementation before either provider's buffering/decoding. Reuse admitted clients,
   cancellation, scope, no-redirect custody and current error classification; introduce no credential
   owner, generic transport escape hatch, network probe/default, new flag or retry loop. Scope this
   bound to the mirror's enumerated JSON page reads (including their conditional variants), not an
   incidental new global cap on unrelated log/archive/write operations. If /75 relocates that
   trusted composition into the approved shared forge library, move/reuse the same implementation
   and conformance test rather than creating another bound or treating relocation as qualification.
   A fixture-only adapter check or /47's post-fetch aggregate accounting does not satisfy row 23;
   source and service activation stay held until both native provider paths pass it.

   Add one frozen case to
   graph-execution/29's regression
   inventory: a partial mirror coverage assesses as could-not-check, never pass. Update
   `forgeread/README.md` and add the changelog fragment.


### Approved workflow-efficiency amendment — 17 October 2026

Include complete review and role-specific issue consumers in mirror scope/coverage fixtures:
requested versus admitted population, per-input observation times/versions, head/body/review/check
applicability and comment/page completeness. Bulk readers must reuse admitted refreshes and
distinguish partial/stale/permission outcomes from complete-empty; no queue-specific second mirror
or policy booleans. This is the existing R6 ingestion/custody owner. Early direct
queue/prepare/issue bridges in desk-tools/39–42 remain independent of this mirror and PostgreSQL.
Consumer-contract conformance is selected through serving /39 and final /45, not a new dependency
that blocks the mirror kernel.

This is an amendment to unfinished implementation acceptance. Preserve every prior Task, Verify
obligation, risk decision and Evidence entry. All tests named here are planned deliverables; this
authoring change claims no implementation or operational adoption.

## Verify

Named tests are implementation deliverables and fail at authoring (they do not exist yet). Each one
keeps a fail-first mutation and exercises the production path, not a mirror implementation. Fake
provider transports only.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check:ci | `cd forgeread && GOWORK=off go test -count=1 ./...` | exit 0: the whole module, root package, adapters and `mirror`, passes with no workspace file |
| 2 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorPaginationEveryPage$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorPaginationEveryPage " "$result")` | exit 0; named PASS. Both provider fixtures serve three pages, and the scan stores all records and marks the scope `complete`. A ceiling hit yields `partial`/`truncated` with the watermark unchanged. **Mutation**: stop after the first page while still reporting complete, and the test fails. Fails at authoring (test absent) |
| 3 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorPartialScanKeepsStaleRows$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorPartialScanKeepsStaleRows " "$result")` | exit 0; named PASS. A scan failing on page 2 keeps every prior row (stale) and deletes none. A record absent only from the partial pages is not marked removed. **Mutation**: treat absence from a partial scan as deletion, and the test fails |
| 4 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorPermissionLossUnavailable$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorPermissionLossUnavailable " "$result")` | exit 0; named PASS. After a complete scan, the source answers 403 and then repository-level 404. The scope reads `unavailable: permission`, the query answer carries that state and is never a bare empty list, and no record is deleted except as the chosen option directs (row 16 pins that option). **Mutation**: cache the refused read as an empty successful scan, and the test fails |
| 5 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorOverlapAndInventoryRepair$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorOverlapAndInventoryRepair " "$result")` | exit 0; named PASS. A record updated inside the overlap window between two polls is caught by the second poll. A record closed without an update event is reconciled by the bounded full inventory. A record missing from a complete inventory reads `not-in-inventory`, not closed. **Mutation**: drop the overlap window, and the straddling update is missed and the test fails |
| 6 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorRefreshCoalescesNoStorm$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorRefreshCoalescesNoStorm " "$result")` | exit 0; named PASS. 50 concurrent refresh requests for one scope cause exactly one provider read. With the provider down, every caller receives stale with `unavailable: transport`, and the fake records no second scan inside the backoff. **Mutation**: remove coalescing, and the recorded read count exceeds 1 and the test fails. A healthy cold or insufficient kernel query is fulfilled through the injected refresh port and fake source, with no second source path; source failure is cached during bounded cooldown with no extra provider requests, fake-clock expiry admits one coalesced retry and recovery clears the failure. Disabled read-through, disabled cooldown or failure-as-empty mutations fail. A partial/truncated native read with nil error remains an unfulfilled query, retains prior LastComplete/zero incomplete ObservedUpTo and kernel Partial backoff; only coverage-sufficient recovery clears failure. Per-caller admission/fairness and request-id refusal are separately pinned by /40 row 1, /42 row 4 and /45 row 2; this network-free command proves only the injected kernel refresh/backoff behavior. |
| 7 | check:ci +flow +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorStoreConformancePostgres$" ./store/... > "$result" && grep -F -- "--- PASS: TestMirrorStoreConformancePostgres " "$result")` | exit 0; named PASS (a skip for a missing harness prints no PASS and fails the row). `mirrortest` passes against real PostgreSQL in graph-execution/19's disposable harness: restart keeps rows and watermarks, and dropping the mirror tables and rebuilding yields `never-read` everywhere. With the library check bypassed, the store refuses a watermark advance on an incomplete scan record. **Mutation**: remove the store constraint, and the bypassed advance commits and the test fails. Unqualified retained history keeps RetainedFrom unknown even when old visible rows exist; qualified history uses the persisted coverage bound with valid stamp ordering. Mutating production derivation to use the oldest visible row instead of qualified coverage must fail this named PostgreSQL test. |
| 8 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorConditionalRefresh$" ./... > "$result" && grep -F -- "--- PASS: TestMirrorConditionalRefresh " "$result")` | exit 0; named PASS. A not-modified answer refreshes only that scope's last-complete time, with rows unchanged. A source without validators reads in full and records `conditional: unsupported`. A not-modified answer to a partial earlier scan does not promote it to complete. **Mutation**: let not-modified promote a partial scope, and the test fails |
| 9 | check:ci +flow +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorSourceLowerScope$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestMirrorSourceLowerScope " "$result")` | exit 0; named PASS. With the mirror's scope check bypassed, the composed source refuses a repository outside the credential's scope before any request (zero requests recorded at the fake), and an in-scope control repository reads. Both provider fixtures give the same mirror records through the adapters and through deskkit's delegating methods. The real query composition receives authenticated B plus request input claiming A; it supplies B to the mirror and returns permission refusal for A's scope, while authenticated A is a positive control. **Mutation**: pass the request-supplied admission through instead of deriving it from the authenticated caller, and B reads A's rows and the test fails. **Mutation**: remove the composition's scope check, and the request reaches the fake and the test fails. Case-variant GitHub owner/repository inputs through actual composition yield one canonical kernel scope and one admission-bound cache population. A permission hold cannot be bypassed via a differently cased alias. Mutating the production mapper to skip case canonicalization creates a second scope and fails the named test. |
| 10 | check +dereference +mutation | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: statusgen links no subpackage of `forgeread`, `mirror` included, so this brief does not rebind forge-neutral/18. It is forge-neutral/36 row 10, unchanged. **Mutation**: a planted statusgen import of `.../forgeread/mirror` counts `1` |
| 11 | check +dereference +mutation | `test -d forgeread/mirror && (cd forgeread && GOWORK=off go list -deps ./mirror > "${TMPDIR:-/tmp}/b37-r11.out") && { grep -x -F -e net/http -e os/exec -e database/sql "${TMPDIR:-/tmp}/b37-r11.out" \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: the mirror library's dependency closure has no HTTP client, no process launch and no database driver. Transport and persistence enter only through the source and the store. A missing package prints nothing and fails the row (it fails at authoring). **Mutation**: a mirror file importing `net/http` counts `1` |
| 12 | check:ci +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestForgeSurfaceUnchangedByDeskread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestForgeSurfaceUnchangedByDeskread " "$result" && GOWORK=off go test -count=1 -v -run "^TestForgeGitlabCoverage$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestForgeGitlabCoverage " "$result" && GOWORK=off go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | exit 0: the surface want list and the GitLab inventory reconcile with exactly the two new operations, and the ambient-token rule finds no forge-token read in `forgeread/`, `mirror` included. That rule recognises only six forge-token variable names; row 18 covers every other environment read. **Mutation**: add a third operation without an inventory row, and `TestForgeGitlabCoverage` fails. **Mutation**: a mirror file reading `GH_TOKEN` from the environment produces a finding, and `TestAmbientTokenGuardCoversForgeread` fails |
| 13 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out" && grep -rlF 'mirror-partial-is-could-not-check' testdata/regressionfloor/)` | exit 0: graph-execution/29's floor passes with the added frozen case, which names the mirror-partial case file. **Mutation**: make the case expect pass for partial coverage, and the floor fails |
| 14 | check:ci +dereference | `statusgen --consumers --root .` | exit 0: the diff-aware consumers gate corroborates /37's actual integration/extension consumers against its implementing diff. Kernel/reference creation remains routed to /46; incremental-method/base-source/inventory creation remains routed to /47. Only /37 self-references edited by its integration diff flip to fixed-here; inherited contribution paths do not require duplicate creation or fabricated current-diff evidence. Out-of-scope judgments remain independently reviewed, and no prior contribution/acceptance criterion is waived. |
| 15 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorQueryAdmissionScope$" ./mirror/... > "$result" && grep -F -- "--- PASS: TestMirrorQueryAdmissionScope " "$result")` | exit 0; named PASS. A complete scope under admission A contains a canary row; currently authorized A reads it as a positive control. Missing or invalid admission, uncovered B, and **covered-but-different B** each answer `unavailable: permission` with zero rows, never complete-empty. The library refuses before any scope-data query (spy call count zero). With the library check bypassed, the store independently returns the same typed permission refusal for those cases, not just a filtered empty list; with a permissive store double, the library still refuses. After A's coverage is withdrawn, its next query is refused in each layer. Only authorized A querying a genuinely empty scope may receive complete-empty. This production-path case is part of `mirrortest`, including PostgreSQL under row 7. **Mutation**: remove library admission equality while retaining coverage; covered B reaches the store and the zero-call assertion fails. **Mutation**: turn the store's nonmatching-admission refusal into a successful empty result; the bypass case fails on coverage/reason even though zero rows remain. **Mutation**: drop the store admission predicate with the library bypassed; B receives A's canary and the test fails. **Mutation**: cache authorization at ingestion; A still reads after withdrawal and the test fails |
| 16 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorPermissionLossRetention$" ./mirror/... > "$result" && grep -F -- "--- PASS: TestMirrorPermissionLossRetention " "$result")` | exit 0; named PASS. **Decision-conditional**: the worker implements only after the human decision is recorded, and the edit that records it deletes the unchosen option's sentence and mutation from this cell. Option 1 (stale, access-checked history): after confirmed permission loss every prior row remains, marked stale and permission-lost, served only with that coverage and only through row 15's query-time admission check; rows past the configured history window are gone. Option 2 (purge to tombstones): after confirmed permission loss, a scan of every stored field of the scope finds none of the fixture's private canary strings (titles, bodies, labels, review text), while identities, the coverage gap and the loss time remain. The case is part of `mirrortest`, so it also runs against PostgreSQL under row 7. **Mutation (option 1)**: serve stale rows without the admission check, and the test fails. **Mutation (option 2)**: make the purge a no-op, and a canary is found and the test fails An oversized/truncated 404 with only status classification is an unqualified failure: within the fixture retention window it preserves private canaries and cannot authorize purge or prove authoritative absence. A separately qualified repository-permission-loss positive control still applies the selected policy; status-alone-to-purge mutation fails. |
| 17 | check +mutation | `test -f forgeread/go.mod && { grep -n -w -e require -e replace forgeread/go.mod \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: `forgeread` keeps no module dependency, so no subpackage can import `statusgen/domain` or a `workflow` package, and no forgeread-to-statusgen require cycle can form. The graph-execution/28 mapping stays in `workflow`. Passes at authoring as an invariant guard. **Mutation**: a planted `require github.com/medici-finance/assay/statusgen v0.0.0` line counts `1` |
| 18 | check:ci +mutation | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run tools/forge-import-check/check_env_imports.go forgeread/mirror` | exit 0 and output `0`: the shared source-only checker parses every non-test Go file below the target and decodes each import path with `strconv.Unquote`. No decoded `os` or `syscall` import is allowed, including raw, escaped, plain, aliased and dot imports. Build-constrained files are scanned regardless of the current platform; `*_test.go` files remain excluded. Missing/empty targets, parse/decode/walk errors and symbolic links fail closed with no clean count. It fails at authoring because the target package is absent. The companion `check_env_imports_test.go` compiles synthetic raw/escaped imports offline without calling their environment-read functions, then runs both rows' exact commands; ordinary aliases, build-tagged files, excluded tests and clean imports remain controls. **Mutation**: use the former literal matcher or stop decoding import literals; compiler-valid raw/escaped `os` and `syscall` plants evade rejection and the fixture test fails. **Mutation**: plant any of those imports in shipped `mirror` source; the row reports a nonzero count and exit status |
| 19 | check:ci +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorCoverageFactMapping$" ./store/... > "$result" && grep -F -- "--- PASS: TestMirrorCoverageFactMapping " "$result")` | exit 0; named PASS. Every mirror coverage state maps to exactly one graph-execution/28 fact-completeness value, and only `complete` inside the freshness window maps to present: `never-read`, `partial`, each `unavailable` reason class and a stale `complete` never do. The mapping lives in `workflow/store/`, not in `forgeread`. **Mutation**: map `partial` to present, and the test fails |
| 20 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorReviewCoverage$' ./mirror/... > "$o" && grep -F -- '--- PASS: TestMirrorReviewCoverage ' "$o")` | exit 0; named PASS. Admitted review/issue populations preserve exact head and partial/denied/page coverage; removing a required input cannot return complete-empty or leak another admission. |

| # | Class | Command | Expect |
|---|---|---|---|
| 21 | check:ci +flow +mutation | `(cd forgeread && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run '^TestRefreshBurstScopeAndCallBudget$' ./mirror/... > "$out" && grep -F -- '--- PASS: TestRefreshBurstScopeAndCallBudget ' "$out")` | exit 0; named PASS; 50 equivalent requests share one paginated inventory, two admissions remain isolated and outage retries stay within the explicit fixture budget; global authorization sharing or per-client scan fallback fails |
| 22 | check:ci +flow +mutation | `(cd forgeread && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorPlannerReuse$' ./... > "$o" && grep -F -- '--- PASS: TestMirrorPlannerReuse ' "$o")` | exit 0; named PASS. compatible prose/discrete consumers query the same mirror cutoff without new provider reads; one changed item refreshes affected scope, and partial/removal/access/expiry failures cannot yield complete idle. Competing caller pollers or hidden producer attempts fail; final refresh requests remain explicit. |
| 23 | check:ci +flow +mutation | `(cd tools/desk && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestMirrorResponseBounds$' ./internal/deskkit/ > "$o" && grep -F -- '--- PASS: TestMirrorResponseBounds ' "$o")` | Exit 0; named PASS. Actual admitted GitHub and GitLab backend-to-source-to-mapper paths accept a useful below-bound population and an exact-bound JSON control. Every oversized control contains syntactically valid JSON, so decode failure cannot substitute for enforcing the cap. Known-length, absent/false Content-Length and chunked oversized controls, plus supported compressed expansion, refuse before native full-body accumulation/SDK decode; a counting streaming body proves no more than bound plus one sentinel byte is consumed by the guarded read. Both providers return a typed limit/could-not-check, zero successful population and zero ObservedUpTo; the assembled mirror's prior complete watermark stays unchanged. Run each real path with the upper aggregate/mapper budget bypassed so that only its native response guard can catch overflow. Independently mutate the actual GitHub and GitLab native guard composition to bypass the cap (or only trust Content-Length); the named TestMirrorResponseBounds must fail its corresponding oversized control for each bypass, while the restored production guard passes both providers. A valid JSON prefix followed by oversized whitespace must never become complete-empty. No production endpoint or deployed-capacity claim. |

## Pre-mortem

An unfinished walk reads as the whole population (rows 2, 8). A failed scan deletes or hides rows
that still exist (row 3). A refused read becomes "nothing open" (row 4). A missed update or a silent
closure never reaches the mirror (row 5). A mirror outage turns into every client polling the forge
(row 6). A crash or a bypassed library check advances the watermark on incomplete data (row 7). The
shared source reads another repository's scope (row 9). statusgen starts linking the mirror and
/18's credential limit erodes (row 10). The library grows its own transport or driver (row 11). New
reads slip past the frozen inventory, or a forge token is read from the environment (row 12). Mirror uncertainty rounds to
pass at assessment (row 13). A reader admitted for one repository reads rows admitted under another
admission, or a covered but different admission receives complete-empty (row 15). Request input
impersonates an admission instead of composition deriving it (row 9). The chosen retention option is left unimplemented (row 16). `forgeread` gains a
dependency on statusgen and a module cycle forms (row 17). A mirror file reads configuration or a
secret through a direct `os` or `syscall` import under a name the ambient-token rule does not know (row 18; cgo is outside that check). Partial
coverage maps to a present fact (row 19). Retention of private content after access loss remains the
human decision above; row 16 pins whichever option is chosen.

## Performance reuse amendment — 2026-10-16

Keep the mirror as the single online refresh/coalescing owner; do not start another
poller or ingest cache from a desk command. Extend the existing refresh tests with a
50-caller same-admission burst and two simultaneously different admissions. Exactly one
scripted paginated inventory should serve equivalent concurrent refresh demand; different
provider/host/repository/permission or admission/access epochs cannot share authorization.
Requests on permission loss, stale data or provider failure retain coverage/backoff rather
than triggering one full scan per caller. Count provider calls, coalesced waiters, bytes and
cache outcomes at the trusted composition boundary; no credential or recorder dependency
is imported into forgeread's offline root. The 50-caller case is synthetic request-count
acceptance, not a production latency threshold. Existing /09 review snapshots and conditional
board reads are baseline features, not newly credited savings.


## Perf-v3 delivery amendment — 19 October 2026

For this brief's existing performance observations, reuse the [perf-v3 contract](../../performance-attribution-spec.md) and /63–65's
shared typed context and request-cost seam when those adapters are available. Each process
records its own attempts, timing and observed GraphQL points with units/basis/coverage;
client-to-service and service-to-provider work are distinct. Carry admitted parent invocation
links when known, preserve missing context and never turn it into operational authority.
No extra provider request or expanded business query for monitoring; qualified bounded cost
metadata may join the same request. No copied descendant call total,
and no v2-only replacement instrument. Keep this brief's scope/Verify/Evidence and independent
service delivery; record unsupported boundaries until producer integration. /45's existing
matched service qualification must include own-request and parent/child no-double-count cases.

## Query/cache measurement amendment — 19 October 2026

Apply docs/performance-attribution-spec.md, including its query/cache boundary contract.

Mirror refresh, pagination, background polling and repair retain their own bounded origin and shared refresh identity. A coalesced source walk counts each actual page/retry once and is not multiplied by waiter count. Missing upstream instrumentation is unknown; zero is measured only under full seam coverage.

Extend this brief's existing production-flow Verify cases with these boundary/false-count
negative fixtures; all existing Tasks, exact selectors and Evidence remain obligations.
No service activation or extra dependency on a live collector is introduced.

## Evidence

<!-- Independent verifier records subject revision, results and retained negative controls. -->

## Review

Gate: human (sensitive-data). The reviewer confirms the coverage states stay honest under partial,
refused and failed reads, and records the permission-loss retention option chosen. Approval
activates nothing; activation stays gated per `docs/proposals/non-ci-architecture/16-delivery-sequence.md`.

## Write-through follow-on — 2026-10-14

The new forge-neutral/39–45 briefs consume this contract. The service maps an authenticated
principal to this mirror's exact composition-derived admission; it does not relax the
query-time/store admission rule or answer the existing retention/payload decision. Confirmed
provider receipts feed mirror updates through the existing R2/R4 owners. This dated note and
reciprocal edges change no historical Task, Verify, Evidence or human decision.

## Checker packaging correction — 2026-10-14

Move the unchanged source-only checker and its compiler-fixture harness to
`tools/forge-import-check/`. The exact Verify commands now name that path. The desk CI
bridge references this neutral developer-tool path; its dependency registry covers the
helper plus the actual authored-row input through whole-stream-root coverage. No withheld
stream name ships in desk source, and neither corpus nor secret guard is exempted or weakened.
The standalone harness still reads and executes the actual two Verify commands and all
22 compiler cases. Human retention choices and historical Evidence remain unchanged.

## Bounded PostgreSQL persistence contribution — 3 November 2026

[52](brief-52-canonical-postgresql-mirror-store-without-execution-dependencies.md) owns
only the real PostgreSQL Store adapter and SQL persistence checks, extending the single
graph-execution/36 migration ledger and container fixture. This brings forward durable
mirror storage without action admission, execution claims or outboxes. The canonical
shared refresh Gate, AttemptAuthority and admitted source/coordinator SQL join belong
to /54. This parent consumes those owners at trusted deskkit composition and retains
its independent persisted acceptance in row23. All23 original Class/Command pairs
and existing human gates remain unchanged.

## Read-through coordinator ownership — 3 November 2026

The bounded contribution assay:assay:forge-neutral:51 supplies the store-injected
coordinator and typed read-refresh gate port over the existing mirror kernel. This
brief consumes that coordinator with /54's canonical R2 PostgreSQL Gate and
AttemptAuthority and /52's mirror.Store at trusted deskkit composition. Those owners
supply generation-bound apply/completion, shared retry eligibility, finite
caller/admission accounting and independent-process/restart controls. This parent
reuses their implementations; it adds no second Gate, quota owner, Store or execution
claim.
The synthetic gate/reference-store profile does not discharge the real mapper,
durable Store and preserved-watermark join in row23. Every existing Task, all23
Class/Command pairs, retention decision, acceptance gate and historical Evidence
remain unchanged.
