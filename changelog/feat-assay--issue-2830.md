### Fixed
- `deskwt prune` no longer removes or reaps a worktree an admitted verifier run lives in. A home holding a verifier admission record is held for 24h after admission, or until `deskwt remove` releases it; an unreadable record holds that worktree, and a record that names no worktree holds the whole sweep.
