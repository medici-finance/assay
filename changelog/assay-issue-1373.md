### Fixed
- `deskdispatch` **resumes onto the change's own branch.** A dispatch that names an
  already-open change (`--pr <N>`) now cuts its worktree from that branch's own remote ref
  (refreshed first), instead of cutting a fresh branch of the same name off the mainline —
  which left the worktree at main's tip while the change's commits lived only on its remote
  branch, so a resuming agent silently started from the wrong commit. A fresh dispatch, and
  the read-only review and verifier lanes, still cut from the mainline.
- `deskwt add` **judges a branch collision against the branch's own remote counterpart**, not
  against the mainline. A branch this tool created tracks the ref it was cut FROM, so every
  real feature branch read as "unfinished work, not a leftover" and its collision was refused
  permanently — even when the local branch was identical to its own pushed counterpart. A
  branch carrying commits beyond that counterpart is still refused, now naming the counterpart
  rather than the mainline. The `@{upstream}` lookup behind that comparison is also fixed: it
  used a ref spelling git rejects, so the upstream arm never resolved at all.
