### Fixed
- `deskevidence` no longer lets a stale local copy of a stream README revert an unrelated
  brief row. When the target's remote content carries the generated Briefs table (the
  `<!-- statusgen:briefs:begin/end -->` region), `--row <NN>` (repeatable) is now required and
  the committed content is rebased onto the remote — only the named row(s) come from the
  local file, every other row and everything outside the markers comes from the remote
  unchanged. A foreign row that differs is reported (`stale-local: row <NN> differed and was
  NOT written`), never silently dropped. The constraint is enforced twice: once against the
  pre-check's fetch, and again by the write op against a fresh fetch taken immediately before
  the commit, so a table change in the race window between the two is still caught. Targets
  with no such table (brief-path merges, `.jsonl` sidecars) are unaffected.
