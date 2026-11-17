### Fixed
- The `forge-local-authority` CI job now prints a failing qualification call's exit code (or timeout) and the last 200 lines of its stderr and stdout before reporting `UNQUALIFIED: <label>`, so a red can be read as a flake or a real failure from the log alone.
