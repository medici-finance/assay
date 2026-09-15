### Added
- A GitLab merge request now opens with a resolvable "merge-hold" discussion thread that
  blocks the merge button on every GitLab tier (`only_allow_merge_if_all_discussions_are_resolved`)
  until the reviewer's approve verdict releases it at the current head; a request-changes
  verdict or a new head re-arms it. `deskflip`'s reviewer-approved condition on GitLab now
  reads this thread directly and never consults the Premium approval-configuration route that
  answers 403 on GitLab Free.

### Fixed
- `deskflip`'s `mergeable` condition on GitLab no longer refuses forever on a brand-new draft
  merge request: `draft_status` and `discussions_not_resolved` are treated as non-blocking
  there, with the reviewer-approved condition immediately after doing the real gating. This
  supersedes an interim same-day fix that mapped `draft_status` to `MERGEABLE` in the shared
  GitLab merge-status mapping — that mapping is reverted to what it was, and the leniency
  moves to the one condition it belongs to.
