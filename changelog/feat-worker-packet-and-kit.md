### Added
- `deskdispatch` writes a packet for worker dispatches too (#2439): one owner-only read-ahead
  file named on the assignment's `Packet:` line. An implementing run gets the run's facts, the
  brief or issue, the board status of the briefs it depends on, the repository's instruction
  files and build entry points, and the files the brief names; a run dispatched with `--pr`
  onto an open change gets the change, how it stands against its base, check states, every
  review with the findings it records, the newest comments and the diff. Caps are tighter than
  the review packet's (32 KiB an item, 192 KiB in all) because a worker run re-reads the packet
  on every later request. A packet that cannot be built never fails the dispatch.
- A worker packet shows every value it did not write — a title, a login, a branch, a label, a
  check or file name, a finding's id and state, an error's text — inside a code span the value
  cannot end, and quotes longer text between boundary lines. It reads a finding record only
  from a review the forge attributes to the reviewer identity: a review by any other account
  is listed under its own heading as not the reviewer's, and when that identity cannot be
  resolved no review is listed as the reviewer's, no record is read and no body is quoted.
- In a worker packet for an open change, a finding record in a review at an earlier commit is
  judged against that review's commit: `resolved` with no evidence commit of its own is not a
  resolution at the current head, and the line says so. Comments are quoted newest first, so
  the packet's cap leaves out the oldest; each comment, and each review by another account,
  says whether its author is the reviewer identity, on the trusted list, not on it, or not
  checked because no list is configured; a comment by an account that is neither is quoted
  only up to 16 KiB (#2439).
- Both worker kits gain three clauses about how a run gathers and waits — batch independent
  reads into one request, read the packet first when the assignment names one, and wait on a
  check or a review in one bounded command — none of which changes what a worker must do or
  wait for.

### Changed
- A worker dispatch is handed what its kind of run needs (#2439). The kind is derived from the
  dispatch: `--pr` onto an open change is a run that works that change, every other worker
  dispatch implements a brief or an issue. The assignment's action half follows the kind (a
  run on an open change is told to push to it and never to open a second one), kit text
  marked for one kind is not quoted to the other, and the objective kit's own copy of the
  common clauses is no longer quoted a second time. Unmarked kit text — nearly all of it —
  still reaches both kinds.
