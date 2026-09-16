### Changed
- `pr-review-desk` generated-table bounce now admits a hunk inside a stream README's
  `statusgen:briefs` markers when it is byte-identical to `statusgen regen --readmes` on the PR's
  tree and the PR body says so — a mechanical, reproducible check. This unblocks authoring PRs,
  which must carry the regenerated rows or `statusgen --lint` fails on the PR head (dangling
  depends/unblocks/consumers references). Any hunk that does not reproduce that way is still bounced,
  and reviewers still never hand-fix the table.
