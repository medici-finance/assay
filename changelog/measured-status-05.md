### Changed
- Bookkeeping only: the same-identity attribution hard-reject (a brief whose
  authoring commit and every commit that touched its Evidence section share one
  git identity now fails the committer-identity cross-check as a hard PROBLEM
  instead of a NOTICE) is already announced under `CHANGELOG.md`'s `v1.0.24`
  entry for `statusgen`'s git-committer-identity cross-check. This fragment adds
  no new behavior; it exists only because this PR's own `Brief:` trailer is the
  board's delivery witness for that landed change.
