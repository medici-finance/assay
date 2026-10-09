### Fixed
- `statusgen reconcile` / `regen` drift comparator: the lifecycle fold's four
  inputs (Witnesses, Approvals, Rulings, IssueLabels) are now populated from
  real reads — both production callers previously built
  `LifecycleInput{Briefs: idents}` alone, so no live cell could derive above
  `implemented` and the drift comparator reported every `done` brief as drift
  (#1787). The verify witness is each brief's own Evidence audit (the
  `verifyrun --check` code path), a gate:human ruling is the Reviewed-cell
  `human:` stamp, the App approval is read at the merged head of each
  witnessed gate:model brief, and one paged open-issues read feeds the
  blocked overlay; every failed read is disclosed and its overlay left off.
- Drift comparator: the asserted-vs-derived join is id-shape-tolerant
  (`canonicalBriefKey`), so brief-v2 hierarchical ids are no longer silently
  invisible to it.
- derived-board/03 brief: Verify row 1 re-authored to a single summed count
  (the three-command form let the witness score only the last suite), new row
  9 asserts a known-done brief derives `done`, `version:` 2 → 3.
