### Fixed
- The `forge-local-authority` CI job now prints a failing qualification call's exit code (or timeout) and the last 200 lines of its stderr and stdout before reporting `UNQUALIFIED: <label>`.
- When a production control fails, the job log now names the control, the failing stage (baseline, uncaught mutant, restored run, or a timeout), its exit code, and the last 150 lines of that stage's nested `go test -v` run, so the log shows which test failed and what it printed last. Whether that failure is a flake or a real one can still need a re-run of the job at the same commit.
