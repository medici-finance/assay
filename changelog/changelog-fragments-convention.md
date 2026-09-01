### Changed
- Changelog highlights are now recorded as per-PR fragment files under `changelog/` instead of shared `## Unreleased` edits: `changelog-check` greens on a fragment (or `changelog:skip`) and refuses a direct `## Unreleased` edit, and the release workflow aggregates fragments into the dated section and release Highlights, then clears `changelog/`.
