### Changed
- `statusgen` board-lint now names the fix inline when a Briefs-table Status cell
  is malformed: the `invalid status` PROBLEM and the row cell-count error both
  explain that the Status cell takes only a bare lifecycle token and that a PR
  reference belongs in the PR body / `Brief:` trailer, never in the cell. The
  cell-count check also now rejects a row with EXTRA cells (a decorated
  `implemented (#NN)` value plus a stray `||` shifts every column right), which
  previously slipped past the lint and surfaced as a confusing downstream error.
