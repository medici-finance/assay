### Fixed
- `deskboard` and `deskflip` no longer fail closed under a `checks:read`-only identity
  (the reviewer App). gh's built-in `statusCheckRollup` JSON field selects a
  `checkSuite.workflowRun` sub-field — a link to the Actions run, not a check conclusion —
  that requires `actions:read`; under an identity without that scope it 403s and takes the
  whole read down with no salvageable output. `deskboard`'s bulk open-PR read (`prs` /
  `actions`) then exited 6 on the first repository alphabetically, blinding the entire
  cross-repository board, and `deskflip`'s single-PR state read refused to flip any private
  PR. Both reads are now hand-authored `gh api graphql` queries that request the status
  rollup contexts WITHOUT `checkSuite`/`workflowRun`; every conclusion these tools classify
  on (`CheckRun.status`/`conclusion`, `StatusContext.state`) is covered by `checks:read`
  alone, so neither read depends on a scope the tool's identity is not guaranteed to hold.
