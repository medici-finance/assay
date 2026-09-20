### Added

- `statusgen --export-audit-pack --release <tag>`: a release-keyed audit pack, walking
  release -> brief -> requirement -> Evidence/review verdict via `docs/release-notes/<tag>.md`'s
  optional scope frontmatter. Reuses `--export-evidence`'s existing `manifest.json` shape
  verbatim and refuses to write when an independent completeness comparison against the
  sdlc/02 rollup disagrees, naming both counts. See `docs/evidence-bundle.md`'s release-keyed
  section.
