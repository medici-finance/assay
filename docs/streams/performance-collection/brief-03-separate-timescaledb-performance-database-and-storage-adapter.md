---
brief: assay:assay:performance-collection:03
title: "Separate TimescaleDB performance database and storage adapter"
why: "Optional history needs independent retention, access and failure behavior. A dedicated time-series database preserves useful analysis without turning cell coordination into a telemetry dependency."
wave: 1
depends: ["performance-collection/01"]
unblocks: ["performance-collection/04"]
effort: L
gate: human
risk: {regulatory: no, customer: no, irreversible: no, sensitive-data: yes}
gate-why: "Correlated operational measurements and their access/retention boundary require review of the implemented field policy and isolation; tests do not authorize live collection."
decision-trigger: spec
issues: []
schema: brief-v2
version: 1
id: cc4d6699-f1c8-4027-b1af-0fdee43fcf54
authored: 2026-10-13 by performance collection authoring
sources:
  - docs/streams/performance-collection/README.md
  - docs/telemetry.md
  - docs/proposals/non-ci-architecture/11-target-architecture.md
  - docs/proposals/non-ci-architecture/14-storage-forge-graph.md
  - docs/streams/continuing-operations/spec.md
  - freshness-checked 2026-10-13 @ 0cb90058470f9608f76b96e2055e7327d9c5c178
exec-tier: strong
exec-tier-why: Cross-component privacy, identity, concurrency and loss semantics require design judgment and negative controls.
domain: complicated
outcome: none
consumers:
  - "perfdata/postgres/: follow-up performance-collection/03"
  - "perfdata/postgres/migrations/: follow-up performance-collection/03"
  - "perfdata/postgres/testdata/: follow-up performance-collection/03"
  - "perfdata/go.mod: follow-up performance-collection/03"
  - "perfdata/go.sum: follow-up performance-collection/03"
  - "perfdata/deploy/compose.yaml: follow-up performance-collection/03"
  - "perfdata/deploy/cnpg/: follow-up performance-collection/03"
  - "perfdata/deploy/versions.json: follow-up performance-collection/03"
  - "perfdata/scripts/: follow-up performance-collection/03"
  - "docs/performance-storage.md: follow-up performance-collection/03"
  - "docs/performance-collection.md: follow-up performance-collection/03"
  - "tools/desk/internal/deskkit/audit.go: out-of-scope (required audit and budget authority unchanged)"
---

# Brief 03 — Separate TimescaleDB performance database and storage adapter

## Context

files: `perfdata/postgres/` (planned), `perfdata/postgres/migrations/` (planned), `perfdata/postgres/testdata/` (planned), `perfdata/go.mod` (planned), `perfdata/go.sum` (planned), `perfdata/deploy/compose.yaml` (planned), `perfdata/deploy/cnpg/` (planned), `perfdata/deploy/versions.json` (planned), `perfdata/scripts/` (planned), `docs/performance-storage.md` (planned), `docs/performance-collection.md` (planned), `changelog/performance-collection-03.md` (planned)

facts (source pin in frontmatter; planned paths/tests are deliverables, not implemented claims):
- The target storage architecture currently places cell coordination in PostgreSQL; the authoring request explicitly separates optional performance history into its own database.
- CloudNativePG chart sources include a TimescaleDB profile, but a profile example is not proof of compatibility for a selected PostgreSQL/extension/operator tuple. Pin and test the chosen tuple; no live cluster access is authorized.
- Timescale hypertable unique keys must include partitioning columns. Global event deduplication cannot be assumed from an event-id-only hypertable constraint; design the receipt/index seam explicitly.
- perfdata/01 supplies the event/batch contract and local fixture corpus. No cell runtime, forge credential or real perf corpus is a database prerequisite.

layering: perfdata storage adapter; performance database lifecycle is operator-owned. A pure typed contract owns validation/aggregation; filesystem, HTTP and database adapters own effects. Task boundaries and the flow/negative Verify cases below enforce this separation. No database or network dependency enters the local writer.

