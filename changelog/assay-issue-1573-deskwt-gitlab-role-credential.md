### Fixed
- `deskwt role-init` now picks the role's credential by the forge that serves the repo: a GitLab-served repo reads the role's provisioned `gitlab-<role>.token` custody file instead of calling the GitHub App minter (which failed asking for `DESK_APP_ID`). A missing custody file is refused, naming the file, and never falls back to the GitHub minter. GitHub repos behave as before.
