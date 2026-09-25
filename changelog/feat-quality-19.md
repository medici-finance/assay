### Added
- `qualgen` gains the re-fix rate metric (quality/19): a `RegressionLinkage` seam
  (explicit `regression-of:` or a shared defect-class label) joins traced
  SZZ fixes against earlier fixes, and `qualgen report` renders a new
  `## Re-fix rate (regression-suite effectiveness)` trend section — report-only,
  gating nothing until it has been measured across ≥ 2 windows.
