### Fixed
- The `forge-local-authority` CI job now prints a failing qualification call's exit code (or timeout) and the last 200 lines of its stderr and stdout before reporting `UNQUALIFIED: <label>`, so a red can be read as a flake or a real failure from the log alone.
- The production-controls step of that job now also prints the last 200 lines of the nested `go test -v` run behind a failed baseline, uncaught mutant, failed restore or timeout, so the log shows which test failed and why.
