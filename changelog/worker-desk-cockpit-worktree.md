### Added
- `worker-desk`: a cockpit-aware, PATH-detected variant of the per-item worktree-create step for
  fanout dispatch — `supacode repo worktree-new` / `herdr worktree create` / plain
  `git worktree add` (fallback). Additive and never required: selection is by command presence on
  PATH, only the worktree-create step changes, and the plain `git worktree add` path stays the
  default with no cockpit installed.
