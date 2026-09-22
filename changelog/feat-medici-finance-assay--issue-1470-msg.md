### Fixed
- `deskdispatch`'s worktree-create failure hint no longer tells the operator to hunt for a
  merged/open PR when `deskwt add` actually failed on its own origin-remote resolution
  (`cannot parse origin repo …` / `cannot parse owner/repo …`) — that class now gets its own
  hint pointing at the checkout's origin remote, and a message matching neither the
  branch-exists nor the origin-parse pattern now gets no guessed cause at all instead of the
  old unconditional "branch already existing" fallback.
