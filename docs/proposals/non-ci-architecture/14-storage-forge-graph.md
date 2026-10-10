# Storage, recovery, forge mirror and graph queries

**Draft proposal, 10 October 2026.** The database **should be rebuildable to a useful, safe operating state**. Recovery need not reproduce every row or in-flight event: some information is transient and some work may repeat after rare database loss. PostgreSQL owns operational coordination; Git retains reviewed intent and durable evidence, and the forge owns its external records. A mandatory lossless Git journal before every action is not required.

## Recommend PostgreSQL for the running cell

Use PostgreSQL in a local Docker container initially, then CloudNativePG (CNPG) on Kubernetes. Keep one runtime database implementation and migration set. SQLite can remain an offline/read-only projection format where useful, but I would not build two interchangeable online backends at the start.

The reason is the now-explicit combination of a queryable forge mirror, graph joins, concurrent service users and a likely Kubernetes deployment. SQLite's durability is not inherently inadequate for a rebuildable local index. PostgreSQL gives this product a more direct operational path to the intended deployment without changing database engines during cutover.

A local PostgreSQL container is still **single-host**, with persistent volumes, backups and restart/recovery requirements. CNPG manages PostgreSQL replication and failover, but HA requires replicas on appropriate failure domains and a chosen replication/durability policy. Several pods on one host do not survive losing that host; replication also does not replace backups. [CloudNativePG overview](https://cloudnative-pg.io/docs/current/), [replication and durability](https://cloudnative-pg.io/docs/devel/replication/)

A shared local PostgreSQL server can host a separate database and restricted application role per cell. That is a proposed first profile, not protection from a compromised database/host administrator. Keep controller/executor isolation and scoped credentials outside that database. A profile requiring stronger failure or trust separation gets separate instances/clusters. Graph traversal and API authorization remain scoped even when the database server is shared.

Local-to-CNPG migration is explicit: pin compatible PostgreSQL/extension versions, quiesce admission, migrate or rebuild at a declared recovery point, validate results, switch endpoints, reconcile outstanding effects and reopen admission. Do not put a live local database directory on a network volume and call that migration.

## Canonical records, operational state and acceptable loss

| Data | Normal owner | Recovery and loss policy |
|---|---|---|
| Specs, briefs, targets, contracts, approved configuration and durable decisions | Reviewed Git sources or the existing decision record's canonical provider | Re-read exact versions; rebuild relationships and work obligations. These are not disposable runtime data. |
| Published verification/evidence records | Existing governed Git publication path for this phase | Rebuild applicability and accepted completion from retained records; do not manufacture missing results. Moving this durable home outside Git is later work. |
| PR/MR/issues, reviews, comments and provider check/status facts | Forge, mirrored in PostgreSQL | Re-enumerate permitted current state. Old versions/deleted content are recoverable only if retained elsewhere; missing history is explicit. |
| Attempts, leases, schedules, effect intents and receipts | PostgreSQL operational store | Normally durable through transactions, backup and replication. After complete loss, reconstruct obligations, reconcile external state, mark lost/unknown attempts and allow safe repeated work. |
| Graph indexes, query caches, cursors, heartbeats and connections | Rebuildable/transient service state | Recompute or reset. An old heartbeat is not proof a process is alive. |
| Unpublished drafts, human attention, local overrides and detailed diagnostics | Policy must classify each field | Persist important accepted changes through their canonical path. Any disposable fields need an explicit loss policy; do not quietly promise their recovery. |

The database is an operational authority, not the sole canonical record of Assay's intent and accepted outcomes. Calling it a cache does not mean clearing it during normal execution is harmless. The recovery contract is **semantic reconstruction with documented gaps and repeated work**, rather than a bit-for-bit recreation of all transient state.

A source-code checkout alone does not contain PR discussions or live provider facts. Useful reconstruction combines Git definitions/evidence with current forge reads, plus retained snapshots where required. An offline reconstruction can show only the recorded cutoff and its missing coverage. We should not archive every webhook into Git merely to make all database rows reproducible.

## Normal operation uses database transactions

The controller commits admission, claim generation and an effect intent in PostgreSQL before invoking an executor. Record the result and update projections transactionally where appropriate; use an outbox/inbox and stable identifiers for delivery/retry. A crash with the database intact resumes from these durable records.

There is **no mandatory Git append on the critical path of every claim, heartbeat or effect**. Git publication remains required at the established durable evidence/approval milestones. This avoids introducing a second distributed coordination system solely to reproduce transient information. Losing SQL-only working state after catastrophic database loss is an accepted tradeoff within the recovery policy.

Use an operation identity derived from recoverable work/candidate/contract/effect coordinates where feasible, rather than relying only on a random database row ID. Provider receipts, tagged artifacts, conditional writes and supported idempotency mechanisms help reconcile after a lost local record. Permission and generation checks stay at the role-bound executor; a restored controller must not assume it has fenced an old process simply because its database is fresh.

Repeated analysis, polling or verification can be acceptable. Repeating a merge, publishing duplicate evidence or creating a second competing work item may not be. Classify operations as safely repeatable, repeatable after dedup/reconciliation, or requiring resolution when the outcome is unknown. This does not promise exactly-once effects; it makes the recovery behavior explicit.

## Recovery is a routine restore path plus a reconstruction fallback

**Ordinary failure:** use PostgreSQL recovery, a validated backup/point-in-time restore or CNPG failover as appropriate. The exact replication and backup policy sets the recoverable interval; do not claim zero loss from the product name. Reconcile any uncertainty at the application boundary before resuming affected operations.

**Complete database loss:** start with mutation admission paused; load trusted definitions, evidence and configuration; re-enumerate forge objects; rebuild graph/indexes and outstanding obligations. Recover stable canonical identities from their source coordinates. Mark missing transient attempts/history explicitly. Discover and stop/reconcile surviving old runners before granting new conflicting ownership; allow a fresh attempt when repetition is safe.

### Authority surviving database loss

The selected first-host design uses a **fresh random authority incarnation and capability-signing key on every complete reconstruction, point-in-time rollback and ownership transfer**. The trusted operator stores the current incarnation outside PostgreSQL in protected host control state, with the repository assignment. SQL generations are valid only inside that incarnation. A database backup or Git configuration revision cannot restore an old incarnation/key as current.

While restoring, pause mutation admission and stop the entire affected cell execution/publisher group using the operator's process/container boundary, not a list of attempts recovered from SQL. Destroy old sockets and executor capabilities, rotate the incarnation/key, and start clean role executors. Every new effect admission checks the current incarnation and matching owner/generation; stale queued requests and capabilities fail even if their old SQL row/generation reappears. Evidence retains the old incarnation as provenance without reactivating it.

This mechanism complements credential containment. Runners never receive provider publication credentials; only the role-bound publisher holds them. Teardown must prove that old publishers cannot remain outside the managed group. Revoke or independently restrict old provider credentials and minting access; where revocation is unavailable, await expiry before reopening any scope they could mutate. Already-submitted requests still require external reconciliation. If the containment or credential inventory is incomplete, keep affected publication paused instead of inferring that a new random ID stopped external effects.

Restore tooling must make this operator procedure unavoidable for both deletion and backup rollback. A mismatched/missing operator incarnation and SQL activation record starts paused. Automatic database failover with intact current state follows the normal lease protocol; it must not silently replay a stale restored cluster under an old activation record. Automatic multi-host recovery/HA needs separate qualification later. Loss of both SQL and operator state requires a fresh incarnation plus complete teardown/credential reconciliation, never reuse from a stale backup.

Recovered pending work must not replay provider writes blindly. Compare against current external objects and accepted evidence, deduplicate where possible, and hold only the unresolved consequential action. A verification may run again even while its publication awaits reconciliation. Missing cost/history data remains an incomplete measurement, not a reset to a claimed zero.

Budget counters, idempotency markers and audit-backed authority need a distinct recovery policy. Preserve them transactionally during ordinary restart; after unrecoverable loss, unknown usage cannot silently become a fresh allowance. Reconstruct from retained evidence where possible, otherwise use conservative bounds or an explicitly authorized reset while affected effects remain held. Canonical required audit records need a retained home; disposable diagnostics may be lost. Permission to repeat verification does not authorize duplicate publication, notification or an unrecorded budget reset.

The first slice needs a **delete-the-whole-database recovery test** proving canonical work/evidence reconstruction, declared transient loss, safe repeated work and controlled publication. It need not restore every intermediate row. Backups make recovery faster and more complete; Git/forge reconstruction is the fallback, not the expected response to every restart. Measure rebuild time and API cost, and rate-limit the reconstruction so it does not cause a provider storm.

The specific tolerable recovery time, history window and loss classes still need to be written into the implementation contract. The design permits some transient loss and repeated work; that does not imply loss of durable human decisions or already published evidence is acceptable.

### Notification deduplication is a carried contract

Notification delivery must preserve deduplication and persisted delivery-unknown outcomes. Ordinary process restart preserves the no-repost obligation. After total SQL loss, reconstruct delivery identities/outcomes where provider records or retained canonical evidence support them; unverifiable pre-loss delivery is held for reconciliation or an explicit operator resend decision. Do not replay all transitions as fresh notifications. Circuit-breaker timers and pending coalescing may be transient. Allowing duplicate notifications requires a separate policy amendment; it is not implied by accepting repeated analysis or verification.

## Reuse and expand the existing forge-mirror work

Reuse existing mirror and event-consumer designs after checking their current source, implementation and approval state. The target includes queryable PR/MR/issue data, freshness, conditional reads, events, desk wakeups and fallback. A poller for a selected issue subset is not a complete forge mirror.

Reconcile existing mirror consumers as part of the public service program. An earlier design does not automatically authorize a public inbound receiver, broader machine authentication or new check-state permissions. Event/poll ingestion needs an explicit receiver, retention and credential-scope contract, with deployment through the normal activation path.

The target read service stores typed provider/repository/issue/change identities, PR versus MR provenance, state, revisions, labels, authors, relationships, requested reviewers, review/approval observations, comments and check/status observations **only where configured permission and retention allow them**. Preserve provider-specific facts beside common concepts; a GitHub review and a GitLab approval rule cannot be reduced to the same boolean by convenience. Missing permission is visible coverage, never “no failing checks.”

The mirror preserves issue closure reason, canonical/superseding target and linked delivery evidence separately from acceptance, released version and active consumer observations. A closed planning question can retain a dependency hold, and a superseded issue can point to a merged repair. Neither `closed` nor a PR merge alone asserts that every deployment is fixed. Source revisions and observation times remain attached to those relationships; downstream authoring queries include closed regressions as well as open incidents.

Proposed pipeline:

```text
Authenticated forge events + scheduled reconciliation
    -> deduplicate, coalesce and fetch the required scope
    -> materialize PostgreSQL mirror and graph links
    -> retain snapshots only where the history/recovery policy requires them
    -> emit scoped change notifications
    -> desk / status / planning / cockpit queries
```

Start with one ingestion owner per authorized repository/scope, not one poller per verb. A cell-scoped service is simplest; sharing ingestion across cells later requires matching access scopes and explicit ownership. Initial enumeration follows every required page. Incremental polling overlaps update windows, reconciles removals/closures, and periodically performs a complete bounded inventory. On failure, preserve prior rows as stale; do not advance a completeness watermark for an unfinished scan or treat absence from a partial response as deletion.

Webhooks accelerate refresh; polling repairs missed notifications and covers unsupported events. Authenticate deliveries, retain provider delivery IDs, deduplicate, tolerate reorder and reconcile authoritative current state before consequential actions. Acknowledge a webhook according to the ingress durability contract, then process asynchronously; polling repairs a lost transient hint. A retained-history watermark is separate from the current mirror's freshness watermark. GitHub documents missed-delivery recovery and stable delivery IDs on redelivery; GitLab provides project/group webhook mechanisms with its own event behavior. [GitHub webhook guidance](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks), [GitLab webhooks](https://docs.gitlab.com/user/project/integrations/webhooks/)

Routine queries use the mirror. A consequential write may need a targeted fresh read or provider precondition rather than another fleet-wide scan. Centralize these refresh requests, request coalescing, rate limits and backoff; a mirror outage must not cause every client to start a competing full poll. Responses include source scope, last complete refresh, observed revision, retained-history window and unavailable/stale status. Events are prompts to reevaluate, not approval or proof that a queue is empty.

Forge history and deletion need an explicit retention policy. Re-polling the provider cannot recover an edited/deleted comment's old body. Selective retained snapshots can preserve what was observed, but expand the private-data footprint and storage volume. Define which history matters and what may disappear after database loss; do not imply a record of every event that ever happened. Canonical verification/evidence stays in Git for this phase. Existing read-only reports should remain useful while freshness-dependent execution is held.

## Graph capabilities from the first slice

GraphQL is a client query language/runtime contract; it does not require a graph database and does not itself provide storage traversal. A GraphQL read API can sit over PostgreSQL if clients benefit from it. Keep mutations behind typed application use cases with their existing authorization. [GraphQL introduction](https://graphql.org/learn/)

For architecture and work relationships, begin with versioned node/edge tables in PostgreSQL, indexed in both directions and scoped by cell, visibility, graph snapshot and target revision. Use bounded recursive queries for dependency/impact traversal, alongside relational joins to issues, plans and evidence. PostgreSQL provides recursive queries and cycle-detection support. [PostgreSQL recursive queries](https://www.postgresql.org/docs/current/queries-with.html)

Qualify this with the actual queries: current and future consumers of a contract; pending briefs affected by a requirement; all blockers with source explanations; evidence applicable to a candidate; and PRs/issues related to an intended architecture change. Test cycles, high fan-out, multiple snapshots and inaccessible nodes, not just a short happy-path chain. Bound depth, rows, time and query cost. Avoid combining an edge from one target revision with a node from another accidentally.

Apache AGE is a credible PostgreSQL graph-extension candidate if Cypher-style matching materially improves those queries. Its published packages are version-specific, so choosing it adds an extension/image/upgrade/recovery qualification obligation for local Docker and CNPG. Pin and qualify the actual supported extension/database release pair before adoption. [Apache AGE releases](https://age.apache.org/download/)

I recommend PostgreSQL graph tables and the query contract from day one, with a small SQL-versus-AGE comparison before freezing the schema if the real workload needs complex pattern matching. Do not add a separate graph database or two graph write owners speculatively. Either representation remains rebuildable from the same source records. Semantic owners and public/private traversal rules belong above the storage engine.
