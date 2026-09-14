### Fixed
- `deskboard actions`' classifier (`classifyPR`) no longer fails the WHOLE sweep when a single
  open PR carries one unreadable change-level field. Four per-change reads — the PR's own
  reviews, the non-commit-resolution label probe, the own-files read and the reviewed-sha
  compare feeding the benign-merge check, and the changed-files read feeding risk
  classification — used to propagate a per-PR read failure as a whole-sweep error (exit 6,
  empty board, one line of diagnosis). Each now degrades only its OWN row, landing on the
  safe side (never the benign/cleared outcome), the same contract the changed-files
  truncation guard already documented for itself.
- A degraded row's RENDERED text now carries the could-not-check reason that produced the
  degrade, not just a line on stderr — so an operator reading the board can see which row
  degraded and why.
