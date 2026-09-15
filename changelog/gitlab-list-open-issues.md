### Fixed

- The GitLab forge backend now serves the bulk open-issue read, so `issueboard` and
  `deskboard queue` work on a GitLab-backed project instead of failing closed with
  could-not-check. The read was withheld on the ground that its summary is only ever
  consumed paired with the issue trust-events read — but that read has since been served
  on GitLab, so the pairing the refusal protected is exactly what was already available,
  and withholding the list was all that kept the issue lane blind. A GitLab adopter no
  longer has a working merge-request board next to a permanently blind issue lane.
- The open-issue walk follows GitLab's page-continuation header to exhaustion and
  **refuses** rather than truncating if a project is still paginating at the page ceiling.
  The issue summary carries no truncation field and the issue lane reads an issue's absence
  from the list as *closed*, so a silently short read would have retired tracking rows for
  issues that are still open.