single-point-of-failure: the collection consent/identity boundary; independent typed payload validation and source-scoped storage checks constrain disclosure, while missing optional data cannot grant authority or reset required accounting. Verify bypasses upper adapters and exercises lower boundaries.

design-fit:
  owner: perfdata storage adapter; performance database lifecycle is operator-owned
  contract: none — register the bounded collection owner under docs/contracts.md in 01; retain existing domain/measurement owners
  retires: []
  weight: verbs +0; flags +0 (operator configuration file); refusals +4 schema/version/role/replay classes; rule-text +0
  why-add: Bounded optional collection has no complete-path owner today. Reusing the load-bearing audit/journal as a prunable transport was considered and rejected; use existing producer seams and one shared contract instead. These are scope estimates; measure the implemented delta.

## Read first

Read [the stream design](README.md), [telemetry promise](../../telemetry.md), and
[library-first boundaries](../../library-first.md). All load-bearing scope and acceptance
requirements for this brief are restated below; sibling briefs describe providers, not
additional hidden acceptance criteria. Refresh active claims and source paths before edits.

## Human decision

Confirm the tested PostgreSQL, TimescaleDB and operator version combination, feature terms and independent database privileges. Production retention, capacity and recovery targets require an explicit deployment profile; fixture defaults do not select them.

Options:
1. **Accept the bounded implementation** — accept only the demonstrated field, failure and isolation contract; keep live activation separately controlled.
2. **Hold for correction** — name the unsupported claim or boundary and revise the implementation before acceptance.

Default if no answer: none — acceptance remains pending. The decision is presented after implementation evidence is concrete; it does not block authoring or offline implementation.

## Ground rules

- Isolate changes; no push to main, merge, workflow dispatch or live infrastructure contact.
- Synthetic fixtures and explicitly disposable local database only; no production credentials/data.
- Preserve operational audit, budget, idempotency and authority when optional collection is off.
- Stop at implemented; independent verification and human acceptance remain separate.
- Unsupported/missing observations are explicit; never convert them into zero or success.

## Task

1. Pin one supported PostgreSQL/TimescaleDB image and digest, CPU architectures, CNPG operator/profile version and extension feature/license inventory. Provide a local Docker profile and CNPG manifests from the same database schema. Validate the pinned combination and extension preload/create requirements; do not silently remove Timescale features or select floating tags when qualification fails.

2. Create a separate performance database with ingestion, reporting and migration/maintenance roles. No role used by the collector can CONNECT to or read/write the cell database; revoke broad PUBLIC privileges where necessary in the disposable profile. Even on a shared instance, configure independent connection/work budgets and document shared CPU/disk failure risks. A stronger physical-isolation profile is optional, not implied by separate schemas.

3. Implement forward-only migrations, event storage and a pure-consumer storage interface. Use time-partitioned Timescale storage for event observations; ordinary relational tables for bounded source/profile metadata and dedup receipts as needed. Use authenticated source plus immutable event identity/digest, with partition-key constraints addressed. Atomically commit event and dedup receipt. Concurrent identical replay gives one event; changed payload under the same identity conflicts.

4. Enforce explicit configured raw, aggregate and receipt/replay horizons. Reject too-old events outside the accepted horizon; support late arrivals inside it and typed corrections. Coordinate pruning and dedup so old replay cannot resurrect expired rows or inflate aggregates. Preserve counts/sums or mergeable distributions; test raw expiry against aggregate-refresh windows. No retention policy may touch cell or audit data.

5. Provide bounded read/export functions returning source/profile/policy versions, time window and coverage alongside data. No arbitrary SQL endpoint, cross-database foreign key, FDW or query back into operational cells. Unknown history and unavailable values remain explicit; a received-only denominator cannot claim complete coverage.

