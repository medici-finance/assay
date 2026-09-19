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
  `could-not-check` on a row that is not explicitly deferred by name in the SAME ROW-NUMBERED
  SEGMENT (see Fixed, below) as that row.

### Fixed
- Added a named regression test for the pipeline-exit worst-stage scoring fix (the shell
  already ran with `bash -o pipefail`; the new test pins the exact reported command shape).
- The HELD/could-not-check-vs-deferred check is now ROW-NUMBER-ANCHORED instead of
  entry-scoped or punctuation-clause-scoped: a deferral clause naming one row no longer
  clears the hold on a different, undeferred row in the same Evidence entry, no matter what
  punctuation (or none) separates them. Two earlier shapes of this fix scoped the check by
  splitting the entry into clauses on an enumerated list of punctuation, and each was
  reopened by a separator the list had not enumerated yet (first a plain sentence boundary
  missed comma/semicolon/dash-separated rows; then that broadened list still missed a plain
  hyphen, a colon, a parenthetical aside, an ampersand). The check now anchors on the `row N`
  reference every real HELD/could-not-check/deferred-per entry in this repo's own corpus
  already names, splitting the entry into row-scoped segments at each `row N` mention instead
  of guessing where prose punctuation ends one row's clause and starts the next — closing the
  laundering class itself rather than the one separator most recently demonstrated.
  **No convention change for Evidence authors** (this corrects the prior fix's narrowing): a
  deferral clause deferring the one row named before it clears the hold whether it sits in the
  same clause (`row 11 is HELD (deferred per alias#99)`) or a later sentence of the same entry
  (`row 11 could-not-check. Deferred per alias#99.`) — the ratified convention is unchanged
  either way. An entry with no `row N` mention at all has no structure to scope a deferral
  against and now fails CLOSED: any `HELD`/`could-not-check` token in it refuses the entry
  outright, regardless of any deferral clause elsewhere in it.

### Changed
- `spec/lifecycle-v1.md` §2.4's "no execution witness" sentence is now scoped to a closure
  the SAME BRANCH makes (relative to its merge-base with main), not merely date-bounded to
  closures before statusgen v1.0.13 — a pre-existing closure keeps its established rollup
  even though it postdates v1.0.13.
- `verify-desk`'s SKILL.md now names `statusgen verifyrun` as the step that produces the
  execution witness and instructs committing it alongside the Evidence row.
