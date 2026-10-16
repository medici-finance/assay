---
stream: performance-collection
repo: medici-finance/assay
serves: assay
status: parked
priority: P2
track: platform
issues: []
board: generated
spec: docs/streams/performance-collection/README.md
---

# Optional performance collection

**Status:** draft — bounded architecture amendment and executable brief set, 13 October 2026.
Collection can start locally before any database, API or managed cell exists. This stream
owns delivery of optional collection/storage plumbing; it does not replace the semantic
owners of work, runner accounting, measurement definitions or continuing outcomes.
The stream is parked pending review and capacity admission. Authored is not implemented;
no collection, deployment or new permission is activated by this document.

## Direction and source boundary

The authoring request of 13 October 2026 selects local-first logging, a performance database
separate from cell information, and an API collector. It asks for the missing pieces too.
This design adds an independently deliverable uploader and a complete-path qualification.
TimescaleDB is the selected time-series candidate, subject to the database brief's pinned
Docker/CNPG compatibility, restore and licensing qualification. No live infrastructure is
needed to author or implement these briefs; deployment remains an operator act.

Source inspection: public main `0cb90058470f9608f76b96e2055e7327d9c5c178`, 13 October 2026.
[PR 2853](https://github.com/medici-finance/assay/pull/2853) was open/draft at head
`5648afffd5db3ad76f6a9365679c3e9bd74e98b0`; its implementation is not main or a release.
The first local contract/logger can be built against existing runner interfaces and synthetic
fixtures now. Import of actual `perf-v1` files becomes useful as desk-tools/23 ships; it is
an optional source, not a dependency that holds the local logger or database work hostage.
No second implementation of desk-tools/23's timing hooks is authorized here.

Read these controlling and proposed sources:

- [Telemetry promise](../../telemetry.md): existing `telemetry-v1` counts-only data,
  exact environment opt-in plus command-line opt-in for transmission, preview before send.
- [desk-tools/23](../desk-tools/brief-23-usage-and-timing-telemetry.md): separately scoped
  local `perf-v1` log and seven-day retention, currently implemented on PR 2853 only.
- [Future architecture](../../proposals/non-ci-architecture/11-target-architecture.md)
  and [storage/recovery](../../proposals/non-ci-architecture/14-storage-forge-graph.md).
- [Continuing operations measurement intent](../continuing-operations/spec.md#5-portable-measurements-before-methodology)
  and [O02/O04 outlines](../continuing-operations/proposed-briefs.md): metric meanings,
  units, sampling, attribution, quality and assessment remain there. These are proposals.
- [graph-execution/06](../graph-execution/brief-06-run-records-and-replay.md): canonical
  execution/run record; this stream copies bounded observations, not a second run authority.
- [graph-execution/26](../graph-execution/brief-26-shared-loop-admin.md): planned durable
  process journal in the cell store. It is not a prerequisite for a local optional sink.
- [Library-first boundaries](../../library-first.md): pure contracts and typed adapters;
  imports do not grant credentials or publication authority.

## What changes in the architecture

| Record class | Owner/home | Optional performance copy |
|---|---|---|
| Work, claims, leases, decisions, effect identities and required audit | Existing operational owners; future cell PostgreSQL database and canonical evidence/approval paths | Selected observations only; never drives admission or acceptance |
| Budget-enforcement usage | Runner/supervisor operational accounting | May be copied after consent; disabling performance collection never resets or disables budget accounting |
| Performance events and coverage | Local append-only segmented files first; dedicated performance PostgreSQL database later | This is the optional store, with independent lifecycle and retention |
| Metric definitions and outcome interpretations | Continuing-operations/quality owners and their versioned definitions | References and versioned derived results; no new AssayScore formula or acceptance predicate |
| Raw prompts, transcripts, credentials and canonical evidence payloads | Existing custody/evidence policies | Excluded from this collection |

A separate **database**, not merely another schema in the cell database, is required.
It may initially share a PostgreSQL server with a cell, with distinct roles, connection
budgets and resource limits; that is logical separation, not an independent failure domain.
No cross-database foreign keys, FDW/dblink bridge, trigger or synchronous dual write.
The collector has no cell-database credential. Cell operation does not depend on the
collector or performance database. Authorized analyses may join exported opaque references;
resolving them back to private work requires the original source's access control.

The new `perfdata/` module is a narrow collection owner: pure event/receipt validation,
local storage and aggregation, storage/transport adapters and thin commands. Its root
contract package and local writer cannot import PostgreSQL clients, HTTP, forge access or
credential custody. Database and server packages are separate imports in the same module.
No new general scheduler, workflow engine, mandatory sidecar or per-operation Git journal.
The new deployment process is justified by ingestion credential and database custody.

## Consent and information classes

Preserve `telemetry-v1` and `perf-v1` unchanged, including their switches and payload promises.
The collector does not silently activate statusgen's empty compiled endpoint.
New records use `performance-event-v1`, with explicit profile and collection-policy revision:

- `tool`: fixed-vocabulary measurements, build/platform profile and random source identity.
  Source identity makes records pseudonymous, not anonymous. No work/session references.
- `workflow`: an explicit additional collection profile for correlated work/attempt/evidence
  observations, usage, human-effort declarations and bounded imported outcomes. Use locally
  assigned random opaque aliases for qualified source identities; keep the private mapping
  locally under its own retention/access policy. Unkeyed hashes of repo names are not anonymity.
  Imported sources are read only within explicitly configured paths/scopes.

`ASSAY_TELEMETRY=1` arms local writing only when a new-schema collection profile is explicitly
selected; this does not turn an existing perf-v1 opt-in into consent to workflow collection.
No selected profile means no new-schema writes. Remote sending additionally requires the
explicit `perfdata send --upload` action and an operator-configured destination/auth profile.
Preview exactly the event payload before transmission; never print credentials. A dry run
opens no socket. Each batch rechecks consent. Removing either switch stops further sends;
a request already submitted may complete and its acknowledgment remains reconcilable.
Old local records are never uploaded automatically when upload is enabled: the operator
selects an explicit date/source window and payload profile. Collection-off retains existing
files until policy expiry or explicit deletion; it does not secretly erase them or upload them.

## Event and delivery contract to freeze in brief 01

Every event has schema, immutable event ID, source/epoch/sequence, profile and policy revision,
producer/build version, occurred-at and recorded-at UTC timestamps, kind, bounded payload,
measurement basis and coverage. Collector received-at is additional provenance, never a
replacement for source time. Monotonic elapsed durations are measured at the producer;
cross-host ordering uses causal IDs and reports clock uncertainty. No negative clamping into
an apparently valid interval. Missing and unavailable differ from measured zero.

Workflow events reuse aliased canonical work/candidate/contract/attempt/invocation identities,
parent/retry links and actor role. A session can serve several items: preserve unattributed
or shared overhead instead of fabricating a split. Source epoch changes after mapping loss
prevent false joins. Usage snapshots are cumulative or delta **explicitly**; repeated polls
must never add the same cumulative usage again. Preserve the actual model/profile from
`runner.Profile` and unknown (nil) token/cost values from `runner.Usage`; monetary estimates
carry source/currency/pricing revision.
Human decision latency, decision count and declared hands-on duration are distinct fields.
Measured or estimated duration records its method; no transcript mining to invent minutes.

Kinds cover tool completion, attempt lifecycle/usage, work-stage/hold transitions, review and
verification applicability, human effort, imported CI/outcome observations and collection
health. Only sources actually wired by brief 02 count as covered. Preserve canonical metric
references where available; elapsed stage dwell is not active service time. No source is
required to claim an event it cannot observe, and no scheduled polling job is added here.

Transport is a versioned batch of these same stored events, with bounded count/bytes,
authenticated source scope, and per-event acknowledged/conflicting/rejected results. An ACK
means committed to the configured performance database, not merely received in memory.
Retries retain event identity and payload. Identical duplicates succeed idempotently;
same identity with a different digest conflicts. Concurrent redelivery and lost ACKs are
normal cases. Delivery is at least once inside the retained replay window, not exactly once.
No task-launch, cell-administration or arbitrary-query route belongs to this API.

## Retention, loss and health

Initial local age cap is seven UTC days, matching the existing diagnostic window; add an
explicit bounded byte budget before activation. A full queue drops optional new data with
an attributable gap rather than blocking work or deleting required operational records.
Age expiry applies to unsent data too: record expired-unsent counts/windows where possible,
so no unbounded backlog is implied. Reader reports persisted losses and uncertain coverage;
a full disk or killed producer can lose the loss marker too, which must remain unknown.
Started without terminal observation means unfinished/unknown, not success or confirmed death.

Server retention and byte/ingestion budgets are operator-selected, published before upload,
and versioned. Qualification includes a 90-day analysis window to support existing baseline
use; that fixture is not a universal production retention default. Event deduplication,
accepted replay age and retention are coordinated: reject records outside the replay horizon
rather than resurrecting expired samples. Late arrivals inside the horizon refresh the affected
aggregates; corrected events retain the old record and a typed correction relationship.
Retain counts/sums and mergeable distributions where needed; never average daily percentiles.

Health reports include configured/enabled producer coverage, first/last observation, open
attempts, dropped/expired/unparseable counts, pending bytes/oldest pending event, last successful
ACK, ingest lag and source-history boundaries. A source with no records is unknown unless an
independent complete observation proves zero. Do not infer the denominator from received events.
Historical coverage survives normal restore; loss outside the recovery point is exposed.
Local cleanup, performance database deletion and cell recovery are independent operations.

## Bounded acceptance and remaining operator choices

Local-only acceptance requires a real producer/reader journey with no server/database and
zero network attempts, preserving the underlying operation on disk-full, queue-full and
invalid-record failures. Collection cost includes dropped observations and writer overhead.
Use the existing desk-tools/23 sub-millisecond median append target only for equivalent
small-record local append benchmarks; report p95/max, platform, payload size, concurrency and
collection-on/off end-to-end cost. A durable writer cannot promise zero blocking: use bounded
queueing/flush work, state crash loss, and do not synchronously fsync on the work path.
New end-to-end or fleet-scale budgets require an explicit workload/profile before activation.

Database qualification pins PostgreSQL, TimescaleDB, CNPG and image digest plus architectures;
checks available feature/license terms; proves schema migrations, role separation, restore,
retention and aggregate behavior on a disposable deployment. The served collector's authentication,
source scope, limits and duplicate handling must be exercised at its real HTTP boundary.
CNPG procedures are supplied for a separately authorized disposable cluster; sessions in this
repository never query a live cluster. A static manifest check is not a failover/restore pass.

Before remote activation, an operator must select destination, authenticated source mapping,
allowed profiles, raw/aggregate/dedup retention, byte/rate/query budgets, backup RPO/RTO and
resource separation. These values are not needed for local fixture implementation. Neither
this draft nor its human-gated implementation briefs grants a live endpoint or credentials.

## Existing work and consumer routing

| Existing owner | Disposition |
|---|---|
| desk-tools/23 / PR 2853 | Preserve active scope and all existing retention/privacy tests. Add a version-selected importer; no edits to its in-flight implementation in this authoring change. |
| statusgen/09, `statusgen/telemetry.go` | Counts-only client remains separate; no automatic endpoint activation or wider payload. |
| graph-execution/06 and /26 | Preserve run-record and operational accounting authority. Optional sinks copy observations; future runtime migration carries those sinks rather than creating another journal. |
| continuing-operations O02/O04 | Continue to own metric meanings and interpretation. The bounded transport schema is not `measurement-v1`; future adapters consume it with explicit basis/coverage and preserve canonical run references. |
| Architecture R1/R2/R3/R6/R7 | Brief 01 documents separate storage/consent contracts; 02 wires existing sources and names unavailable future sources; 03–06 qualify storage, serving, transport and migration. No R1–R8 runtime activation dependency for local logging. |
| qualgen, AssayScore, cockpit | Future consumers use a bounded offline export with version/coverage. Full dashboard/score changes and causal improvement claims are outside this delivery. |

Source-level gaps motivating this work are visible in `statusgen/flow.go` (historian dwell,
unimplemented check-run reader), `statusgen/assayscore.go` (decision-latency proxy), and
`loopadmin/runner/client.go` (in-memory state, future supervisor owns durable journal).
No fresh runtime capacity, churn or complexity baseline was collected. Proceed with bounded
adapters and preservation tests; do not use missing quality evidence to justify a tool rewrite.

## Briefs

<!-- statusgen:briefs:begin -->
| # | Brief | Wave | Effort | Status | Verified | Reviewed |
|---|-------|------|--------|--------|----------|----------|
| 01 | [Local performance event contract, spool and offline report](brief-01-local-performance-event-contract-spool-and-offline-report.md) | 0 | L | todo | — | — |
| 02 | [Producer adapters for workflow, usage and local performance](brief-02-producer-adapters-for-workflow-usage-and-local-performance.md) | 2 | L | todo | — | — |
| 03 | [Separate TimescaleDB performance database and storage adapter](brief-03-separate-timescaledb-performance-database-and-storage-adapter.md) | 1 | L | todo | — | — |
| 04 | [Authenticated performance ingestion API](brief-04-authenticated-performance-ingestion-api.md) | 2 | L | todo | — | — |
| 05 | [Explicit resumable upload from the local spool](brief-05-explicit-resumable-upload-from-the-local-spool.md) | 1 | M | todo | — | — |
| 06 | [Collection replay, retention, recovery and release qualification](brief-06-collection-replay-retention-recovery-and-release-qualification.md) | 3 | M | todo | — | — |
<!-- statusgen:briefs:end -->

## Critical path and dependency waves

`01 local contract/log → 03 database → 04 collector → 06 complete-path qualification`.
The parallel `01 + desktools-v2/61 → 02 producer adapters → 06` and `01 → 05 uploader → 06` paths also bind.
The first useful delivery is 01 followed by 02; neither depends on 03, 04 or 05.
The uploader develops against the contract and fake HTTP transport while the server is built.

- Wave 0: 01 — local contract, spool, preview and offline report.
- Wave 1: 03 — separate database; 05 — explicit uploader.
- Wave 2: 02 — source adapters after 01 and desktools-v2/61; 04 — authenticated collector over the database adapter.
- Wave 3: 06 — complete-path, retention, recovery and packaging qualification.

The real first constraint is review of this bounded collection/consent contract, not a live
PostgreSQL deployment, the parked continuing-operations subject program or the full cell runtime.
Existing local-only desk-tools/23 can ship independently. New schema/local logger implementation
starts from checked-in fixtures and existing runner interfaces. API/db credentials are not prerequisites.

## Local instrument reuse — 2026-10-16

Fresh main already supplies perfrec and statusgen's local adapter. Desktools-v2/61 owns
their compatible boundary/schema evolution; /62 and statusgen/26 own producer phases.
Brief 01 preserves separate correlated collection semantics. Brief 02 consumes the
versioned fixture contract from /61 and therefore moves to wave 2; /06 remains wave 3.
The dependency is one-way: immediate local measurements and credential improvements do
not wait for this stream, a collector or a database. Import old/new timing bases and
availability explicitly; do not fabricate comparable zeroes or additive nested time.
