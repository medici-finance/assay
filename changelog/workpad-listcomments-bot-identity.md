### Fixed
- `deskreply --workpad` now finds and edits its own prior workpad comment instead of
  appending a new one on every call. `GitHubForge.ListComments` re-suffixes a GraphQL Bot
  author's bare slug to the `<slug>[bot]` REST rendering, so a worker's own comment matches
  its own identity through `SameActor` and the one-workpad-per-PR upsert holds (#747).
