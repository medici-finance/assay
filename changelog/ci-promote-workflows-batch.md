### Changed
- The `changelog-check` PR gate now enforces the fragment convention (`changelog/<slug>.md` with at least one highlight bullet, or `changelog:skip`) and refuses direct `## Unreleased` edits; `release.yml` aggregates the fragments into the release highlights, refuses to cut with nothing to aggregate, and rolls them into a dated `CHANGELOG.md` section in the release commit.

### Added
- `assay-statusgen.yml` gains a `model-autoflip` job: after each push to main, `statusgen --auto-flip-model` advances `gate: model` briefs from `verified` to `done` only when the reviewer App's approval sits at the merging PR's head; anything it cannot corroborate stays `verified` and fails the run loudly.
