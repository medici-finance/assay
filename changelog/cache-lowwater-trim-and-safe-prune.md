### Added
- A managed cell Go cache is now trimmed to a low-water mark: once its size passes the high-water mark (75% of the budget, the margin the lock-free admission pre-check needs) or it is under pressure with consumers live, compiler-cache entries are removed oldest first until it is at or under `CELL_GO_CACHE_LOW_WATER_PERCENT` (default 60%), so the next launches admit without the lock instead of every build re-triggering a trim. No entry younger than `CELL_GO_CACHE_TRIM_MIN_AGE` (default `2h`, always above `1h`) is ever removed, the floor is re-checked at removal, and a trim the floor stops short is reported with its shortfall in the report's `low_water_trim` object. Default policies serialize exactly as before.
- `deskwt add --lock-session <id>` locks the new worktree for a live session as its final, verified step, rolling the worktree back if the lock cannot be taken.
- `deskwt prune --min-idle <dur>` tunes the new activity gate (below).

### Fixed
- `deskwt prune` no longer removes a worktree a session is still using: every unlocked candidate, including those `--reap-dead-sessions` would take, is held while its root directory or the files git rewrites as a session works (`HEAD`, `index`, `logs/HEAD`, `ORIG_HEAD`, `FETCH_HEAD`) were touched within `--min-idle` (default `1h`), and unreadable evidence holds the tree.
- The prune singleton is keyed on the repository's shared git directory, so sweeps started from the shared checkout and from its worktrees no longer run concurrently over the same repository.
