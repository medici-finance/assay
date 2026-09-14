### Fixed
- `deskpushguard`'s pre-push foreign-commit check no longer takes minutes on a checkout with many
  remote-tracking refs. `gitcore.RefsContaining` (the in-process port of `git for-each-ref
  --contains`) walked every ref's entire history independently — ~1000 refs over ~17k commits cost
  ~108s per commit ahead of `origin/main`, enough to blow the desk preflight's 45s write-transport
  probe and keep the review desk from booting. All refs now share one memoised walk, and when the
  repository carries a commit-graph file its generation numbers cut that walk off exactly (a
  topological invariant, never a date). Measured on that checkout: the full pre-push run dropped
  from ~194s to ~1.4s, and the call itself to well under a second. The answer is unchanged and
  still checked against real git's, now including annotated-tag peeling; an unanswerable question
  (target is not a readable commit, broken object store) is reported as an error rather than an
  empty list, so the guard hears could-not-check instead of "no ref contains it".
