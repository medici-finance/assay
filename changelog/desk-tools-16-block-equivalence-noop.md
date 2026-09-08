### Changed
- `deskevidence --brief-path` is now idempotent at the block level, not just the file level: a
  fresh Evidence block byte-equivalent (after normalising line endings, per-line trailing
  whitespace, and trailing blank lines) to the block already standing at the end of the brief's
  `## Evidence` section is a no-op — it prints `noop: Evidence block already present …`, exits 0,
  and commits nothing, instead of appending a duplicate. Equivalence is narrow: a re-run on a
  different date or runner, a one-character change, a partial (prefix) re-run, or a superset that
  adds new rows all count as new evidence and still land.
