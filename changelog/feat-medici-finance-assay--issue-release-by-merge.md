### Added
- Release by merge (design + staged implementation): a prepared release PR (title
  `release: vX.Y.Z` + a `RELEASE: vX.Y.Z` body marker) becomes the release cut when a
  maintainer MERGES it — the merge is the human gate and the recorded authorizer. A staged
  `release-on-merge.yml` detects the merge and creates the plain `vX.Y.Z` and umbrella
  `assay/vX.Y.Z` tags at the merge commit with a GitHub App token (a GITHUB_TOKEN-created tag
  would not trigger the build), and a staged twin of `release.yml` resolves the tag-push
  authorizer from the merged PR's `merged_by.login` and refuses to release any `v*` tag whose
  commit is not a merged release PR — closing the iso-9001/04 tag-push traceability gap.
