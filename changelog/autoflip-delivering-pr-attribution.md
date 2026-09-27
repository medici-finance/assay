### Fixed
- `statusgen --auto-flip-model` no longer credits a PR that did not deliver the brief. A candidate PR is credited only when its single `Brief:` trailer names this brief. The resolver walks past a PR that names a different brief, an `Authors:`-only authoring PR, and a bulk-shaped PR (a `docs/streams/**`-only diff, or 3 or more `Brief:`/`Authors:` trailers) to older commits. A PR with no `Brief:` trailer, or with several, is could-not-check (#1691).
- Walking past a candidate never removes the approval requirement. Every PR the walk reaches must carry the reviewer App's APPROVED review at its own merged head, so an unapproved later change to a brief blocks the flip as before (#1691).
- The candidate's file list is read from the paginated REST endpoint and checked against the PR's `changed_files` count. A truncated list is could-not-check. `gh pr view --json files` stops at 100 entries and was previously trusted as complete (#1691).

### Changed
- A docs-only Verify PR is no longer credited as a brief's delivering PR. The walk continues past it to the delivery PR. A brief whose delivery PR never touched the brief file, or carries no `Brief:` trailer naming it, now stays `verified` with a structural NOTICE (exit 0) instead of flipping to `done`. It is not refused (#1691).
