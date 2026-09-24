### Changed
- `pr-review-desk`: the generated-table bounce now admits a second, narrow carve-out — a
  cross-repo brief's existing row, promoted Status-only from `todo`/`in-progress` to
  `implemented`, admitted only when `statusgen reconcile --backfill --apply --repo <delivery repo>`
  run on main reproduces it byte-identically. The delivery repo is read from the brief
  (`homed-in:`, else the stream's `repo:`, else the board repo), never from the PR, and must
  differ from the board repo: a same-repo row still bounces. Every admitted row, whether its
  witness is a `Brief:`-trailer PR or a backfill branch/body match, also needs the reviewer's
  code-existence check in the delivery repo, recorded in the verdict. Every other change inside
  the generated table still bounces.
