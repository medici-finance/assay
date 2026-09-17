### Added
- `statusgen --lint` treats a `verified`/`done` closure made on the current branch with NO
  execution witness at all as a PROBLEM (not merely the existing per-stream NOTICE) —
  reusing the same merge-base scoping the witness-contradiction check already uses, so a
  pre-existing closure keeps its established rollup.
- The `**VERIFY: PASS**`/`**VERIFY: FAIL**` marker match is now a regex over the bold span
  (`\*\*VERIFY: (PASS|FAIL)\b[^*]*\*\*`), so a qualifier inside the bold — `**VERIFY: PASS
  (4/4 offline-runnable rows)**`, `**VERIFY: PASS — all 6 rows green.**` — is recognised;
  `BLOCKED` never matches.
- A `**VERIFY: PASS**` entry is no longer treated as a flip signal — by the verify-gate card
  or the `gate: model` autoflip — when its own Evidence entry also reads `HELD` or
  `could-not-check` on a row that is not explicitly deferred by name in the SAME ROW-SCOPED
  CLAUSE (sentence/line — see Fixed, below) as that row.

### Fixed
- Added a named regression test for the pipeline-exit worst-stage scoring fix (the shell
  already ran with `bash -o pipefail`; the new test pins the exact reported command shape).
- The HELD/could-not-check-vs-deferred check is now row-scoped (per sentence/line, split on
  `.`/`!`/`?`/`,`/`;`/em-dash/en-dash plus whitespace, or a newline) instead of entry-scoped:
  a deferral clause naming one row no longer clears the hold on a different, undeferred row
  in the same Evidence entry, including when the two rows are separated only by a comma,
  semicolon, or dash rather than sentence-ending punctuation.
  **Convention change for Evidence authors:** a deferral clause must now sit in the SAME
  clause as the row it defers — writing the deferral in a later sentence of the same entry
  (e.g. `row 11 could-not-check. Deferred per alias#99.`) is refused where it was previously
  accepted; write it in the same clause instead (`row 11 could-not-check, deferred per
  alias#99` or `row 11 is HELD (deferred per alias#99)`).

### Changed
- `spec/lifecycle-v1.md` §2.4's "no execution witness" sentence is now scoped to a closure
  the SAME BRANCH makes (relative to its merge-base with main), not merely date-bounded to
  closures before statusgen v1.0.13 — a pre-existing closure keeps its established rollup
  even though it postdates v1.0.13.
- `verify-desk`'s SKILL.md now names `statusgen verifyrun` as the step that produces the
  execution witness and instructs committing it alongside the Evidence row.
