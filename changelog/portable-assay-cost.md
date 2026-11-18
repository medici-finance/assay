### Changed
- `deskroster list` reuses the current repository listing for registered open changes and checks missing entries individually before pruning.
- A legacy roster entry whose short name matches multiple repositories retains unknown state and protects the matching PR number in each repository until the owner is identified.
