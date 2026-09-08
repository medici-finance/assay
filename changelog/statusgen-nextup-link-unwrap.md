### Fixed
- **Next-up rows with a backtick-bracket-led title no longer keep a dead README link.** When a
  stream README status-table row's title began with a backticked bracketed tag (e.g.
  `` [`[assay]` …](./brief-…md) ``), statusgen copied the row into `STATUS.md`'s Next-up table
  with the stream-relative `./brief-…` target intact — dead from the repo root and reddening any
  snapshot that copied it. The title-cell unwrap used a `\[([^\]]+)\]\(…\)` regexp that stopped at
  the first inner `]`, so the outer link never unwrapped. It now unwraps with a bracket-DEPTH walk
  (matching `[`↔`]` and `(`↔`)` by depth), so a title whose link text itself contains brackets
  strips to a bare title like every other row. (#591)
