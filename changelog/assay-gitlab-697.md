### Fixed
- GitLab CE/Free review desks are no longer blinded by the missing Premium approval-config
  route. `ReviewsAtHead` now treats a 404 on `GET /projects/:id/approvals` as the documented
  CE gap and degrades head-pinning only — approvals are reported unpinned (advisory) and the
  head is taken from the verdict note SHA — instead of failing the whole review read closed.
  A 403 (tier gate) or 401 stays a could-not-check for the entire read. This lets
  `deskboard actions`/`reviews` classify NEEDS-REVIEW/RE-REVIEW on CE queues again.
