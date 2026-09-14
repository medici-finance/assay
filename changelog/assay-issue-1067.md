### Fixed
- `deskboard actions` no longer fails the WHOLE sweep when one change carries a review verdict
  the forge could not pin to a commit. A single such change exited the command 6 with an empty
  stdout and the message `compare needs both base and head`, so a review desk lost sight of its
  entire queue — not just the affected row.
- The cause was a seam between two correct halves. The review reduction deliberately folds "the
  head advanced past the review" and "one of the two shas was never established" into a single
  not-at-head answer, so an unread sha can never be mistaken for an at-head one. The
  benign-merge (`MERGE-CURR`) arm then read that one answer as if it always meant the first, and
  asked for a diff between the reviewed sha and the head — which, in the second case, has no
  endpoints to span.
- An unestablished sha is now treated as what it is: a could-not-check that degrades ONE ROW to
  `RE-REVIEW`, the same safe side a truncated diff already degrades to, and the sweep carries on.
  "The change's own files are unchanged since the last review" is a claim nobody can make
  without both endpoints, so the benign classification is never the fallback.
- The row's diagnostic now names WHICH sha was missing and on which change, instead of a bare
  `compare needs both base and head` that identified neither.
- This is a fix to the CONSUMER, not to any forge backend. On GitLab an approval carries no
  commit sha and — unless the project resets approvals on push — survives a push, so the GitLab
  backend reports no sha rather than stamping the current head; doing otherwise would
  manufacture exactly the at-head evidence the ready-flip gate exists to require. That reading
  is correct and is unchanged. GitLab verdicts only began reaching this code path once a desk
  role's expected login started resolving per-forge, which is why the arm had never been
  exercised there before.
- The benign keep-current classification still runs, and is still reachable, wherever both shas
  ARE established — pinned by a test alongside the two degrade cases, so a future change cannot
  quietly retire it in the name of never comparing.
