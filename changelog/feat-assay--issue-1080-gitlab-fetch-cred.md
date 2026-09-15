### Fixed
- `cellctl`'s boot fetch on a `gitlab` cell now runs with `GIT_TERMINAL_PROMPT=0` and an inline
  credential helper reading `DESKD_GITLAB_TOKEN_FILE`, instead of hanging on an interactive
  `Username for 'https://gitlab.com':` prompt (or failing with `fatal: could not read Username`)
  — the `github` arm is unchanged, since the operator's `gh` credential helper already answers
  for it.
- `cellctl check` on the `gitlab` arm gains a row proving `git ls-remote` succeeds against
  `CELL_REPO` with prompts disabled, using the same helper, so a broken or missing token is
  caught at check time rather than as a boot hang.
