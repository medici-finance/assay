### Fixed
- Added the `system-demo` skill to the Codex and Cursor packaging coverage rosters
  (`plugins/assay/codex/packaging.md`, `plugins/assay/cursor/packaging.md`) as
  `packaged`, plus its degradation cell in both harness binding files
  (`plugins/assay/references/codex.md`, `plugins/assay/references/cursor.md`) —
  `harnessgen codex --check` / `harnessgen cursor --check` were failing with an
  unaccounted-for-skill coverage error.
