### Changed
- The plugin's `paired-versions.yaml` is re-pinned to statusgen / desk-tools **v0.25.1** and
  plugin **0.5.0** — both sides of the pairing move to the same tag, with fresh per-platform
  sha256 pins harvested from the v0.25.1 release checksums.

### Added
- A `check-paired-versions.sh` guard (with an offline test) now asserts the pairing holds, both
  locally and in CI: the manifest's `plugin` must equal `plugin.json`'s `version`, every pinned
  `statusgen` and `desk-tools` tag must be the SAME tag, and every `sha256` must be 64 lowercase
  hex — so a re-pin that leaves the two files disagreeing (the drift that once shipped adopters a
  stale tool) fails the check instead of merging.
