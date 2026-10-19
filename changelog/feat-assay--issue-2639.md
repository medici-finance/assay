### Fixed
- `muhar` reports Go mutants that fail to build as `COULD_NOT_MUTATE` and bounds each mutant's build-plus-test run using the measured baseline duration.
- An interrupted `muhar` run (SIGINT or SIGTERM) kills the suite commands it started instead of leaving them running.
- Two `internal/deskkit/mutations.json` mutants that never compiled now compile and are caught by a test.
