### Changed
- **Flag day: this repo's own board is now brief-v2.** All 148 briefs under
  `docs/streams/` were migrated from `schema: brief-v1` to `schema: brief-v2` by the
  declarative `v0.28.0 → v1.0.0` migration: each brief's `brief:` id becomes the
  hierarchical `<cell>:<repo>:<stream>:<NN>` form resolved through the alias registry,
  every brief gains `version: 1` and a once-minted uuid `id:`, and each of the 16 stream
  READMEs has its Briefs table wrapped in the generated-region markers with
  `board: generated` in its frontmatter. The lifecycle cells of every row were carried
  through unchanged — the migration re-shapes the board, it does not re-decide it.

### Added
- The migration this repo runs against itself now lives at
  `migrations/0001-v0.28.0-to-v1.0.0-derived-board.md`, so `deskmigrate` and
  `assay:upgrade-assay` can be dry-run against this tree the same way an adopter runs
  them against theirs.
- `docs/UPGRADING.txt` — the append-only local record of which migrations have been
  applied to this tree.
