### Fixed
- **`statusgen migrate brief-v1-to-v2` no longer refuses a tree that has a register.**
  The migration enumerated every directory under `docs/streams/` as a stream, so a
  REGISTER directory — which by design carries a README with no frontmatter and no
  Briefs table — aborted the whole flag day with `exit 5: no recognisable Briefs table`
  and left every real stream unmigrated. It now applies the same two rules stream
  discovery already applies: the reserved register names, and the "a register, not a
  stream" self-declaration.
- **A brief whose title contains a `|` no longer breaks the board it is rendered into.**
  The generated Briefs table interpolated titles raw, so a title such as
  `` `--cadence weekly|monthly` `` emitted a row with one cell too many and the board's
  own parser then rejected the whole stream. Titles are now escaped for the cell they
  land in; the parser already understood the escaped form, so only the render half was
  missing.
- **A board with an extra authoring column keeps its lifecycle cells through a
  re-render.** The Status / Verified / Reviewed columns were read back from fixed
  offsets, which is correct only for the canonical seven-column table. A board carrying
  an extra column (a `Gate` column between Effort and Status is the shape in the wild)
  had every lifecycle cell read one position to the left, so a re-render wrote the gate
  value into Status and the real status into Verified — silent loss of lifecycle state,
  on the one run hardest to notice because a hundred other files change with it. The
  columns are now keyed on the header names, as the board parser already does.
