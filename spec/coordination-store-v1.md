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

The bounded mirror adapter extends this exact migration set and independently validates
the schema floor and its admission/version/completeness rules.
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
is implemented. Deletes refuse. These guards assume a role that cannot disable triggers
or alter schema; a database owner is not an untrusted application client.

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

## Planned domain schema

**Not implemented by the foundation:** canonical workflow instances/nodes/attempts and
artifact bindings; leases/claim generations; operator-incarnation activation records;
admission/effect-intent transactions and outbox; receipts, schedules, pending decisions
and feedback; retirement and whole-database reconstruction; domain assessment/evaluation
bindings, stale-generation restore and brake/pacing accounting. Full graph-execution/19
retains all its original Tasks and 13 Verify rows. Its full schema-match obligation is not
satisfied by checking only this implemented foundation section.

**Read-only mirror follow-up:** the bounded Store is implemented above. Shared refresh
generation/completion, admission epochs, retention policy, ingestion daemon, served API,
provider credential custody and cutover remain future contributions. All forge-neutral/37 acceptance and retention gates
remain. A completed foundation is neither full R2 nor service activation.
