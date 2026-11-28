### Fixed
- `muhar -select` now instruments the mutated files when the spec root is relative or is reached through a symlink (macOS's `TMPDIR` is one), and when a mutation names its file through a different alias than the root. Before, nothing was instrumented and every mutant quietly fell back to the full suite.

### Added
- `muhar -select` accepts a spec whose `go test` command carries one top-level `-skip`. A skipped test is never listed, attributed or run alone, and every narrowed command keeps the spec's `-skip` unchanged. A repeated or subtest `-skip`, and any other flag selection does not model, are still refused.
