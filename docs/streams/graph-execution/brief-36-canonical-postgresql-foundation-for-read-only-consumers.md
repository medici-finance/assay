---
brief: assay:assay:graph-execution:36
title: Canonical PostgreSQL foundation for read-only consumers
why: "Read-only mirror persistence needs the canonical PostgreSQL migration and test substrate, but not workflow execution. Deliver that substrate first so shared reads do not wait for admission, outbox or executor integration."
wave: 0
depends: []
unblocks: ["graph-execution/19", "forge-neutral/52"]
split-from: graph-execution/19
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "The new CI job executes repository source with runner and checkout-token access, and the explicit database configuration handles credentials. Human acceptance reviews this boundary after disposable-fixture evidence; no operational service or database is authorized."
decision-trigger: spec
decision-issue: 5405
design: DR-graph-exec-36
ruling: "https://github.com/medici-finance/assay/issues/5405#issuecomment-6621165749"
issues: []
schema: brief-v2
version: 3
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
- "workflow/store/: fixed-here"
- "workflow/internal/pgtest/: fixed-here"
- "workflow/go.mod: fixed-here"
- "workflow/go.sum: fixed-here"
- "workflow/README.md: fixed-here"
- "workflow/testdata/compose.yaml: fixed-here"
- "spec/coordination-store-v1.md: fixed-here"
- "docs/contracts.md: fixed-here"
- ".github/workflows/assay-workflow.yml: fixed-here"
- "tools/desk/internal/deskkit/citrigger_test.go: fixed-here"
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

## Human decision

The isolated PostgreSQL foundation is ready for independent review of its bounded database-credential and CI execution boundary. It provides explicit connections, compatible-schema validation and a single forward-only migration/checksum ledger, qualified using synthetic data in a disposable container. Eight named tests, a race run and eight production fault controls supply implementation evidence; they are not independent acceptance. The CI job has read-only repository permission, a pinned synthetic PostgreSQL container and checksum-verified toolchain; checkout credentials are not persisted. No production database, provider identity or executor is configured.

1. Accept this bounded foundation after independent review and verification of the demonstrated credential, migration and CI boundary; retain all complete domain-store, mirror-retention, deployment and activation gates.
2. Hold acceptance and name the specific unsupported boundary or correction.

Default if no answer: none — blocks until answered

**Recorded disposition:** [The human ruling on #5405](https://github.com/medici-finance/assay/issues/5405#issuecomment-6621165749) accepts the bounded PostgreSQL foundation in merged [PR #5418](https://github.com/medici-finance/assay/pull/5418), relying on independent security review and the correctness review of the credential, migration and CI boundary. It also required the PostgreSQL `test` job, including `-race`, and both review lanes to pass at that PR's merge candidate head. This records the ruling; it does not supply post-merge Verify evidence. The complete domain-store, mirror-retention, deployment and activation gates remain in place, and full /19 acceptance remains owed.

This decision governed foundation acceptance. Offline contract, source and disposable-container fixture work was permitted before that acceptance. No production connection, live migration, role grant, service enrollment, tool cutover or activation follows from this decision. The separate dependency installation blessing, passing tests, publication and merge do not supply this acceptance. The human records the selected option in their own account; a worker relay is not a ruling.

On 2026-11-18 the driver approved [DR-graph-exec-36](../decisions/DR-graph-exec-36.md) as the design record for the bounded foundation delivered in #5418. The ruling preserves independent verification and the full domain-store, mirror-retention, deployment and activation gates. [Design ratification on #5405](https://github.com/medici-finance/assay/issues/5405#issuecomment-6967687802)

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
| 10 | check +mutation | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/mutations.py > "$o" && cat "$o" && grep -F -- "PASS: all8 production-path fault controls killed; original bytes restored" "$o")` | Run in an owned clean linked worktree on an owned feature branch, not main, master or detached HEAD. Exit 0; all eight named production fault controls print KILLED, compile/setup-only failures are refused and exact original source bytes are restored. This row executes all eight production fault controls; a baseline-only run cannot supply mutation evidence. |
| 11 | check +flow | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationDirectivePolicy$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationDirectivePolicy " "$o")` | Exit 0; named PASS. No-space, repeated-dash and case-insensitive transaction-disabling, environment-substitution, Down, duplicate-Up and unknown annotations refuse before connection. A normalized allowed forward migration executes through Goose on the disposable PostgreSQL fixture. |

The real PostgreSQL rows are explicitly environment-bound (`check`), so the independent verifier uses the selected disposable-container profile rather than an isolated namespace that cannot reach its fixture. The dedicated CI job still runs them with its own disposable service. No original /19 Class/Command is changed.

