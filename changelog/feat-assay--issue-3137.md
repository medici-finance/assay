### Fixed
- The cache mutation entry "trim stops at the target with entries past the trim age left" now builds: its mutant keeps the age variable in use, so the mutation gate can run it and the existing trim tests kill it instead of the entry reporting a compile failure as missed.
