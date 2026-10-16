### Fixed
- `cellctl` desk launch no longer takes the cell Go cache lock or walks the cache when the cache is comfortably within budget; it uses the same lock-free pre-check as `deskdispatch` and falls through to the locked check only near the threshold or when the recorded estimate is stale or unreadable, so concurrent desk launches no longer queue behind one another's full measurement.
