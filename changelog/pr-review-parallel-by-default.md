### Changed
- `pr-review-desk` now reviews **in parallel by default**. Within a risk-classed
  PR the correctness and security lanes dispatch in the same turn (the board's
  `SECURITY-REVIEW-REQUIRED` row is a missed-dispatch alarm, never the trigger),
  and across PRs every actionable `(PR, lane)` fills a free slot in one dispatch
  turn up to the pool width — refill fills all free slots, not one. Dispatch keys
  are lane-suffixed (`<alias>--pr-<N>` / `<alias>--pr-<N>--security`).
