### Added
- `scanloop` has its own replay bound, `INTAKE_REPLAY_CAP`: a positive integer sets how many retained intake events one pass presents per repository, so a large owed backlog can be drained in bigger batches without widening the poller's burst bound. Unset or invalid, the bound falls back to `INBOUND_MONITOR_BURST_CAP`, then 25, exactly as before.
