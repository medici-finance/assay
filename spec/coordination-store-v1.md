# coordination-store-v1

Status: foundation and bounded mirror Store implementation; full R2 domain schema remains planned.
Owner: `workflow/store` (graph-execution/36 substrate; graph-execution/19 domain delivery).

## Implemented foundation schema

The cell has one PostgreSQL coordination database and one forward-only Goose migration
ledger. `workflow/store/migrations/ledger/create.sql` defines its bootstrap; numbered
SQL migrations in the same directory extend it. Goose owns execution and its PostgreSQL
session lock; the project's Goose Store adapter enforces checksums and forward-only policy
in this same table. There is no second version ledger, engine, automatic startup DDL or
embedded fallback. `Migrate` is explicit; `Open` refuses absent or incompatible foundation
state and never creates it.

| Table | Column | PostgreSQL type |
|---|---|---|
| coordination_schema | version_id | bigint |
| coordination_schema | checksum | text |
| coordination_schema | is_applied | boolean |
| coordination_schema | applied_at | timestamp with time zone |

All columns are non-null. `version_id` is the primary key and nonnegative. Version 0 is
the bootstrap checksum; consecutive positive versions are the exact numbered SQL file
checksums. `checksum` is lowercase SHA-256 hex. `is_applied` is constrained true; deletion,
down migrations, missing versions, altered checksums and unknown mandatory/future versions
refuse. The DDL and its version/checksum acknowledgment share one transaction. A failed
migration may leave the prior complete prefix, never its partially acknowledged successor.
The session lock serializes the complete Goose operation across independent processes and
is released on the operation's completion or connection loss. No application memory mutex
substitutes for that database lock.

The manifest recognizes annotation comments using the pinned Goose parser's detection
and normalization, including no-space, repeated-dash and case-insensitive spellings.
Only `Up`, `StatementBegin` and `StatementEnd` are allowed. Transaction disabling,
environment substitution and all other directives refuse before database connection.

The bounded mirror adapter extends this exact migration set. Production requires the
checksummed ledger floor and enforces admission/version/completeness rules. The schema
boundary test checks actual table/column/trigger shape; Open does not probe that catalog
shape or detect administrative tampering beyond the ledger.
One current foundation handle grants no claim, execution, acceptance or effect authority.

## Connection and fixture boundary

Production `Config` has explicit host, port, user, database, password, TLS and timeout.
There is no DSN/URL/token/raw provider API or credential-discovery path. Nonempty `PG*`
process configuration is refused before driver parsing; the fixed parser seed has no
empty password or service reference. Caller values overwrite the seed, including an
empty explicit password, no fallback endpoint and fixed runtime parameters. Errors omit
connection strings, credentials and PostgreSQL message/detail; cancellation stays inspectable.
TCP requires verified TLS except a literal loopback address; the local test profile uses
only an explicitly supplied Docker-container endpoint and synthetic credentials.

The library opens no server and performs no download. The internal test harness accepts
only an explicit loopback container URI with port, user, nonempty synthetic password and
`postgres` admin database. It scrubs PostgreSQL environment discovery, creates a random
owned test database, and requires its own name, database owner and marker before reset/drop.
A failed setup also retains the owned-creation receipt for bounded cleanup. Missing input
fails tests; no skip is a qualification pass. Local server binaries and ambient Docker or
Kubernetes discovery are excluded. CI supplies its own disposable PostgreSQL container.

## Implemented mirror schema

Migration `00002_mirror.sql` adds these tables in the same Goose ledger. Only the
six scope timestamp columns and two record timestamp columns are nullable; NULL is
unknown. Timestamps are exact numeric(30,0) Unix nanoseconds, not rounded SQL timestamps.

