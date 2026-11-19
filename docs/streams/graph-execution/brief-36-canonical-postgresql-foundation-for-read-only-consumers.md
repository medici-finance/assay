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

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 1 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationMigrations$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationMigrations " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 2 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationConcurrent$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationConcurrent " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 3 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationCompatibility$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationCompatibility " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 4 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationSchema$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationSchema " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 5 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationNoFallback$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationNoFallback " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 6 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationFixture$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationFixture " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
| 8 | `(cd workflow && GOWORK=off go vet ./... && GOWORK=off go test -count=1 -race ./...)` | fail exit=1 | sha256:fb176c14db7d | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 9 | `statusgen --root . --consumers --brief graph-execution/36 --base "$(git merge-base refs/remotes/origin/main HEAD)"` | fail exit=2 | sha256:7246e71fa47f | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (reads could-not-check: recorder is Linux-only (darwin)) (on-behalf-of human:ian) (forge-identity) |
| 11 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationDirectivePolicy$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationDirectivePolicy " "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7137 run=1136069afb1b8de7f6388e4c4e30908fae59dbda577a56e0be1f19708e2197ec source=37483b76bfad14f33b00940feb4115b6ea1340ec brief=docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md model=claude-opus-5-5 tier=strong

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 10 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && python3 store/testdata/mutations.py > "$o" && cat "$o" && grep -F -- "PASS: all8 production-path fault controls killed; original bytes restored" "$o")` | could-not-run exit=- — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
Verification-Attestation: medici-finance/assay#7137 run=1136069afb1b8de7f6388e4c4e30908fae59dbda577a56e0be1f19708e2197ec source=37483b76bfad14f33b00940feb4115b6ea1340ec brief=docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md model=claude-opus-5-5 tier=strong

Row 7 is a check:ci row, and this darwin host cannot sandbox it, so it ran in the closed admission bracket (brief-rules rule 45). Every step started from the home root, with no edit to the home between steps. Host `statusgen verifyrun --open-bracket` used a new module cache (digest sha256:7f8fa9800956) and recorded tree sha256:408f97708f1f. The container was the release-pinned ghcr.io/medici-finance/verify-runner:v1.0.88 at sha256:dd758003b77f, carrying statusgen v1.0.88. It ran with `--network none --read-only --cap-drop ALL`, no-new-privileges, uid 501, a tmpfs /tmp, the module cache mounted read-only with GOPROXY=off, GOTOOLCHAIN=local and GOFLAGS=-mod=readonly, and the home mounted read-only at its own path. The container step was `statusgen verifyrun --sandbox=container-netns --rows 7`, and host `--close-bracket` closed it. `statusgen verifyrun --bracket-digest` on the closed witness reports verdict=match (witness sha256:5d5387e26e81, span-lines=tree-digest:1,container-image:1,attestation:1). The closed witness, verbatim:

| # | Command | Result | Output | Date | Runner |
|---|---------|--------|--------|------|--------|
| 7 | `(cd workflow && test -n "$ASSAY_TEST_POSTGRES_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run "^TestPGFoundationBoundary$" ./store/... > "$o" && grep -F -- "--- PASS: TestPGFoundationBoundary " "$o")` | could-not-run exit=- sandbox=container-netns — the row reads ASSAY_TEST_POSTGRES_DSN, which an admitted run removes from every row's environment, so it cannot run as written here. This note is not permission to run it outside admission, where the row sees the full environment admission removes: do that only once the row itself has been reviewed for what it reads | sha256:e3b0c44298fc | 2026-11-19 | assay-verifier-app[bot] @ 37483b76bfad (on-behalf-of human:ian) (forge-identity) |
Tree-Digest: sha256:408f97708f1f60581e1dd907ca6bb6e38b1fd9878df6da8bc222c81d4792bcd2
Container-Image: sha256:dd758003b77fc718ec5b9b3e3a34529a422eb87316a605207bdad2560ca169bc
Module-Cache: sha256:7f8fa9800956b328a3851321def73d53e5ec63663bf2209818288608b6dc2ed6
Verification-Attestation: medici-finance/assay#7137 run=1136069afb1b8de7f6388e4c4e30908fae59dbda577a56e0be1f19708e2197ec source=37483b76bfad14f33b00940feb4115b6ea1340ec brief=docs/streams/graph-execution/brief-36-canonical-postgresql-foundation-for-read-only-consumers.md model=claude-opus-5-5 tier=strong
Admission-Bracket: run=1136069afb1b8de7f6388e4c4e30908fae59dbda577a56e0be1f19708e2197ec source=37483b76bfad14f33b00940feb4115b6ea1340ec tree=sha256:408f97708f1f60581e1dd907ca6bb6e38b1fd9878df6da8bc222c81d4792bcd2 witness=sha256:5d5387e26e8193b429c4df3d504890a592115c00b39be76633a2f2d82458e0e9 opened=2026-11-19T03:48:25Z closed=2026-11-19T03:48:47Z before=receipt after=check-verifier

Row accounting: 11 required rows, 0 pass, 2 fail, 9 could-not-run.

- Rows 1, 2, 3, 4, 5, 6, 10 and 11 (host) and row 7 (container): could-not-run. statusgen v1.0.88 removes ASSAY_TEST_POSTGRES_DSN from every row's environment in an admitted run. Each of these rows begins with `test -n "$ASSAY_TEST_POSTGRES_DSN"`, so it cannot run as written. The tool's note says this is not permission to run the row outside admission until the row has been reviewed for what it reads, so no row was run with the DSN outside admission. A disposable fixture was provisioned for the run: the brief's own compose profile, with an explicit local OrbStack context, a unique project, loopback-only publishing, synthetic credentials and the pinned postgres:18.6-bookworm digest. The admitted run stripped the variable before any row could reach the fixture. The project was then removed with `compose down --volumes`. Row 10 has a second precondition that the admitted home cannot meet: the committed mutations script exits unless the checkout is on a named feature branch, and an admitted verifier home is detached at its source.
- Row 8: fail exit=1, and the cause is environmental. The row has no `test -n` guard, so it executed after the same DSN removal. The brief's harness fails rather than skips without an explicit DSN. A supplemental host probe with no acceptance credit reran the row's exact command with ASSAY_TEST_POSTGRES_DSN unset, the same environment the admitted run produced. It exited 1. Every PostgreSQL-backed test failed with "explicit container DSN is required" (for example foundation_test.go:88 and fixture_test.go:25), and the internal pgtest package passed. This failure therefore shows the missing-fixture refusal the brief requires. It is not evidence that the implementation is defective, and it is also not a pass against disposable PostgreSQL.
- Row 9: fail exit=2. The instrument reported COULD-NOT-CHECK: graph-execution/36 is not in the diff against 37483b76bfad, because merge-base(origin/main, HEAD) equals the detached source on a post-merge home. This is not a disproved claim. The row's own Expect says that post-merge verification needs independently established immutable delivery and base bounds. A supplemental probe with no acceptance credit, `statusgen --root . --consumers --brief graph-execution/36 --delivery ca29a8a49e7d26dfbd3b892708273461a1de17c1 --base d152c18c9d3bc22eee414e889b5467290f7d0045` (the merge of PR 5418 and its first parent), exited 0 with 11 corroborated, 0 disproved and 3 unchecked (the out-of-scope judgements).

Why this is BLOCKED rather than FAIL: no acceptance obligation was observed to fail. Each non-pass row is either a could-not-run or a failure caused by the admission environment or by post-merge instrument bounds. The needed capability is a reviewed way to supply the explicit disposable-fixture DSN to env-bound `check` rows inside an admitted run, or a reviewed definition repair of those rows. Row 10 also needs a definition repair, or an admitted lane that runs it on an owned branch. A repair needs fresh dispatch and admission after it lands.

Risk-bearing values (trigger: risk sensitive-data yes; enumeration covered the delivery diff of PR 5418, merge ca29a8a49e7d against first parent d152c18c9d3b, for config.go, migrate.go, ledger.go, the migrations, pgtest fixture.go, go.mod and the workflow file, read at the verified source):

- RISK-VALUE: DERIVED — permissions.contents = read @ .github/workflows/assay-workflow.yml:31 — the job checks out and tests source and writes nothing to the repository. Read-only is the least scope that lets checkout work, which is the CI execution boundary named in gate-why.
- RISK-VALUE: DERIVED — persist-credentials = false @ .github/workflows/assay-workflow.yml:53 — the job executes repository test code. Persisting the checkout credential in the local git config would let that code reuse it, so false is the only value consistent with the stated credential boundary.
- RISK-VALUE: DERIVED — TLS floor MinVersion < tls.VersionTLS12 refused, with InsecureSkipVerify refused @ workflow/store/config.go:79 — the store carries database credentials. TLS 1.2 is the lowest version without known protocol-level breaks that current guidance (NIST SP 800-52r2) still permits.
- RISK-VALUE: DERIVED — ledger checksum CHECK (checksum ~ '^[0-9a-f]{64}$') @ workflow/store/migrations/ledger/create.sql:3 — digest() in migrate.go is hex.EncodeToString(sha256.Sum256(b)), which is always 64 lowercase hex characters. The constraint matches the algorithm exactly. Recorded ledger rows are durable and forward-only, so a mismatch would be costly to undo.
- RISK-VALUE: DERIVED — github.com/jackc/pgx/v5 v5.12.0 and github.com/pressly/goose/v3 v3.29.0 @ workflow/go.mod:8 and :10 — these equal the versions the brief's Dependency admission names, assessed on issue 5396.
- RISK-VALUE: NAMED, NOT DERIVED — postgres image = postgres:18.6-bookworm@sha256:5c4783a8cc3f0074912164f94dfa1be4e043438f0f707cf968d7af9a0d31a2c8 @ .github/workflows/assay-workflow.yml:40 (and workflow/testdata/compose.yaml:5) — the digest pin makes the bytes content-addressed and the local pull resolved it. That this digest is the official 18.6-bookworm publication was not checked against the registry's tag history in this offline pass.
- RISK-VALUE: NAMED, NOT DERIVED — GO_SHA256 = 7226736c2339719cda28fbf3a54d71aa63bcefe376e98cef00034268dab24840 @ .github/workflows/assay-workflow.yml:57 — not compared against the go.dev published checksum in this offline pass. The value has changed since delivery (it was 65efbc80fdc1 at the PR 5418 merge).
- Ranked last as reversible operational knobs, with no derivation needed: lock.WithLockTimeout(1, 10) and WithUnlockTimeout(1, 2) @ workflow/store/migrate.go:91; c.Timeout > time.Minute @ workflow/store/config.go:63; MaxProtocolMessageBodyLen = 16 << 20 @ config.go:126; SetMaxOpenConns(4) and SetMaxIdleConns(2) @ config.go:136-137; fixture connect_timeout bound seconds > 10 @ workflow/internal/pgtest/fixture.go:53; timeout-minutes: 15 @ assay-workflow.yml:37.

Attempts: (1) 2026-11-19T03:46:09Z to 03:46:13Z, row 9, host dry run; superseded by attempt 2. (2) 2026-11-19T03:46:50Z to 03:46:59Z, host rows 1,2,3,4,5,6,8,9,11. (3) 2026-11-19T03:47:28Z to 03:47:31Z, rows 7 and 10, host dry run; superseded by attempts 4 and 5. (4) 2026-11-19T03:48:09Z to 03:48:47Z, container bracket, row 7. (5) 2026-11-19T03:49:40Z to 03:49:42Z, host row 10. No attempt is credited.

**VERIFY: BLOCKED** — 0/11 rows pass. Rows 1–7, 10 and 11 could not run because the admitted run removes ASSAY_TEST_POSTGRES_DSN; row 10 also needs a feature branch the admitted detached home cannot have. The row 8 failure is the same environmental cause, and row 9 is the instrument's COULD-NOT-CHECK at post-merge bounds (the supplemental delivery-bounded probe corroborated 11 and disproved 0). No acceptance obligation was observed to fail.

## Review
Gate: human. The bounded foundation acceptance is recorded by the linked ruling on #5405; independent post-merge verification still owns the Verify table and Evidence. No operational database, provider credentials, deployed service or irreversible production migration is in scope. Complete domain-store, mirror-retention, deployment and activation gates remain unchanged.
