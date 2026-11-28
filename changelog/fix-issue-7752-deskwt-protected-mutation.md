### Fixed

- `deskwt prune --reap-dead-sessions` now has tests that fail if its identity refusals stop binding the reap arm: the current worktree and a worktree outside the sanctioned prefixes must survive. Tool behaviour is unchanged.