| Table | Column | PostgreSQL type |
|---|---|---|
| mirror_scopes | scope_key | bytea |
| mirror_scopes | provider | text |
| mirror_scopes | host | bytea |
| mirror_scopes | repository | bytea |
| mirror_scopes | kind | text |
| mirror_scopes | admission | bytea |
| mirror_scopes | version | bigint |
| mirror_scopes | state | text |
| mirror_scopes | reason | text |
| mirror_scopes | last_attempted | numeric |
| mirror_scopes | last_complete | numeric |
| mirror_scopes | last_inventory | numeric |
| mirror_scopes | observed_up_to | numeric |
| mirror_scopes | retained_from | numeric |
| mirror_scopes | validator_since | numeric |
| mirror_scopes | validator | bytea |
| mirror_scopes | revision | bytea |
| mirror_scopes | validator_full | boolean |
| mirror_scopes | complete_scan | boolean |
| mirror_scopes | full_inventory | boolean |
| mirror_scopes | retention_required | boolean |
| mirror_scopes | write_xid | xid8 |
| mirror_records | scope_key | bytea |
| mirror_records | record_kind | text |
| mirror_records | number | bigint |
| mirror_records | payload | bytea |
| mirror_records | in_inventory | boolean |
| mirror_records | updated_at | numeric |
| mirror_records | observed_at | numeric |

The scope key is a length-prefixed SHA-256 index over provider, host, canonical repository,
kind and admission. Full identity is stored and independently matched before disclosure;
a hash or admission string is never authorization. The record primary key is
(scope_key, record_kind, number), with a foreign key to the scope. Positive IDs, provider/kind
provenance, current-transaction scope transition and monotonic stamps are SQL checked.

A scope starts version0/never-read. Each transition advances exactly one version, using
an SQL ExpectedVersion predicate. Rows and coverage share its transaction. Triggers reject
incomplete completeness/observed/inventory advances, regressions, partial absences and
unqualified permission-hold recovery even when Go validation is bypassed. A new observed
row must be in inventory; only an existing row under complete full-inventory proof can be
marked absent. RetainedFrom is constrained NULL; no qualified history or retention action
is implemented. Row deletes refuse; TRUNCATE is not protected by these row triggers.
These guards assume a role that cannot truncate, disable triggers or alter schema; a
database owner is not an untrusted application client. Apply also refuses absence IDs
without complete full-inventory proof when an ID is missing or already absent, where a
row trigger would see no inventory transition.

Payload bytea uses private codec version1: bounded gob facts plus explicit collection
presence bits. NUL and arbitrary string bytes, BodyKnown, nil versus known-empty slices,
review provenance and nanosecond instants survive. SQL identity/inventory are excluded
from the payload and reconstructed from checked columns. A record has exactly one codec
frame; unsupported versions or corruption refuse. Configured finite record/byte budgets
bound complete snapshots and target writes before row staging. This is a private storage
format, not a public request protocol; semantic changes require migration/codec review.

Current capability checks bracket database I/O and occur immediately before commit.
A revocation discovered after an acknowledged commit returns permission failure without
rows; it cannot retroactively undo a durable commit. No SQL trigger can authenticate an
external capability. Future shared-refresh integration must bind generation validation,
Store.Apply and completion in one transaction before production coordinator use. Naked
Apply supplies CAS and durability, not stale refresh-owner fencing.

## Implemented read-only refresh coordination

Migration00003 extends the same forward-only Goose manifest and private pool. The
canonical adapter implements `refresh.Gate` and `mirror.AttemptAuthority`; it
initializes no policy/accounting and adds no execution or activation authority.
`RefreshConfig` is explicit trusted composition, with no production defaults.
Source/native, admission and caller accounting have separate persisted rows.
Missing, unknown or mismatched profiles hold; an epoch/new client does not create
an allowance. Only the disposable test fixture seeds known profiles.

