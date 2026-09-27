### Added
- `qualgen` gains the re-fix rate metric (quality/19): a `RegressionLinkage` seam
  (explicit `regression-of:` or a shared defect-class label) joins traced
  SZZ fixes against earlier fixes, and `qualgen report` renders a new
  `## Re-fix rate (regression-suite effectiveness)` trend section — report-only,
  gating nothing until it has been measured across ≥ 2 windows.

### Fixed
- Re-fix linkage fails closed on unresolvable data: an errored or unverifiable
  linkage check (an explicit-path error, a `regression-of:` reference that
  matches no identified fix — including one unmatched reference alongside
  matched ones — an earlier fix's unreadable labels, an unknown mined-repo
  identity behind a repo-qualified `regression-of:`) is could-not-measure —
  never a fabricated measured non-re-fix — whenever no resolved candidate
  satisfies the ordering rule; a matched candidate that does satisfy it still
  earns the measured re-fix.
- Report rendering escapes a could-not-measure Reason for the markdown table
  (pipes escaped, newlines flattened), so adapter error text or frontmatter
  values can no longer break a table row.
