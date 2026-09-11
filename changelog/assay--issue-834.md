### Changed
- `deskroster`'s two display reads (`ghViewPR`, `ghListOpenPRs`) now route through the
  enumerated `Forge` seam (`GetPullRequest`, `ListOpenChanges`) instead of shelling `gh`,
  closing two of the three residual forge-CLI call sites the shell-exec ban still permitted.
  The forge-CLI ratchet ceiling drops 9 → 7.
- The remaining `repohardenguard` `gh api` site is recorded as **open work**, not a standing
  keep-as-CLI exception: its `forgeban` permit row is permitted only until the guard-read-custody
  brief lands, and the ban closes to zero with that brief.
- The `askassay` silent-cap register retires `deskroster`'s `--limit 50` row — the read is now
  bounded by the forge backend's declared page cap rather than a deskroster-local literal.