| Table | Column | PostgreSQL type |
|---|---|---|
| refresh_budgets | source_id | bytea |
| refresh_budgets | profile | bytea |
| refresh_budgets | known | boolean |
| refresh_budgets | write_xid | xid8 |
| refresh_budgets | window_at | timestamp with time zone |
| refresh_budgets | window_ns | bigint |
| refresh_budgets | http_used | bigint |
| refresh_budgets | http_limit | bigint |
| refresh_budgets | inflight_limit | bigint |
| refresh_budgets | reservation_limit | bigint |
| refresh_budgets | retry_at | timestamp with time zone |
| refresh_budgets | unknown_hold | boolean |
| refresh_admissions | admission | bytea |
| refresh_admissions | source_id | bytea |
| refresh_admissions | profile | bytea |
| refresh_admissions | known | boolean |
| refresh_admissions | write_xid | xid8 |
| refresh_admissions | window_at | timestamp with time zone |
| refresh_admissions | window_ns | bigint |
| refresh_admissions | fills_used | bigint |
| refresh_admissions | fill_limit | bigint |
| refresh_admissions | http_used | bigint |
| refresh_admissions | http_limit | bigint |
| refresh_admissions | active_limit | bigint |
| refresh_admissions | waiter_limit | bigint |
| refresh_admissions | failure_limit | bigint |
| refresh_callers | admission | bytea |
| refresh_callers | caller_id | bytea |
| refresh_callers | known | boolean |
| refresh_callers | write_xid | xid8 |
| refresh_callers | window_at | timestamp with time zone |
| refresh_callers | fills_used | bigint |
| refresh_callers | fill_limit | bigint |
| refresh_callers | http_used | bigint |
| refresh_callers | http_limit | bigint |
| refresh_callers | active_limit | bigint |
| refresh_callers | waiter_limit | bigint |
| refresh_fills | fill_key | bytea |
| refresh_fills | admission | bytea |
| refresh_fills | identity | bytea |
| refresh_fills | scope_key | bytea |
| refresh_fills | generation | bytea |
| refresh_fills | owner_id | bytea |
| refresh_fills | owner_reviews | boolean |
| refresh_fills | expires_at | timestamp with time zone |
| refresh_fills | retire_at | timestamp with time zone |
| refresh_fills | state | text |
| refresh_fills | reason | text |
| refresh_fills | retry_at | timestamp with time zone |
| refresh_fills | snapshot_version | bigint |
| refresh_fills | base_retry_at | timestamp with time zone |
| refresh_fills | review_retry_at | timestamp with time zone |
| refresh_fills | attempts_used | bigint |
| refresh_fills | attempt_limit | bigint |
| refresh_fills | attempt_held | boolean |
| refresh_fills | review_only_partial | boolean |
| refresh_fills | completion_xid | xid8 |
| refresh_waiters | ticket_id | bytea |
| refresh_waiters | admission | bytea |
| refresh_waiters | fill_key | bytea |
| refresh_waiters | generation | bytea |
| refresh_waiters | caller_id | bytea |
| refresh_waiters | reviews | boolean |
| refresh_waiters | expires_at | timestamp with time zone |
| refresh_waiters | released | boolean |
| refresh_attempts | reservation_id | bytea |
| refresh_attempts | admission | bytea |
| refresh_attempts | source_id | bytea |
| refresh_attempts | fill_key | bytea |
| refresh_attempts | generation | bytea |
| refresh_attempts | caller_id | bytea |
| refresh_attempts | grant_id | bytea |
| refresh_attempts | expires_at | timestamp with time zone |
| refresh_attempts | observed | boolean |
| refresh_attempts | unknown_hold | boolean |
| refresh_attempts | http_count | bigint |
| refresh_attempts | http_known | boolean |
| refresh_attempts | decoded_bytes | bigint |
| refresh_attempts | decoded_known | boolean |
| refresh_attempts | response_complete | boolean |
| refresh_attempts | status | integer |
| refresh_attempts | retry_knowledge | text |
| refresh_attempts | retry_at | timestamp with time zone |
| refresh_attempts | observed_at | timestamp with time zone |
| refresh_attempts | source_observed_at | numeric |
| refresh_attempts | reported_retry_at | numeric |
| refresh_attempts | retry_delay_ns | bigint |
| refresh_attempts | delay_known | boolean |

Every mutation locks admission, then source budget, caller usage, fill, waiter or
individual attempt, then observation scope/records. Identity-only lookups before
those locks are rechecked. Native HTTP runs after reservation commit with no SQL
locks/pool connection retained. Different admissions sharing a source budget use
the same budget row and never acquire a second admission lock.
Shared profiles require `Limits.Refresh.FailureBackoff == 0`; the Gate is the sole
shared retry authority. Construction rejects nonzero values before I/O rather than
normalizing them. The standalone mirror planner retains its own policy contract.

