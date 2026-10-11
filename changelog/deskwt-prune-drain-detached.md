### Fixed
- `deskwt prune` now drains a clean, unlocked worktree whose HEAD is the `origin/main` tip — including the detached-at-main shape reclaim flows leave behind — instead of holding every at-tip worktree unconditionally; one holding uncommitted or untracked work is still held.
- `deskwt prune` clears stale worktree registrations whose directory survives but whose `.git` file is gone (previously immortal when locked: `git worktree remove` refuses them and `git worktree prune` keeps them); only the registration is cleared — the directory is left on disk.
