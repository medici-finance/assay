---
brief: assay:assay:forge-neutral:37
title: Scoped forge mirror with persisted freshness, coverage and polling repair
why: "Every desk, status run and planning read asks the forge the same questions again, one verb at a time, and a throttled or partial read can look like an empty queue. One scoped mirror that records what it read, when, and how completely lets those readers share one answer and see when it is stale or incomplete, without each of them polling the forge."
wave: 7
depends: ["graph-execution/28", "graph-execution/29", "graph-execution/19", "forge-neutral/36", "forge-neutral/33", "desktools-v2/09"]
unblocks: ["forge-neutral/38", "forge-neutral/41", "forge-neutral/43"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "sensitive-data is yes because the mirror copies private pull request, merge request, issue and review content into a database that outlives a single read. The human confirms that rows are stored only for an admitted scope, that losing permission makes coverage visibly unavailable instead of silently empty, and what happens to already-mirrored content when that permission is lost (the decision below)."
issues: []
outcome: none
schema: brief-v2
version: 1
id: 06dc3c03-bfb0-49cf-b171-8f7d192b7aea
authored: 2026-10-13 by statusgen newbrief (non-CI R6 authoring follow-up)
sources:
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
exec-tier: strong
exec-tier-why: "Question (c), silent failure toward a pass: an unfinished scan that advances a watermark, an absence read as deletion, or a refused read cached as an empty list all pass a happy-path test and hand every consumer a confident wrong answer."
domain: complicated
decision-trigger: start
consumers:
  - "forgeread/mirror/ (new), forgeread/mirror/mirrortest/ (new): follow-up forge-neutral/37"
  - "forgeread/adapters/, forgeread/adapters/github/, forgeread/adapters/gitlab/: follow-up forge-neutral/37 (the two incremental list reads and the optional conditional page access of Task 3)"
  - "tools/desk/internal/deskkit/forge.go, forge_github.go, forge_gitlab.go, sharedread.go, forge_surface_deskread_test.go: follow-up forge-neutral/37 (the two new operations delegate to the shared adapters; the surface test's want list gains exactly their names)"
  - "tools/desk/internal/deskkit/mirrorsource.go (new): follow-up forge-neutral/37 (the trusted composition that supplies the mirror its source)"
  - "docs/streams/forge-gitlab/inventory.md: follow-up forge-neutral/37 (two inventory rows at the next free numbers)"
  - "workflow/store/ (as planned by graph-execution/19 after its R2 amendment): follow-up forge-neutral/37 (the mirror tables, their migration, the mirror.Store implementation and the graph-execution/28 fact-completeness mapping)"
  - "statusgen/testdata/regressionfloor/ (as planned by graph-execution/29): follow-up forge-neutral/37 (one frozen mirror-coverage case under 29's inventory rules)"
  - "statusgen/: out-of-scope (forge-neutral/18 owns statusgen's forge reads; this brief adds no statusgen link and no deskread kind)"
  - "comments, check and status observations, event delivery and reevaluation demand: follow-up forge-neutral/38"
---

# Brief 37 — Scoped forge mirror with persisted freshness, coverage and polling repair

## Context

files: `forgeread/mirror/` (new), `forgeread/mirror/mirrortest/` (new), `forgeread/adapters/access.go`,
`forgeread/adapters/github/`, `forgeread/adapters/gitlab/`, `forgeread/README.md`,
`tools/desk/internal/deskkit/forge.go`, `tools/desk/internal/deskkit/forge_github.go`,
`tools/desk/internal/deskkit/forge_gitlab.go`, `tools/desk/internal/deskkit/sharedread.go`,
`tools/desk/internal/deskkit/mirrorsource.go` (new) and tests,
`tools/desk/internal/deskkit/forge_surface_deskread_test.go`, `docs/streams/forge-gitlab/inventory.md`,
`workflow/store/` (planned by graph-execution/19; mirror tables, migration and the
graph-execution/28 fact-completeness mapping),
`statusgen/testdata/regressionfloor/` (planned by graph-execution/29; one case),
`changelog/forge-mirror-polling.md` (new).

facts (4696d598e, 2026-10-13):
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
observations is supplied by trusted composition in desk-tools (`mirrorsource.go` (planned)) over existing and new
adapters. PostgreSQL persistence implements `Store` inside the R2 store. Rows 2, 10, 11 and 18 hold the split.
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

Default if no answer: none. The mirror stays unactivated and the brief stays held until answered.

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
- If graph-execution/19's R2 amendment has not landed, or its store has no migration and test
  harness for an added table, stop and report NEEDS_CONTEXT. Do not build a second store.
- Stop at `implemented`; an independent verifier owns verified and done.

## Task

1. **Mirror model (`forgeread/mirror`).** Define typed records: scope (provider, host, repository,
   record kind: change or issue); change and issue observations with PR versus MR provenance, state,
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
3. **Reads.** Add exactly two operations, `ChangesUpdatedSince` and `IssuesUpdatedSince`, for both
   providers in `forgeread/adapters/{github,gitlab}`. Each lists records updated at or after a bound,
   ordered by update time, walks every page up to a ceiling and returns `Incomplete` at the ceiling,
   never a short list. deskkit's methods delegate to them, as forge-neutral/36's OpenIssues does.
   Add an optional `ConditionalAccess` beside `adapters.Access`: one page with a supplied validator,
   answering not-modified or a page with its new validator. Where the provider or endpoint gives no
   validator, the read happens in full and the scan records `conditional: unsupported`. Inventory
   rows and the surface test's want list gain exactly the two names.
4. **Refresh planner and polling repair.** One ingestion owner per (admission ID, repository,
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
   check. Row 16 pins whichever option is chosen.
7. **Persistence.** Implement `mirror.Store` in `workflow/store/` with a versioned migration. The
   watermark advance and the complete scan's rows commit in one transaction, and the store refuses
   an advance whose scan record is not complete. Ship `mirrortest`, one conformance suite run
   against the in-memory reference store and the PostgreSQL store. Restart keeps rows and
   watermarks. Deleting the mirror tables and rebuilding starts every scope at `never-read` and
   claims no earlier completeness.
8. **Composition and floor.** `tools/desk/internal/deskkit/mirrorsource.go` (planned) implements the mirror's
   source over the already-admitted access deskkit composes; it refuses a repository outside the
   credential's scope before any request. It supplies the admission ID with each access, and the
   query-time authorizer over the same admitted repository set to both library and store. Query
   composition derives the admission from the authenticated caller; a request-supplied ID cannot
   select another admission (row 9). Add one frozen case to
   graph-execution/29's regression
   inventory: a partial mirror coverage assesses as could-not-check, never pass. Update
   `forgeread/README.md` and add the changelog fragment.

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
| 6 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorRefreshCoalescesNoStorm$" ./mirror/ > "$result" && grep -F -- "--- PASS: TestMirrorRefreshCoalescesNoStorm " "$result")` | exit 0; named PASS. 50 concurrent refresh requests for one scope cause exactly one provider read. With the provider down, every caller receives stale with `unavailable: transport`, and the fake records no second scan inside the backoff. **Mutation**: remove coalescing, and the recorded read count exceeds 1 and the test fails |
| 7 | check:ci +flow +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorStoreConformancePostgres$" ./store/... > "$result" && grep -F -- "--- PASS: TestMirrorStoreConformancePostgres " "$result")` | exit 0; named PASS (a skip for a missing harness prints no PASS and fails the row). `mirrortest` passes against real PostgreSQL in graph-execution/19's disposable harness: restart keeps rows and watermarks, and dropping the mirror tables and rebuilding yields `never-read` everywhere. With the library check bypassed, the store refuses a watermark advance on an incomplete scan record. **Mutation**: remove the store constraint, and the bypassed advance commits and the test fails |
| 8 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorConditionalRefresh$" ./... > "$result" && grep -F -- "--- PASS: TestMirrorConditionalRefresh " "$result")` | exit 0; named PASS. A not-modified answer refreshes only that scope's last-complete time, with rows unchanged. A source without validators reads in full and records `conditional: unsupported`. A not-modified answer to a partial earlier scan does not promote it to complete. **Mutation**: let not-modified promote a partial scope, and the test fails |
| 9 | check:ci +flow +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorSourceLowerScope$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestMirrorSourceLowerScope " "$result")` | exit 0; named PASS. With the mirror's scope check bypassed, the composed source refuses a repository outside the credential's scope before any request (zero requests recorded at the fake), and an in-scope control repository reads. Both provider fixtures give the same mirror records through the adapters and through deskkit's delegating methods. The real query composition receives authenticated B plus request input claiming A; it supplies B to the mirror and returns permission refusal for A's scope, while authenticated A is a positive control. **Mutation**: pass the request-supplied admission through instead of deriving it from the authenticated caller, and B reads A's rows and the test fails. **Mutation**: remove the composition's scope check, and the request reaches the fake and the test fails |
| 10 | check +dereference +mutation | `test -d statusgen && { grep -rnE --include='*.go' --exclude='*_test.go' '"([^"]*/)?forgeread/[^"]+"' statusgen \|\| [ $? -eq 1 ]; } \| { grep -v -E '^[^:]+:[0-9]+:[[:space:]]*//' \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: statusgen links no subpackage of `forgeread`, `mirror` included, so this brief does not rebind forge-neutral/18. It is forge-neutral/36 row 10, unchanged. **Mutation**: a planted statusgen import of `.../forgeread/mirror` counts `1` |
| 11 | check +dereference +mutation | `test -d forgeread/mirror && (cd forgeread && GOWORK=off go list -deps ./mirror > "${TMPDIR:-/tmp}/b37-r11.out") && { grep -x -F -e net/http -e os/exec -e database/sql "${TMPDIR:-/tmp}/b37-r11.out" \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: the mirror library's dependency closure has no HTTP client, no process launch and no database driver. Transport and persistence enter only through the source and the store. A missing package prints nothing and fails the row (it fails at authoring). **Mutation**: a mirror file importing `net/http` counts `1` |
| 12 | check:ci +mutation | `(cd tools/desk && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestForgeSurfaceUnchangedByDeskread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestForgeSurfaceUnchangedByDeskread " "$result" && GOWORK=off go test -count=1 -v -run "^TestForgeGitlabCoverage$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestForgeGitlabCoverage " "$result" && GOWORK=off go test -count=1 -v -run "^TestAmbientTokenGuardCoversForgeread$" ./internal/deskkit/ > "$result" && grep -F -- "--- PASS: TestAmbientTokenGuardCoversForgeread " "$result")` | exit 0: the surface want list and the GitLab inventory reconcile with exactly the two new operations, and the ambient-token rule finds no forge-token read in `forgeread/`, `mirror` included. That rule recognises only six forge-token variable names; row 18 covers every other environment read. **Mutation**: add a third operation without an inventory row, and `TestForgeGitlabCoverage` fails. **Mutation**: a mirror file reading `GH_TOKEN` from the environment produces a finding, and `TestAmbientTokenGuardCoversForgeread` fails |
| 13 | check:ci +mutation | `(cd statusgen && out=$(mktemp) && trap 'rm -f "$out"' 0 && GOWORK=off go test -count=1 -v -run "^TestRegressionFloorOldNewAgree$" . > "$out" && grep -F -- "--- PASS: TestRegressionFloorOldNewAgree " "$out" && grep -rlF 'mirror-partial-is-could-not-check' testdata/regressionfloor/)` | exit 0: graph-execution/29's floor passes with the added frozen case, which names the mirror-partial case file. **Mutation**: make the case expect pass for partial coverage, and the floor fails |
| 14 | check:ci +dereference | `statusgen --consumers --root .` | exit 0: the diff-aware consumers gate corroborates every routing token against the implementing branch's diff (each `follow-up forge-neutral/37` self-reference is satisfied by this brief's own changes and flips to `fixed-here` at implementation) |
| 15 | check:ci +flow +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorQueryAdmissionScope$" ./mirror/... > "$result" && grep -F -- "--- PASS: TestMirrorQueryAdmissionScope " "$result")` | exit 0; named PASS. A complete scope under admission A contains a canary row; currently authorized A reads it as a positive control. Missing or invalid admission, uncovered B, and **covered-but-different B** each answer `unavailable: permission` with zero rows, never complete-empty. The library refuses before any scope-data query (spy call count zero). With the library check bypassed, the store independently returns the same typed permission refusal for those cases, not just a filtered empty list; with a permissive store double, the library still refuses. After A's coverage is withdrawn, its next query is refused in each layer. Only authorized A querying a genuinely empty scope may receive complete-empty. This production-path case is part of `mirrortest`, including PostgreSQL under row 7. **Mutation**: remove library admission equality while retaining coverage; covered B reaches the store and the zero-call assertion fails. **Mutation**: turn the store's nonmatching-admission refusal into a successful empty result; the bypass case fails on coverage/reason even though zero rows remain. **Mutation**: drop the store admission predicate with the library bypassed; B receives A's canary and the test fails. **Mutation**: cache authorization at ingestion; A still reads after withdrawal and the test fails |
| 16 | check:ci +mutation | `(cd forgeread && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorPermissionLossRetention$" ./mirror/... > "$result" && grep -F -- "--- PASS: TestMirrorPermissionLossRetention " "$result")` | exit 0; named PASS. **Decision-conditional**: the worker implements only after the human decision is recorded, and the edit that records it deletes the unchosen option's sentence and mutation from this cell. Option 1 (stale, access-checked history): after confirmed permission loss every prior row remains, marked stale and permission-lost, served only with that coverage and only through row 15's query-time admission check; rows past the configured history window are gone. Option 2 (purge to tombstones): after confirmed permission loss, a scan of every stored field of the scope finds none of the fixture's private canary strings (titles, bodies, labels, review text), while identities, the coverage gap and the loss time remain. The case is part of `mirrortest`, so it also runs against PostgreSQL under row 7. **Mutation (option 1)**: serve stale rows without the admission check, and the test fails. **Mutation (option 2)**: make the purge a no-op, and a canary is found and the test fails |
| 17 | check +mutation | `test -f forgeread/go.mod && { grep -n -w -e require -e replace forgeread/go.mod \|\| [ $? -eq 1 ]; } \| wc -l` | output is `0`: `forgeread` keeps no module dependency, so no subpackage can import `statusgen/domain` or a `workflow` package, and no forgeread-to-statusgen require cycle can form. The graph-execution/28 mapping stays in `workflow`. Passes at authoring as an invariant guard. **Mutation**: a planted `require github.com/medici-finance/assay/statusgen v0.0.0` line counts `1` |
| 18 | check:ci +mutation | `GOWORK=off GOPROXY=off GOTOOLCHAIN=local go run docs/streams/forge-neutral/check_env_imports.go forgeread/mirror` | exit 0 and output `0`: the shared source-only checker parses every non-test Go file below the target and decodes each import path with `strconv.Unquote`. No decoded `os` or `syscall` import is allowed, including raw, escaped, plain, aliased and dot imports. Build-constrained files are scanned regardless of the current platform; `*_test.go` files remain excluded. Missing/empty targets, parse/decode/walk errors and symbolic links fail closed with no clean count. It fails at authoring because the target package is absent. The companion `check_env_imports_test.go` compiles synthetic raw/escaped imports offline without calling their environment-read functions, then runs both rows' exact commands; ordinary aliases, build-tagged files, excluded tests and clean imports remain controls. **Mutation**: use the former literal matcher or stop decoding import literals; compiler-valid raw/escaped `os` and `syscall` plants evade rejection and the fixture test fails. **Mutation**: plant any of those imports in shipped `mirror` source; the row reports a nonzero count and exit status |
| 19 | check:ci +mutation | `(cd workflow && result=$(mktemp) && trap 'rm -f "$result"' 0 && GOWORK=off go test -count=1 -v -run "^TestMirrorCoverageFactMapping$" ./store/... > "$result" && grep -F -- "--- PASS: TestMirrorCoverageFactMapping " "$result")` | exit 0; named PASS. Every mirror coverage state maps to exactly one graph-execution/28 fact-completeness value, and only `complete` inside the freshness window maps to present: `never-read`, `partial`, each `unavailable` reason class and a stale `complete` never do. The mapping lives in `workflow/store/`, not in `forgeread`. **Mutation**: map `partial` to present, and the test fails |

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
secret from the environment under a name the ambient-token rule does not know (row 18). Partial
coverage maps to a present fact (row 19). Retention of private content after access loss remains the
human decision above; row 16 pins whichever option is chosen.

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
