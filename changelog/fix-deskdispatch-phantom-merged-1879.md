### Fixed
- `deskdispatch` now RELEASES the durable claim when the worktree-create step fails, instead of
  leaving it orphaned. An aborted `deskwt add` used to hold the claim it had just placed, wedging
  every later re-dispatch of that item behind a claim nobody was acting on. The failure report is
  reworded too: a worktree-create failure on a fresh dispatch is most often the brief's branch
  already existing (the brief is already delivered or in progress — a merged/open PR to look for),
  not the transient "fix the tree and re-run" fault the old message implied.

### Changed
- `deskdispatch` session-scopes the worktree DIR name it derives (`<item>-<session>`, from
  `$DESK_SESSION` / `$CLAUDE_SESSION_ID`, mirroring `deskwt role-init`'s `tracker-<prefix>-<sess>`),
  so a foreign session's leftover canonical dir (`/private/tmp/tracker-<item>`) can no longer
  dead-end an otherwise-valid dispatch with `deskwt add … target already exists`. The branch and
  claim key stay deterministic — they are the deliverable's cross-session identity; only the local
  scratch dir gains the suffix. With no resolvable session the name falls back to the bare
  item-derived form.
