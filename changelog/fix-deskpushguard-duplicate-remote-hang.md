### Fixed
- `deskpushguard`'s register-id and foreign-commit pre-push checks no longer peg one CPU
  core indefinitely on a checkout with several remotes pointing at the same upstream repo.
  Both checks called `gitcore.Repo.IsAncestor` once per candidate remote branch (register-id
  check: once per push that touches a new register entry, regardless of the push's own
  size; foreign-commit check: once per commit ahead of `origin/main`) — that call resolves
  to go-git's native, unmemoized `Commit.IsAncestor`, which re-walks `origin/main`'s entire
  history from scratch on every single invocation. A checkout with N literal-duplicate
  remotes for the same repo multiplies the candidate-branch count by N directly, and a
  large main-catch-up merge multiplies the commit count on the foreign-commit side —
  together this could run for minutes without deciding. Both checks now walk
  `origin/main`'s ancestry exactly once per push and answer every subsequent
  "already merged?" question with an O(1) set-membership lookup instead. Measured on a
  synthetic fixture (800-commit `origin/main`, 20 never-merged sibling branches visible
  under 5 remote names): 5.1s+ before the fix, 129ms after.
