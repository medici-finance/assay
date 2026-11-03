---
brief: assay:assay:graph-execution:36
title: Canonical PostgreSQL foundation for read-only consumers
why: "Read-only mirror persistence needs the canonical PostgreSQL migration and test substrate, but not workflow execution. Deliver that substrate first so shared reads do not wait for admission, outbox or executor integration."
wave: 0
depends: []
unblocks: ["graph-execution/19"]
split-from: graph-execution/19
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new CI job executes repository source with runner and checkout-token access, and the explicit database configuration handles credentials. Human acceptance reviews this boundary after disposable-fixture evidence; no operational service or database is authorized."
decision-trigger: spec
issues: []
schema: brief-v2
version: 2
outcome: none
id: d632866d-1170-4aff-bb14-2344b2f786c1
authored: 2026-11-03 by statusgen newbrief
sources:
- docs/streams/graph-execution/brief-19-durable-instance-store.md
- docs/proposals/non-ci-architecture/14-storage-forge-graph.md
- docs/proposals/non-ci-architecture/15-regression-from-day-one.md
- docs/proposals/non-ci-architecture/20-amendment-manifest.md
- docs/library-first.md
- docs/go-engineering.md
- "freshness-checked 2026-11-03 against App-fetched main 863c43dbcd84d6f05ac86f9197c586e14a3a3828"
exec-tier: strong
exec-tier-why: "Migration atomicity, concurrent initialisation and schema compatibility must be checked on real PostgreSQL rather than an in-memory substitute."
domain: complicated
consumers:
- "workflow/store connection, migrations and test harness: follow-up graph-execution/36 (this brief; implementation owns these paths)"
- "workflow/store domain tables, admission, outbox and reconstruction: follow-up graph-execution/19"
- "workflow/store mirror persistence: out-of-scope (no SQL mirror caller exists; separate /37 precursor amendment must bind this ledger before partial dispatch)"
- "loopadmin supervisor persistence: out-of-scope (same engine, separate schema and no Go dependency on this foundation)"
- "forgeread/mirror: out-of-scope (pure kernel keeps its injected Store port and no SQL dependency)"
---

# Brief 36 — Canonical PostgreSQL foundation for read-only consumers

## Context

