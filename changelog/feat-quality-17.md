### Added
- `TestRegression_<repo>_<issue>[_Desc]` regression-test naming convention
  (`docs/test-policy.md` § Regression suite) and its CI mechanism: the new
  `tools/regsuite` module, a `gate` subcommand that compares a HEAD tree
  against a BASE tree and fails on a failing regression test, a drop in the
  total `TestRegression_` count (whatever renamed or not — the comparison is
  on count, never on name set), or a module whose selector executed fewer
  top-level regression tests than it listed — including the `go test -run`
  "matched nothing, exited 0" vacuous pass docs/test-policy.md already named
  as a hazard. The `.github/workflows/regression-suite.yml` `regression-gate`
  job that wires `tools/regsuite` into CI is prepared but NOT part of this
  PR's pushed diff — see the PR body for why (a workflow-file push needs a
  credential this PR's author App does not hold) and for the ready-to-apply
  file content.
- `plugins/assay/skills/author-brief/SKILL.md` rule 14's class-guard
  Verify-row paragraph now points at `docs/test-policy.md` § Regression
  suite for the naming convention a class-guard instance test should follow.

### Changed
- `tools/desk/cmd/deskwt/roleinitgitlabcred_test.go`:
  `TestRoleInitGitLabReadsCustodyNeverGitHubMinter` renamed to
  `TestRegression_assay_1573_RoleInitGitLabReadsCustodyNeverGitHubMinter`
  (body unchanged) — the regression suite's seed member, so it is counted by
  `regression-gate` rather than deletable without a trace.
