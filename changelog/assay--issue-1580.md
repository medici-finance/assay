### Changed
- The worker dispatch kits (`worker` and `worker-objective`) gain a bug-fix clause: a fix names
  its defect CLASS in a `## Defect class` PR section, adds a guard that fails if any other site
  repeats the defect (modelled on an allow-list structural test over every caller of a
  hazardous primitive), and shows that guard red against a deliberately planted second
  instance. A test of the reported instance alone no longer discharges a fix. The
  `worker-desk` skill states the same obligation.
- The `author-brief` skill gains rule 14: a fix/bug brief carries an optional `regression-of:`
  frontmatter key (the earlier fix's issue reference or commit sha, when one exists) and a
  mandatory fail-first class-guard Verify row tagged `+mutation`. `regression-of:` is
  tolerated as an unrecognised key today; no lint validates it yet.

### Added
- `TestDefectClassClauseIsOneWordingAcrossImplementerKits` in `tools/desk/cmd/deskdispatch`
  fails when the defect-class clause is missing from either implementer kit or its wording
  differs between them.
