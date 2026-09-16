### Changed
- `pr-review-desk` generated-table bounce now admits a NARROW authoring case: a hunk that ADDS
  brand-new brief rows (and modifies no existing row) is admitted when those rows reproduce exactly
  under `statusgen regen --readmes` — run in a throwaway worktree at the PR head with the pinned CI
  `statusgen`, never one built from the untrusted tree. This unblocks brief-authoring PRs, which
  must carry the regenerated rows or `statusgen --lint` fails on the PR head (dangling
  depends/unblocks/consumers references). Any change to an EXISTING row is still bounced
  unconditionally, because regen preserves the `Status`/`Verified`/`Reviewed` cells, so
  "byte-identical to regen" is not evidence a forged stamp in them was not planted. Reviewers still
  never hand-fix the table.
