### Added
- `deskreconcile` — a desk-side board-reconcile writer. It fetches `origin/main`
  into an isolated worktree, runs `statusgen reconcile --backfill --apply` (the
  only writer of a stream README Status cell: `todo`/`in-progress` →
  `implemented`, real merged-PR witness only), and — only when a stream README
  changed — commits ONLY those README files as ONE commit on the fixed branch
  `board/reconcile` and opens or UPDATES exactly one draft PR titled
  `chore(board): reconcile`. `--dry-run` reports the rows it would flip and writes
  nothing. This runs the scheduled-reconcile job from a verb the desk/worker App
  can run, removing the CI-workflow dependency that #1175 is blocked on (no App
  may push the workflow change). (#1339)
