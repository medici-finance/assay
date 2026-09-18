### Added
- `cellctl new --kind scrubbed`: a host-local harness cell whose launch environment is fully
  COMPOSED (`env -i` plus an explicit allowlist) rather than inherited — nothing from the
  launching shell reaches the harness. Its own real (never symlinked) config home, scoped to
  exactly one repo, with its own harness login.
- `cellctl smoke <cell>`: a one-shot, tool-free, read-only readiness probe for a scrubbed cell —
  the harness answers `READY` or the verb names what it said instead.
- `cellctl status <cell>`: `running <session>` / `stopped` / `stale-lock <pid>` — a read, not a
  check.
- A scrubbed cell is single-occupancy, enforced by two independent layers: a private-socket tmux
  session and an atomic-`mkdir` session lock; a second `desk` while one is live is refused (exit
  4) naming the running pid and session.
- `cellctl check` gains scrubbed-specific rows: config-home real-directory + 0700 mode, every PEM
  regular/non-symlink/0600, the roster's `ASSAY_ALLOWED_REPOS` scoped to exactly one repo, and
  harness login proven under the cell's own home.
