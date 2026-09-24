### Fixed
- `deskdispatch`'s phantom check now tells the operator a MERGED representing PR already
  delivered the brief and points any follow-up fix at a fresh `Issue:` claim key
  (`<repo>--issue-<N>`) instead of reusing stale "resume the PR" wording that has nothing
  to resume.
