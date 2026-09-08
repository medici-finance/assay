### Fixed
- `deskwt add` and `role-init` now place a new worktree under the sanctioned prefix
  that is portable on the host OS: `/private/tmp/tracker-<name>` on POSIX, and
  `<repo-root>/.claude/worktrees/tracker-<name>` on Windows. Previously both always
  constructed the `/private/tmp` path, which on native Windows becomes a drive-rooted
  `\private\tmp\…` that fails the sanctioned-prefix check — so no desk worktree could be
  created and `deskboot` refused the shared checkout. Both prefixes were already in the
  allowlist; only the target selection was Unix-locked. The prefix guard is unchanged, so
  the isolation guarantee still holds on every platform (#656).
