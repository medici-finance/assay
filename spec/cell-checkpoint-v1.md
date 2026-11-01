# Cell checkpoint v1

Status: passive schema/test preparation for graph-execution/16, not an active
reservation store or policy engine. The six-method Loop interface is unchanged.

`drainloop.CellCheckpoint` serializes `schema=cell-checkpoint/v1`, ownership,
activation, budget, cancellation intent, durable stop, override references and
outstanding effect references. A restore must preserve them before any admission.
Missing history stays unknown; the store cannot choose the protected operator
incarnation. See [operator incarnation](operator-incarnation-v1.md).

Readers decode through `drainloop.DecodeCellCheckpoint`. It refuses a record with
any field absent at any depth, an unknown field, trailing data or a schema other
than `cell-checkpoint/v1`. A boolean hold flag or a count cannot express absence,
so an absent `paused`, `stopped` or `cancel_requested` is refused rather than read
as not held. A nullable count written as explicit `null` stays unknown. A plain
JSON decode of this record is not a conforming reader.

`CellBudget` contains separate balances, outstanding reservations and attempt
usage. Each balance binds account/provider/unit, policy reference, exact window,
limit, operator headroom and nullable consumed amount. Distinct units and windows
are never added together or silently converted. Nil consumed usage is unknown.

Reservations bind original work, attempt, request, balance and amount. An unknown
reservation remains charged until authoritative reconciliation. Future adapters
must reserve atomically against one durable balance across processes, including
consumed usage, all outstanding reservations and headroom. Implementing a Go
interface or persisting JSON alone cannot prove this property. No implementation
of that transaction, release, override or handback is added here.

Attempt usage binds original work/parent attempt/request, phase/outcome,
account/provider, actual model, pricing identity, exact timestamps and reported
input/cache-write/cache-read/output counts. Nullable counts preserve unknown
telemetry separately from measured zero. Preparation, summary, failed attempts,
fallback and independent verification charge the original work. SourceRef carries
an authorized artifact reference with WI-1 trust disposition; no secret, credential
token, authentication header or arbitrary raw provider payload belongs in this
record. Numeric usage-token counts are retained. A reference's syntax alone proves
neither authorization nor trust; the adopting ingress owns those checks.

ControlBoundary records whether a runner can stop requests or only launches.
Launch control is not a hard per-request cost limit. This schema specifies no
fixed price, provider SDK, calibration, automatic fallback or commercial default.

Cumulative attempts, cost/time/resource limits and write ceilings remain brakes:
unknown/unverifiable usage holds or uses a separately declared conservative
fallback, never a fresh allowance. Pacing is a separate best-effort owner:
effect-class estimates re-derive from durable intents/receipts before every
admission; read-class seeds from retained shared state. One recorded unseeded reset
per window/scope/incarnation is allowed; a second or unreadable reset history
exhausts the window. No pacing implementation is supplied here.

The independently authored synthetic fixture
`drainloop/testdata/cell-checkpoint.json` and `TestCellCheckpointRoundTrip`
exercise the exported types through serialization/restoration and a fake boundary.
They retain failed spend, unknown telemetry/reservation, distinct units/windows,
stop/cancellation, override and unresolved effects, and assert the fixture's
schema value. `TestCheckpointAbsentFieldsHeld` removes each field of the fixture
in turn, nested ones included, and requires the decoder to refuse every result. This is schema preservation,
not a durable restart/concurrency proof or accepted GE/16 Verify evidence.

Remaining GE/16 work: transactional reservation and intent coupling at the existing
authority; cohort credential cutover; independently enforcing real effect adapters;
crash/concurrent-owner/unknown-handback tests; production restore and pacing/brake
failure cases; consumer wiring and full Verify rows 1–11 with independent evidence.
GE/19 retains real PostgreSQL rollback qualification, GE/26 launch enforcement.
R3 exclusive repository assignment remains outside this amendment.
