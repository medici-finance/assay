### Added
- `deskwt prune --reap-dead-sessions` (default OFF) clears the worktrees a dead desk session
  leaves behind — the shape no earlier sweep could reach, because a session that died with
  work in flight sits on an unmerged branch that the merge gate reads as active work and
  holds forever. Since git permits one worktree per branch, every later resume of such a
  branch failed at worktree-create and the drain queue wedged on its own leftovers. The arm
  removes a worktree only when no LIVE session owns it (its lock names a session the roster
  shows is gone, or it carries no lock at all) AND removing it is provably lossless: the tree
  is clean with untracked files COUNTED, and HEAD is already reachable from its upstream or
  from `refs/remotes/origin/main`. A live session's lock still holds its worktree
  unconditionally, and anything dirty, unpushed or unverifiable is listed with the reason that
  held it rather than removed. (#1375)
- On a reap the stale local branch is deleted too, when it is equal to or behind its upstream,
  with the non-force `git branch -d` — so a later `deskwt add --branch` cuts fresh from origin
  instead of colliding with a leftover ref. git's own merged-into-upstream refusal is a second,
  independent layer over the ancestry check the sweep already made; there is still no `--force`
  anywhere in the verb. (#1375)
- `deskwt prune --dry-run --reap-dead-sessions` prints the full plan — path, session (or
  `unowned`), `REAP`/`KEEP`, and the reason — for every registered worktree, including the ones
  the identity refusals put out of reach, and changes nothing. (#1375)

### Changed
- The prune summary and audit line now carry two further counts, `dead-session-reaped` and
  `branches-deleted`, so a drained repo can be told from a stuck one at a glance. (#1375)
- `--lock-ttl` is accepted with `--reap-dead-sessions` as well as with
  `--reclaim-stale-locks`; alone it is still refused rather than silently inert. (#1375)
