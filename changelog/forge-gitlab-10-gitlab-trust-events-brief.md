### Added
- **Scoped brief `forge-gitlab/10` — the GitLab trust-events read + commit author-login.** Planning-only:
  the brief scopes (does not implement) bringing the GitLab backend's `PRTrustEvents` trust read and
  `GetCommit` author/committer-login resolution to parity with GitHub, so a GitLab review desk can form
  the trust verdict `deskpost`'s review precondition chain requires and read a commit's attributed
  identity. It closes one named blocker of the deskpost-verdict wiring (#798; write ops + reviewer PAT
  auth landed in #800) and adds brief-10 to the `forge-gitlab` stream README status table and
  minimum-tier matrix.
