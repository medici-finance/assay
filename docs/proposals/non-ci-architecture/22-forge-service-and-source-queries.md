# Write-through forge service and source queries

**Status:** draft amendment to the future architecture, 14 October 2026. The authoring
request selects this direction; detailed contracts, implementation, credentials and activation
remain review-bound. The ratification at revision `23f655f5d` does not include this later amendment.
This document describes a destination and its proof obligations, not an installed service.

## Source and brownfield boundary

[Anthropic's Scion design](https://www.anthropic.com/engineering/redesigning-source-control)
(published 13 October 2026, read 14 October) describes immutable content-addressed objects,
server-side repository queries, reusable derived indexes, shared node caches and enforced
cache tiers. It does not describe write-through caching of mutable issues and change requests.
The mechanisms below are our adaptations. We retain Git and GitHub/GitLab as their respective
authorities; no new version-control format or git push protocol is proposed.

Implementation inspection is pinned to public main `56132c5ffb27b5c614a9be2111121c525a7afb50`.
`forgeread/reader.go`, `envelope.go` and `offline.go` supply a pure open-issue read contract;
`forgeread/adapters/` accepts admitted transports without custody. `tools/desk/internal/deskkit/`
owns provider resolution, typed GitHub/GitLab effects and existing guards. These are internal
packages of the desk module, not imports an arbitrary new module can consume. `tools/desk/internal/gitcore/`
(internal to the desk module, like `deskkit/`) and the existing worktree/maintenance paths supply Git readers and object-store lifecycle.
`statusgen/evaluation/` remains an evaluation owner, not a cache implementation to rewrite.

The mirror and event contracts in [merged PR 2743](https://github.com/medici-finance/assay/pull/2743)
(`forge-neutral/37` and `/38`, merge commit `6b479a573e6eaf398f2b0adcc6fbedfdc96ab60b`)
are briefs on main, not shipped code. They own admission-bound mirroring,
refresh/coalescing, polling repair, event hints and feedback generations. R2 owns the
PostgreSQL store and effect progress; R3/R4 own authority containment and publication.
The extensions reuse those owners and preserve their decisions and Evidence.

No new fleet-scale cost or churn baseline was obtained. The current read-cost code and tests
in `tools/desk/cmd/deskboard/{readcost,sweepcost_test}.go` provide bounded preservation input,
not proof of a source-cache bottleneck. Proceed with scoped adapters and comparisons; an
unmeasured optimization does not justify rewriting a provider backend or deploying a daemon.

## Responsibility and deployment

The forge service is a logical capability in the modular cell application. Initial trusted
composition can live in `tools/desk/internal/forgeservice/`, with a thin `forgeserve` command;
the public typed read/client contracts extend the dependency-free `forgeread` module in
separate packages. Root/offline consumers acquire no HTTP, database or credential dependency.
Do not export all of deskkit or add a generic raw-request interface.

A query process holds only its admitted read access and mirror-reader role. Each existing
role-bound publisher/executor retains its own credential custody and process boundary;
the query process requests effects over a private typed channel. The separate executor
process is justified by custody and revocation. One service does not mean one all-powerful
forge account, another scheduler, or a second lease/effect ledger.

```text
CLI / agent / cockpit
    -> authenticated, authorized typed query or operation
    -> cell application and existing policy/admission owner
       -> mirror / SHA-bound source-query reader -> scoped observations
       -> stable effect intent -> role-bound executor -> GitHub or GitLab
                                   -> provider receipt
                                       -> R2 transaction: receipt + mirror update
GitHub / GitLab -> authenticated event hints + polling repair -> same mirror
```

Agents receive no SQL credentials and cannot UPDATE an issue row to request publication.
The mirror is view-only to consumers while ingestion and confirmed effects update it.
SQL projections do not become authoritative issue/PR/MR records. Published evidence and
accepted decisions retain their existing canonical owners.

## Write-through behavior and failure contract

Begin with synchronous provider effects, not queued editing of mirrored rows. Initial
consumers are issue creation and change comments through the existing desk operations.
Review approval, ready flips, merges, releases and evidence publication keep their current
separate authorities and are not implicitly added to this surface.

1. Admit the explicit operation under current policy, identity, candidate and ownership.
   Persist its stable identity and bounded payload digest in R2's effect ledger before contact.
   Retried delivery keeps the logical operation identity; a new attempt is not a new effect.
2. The role executor independently checks its capability and relevant current state, applies
   body/trust/budget/stop controls and uses its configured provider credential. Preconditions
   are checked by the provider where supported; otherwise record the read/write race and
   use the operation's reconciliation contract. Never invent universal conditional writes.
3. Persist confirmed provider identity/result and the mirror's confirmed fields in one local
   transaction when possible. Update using the mirror's admission and refresh ordering rules;
   a late pre-write refresh cannot overwrite a newer observation. Treat webhook data as a hint.
4. Return an operation receipt plus a scoped opaque local observation token. Do not expose
   a global SQL sequence/LSN or a cross-scope activity counter. A read requesting that token
   receives an observation containing the effect, a newer authoritative observation, or explicit
   pending/unavailable coverage. It must not silently read an older cached result.

There is no atomic transaction across the forge and PostgreSQL. Return separate provider
outcome and projection status; do not translate a successful external write plus failed
projection into permission to repeat it. Missing reply, process death or lost database
record leaves an unknown outcome until operation-specific reconciliation succeeds.
A caller timeout is not evidence that the provider rejected the request.

Native idempotency keys are used only where that endpoint supports them. Operation-specific
markers/receipts and scoped provider enumeration can help reconcile; absence from a partial
or stale search proves nothing. A comment marker is identification, not a provider-enforced
uniqueness constraint. Serialize the logical operation and hold an unresolved potentially
submitted write rather than promise exactly-once publication. Different payloads under one
operation identity conflict. Reads of an existing receipt still require current caller access.

Human edits and other integrations bypass our path. Webhooks plus bounded polling repair
refresh them; write-through visibility applies to our confirmed writes, not instant global
consistency. A newer authoritative observation may legitimately reverse an earlier effect.
Losing admission access blocks cached content even if the bytes remain retained under the
mirror's independently selected retention policy.

## Authentication and authorization

There are three independently enforced boundaries: caller identity, application policy and
provider credential/permissions. Authentication answers who sent the request; neither a role
label nor a forge credential's broad scope supplies application authority.

| Caller or boundary | Identity and enforcement |
|---|---|
| Local agent / CLI | Trusted launcher supplies a short-lived signed capability over a protected per-cell channel. Socket permissions restrict peers, but same-UID peers are not distinct role identities; R3 isolation protects capabilities and executor custody. Never derive role from an HTTP header or agent-chosen environment value. |
| Human cockpit / remote client | Trusted ingress validates an operator-configured OIDC identity (issuer, audience, stable subject, expiry) or an explicitly supported workload credential; operator policy maps it to permitted cells/repos/actions. Browser sessions require secure cookies and request-forgery protection. R7 owns remote login integration; no remote listener is activated by the local slice. |
| Query service | Authorize every request against current subject grants and source admission, including cached hits. Filter inputs before joins, counts, errors, notifications and pagination. A reader receives only the intersection of its grant and the admitted source scope; the ingestion App's broader visibility is not the reader's grant. |
| Role executor | Validate the scoped capability and current incarnation/assignment/attempt generation, policy and stop state immediately before contact. The process holds only its role's provider credentials and accepts enumerated operations, never arbitrary URLs, roles or token requests. |
| Forge | Enforce configured App/token repository scope, endpoint permissions and branch protection/approval rules. Provider authorization is an independent ceiling; coarse provider permissions still require our narrower operation policy. |

Capabilities bind issuer/audience, stable caller, cell, provider/host/repository identity,
role, allowed operation, logical operation identity and payload digest for writes,
work/attempt/candidate where applicable, authority incarnation,
generation, policy/access epoch and expiry. The existing R2/R3 authority owner issues them;
this service cannot mint itself a stronger role. Reusable read grants and single logical-write
authorizations have different replay rules. Re-presenting a write authorization can retrieve
its receipt, not execute another payload or a second logical effect. Authentication material
never enters ordinary observations, snapshots, cache keys, diagnostics or performance export.

Access checking extends mirror 37's existing rule: current authorization and the exact
composition-derived admission must match at both library and store. Authenticated ingress
maps principals to admissions server-side; caller-supplied admission IDs cannot select another
reader's rows. A wider or different admission cannot reuse a cache entry as permission.
Counts and the existence of private repositories are protected as well as bodies.

Policy/access epochs invalidate responses and derived caches. Before exposure, deployment must
select and test the maximum entitlement lease, invalidation/expiry behavior and revocation bound.
Revocation affects new queries and effects within that declared contract; already-returned bytes
and already-submitted provider writes cannot be recalled. If current authorization cannot be
established, hold affected private reads/effects. An operator-managed delegation must be explicit:
logging in with GitHub or GitLab does not automatically grant everything the ingestion bot sees.

GitHub execution keeps the role App and mints repository/permission-restricted installation
tokens where supported. Those tokens cannot exceed the installation's grant and expire after
one hour. [GitHub installation-token documentation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app)
GitLab retains the configured service-account/project credential boundary, matching its actual
role, scope, expiry and edition. Project access tokens have provider-specific availability and
internal-visibility caveats; they are not an automatic equivalent of a GitHub role App.
[GitLab project-token documentation](https://docs.gitlab.com/user/project/settings/project_access_tokens/)
Never fall back to ambient human CLI authentication or share a read token with a writer.

Record authenticated requester, represented actor, executing provider identity, role, approved
operation, subject and result separately. A bot acting for a human remains a bot provider write;
that provenance cannot manufacture a human decision or independent review.

## Applicable Scion mechanisms

| Mechanism adopted | Bounded application and proof |
|---|---|
| Metadata queries before broad source checkout changes | Provide typed FileAt, Diff and MergeBase queries bound to exact repository and commits. Clients can inspect a change without a checkout. Merge time/review/check questions go to forge observations; content/ancestry goes to Git; acceptance goes to its evidence owner. Unsupported provider/Git history is explicit. |
| Immutable result reuse | Key source results on qualified repository, commit/object identity, path/query parameters and derivation version. Prove content integrity and authorized repository membership before a hit. Mutable branch refs, issue state, review currency and check results use separate refresh/version rules. SHA equality alone is not permission. |
| Incremental derived data | Keep reverse input-to-result dependencies and invalidate only safely bounded affected queries. Membership changes, missing dependencies, time/expiry and access epochs can require broader invalidation. Compare against a fresh full evaluation. No new readiness or acceptance algorithm. |
| Shared node objects / small clients | Trial the existing common Git store, isolated worktrees and serialized/coalesced fetches. Benchmark cold/warm operation, native Git and go-git, alternate-store lifecycle and access isolation. A daemon, snapshots or virtual filesystem require a demonstrated benefit and a separately qualified lifecycle; no automatic deployment. |
| Enforced cache tiers | One scoped refresh owner coalesces demand, respects provider limits/backoff and protects the upstream. A cache failure cannot fan out into one full scan per client. Bounded stale reporting remains useful; execution requiring current facts holds. |
| Candidate separate from mutable refs | Bind review/source analysis to exact commits and input versions. Reuse graph-execution/23–25 and existing detached-review/workspace ownership; do not author a second candidate or cleanup owner, delete necessary PR branches or change the merge-never-rebase policy. |
| Hermetic simulations | Exercise served queries, a real disposable PostgreSQL store, isolated role executors and fake GitHub/GitLab transports under reordered events, lost replies, crashes, revocation and restore. A fake cannot certify provider permissions or live deployment. |

## Acceptance, cost and rollout

The first useful slice is the typed source-query contract plus authorized offline/local fixtures.
R6's live mirror depends on R1/R2; managed write-through additionally needs R3/R4 containment
and authority. Those are real blockers to activation, not reasons to delay pure contract tests.
Read-only service release and write activation are separately qualified. Preserve current
working desk paths until a complete supported consumer flow and credential inventory pass.

Tests must prove a permitted request and bypass the upper gate to exercise executor and store
refusals. Cover worker-to-reviewer escalation, wrong audience/repository/instance/candidate,
expired or replayed capabilities, cached private content/counts after permission loss,
provider-success/SQL-failure, crash after send, stale refresh after receipt, complete database
loss and queued stale incarnation. No automatic replay after unknown outcomes or restore.

For a burst of 50 identical authorized reads with an identical scope/admission, no invalidation
and one scripted paginated inventory, one source walk suffices; different admissions do not
coalesce into a shared authorization. Under a provider outage, measure attempted calls against
the configured retry budget and show incomplete coverage. This is a synthetic request-count
acceptance fixture, not a production latency promise.

Measure cold/warm p50/p95/p99 latency, provider calls, bytes, cache hit rate, clone/fetch and
maintenance CPU/space, correctness coverage and recovery cost. Select workload-specific budgets
before each experiment; unknown fleet measurements remain unknown. A faster stale/underprivileged
answer is not a pass. The shared-Git trial can validly conclude that the current path is cheaper.

Implement and compare under synthetic fixtures first; then shadow authorized reads, qualify the
served boundary and recovery, inventory legacy credentials, and request one scoped operator
cutover. Preserve one mutating cell per external repository. No SQL edit is a publication API,
no new universal service token is provisioned, and no deployment/merge/release is performed by
this authoring change.

## Performance boundary contract — requested amendment, 19 October 2026

[Perf-v3](../../performance-attribution-spec.md#querycache-service-and-passthrough-boundaries)
measures service client calls, per-handler query/cache/coalescing work, and actual upstream
direct/passthrough/refresh/poll/repair requests separately. Local/cache answers remain observed
operations even when their covered upstream call count is zero. Client/server views of one
logical request cannot be added into two queries, and a shared refresh's provider pages or
GraphQL points cannot be copied to every waiter. Missing service instrumentation stays unknown.

Per-request context is immutable; concurrent handler CPU cannot be inferred from process CPU.
Opaque diagnostic links require explicit service-protocol/transport admission and workflow
consent on both ends; profile selection alone adds no headers, authority, endpoint discovery
or raw work locator forwarding. Required auth/access/receipt fields and optional diagnostics
remain distinct. /39 owns typed client context, /37–38 refresh/poll, /40–41 handler/executor,
/42–43 cache/source outcomes and /45 the real served workload proof. This does not claim the
planned service exists or add a service prerequisite to the local perf-v3 recorder.

## Candidate-plan query consumer — requested 23 October 2026

Expose the [existing candidate-plus-priority planning contract](../../desk-workflow-efficiency-spec.md#21-candidate-plus-priority-planner--requested-amendment-23-october-2026)
through current /39–43 serving/derived-query owners. Scope filtering precedes joins and
counts. The supplied policy/evaluator revision, time/capacity, candidate membership and
source freshness bind selection; a local priority change need not fetch unchanged GH
facts. Preserve visible missing matches and partial coverage. One mirror refresh can feed
prose/discrete planning and depth only at compatible scope/cutoff, while role executors
retain consequential current-state checks. No new endpoint is promised by this pointer;
qualify producer/consumer versions and count the retained provider work once.
