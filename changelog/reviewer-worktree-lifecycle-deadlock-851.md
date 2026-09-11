### Fixed
- `deskwt remove` no longer refuses a detached-HEAD worktree whose commit is provably present on
  the remote (reachable from a remote-tracking ref). A review kit checks the PR/MR head out as a
  detached HEAD, so a reviewer worktree is by construction detached and never an ancestor of
  `origin/main`; the old blanket "detached ⇒ refuse" left it unreclaimable, and the next dispatch on
  the same lane key then failed worktree-create ("target already exists") — the review lane wedged
  permanently after one review (#851). "No upstream" and "not on the remote" are now treated as
  different questions: a detached HEAD whose commit is on NO remote is still refused (the
  never-remove-unpushed-work invariant is unchanged), while one whose commit is proven pushed is
  reclaimable, which unwedges the lane. `deskwt prune` continues to reclaim a reviewer worktree once
  its PR merges via its existing origin/main ancestor gate, and to leave an open PR's (unmerged)
  reviewer worktree in place.
- `deskdispatch`'s `worktree-create` failure hint is now selected by kit. The brief-lane hint
  ("the brief's `feat/<id>` branch already exists — look for a merged/open PR") is meaningless on the
  review lane, which has no brief and no feat branch; the review lane now gets a hint that points at
  the reviewer-worktree lifecycle (reclaim the earlier reviewer worktree with `deskwt remove` before
  re-dispatching) instead of a phantom PR (#851).
