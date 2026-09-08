### Changed
- `deskboard` now reaches every forge read through the typed `Forge` seam: its last
  `gh` reads — PR search, commit-history listing, single-commit read, the combined-status
  total, the workflow-directory listing, plus the reviews / changed-files / compare /
  label-events / comments / contents / raw-diff / repo-visibility / verify-gate-issue reads —
  are migrated onto typed ops, and the `gh` choke point (`board.go`'s `ghRun`) is deleted.

### Added
- Six typed read ops on the frozen `Forge` interface, each with its `deskboard` call site
  in the same change: `ListRecentCommits`, `GetCommit`, `CompareRefs`, `SearchOpenChanges`,
  `ListWorkflowFiles`, and `ChangeDiff`. The commit reads map 1:1 on GitLab; the other four
  are could-not-check-with-gap there (a genuine non-1:1 each). `PullRequest` gains `MergedAt`
  and `Merged`, `IssueSummary` a `URL`, and `LabelEvent` a `CreatedAt`, each with its
  consumer; the combined-status total folds into `ChecksAtHead` rather than a new op.

### Fixed
- The forge-CLI ban ceiling falls 13 → 12 with `deskboard`'s permit row removed — `cmd/deskboard`
  now carries no `gh` literal at all.
