### Changed
- `pr-review-desk`: the generated-table bounce's carve-out B now admits a same-repo brief's
  existing row as well as a cross-repo one, on the same bar. The row is promoted Status-only from
  `todo`/`in-progress` to `implemented` with its `Verified`/`Reviewed` stamps untouched, and only
  when `statusgen reconcile --backfill --apply` run on main (with the board repo as `--repo` for a
  same-repo brief) reproduces it byte-identically. The reviewer still checks, on every row, that
  the brief's named files and symbols exist on the delivery repo's main. A row with a trailer-only
  witness is admitted only when that check passes, a hunk that touches any row carve-out B does not
  admit bounces whole, and statusgen-source PRs stay outside it. Previously a same-repo row always
  bounced, and the only compliant path was to carry the whole reconcile output, which also
  promotes backfill-only rows whose work has not landed.
