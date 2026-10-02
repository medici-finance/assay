### Fixed
- `deskwt role-init` now provisions App transport for new and reused role worktrees, preserving the parent checkout's disabled push destination.
- Role transport refuses unsupported explicit HTTP/HTTPS service endpoints before URL or credential provisioning.
- A role transport failure after the URL writes restores the worktree's prior push destination, and no inherited credential helper answers inside the retained worktree.
