### Fixed
- Dead-claim decay's GitHub reader no longer lets a fork's branch name decay a live claim. The
  pass drops branches whose pull request has merged or closed, and it keyed that on the bare
  `headRefName` of every PR `gh pr list` returned — forks included. A fork's head branch is named
  inside the fork and names nothing in the tracked repository, so a throwaway fork PR named after
  a live dispatch branch, then closed, decayed that live claim and a second worker was dispatched
  onto work already in flight. The reader now asks `gh` for `isCrossRepository`, `headRepository`
  and `headRepositoryOwner` and admits a PR as a decay candidate only when its head repository IS
  the tracked repository, with the same fail direction the GitLab arm took in #1135: a PR whose
  head repository cannot be read is not read as same-repo, and each such skip is counted and
  reported as a `could-not-check` line. Under-decay that says so, never over-decay (#1147).
