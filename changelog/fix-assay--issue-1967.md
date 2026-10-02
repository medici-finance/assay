### Fixed
- The publish-identity gate in `deskpr update` now judges only the commits the push adds, so a PR
  whose head already carries a commit by another trusted App can be updated. The narrowing fails
  closed: a missing, stale, diverged or oddly-spelled remote tip falls back to the whole
  `origin/<base>..HEAD` range, and `update` re-checks against the forge's live PR head before it
  pushes.
