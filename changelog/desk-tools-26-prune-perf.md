### Changed
- `deskwt prune` — the boot-step sweep every desk window runs — no longer asks one question
  of one history once per candidate. It walks `origin/main` **once per sweep** into an
  ancestor-hash set (the per-candidate merge test becomes a HEAD resolve plus a map lookup,
  replacing go-git's unmemoized ancestor walk, which ran to exhaustion for the 19-in-20
  candidates that are genuinely unmerged); runs the merge gate **before** the full-worktree
  `Status()` it used to run first, so that walk happens only for candidates that can still
  be removed; shares **one object cache** across the sweep instead of building a fresh one
  on each of the 3–4 repository opens per worktree; and drops the commit COUNT from the
  "unpushed" skip string, which cost two full history walks per held worktree to render a
  number nothing parses. Measured on a synthetic repository with a 5,000-commit mainline:
  the per-candidate ancestry work alone was ~372 ms/candidate before; a whole sweep over
  600 worktrees is now ~2.8 s, with `origin/main` walked exactly once.
- `deskwt prune` removals are **batched**: one `git worktree prune` and one
  `git worktree list --porcelain` after the loop, instead of both per removal. The positive
  deregistration check is kept, not dropped — every removed path is verified against that
  single listing, and one that is still registered is reported by name and not counted as
  removed.

### Added
- A **prune singleton**, so N desk windows booting together run ONE sweep. A non-blocking
  advisory lock (released by the kernel when its holder exits, so there is nothing to time
  out) plus a `--singleton-ttl` recency debounce, default 10m; `--singleton-ttl 0` /
  `--no-singleton` disable the debounce only. The lock fails closed and the TTL fails open:
  a missing, truncated, unparseable or future-dated stamp means SWEEP, so no leftover stamp
  can wedge prune. The `--interval` supervisor takes and releases it per tick, never for its
  lifetime.
- `gitcore.OpenWith(dir, cache)` and `gitcore.NewObjectCache()` — open a repository through
  a caller-supplied object cache, so a pass over many worktrees of one repository decodes
  each object once. `gitcore.Open` is unchanged for every existing caller.

### Fixed
- `deskwt prune --dry-run` is now genuinely read-only. It ran `git worktree prune` — and,
  with `--reclaim-stale-locks`, unlocked worktrees — before it ever reached the dry-run
  check. It now uses git's own `--dry-run` for the bookkeeping count, reports the locks it
  would retire without retiring them, and writes no singleton stamp.