## Pre-mortem and detection

- Migration acknowledged before durable schema: row 1 atomicity mutation.
- Independent processes initialise incompatible ledgers: row 2 real-process race.
- Drift or a newer schema accepted as current: rows 3–4 checksum/version/catalog controls.
- A developer machine or substitute backend silently supplies the fixture: rows 5–6 ambient-input and absent-input controls.
- Foundation work is mistaken for all R2 execution semantics: row 7 boundary plus independent review of unchanged parent rows; full /19 and /37 remain held to their complete acceptance.

## Evidence
<!-- A NON-implementer supplies independent Verify evidence; no prospective row is claimed passed here. -->

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationMigrations$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationMigrations " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationConcurrent$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationConcurrent " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationCompatibility$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationCompatibility " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationSchema$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationSchema " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationNoFallback$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationNoFallback " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 6 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationFixture$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationFixture " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 8 | `(cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | fail exit=1 | sha256:f4009cd33531 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b red rc=1 · head=034614a8c462 · delivering=ca29a8a49e7d output=sha256:37408d02d42ab39d9d4da410c66e9a8913d12f693fabfc42ebd5520844b7bdf2 log=row-000001.log |
| 9 | `statusgen --root . --consumers --brief graph-execution/36 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:448bc943b19b | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b red rc=2 · head=034614a8c462 · delivering=ca29a8a49e7d output=sha256:819ce5578a30206805dd32da046b429610fe6a7e4ac38da136e396bcb23fe378 log=row-000002.log |
| 10 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/mutations.py > "$o" && cat "$o" && grep -F -- "PASS: all8 production-path fault controls killed; original bytes restored" "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
| 11 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationDirectivePolicy$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationDirectivePolicy " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
Verification-Attestation: medici-finance/assay#7554 run=794479ac1ad584ee6c4da8fb6ad9c5ae421b86322a756e2a5d338ba76b2ada59 source=034614a8c462b094fa7ec9396c4d70f6e6595a80 brief=docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md model=claude-opus-5-5 tier=strong

Container lane, row 7 (`check:ci`). The darwin host cannot sandbox a row with `unshare --net`, so row 7 ran inside a closed container-netns admission bracket with installed statusgen v1.0.92: open-bracket exit 0 at 2026-11-24T16:32:53Z, the verify-runner v1.0.92 image named by digest, `--network none`, a read-only tree, a read-only module cache and `GOPROXY=off`; close-bracket exit 0 at 2026-11-24T16:39:12Z; `--bracket-digest` reports `verdict=match`. The row's own guard reads ASSAY_TEST_POSTGRES_DSN, which admission removes, so row 7 is could-not-run in the bracket as on the host. The witness file's bytes follow verbatim.

| # | Command | Result | Output | Date | Runner | Base |
|---|---------|--------|--------|------|--------|------|
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationBoundary$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationBoundary " "$o")` | could-not-run exit=- sandbox=container-netns — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (on-behalf-of human:ian) (forge-identity) | base=d152c18c9d3b unproven · head=034614a8c462 |
Tree-Digest: sha256:d4d2a33395262730e607e326d77810bb696ffea7f79ac1d5e955d42b5e5fec4d
Container-Image: sha256:d3d958719dea47fc611e096612c1478bcc07376b7f9783f92433ac8faeeee380
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7554 run=794479ac1ad584ee6c4da8fb6ad9c5ae421b86322a756e2a5d338ba76b2ada59 source=034614a8c462b094fa7ec9396c4d70f6e6595a80 brief=docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=794479ac1ad584ee6c4da8fb6ad9c5ae421b86322a756e2a5d338ba76b2ada59 source=034614a8c462b094fa7ec9396c4d70f6e6595a80 tree=sha256:d4d2a33395262730e607e326d77810bb696ffea7f79ac1d5e955d42b5e5fec4d witness=sha256:f8fd889cacb0f3f818ecfacbf0fa91710012e006b2d131d9e774ee5e116f710f opened=2026-11-24T16:32:53Z closed=2026-11-24T16:39:12Z before=receipt after=check-verifier

Supplemental probes, no acceptance credit. They ran on the host at source 034614a8c462, outside admission, between 2026-11-24T16:32:21Z and 16:36:23Z. Each row command was first read for what it consumes: these rows read only ASSAY_TEST_POSTGRES_DSN, and no row reads forge, roster or home configuration. The variable named an explicit disposable PostgreSQL fixture: the compose profile the delivery ships, postgres 18.6-bookworm pinned by digest, published on 127.0.0.1 only, with synthetic fixture-only credentials. It was torn down after the run. There was no cluster or production contact.

