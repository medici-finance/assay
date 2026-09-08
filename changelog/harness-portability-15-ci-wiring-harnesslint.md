### Added
- harness-portability/15 (spec): the follow-up brief to hp/14 (#631) — wire the three de-housed
  Go modules' test suites (`harnessgen`, `harnesslint`, `plugindrift`) plus the real-tree
  harness-neutrality lint into public `ci.yml` so roster/version drift can no longer land green,
  scrub the four banned harness tokens flagged in `ask-decision`/`install` skill bodies, and
  declare `references/desk-shell.md` a non-matrix reference the `harnesslint bindings` check skips
  by declaration (not nineteen per-line suppressions).
