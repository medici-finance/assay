### Added
- `statusgen --lint --changed-only <paths>`: a local pre-push convenience that
  scopes a lint to a stated path set and prints a scope banner naming what was
  (and was not) examined — refuses outright, non-zero, with no override, when
  it detects it is running inside the CI gate.
- `docs/statusgen-lint-reach.md`: a short contract stating exactly what
  `statusgen --lint` may reach on the network, with and without `--forge`.

### Changed
- `LoadHistory` (the `docs/streams/.history.jsonl` reader) is memoised on
  `(path, mtime, size)`, the same shape the brief-file parse memo already
  uses — one `--lint` previously re-read and re-decoded the same history log
  from multiple call sites in a single run.

### Fixed
- Added test coverage proving `--lint` (no `--forge`) makes no forge process
  start against a fixture that genuinely exercises dead-claim decay's forge
  read, and that every check reading through the run's forge reader renders
  could-not-check offline rather than a fabricated clean result.
