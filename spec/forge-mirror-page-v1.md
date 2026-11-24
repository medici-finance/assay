# Forge mirror page proposal

## Architecture pickup

The baseline is Assay main `4c4ac6e5` and the offline read-only runtime binding
in draft PR #7609. The forge-service-v1 contract allows 256 records and 1 MiB per
message, and its mirror-query response represents a complete population. The
selected pilot repository had about 3,569 pull requests. A bounded variant can
fit this repository without changing the existing wire or its ceilings.

This proposal affects the forge-neutral/39 wire owner, the canonical PostgreSQL
mirror store under /52, and the /56 serving and client integration. The
existing /39 contract and acceptance evidence are preserved. The selected
/56 authenticated transport still depends on the qualified /57 launch profile.
This isolated implementation prepares the new path without choosing a transport,
activating a runtime, or changing an approved product target.

## Contract

An explicitly selected mirror-page endpoint accepts a closed
`forge-mirror-page-v1` envelope. It carries an otherwise valid v1 mirror query,
a positive page size within the existing wire cap, and a nullable continuation.
Minimum-progress tokens and diagnostic links are outside the page variant.
Clients opt in to the endpoint. Both entrypoints retain their existing caller
authority and admission requirements.

A successful page contains an ascending range of record numbers from one
complete snapshot. It carries the original observation time, coverage, scoped
snapshot version, and a continuation when there are more rows. Completeness
describes observation coverage. The absence of a continuation marks the end
of the traversal. The byte ceiling can reduce a page's size. A record that
cannot fit alone returns an unavailable response.

A continuation binds the full request scope, repository, record kind, exact
snapshot version and last record number. It grants no authority. Each request
establishes current caller and admission independently, and the handler
rechecks them after encoding. A moved or stale snapshot requires an explicit
restart, without records or an automatic client retry.

The first page may use the existing read-through coordinator. Continuations
read the same scoped snapshot version from PostgreSQL with an indexed
record-number boundary. They neither refresh the source nor join a different
generation. The existing native scan and its limits continue to govern cold
inventory acquisition.

## Acceptance evidence

Use an owned disposable PostgreSQL instance for the snapshot and keyset
controls. A stable walk over more than 256 rows must return every row once.
An interleaved version change, foreign scope or revoked caller must release no
records. Exercise the byte limit with large individually valid records, and
confirm an existing v1 client still reads only the original response format.
Live pilot acceptance also requires qualification of the admitted source's cold
inventory and the actual caller launch profile.
