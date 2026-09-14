### Added
- desk-tools brief 26 is authored, from the measurement in #1037: `deskwt prune` — a boot step
  every desk window runs — spent 8–12 minutes at ~100 % CPU on a checkout with ~657 registered
  worktrees because it asks one question of one history once per candidate and shares nothing
  between the answers. The brief hoists a single `origin/main` walk into a per-sweep
  ancestor-hash set, moves the merge gate ahead of the full-worktree `Status()` it currently
  runs first, shares one object cache across the sweep, drops a commit count that was walked
  twice per candidate to render a skip string nothing parses, batches the per-removal
  `git worktree prune` + full worktree listing into one pass, makes `--dry-run` genuinely
  read-only, and adds a prune singleton whose lock fails closed while its TTL debounce fails
  open — so N windows booting together run one sweep, and no leftover stamp can wedge the next
  one. Every removal gate is preserved: the brief changes the order and the sharing of the
  work, never which worktrees are eligible for removal.
