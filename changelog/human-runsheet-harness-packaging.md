### Fixed
- `human-runsheet` is now accounted for in the Codex and Cursor packaging
  rosters and carries a degradation cell in all three capability-matrix
  reference files, clearing the coverage gap that failed `harnessgen codex`
  (exit 2) inside `release.yml`'s plugin-manifest-stamp step and blocked the
  v1.0.7 cut (#970).
