# coordination-store-v1

Status: foundation implementation; full R2 domain schema remains planned.
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

The foundation contains no domain table. A later adapter extends this exact migration set
and independently validates the schema floor and its admission/version/completeness rules.
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

## Planned domain schema

**Not implemented by the foundation:** canonical workflow instances/nodes/attempts and
artifact bindings; leases/claim generations; operator-incarnation activation records;
admission/effect-intent transactions and outbox; receipts, schedules, pending decisions
and feedback; retirement and whole-database reconstruction; domain assessment/evaluation
bindings, stale-generation restore and brake/pacing accounting. Full graph-execution/19
retains all its original Tasks and 13 Verify rows. Its full schema-match obligation is not
satisfied by checking only this implemented foundation section.

**Read-only mirror follow-up:** its observations, coverage, admission epochs, retention and
transactional watermarks extend this same manifest after explicit consumer routing. No
mirror Store, refresh gate, ingestion daemon, served API, provider credential custody or
cutover is delivered by the foundation. All forge-neutral/37 acceptance and retention gates
remain. A completed foundation is neither full R2 nor service activation.
