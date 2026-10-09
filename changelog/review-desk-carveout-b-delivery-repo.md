### Fixed
- `pr-review-desk` skill, carve-out B: the delivery-repo bullet now states where a brief's
  `deliverable_repo:` alias sits in the reading order (after `homed-in:`, before the stream
  README's `repo:`), resolved through `docs/streams/graph-repos.yaml` at the same fetched ref.
  An alias that does not resolve, or one that disagrees with `homed-in:`, is could-not-check
  and bounces the row; it never falls through to the board repo. Same wording as carve-out C
  (#2430).
