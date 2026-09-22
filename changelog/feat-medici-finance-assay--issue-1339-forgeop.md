### Added

- Typed forge seam op `ListChanges(repo, states)` — reads a repo's changes (PRs ↔ MRs) in the requested lifecycle states, keeping `MERGED` distinct from `CLOSED`, with bounded most-recently-updated pagination that reports `Incomplete` rather than hand back a silent partial. GitHub (GraphQL, states variable) and GitLab (`state=all` narrowed client-side) backends, both under the closed forge surface (no forge CLI).

### Fixed

- `deskdispatch`'s pre-claim phantom check now goes live: its represented-PR transport reads the repo's open+merged changes through the new typed seam op and refuses a fresh worker dispatch whose brief already has an open or merged PR (matched on the PR body's `Brief:` trailer, not a branch name) (#1339).
