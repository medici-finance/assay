### Changed
- `deskflip`'s `reviewer-approved` condition gains ONE narrowed exemption to the
  standing-`CHANGES_REQUESTED` block: a check-only CR. A re-approve at an unchanged
  head clears the block only when the CR declares `Blocked-On-Check: <check>` as its
  sole finding, a later approve from the same reviewer at the same head cites
  `Cleared-Check-Run: <id>`, and that run is in the rollup at that head, carries the
  check the CR named, is a completed success, and completed after the CR. Anything
  short of all five refuses exactly as before.

### Added
- The forge interface's `CheckRun` now carries the forge's per-execution run `ID`
  (GitHub check-run id, GitLab pipeline-job id). An absent id maps to `""`, never
  `"0"`, so a citation of `0` can never match a run the forge never identified.
