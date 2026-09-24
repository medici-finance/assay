### Changed
- `pr-review-desk`: the generated-table bounce now admits a second, narrow carve-out — an
  existing row's Status-only `todo`/`in-progress` → `implemented` promotion, admitted only when
  `statusgen reconcile --backfill --apply --repo <delivery repo>` run on main reproduces it
  byte-identically. A row witnessed by a `Brief:`-trailer PR is admitted on that; a row witnessed
  only by the backfill branch/body match also needs the reviewer's code-existence check in the
  delivery repo, recorded in the verdict. Every other change inside the generated table still
  bounces.
