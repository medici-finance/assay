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

## Workspace retirement is a resumable operation

Extend the existing attempt/lease model with resource identities: repository, common Git store, work/candidate, owning attempt and generation, resource kind/locator, retention obligation and durable evidence destination. Cover owned worktrees, local task branches, detached review HEADs and scratch. Remote branches and local tracking refs are distinct resources with distinct authority. Keep PostgreSQL as operational coordination and the established Git publication path for retained evidence; no mandatory per-step Git journal is introduced.

Track `requested → eligibility-checked → preservation-confirmed → removing → reconciled`, with explicit held, failed and unknown outcomes. Persist the operation and each completed step so removal can resume after a crash without guessing. Preservation may be explicitly not required under the approved loss policy; otherwise verify the saved artifact/receipt before deletion. A cleanup request grants no new deletion authority.

Before each destructive step, revalidate the resource identity, expected ref tip and owner/generation against allocation, and confirm no active holder or required unpublished work remains. Reacquisition invalidates an earlier retirement decision. Check untracked and ignored evidence as well as tracked changes. Age, missing database rows, a clean working tree or expired heartbeat alone never proves disposability. Database-loss reconstruction inventories unknown resources and holds them for ownership/evidence reconciliation; it does not delete them as orphans.

Retirement revalidates against allocation, so allocation must be recorded when the resource is created. Fleet tooling that provisions a worktree, task branch or scratch directory stamps the resource identity — owner, attempt/generation, class and retention obligation — through its allocation verb. A resource created outside those verbs is **unclassified** and is held for classification; age, a path pattern or a cleanup promise written in prompt text never retires it, because a crashed executor keeps none of those promises and no later reader can tell a spent scratch resource from an abandoned live one. Classification is an explicit recorded act — an owner claims the resource or a provisioning verb records its class — and it converts the hold into normal eligibility; no age threshold or default class ever classifies a resource.

Eligibility is class-aware. A detached review head is fetch-only by construction: the reviewer authors findings on the forge, not commits in the tree, so preservation of tree content is explicitly not required under the approved loss policy and the pushed/merged proofs are inapplicable rather than waived. Its retirement eligibility is the completed owning review attempt plus the recorded class and holder; a sweep that cannot see an allocation record holds the head instead of inferring the class from a name.

For branch retirement, forge observations extend topology. A change record that is merged and not later reverted, with its owning attempt complete and required evidence retained, establishes retirement eligibility even when a squash-merged local branch is not an ancestor of main, provided the local tip is that record's merged head revision or one of its ancestors — a tip that advanced past the merged head holds as unpublished work, and a revert state that cannot be determined holds; an open change record holds the branch regardless of idle age. Idle age may trigger an eligibility check and never proves disposability by itself; every age-triggered check resolves through ownership, class and evidence state.

A lock is itself a resource with a lifecycle. Acquisition records the owning session, attempt and generation in the lock; liveness evidence is the owning session's recorded activity while it lives, and a quiet beacon or expired heartbeat alone never proves the owner gone, consistent with the disposability rule above. A lock whose owning session is recorded as dead is retired as part of resolving that session's resources; an unstamped lock may be checked after a declared window measured from recorded acquisition, and holds until recorded holder-gone evidence or an explicit recorded classification names an owner; an unreadable or missing lock record holds rather than guesses. Age never retires a lock by itself, so a live unstamped holder keeps its mutual exclusion. Retiring a lock never removes the resource it guarded — it only returns that resource to the normal eligibility checks.

Worktree removal, owned local-branch retirement and remote-reference reconciliation are separate steps. A squash-merged branch is not necessarily an ancestor of main; a historic patch match does not establish disposability if main later reverted it. Retain ambiguous work. Preserve normal reflog recovery periods and inventory alternate-object borrowers before reclamation. A retry after partial scratch deletion must retain enough ownership information to finish or hold safely.

## Repository maintenance has one owner and a measured budget

Four operations have different purposes: retire completed work and refs; organize object packs and indexes; reclaim objects when retention permits; remove repeated lookup work in readers. None is a substitute for the others. Resolve linked worktrees to a common Git directory and give that store one maintenance owner under the cell's existing resource coordination. Separate stores remain separate maintenance units even when they represent the same external repository.

Run explicit tasks with version/capability checks, measured work/time limits, adequate temporary space and a declared cancellation/recovery policy. Coordinate tasks with active Git operations and other maintenance; application locks alone do not establish compatibility with unmanaged Git processes. Keep expensive maintenance outside task-startup latency. Network prefetch is an explicit profile choice, not a side effect of enabling a default schedule. Do not add immediate reflog expiry or `gc --prune=now` as routine cleanup.

The maintenance owner runs through a standing supervisor with an explicit interval, not through desk-loop ticks or prompt-driven sweeps. One sweep implementation holds the store's maintenance singleton and releases it between sweeps, so manual and boot-time sweeps are never starved across intervals; a per-OS service definition keeps the supervisor alive and is recorded as deployment profile rather than product logic. Role boots may sweep opportunistically; no coordinator or role session owns the cadence.

Per-resource policy at lifecycle moments — creation and pre-removal — is a hook surface, deliberately separate from scheduling: hooks carry machine-local policy such as transcript and scratch cleanup and never start, delay or replace a maintenance sweep. A hook never removes a classified resource outside the retirement state machine and its revalidation; hooks carry policy beside the sweep, never a second pruner. Repository-local sweeper scripts consolidate behind the single owner: their eligibility gates fold into the sweep behind explicit flags, and the script becomes a shim or fails with an unsupported-version message rather than remaining a second pruner.

