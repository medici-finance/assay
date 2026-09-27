### Added
- `qualgen` gains the re-fix rate metric (quality/19): a `RegressionLinkage` seam
  (explicit `regression-of:` or a shared defect-class label) joins traced
  SZZ fixes against earlier fixes, and `qualgen report` renders a new
  `## Re-fix rate (regression-suite effectiveness)` trend section — report-only,
  gating nothing until it has been measured across ≥ 2 windows.

### Fixed
- Re-fix linkage fails closed on unresolvable data: an errored or unverifiable
  linkage check (an explicit-path error, an earlier fix's unreadable labels, an
  unknown mined-repo identity behind a repo-qualified `regression-of:`) is
  could-not-measure — never a fabricated measured non-re-fix — whenever no
  resolved candidate satisfies the ordering rule.
