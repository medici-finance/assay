### Fixed
- `statusgen verifyrun` now stops as could-not-run (exit 2) when an admitted run cannot write the line saying the declared replay path was withheld from its rows. Before, the failed write was dropped and the run could pass with no record that the path had been refused.
- `deskinbox show` now exits unverifiable (6) when its report cannot be written, instead of exiting 0 with a partial or missing report. The report is composed in memory and written once.
- `fanoutloop plan` now returns the write error when an issue-PR hold row cannot be written.
- `statusgen acceptance-route prepare` and the issue-PR placeholder read now return a failed close of the file they read, when nothing else failed first.
- The go-lint ratchet repairs the unchecked writes and closes that merged after the last repair (in `statusgen` acceptance-route and verifyrun, `deskboard dispatch`, `deskinbox changes` and `show`, `fanoutloop`, and `deskkit`), and prunes 4 baseline allowances that no longer match a live finding. Nothing was added to the baseline, and the gate passes at main.