6. Prove migration, crash/transaction rollback and backup/restore on a real disposable local database. Provide a pinned CNPG restore/failover qualification procedure for the separately authorized operator; report not-run rather than satisfying it through YAML validation. Specify RPO/RTO/retention as activation inputs, not unmeasured promises. Neither this brief nor a passing disposable test activates a cluster.

## Interface contract

Inputs and outputs use performance-event-v1 and performance-batch-v1 from 01, with
source/profile/policy revision, immutable identity, provenance and coverage. No extension
may reinterpret old fields silently. The local producer owns event identity; authenticated
collector policy owns admission scope; the store owns committed receipts. Performance
observations confer no work, acceptance or effect authority. Unsupported mandatory schema
or unavailable measurements must remain visible through every downstream reader.

## Verify

All named tests below are **implementation deliverables**, not tests already executed.
Each command requires the named PASS line, so a missing test cannot pass through Go's
"no tests to run" behavior. Each test also includes the planted failing control stated
below, exercising production code rather than an independently reimplemented checker.

Database rows are classed `check` (env-bound, [brief rule 45](../../brief-rules.md)), not
`check:ci`: they require ASSAY_TEST_PERF_DSN pointing to an explicitly disposable local
Timescale database provisioned from the pinned profile, which CI does not provision, so the
verifier runs them against that database. Missing DSN fails qualification; tests must never
skip to green or fall back to a live/default database. Before any write or destructive step
(crash, restore, role or privilege change, database removal) the harness mechanically refuses
a non-disposable target: it must read back a sentinel the pinned disposable profile writes at
provisioning, and the DSN host must be loopback or the local container host. A mis-set DSN
refuses instead of acting on a real instance. Isolation rows also provision a separate
synthetic cell database and actual restricted roles on that instance.

| # | Class | Command | Expect |
|---|---|---|---|
| 1 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStoreDedupe$' ./postgres/... > "$o" && grep -F -- '--- PASS: TestStoreDedupe ' "$o")` | exit 0; named PASS. Real database: concurrent duplicates, conflict, crash between event/receipt writes and retry leave one committed logical event and stable result. |
| 2 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestDatabaseSeparation$' ./postgres/... > "$o" && grep -F -- '--- PASS: TestDatabaseSeparation ' "$o")` | exit 0; named PASS. Actual SQL roles cannot connect to/read cell data; collector outage or performance DB removal leaves disposable cell operations working. |
| 3 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestRetentionReplay$' ./postgres/... > "$o" && grep -F -- '--- PASS: TestRetentionReplay ' "$o")` | exit 0; named PASS. Advance fixture time through raw and dedup expiry, replay too-old and late events, and verify rollups do not double-count or vanish unexpectedly. |
| 4 | check +flow +mutation | `(cd perfdata && test -n "$ASSAY_TEST_PERF_DSN" && o=$(mktemp) && trap 'rm -f "$o"' 0 && GOWORK=off go test -count=1 -v -run '^TestStoreRestore$' ./postgres/... > "$o" && grep -F -- '--- PASS: TestStoreRestore ' "$o")` | exit 0; named PASS. Real backup restore of source metadata, events and receipts preserves known totals/coverage and exposes the declared recovery gap. |
| 5 | check:ci +dereference | `statusgen --root . --consumers --brief performance-collection/03` | exit 0; routing claims corroborated against the implementation diff, including each changed consumer. |

## Pre-mortem and detection

Remove source scoping or transactionality, grant the ingest role cell access, or prune dedup before replay expiry; the corresponding real-database tests must fail. Missing coverage is not a successful zero-data test. The end-to-end
row must traverse actual public producer/reader or served interfaces. Scope/retention
adequacy and deployment workload selection are review judgments; their measured evidence
is required before activation, not invented from component-test success.

## Evidence

<!-- Independent verifier records subject revision, environment, command, exit, named result and date. -->

## Review

Gate: human. Review actual disclosure/identity boundaries, loss visibility and shared-value
flow evidence. Required operational records must still function with optional collection
disabled. Human acceptance changes no live destination, credential, collection switch or
retention policy.
