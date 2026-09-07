### Fixed
- `deskflip`'s `checks-green` condition now keys an ABSENT check rollup on the base branch's
  ACTUAL required status checks (branch protection's `required_status_checks`), not the coarse
  roster ci-tag. A change on a repo that runs CI but requires no check to merge — checks that
  never fire on App-authored PRs, or a branch with no required checks — is no longer refused
  forever: an empty required set makes an absent rollup GREEN (nothing gates the merge on a
  check), a non-empty set keeps it could-not-verify (the required checks have not reported),
  and a required-set that cannot be read stays could-not-check and REFUSES (fail closed). The
  non-empty-rollup behaviour is unchanged.

### Added
- `RequiredStatusChecks(repo, branch)` joins the frozen `Forge` seam — the twentieth operation
  — landing with its one consuming call site (`deskflip`'s checks-green condition) per the
  freeze rule, with a contract case per backend. GitHub reads
  `branches/<branch>/protection/required_status_checks` (404 = nothing required = empty set,
  every other non-2xx = could-not-check), unioning the legacy `contexts` and the newer
  `checks[].context`; GitLab reads the all-tier `only_allow_merge_if_pipeline_succeeds`
  pipeline-gating setting. `PullRequest` grows `BaseRef` (the target branch) to feed it.