files: `workflow/go.mod` (planned), `workflow/go.sum` (planned), `workflow/README.md` (planned), `workflow/store/` (planned), `workflow/store/migrations/` (planned), `workflow/internal/pgtest/` (planned), `workflow/testdata/compose.yaml` (planned), `spec/coordination-store-v1.md` (planned), `docs/contracts.md`, `.github/workflows/assay-workflow.yml` (planned), `tools/desk/internal/deskkit/citrigger_test.go` (only if this repository's current cross-module registry requires it), `changelog/graph-execution-36-pg-foundation.md` (planned).

facts:
- At App-fetched main `863c43db`, `workflow/`, the coordination-store contract and the workflow CI job do not exist. The /19 contract remains version 8 and contains 13 unfulfilled Verify rows.
- The approved R2 engine is PostgreSQL, with one coordination database/restricted application role per cell. SQLite and a second online coordination backend are forbidden; approval activates nothing.
- The current `forgeread/mirror` kernel owns query/scan semantics through injected ports. A SQL implementation must extend this foundation later; this brief supplies no mirror table or Store implementation.
- Open authoring PRs 5033 and 4429 add desk-instance records and a restore row to full /19. Their inspected patches add no foundation implementation. Preserve that future scope; this split does not supersede either proposal or qualify desk-instance restore.
- Milestone/brief-v4 reader adoption is not established for all required readers. Use supported brief-v2 identifiers and prose outcome mapping here, with no operational milestone declarations.

layering: R2 remains the only coordination-store owner. Keep connection/migration effects in `workflow/store` and disposable fixture lifecycle in `workflow/internal/pgtest`; no new command, daemon, backend seam, domain authority or scheduling implementation. Rows 1–7 test the actual driver and PostgreSQL boundary.

design-fit:
  owner: R2 workflow/store
  contract: "coordination-store-v1; foundation migration ledger implemented, domain schemas explicitly planned"
  retires: []
  weight: "verbs 0; flags 0; online backends 0; one planned module/driver/migration ledger brought forward"
  why-add: "Reuse /19's canonical owner rather than a mirror-specific database or temporary embedded backend. Removing the foundation would force consumers to duplicate migrations and fixture lifecycle."

single-point-of-failure: a connection opened without compatibility checks could use a partially migrated schema. The migration entry point independently serializes and verifies the recorded manifest before commit; subsequent domain adapters must recheck their own schema floor. This brief grants no capability to execute work.

## Read first

- [Full store delivery](brief-19-durable-instance-store.md), all Tasks and 13 Verify rows.
- [R2 routing](README.md#non-ci-r2-routing--postgresql-coordination--2026-10-13).
- [One store and reconstruction](../../proposals/non-ci-architecture/14-storage-forge-graph.md) and [real PostgreSQL qualification](../../proposals/non-ci-architecture/15-regression-from-day-one.md).
- `docs/library-first.md`, `docs/go-engineering.md`, `spec/brief-v1.md` §4.1 and `docs/contracts.md`.

## Dependency admission

Reuse pgx/v5 v5.12.0 for PostgreSQL and Goose/v3 v3.29.0 for migration mechanics, per `docs/go-engineering.md`. The version-specific source/permissions/persistence assessment is medici-finance/assay#5396. Installation of the under-seven-day pgx release requires the driver's own bless on that issue before it proceeds; authoring may continue without installing it. Goose is older than seven days. No custom pool, protocol parser or migration runner.

## Ground rules

- Own isolated branch and draft PR only; no merge, release, deployment, activation, cluster or production contact.
- Tests use explicit disposable PostgreSQL input and synthetic data. Never discover a host's DSN, credential, service, Docker context or kubeconfig. No default network bootstrap; an unavailable fixture fails, never skips to PASS.
- One canonical migration ledger in `workflow/store/migrations/` (planned); no mirror migration directory, online fallback, SQLite or interchangeable store interface.
- No claim, admission, incarnation activation, effect intent/outbox, executor, daemon, provider polling, public HTTP or tool cutover APIs. Those remain with their existing owners.
- Preserve /19's complete Tasks, dependencies, 13 Verify Class/Command pairs and independent Evidence. Foundation acceptance is never full /19 acceptance; /37's 23 rows and joined persisted row remain owed.
- Stop at `implemented`; independent verification owns Evidence, verified/done.

## Task

1. Create the importable `github.com/medici-finance/assay/workflow` Go module with a maintained, pinned PostgreSQL driver and explicit caller-supplied connection configuration. No environment/credential discovery in the production library. Document supported PostgreSQL and durability assumptions, connection ownership/Close and bounded cancellation. Missing/unreachable input returns an error; it does not select another backend.
2. Reuse the catalogue-selected pinned Goose provider for one append-only ordered migration manifest and ledger. Use its PostgreSQL session locker on an owned connection to serialize concurrent migrators; each migration and its checksum/version record commit in one transaction. A narrow project-policy Goose Store adapter adds checksum validation in that same ledger, not a second runner or ledger. Reject duplicate/out-of-order manifests, changed recorded checksums, unknown mandatory/future versions and incomplete ledgers. A failed migration leaves neither partial DDL nor an acknowledged ledger row. No down migrations, reset verb or silent repair.
3. Write `spec/coordination-store-v1.md` (planned) with separately labelled **implemented foundation schema** and **planned /19 domain schema**. The foundation section precisely names its actual ledger columns; future attempts/leases/claims, activation, outbox, bindings, feedback and retirement remain unimplemented. Provide a bidirectional foundation schema-contract test against real PostgreSQL. Do not make /19's full schema-match test or any other parent row pass by excluding planned domain obligations.
4. Add an explicitly configured disposable PostgreSQL harness: create unique fixture databases with synthetic content, reset and teardown only those it created, and bound startup/connection/cleanup time. CI supplies a disposable service with no secrets. Local server execution uses an explicitly selected disposable Docker container (or separately authorized fixture profile), never a locally installed PostgreSQL server. Tests take an explicit disposable container DSN; setup documents the selected local Docker context/endpoint before contact and never probes an ambient daemon or cluster. It never downloads/installs dependencies or starts/probes Docker by default. Scrub inherited PostgreSQL connection settings so they cannot redirect it. Verify both a healthy run and missing/unreachable fixture refusal.
5. Exercise the real library → driver → PostgreSQL → ledger flow with independent clients, restarts, migration failure, compatibility rejection and cancellation. Use at least two independent processes for concurrent migration, with deterministic barriers, not only goroutines sharing one connection. Plant independent production-path mutations for locking, checksum/version checks and transaction boundaries; each named row must fail while its baseline passes.
6. Register this foundation's single semantic owner in `docs/contracts.md`, including exact implemented scope and later consumers. Add `.github/workflows/assay-workflow.yml` (planned) with real disposable PostgreSQL qualification and paths covering `workflow/**`, its contract, engineering inputs read by tests and the workflow itself. Inspect the actual cross-module registry/workflow roster; register every cross-module read or its honest opt-out, never silence a check. Keep existing module jobs unchanged. Document the new module's build/test commands and add the changelog fragment.

## Interface contract

Callers supply explicit PostgreSQL configuration and context; they receive a compatible, closed-on-failure database handle and typed migration/compatibility failures without secrets or connection strings. The package owns migration ordering and the single ledger. It exposes no workflow authority or arbitrary provider operation. Later mirror/domain adapters extend this same manifest and test harness and enforce their own admission, version and completeness predicates independently.

## Verify (executable — no prose-only DoD items)
| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationMigrations$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationMigrations " "$o")` | exit 0; named PASS; Real PostgreSQL fresh creation and reopen retain the same ordered ledger; a failing migration rolls back both its DDL and ledger entry, and a healthy subsequent migration succeeds. Independently removing transaction rollback or recording success before DDL completes makes the named test fail. |
| 2 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationConcurrent$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationConcurrent " "$o")` | exit 0; named PASS; Two independent processes with independent clients race migration through a deterministic barrier; each migration commits once, no partial schema is observable and both return the same compatible version. Removing the upstream PostgreSQL migration lock fails; no shared in-process mutex substitutes. |
| 3 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationCompatibility$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationCompatibility " "$o")` | exit 0; named PASS; Valid reopen succeeds; changed checksum, missing ledger entry, duplicate/out-of-order manifest, unknown mandatory/future version and partial ledger refuse. Independently bypassing checksum or schema-version refusal fails. |
| 4 | check +dereference +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationSchema$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationSchema " "$o")` | exit 0; named PASS; Resolve implemented foundation contract tables/columns and actual PostgreSQL catalog in both directions. A missing contract column, unexpected migrated table and removed SQL column each fail. Planned /19 domain tables stay explicitly unimplemented and cannot qualify the parent full-schema row. |
| 5 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationNoFallback$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationNoFallback " "$o")` | exit 0; named PASS; Explicit healthy fixture succeeds; unset configuration, unreachable database and cancellation refuse without any alternate backend, environment DSN or external process discovery; errors redact credentials and DSNs. A planted ambient-DSN or embedded fallback makes the named test fail. |
| 6 | check +flow +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationFixture$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationFixture " "$o")` | exit 0; named PASS; Unique fixture creation/reset/teardown affects only owned databases, even after failed setup or cancellation; hostile ambient PostgreSQL variables cannot redirect contact. Missing explicit fixture input fails rather than skipping. Removing the ownership predicate or consulting ambient connection configuration fails. |
| 7 | check:ci +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationBoundary$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationBoundary " "$o")` | exit 0; named PASS; Module imports no embedded/alternate engine and exposes no execution, claim, admission or outbox domain API. Adding an embedded runtime import fails. CI trigger inputs cover every out-of-module fixture/contract actually read; the check does not treat future parent domain semantics as implemented. |
| 8 | check | `(cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | exit 0 against explicit disposable PostgreSQL; no skipped integration test is credited. |
| 9 | check +dereference | `statusgen --root . --consumers --brief graph-execution/36 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | Before merge, actual implementation edits corroborate fixed-here consumers; follow-up rows prove routing only. Before post-merge verification use independently established immutable --delivery and --base bounds. Routing alone never proves producer identity or full parent delivery. |

The real PostgreSQL rows are explicitly environment-bound (`check`), so the independent verifier uses the selected disposable-container profile rather than an isolated namespace that cannot reach its fixture. The dedicated CI job still runs them with its own disposable service. No original /19 Class/Command is changed.

## Pre-mortem and detection

- Migration acknowledged before durable schema: row 1 atomicity mutation.
- Independent processes initialise incompatible ledgers: row 2 real-process race.
- Drift or a newer schema accepted as current: rows 3–4 checksum/version/catalog controls.
- A developer machine or substitute backend silently supplies the fixture: rows 5–6 ambient-input and absent-input controls.
- Foundation work is mistaken for all R2 execution semantics: row 7 boundary plus independent review of unchanged parent rows; full /19 and /37 remain held to their complete acceptance.

## Evidence
<!-- A NON-implementer supplies independent Verify evidence; no prospective row is claimed passed here. -->

## Review
Gate: human. Acceptance follows concrete fixture/CI/credential-boundary evidence; source and disposable fixture work may proceed before that acceptance. No operational database, provider credentials, deployed service or irreversible production migration is in scope.
