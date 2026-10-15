### Fixed
- `make paired-versions` (the `plugin-drift` check) now fails when `plugins/assay/paired-versions.yaml` has been left behind by a newer published release for more than seven days, and names the re-pin. Before this, a pin that was consistent but stale passed every check, so new adopters installed old tools.
