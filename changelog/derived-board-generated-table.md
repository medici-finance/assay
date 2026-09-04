### Added
- **Generated Briefs table in stream READMEs (`board: generated`).** A stream
  README whose frontmatter carries `board: generated` opts its Briefs table into a
  marker-wrapped generated region (`<!-- statusgen:briefs:begin -->` …
  `<!-- statusgen:briefs:end -->`). The new `statusgen regen --readmes` verb
  writes the authoring columns (#, title, wave, effort) from the brief frontmatter
  and is idempotent; a hand edit to those columns is a `statusgen --lint` PROBLEM,
  the same single-writer discipline `STATUS.md` already has (rule 47). Everything
  outside the markers is left byte-for-byte untouched.
- **Board-vs-witness drift comparator (interim).** `statusgen regen --readmes`
  with an online `--repo` folds the PR witnesses through the lifecycle derivation
  and prints a NOTICE for every board cell that disagrees with the derived state,
  making drift visible before a cell is hard-flipped to witness-written. Offline it
  is inert — a could-not-check is never rendered as a drift.
- `statusgen init` gains `--dry-run` (preview the scaffold — paths and bodies —
  without writing) and accepts the target as a positional directory; the scaffolded
  CI workflow regenerates the stream README tables alongside `STATUS.md`.