| Probe | Verify row | What ran | Observed | Date | Runner |
|---|---|---|---|---|---|
| S1 | 1 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationMigrations (0.42s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S2 | 2 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationConcurrent (1.21s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S3 | 3 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationCompatibility (1.16s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S4 | 4 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationSchema (0.28s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S5 | 5 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationNoFallback (0.17s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S6 | 6 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationFixture (0.68s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S7 | 7 | row command verbatim, fixture DSN present, host lane (not network-off; the test itself opens no connection) | exit 0; --- PASS: TestPGFoundationBoundary (0.13s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S8 | 11 | row command verbatim, fixture DSN present | exit 0; --- PASS: TestPGFoundationDirectivePolicy (0.15s) | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S9 | 8 | row command verbatim, fixture DSN present; then the same go test with -v, keeping only SKIP and FAIL lines | exit 0; ok workflow/internal/pgtest 2.874s, ok workflow/pgtestfixture 2.081s, ok workflow/store 58.998s; the -v rerun prints 0 SKIP and 0 FAIL lines, so no skipped integration test is credited | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S10 | 8 | row command verbatim with ASSAY_TEST_POSTGRES_DSN unset, reproducing the admitted environment | exit 1, the admitted result; TestOwnedRefreshSeed and the TestPGFoundation tests fail at 0.00s on the missing input instead of skipping, as the brief requires | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S11 | 9 | row command verbatim on the host, re-run at 2026-11-24T19:01Z on a clean checkout of 034614a8c462 and of main 4c4ac6e5d650. After the merge, the merge-base of origin/main and HEAD is HEAD itself, so the base bound equals HEAD | exit 2 at both commits; COULD-NOT-CHECK: assay:assay:graph-execution:36 is not in the diff against HEAD, so no entry is corroborated or disproved, as in the witness row. The value first recorded here (exit 0; 0 corroborated, 0 disproved, 14 unchecked) came from the same command run inside the verifier home while this brief held uncommitted Evidence edits. statusgen reads the committed diff plus the working tree, so that edit put the brief in the diff and every entry printed UNCHECKED as unchanged since the merge-base. Neither run is evidence about the claims | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S12 | 9 | statusgen --root . --consumers --brief graph-execution/36 --delivery ca29a8a49e7d26dfbd3b892708273461a1de17c1 --base d152c18c9d3bc22eee414e889b5467290f7d0045 (the #5418 merge and its first parent, the bounds the row's Expect calls for) | exit 0; 11 corroborated, 0 disproved, 3 unchecked (the out-of-scope entries); PRODUCER-DELIVERY: COULD-NOT-CHECK. One out-of-scope entry, that no SQL mirror caller exists, is stale at HEAD: mirror persistence landed later with #5423 | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |
| S13 | 10 | row command verbatim, fixture DSN present, in an owned clean linked worktree on the owned branch verify/g36-r10-probe at 034614a8c462 (a scratch clone outside the home) | exit 0; KILLED: premature-ack, nontransactional, locking, checksum, version, schema, ambient, fixture-owner; PASS: all8 production-path fault controls killed; original bytes restored; worktree status empty afterwards | 2026-11-24 | assay-verifier-app[bot] @ 034614a8c462 (claude-opus-5-5) (on-behalf-of human:ian) |

Row accounting, 11 required rows: no row passes under admission, and no obligation on the delivered behaviour was observed failing.

- Rows 1 to 6, 8 and 11 are could-not-run, environment plus check-definition. Each reads ASSAY_TEST_POSTGRES_DSN: rows 1 to 6 and 11 through their guard, row 8 through its tests. The admitted runner removes that variable from every row's environment, and no admitted fixture lane exists yet. The planned lane will not unblock these rows as written. It admits only declared names matching DEVENV_[A-Z0-9_]+ through a service: Env cell, admission always removes the ASSAY_* family, and this Verify table has no Env column. Both prerequisites below are needed.
- Row 8's `fail exit=1` measures the same removal, not the deliverable. S9 gives exit 0 with the fixture and S10 gives exit 1 without it.
- Row 7 is could-not-run (check-definition). Its guard reads ASSAY_TEST_POSTGRES_DSN, but TestPGFoundationBoundary (workflow/store/foundation_test.go:454) opens no connection: it lists dependencies and parses source. No environment lane unblocks it; removing the guard would. S7 passes.
- Row 9's `fail exit=2` is a check-definition defect, the merge-base class of medici-finance/assay#6099. After the merge, the command's merge-base bound collapses to HEAD, and the row's own Expect calls for independently established delivery and base bounds that the command does not take. With those bounds, S12 corroborates 11 entries and disproves none.
- Row 10 is check-definition as well as environment. It must run on an owned feature branch, an admitted home is detached, and it shares the DSN guard. S13 kills all eight controls.
- Prerequisite (a), environment: desk-containers/35 to 37 implemented, all todo today, with a PostgreSQL dev-env instance declared for the cell. That is the cell dev-env file ruled on medici-finance/assay#6615.
- Prerequisite (b), check-definition: an amendment of this brief's Verify table, tracked at medici-finance/assay#7602. It covers DEVENV names and a service: Env cell for rows 1 to 6, 8, 10 and 11, removing row 7's DSN guard, delivery and base bounds for row 9, and a form of row 10 an admitted home can run.
- The verdict below cites medici-finance/assay#7551. That is a spec-authoring enhancement, not the item that unblocks these rows.
- Fail-first: rows 8 and 9 are red at base d152c18c9d3b. The other rows are unproven at base, because they could not run there.

The S1 to S13 runs are diagnostic only and credit nothing.

RISK-VALUE: DERIVED — migration ledger checksum = lowercase SHA-256 hex, 64 characters @ workflow/store/migrate.go:76, enforced by CHECK (checksum ~ '^[0-9a-f]{64}$') at workflow/store/migrations/ledger/create.sql:3. SHA-256 yields 32 bytes, and hex.EncodeToString emits two lowercase hex digits per byte, which makes exactly 64 characters matching the CHECK. spec/coordination-store-v1.md:23-25 fixes this ledger form. Every applied migration persists the value, so it cannot be changed once written. S3 (a changed checksum is refused) and S13 (checksum control killed) exercise it.
RISK-VALUE: DERIVED — TLS floor: a configuration with InsecureSkipVerify or a MinVersion below tls.VersionTLS12 is refused @ workflow/store/config.go:79. RFC 8996 deprecates TLS 1.0 and 1.1, so 1.2 is the lowest version a client may still offer. Certificate verification must stay on, or the DSN credentials travel to an unauthenticated peer.
RISK-VALUE: DERIVED — ledger version_id >= 0 @ workflow/store/migrations/ledger/create.sql:2, and contiguous five-digit manifest numbering (len(prefix) != 5 and n != i+1 refuse) @ workflow/store/migrate.go:41. Version 0 is the bootstrap entry (spec/coordination-store-v1.md:23-25), so the floor is 0. Requiring n = i+1 from 1 makes any gap, duplicate or reorder refuse. A fixed width of five digits makes lexical file order equal numeric order up to 99999. S3 and the S13 version control exercise it.
RISK-VALUE: NAMED, NOT DERIVED — operational bounds, all reversible configuration and ranked below the ledger and TLS values: pool limits of 4 open and 2 idle connections (workflow/store/config.go:136-137); a maximum protocol message body of 16 MiB (config.go:126); a configured timeout of at most one minute (config.go:63); session-lock retry and unlock timeouts (workflow/store/migrate.go:91); fixture connect and setup timeouts of 5s, 10s and 30s (workflow/internal/pgtest/fixture.go:89, 121, 193, 298); the CI job's 15-minute and 180s timeouts; and the pinned pgx v5.12.0, goose v3.29.0 and PostgreSQL 18.6 image.

**VERIFY: BLOCKED** — 0/11 rows pass under admission, and no obligation on the delivered behaviour was observed failing. Rows 1 to 8, 10 and 11 are could-not-run (environment): admission removes ASSAY_TEST_POSTGRES_DSN and offers no fixture channel (#7551). With an explicit disposable fixture, every one of them exits 0 (S1 to S10 and S13, uncredited). Row 9 is check-definition: after the merge its merge-base bound collapses to HEAD, and with delivery bounds it corroborates 11 entries and disproves none (S12, uncredited). Row 10 also needs an owned branch, which a detached admitted home cannot be. This brief is risk sensitive-data, so this run supplies Evidence only, and no flip follows from it.

## Review
Gate: human. The bounded foundation acceptance is recorded by the linked ruling on #5405; independent post-merge verification still owns the Verify table and Evidence. No operational database, provider credentials, deployed service or irreversible production migration is in scope. Complete domain-store, mirror-retention, deployment and activation gates remain unchanged.
