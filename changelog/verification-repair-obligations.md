### Added
- Durable **repair obligations** (`repair-obligation-v1`): a failed or blocked verifier run now
  records a structured, idempotent obligation — keyed by (repo, brief, source receipt, failing
  rows) — that survives the reporting agent. The immutable key means a duplicate delivery, a lost
  acknowledgement, or a process restart all reconcile to the SAME obligation rather than
  manufacturing a second, and an expired worker lease returns it to the queue for a replacement
  worker without duplicating the work. Obligation states (needs-assignment, repairing,
  awaiting-review/merge/reverification, waiting-external, resolved) are scheduling state, never
  acceptance.
- `worker-desk` (fanoutloop) reads outstanding repair obligations across the configured roots as
  a rework source: an actionable, still-unresolved obligation is dispatched like any other rework
  item, carrying its reproduction and expected behaviour so the worker starts from the failure —
  and, when the original deliverable PR has merged, the repair opens a FRESH follow-up branch in
  the correct deliverable repo instead of resuming immutable history.

### Changed
- A repair obligation resolves ONLY on a valid INDEPENDENT verification at the repaired revision.
  A merge wakes reverification but does not close the obligation; a same-actor pass (the worker
  that produced the repair) and a wrong-revision pass are both refused. Worker completion, issue
  closure and merge alone can never resolve an implementation obligation.
