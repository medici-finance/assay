### Fixed
- `fanoutloop plan` no longer offers IMPLEMENT for a brief row whose own README status is past
  `in-progress` (implemented, verified, done). Such rows, including every `Awaiting implementer
  rework` row, are listed under a new `ROUTED TO VERIFY` heading and are not dispatched, so a
  delivered `gate: human` brief is never re-implemented. A row whose status cannot be read is still
  offered, and durable repair obligations still dispatch as REPAIR OBLIGATION.
