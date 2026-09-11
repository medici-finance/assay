### Changed
- `deskroster`'s two display reads (`ghViewPR`, `ghListOpenPRs`) now route through the
  enumerated `Forge` seam (`GetPullRequest`, `ListOpenChanges`) instead of shelling `gh`,
  closing two of the three residual forge-CLI call sites the `forge-gitlab/08` shell-exec ban
  still permitted. The forge-CLI ratchet ceiling drops 9 → 7.
