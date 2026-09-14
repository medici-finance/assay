### Fixed
- GitLab `ReviewsAtHead` no longer aborts the whole review read when the project
  approval-configuration route (`GET /projects/:id/approvals`, Premium+) answers 403 instead of
  404 — the shape gitlab.com's Free tier actually returns. A 403 there now degrades head-pinning
  only (same as the documented CE/Free 404 gap), but only when the per-MR approvals read that
  follows still succeeds, so a genuinely rejected credential (which 403s that read too) still
  fails the whole read closed. Previously every `deskpost review` / `security-review` on a
  gitlab.com Free-tier project aborted outright.
- `deskpost comment` gains `--kind issue|mr`, reusing the `TargetKind` / `GetIssueTyped` /
  `PostCommentTyped` typed forge operations, so it can target a GitLab merge request or issue
  explicitly when the same number names both (GitLab numbers the two in separate sequences).
