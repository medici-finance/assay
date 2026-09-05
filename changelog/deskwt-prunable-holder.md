### Fixed
- `deskwt add` no longer counts a `prunable` (directory-gone) worktree as a live branch holder — such stale registrations are skipped so a same-name add can reclaim the branch, while a branch that is genuinely checked out somewhere or carries unpushed commits is still refused.
