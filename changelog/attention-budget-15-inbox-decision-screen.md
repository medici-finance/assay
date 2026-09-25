### Added
- `assay-inbox.sh --walk` classifies every item before asking: `already-ruled` (a human
  comment lands after the desk's own relay, or after the point options were put — a bare
  desk relay is never a ruling), `no-fork` (the Options section, or a fork-test block,
  parses to fewer than two entries), `reversible-default` (a live `caught-by:` plus a
  `default:` with no `class:` line and no one-way term — the desk is already proceeding
  behind a gate the driver still holds), or `genuine`. Only `genuine` items are ever
  presented; "question k of n" now counts genuine items only. Nothing screened out is
  silently dropped: every `--walk` question prints a tail line of counts, `--walk
  --screened` lists every screened item in full (repo#number, class, the evidence), and
  the table/`--html` renderings carry a `class` on every row and hide nothing. `--no-screen`
  turns the screen off in one flag, byte-identical to the pre-screen rendering. An item the
  screen cannot read, or cannot classify (no known human-login list — printed once as a
  NOTICE), is always treated as genuine.
- The `ask-decision` skill states the four classes and what the desk does with each before
  its five-part format, which now applies to every GENUINE item rather than every item.
