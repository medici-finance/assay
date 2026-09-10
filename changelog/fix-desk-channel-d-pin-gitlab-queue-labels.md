### Fixed
- **`deskboard`'s drift banner now recognises the `desk-tools-source <40-hex-commit> channel-D`
  pin shape (#795 §3).** A channel-D adopter writes the source commit in field 2 with a literal
  `channel-D` marker in field 3, but the reader only accepted the commit in field 3
  (`desk-tools-source <tag> <40-hex-commit>`). The commit went unrecognised, so a correctly
  pinned install reported `STALE-UNKNOWN … no readable desk-tools pin` and `reviewloop`'s idle
  gate sat at could-not-check. The reader now takes the 40-hex commit from whichever column
  holds it (field 3 preferred, else field 2); a line with a commit in neither column still falls
  through to the in-tree ref / could-not-check, so real drift detection is unchanged.

### Added
- **`deskdispatch` applies the review-lane queue label `authorization-needed` when a reviewer is
  dispatched onto a change, forge-neutrally (#795 §4).** A new non-fatal `queue-label` step runs
  on a `--kit review` dispatch with `--pr` known and applies `authorization-needed` through the
  resolved forge's idempotent label ensure+apply, under the reviewer role's own credential (a
  GitHub App token or a GitLab PAT) — so a GitLab merge request now carries the same review-queue
  signal a GitHub pull request does, instead of an empty label set. A label the forge will not
  accept is a loud warning and the dispatch still stands (the label is a legibility aid, not a
  correctness gate), mirroring `deskflip`'s `approval-needed` swap. Both queue labels are now
  covered by GitLab forge golden tests for idempotent create-and-apply.
