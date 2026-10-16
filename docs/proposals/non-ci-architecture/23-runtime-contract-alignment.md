# One public runtime: authority, evidence and recovery

**Status: requested architecture amendment, 14 October 2026; implementation and activation
remain separately qualified.** This amendment asks that one runtime implementation in public
`medici-finance/assay` be the destination. It reconciles the earlier refs-first proposal; it does not certify deployment, approve every
unsettled custody mechanism, or delete historical sources. The earlier whole-target approval
is revision-bound at `23f655f5d`.

The earlier whole-target ruling is recorded on [medici-finance/assay#2500](https://github.com/medici-finance/assay/issues/2500).

## Controlling contract and ownership

Use the [whole-system target](11-target-architecture.md), [storage/recovery contract](14-storage-forge-graph.md),
[progressive adoption](19-progressive-adoption-and-oversight.md) and [forge-service amendment](22-forge-service-and-source-queries.md)
as one design. Runtime is the execution capability of that product, not a second controller,
SCM, scheduler, database or cockpit. Keep one semantic owner per domain and compose a modular
Go application. A module boundary remains justified where there is an actual independent
library/release consumer; it does not create independent runtime authority.

| Responsibility | Owner and retained invariant |
|---|---|
| Definitions, acceptance requirements and policy | Reviewed Git sources and existing decision owners; immutable revision references, not caller-supplied replacements |
| Eligibility and evidence applicability | Existing pure evaluators plus R1 records; missing facts, wrong candidate and unavailable corroboration hold |
| Attempts, leases, schedules, brake budgets and effect intent/receipt | R2 PostgreSQL coordination; one transaction owner and stable logical operation identity. Pacing counts are seeded best-effort estimates, separate from effect reservations. |
| Execution isolation and operator authority | R3 runner/cell operation; protected operator incarnation, exclusive mutating repository assignment and separate credentials |
| Provider effects and ambiguous outcomes | R4 role executors; current admission, reconciliation and no blind retry |
| Source and forge observations | R6 typed forge/source service; admitted observations and exact revisions, no acceptance or publication grant from SQL |
| Human/agent interaction | R7 thin clients over the same contracts; agent surfaces cannot manufacture a human act |
| Handover and retirement | R8 qualification and operator cutover; legacy consumers remain supported until their mapped replacement is proven |

Knowledge and learned context may supply versioned, access-controlled inputs. They cannot
append acceptance evidence, grant authority or revise a running plan without the existing
reviewed revision/admission procedure. Runtime remains useful without a knowledge adapter.

## Changes to the earlier refs-first design

| Earlier mechanism | Selected disposition |
|---|---|
| Git fact refs as the sole operational spend/lease authority | Replace with R2 transactions for managed coordination. Retain canonical definitions, durable decisions and published evidence at their existing retained homes. No compulsory Git append before every heartbeat, claim or effect. |
| Per-epoch Git spent chains and a separate ref acceptor | Preserve unique admission, expected-version checks, fencing and receipts through the existing coordination/effect owners. Do not build a parallel ref-spending implementation. |
| Every intermediate fact recoverable from refs | Ordinary restart uses transactional state and backups. Catastrophic reconstruction has explicit gaps and allowed repeated analysis; required audit/decisions/evidence and unknown effect safeguards are not disposable. |
| One physical cell tree prerequisite | Govern qualified repository/component identities without merging histories. Cross-repository reads and planning do not imply atomic cross-repository writes. |
| Forge replacement as the destination prerequisite | Use Git/GitHub/GitLab through the typed service. A native SCM, bidirectional history rewriting and new checkout protocol are optional later proposals requiring evidence. |
| Custom signature/threshold custody treated as initial platform capability | Retain narrow authority and attributable evidence. State what the selected authentication/custody profile actually proves; self-custodial human acts, threshold minting and independent witnesses need separate qualification before those claims are made. |

Existing historical facts, human decisions and Evidence remain unchanged. Source repositories
can retain a provenance archive and a pointer to this contract; they must not dispatch another
runtime implementation. No repository archival/deletion or deployment is performed here.

## Admission, evidence and effects

Candidate content is immutable input, not an admitted change. The workspace owner may capture
and inspect a candidate without moving authoritative refs; checks bind the exact candidate,
acceptance definition and environment. Repair creates a new candidate and preserves the prior
evidence. Necessary Git/PR branches remain until their existing retirement obligations permit
removal; there is no Scion-inspired branch-deletion policy.

The controller admits work using current policy, observation coverage, ownership and budgets.
It commits admission/ownership and a stable effect intent before invoking a role executor.
Nonce/capability replay checks and delivery attempt identity are distinct from logical effect
identity: retrying delivery cannot mint a second operation by changing the attempt or nonce.
Expected versions prevent competing authoritative transitions. The executor checks current
incarnation, generation, candidate, scope and stop state at contact; process isolation keeps
candidate code away from publisher/operator credentials.

There is no distributed transaction joining PostgreSQL to GitHub/GitLab. A confirmed provider
result updates its receipt and mirror projection locally when possible; failed projection is
not permission to repeat the successful write. Read-after-write tokens expose scoped visibility
or explicit pending/unavailable state, not global counters. External edits still require refresh.

Expiry of a warrant or lease, a timeout, absence from a partial search, or a database reset
does not prove an already-submitted request cannot complete. Hold unresolved effects until
operation-specific reconciliation establishes their outcome. Restore/rollback rotates the
operator incarnation outside SQL, contains/revokes old writers and reconciles submitted writes
before conflicting admission. Use the same R2/R4 ledger and the existing recovery contract.

Acceptance requires all applicable requirement rows, policy/security lanes and integration
obligations. Process success, evidence publication, independent acceptance, exposure and observed
benefit remain separate claims. A shared issuer does not establish dependence between two
otherwise independent principals; different keys, role Apps or sessions do not prove independence
when the accountable/delegating principals overlap. Missing corroboration holds the affected
claim. A sole human's act does not gain an independent approval by asking their own agents;
no automatic one-human waiver is introduced by this amendment.

## Source queries and derived results

[Scion](https://www.anthropic.com/engineering/redesigning-source-control) motivates metadata-first
APIs, optional sparse working copies, shared immutable objects, recursive derived data and
enforced cache tiers. Its persistent cache of immutable/derived source data is different from
the write-through mutable forge adaptation here. The workload comparison supplies no Assay
throughput, latency or savings claim.

Use FileAt/Diff/MergeBase at qualified repository and exact commits before broad checkout changes.
Ask the owner of a fact: content/ancestry from the source adapter; provider review/merge observations
from the forge adapter; acceptance from evidence/policy. Jobs need working copies only for
operations that require files. Trial shared Git objects and coalesced fetch with isolated writable
workspaces; a measured no-go is valid. No production daemon, virtual filesystem or SCM rewrite
is selected. Immutable bytes can share storage; current repository membership and caller access
are still enforced before every response, including counts, joins and cached receipts.

Distinguish immutable derivation identity from current validity. A complete derivation fingerprint
names source tree/fact frontiers, candidate/contract, evaluator version, query parameters,
dependency membership/coverage and required policy/environment versions. Time-dependent inputs
also declare their validity bound. No-input-change does not imply an unexpired lease or grant.
Authorization is rechecked by the current boundary; a fingerprint or cached eligible result is
never a capability. Secret material stays out of fingerprints and observations.

Eager/lazy indexes are rebuildable accelerators using those existing owners. Membership removal,
previously missing input, incomplete dependency coverage, clock expiry, revocation or policy
change can require broader reevaluation. Compare incremental results against a fresh full
evaluation with independently specified expected results. Coalesce equal authorized reads,
bound retries and upstream fallback, and prevent cache failure from becoming a provider storm.
Private reads hold if current authorization cannot be established.

## Offline operation, audit and measurement

Offline authoring, signing, inspection and pure evaluation over explicit snapshots remain useful.
They name their cutoff and missing coverage and cannot assert fresh unspent authority or maintain
a live lease. Managed admission/publication requires current cell coordination and executor
authority. Losing a cache permits scoped fallback when available; losing coordination pauses
affected admission. Do not equate offline inspection with offline admission.

Required evidence/audit and the declared fail-closed (brake) budgets use the existing retention and
recovery owners; unknown brake usage holds. Pacing-count accounting follows the
[brake and pacing contract](11-target-architecture.md#pacing-counts-are-best-effort-brakes-fail-closed):
seeded, shared estimates (bounded only where that contract declares a bound) do not impose transactional durability or
hold an otherwise-authorized effect. Diagnostic loss cannot erase a human decision, clear an
unresolved effect or become a fresh allowance. Authentication,
authenticated requester, represented actor and executing provider identity remain distinct.
Retention must identify whether a record is canonical, normally durable operational state,
rebuildable or disposable; required-record uncertainty remains visible.

Opt-in performance data uses the separate performance-collection database and local-first spool.
It is not the cell's coordination store or an acceptance ledger. Disabling it does not disable
required audit records or the declared fail-closed (brake) budgets. Measure cold/warm source and fold
latency, provider calls, bytes, fetch/maintenance CPU/space, coverage and recovery cost before
optimization. No new fleet baseline was obtained in this design review; measurements and cost
comparison remain implementation work.

## Coverage and remaining gaps

Inspection is pinned to public main `56132c5ffb27b5c614a9be2111121c525a7afb50` on 14 October 2026, which includes the merged forge-service briefs of
[2743](https://github.com/medici-finance/assay/pull/2743); the Scion authoring PR
[2907](https://github.com/medici-finance/assay/pull/2907) is this change.
Planned contracts are not reported as shipped: `workflow/` and `statusgen/domain/` are absent
at that main revision. Existing evaluation APIs and runner protocol remain reusable foundations.

| Requirement | Existing owner or new gap |
|---|---|
| Instances, candidate/evidence binding and regression floor | graph-execution/09, /23, /28, /29; do not duplicate |
| Stable effect identity, lost acknowledgments and transactional handoff | graph-execution/04, /19, /25; stable logical keys already belong to /04 |
| Incarnation, ownership, budgets and supervised isolated execution | graph-execution/16, /19, /20, /22, /26; R3/R4 package amendments/activation remain required |
| Admission, controller and complete verifier proof | graph-execution/14, /21, /18; R5 proof remains required |
| Retained records and truthful audit exports | graph-execution/15 and existing records policy; no new audit store |
| Source queries, immutable cache, incremental indexes and Git reuse | merged forge-neutral/39–45 beside /37–38; preserve their existing owners |
| Local-first opt-in performance collection | performance-collection/01–09; collection first, offline/daily reports, bounded issue/draft-repair proposals; separate database and required audit unaffected |
| Complete derived-evaluation context and time-bounded applicability | **new graph-execution/30**; existing /03 coverage handles source/policy/build/environment reuse, while this adds explicit validity/context records for runtime consumers |
| Accountable principals and evidence assurance provenance | **new graph-execution/31**; existing attribution/role checks do not supply principal/delegation independence or proof-class qualification |

Amend unfinished /14, /19 and /25 to consume the two gap contracts, including negative and
end-to-end cases. Preserve historical Evidence, gates and implementation records. The gaps can
be developed after /28 without waiting for a running database; activation still requires R1–R8
qualification. The real R1 head remains independent verification of /02 before /09 and /28.
R7 owner selection and R3–R8 follow-up amendments remain with their existing package owners,
not silently completed by this document. Threshold signing, federation, native SCM, cold-log
storage and multi-host overlapping writers remain separately scoped later work.

## Operating reporting amendment — 15 October 2026

Reuse performance-collection/01–09: local producer capture/coverage → saved snapshots →
one deterministic on-demand command/thin skill → daily report generation → separately
qualified issue/draft-repair proposal mode. All three views (brief completion history,
stream movement/stall evidence, tool run/total/Avg/P95 wall-child-CPU-forge trends) use the
same typed results for CLI, daily artifacts and R7 clients. Local collection/reporting is
useful before R2–R8 and before the optional remote store. It has no work admission/effect
credentials; optional data loss cannot reset required accounting.

R2 owns managed occurrences, schedules, leases and cumulative budgets; R3 owns execution;
R4 reconciles stable issue/PR effects; R6 supplies authorized source snapshots. The optional
performance database holds observations/distributions, not current work/acceptance truth.
Continuing-operations owns assessment meaning; existing desks own investigation, candidate,
review and verification. A merged repair remains awaiting measured outcome, and missing
coverage cannot justify a repair. No second scheduler, runtime DB, forge writer or policy
self-modification is introduced. Existing default gates and qualified legacy paths remain.

## Portable execution follow-through — 16 October 2026

The [execution-environment amendment](27-agent-execution-environments.md) extends this same
public runtime. Agent-execution/01–02 supply capability/profile and preparation contracts;
/03 consumes the existing local supervisor/adapter; /04 tests the existing controller/effect
path under restore; /05 supplies a truthful lifecycle snapshot to current status clients;
/06 independently investigates AX offline. R2 PostgreSQL and R4 effect identity/reconciliation
remain authoritative. No AX task state, runner receipt or filesystem restoration grants
work acceptance or renews authority. Existing R3/R4, celld and R7 activation obligations remain.
