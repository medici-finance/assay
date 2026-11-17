### Fixed
- `fanoutloop plan` no longer offers IMPLEMENT for a brief row whose own status is known and is not
  `todo`/`in-progress`. An `Awaiting implementer rework` row (status implemented or verified, a
  failed verification with a cited blocker) is listed under a new `AWAITING REWORK, owner worker`
  heading with the board's next act (`fix, cite <ref>`); the cited blocker is the work to start, and
  the delivered brief is not dispatched again. Any other such row on a board source the queue reads
  (implemented, verified, done, blocked) is listed under `WITHHELD BY STATUS` with what it waits on.
  A row whose status cannot be read is still offered. Next-up rows past `in-progress` were already
  dropped by the board reader with a stderr note and are not listed.
