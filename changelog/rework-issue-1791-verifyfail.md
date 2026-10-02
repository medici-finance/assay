### Fixed
- `deskpost` no longer carries the retired per-item `+1` reaction gate's read path: the orphaned reaction-read method on its forge-backed adapter is gone, the adapter's interface comment now names only the live-visibility read the `:public` write gate consumes, and a structural guard test fails if any reaction/award read reappears anywhere in `deskpost`.
