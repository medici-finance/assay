### Added
- `plugins/assay/scripts/pr-monitor.sh` — a durable, stateful open-PR monitor for the review
  desk: per-repo head-sha / draft-state / state / merge-state baselines, silent seed on first
  sight, and one machine-parsable `PR-EVENT: <slug>#<num> <kind> <old> -> <new>` line per change
  (`opened | pushed | draft-flip | state | merge-state | closed`). It paces its reads
  (`ASSAY_MONITOR_PACE_SECONDS`, default 2) and caps a cycle (`ASSAY_MONITOR_MAX_REPOS_PER_CYCLE`,
  default 0 = all), and ends a cycle on a secondary-rate-limit / 429 signature without further
  calls, so the watcher can no longer become the tight-loop poll that trips the forge's limit.

### Changed
- `plugins/assay/scripts/inbound-monitor.sh` adopts the same `ASSAY_MONITOR_PACE_SECONDS` sleep
  between repo reads and the same stop-on-limit rule; its behaviour is otherwise unchanged.
