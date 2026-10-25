### Fixed
- The cadence supervisor's heartbeat-time guard check, which stops a running pass when the guard trips mid-pass, is now pinned by its own test and by a mutation entry. Previously the package's tests passed with that check disabled, so a regression there would have gone unnoticed.
