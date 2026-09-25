### Fixed
- `deskevidence` no longer lets a stale local copy of a stream README revert an unrelated
  brief row. When the target's remote content carries the generated Briefs table (the
  `<!-- statusgen:briefs:begin/end -->` region), `--row <NN>` (repeatable) is now required and
  the committed content is rebased onto the remote — only the named row(s)' lifecycle cells
  (Status / Verified / Reviewed) come from the local file; the named rows' authoring cells,
  every other row and everything outside the markers come from the remote unchanged. A row
  that differs is reported (`stale-local: row <NN> ...`), never silently dropped. Duplicated
  row keys, malformed named rows, and a README whose marker does not parse are refused rather
  than guessed at. The constraint is enforced twice — against the pre-check's fetch, and again
  by the write op against a fresh fetch taken immediately before the commit — and the write
  itself is conditional on that fresh fetch's content id (new `WriteFileInput.ExpectedSHA`),
  so a table change after the re-check is refused by the forge rather than overwritten.
  Targets with no such table (brief-path merges, `.jsonl` sidecars) are unaffected.