Usage is debited before the actual native base-transport entry, not after a scan.
An individual opaque reservation is required; its qualified observation can close
only that slot. Duplicates/foreign reports cannot replace another reservation.
A final scan aggregate cannot synthesize missing individual observations.
Count/prefix values and knowledge are stored separately: a positive decoded prefix
with unknown total is retained and does not make qualified HTTP/retry knowledge
unknown. Decoded work is actual consumed bytes, independent of response completeness;
there is no byte quota. Unknown
reports and expired unobserved reservations hold new work without refund. A
qualified zero-entry observation (permit never returned, or lower pre-base refusal)
closes only its own slot with known zero HTTP; its budget charge remains consumed.
It is distinct from a known entered transport with no response and from unknown
commit/report. Postcommit caller refusal records this bounded bookkeeping before
returning no permit; uncertain bookkeeping retains its hold. Explicit
finite source concurrency permits already-authorized in-flight work; a published
floor applies to every later reservation. Missing/unknown timing is a hold.

`DB.Refresh` takes a required explicit trusted `refresh.Clock` for data validation
and `ApplyCompletion` evaluation, shared with the admitted native source and
coordinator. Constructor nil/typed-nil/zero samples refuse before database I/O;
after-I/O and final samples must remain valid. Native decoded-data bounds are
validated in this application clock domain, never compared to database wall time
or replaced by report/commit time. Missing/future data bounds still hold. The clock
is not serialized in the known profile and grants no budget, generation or identity.
Held/done completion takes only the scope identity/version lock and establishes its
actual baseline before the existing fenced terminal transition; no observations or
coverage are decoded/applied for that bookkeeping.

PostgreSQL time owns windows, expiry and retry eligibility. A relative retry delay
is anchored to database observation time and combined with the reported absolute
floor using the maximum; lower bounds round upwards to SQL microseconds. The
observation Store's separate nanosecond watermark codec stays exact. Window reset
requires elapsed canonical time and no unresolved individual observations. Row
limits on retained reservations and failures are finite explicit trusted inputs.
`NativeLimits.Reservations` counts every retained source reservation, including
observed entries, over the source identity's lifetime; it is not a window allowance
or just the in-flight count. At capacity new attempts hold across window rollover
and restart. Runtime does not delete this history, reset its profile or repair the
cap. Future archival/reconstruction requires an explicit accounting contract;
unexpired failures/unknown holds cannot be evicted into allowance.

The full base-fill identity is bounded version1 canonical JSON over byte-array
identity fields alongside an index hash; opaque string bytes are preserved.
All scope/source/admission/epoch/policy fields are matched before use. This is a
private storage encoding, not a request protocol. Requirements failure classes
are the two bounded base/Reviews cases, separately bound to generation and actual
queried snapshot version. The canonical `Completion.ReviewOnlyPartial` class is
persisted only after its qualified partial Apply; a weaker joined query cannot
install a review-owner-induced failure. Strong requirements and actual source
failures retain their backoff without upgrading LastComplete. Unchecked outcomes
only release their waiter debit. After generation replacement, Leave still releases
the exact old ticket but never interprets its outcome against the new generation.

`Complete` verifies its exact bound canonical Store, current generation/lease and
current caller/admission before any observation write. Its private transaction-
bound Store runs the existing typed `ApplyCompletion` seam; rows/coverage/version
and completion commit together. SQL also rejects expired/foreign completion and
completion outside the observation write transaction. Public standalone Store
Apply retains its own transaction and supplies no refresh grant. Final current
authorization/deadline checks bracket terminal I/O. A failed/uncertain acknowledgement
never authorizes replay. These checks assume a role unable to disable triggers or
alter schemas; they cannot authenticate an external capability in SQL.

## Planned domain schema

**Not implemented by the foundation:** canonical workflow instances/nodes/attempts and
artifact bindings; leases/claim generations; operator-incarnation activation records;
admission/effect-intent transactions and outbox; receipts, schedules, pending decisions
and feedback; retirement and whole-database reconstruction; domain assessment/evaluation
bindings, stale-generation restore and brake/pacing accounting. Full graph-execution/19
retains all its original Tasks and 13 Verify rows. Its full schema-match obligation is not
satisfied by checking only this implemented foundation section.

**Read-only mirror follow-up:** the bounded Store is implemented above. The bounded read-only shared refresh adapter is implemented above. Retention policy, ingestion daemon, served API,
provider credential custody and cutover remain future contributions. All forge-neutral/37 acceptance and retention gates
remain. A completed foundation is neither full R2 nor service activation.
