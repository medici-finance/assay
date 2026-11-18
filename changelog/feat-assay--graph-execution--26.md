### Added
- `drainloop` gains `RoleCapacity`, a per-desk approximate role-capacity estimate for bounded refill. It counts every instance and draining predecessors, ages out stale readings, treats missing data as unknown rather than zero, and never takes a cross-desk lock or lets a failed refresh block work. Per-item claims stay exact.
