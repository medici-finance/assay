### Fixed
- `deskdispatch` storage admission no longer holds the cell Go cache lock for two full walks of `build/` and `mod/` when nothing needs cleaning: a lock-free pre-check (free space plus the last recorded size) admits a cache that is comfortably under budget and above the free-space floor, and anything it cannot read takes the unchanged locked path, so concurrent desk dispatches stop queuing behind one another.
- A cache pass that removes nothing no longer re-measures the cache a second time.
