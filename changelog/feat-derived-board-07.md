### Fixed
- `statusgen reconcile`'s PR-trailer join matched a brief-v2 hierarchical id
  (`<cell>:<repo>:<stream>:<NN>`) against a PR's short `Brief: <stream>/<NN>`
  trailer as a bare string, so the two never matched — every PR-derived lifecycle
  cell on a brief-v2 tree stuck at `todo` no matter how many trailer-carrying PRs
  had merged. Both sides now reduce to the same `<stream>/<NN>` key before the
  join.

### Added
- This repo's own `docs/streams/*/README.md` boards get a `statusgen reconcile
  --backfill --report` pass: `docs/streams/board-drift-2026-09-16.md` records
  every brief where the hand-said lifecycle cell disagrees with what PR history
  (plus the declared history-only backfill fallback) now derives, for a human to
  resolve by linking or accepting.
- Regenerated `.github/assay-statusgen.reconcile.patch` (still staged, not
  applied — a workflow-file push needs the workflows scope) as an actual
  `git apply`-able unified diff; the prior version's bare `@@` hunk headers
  carried no line-range info and could not be applied as its own instructions
  said.
