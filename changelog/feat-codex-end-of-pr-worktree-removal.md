### Fixed
- `deskwt remove` no longer deletes a LOCKED worktree and then strands its registration (exit 6, branch still held). It retires the lock only when it is the caller's own session's and the tree holds no untracked or uncommitted files, and restores the lock if the delete fails; any other lock — another session's, whatever its roster beacon says, one naming no session, or git's initializing lock — is refused with exit 5 and the tree untouched. A dead session's clean, pushed tree is reaped by `deskwt prune --reap-dead-sessions` once it has been idle past `--min-idle` (1h by default).

### Changed
- The Codex bindings reference gains an end-of-PR step: a Codex session that cut its own worktree (`deskwt add --lock-session`) removes it with `deskwt remove` once the PR is pushed and the tree is clean, so its branch never stays held in a locked worktree that blocks a later shepherd.
