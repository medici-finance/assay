### Added

- GitLab backend now serves `PRTrustEvents`/`IssueTrustEvents` and resolves commit author/committer logins, so the `deskpost` trust gate can form a real verdict on GitLab instead of stopping at a could-not-check stub (forge-gitlab/10, #887 item 1).
