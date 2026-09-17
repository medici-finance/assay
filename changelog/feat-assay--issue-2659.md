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
  `could-not-check` on a row that is not explicitly deferred by name in the same entry.

### Fixed
- Added a named regression test for the pipeline-exit worst-stage scoring fix (the shell
  already ran with `bash -o pipefail`; the new test pins the exact reported command shape).

### Changed
- `spec/lifecycle-v1.md` §2.4's "no execution witness" sentence is now date-bounded to
  closures before statusgen v1.0.13.
- `verify-desk`'s SKILL.md now names `statusgen verifyrun` as the step that produces the
  execution witness and instructs committing it alongside the Evidence row.
