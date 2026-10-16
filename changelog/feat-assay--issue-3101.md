### Added
- `statusgen` writes the same local `perf-v1` performance record as the desk tools when `ASSAY_TELEMETRY=1` (tool `statusgen`, verb = its mode, wall and CPU milliseconds, exit and result), so `deskperf report --tool statusgen` works. With the variable unset it writes nothing. The network telemetry payload and its double opt-in are unchanged.

### Changed
- The `perf-v1` writer moved into a small shared module, `perfrec/`, that the desk tools and `statusgen` both call; the record schema, day-file layout, 7-day retention and install id are unchanged.
