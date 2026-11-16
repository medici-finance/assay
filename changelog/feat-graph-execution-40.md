### Added
- `loopadmin/process` now has a test pinning that a launch with a nil environment starts the child with an empty environment and inherits nothing from the parent (graph-execution/40 residue).
- `docs/streams/decisions/DR-process-extract.md` adds the approved design record (`**Status:** approved`) for the process-runner move (one runner behind a mandatory host-custody port). Its `ruling:` links the driver's ruling on the brief's decision issue, and graph-execution/40 cites it with `design:` and records the same ruling.
