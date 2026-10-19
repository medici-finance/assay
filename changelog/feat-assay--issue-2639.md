### Fixed
- `muhar` reports Go mutants that fail to build as `COULD_NOT_MUTATE`. It bounds each mutant's build and suite runs separately, each to a limit taken from the measured baseline duration. A suite that hits its limit is `CAUGHT` and marked `TIMEOUT`. A positive control caught only by a timeout makes the run `HARNESS BROKEN`.
- An interrupted `muhar` run (SIGINT or SIGTERM) kills the suite commands it started instead of leaving them running.
- Two `internal/deskkit/mutations.json` mutants that never compiled now compile and are caught by a test.
