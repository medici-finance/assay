### Fixed
- The cadence supervisor's two heartbeat-time guard checks, which stop a run when the guard trips mid-pass or during between-pass maintenance, are now each pinned by their own test and by a mutation entry. Previously the package's tests passed with either check disabled, so a regression there would have gone unnoticed.
