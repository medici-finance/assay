# Operator incarnation v1

Status: schema/test preparation for graph-execution/16. No live authority source,
key provisioning, credential cutover, Kubernetes resource or runtime wiring is
installed. A passing record comparison is not effect authorization.

## Record fields

The ordered field inventory below is checked against the exported Go type
`drainloop.OperatorIncarnation`: field order, Go field name and JSON wire name.
JSON names are stable wire names, and the `Schema` line states the exact value.

<!-- incarnation-fields:begin -->
- `Schema` — `schema`; exactly `operator-incarnation/v1`.
- `CellID` — `cell_id`; the affected Cell, never inferred from a database connection.
- `ID` — `id`; a fresh random identity selected by the trusted operator.
- `KeyRef` — `key_ref`; an authorized reference to the matching signing key, never its secret.
<!-- incarnation-fields:end -->

The schema does not specify a random-ID format, secret backend or signature
algorithm. Those are qualified operator-profile choices, not fields supplied by a
worker. Empty, unreadable or unsupported current state cannot establish a match.

## Trust and rotation

The trusted operator holds current incarnation and signing-key custody in
protected control state outside PostgreSQL and outside a Git configuration
revision. A runtime activation record contains only the selected incarnation ID,
key reference, paused state and an authorized stop-confirmation receipt. Restoring
that record never makes it current.

Rotate the identity AND signing key on complete reconstruction, point-in-time
rollback and ownership transfer. Ordinary failover with proven intact current
state follows the existing lease protocol; automatic multi-host recovery is a
separate qualification. Loss of both operator and database state requires fresh
rotation and complete teardown/credential reconciliation.

While recovering, pause mutation admission, stop the entire affected execution
and publisher group, invalidate old sockets/capabilities, fence/revoke independent
legacy credentials or wait for expiry, reconcile outstanding external effects,
then record operator confirmation before reopening. A recovered SQL attempt list
is not proof that the group stopped. Unknown stop or external outcomes hold.

## Consumer read contract

`IncarnationSource.CurrentIncarnation(context.Context, cellID)` returns the
protected current record or an error. Composition supplies this source from
trusted operator state; no source is installed by this preparation. Implementations
must not echo a request or read current authority from restored SQL/model text.
Errors or missing current state hold affected admission.

`MatchCellFence(current, activation, owned, presented)` compares passive records:
supported current schema, cell/incarnation identity, activation/key binding,
retained pause/stop confirmation and exact owner/generation/backend tuple.
Its successful result is consistency only. Signature authenticity, caller/role,
scope, expiry, policy, authoritative ownership freshness, atomic reservation,
durability and provider restrictions remain independent mandatory checks. There is
no check-then-apply concurrency guarantee here.

Every future effect boundary must enforce the current tuple independently of the
scheduler; an earlier upper-layer check is insufficient. Stale evidence retains
its original incarnation as provenance only. Consumers GE/19 (activation store),
GE/26 (launch admission), GE/14 and GE/25 (effect consumers) conform to this public
contract without importing drainloop. This preparation changes no consumer behavior.

## Qualification and limits

`TestCellFenceBoundary` calls a separately invoked fake boundary directly. A
restored old owner and generation are refused when protected current incarnation
differs, and the valid current tuple reaches exactly one fake effect. Missing
source, incarnation, activation, stop confirmation, pause, stale generation,
wrong cell/owner/backend and key mismatch are negative controls. Each guard of
`MatchCellFence` also has a one-field case that keeps every other record
consistent: unsupported current schema; empty current cell, ID or key reference;
an owned cell that differs from the current one while the presented tuple equals
the owned one; and empty owned owner, generation or backend.

`TestIncarnationContractFields` compares the field inventory with the exported
type in both directions, Go names and wire names together. The committed
incarnation mutations in `drainloop/testdata/effect_mutations.py`, one per guard
plus wire-name drift, must fail named runtime assertions.

These are implementation tests of a pure record contract. They do not satisfy the
whole GE/16 Verify table, prove real provider fencing or qualify PostgreSQL rollback,
host control-state custody, Kubernetes containment, signatures or live credentials.
All existing Verify rows, independent Evidence and human activation gates remain.
