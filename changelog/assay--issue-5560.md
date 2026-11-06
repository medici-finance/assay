### Fixed
- `cellctl scratch sweep` no longer reclaims nothing when one task is too large to walk inside the maintenance inventory limit: that task is kept and named, and the rest of the plan still runs, so ordinary reclaimable tasks beside it are reclaimed. The sweep still reports partial accounting, and the oversized task is never reclaimed without a complete walk.
