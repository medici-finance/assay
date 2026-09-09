### Fixed
- The generated Briefs table no longer TRUNCATES a board's own columns. A stream
  README whose table carried a column beyond the canonical seven — a trailing
  `What's landed`, an `Owner` column — lost that column entirely on the first
  `statusgen migrate` / `regen --readmes`, header and every cell, because the
  render emitted a fixed seven cells. Non-canonical columns are now carried
  through verbatim, appended after `Reviewed` in the order the source header
  lists them. `Gate` remains the one column the layout deliberately drops (it
  duplicates the brief's own `gate:` frontmatter key), and it is now dropped
  header-and-cells together rather than shifting the row.
- `deskmigrate` no longer reports a SILENT no-op on an unmigrated tree. When no
  migration covers the requested span but the tree still carries
  `schema: brief-v1` files under `docs/streams/`, it exits non-zero naming the
  migrations directory it looked in and the vendoring step, instead of printing
  `no migrations for vX -> vY (clean no-op)` at exit 0 — output an operator
  cannot tell from a completed migration. A genuinely migrated tree is still a
  clean no-op at exit 0.

### Changed
- Every `deskmigrate` run now prints the number of migrations SELECTED and the
  number of planned file actions, so "nothing matched the span" reads
  differently from "matched, and already applied".
