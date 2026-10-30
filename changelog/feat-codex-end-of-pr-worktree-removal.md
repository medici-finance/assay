### Fixed
- `deskwt remove` no longer deletes a LOCKED worktree and then strands its registration (exit 6, branch still held). It retires the lock only when it is the caller's own session's, or the roster proves its session gone, and restores the lock if the delete fails; any other lock — including another live session's — is refused with the tree untouched.

### Changed
- The Codex bindings reference gains an end-of-PR step: a Codex session that cut its own worktree (`deskwt add --lock-session`) removes it with `deskwt remove` once the PR is pushed and the tree is clean, so its branch never stays held in a locked worktree that blocks a later shepherd.
