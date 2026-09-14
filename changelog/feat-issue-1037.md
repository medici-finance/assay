### Fixed
- `deskwt prune` no longer runs an independent full-history walk of `origin/main` per candidate
  worktree. `mergedToOriginMain`'s `IsAncestor` and `unmergedReason`'s `AheadCount` each walked
  the whole remote-tracking history from scratch for every worktree the sweep looked at — on a
  checkout with N registered worktrees that was N walks of the same history instead of one, and
  on a large house checkout (~657 worktrees, ~5,779 commits on `origin/main`) it made `deskwt
  prune` cost 8-12 minutes per sweep, with five desk windows booting together running it five
  times over. The sweep now walks `origin/main`'s history exactly once into a shared ancestor
  set; each worktree's merge/fresh-tip position is then answered by a single
  `Resolve("HEAD")` plus an O(1) lookup. The tracked-clean (`git status`-equivalent) check also
  now runs *after* this cheaper merge gate, so it is only ever paid by candidates that are
  otherwise removable, not by every worktree the sweep looks at. A synthetic 60-worktree /
  1,000-commit-history fixture went from 8.3s to 0.29s under the fix (~29x); functional behaviour
  (which worktrees are removed, held, or reported unmerged/unpushed/fresh) is unchanged.
