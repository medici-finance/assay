### Fixed
- `writeguard` no longer refuses ordinary `gh` commands that mention `status` (`gh run view --json status,conclusion`, `gh pr checks | grep status`, `git status && gh pr view`). `gh` and `status` now refuse only alongside a token-print flag (`-t`, `-at`, `--show-token`); every `gh auth` form stays refused.
