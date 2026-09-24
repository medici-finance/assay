### Changed
- `pr-review-desk`: the generated-table bounce now admits a second, narrow carve-out — an
  existing row's Status-only `todo`/`in-progress` → `implemented` promotion, admitted only when
  `statusgen reconcile --backfill --apply --repo <delivery repo>` run on main reproduces it
  byte-identically. The delivery repo is read from the brief (`homed-in:`, else the stream's
  `repo:`, else the board repo), never from the PR. A row witnessed by a `Brief:`-trailer PR in
  the board repo itself, whose body was not edited after the merge, is admitted on that; every
  other row (a backfill branch/body match, a cross-repo trailer, a post-merge body edit) also
  needs the reviewer's code-existence check in the delivery repo, recorded in the verdict. Every
  other change inside the generated table still bounces.
