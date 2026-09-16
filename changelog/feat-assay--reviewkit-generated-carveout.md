### Changed
- `pr-review-desk` generated-table bounce now admits a NARROW authoring case: a hunk that ADDS
  brand-new brief rows (modifying no existing row) is admitted when each added row is honest-base —
  `Status` = `todo`, empty `Verified`/`Reviewed` — AND the rows reproduce exactly under
  `statusgen regen --readmes` (run in a throwaway worktree at the PR head with the pinned CI
  `statusgen`, never one built from the untrusted tree). This unblocks brief-authoring PRs, which
  must carry the regenerated rows or `statusgen --lint` fails on the PR head (dangling
  depends/unblocks/consumers references). The honest-base check is what blocks forgery: regen
  PRESERVES the `Status`/`Verified`/`Reviewed` cells for any row in the region — including a row the
  PR just added — so "byte-identical to regen" can never certify those columns; only requiring
  `todo`/`—`/`—` on an added row (stamps come later, from the verifier/reviewer) does. Any change to
  an existing row, and any stamped or non-`todo` row inside the markers, still bounces. Reviewers
  never hand-fix the table.
