### Fixed
- Stamp each GitLab desk role's worktree with its own commit address instead of the first `ASSAY_GITLAB_SESSION_EMAILS` entry for every role. The new roster key `ASSAY_GITLAB_ROLE_EMAILS` (`role=email`) binds each role to its address; without it, a session list that does not name one address unambiguously now refuses (exit 5) instead of stamping one account on every role.