[PR #2516](https://github.com/medici-finance/assay/pull/2516) supplies bounded manual ref packing and commit-graph maintenance. Object-pack consolidation, scheduling and unreachable-object reclamation need separate qualification. Upstream [Git maintenance](https://git-scm.com/docs/git-maintenance) and [multi-pack indexes](https://git-scm.com/docs/git-multi-pack-index) are mechanisms to evaluate, not proof every library reader uses them.

Measure ref/worktree counts, held reasons, loose objects, pack count/bytes, reader index/object opens, operation latency, maintenance duration and peak temporary space. Report stale/unavailable measurements rather than zero. Benchmark the actual native-Git and go-git paths independently on old repositories; reduced index opens are not an end-to-end latency claim. Select cadence, limits and recovery objectives from the workload before activation rather than inventing universal thresholds.

Any future hot/cold evidence tiering must preserve the canonical-record and retention contracts above. A checkpoint can reduce projection work while Git ancestry still retains all old objects; a cold export alone proves neither local reclamation nor restorability. Require a physical reachability/layout explanation and verified restore before claiming storage savings. Moving durable evidence or acceptance authority to another store is a separate architectural decision.

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

### Feedback received during an active attempt

Persist a scoped reevaluation requirement when new relevant feedback arrives while work is active, rather than admitting a second writer. Bind it to the work/candidate and observed source revision or refresh requirement. Distinct instructions, decisions and effect requests remain separate attributed records; coalescing refresh demand must not erase their contents or authorization requirements.

Record demand and finish/consume it transactionally with attempt ownership and the existing outbox. Use an observed/processed generation or equivalent contract so feedback racing completion cannot be cleared by an older run. Admission consumes a known generation; later arrivals remain pending. A successor refreshes authoritative permitted state and reevaluates eligibility instead of replaying an old webhook as an instruction. Superseded candidates retain explicit disposition. A crash between terminal state and enqueue is repaired from durable demand; duplicate delivery cannot create competing attempts.

After complete database loss, re-enumerate source obligations where possible and report missing historical coverage; do not claim every transient message was recovered. Provider polling repairs refresh hints, not deleted instructions or missing authority records.

## Graph capabilities from the first slice

GraphQL is a client query language/runtime contract; it does not require a graph database and does not itself provide storage traversal. A GraphQL read API can sit over PostgreSQL if clients benefit from it. Keep mutations behind typed application use cases with their existing authorization. [GraphQL introduction](https://graphql.org/learn/)

For architecture and work relationships, begin with versioned node/edge tables in PostgreSQL, indexed in both directions and scoped by cell, visibility, graph snapshot and target revision. Use bounded recursive queries for dependency/impact traversal, alongside relational joins to issues, plans and evidence. PostgreSQL provides recursive queries and cycle-detection support. [PostgreSQL recursive queries](https://www.postgresql.org/docs/current/queries-with.html)

Qualify this with the actual queries: current and future consumers of a contract; pending briefs affected by a requirement; all blockers with source explanations; evidence applicable to a candidate; and PRs/issues related to an intended architecture change. Test cycles, high fan-out, multiple snapshots and inaccessible nodes, not just a short happy-path chain. Bound depth, rows, time and query cost. Avoid combining an edge from one target revision with a node from another accidentally.

Apache AGE is a credible PostgreSQL graph-extension candidate if Cypher-style matching materially improves those queries. Its published packages are version-specific, so choosing it adds an extension/image/upgrade/recovery qualification obligation for local Docker and CNPG. Pin and qualify the actual supported extension/database release pair before adoption. [Apache AGE releases](https://age.apache.org/download/)

I recommend PostgreSQL graph tables and the query contract from day one, with a small SQL-versus-AGE comparison before freezing the schema if the real workload needs complex pattern matching. Do not add a separate graph database or two graph write owners speculatively. Either representation remains rebuildable from the same source records. Semantic owners and public/private traversal rules belong above the storage engine.

## Oversight state uses the existing authority and recovery owners

The [oversight contract](19-progressive-adoption-and-oversight.md) adds policy bindings,
approval units, sampling decisions and pending human gates to the existing transactional
coordination/decision/outbox owners. Canonical approvals and required audit records
follow their retained-record policy. Ordinary restart preserves sampling/consumption;
after reconstruction unknown history is held for authorized reconciliation rather
than treated as fresh permission or unsampled work. Old capability/incarnation
refusal and notification delivery-unknown/deduplication remain controlling.

Standalone Inspect/Assist has canonical local records/observations, without in-flight
leases, authority grants or another coordination database. Offline output remains
usable without PostgreSQL; it cannot publish governed evidence. Notification batching
coalesces presentation only, preserving distinct pending decisions and their subjects.

## Message transport ownership (proposed; pending approval)

Proposal against the ratified revision `23f655f5d`; not baseline text until approved and reconciled in the amendment manifest.

The [communications target](21-communications.md#storage-boundary-and-broker-decision)
assigns durable message transport to a selected broker. PostgreSQL owns the application
transaction and its outbox/inbox deduplication bridge, not a competing mailbox engine.
Broker acceptance, recipient acknowledgment and completed business effect are distinct.
Messages that exist only in the broker cannot be reconstructed from Git/forge; the
selected profile must qualify its own persistence, replication, backup and loss policy.
The current file backend remains a transition implementation until explicit migration.
