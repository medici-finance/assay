### Fixed
- CLI fixture repositories explicitly disable detached Git maintenance before snapshot and cleanup checks, preserving whole-tree dry-reset immutability assertions.
- Dry-reset fixture failures now show normalized before/after snapshots for diagnosis.
